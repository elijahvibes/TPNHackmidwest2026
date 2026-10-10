package travel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testSerpAPI(t *testing.T, handler http.HandlerFunc) *SerpAPI {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := NewSerpAPI("test-secret")
	client.baseURL = server.URL
	client.http = server.Client()
	return client
}

func TestSerpAPIFlights(t *testing.T) {
	client := testSerpAPI(t, func(w http.ResponseWriter, r *http.Request) {
		for key, want := range map[string]string{
			"api_key": "test-secret", "engine": "google_flights", "type": "1",
			"departure_id": "MCI", "arrival_id": "ORD", "adults": "2",
			"outbound_date": "2026-11-10", "return_date": "2026-11-13", "currency": "USD",
		} {
			if got := r.URL.Query().Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
		fmt.Fprint(w, `{
			"search_metadata":{"status":"Success"},
			"best_flights":[{"price":800,"total_duration":150,"flights":[{"airline":"United"}]}],
			"other_flights":[
				{"price":900,"total_duration":240,"flights":[{"airline":"Delta"},{"airline":"Delta"},{"airline":"United"}]},
				{"price":0,"total_duration":120,"flights":[{"airline":"Unknown"}]},
				{"price":600,"total_duration":120,"flights":[]}
			]
		}`)
	})
	quotes, err := client.Flights(context.Background(), "MCI", "ORD", "2026-11-10", "2026-11-13", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(quotes) != 2 {
		t.Fatalf("quotes = %d, want 2 usable flights", len(quotes))
	}
	if quotes[0].PriceUSD != 400 || quotes[0].Source != "serpapi" || quotes[0].Return != "2026-11-13" {
		t.Errorf("round-trip per-person quote = %+v", quotes[0])
	}
	if quotes[1].Carrier != "Delta / United" || quotes[1].Stops != 2 || quotes[1].DurationMin != 240 {
		t.Errorf("connecting quote = %+v", quotes[1])
	}
}

func TestSerpAPIHotels(t *testing.T) {
	client := testSerpAPI(t, func(w http.ResponseWriter, r *http.Request) {
		for key, want := range map[string]string{
			"engine": "google_hotels", "q": "Chicago hotels", "adults": "2", "currency": "USD",
			"check_in_date": "2026-11-10", "check_out_date": "2026-11-13",
		} {
			if got := r.URL.Query().Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
		fmt.Fprint(w, `{
			"search_metadata":{"status":"Success"},
			"properties":[
				{"type":"hotel","name":"Real hotel","overall_rating":4.5,
				 "rate_per_night":{"extracted_lowest":100},
				 "total_rate":{"extracted_lowest":360,"extracted_before_taxes_fees":300}},
				{"type":"vacation rental","name":"Real rental","total_rate":{"extracted_lowest":450}},
				{"name":"Nightly only","rate_per_night":{"extracted_lowest":80}},
				{"name":"Pre-tax only","total_rate":{"extracted_before_taxes_fees":240}},
				{"name":"No price"}
			]
		}`)
	})
	quotes, err := client.Hotels(context.Background(), "Chicago", "2026-11-10", "2026-11-13", 3, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(quotes) != 2 {
		t.Fatalf("quotes = %d, want 2 full-stay prices", len(quotes))
	}
	if quotes[0].TotalUSD != 360 || quotes[0].NightlyUSD != 120 || quotes[0].Rating != 4.5 || quotes[0].Source != "serpapi" {
		t.Errorf("hotel quote = %+v", quotes[0])
	}
	if quotes[1].Kind != "rental" || quotes[1].Nights != 3 {
		t.Errorf("rental quote = %+v", quotes[1])
	}
}

func TestSerpAPIFailuresNeverReturnMocks(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"invalid key", 401, `{"error":"Invalid API key"}`},
		{"quota exceeded", 429, `{"error":"Search quota exhausted"}`},
		{"server failure", 500, `{}`},
		{"provider error", 200, `{"error":"Bad request test-secret"}`},
		{"malformed JSON", 200, `not json`},
		{"incomplete search", 200, `{"search_metadata":{"status":"Processing"}}`},
		{"no offers", 200, `{"search_metadata":{"status":"Success"}}`},
		{"no usable prices", 200, `{"search_metadata":{"status":"Success"},"best_flights":[{"price":0}],"properties":[{"name":"No price"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := testSerpAPI(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			})
			flights, err := client.Flights(context.Background(), "MCI", "ORD", "2026-11-10", "2026-11-13", 2)
			assertProviderError(t, flights == nil, err)
			hotels, err := client.Hotels(context.Background(), "Chicago", "2026-11-10", "2026-11-13", 3, 2)
			assertProviderError(t, hotels == nil, err)
		})
	}
}

func assertProviderError(t *testing.T, noQuotes bool, err error) {
	t.Helper()
	if !noQuotes || !errors.Is(err, ErrProvider) {
		t.Fatalf("expected provider error and no quotes, got %v", err)
	}
	if strings.Contains(err.Error(), "test-secret") {
		t.Fatal("error leaked API key")
	}
}

func TestSerpAPICanceledRequestDoesNotLeakKey(t *testing.T) {
	client := testSerpAPI(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("canceled request should not reach provider")
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	quotes, err := client.Flights(ctx, "MCI", "ORD", "2026-11-10", "2026-11-13", 1)
	assertProviderError(t, quotes == nil, err)
}

func TestSerpAPIRequiresKey(t *testing.T) {
	for _, key := range []string{"", "   "} {
		client := testSerpAPI(t, func(w http.ResponseWriter, r *http.Request) {
			t.Error("missing credentials must not send a request")
		})
		client.key = strings.TrimSpace(key)
		flights, err := client.Flights(context.Background(), "MCI", "ORD", "2026-11-10", "2026-11-13", 2)
		assertProviderError(t, flights == nil, err)
		if !strings.Contains(err.Error(), "SERPAPI_KEY") {
			t.Fatalf("missing configuration error = %v", err)
		}
		hotels, err := client.Hotels(context.Background(), "Chicago", "2026-11-10", "2026-11-13", 3, 2)
		assertProviderError(t, hotels == nil, err)
	}
}

func TestSerpAPIInvalidInputs(t *testing.T) {
	live := NewSerpAPI("test-secret")
	if _, err := live.Flights(context.Background(), "", "ORD", "2026-11-10", "2026-11-13", 1); err == nil {
		t.Error("expected unsupported airport error")
	}
	if _, err := live.Flights(context.Background(), "MCI", "ORD", "2026-11-10", "2026-11-13", 0); err == nil {
		t.Error("expected invalid travelers error")
	}
	if _, err := live.Hotels(context.Background(), "Chicago", "2026-11-10", "2026-11-13", 0, 1); err == nil {
		t.Error("expected invalid nights error")
	}
}
