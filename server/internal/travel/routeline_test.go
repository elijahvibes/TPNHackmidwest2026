package travel

import "testing"

func TestRouteLineThinsAndKeepsEnds(t *testing.T) {
	a, b := LatLng{Lat: 39, Lng: -94.6}, LatLng{Lat: 36.2, Lng: -86.8}
	line := make([]LatLng, 0, 1000)
	for i := 0; i < 1000; i++ {
		f := float64(i) / 999
		line = append(line, LatLng{Lat: a.Lat + (b.Lat-a.Lat)*f, Lng: a.Lng + (b.Lng-a.Lng)*f})
	}
	g := &Geo{}
	if got := g.RouteLine(a, b, 400); got != nil {
		t.Fatalf("no cached route: want nil, got %d points", len(got))
	}
	routeMu.Lock()
	routeLines[routeKey(a, b)] = line
	routeMu.Unlock()
	got := g.RouteLine(a, b, 400)
	if len(got) != 400 {
		t.Fatalf("want 400 points, got %d", len(got))
	}
	if got[0] != a || got[len(got)-1] != b {
		t.Fatalf("ends must be kept: %v … %v", got[0], got[len(got)-1])
	}
	if full := g.RouteLine(a, b, 0); len(full) != 1000 {
		t.Fatalf("max<2 means the whole line, got %d", len(full))
	}
}
