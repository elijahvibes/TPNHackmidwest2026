package travel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Geoapify struct {
	key     string
	baseURL string
	http    *http.Client
}

func NewGeoapify(key string) *Geoapify {
	return &Geoapify{key: strings.TrimSpace(key), baseURL: "https://api.geoapify.com/v2/places",
		http: &http.Client{Timeout: 15 * time.Second}}
}

var geoapifyCategories = map[string]string{
	"attraction": "tourism.sights,entertainment.museum,leisure.park",
	"food":       "catering.restaurant",
	"breakfast":  "catering.cafe,commercial.food_and_drink.bakery",
	"rest_stop":  "service.vehicle.fuel",
	"outdoors":   "leisure.park,natural",
	"museums":    "entertainment.museum",
	"nightlife":  "catering.bar,catering.pub",
	"family":     "entertainment.zoo,entertainment.theme_park",
	"history":    "tourism.sights",
}

func (g *Geoapify) Nearby(ctx context.Context, at LatLng, kind string, limit int) ([]Place, error) {
	return g.search(ctx, at, at, kind, limit, 20000)
}

func (g *Geoapify) Explore(ctx context.Context, center LatLng, kind string, limit int, radius float64) ([]Place, error) {
	if !validGeoPoint(center) || math.IsNaN(radius) || math.IsInf(radius, 0) || radius < 1 || radius > 150*1609.34 ||
		limit < 1 || limit > 500 {
		return nil, fmt.Errorf("invalid exploration coordinates, radius or limit")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	points := []LatLng{center}
	for _, bearing := range []float64{0, 90, 180, 270} {
		points = append(points, explorationPoint(center, radius*0.65, bearing))
	}
	lists := make([][]Place, 0, len(points))
	for _, point := range points {
		places, err := g.search(ctx, center, point, kind, limit, radius)
		if err != nil {
			return nil, err
		}
		lists = append(lists, places)
	}
	out := make([]Place, 0, limit)
	seen := map[string]bool{}
	for i := 0; i < limit; i++ {
		for _, list := range lists {
			if i >= len(list) {
				continue
			}
			p := list[i]
			id := fmt.Sprintf("%s:%.5f:%.5f", p.Name, p.Location.Lat, p.Location.Lng)
			if !seen[id] && haversineMiles(center, p.Location)*1609.34 <= radius {
				seen[id] = true
				out = append(out, p)
				if len(out) == limit {
					return out, nil
				}
			}
		}
	}
	return out, nil
}

func (g *Geoapify) search(ctx context.Context, center, bias LatLng, kind string, limit int, radius float64) ([]Place, error) {
	category, ok := geoapifyCategories[kind]
	if !ok || limit < 1 || limit > 500 || !validGeoPoint(center) || !validGeoPoint(bias) ||
		math.IsNaN(radius) || math.IsInf(radius, 0) || radius <= 0 {
		return nil, fmt.Errorf("invalid places category, coordinates, radius or result limit")
	}
	if g.key == "" {
		return nil, fmt.Errorf("%w: GEOAPIFY_KEY is required", ErrProvider)
	}
	q := url.Values{
		"apiKey": {g.key}, "categories": {category}, "limit": {strconv.Itoa(limit)}, "lang": {"en"},
		"filter": {fmt.Sprintf("circle:%f,%f,%.0f", center.Lng, center.Lat, radius)},
		"bias":   {fmt.Sprintf("proximity:%f,%f", bias.Lng, bias.Lat)},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.baseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: could not construct Geoapify request", ErrProvider)
	}
	res, err := g.http.Do(req)
	if err != nil {
		// Transport errors can include the URL containing the API key.
		return nil, fmt.Errorf("%w: Geoapify request failed (connectivity or timeout)", ErrProvider)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: Geoapify HTTP %d (check key and quota)", ErrProvider, res.StatusCode)
	}
	var out struct {
		Type     string `json:"type"`
		Features []struct {
			Properties struct {
				Name       string   `json:"name"`
				Formatted  string   `json:"formatted"`
				Categories []string `json:"categories"`
				Lat        *float64 `json:"lat"`
				Lon        *float64 `json:"lon"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&out); err != nil || out.Type != "FeatureCollection" {
		return nil, fmt.Errorf("%w: invalid Geoapify response", ErrProvider)
	}
	places := make([]Place, 0, len(out.Features))
	for _, f := range out.Features {
		p := f.Properties
		if strings.TrimSpace(p.Name) == "" {
			continue
		}
		if p.Lat == nil || p.Lon == nil || math.IsNaN(*p.Lat) || math.IsNaN(*p.Lon) ||
			math.Abs(*p.Lat) > 90 || math.Abs(*p.Lon) > 180 {
			return nil, fmt.Errorf("%w: invalid Geoapify place coordinates", ErrProvider)
		}
		places = append(places, Place{Name: p.Name, Category: kind, Categories: p.Categories,
			Location: LatLng{Lat: *p.Lat, Lng: *p.Lon}, Source: "geoapify",
			Note: p.Formatted, PriceUnknown: true})
		if len(places) == limit {
			break
		}
	}
	return places, nil
}

func validGeoPoint(p LatLng) bool {
	return !math.IsNaN(p.Lat) && !math.IsNaN(p.Lng) && !math.IsInf(p.Lat, 0) &&
		!math.IsInf(p.Lng, 0) && math.Abs(p.Lat) <= 90 && math.Abs(p.Lng) <= 180
}

func explorationPoint(center LatLng, meters, bearing float64) LatLng {
	lat, lon := center.Lat*math.Pi/180, center.Lng*math.Pi/180
	angle, direction := meters/6371000, bearing*math.Pi/180
	nextLat := math.Asin(math.Sin(lat)*math.Cos(angle) + math.Cos(lat)*math.Sin(angle)*math.Cos(direction))
	nextLon := lon + math.Atan2(math.Sin(direction)*math.Sin(angle)*math.Cos(lat), math.Cos(angle)-math.Sin(lat)*math.Sin(nextLat))
	return LatLng{Lat: nextLat * 180 / math.Pi, Lng: math.Mod(nextLon*180/math.Pi+540, 360) - 180}
}
