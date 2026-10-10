package travel

import (
	"context"
	"testing"
)

func TestGazetteerSpellings(t *testing.T) {
	cases := map[string]string{
		"Kansas City":                     "Kansas City, MO",
		"kansas city, mo":                 "Kansas City, MO",
		"Kansas City, KS":                 "Kansas City, KS",
		"St. Louis":                       "St. Louis, MO",
		"st louis missouri":               "St. Louis, MO",
		"Lawrence, Kansas":                "Lawrence, KS",
		"Estes Park, CO, USA":             "Estes Park, CO",
		"Portland":                        "Portland, OR", // first listed wins
		"Portland, Maine":                 "Portland, ME",
		"Rocky Mountain National Park":    "Rocky Mountain National Park, CO",
		"yellowstone national park":       "Yellowstone, WY",
		"Denver, Colorado, United States": "Denver, CO",
	}
	for in, want := range cases {
		p, ok := lookupGazetteer(in)
		if !ok {
			t.Errorf("%q: not found", in)
			continue
		}
		if p.Label != want {
			t.Errorf("%q → %q, want %q", in, p.Label, want)
		}
	}
	if _, ok := lookupGazetteer("Nashville, Indiana"); ok {
		t.Error("exact lookup must not confuse Nashville IN with Nashville TN")
	}
	if p, ok := lookupGazetteerLoose("downtown Kansas City"); !ok || p.Name != "Kansas City" {
		t.Errorf("loose lookup failed: %+v %v", p, ok)
	}
}

func TestSuggestGazetteerOffline(t *testing.T) {
	out := suggestGazetteer("law", 5)
	if len(out) == 0 || out[0].Label != "Lawrence, KS" {
		t.Errorf("suggest 'law' = %+v", out)
	}
	if len(suggestGazetteer("zzzz", 5)) != 0 {
		t.Error("nonsense prefix should suggest nothing")
	}
}

func TestLookupPrefersGazetteerWithoutNetwork(t *testing.T) {
	g := NewGeo("")
	p, err := g.Lookup(context.Background(), "Omaha")
	if err != nil || p.Source != "gazetteer" || p.Region != "NE" {
		t.Errorf("Omaha → %+v, %v", p, err)
	}
}

func TestNearestAirport(t *testing.T) {
	lawrence := LatLng{38.9717, -95.2353}
	a, ok := NearestAirport(lawrence)
	if !ok || a.Code != "MCI" {
		t.Fatalf("Lawrence KS nearest = %+v", a)
	}
	if a.DistanceMiles < 30 || a.DistanceMiles > 60 {
		t.Errorf("MCI should be ~45 mi from Lawrence, got %.0f", a.DistanceMiles)
	}
	estes := LatLng{40.3772, -105.5217}
	if a, _ := NearestAirport(estes); a.Code != "DEN" {
		t.Errorf("Estes Park nearest = %s, want DEN", a.Code)
	}
	if _, ok := NearestAirport(LatLng{0, 0}); ok {
		t.Error("middle of the Atlantic should have no airport")
	}
}

func TestMockHotelsSitOnTheMap(t *testing.T) {
	denver := LatLng{39.7392, -104.9903}
	hs := mockHotels(denver, "Denver, CO", 3, "2026-11-01", "2026-11-04")
	if len(hs) < 4 {
		t.Fatalf("want several mock hotels, got %d", len(hs))
	}
	for _, h := range hs {
		if h.Location.Lat == 0 || h.Location.Lng == 0 {
			t.Errorf("%s has no location", h.Name)
		}
		if d := haversineMiles(denver, h.Location) * 1.609; d > h.DistanceKm*1.5+0.5 {
			t.Errorf("%s drawn %.1f km out but claims %.1f km", h.Name, d, h.DistanceKm)
		}
		if h.Area == "" || h.URL == "" || len(h.Amenities) == 0 {
			t.Errorf("%s missing area/url/amenities: %+v", h.Name, h)
		}
		if h.TotalUSD != round2(h.NightlyUSD*3) {
			t.Errorf("%s total %.2f != 3 × %.2f", h.Name, h.TotalUSD, h.NightlyUSD)
		}
	}
	if hs[0].Name != "The Denver Grand" {
		t.Errorf("mock names should carry the city: %q", hs[0].Name)
	}
}

func TestStateCode(t *testing.T) {
	if StateCode("Kansas") != "KS" || StateCode("ks") != "KS" || StateCode("Ontario") != "Ontario" {
		t.Error("StateCode mapping broken")
	}
}
