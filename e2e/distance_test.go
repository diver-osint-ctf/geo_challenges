// Distance / boundary / API-shape tests for geo_challenges beyond the basic
// accept/reject smoke. Coordinate validation is best-effort: the plugin only
// rejects non-numeric inputs (float() ValueError), so out-of-range numerics
// are still attempted via Haversine and merely produce incorrect-far results.
package e2e

import (
	"math"
	"net/http"
	"testing"

	"github.com/diver-osint-ctf/ctfd-plugin-e2e/testutil"
)

// TestGeo_NonNumericCoordinatesIncorrect — the plugin's attempt() returns
// `False, "Invalid coordinates submitted"` on non-numeric inputs, which CTFd
// surfaces as a normal incorrect 200 response.
func TestGeo_NonNumericCoordinatesIncorrect(t *testing.T) {
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

	r := submitCoordsRaw(t, uc, chal.ID, "abc", "def")
	if r.HTTPStatus != http.StatusOK {
		t.Fatalf("expected 200 (plugin returns incorrect), got %d", r.HTTPStatus)
	}
	if r.Status == "correct" {
		t.Fatalf("non-numeric coords must not be accepted as correct")
	}
}

// TestGeo_ToleranceBoundary — a point exactly inside the radius is correct,
// while a point clearly outside the radius is incorrect. Uses a generous
// margin (10× tolerance) on the "outside" case to avoid float-precision flakes.
func TestGeo_ToleranceBoundary(t *testing.T) {
	admin := testutil.AdminClient(t)
	ns := testutil.Namespace(t)
	chal := testutil.CreateChallenge(t, admin, ns, "geo", testutil.ChallengeGeo, testutil.ChallengeOpts{
		Value: 100,
		Extra: map[string]any{
			"latitude":         tokyoLat,
			"longitude":        tokyoLon,
			"tolerance_radius": 100, // 100 m
		},
	})
	user := testutil.CreateUser(t, admin, ns, 1)
	uc := testutil.UserClient(t, user.Name, user.Password)

	// 50 m north — well inside the 100 m circle.
	insideLat := tokyoLat + metersToLatDeg(50)
	r1 := submitCoords(t, uc, chal.ID, insideLat, tokyoLon)
	if r1.Status != "correct" {
		t.Fatalf("50m offset should be correct, got %s (%s)", r1.Status, r1.Message)
	}

	// 1 km north — well outside.
	outsideLat := tokyoLat + metersToLatDeg(1000)
	r2 := submitCoords(t, uc, chal.ID, outsideLat, tokyoLon)
	if r2.Status == "correct" {
		t.Fatalf("1000m offset should be incorrect")
	}
}

// TestGeo_APIResponseIncludesTolerance — the plugin patches the Challenge
// detail API to include tolerance_radius in the response body.
func TestGeo_APIResponseIncludesTolerance(t *testing.T) {
	admin := testutil.AdminClient(t)
	ns := testutil.Namespace(t)
	chal := testutil.CreateChallenge(t, admin, ns, "geo", testutil.ChallengeGeo, testutil.ChallengeOpts{
		Value: 100,
		Extra: map[string]any{
			"latitude":         tokyoLat,
			"longitude":        tokyoLon,
			"tolerance_radius": 250,
		},
	})

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			ToleranceRadius float64 `json:"tolerance_radius"`
		} `json:"data"`
	}
	r, err := admin.GetJSON("/api/v1/challenges/"+itoa(chal.ID), &resp)
	if err != nil || r.StatusCode >= 400 {
		t.Fatalf("get challenge: %v / %v", r, err)
	}
	if math.Abs(resp.Data.ToleranceRadius-250) > 0.01 {
		t.Errorf("expected tolerance_radius=250, got %v", resp.Data.ToleranceRadius)
	}
}

// metersToLatDeg converts meters of latitude offset into degrees. ~111km/deg.
func metersToLatDeg(m float64) float64 {
	return m / 111_000.0
}

// itoa is local because testutil doesn't export one.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// submitCoordsRaw lets us send non-numeric strings.
func submitCoordsRaw(t *testing.T, c *testutil.Client, challengeID int, lat, lon string) testutil.SubmitResult {
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
		t.Fatalf("submit raw coords: %v", err)
	}
	out := resp.Data
	out.HTTPStatus = r.StatusCode
	return out
}
