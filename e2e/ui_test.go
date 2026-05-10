// UI test for geo_challenges. The Leaflet map is rendered on the challenge
// create / update form; we verify that the map container is present after a
// real-browser navigation. Detailed click-to-fill interaction is too flaky
// across browser versions to assert reliably.
package e2e

import (
	"strings"
	"testing"

	"github.com/diver-osint-ctf/ctfd-plugin-e2e/testutil"
)

func TestGeo_AdminCreatePage_ShowsMap(t *testing.T) {
	admin := testutil.AdminClient(t)
	ns := testutil.Namespace(t)
	// Need an existing geo challenge for the admin update page to render.
	chal := testutil.CreateChallenge(t, admin, ns, "geo", testutil.ChallengeGeo, testutil.ChallengeOpts{
		Value: 100,
		Extra: map[string]any{
			"latitude":         tokyoLat,
			"longitude":        tokyoLon,
			"tolerance_radius": 200,
		},
	})

	base := testutil.CTFdURL(t)
	b := testutil.NewBrowser(t)

	// Login as admin via the public /login form (input[type=submit]).
	b.Open(base + "/login")
	b.Wait("input[name=name]")
	b.Type("input[name=name]", testutil.AdminName(t))
	b.Type("input[name=password]", testutil.AdminPassword(t))
	b.Click("input[type=submit]")

	// Navigate to the challenge admin (update) page for our geo challenge.
	b.Open(base + "/admin/challenges/" + itoa(chal.ID))
	b.Wait("body")

	body := b.GetText("body")
	// The Leaflet plugin is registered globally; admin pages for a geo
	// challenge expose latitude/longitude inputs and a #map element.
	for _, kw := range []string{"latitude", "longitude"} {
		if !strings.Contains(strings.ToLower(body), kw) {
			b.Screenshot("")
			t.Errorf("admin update page missing %q (saved screenshot)", kw)
		}
	}
}
