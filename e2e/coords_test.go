// Coordinate-handling tests beyond accept/reject smoke: response messages,
// submission record persistence, PATCH-based hot reload of the target
// coordinates, negative-hemisphere coordinates, and the zero-radius edge.
package e2e

import (
	"net/http"
	"strings"
	"testing"

	"github.com/diver-osint-ctf/ctfd-plugin-e2e/testutil"
)

// TestGeo_SubmitResponseMessages — the plugin's attempt() returns three
// distinct messages depending on the path taken. Each one is part of the
// plugin's contract with the UI, so a regression in wording is worth catching.
func TestGeo_SubmitResponseMessages(t *testing.T) {
	admin := testutil.AdminClient(t)
	ns := testutil.Namespace(t)
	chal := testutil.CreateChallenge(t, admin, ns, "geo", testutil.ChallengeGeo, testutil.ChallengeOpts{
		Value: 100,
		Extra: map[string]any{
			"latitude":         tokyoLat,
			"longitude":        tokyoLon,
			"tolerance_radius": 500,
		},
	})
	user := testutil.CreateUser(t, admin, ns, 1)
	uc := testutil.UserClient(t, user.Name, user.Password)

	// Exact hit.
	if r := submitCoords(t, uc, chal.ID, tokyoLat, tokyoLon); !strings.Contains(r.Message, "Correct") {
		t.Errorf("correct submit: expected message to contain 'Correct', got %q", r.Message)
	}

	// Far miss — Osaka, ~400 km away. Need a fresh user because the previous
	// solve marks the challenge as already_solved.
	user2 := testutil.CreateUser(t, admin, ns, 2)
	uc2 := testutil.UserClient(t, user2.Name, user2.Password)
	if r := submitCoords(t, uc2, chal.ID, 34.6937, 135.5023); !strings.Contains(r.Message, "Incorrect") {
		t.Errorf("incorrect submit: expected message to contain 'Incorrect', got %q", r.Message)
	}

	// Garbage payload.
	user3 := testutil.CreateUser(t, admin, ns, 3)
	uc3 := testutil.UserClient(t, user3.Name, user3.Password)
	if r := submitCoordsRaw(t, uc3, chal.ID, "abc", "def"); !strings.Contains(r.Message, "Invalid coordinates") {
		t.Errorf("invalid submit: expected message to contain 'Invalid coordinates', got %q", r.Message)
	}
}

// TestGeo_SubmissionRecordsCoordinates — both solve() and fail() write the
// submitted coordinates into the Submissions table as "lat:X,lon:Y" strings.
// Admins reading the submissions API should be able to see what each user
// actually typed.
func TestGeo_SubmissionRecordsCoordinates(t *testing.T) {
	admin := testutil.AdminClient(t)
	ns := testutil.Namespace(t)
	chal := testutil.CreateChallenge(t, admin, ns, "geo", testutil.ChallengeGeo, testutil.ChallengeOpts{
		Value: 100,
		Extra: map[string]any{
			"latitude":         tokyoLat,
			"longitude":        tokyoLon,
			"tolerance_radius": 500,
		},
	})
	user := testutil.CreateUser(t, admin, ns, 1)
	uc := testutil.UserClient(t, user.Name, user.Password)

	// One fail, then one correct.
	if r := submitCoords(t, uc, chal.ID, 34.6937, 135.5023); r.Status == "correct" {
		t.Fatalf("setup: far point should be incorrect")
	}
	if r := submitCoords(t, uc, chal.ID, tokyoLat, tokyoLon); r.Status != "correct" {
		t.Fatalf("setup: exact point should be correct, got %s (%s)", r.Status, r.Message)
	}

	type subRow struct {
		ID          int    `json:"id"`
		Type        string `json:"type"`
		Provided    string `json:"provided"`
		ChallengeID int    `json:"challenge_id"`
		UserID      int    `json:"user_id"`
	}
	var resp struct {
		Success bool     `json:"success"`
		Data    []subRow `json:"data"`
	}
	r, err := admin.GetJSON("/api/v1/submissions?challenge_id="+itoa(chal.ID), &resp)
	if err != nil || r.StatusCode >= 400 {
		t.Fatalf("list submissions: status=%v err=%v", r.Status, err)
	}

	gotCorrect, gotFail := false, false
	for _, s := range resp.Data {
		if s.ChallengeID != chal.ID || s.UserID != user.ID {
			continue
		}
		if !strings.HasPrefix(s.Provided, "lat:") || !strings.Contains(s.Provided, ",lon:") {
			t.Errorf("submission %d: provided=%q, want lat:X,lon:Y form", s.ID, s.Provided)
		}
		switch s.Type {
		case "correct":
			gotCorrect = true
		case "fail", "incorrect":
			gotFail = true
		}
	}
	if !gotCorrect || !gotFail {
		t.Errorf("expected both a correct and fail submission row, got correct=%t fail=%t", gotCorrect, gotFail)
	}
}

// TestGeo_CoordinatesHotReload — moving the target coordinates via PATCH
// must change what counts as a hit on the very next submission. CTFd's
// BaseChallenge.update setattrs the fields and the plugin's update() method
// honours that, so editing through the admin API has to propagate.
func TestGeo_CoordinatesHotReload(t *testing.T) {
	admin := testutil.AdminClient(t)
	ns := testutil.Namespace(t)

	const (
		osakaLat = 34.6937
		osakaLon = 135.5023
	)

	chal := testutil.CreateChallenge(t, admin, ns, "geo", testutil.ChallengeGeo, testutil.ChallengeOpts{
		Value: 100,
		Extra: map[string]any{
			"latitude":         tokyoLat,
			"longitude":        tokyoLon,
			"tolerance_radius": 500,
		},
	})

	// Move the target to Osaka.
	patch := map[string]any{
		"latitude":         osakaLat,
		"longitude":        osakaLon,
		"tolerance_radius": 500,
	}
	if r, err := admin.DoJSON(http.MethodPatch, "/api/v1/challenges/"+itoa(chal.ID), patch, nil); err != nil || r.StatusCode >= 400 {
		t.Fatalf("patch challenge: status=%v err=%v", r.Status, err)
	}

	user := testutil.CreateUser(t, admin, ns, 1)
	uc := testutil.UserClient(t, user.Name, user.Password)

	// Tokyo coords were correct before the patch; now they must miss.
	if r := submitCoords(t, uc, chal.ID, tokyoLat, tokyoLon); r.Status == "correct" {
		t.Errorf("after retargeting to Osaka, Tokyo coords should miss; got correct (%s)", r.Message)
	}
	// Osaka coords are now the target — should hit.
	if r := submitCoords(t, uc, chal.ID, osakaLat, osakaLon); r.Status != "correct" {
		t.Errorf("after retargeting to Osaka, Osaka coords should hit; got %s (%s)", r.Status, r.Message)
	}
}

// TestGeo_NegativeCoordinates — Haversine works in southern / western
// hemispheres too. Use Sydney, Australia (lat<0, lon>0) and São Paulo (both
// negative) to cover sign-handling.
func TestGeo_NegativeCoordinates(t *testing.T) {
	admin := testutil.AdminClient(t)
	ns := testutil.Namespace(t)

	cases := []struct {
		name string
		lat  float64
		lon  float64
	}{
		{"sydney", -33.8688, 151.2093}, // southern hemisphere, east
		{"saopaulo", -23.5505, -46.6333}, // southern hemisphere, west
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			chal := testutil.CreateChallenge(t, admin, ns+c.name, "geo", testutil.ChallengeGeo, testutil.ChallengeOpts{
				Value: 100,
				Extra: map[string]any{
					"latitude":         c.lat,
					"longitude":        c.lon,
					"tolerance_radius": 200,
				},
			})
			user := testutil.CreateUser(t, admin, ns+c.name, 1)
			uc := testutil.UserClient(t, user.Name, user.Password)

			if r := submitCoords(t, uc, chal.ID, c.lat, c.lon); r.Status != "correct" {
				t.Errorf("exact %s coords should hit, got %s (%s)", c.name, r.Status, r.Message)
			}
		})
	}
}

// TestGeo_ZeroToleranceRequiresExactMatch — tolerance_radius=0 collapses the
// accept region to a single point. Even a tiny offset (1 m) should miss.
func TestGeo_ZeroToleranceRequiresExactMatch(t *testing.T) {
	admin := testutil.AdminClient(t)
	ns := testutil.Namespace(t)
	chal := testutil.CreateChallenge(t, admin, ns, "geo", testutil.ChallengeGeo, testutil.ChallengeOpts{
		Value: 100,
		Extra: map[string]any{
			"latitude":         tokyoLat,
			"longitude":        tokyoLon,
			"tolerance_radius": 0,
		},
	})
	user := testutil.CreateUser(t, admin, ns, 1)
	uc := testutil.UserClient(t, user.Name, user.Password)

	// Exact match (distance == 0) — must be accepted because the comparison is
	// `distance <= tolerance_radius`.
	if r := submitCoords(t, uc, chal.ID, tokyoLat, tokyoLon); r.Status != "correct" {
		t.Errorf("exact match at zero tolerance should be correct, got %s (%s)", r.Status, r.Message)
	}

	// 1 m offset — must miss.
	user2 := testutil.CreateUser(t, admin, ns, 2)
	uc2 := testutil.UserClient(t, user2.Name, user2.Password)
	if r := submitCoords(t, uc2, chal.ID, tokyoLat+metersToLatDeg(1), tokyoLon); r.Status == "correct" {
		t.Errorf("1m offset at zero tolerance should miss, got correct (%s)", r.Message)
	}
}
