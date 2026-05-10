// Package e2e tests geo_challenges.
//
// The plugin defines a "geo" challenge type. Submission body carries
// latitude/longitude pairs instead of a string flag, and the plugin
// computes distance-to-target. A challenge is considered solved when the
// distance is within tolerance_radius (meters).
package e2e

import (
	"net/http"
	"testing"

	"github.com/diver-osint-ctf/ctfd-plugin-e2e/testutil"
)

const tokyoLat, tokyoLon = 35.6586, 139.7454 // Tokyo Tower

func TestGeoChallenges_TypeRegistered(t *testing.T) {
	admin := testutil.AdminClient(t)
	ns := testutil.Namespace(t)

	chal := testutil.CreateChallenge(t, admin, ns, "geo", testutil.ChallengeGeo, testutil.ChallengeOpts{
		Description: "smoke",
		Value:       100,
		Extra: map[string]any{
			"latitude":         tokyoLat,
			"longitude":        tokyoLon,
			"tolerance_radius": 200,
		},
	})
	if chal.ID == 0 {
		t.Fatalf("expected a challenge ID, got 0")
	}
}

// submitCoords posts {challenge_id, latitude, longitude} to the attempt API.
// The geo plugin reads lat/lng directly from request.get_json().
func submitCoords(t *testing.T, c *testutil.Client, challengeID int, lat, lon float64) testutil.SubmitResult {
	t.Helper()
	body := map[string]any{
		"challenge_id": challengeID,
		"latitude":     lat,
		"longitude":    lon,
	}
	var resp struct {
		Success bool                  `json:"success"`
		Data    testutil.SubmitResult `json:"data"`
	}
	r, err := c.PostJSON("/api/v1/challenges/attempt", body, &resp)
	if err != nil {
		t.Fatalf("submit coords: %v", err)
	}
	out := resp.Data
	out.HTTPStatus = r.StatusCode
	return out
}

// TestGeoChallenges_AcceptsCoordinatesWithinRadius — exact-coordinate hit.
func TestGeoChallenges_AcceptsCoordinatesWithinRadius(t *testing.T) {
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

	// 真値そのもの → distance 0m → tolerance 内 → correct
	r := submitCoords(t, uc, chal.ID, tokyoLat, tokyoLon)
	if r.HTTPStatus != http.StatusOK || r.Status != "correct" {
		t.Fatalf("expected correct, got %d/%s (%s)", r.HTTPStatus, r.Status, r.Message)
	}
}

// TestGeoChallenges_RejectsCoordinatesOutOfRadius — distant point should be incorrect.
func TestGeoChallenges_RejectsCoordinatesOutOfRadius(t *testing.T) {
	admin := testutil.AdminClient(t)
	ns := testutil.Namespace(t)

	chal := testutil.CreateChallenge(t, admin, ns, "geo", testutil.ChallengeGeo, testutil.ChallengeOpts{
		Value: 100,
		Extra: map[string]any{
			"latitude":         tokyoLat,
			"longitude":        tokyoLon,
			"tolerance_radius": 100, // 100m
		},
	})

	user := testutil.CreateUser(t, admin, ns, 1)
	uc := testutil.UserClient(t, user.Name, user.Password)

	// 大阪付近 — 約 400km 離れているので 100m 半径外
	r := submitCoords(t, uc, chal.ID, 34.6937, 135.5023)
	if r.HTTPStatus != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", r.HTTPStatus, r.Message)
	}
	if r.Status == "correct" {
		t.Fatalf("expected incorrect for far-away coordinates, got correct")
	}
}
