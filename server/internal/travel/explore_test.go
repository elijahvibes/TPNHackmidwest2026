package travel

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestGeoapifyExplorationDiversifiesWithinRequestedBoundary(t *testing.T) {
	calls := 0
	biases := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		if q.Get("filter") != "circle:0.000000,0.000000,10000" {
			t.Errorf("boundary changed: %s", q.Get("filter"))
		}
		biases[q.Get("bias")] = true
		fmt.Fprintf(w, `{"type":"FeatureCollection","features":[
			{"properties":{"name":"Area %d","lat":0.01,"lon":0.01}},
			{"properties":{"name":"Duplicate","lat":0.02,"lon":0.02}},
			{"properties":{"name":"Outside","lat":10,"lon":10}}]}`, calls)
	}))
	defer server.Close()
	g := NewGeoapify("test")
	g.baseURL = server.URL
	places, err := g.Explore(context.Background(), LatLng{}, "attraction", 12, 10000)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 5 || len(biases) != 5 || len(places) != 6 {
		t.Fatalf("insufficient diversity or duplicates: calls=%d biases=%d results=%d", calls, len(biases), len(places))
	}
	for _, p := range places {
		if p.Name == "Outside" {
			t.Fatal("included a place beyond user range")
		}
	}
}

func TestGeoapifyExplorationStopsOnFailure(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(429)
	}))
	defer s.Close()
	g := NewGeoapify("private-key")
	g.baseURL = s.URL
	places, err := g.Explore(context.Background(), LatLng{}, "food", 6, 10000)
	if err == nil || places != nil || calls != 1 || strings.Contains(err.Error(), "private-key") {
		t.Fatalf("failure swallowed or leaked key: %v %v %d", places, err, calls)
	}
}

func TestExplorationInvalidInputsAndPolarBias(t *testing.T) {
	g := NewGeoapify("test")
	for _, tc := range []struct {
		center LatLng
		radius float64
		limit  int
	}{
		{LatLng{Lat: 91}, 10000, 12}, {LatLng{}, math.NaN(), 12},
		{LatLng{}, 0, 12}, {LatLng{}, 10000, 0},
	} {
		if _, err := g.Explore(context.Background(), tc.center, "food", tc.limit, tc.radius); err == nil {
			t.Fatalf("accepted invalid exploration: %+v", tc)
		}
	}
	for _, center := range []LatLng{{Lat: 89.99, Lng: 179.99}, {Lat: -89.99, Lng: -179.99}} {
		for _, bearing := range []float64{0, 90, 180, 270} {
			if point := explorationPoint(center, 150000, bearing); !validGeoPoint(point) {
				t.Fatalf("invalid polar/date-line bias: %+v", point)
			}
		}
	}
}

func TestExplorationFallbackRespectsRadius(t *testing.T) {
	for _, p := range []*Places{NewPlaces(""), NewPlacesWithGeoapify("", "test")} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		center := LatLng{Lat: 39.74, Lng: -104.99}
		for _, place := range p.Explore(ctx, center, "attraction", 12, 1) {
			if place.Source != "mock" || haversineMiles(center, place.Location) > 1 {
				t.Fatalf("unlabeled or out-of-range fallback: %+v", place)
			}
		}
	}
}

func TestGeoapifyExplorationLive(t *testing.T) {
	if os.Getenv("GEOAPIFY_LIVE_TEST") != "1" {
		t.Skip("set GEOAPIFY_LIVE_TEST=1 and GEOAPIFY_KEY for live verification")
	}
	center := LatLng{Lat: 39.7392, Lng: -104.9903}
	places, err := NewGeoapify(os.Getenv("GEOAPIFY_KEY")).Explore(context.Background(), center, "attraction", 12, 50*1609.34)
	if err != nil {
		t.Fatal(err)
	}
	farthest := 0.0
	for _, p := range places {
		miles := haversineMiles(center, p.Location)
		if p.Source != "geoapify" || miles > 50 {
			t.Fatalf("invalid live result: %+v", p)
		}
		farthest = math.Max(farthest, miles)
	}
	if len(places) < 5 || farthest < 20 {
		t.Fatalf("discovery did not expand: %d results, farthest %.1f miles", len(places), farthest)
	}
	t.Logf("%d real results within 50 miles; farthest %.1f miles", len(places), farthest)
}
