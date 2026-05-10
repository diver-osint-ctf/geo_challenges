// Dynamic-scoring tests for geo_challenges. The plugin reuses CTFd's standard
// (initial, minimum, decay) value formula on solve, so the challenge's stored
// `value` should drop monotonically as more correct solves accumulate.
package e2e

import (
	"testing"

	"github.com/diver-osint-ctf/ctfd-plugin-e2e/testutil"
)

func TestGeo_DynamicScoringDecay(t *testing.T) {
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

	prev := readValue(t, admin, chal.ID)
	for i := 1; i <= 3; i++ {
		u := testutil.CreateUser(t, admin, ns, i)
		uc := testutil.UserClient(t, u.Name, u.Password)
		r := submitCoords(t, uc, chal.ID, tokyoLat, tokyoLon)
		if r.Status != "correct" {
			t.Fatalf("solve %d: expected correct, got %s (%s)", i, r.Status, r.Message)
		}
		// First solve sets value to initial; second/third decay it down.
		cur := readValue(t, admin, chal.ID)
		if i >= 2 && cur >= prev {
			t.Errorf("solve %d: value did not decrease (was %d, now %d)", i, prev, cur)
		}
		prev = cur
	}
	// Value must never go below the configured minimum.
	if prev < 100 {
		t.Errorf("value %d violates minimum=100", prev)
	}
}

func readValue(t *testing.T, admin *testutil.Client, challengeID int) int {
	t.Helper()
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Value int `json:"value"`
		} `json:"data"`
	}
	r, err := admin.GetJSON("/api/v1/challenges/"+itoa(challengeID), &resp)
	if err != nil || r.StatusCode >= 400 {
		t.Fatalf("read challenge value: %v / %v", r, err)
	}
	return resp.Data.Value
}
