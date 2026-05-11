// Regression test for d3d7687 ("feat: improve challenge scoring parameter
// handling"). The bug it fixes: when ctfcli sync re-pushes a geo challenge
// after the CTF, the dynamic-scoring parameters (initial / minimum / decay)
// are re-sent — sometimes as empty strings — and the previous code wrote
// those empty strings into Integer DB columns, killing dynamic scoring
// because `all([challenge.initial, ...])` becomes False thereafter.
package e2e

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/diver-osint-ctf/ctfd-plugin-e2e/testutil"
)

// TestGeo_DynamicScoringSurvivesEmptyParamSync — a dynamic geo challenge
// must keep its scoring parameters even after a CTFd update that sends
// empty-string initial/minimum/decay (ctfcli's `instance config push` /
// `challenge sync` does this). The bug manifests as the value freezing at
// whatever it dropped to: subsequent solves no longer recompute it.
func TestGeo_DynamicScoringSurvivesEmptyParamSync(t *testing.T) {
	admin := testutil.AdminClient(t)
	ns := testutil.Namespace(t)

	chal := testutil.CreateChallenge(t, admin, ns, "geo", testutil.ChallengeGeo, testutil.ChallengeOpts{
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

	// Two solves to drop the value below initial.
	for i := 1; i <= 2; i++ {
		u := testutil.CreateUser(t, admin, ns, i)
		uc := testutil.UserClient(t, u.Name, u.Password)
		r := submitCoords(t, uc, chal.ID, tokyoLat, tokyoLon)
		if r.Status != "correct" {
			t.Fatalf("solve %d: expected correct, got %s (%s)", i, r.Status, r.Message)
		}
	}
	dropped := readValue(t, admin, chal.ID)
	if dropped >= 500 {
		t.Fatalf("expected dynamic value to drop below 500 after 2 solves, got %d", dropped)
	}

	// Simulate ctfcli re-pushing the challenge after the event. ctfcli sends
	// the dynamic-scoring fields back, occasionally as empty strings when
	// the source yaml leaves them blank. Pre-fix, this would clobber the
	// Integer columns and stick the value at `dropped`.
	if _, err := patchChallenge(t, admin, chal.ID, map[string]any{
		"initial": "",
		"minimum": "",
		"decay":   "",
	}); err != nil {
		t.Fatalf("PATCH challenge: %v", err)
	}

	// Confirm the stored params weren't wiped to zero/null. The plugin
	// should have normalised empty strings to None on parse — but the
	// crucial behaviour is that subsequent solves *still* recompute value.
	var detail struct {
		Data struct {
			Initial *int `json:"initial"`
			Minimum *int `json:"minimum"`
			Decay   *int `json:"decay"`
		} `json:"data"`
	}
	if _, err := admin.GetJSON(fmt.Sprintf("/api/v1/challenges/%d", chal.ID), &detail); err != nil {
		t.Fatalf("read challenge after PATCH: %v", err)
	}
	// A reset would surface as initial == 0 (Integer column reading back ""
	// post-coerce) or nil with no scoring; either way the next solve would
	// short-circuit through `return challenge.value`.
	if detail.Data.Initial != nil && *detail.Data.Initial == 0 {
		t.Errorf("initial was reset to 0 after sync; dynamic scoring would break (%+v)", detail.Data)
	}

	// Solve a third time — if dynamic scoring is still wired up, the value
	// keeps falling. If the bug is back, the value is pinned at `dropped`
	// (or equal to it within 1 due to ceil rounding).
	u3 := testutil.CreateUser(t, admin, ns, 3)
	uc3 := testutil.UserClient(t, u3.Name, u3.Password)
	if r := submitCoords(t, uc3, chal.ID, tokyoLat, tokyoLon); r.Status != "correct" {
		t.Fatalf("third solve: expected correct, got %s (%s)", r.Status, r.Message)
	}
	after := readValue(t, admin, chal.ID)
	if after >= dropped {
		t.Errorf("dynamic scoring appears reset: value stayed at >= %d (now %d) after a third solve", dropped, after)
	}
}

// patchChallenge wraps PATCH /api/v1/challenges/<id> for the regression test.
func patchChallenge(t *testing.T, admin *testutil.Client, id int, body map[string]any) (*http.Response, error) {
	t.Helper()
	return admin.DoJSON(http.MethodPatch, fmt.Sprintf("/api/v1/challenges/%d", id), body, nil)
}
