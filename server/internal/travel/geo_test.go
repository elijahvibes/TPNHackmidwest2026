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

func TestStateCode(t *testing.T) {
	if StateCode("Kansas") != "KS" || StateCode("ks") != "KS" || StateCode("Ontario") != "Ontario" {
		t.Error("StateCode mapping broken")
	}
}
