package travel

import "testing"

const denverWikitext = `
==See==
{{see
| name=Denver Art Museum | alt= | url=https://www.denverartmuseum.org/ | email=
| address=100 W 14th Ave Pkwy | lat=39.737222 | long=-104.989722 | directions=
| phone=+1 720 865-5000 | hours=Tu-Su 10AM-5PM | price=Adult $27 ($30 weekends and holidays), youth free
| content=A '''huge''' museum with art from [[Native American|all over]] the world. You'll want to give yourself a few hours.
}}
{{see | name=Colorado State Capitol | lat=39.739232 | long=-104.984868 | price=Free | content=A gorgeous domed building {{dead link|date=2024}} at the southern edge of Downtown. }}
{{do | name=Red Rocks Park and Amphitheatre | lat=39.6654 | long=-105.2057 | price= | content=Concerts and hikes among red sandstone monoliths, 15 miles west of the city. }}
{{see | name=No Coordinates Hall | price=$5 | content=skipped }}
{{eat | name=Snooze | lat=39.7580 | long=-104.9920 | price=$12-18 | content=Breakfast with a line out the door. }}
{{listing | type=see | name=Union Station | lat=39.7530 | long=-105.0000 | price=free | content=Restored 1914 rail station. }}
`

func TestParseWikivoyageListings(t *testing.T) {
	see := ParseWikivoyage(denverWikitext, []string{"see", "do"}, "attraction")
	if len(see) != 4 {
		t.Fatalf("want 4 see/do listings, got %d: %+v", len(see), see)
	}
	dam := see[0]
	if dam.Name != "Denver Art Museum" || dam.PriceUSD != 27 || dam.PriceUnknown || dam.Source != "wikivoyage" {
		t.Errorf("DAM: %+v", dam)
	}
	if dam.Note != "A huge museum with art from all over the world. You'll want to give yourself a few hours. · hours Tu-Su 10AM-5PM" {
		t.Errorf("note: %q", dam.Note)
	}
	if see[1].PriceUSD != 0 || see[1].PriceUnknown || see[1].Note != "A gorgeous domed building at the southern edge of Downtown." {
		t.Errorf("capitol: %+v", see[1])
	}
	if !see[2].PriceUnknown || see[2].Categories[0] != "wikivoyage.do" {
		t.Errorf("red rocks: %+v", see[2])
	}
	if see[3].Name != "Union Station" || see[3].PriceUSD != 0 || see[3].PriceUnknown {
		t.Errorf("generic listing: %+v", see[3])
	}
	eat := ParseWikivoyage(denverWikitext, []string{"eat"}, "food")
	if len(eat) != 1 || eat[0].Name != "Snooze" || eat[0].PriceUSD != 12 {
		t.Errorf("eat: %+v", eat)
	}
}

func TestMergeNotableRanksAndDedupes(t *testing.T) {
	cands := []Place{
		{Name: "MacIntosh Park", Source: "geoapify", PriceUnknown: true},
		{Name: "Denver Art Museum", Source: "geoapify", PriceUSD: 18, PriceEstimated: true},
		{Name: "Red Rocks Park", Source: "geoapify", PriceUnknown: true},
	}
	notable := []Place{
		{Name: "Red Rocks Park and Amphitheatre", Source: "wikivoyage", Popularity: 900, Note: "A mountain park.", PriceUnknown: true},
		{Name: "Denver Art Museum", Source: "wikivoyage", Popularity: 48, Note: "An art museum.", PriceUSD: 27},
		{Name: "Denver Botanic Gardens", Source: "wikivoyage", Popularity: 120, Note: "Gardens.", PriceUSD: 15},
	}
	got := MergeNotable(cands, notable)
	if len(got) != 4 {
		t.Fatalf("want 4 places (3 provider + 1 new), got %d: %+v", len(got), got)
	}
	if got[0].Name != "Red Rocks Park" || got[0].Popularity != 900 || got[0].Source != "geoapify" {
		t.Errorf("provider entry should absorb the popularity: %+v", got[0])
	}
	if got[1].Name != "Denver Botanic Gardens" || got[1].Source != "wikivoyage" {
		t.Errorf("missing provider place should be added: %+v", got[1])
	}
	if got[2].Name != "Denver Art Museum" || got[2].PriceUSD != 27 || got[2].PriceEstimated {
		t.Errorf("a published price replaces the estimate: %+v", got[2])
	}
	if got[3].Name != "MacIntosh Park" || got[3].Popularity != 0 {
		t.Errorf("unmatched place sinks to the end: %+v", got[3])
	}
}

func TestParseWikivoyageSkipsTeams(t *testing.T) {
	wt := `{{do | name=Denver Broncos | alt=Empower Field | lat=39.74 | long=-105.02 | content=The city's NFL team plays its home games here. }}
{{do | name=Red Rocks Park and Amphitheatre | lat=39.6654 | long=-105.2057 | content=Concerts and hikes among red sandstone monoliths. }}`
	got := ParseWikivoyage(wt, []string{"do"}, "attraction")
	if len(got) != 1 || got[0].Name != "Red Rocks Park and Amphitheatre" {
		t.Errorf("teams should be skipped: %+v", got)
	}
}
