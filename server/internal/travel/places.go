package travel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Places finds things to do and stops along a route. Google Places (new API)
// when GOOGLE_PLACES_KEY is set, otherwise curated mock results.
type Places struct {
	key    string
	http   *http.Client
	osm    bool   // real-data-only mode: Google, then OpenStreetMap, never mock
	ranker Ranker // optional AI that picks among real OSM candidates
}

func NewPlaces(key string) *Places {
	return &Places{key: key, http: &http.Client{Timeout: 25 * time.Second}}
}

// Nearby returns up to limit places of the given kind near a point.
// kind: "attraction" | "food" | "rest_stop"
func (p *Places) Nearby(ctx context.Context, at LatLng, kind string, limit int) []Place {
	if p.key != "" {
		if res := p.google(ctx, at, kind, limit); len(res) > 0 {
			return res
		}
	}
	if p.osm {
		return p.openStreetMap(ctx, at, kind, limit) // may be empty; never fabricated
	}
	return mockPlaces(at, kind, limit)
}

var googleTypes = map[string][]string{
	"attraction": {"tourist_attraction", "museum", "park"},
	"food":       {"restaurant", "cafe"},
	"rest_stop":  {"gas_station", "rest_stop"},
	// interest-driven kinds (from the trip form)
	"outdoors":  {"park", "hiking_area", "national_park", "campground"},
	"museums":   {"museum", "art_gallery"},
	"nightlife": {"bar", "night_club", "live_music_venue"},
	"family":    {"amusement_park", "zoo", "aquarium"},
	"history":   {"historical_landmark", "monument"},
}

// Interests lists the kinds a user can pick on the form, in display order.
var Interests = []string{"outdoors", "museums", "food", "nightlife", "family", "history"}

func (p *Places) google(ctx context.Context, at LatLng, kind string, limit int) []Place {
	body := map[string]any{
		"includedTypes":  googleTypes[kind],
		"maxResultCount": limit,
		"locationRestriction": map[string]any{
			"circle": map[string]any{
				"center": map[string]float64{"latitude": at.Lat, "longitude": at.Lng},
				"radius": 20000.0,
			},
		},
		"rankPreference": "POPULARITY",
	}
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://places.googleapis.com/v1/places:searchNearby", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", p.key)
	req.Header.Set("X-Goog-FieldMask", "places.displayName,places.rating,places.priceLevel,places.location")
	res, err := p.http.Do(req)
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	var out struct {
		Places []struct {
			DisplayName struct {
				Text string `json:"text"`
			} `json:"displayName"`
			Rating     float64 `json:"rating"`
			PriceLevel string  `json:"priceLevel"`
			Location   struct {
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
			} `json:"location"`
		} `json:"places"`
	}
	if json.NewDecoder(res.Body).Decode(&out) != nil {
		return nil
	}
	places := make([]Place, 0, len(out.Places))
	for _, g := range out.Places {
		places = append(places, Place{
			Name: g.DisplayName.Text, Category: kind, Rating: g.Rating,
			PriceUSD: priceLevelUSD(g.PriceLevel),
			Location: LatLng{Lat: g.Location.Latitude, Lng: g.Location.Longitude},
			Source:   "google",
		})
	}
	return places
}

func priceLevelUSD(level string) float64 {
	switch level {
	case "PRICE_LEVEL_INEXPENSIVE":
		return 12
	case "PRICE_LEVEL_MODERATE":
		return 25
	case "PRICE_LEVEL_EXPENSIVE":
		return 55
	case "PRICE_LEVEL_VERY_EXPENSIVE":
		return 110
	}
	return 0
}

func mockPlaces(at LatLng, kind string, limit int) []Place {
	s := seed(fmt.Sprintf("%.2f,%.2f", at.Lat, at.Lng), kind)
	pools := map[string][]Place{
		"attraction": {
			{Name: "City Art Museum", Rating: 4.6, PriceUSD: 18, Note: "Free first Fridays"},
			{Name: "Riverfront Park", Rating: 4.7, PriceUSD: 0, Note: "Great sunset walk"},
			{Name: "Historic Old Town Walking Tour", Rating: 4.5, PriceUSD: 15},
			{Name: "Science & Discovery Center", Rating: 4.4, PriceUSD: 22},
			{Name: "Botanical Gardens", Rating: 4.8, PriceUSD: 12},
			{Name: "Observation Deck", Rating: 4.3, PriceUSD: 28},
		},
		"food": {
			{Name: "Smokehouse BBQ", Rating: 4.7, PriceUSD: 22, Note: "Local favorite"},
			{Name: "Corner Taqueria", Rating: 4.6, PriceUSD: 13},
			{Name: "Farm Table Bistro", Rating: 4.5, PriceUSD: 34},
			{Name: "Early Bird Diner", Rating: 4.4, PriceUSD: 14, Note: "Best breakfast value"},
		},
		"rest_stop": {
			{Name: "Travel Plaza & Fuel", Rating: 4.1, PriceUSD: 0, Note: "Cheapest gas on this stretch"},
			{Name: "Scenic Overlook", Rating: 4.6, PriceUSD: 0, Note: "15 min stretch break"},
			{Name: "Roadside Pie Shop", Rating: 4.8, PriceUSD: 9, Note: "Worth the detour"},
			{Name: "State Welcome Center", Rating: 4.0, PriceUSD: 0},
		},
		"outdoors": {
			{Name: "Ridge Trail Loop", Rating: 4.8, PriceUSD: 0, Note: "6 mi, moderate"},
			{Name: "Lakeside State Park", Rating: 4.6, PriceUSD: 8, Note: "Day pass per car"},
			{Name: "Sunrise Kayak Rental", Rating: 4.7, PriceUSD: 35},
		},
		"museums": {
			{Name: "City Art Museum", Rating: 4.6, PriceUSD: 18, Note: "Free first Fridays"},
			{Name: "Natural History Museum", Rating: 4.5, PriceUSD: 20},
			{Name: "Contemporary Gallery District", Rating: 4.4, PriceUSD: 0, Note: "Self-guided"},
		},
		"nightlife": {
			{Name: "Downtown Jazz Lounge", Rating: 4.6, PriceUSD: 15, Note: "No cover before 9"},
			{Name: "Rooftop Bar & Patio", Rating: 4.3, PriceUSD: 25},
			{Name: "Live Music Hall", Rating: 4.7, PriceUSD: 30},
		},
		"family": {
			{Name: "City Zoo", Rating: 4.6, PriceUSD: 22, Note: "Kids under 3 free"},
			{Name: "Children's Science Center", Rating: 4.5, PriceUSD: 16},
			{Name: "Adventure Mini Golf", Rating: 4.3, PriceUSD: 12},
		},
		"history": {
			{Name: "Old Fort Historic Site", Rating: 4.5, PriceUSD: 10},
			{Name: "Heritage Walking Tour", Rating: 4.6, PriceUSD: 15, Note: "90 min, guided"},
			{Name: "Founders' Memorial", Rating: 4.4, PriceUSD: 0},
		},
	}
	pool := pools[kind]
	if pool == nil {
		pool = pools["attraction"]
	}
	out := make([]Place, 0, limit)
	for i := 0; i < limit && i < len(pool); i++ {
		p := pool[(int(s)+i)%len(pool)]
		p.Category = kind
		p.Location = LatLng{Lat: at.Lat + float64(int(s>>uint(i))%20-10)/500, Lng: at.Lng + float64(int(s>>uint(i+1))%20-10)/500}
		p.Source = "mock"
		out = append(out, p)
	}
	return out
}
