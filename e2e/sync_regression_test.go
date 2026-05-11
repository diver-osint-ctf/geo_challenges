// Regression tests for d3d7687 ("feat: improve challenge scoring parameter
// handling"). The bug: running `ctf sync` re-PUTs the challenge with the
// dynamic-scoring fields (initial / minimum / decay) and, when the source
// yaml emptied any of them, the previous code wrote empty strings into the
// Integer DB columns and killed dynamic scoring — `all([challenge.initial,
// ...])` becomes False thereafter, so the value pinned at whatever it had
// dropped to during play. Subsequent solves no longer recompute it.
//
// We cover both the "sync during the CTF" and "sync after the CTF ended"
// paths because the user-reported failure happens specifically when the
// admin re-runs sync after the event for cleanup / archival.
//
// We can't directly read initial/minimum/decay back via the JSON API:
// CTFd's BaseChallenge.read masks them as null whenever `function`
// (CTFd's own scoring function) is "static", which is geo's default —
// the plugin runs its own dynamic-scoring loop on top. So instead we
// probe behaviour: another solve must still drop the value.
package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/diver-osint-ctf/ctfd-plugin-e2e/testutil"
)

// dynamicParams mimics the body ctfcli re-sends for a dynamic geo
// challenge during sync. Numbers are kept as the yaml authored them.
var dynamicParams = map[string]any{
	"initial": 500,
	"minimum": 100,
	"decay":   2,
}

// TestGeo_DynamicScoringSurvivesSyncDuringCTF — value-drop, sync mid-event,
// then another solve must keep dropping.
func TestGeo_DynamicScoringSurvivesSyncDuringCTF(t *testing.T) {
	admin := testutil.AdminClient(t)
	ns := testutil.Namespace(t)

	chal := createDynamicGeo(t, admin, ns)
	dropped := driveValueDown(t, admin, ns, chal.ID, 2)

	if _, err := patchChallenge(t, admin, chal.ID, dynamicParams); err != nil {
		t.Fatalf("PATCH challenge mid-CTF: %v", err)
	}
	requireValueDropsOnNextSolve(t, admin, ns, chal.ID, dropped, 3)
}

// TestGeo_DynamicScoringSurvivesSyncAfterCTF — value-drop, mark CTF over,
// sync, reopen, solve once more — the value must still fall. This is the
// concrete shape of the user-reported regression.
func TestGeo_DynamicScoringSurvivesSyncAfterCTF(t *testing.T) {
	admin := testutil.AdminClient(t)
	ns := testutil.Namespace(t)

	chal := createDynamicGeo(t, admin, ns)
	dropped := driveValueDown(t, admin, ns, chal.ID, 2)

	original := readEnd(t, admin)
	t.Cleanup(func() { setEnd(t, admin, original) })

	// 1) End the event.
	setEnd(t, admin, fmt.Sprintf("%d", time.Now().Unix()-10))

	// 2) Run the sync that the bug report blames (ctfcli post-event push).
	if _, err := patchChallenge(t, admin, chal.ID, dynamicParams); err != nil {
		t.Fatalf("PATCH challenge post-CTF: %v", err)
	}

	// 3) Reopen so we can observe the behaviour. (The DB state was
	//    already written by step 2; reopening just lets a solve probe it.)
	setEnd(t, admin, "")

	requireValueDropsOnNextSolve(t, admin, ns, chal.ID, dropped, 3)
}

// ----- helpers -----

func createDynamicGeo(t *testing.T, admin *testutil.Client, ns string) *testutil.Challenge {
	t.Helper()
	return testutil.CreateChallenge(t, admin, ns, "geo", testutil.ChallengeGeo, testutil.ChallengeOpts{
		Value: 500,
		Extra: map[string]any{
			"latitude":         tokyoLat,
			"longitude":        tokyoLon,
			"tolerance_radius": 1000,
			"initial":          500,
			"minimum":          100,
			"decay":            2,
		},
	})
}

// driveValueDown solves the challenge `n` times with fresh users and returns
// the resulting (decreased) challenge value.
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
		t.Fatalf("expected value to drop below initial 500, got %d after %d solves", v, n)
	}
	return v
}

// requireValueDropsOnNextSolve asserts that solving once more (with a fresh
// user numbered `nextUser`) further reduces the challenge value. This is
// the behaviour the bug breaks — when initial/minimum/decay got wiped,
// `calculate_value` short-circuited via `return challenge.value` and the
// score froze at whatever it dropped to.
func requireValueDropsOnNextSolve(t *testing.T, admin *testutil.Client, ns string, challengeID, before, nextUser int) {
	t.Helper()
	u := testutil.CreateUser(t, admin, ns, nextUser)
	uc := testutil.UserClient(t, u.Name, u.Password)
	if r := submitCoords(t, uc, challengeID, tokyoLat, tokyoLon); r.Status != "correct" {
		t.Fatalf("solve #%d: expected correct, got %s (%s)", nextUser, r.Status, r.Message)
	}
	after := readValue(t, admin, challengeID)
	if after >= before {
		t.Errorf("dynamic scoring appears frozen: value was %d, stayed at %d after another solve", before, after)
	}
}

func patchChallenge(t *testing.T, admin *testutil.Client, id int, body map[string]any) (*http.Response, error) {
	t.Helper()
	return admin.DoJSON(http.MethodPatch, fmt.Sprintf("/api/v1/challenges/%d", id), body, nil)
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
