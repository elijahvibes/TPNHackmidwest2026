package travel

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Geo handles geocoding and driving routes. Mapbox when MAPBOX_TOKEN is set,
// otherwise a small built-in gazetteer plus great-circle math.
type Geo struct {
	token string
	http  *http.Client
}

func NewGeo(token string) *Geo {
	return &Geo{token: token, http: &http.Client{Timeout: 15 * time.Second}}
}

var gazetteer = map[string]LatLng{
	"kansas city":   {39.0997, -94.5786},
	"chicago":       {41.8781, -87.6298},
	"denver":        {39.7392, -104.9903},
	"st. louis":     {38.6270, -90.1994},
	"st louis":      {38.6270, -90.1994},
	"nashville":     {36.1627, -86.7816},
	"minneapolis":   {44.9778, -93.2650},
	"omaha":         {41.2565, -95.9345},
	"dallas":        {32.7767, -96.7970},
	"austin":        {30.2672, -97.7431},
	"new orleans":   {29.9511, -90.0715},
	"las vegas":     {36.1699, -115.1398},
	"los angeles":   {34.0522, -118.2437},
	"new york":      {40.7128, -74.0060},
	"miami":         {25.7617, -80.1918},
	"orlando":       {28.5383, -81.3792},
	"seattle":       {47.6062, -122.3321},
	"san francisco": {37.7749, -122.4194},
	"phoenix":       {33.4484, -112.0740},
	"atlanta":       {33.7490, -84.3880},
	"yellowstone":   {44.4280, -110.5885},
	"grand canyon":  {36.1069, -112.1129},
}

func (g *Geo) Geocode(ctx context.Context, place string) (LatLng, error) {
	key := strings.ToLower(strings.TrimSpace(place))
	for name, ll := range gazetteer {
		if strings.Contains(key, name) {
			return ll, nil
		}
	}
	if g.token == "" {
		return LatLng{}, fmt.Errorf("unknown place %q (set MAPBOX_TOKEN for live geocoding)", place)
	}
	u := fmt.Sprintf("https://api.mapbox.com/geocoding/v5/mapbox.places/%s.json?limit=1&access_token=%s",
		url.PathEscape(place), url.QueryEscape(g.token))
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	res, err := g.http.Do(req)
	if err != nil {
		return LatLng{}, err
	}
	defer res.Body.Close()
	var out struct {
		Features []struct {
			Center []float64 `json:"center"`
		} `json:"features"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil || len(out.Features) == 0 {
		return LatLng{}, fmt.Errorf("geocode %q: no result", place)
	}
	c := out.Features[0].Center
	return LatLng{Lat: c[1], Lng: c[0]}, nil
}

// Route returns driving distance (miles), duration (minutes), and the data source.
func (g *Geo) Route(ctx context.Context, from, to LatLng) (miles float64, minutes int, source string) {
	if g.token != "" {
		u := fmt.Sprintf("https://api.mapbox.com/directions/v5/mapbox/driving/%f,%f;%f,%f?overview=false&access_token=%s",
			from.Lng, from.Lat, to.Lng, to.Lat, url.QueryEscape(g.token))
		req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
		if res, err := g.http.Do(req); err == nil {
			defer res.Body.Close()
			var out struct {
				Routes []struct {
					Distance float64 `json:"distance"` // meters
					Duration float64 `json:"duration"` // seconds
				} `json:"routes"`
			}
			if json.NewDecoder(res.Body).Decode(&out) == nil && len(out.Routes) > 0 {
				return out.Routes[0].Distance / 1609.34, int(out.Routes[0].Duration / 60), "mapbox"
			}
		}
	}
	// Great-circle distance with a road-winding factor; 62 mph average.
	d := haversineMiles(from, to) * 1.18
	return d, int(d / 62 * 60), "estimate"
}

func haversineMiles(a, b LatLng) float64 {
	const r = 3958.8
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLng := (b.Lng - a.Lng) * math.Pi / 180
	la1, la2 := a.Lat*math.Pi/180, b.Lat*math.Pi/180
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(la1)*math.Cos(la2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * r * math.Asin(math.Sqrt(h))
}

// Interpolate returns n evenly spaced points between a and b (exclusive),
// used to look up stops along a drive.
func Interpolate(a, b LatLng, n int) []LatLng {
	pts := make([]LatLng, 0, n)
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n+1)
		pts = append(pts, LatLng{Lat: a.Lat + (b.Lat-a.Lat)*t, Lng: a.Lng + (b.Lng-a.Lng)*t})
	}
	return pts
}
