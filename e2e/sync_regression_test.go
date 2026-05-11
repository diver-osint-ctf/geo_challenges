// Regression tests for the user-reported bug: after dynamic scoring drops a
// geo challenge's value (e.g. 500 → 100 across several solves), running
// `ctf challenge sync` resets the score back to the yaml-authored initial
// (500). The default CTFd update path is the culprit — BaseChallenge.update
// blindly setattrs every field the yaml carries, including `value`,
// clobbering whatever dynamic scoring computed.
//
// These tests reproduce the bug end-to-end (yaml → ctfcli install →
// solves → ctfcli sync) and ASSERT the dynamic-scored value is preserved.
// They fail on a vanilla geo_challenges install today; they will pass
// once the plugin (or ctfcli, or CTFd) stops overwriting `value` during
// sync.
package e2e

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/diver-osint-ctf/ctfd-plugin-e2e/testutil"
)

const dynamicChallengeYAML = `name: %s
category: %s
description: regression coverage for ctfcli sync resetting dynamic score
value: 500
type: geo
state: visible
flags:
  - flag{cli}
extra:
  latitude: 35.6586
  longitude: 139.7454
  tolerance_radius: 1000
  initial: 500
  minimum: 100
  decay: 2
`

// TestGeo_CTFCLISyncDuringCTFMustPreserveDynamicValue — sync runs while the
// CTF is still live. Even then the dynamic-scored value must not snap back
// to yaml's `value:`.
func TestGeo_CTFCLISyncDuringCTFMustPreserveDynamicValue(t *testing.T) {
	admin := testutil.AdminClient(t)
	ns := testutil.Namespace(t)

	chalID := installViaCTFCLI(t, admin, ns)
	dropped := driveValueDown(t, admin, ns, chalID, 3)

	syncViaCTFCLI(t, ns)

	got := readValue(t, admin, chalID)
	if got != dropped {
		t.Errorf("ctfcli sync mid-CTF reset the dynamic-scored value: before=%d, after=%d", dropped, got)
	}
}

// TestGeo_CTFCLISyncAfterCTFMustPreserveDynamicValue — same flow but the
// admin marks the CTF as over (sets `end` to the recent past) before
// syncing. This is the exact sequence in the bug report.
func TestGeo_CTFCLISyncAfterCTFMustPreserveDynamicValue(t *testing.T) {
	admin := testutil.AdminClient(t)
	ns := testutil.Namespace(t)

	chalID := installViaCTFCLI(t, admin, ns)
	dropped := driveValueDown(t, admin, ns, chalID, 3)

	original := readEnd(t, admin)
	t.Cleanup(func() { setEnd(t, admin, original) })
	setEnd(t, admin, fmt.Sprintf("%d", time.Now().Unix()-10))

	syncViaCTFCLI(t, ns)

	got := readValue(t, admin, chalID)
	if got != dropped {
		t.Errorf("ctfcli sync after CTF end reset the dynamic-scored value: before=%d, after=%d", dropped, got)
	}
}

// ----- ctfcli orchestration -----

func projectRoot(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "ctfd-plugin-e2e"))
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(p, ".ctf", "config")); err != nil {
		if v := os.Getenv("E2E_PROJECT_ROOT"); v != "" {
			return v
		}
		t.Fatalf("could not locate project root at %s (no .ctf/config); set E2E_PROJECT_ROOT to override", p)
	}
	return p
}

// challengeDir creates a per-test challenge directory under
// .data/test-challenges/ inside the project root and writes challenge.yml.
// ctfcli rejects paths outside its project_path, so a temp dir elsewhere
// would not work.
func challengeDir(t *testing.T, ns string) string {
	t.Helper()
	base := filepath.Join(projectRoot(t), ".data", "test-challenges", ns+"-dyn")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir challenge dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	yaml := fmt.Sprintf(dynamicChallengeYAML, challengeName(ns), ns+"-cat")
	if err := os.WriteFile(filepath.Join(base, "challenge.yml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write challenge.yml: %v", err)
	}
	return base
}

func challengeName(ns string) string { return ns + "-dyn-cli" }

func runCTFCLI(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("ctf", args...)
	cmd.Dir = projectRoot(t)
	cmd.Env = append(os.Environ(),
		"CTFCLI_URL="+testutil.CTFdURL(t),
		"CTFCLI_ACCESS_TOKEN="+testutil.AdminToken(t),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ctf %v failed: %v\n%s", args, err, out)
	}
	t.Logf("ctf %v ok\n%s", args, out)
}

func installViaCTFCLI(t *testing.T, admin *testutil.Client, ns string) int {
	t.Helper()
	dir := challengeDir(t, ns)
	runCTFCLI(t, "challenge", "install", "--challenge", dir)
	id := findChallengeIDByName(t, admin, challengeName(ns))
	t.Cleanup(func() {
		_, _ = admin.DoJSON(http.MethodDelete, fmt.Sprintf("/api/v1/challenges/%d", id), nil, nil)
	})
	return id
}

func syncViaCTFCLI(t *testing.T, ns string) {
	t.Helper()
	dir := challengeDir(t, ns)
	runCTFCLI(t, "challenge", "sync", "--challenge", dir)
}

func findChallengeIDByName(t *testing.T, admin *testutil.Client, name string) int {
	t.Helper()
	var resp struct {
		Data []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if _, err := admin.GetJSON("/api/v1/challenges?per_page=200&view=admin", &resp); err != nil {
		t.Fatalf("list challenges: %v", err)
	}
	for _, c := range resp.Data {
		if c.Name == name {
			return c.ID
		}
	}
	t.Fatalf("challenge %q not found after ctfcli install", name)
	return 0
}

// ----- shared helpers -----

// driveValueDown solves the challenge `n` times with fresh users and returns
// the resulting (decreased) challenge value. Fails if the value didn't drop.
func driveValueDown(t *testing.T, admin *testutil.Client, ns string, challengeID, n int) int {
	t.Helper()
	for i := 1; i <= n; i++ {
		u := testutil.CreateUser(t, admin, ns, i)
		uc := testutil.UserClient(t, u.Name, u.Password)
		r := submitCoords(t, uc, challengeID, tokyoLat, tokyoLon)
		if r.Status != "correct" {
			t.Fatalf("solve %d: expected correct, got %s (%s)", i, r.Status, r.Message)
		}
	}
	v := readValue(t, admin, challengeID)
	if v >= 500 {
		t.Fatalf("expected value to drop below initial 500 after %d solves, got %d", n, v)
	}
	return v
}

func readEnd(t *testing.T, admin *testutil.Client) string {
	t.Helper()
	var resp struct {
		Data struct {
			Value string `json:"value"`
		} `json:"data"`
	}
	r, _ := admin.GetJSON("/api/v1/configs/end", &resp)
	if r != nil && r.StatusCode >= 400 {
		return ""
	}
	return resp.Data.Value
}

func setEnd(t *testing.T, admin *testutil.Client, value string) {
	t.Helper()
	resp, err := admin.DoJSON(http.MethodPatch, "/api/v1/configs/end",
		map[string]any{"value": value}, nil)
	if err != nil {
		t.Fatalf("set end: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		t.Fatalf("set end %q: HTTP %s", value, resp.Status)
	}
}
