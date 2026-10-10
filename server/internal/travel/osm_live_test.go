package travel

import (
	"context"
	"os"
	"testing"
)

func TestOSMLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip("set LIVE=1 to hit Overpass")
	}
	p := NewPlaces("").EnableOSM(nil)
	got := p.Nearby(context.Background(), LatLng{Lat: 38.9717, Lng: -95.2353}, "food", 5)
	if len(got) == 0 {
		t.Fatal("no restaurants returned")
	}
	for _, g := range got {
		t.Logf("%s | %s | %.4f,%.4f | $%.0f | %s", g.Name, g.Source, g.Location.Lat, g.Location.Lng, g.PriceUSD, g.Note)
		if g.Source != "osm" {
			t.Fatalf("unexpected source %s", g.Source)
		}
	}
}
