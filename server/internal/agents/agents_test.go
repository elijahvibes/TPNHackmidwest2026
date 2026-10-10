package agents

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"trailmix/internal/travel"
)

// fakeChat replies with a canned JSON object and records the prompt.
type fakeChat struct {
	reply string
	user  string
	err   error
}

func (f *fakeChat) Enabled() bool { return true }
func (f *fakeChat) ChatJSON(_ context.Context, _, user string, out any) error {
	f.user = user
	if f.err != nil {
		return f.err
	}
	return json.Unmarshal([]byte(f.reply), out)
}

type offChat struct{}

func (offChat) Enabled() bool                                        { return false }
func (offChat) ChatJSON(context.Context, string, string, any) error { return nil }

var cands = []travel.Place{
	{Name: "City Art Museum", Categories: []string{"entertainment.museum"}, PriceUSD: 18},
	{Name: "QuikTrip", Categories: []string{"service.vehicle.fuel"}},
	{Name: "Riverfront Park", Categories: []string{"leisure.park"}},
	{Name: "City Art Museum", PriceUSD: 18}, // duplicate
	{Name: "Union Station", Categories: []string{"tourism.sights"}, Note: "hours 09:00-17:00"},
}

func TestCurateKeepsOnlyListedPlaces(t *testing.T) {
	fc := &fakeChat{reply: `{"keep":[{"index":3,"note":"grand hall"},{"index":0,"note":""},{"index":9,"note":"made up"},{"index":3,"note":"dup"}],"dropped":[{"index":1,"reason":"gas station"}]}`}
	got, rep := Curate(context.Background(), fc, Trip{Destination: "Kansas City, MO"}, "attraction", cands, 2)
	if len(got) != 2 || got[0].Name != "Union Station" || got[1].Name != "City Art Museum" {
		t.Fatalf("got %+v", got)
	}
	if got[0].Note != "grand hall" {
		t.Errorf("note not applied: %q", got[0].Note)
	}
	if got[1].Note != "" {
		t.Errorf("empty note should leave original: %q", got[1].Note)
	}
	if rep.Status != "ok" || len(rep.Notes) != 1 || !strings.Contains(rep.Notes[0], "QuikTrip") {
		t.Errorf("report %+v", rep)
	}
	if !strings.Contains(fc.user, "Keep the best 2") || strings.Contains(fc.user, "4. ") {
		t.Errorf("prompt should list the 4 deduped candidates: %s", fc.user)
	}
}

func TestCurateFallsBackWhenModelFails(t *testing.T) {
	fc := &fakeChat{err: context.DeadlineExceeded}
	got, rep := Curate(context.Background(), fc, Trip{}, "food", cands, 3)
	if len(got) != 3 || rep.Status != "failed" {
		t.Fatalf("got %d places, report %+v", len(got), rep)
	}
	got, rep = Curate(context.Background(), offChat{}, Trip{}, "food", cands, 10)
	if len(got) != 4 || rep.Status != "skipped" { // deduped, nothing to filter
		t.Fatalf("offline: got %d places, report %+v", len(got), rep)
	}
}

func TestResearchDropsUnknownPlaceNotes(t *testing.T) {
	fc := &fakeChat{reply: `{"overview":"A river city.","tips":["Go early","","Bring layers"],"place_notes":[{"name":"Union Station","note":"Look up."},{"name":"Fake Tower","note":"nope"}]}`}
	r, rep := DoResearch(context.Background(), fc, Trip{Destination: "Kansas City, MO"}, cands)
	if rep.Status != "ok" || r == nil {
		t.Fatalf("report %+v", rep)
	}
	if len(r.PlaceNotes) != 1 || r.PlaceNotes[0].Name != "Union Station" {
		t.Errorf("place notes %+v", r.PlaceNotes)
	}
	if len(r.Tips) != 2 {
		t.Errorf("tips %+v", r.Tips)
	}
	if !strings.Contains(r.Source, "verify") {
		t.Errorf("research must be labeled as model knowledge: %q", r.Source)
	}
}

func TestAuditFlagsInventedPlacesAndClampsPrices(t *testing.T) {
	items := []Item{
		{Day: 1, Title: "Fly MCI → DEN", Category: "transport", CostUSD: 500},
		{Day: 2, Title: "Dinner at Union Station", Category: "food", CostUSD: 5},
		{Day: 2, Title: "City Art Museum", Category: "activity", CostUSD: 200}, // 2 travelers × $18 = $36
		{Day: 2, Title: "Riverfront Park", Category: "activity", CostUSD: 20},  // free place billed
		{Day: 3, Title: "Secret Rooftop Speakeasy", Category: "activity", CostUSD: 0},
	}
	drop, clamp := Audit(items, Known{Places: cands, Travelers: 2})
	if len(drop) != 1 || drop[4] == "" {
		t.Errorf("drop %+v", drop)
	}
	if clamp[2] != 36 || clamp[3] != 0 {
		t.Errorf("clamp %+v", clamp)
	}
	if _, ok := clamp[1]; ok {
		t.Errorf("under-priced items are fine: %+v", clamp)
	}
}
