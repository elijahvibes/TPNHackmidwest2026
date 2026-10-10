package travel

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestGeoapifyNearby(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("apiKey") != "secret" || q.Get("categories") != geoapifyCategories["food"] ||
			q.Get("filter") != "circle:-87.629800,41.878100,20000" || q.Get("limit") != "4" {
			t.Errorf("incorrect search parameters")
		}
		w.Write([]byte(`{"type":"FeatureCollection","features":[{"properties":{"name":"Real cafe","formatted":"Chicago","categories":["catering","catering.restaurant"],"lat":41.88,"lon":-87.63}},{"properties":{"lat":41.88,"lon":-87.63}}]}`))
	}))
	defer server.Close()
	g := NewGeoapify("secret")
	g.baseURL = server.URL
	places, err := g.Nearby(context.Background(), LatLng{41.8781, -87.6298}, "food", 4)
	if err != nil || len(places) != 1 {
		t.Fatalf("search returned %v, %v", places, err)
	}
	p := places[0]
	if p.Name != "Real cafe" || p.Source != "geoapify" || !p.PriceUnknown || p.Rating != 0 || p.PriceUSD != 0 {
		t.Fatalf("invented or missing place data: %+v", p)
	}
	if len(p.Categories) != 2 || p.Categories[1] != "catering.restaurant" {
		t.Fatalf("provider categories not preserved: %+v", p)
	}
}

func TestGeoapifyResponses(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		body      string
		wantError bool
	}{
		{"empty", 200, `{"type":"FeatureCollection","features":[]}`, false},
		{"unauthorized", 401, `secret`, true},
		{"quota", 429, `secret`, true},
		{"server", 503, `secret`, true},
		{"invalid json", 200, `<html>secret</html>`, true},
		{"wrong shape", 200, `{}`, true},
		{"missing coordinates", 200, `{"type":"FeatureCollection","features":[{"properties":{"name":"Cafe"}}]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}))
			defer s.Close()
			g := NewGeoapify("secret")
			g.baseURL = s.URL
			result, err := g.Nearby(context.Background(), LatLng{}, "food", 4)
			if tc.wantError {
				if !errors.Is(err, ErrProvider) || strings.Contains(err.Error(), "secret") {
					t.Fatalf("expected sanitized provider error: %v", err)
				}
			} else if err != nil || result == nil || len(result) != 0 {
				t.Fatalf("empty results must be an empty array, not mock data: %v, %v", result, err)
			}
		})
	}
}

func TestGeoapifyFailureFallbackIsLabeled(t *testing.T) {
	p := NewPlacesWithGeoapify("", "secret")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	places := p.Nearby(ctx, LatLng{}, "food", 2)
	if len(places) != 2 || places[0].Source != "mock" || !strings.Contains(places[0].Note, "Geoapify unavailable") {
		t.Fatalf("unlabeled fallback: %+v", places)
	}
}

func TestGeoapifyEmptyDoesNotFallback(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"type":"FeatureCollection","features":[]}`))
	}))
	defer s.Close()
	p := NewPlacesWithGeoapify("google-key", "secret")
	p.geoapify.baseURL = s.URL
	if got := p.Nearby(context.Background(), LatLng{}, "food", 2); got == nil || len(got) != 0 {
		t.Fatalf("successful empty search must not use Google or mocks: %+v", got)
	}
}

func TestGeoapifyLive(t *testing.T) {
	if os.Getenv("GEOAPIFY_LIVE_TEST") != "1" {
		t.Skip("set GEOAPIFY_LIVE_TEST=1 and GEOAPIFY_KEY for live verification")
	}
	g := NewGeoapify(os.Getenv("GEOAPIFY_KEY"))
	for kind := range geoapifyCategories {
		places, err := g.Nearby(context.Background(), LatLng{41.8781, -87.6298}, kind, 2)
		if err != nil {
			t.Errorf("%s: %v", kind, err)
			continue
		}
		t.Logf("%s: %d real results", kind, len(places))
	}
}
