package travel

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Geo resolves place names to coordinates and prices driving routes.
//
// Geocoding is layered so any city works, not just the demo list:
//  1. built-in gazetteer (exact spelling, offline, instant)
//  2. Mapbox when MAPBOX_TOKEN is set
//  3. Photon (OpenStreetMap, no key) with Nominatim as a second opinion
//  4. loose gazetteer match as the last resort when the network is down
//
// Results are cached per process so re-plans never hit the network twice.
type Geo struct {
	token string
	http  *http.Client
	cache sync.Map // normalized query → Placemark
}

func NewGeo(token string) *Geo {
	return &Geo{token: token, http: &http.Client{Timeout: 8 * time.Second}}
}

const geoUserAgent = "TPN trip planner (hackmidwest 2026; github.com/elijahvibes/TPNHackmidwest2026)"

// Geocode keeps the old shape for callers that only need a point.
func (g *Geo) Geocode(ctx context.Context, place string) (LatLng, error) {
	p, err := g.Lookup(ctx, place)
	return p.LatLng, err
}

// Lookup resolves free text ("lawrence ks", "Estes Park, Colorado") to a Placemark.
func (g *Geo) Lookup(ctx context.Context, place string) (Placemark, error) {
	q := strings.TrimSpace(place)
	if q == "" {
		return Placemark{}, fmt.Errorf("empty place")
	}
	if p, ok := lookupGazetteer(q); ok {
		return p, nil
	}
	if v, ok := g.cache.Load(normalize(q)); ok {
		return v.(Placemark), nil
	}
	var p Placemark
	var ok bool
	if g.token != "" {
		p, ok = g.mapbox(ctx, q)
	}
	if !ok {
		p, ok = g.photon(ctx, q)
	}
	if !ok {
		p, ok = g.nominatim(ctx, q)
	}
	if !ok {
		p, ok = lookupGazetteerLoose(q)
	}
	if !ok {
		return Placemark{}, fmt.Errorf("could not find %q; try \"City, ST\"", place)
	}
	g.cache.Store(normalize(q), p)
	return p, nil
}

// Suggest powers the autocomplete box: live OSM matches (cities, towns,
// parks), US first, with the gazetteer as an offline fallback.
func (g *Geo) Suggest(ctx context.Context, q string, limit int) []Placemark {
	q = strings.TrimSpace(q)
	if len(q) < 2 {
		return nil
	}
	if limit <= 0 || limit > 10 {
		limit = 6
	}
	out := g.photonSuggest(ctx, q, limit)
	local := suggestGazetteer(q, limit)
	// Merge: gazetteer entries lead when they prefix-match (they are curated), then live hits.
	seen := map[string]bool{}
	merged := make([]Placemark, 0, limit)
	for _, p := range append(local, out...) {
		k := normalize(p.Label)
		if seen[k] {
			continue
		}
		seen[k] = true
		merged = append(merged, p)
		if len(merged) == limit {
			break
		}
	}
	return merged
}

// --- Mapbox ---

func (g *Geo) mapbox(ctx context.Context, place string) (Placemark, bool) {
	u := fmt.Sprintf("https://api.mapbox.com/geocoding/v5/mapbox.places/%s.json?limit=1&types=place,locality,region,poi&access_token=%s",
		url.PathEscape(place), url.QueryEscape(g.token))
	var out struct {
		Features []struct {
			Text    string    `json:"text"`
			Center  []float64 `json:"center"`
			Context []struct {
				ID        string `json:"id"`
				ShortCode string `json:"short_code"`
				Text      string `json:"text"`
			} `json:"context"`
		} `json:"features"`
	}
	if err := g.getJSON(ctx, u, &out); err != nil || len(out.Features) == 0 || len(out.Features[0].Center) < 2 {
		return Placemark{}, false
	}
	f := out.Features[0]
	p := Placemark{Name: f.Text, LatLng: LatLng{Lat: f.Center[1], Lng: f.Center[0]}, Source: "mapbox"}
	for _, c := range f.Context {
		switch {
		case strings.HasPrefix(c.ID, "region"):
			p.Region = strings.ToUpper(strings.TrimPrefix(c.ShortCode, "US-"))
		case strings.HasPrefix(c.ID, "country"):
			p.Country = strings.ToUpper(c.ShortCode)
		}
	}
	p.Label = makeLabel(p)
	return p, true
}

// --- Photon (komoot's OSM geocoder; no key, fair-use) ---

type photonFeature struct {
	Geometry struct {
		Coordinates []float64 `json:"coordinates"`
	} `json:"geometry"`
	Properties struct {
		Name        string `json:"name"`
		City        string `json:"city"`
		State       string `json:"state"`
		Country     string `json:"country"`
		CountryCode string `json:"countrycode"`
		OSMKey      string `json:"osm_key"`
		OSMValue    string `json:"osm_value"`
		Type        string `json:"type"`
	} `json:"properties"`
}

func (g *Geo) photonQuery(ctx context.Context, q string, limit int) []photonFeature {
	v := url.Values{"q": {q}, "limit": {fmt.Sprint(limit)}, "lang": {"en"}}
	// Only places people travel to: settlements and parks. Streets/houses excluded.
	u := "https://photon.komoot.io/api/?" + v.Encode() +
		"&osm_tag=place:city&osm_tag=place:town&osm_tag=place:village&osm_tag=place:hamlet&osm_tag=place:island" +
		"&osm_tag=boundary:national_park&osm_tag=leisure:nature_reserve&osm_tag=boundary:protected_area&osm_tag=place:locality"
	var out struct {
		Features []photonFeature `json:"features"`
	}
	if err := g.getJSON(ctx, u, &out); err != nil {
		return nil
	}
	return out.Features
}

func (g *Geo) photon(ctx context.Context, q string) (Placemark, bool) {
	feats := g.photonQuery(ctx, q, 5)
	if len(feats) == 0 {
		return Placemark{}, false
	}
	// Prefer US results (this is a US road-trip app), then whatever Photon ranked first.
	best := feats[0]
	for _, f := range feats {
		if strings.EqualFold(f.Properties.CountryCode, "US") {
			best = f
			break
		}
	}
	return photonPlacemark(best)
}

func (g *Geo) photonSuggest(ctx context.Context, q string, limit int) []Placemark {
	feats := g.photonQuery(ctx, q, limit*2)
	us, other := []Placemark{}, []Placemark{}
	for _, f := range feats {
		p, ok := photonPlacemark(f)
		if !ok {
			continue
		}
		if p.Country == "US" {
			us = append(us, p)
		} else {
			other = append(other, p)
		}
	}
	return append(us, other...)
}

func photonPlacemark(f photonFeature) (Placemark, bool) {
	if len(f.Geometry.Coordinates) < 2 || f.Properties.Name == "" {
		return Placemark{}, false
	}
	p := Placemark{
		Name:    f.Properties.Name,
		Country: strings.ToUpper(f.Properties.CountryCode),
		LatLng:  LatLng{Lat: f.Geometry.Coordinates[1], Lng: f.Geometry.Coordinates[0]},
		Source:  "photon",
		Kind:    placeKind(f.Properties.OSMKey, f.Properties.OSMValue),
	}
	if p.Country == "US" {
		p.Region = StateCode(f.Properties.State)
	} else {
		p.Region = f.Properties.State
		if f.Properties.Country != "" {
			p.Country = f.Properties.Country // full name reads better for non-US
		}
	}
	p.Label = makeLabel(p)
	return p, true
}

// --- Nominatim (OSM; strict usage policy, so it is only a fallback) ---

func (g *Geo) nominatim(ctx context.Context, q string) (Placemark, bool) {
	v := url.Values{"q": {q}, "format": {"jsonv2"}, "limit": {"1"}, "addressdetails": {"1"}, "accept-language": {"en"}}
	var out []struct {
		Lat     string `json:"lat"`
		Lon     string `json:"lon"`
		Name    string `json:"name"`
		Address struct {
			State       string `json:"state"`
			CountryCode string `json:"country_code"`
			Country     string `json:"country"`
		} `json:"address"`
	}
	if err := g.getJSON(ctx, "https://nominatim.openstreetmap.org/search?"+v.Encode(), &out); err != nil || len(out) == 0 {
		return Placemark{}, false
	}
	r := out[0]
	var lat, lng float64
	fmt.Sscanf(r.Lat, "%f", &lat)
	fmt.Sscanf(r.Lon, "%f", &lng)
	if lat == 0 && lng == 0 {
		return Placemark{}, false
	}
	p := Placemark{Name: r.Name, Country: strings.ToUpper(r.Address.CountryCode), LatLng: LatLng{Lat: lat, Lng: lng}, Source: "nominatim"}
	if p.Country == "US" {
		p.Region = StateCode(r.Address.State)
	} else {
		p.Region, p.Country = r.Address.State, r.Address.Country
	}
	p.Label = makeLabel(p)
	return p, true
}

func makeLabel(p Placemark) string {
	parts := []string{p.Name}
	if p.Region != "" && p.Region != p.Name {
		parts = append(parts, p.Region)
	}
	if p.Country != "" && p.Country != "US" {
		parts = append(parts, p.Country)
	}
	return strings.Join(parts, ", ")
}

func (g *Geo) getJSON(ctx context.Context, u string, v any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", geoUserAgent)
	req.Header.Set("Accept", "application/json")
	res, err := g.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("geocoder %d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(v)
}

// Route returns driving distance (miles), duration (minutes), and the data source.
func (g *Geo) Route(ctx context.Context, from, to LatLng) (miles float64, minutes int, source string) {
	if g.token != "" {
		u := fmt.Sprintf("https://api.mapbox.com/directions/v5/mapbox/driving/%f,%f;%f,%f?overview=false&access_token=%s",
			from.Lng, from.Lat, to.Lng, to.Lat, url.QueryEscape(g.token))
		var out struct {
			Routes []struct {
				Distance float64 `json:"distance"` // meters
				Duration float64 `json:"duration"` // seconds
			} `json:"routes"`
		}
		if err := g.getJSON(ctx, u, &out); err == nil && len(out.Routes) > 0 {
			return out.Routes[0].Distance / 1609.34, int(out.Routes[0].Duration / 60), "mapbox"
		}
	}
	// OSRM's public demo server: real road routing, no key, light use only.
	// The simplified geometry comes along so Waypoints can place road stops
	// on the actual highway rather than on a straight line.
	if !Offline {
		u := fmt.Sprintf("https://router.project-osrm.org/route/v1/driving/%f,%f;%f,%f?overview=simplified&geometries=geojson", from.Lng, from.Lat, to.Lng, to.Lat)
		var osrm struct {
			Code   string `json:"code"`
			Routes []struct {
				Distance float64 `json:"distance"`
				Duration float64 `json:"duration"`
				Geometry struct {
					Coordinates [][]float64 `json:"coordinates"` // [lng, lat]
				} `json:"geometry"`
			} `json:"routes"`
		}
		if err := g.getJSON(ctx, u, &osrm); err == nil && osrm.Code == "Ok" && len(osrm.Routes) > 0 && osrm.Routes[0].Distance > 0 {
			r := osrm.Routes[0]
			line := make([]LatLng, 0, len(r.Geometry.Coordinates))
			for _, c := range r.Geometry.Coordinates {
				if len(c) == 2 {
					line = append(line, LatLng{Lat: c[1], Lng: c[0]})
				}
			}
			routeMu.Lock()
			routeLines[routeKey(from, to)] = line
			routeMu.Unlock()
			return r.Distance / 1609.34, int(r.Duration / 60), "osrm"
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

// HaversineMiles is the exported straight-line distance between two points.
func HaversineMiles(a, b LatLng) float64 { return haversineMiles(a, b) }

var (
	routeMu    sync.Mutex
	routeLines = map[string][]LatLng{} // road geometry from the last Route call for a pair
)

func routeKey(a, b LatLng) string {
	return fmt.Sprintf("%.3f,%.3f>%.3f,%.3f", a.Lat, a.Lng, b.Lat, b.Lng)
}

// Waypoints returns n points spread evenly along the driven road between a
// and b (exclusive of the ends), for looking up stops on the way. It uses the
// geometry the last Route call fetched; without one it falls back to a
// straight line.
func (g *Geo) Waypoints(a, b LatLng, n int) []LatLng {
	routeMu.Lock()
	line := routeLines[routeKey(a, b)]
	routeMu.Unlock()
	if pts := PointsAlong(line, n); len(pts) == n {
		return pts
	}
	return Interpolate(a, b, n)
}

// WaypointsAt returns the road point at each fraction (0..1) of the drive
// from a to b, falling back to straight-line interpolation.
func (g *Geo) WaypointsAt(a, b LatLng, fracs []float64) []LatLng {
	routeMu.Lock()
	line := routeLines[routeKey(a, b)]
	routeMu.Unlock()
	if pts := PointsAtFractions(line, fracs); len(pts) == len(fracs) {
		return pts
	}
	out := make([]LatLng, 0, len(fracs))
	for _, t := range fracs {
		out = append(out, LatLng{Lat: a.Lat + (b.Lat-a.Lat)*t, Lng: a.Lng + (b.Lng-a.Lng)*t})
	}
	return out
}

// PointsAlong picks n points at equal distance fractions along a polyline
// (exclusive of its ends). Fewer than two vertices → nil.
func PointsAlong(line []LatLng, n int) []LatLng {
	if n < 1 {
		return nil
	}
	fracs := make([]float64, 0, n)
	for k := 1; k <= n; k++ {
		fracs = append(fracs, float64(k)/float64(n+1))
	}
	return PointsAtFractions(line, fracs)
}

// PointsAtFractions returns the point at each distance fraction of a polyline.
func PointsAtFractions(line []LatLng, fracs []float64) []LatLng {
	if len(line) < 2 || len(fracs) == 0 {
		return nil
	}
	cum := make([]float64, len(line))
	for i := 1; i < len(line); i++ {
		cum[i] = cum[i-1] + haversineMiles(line[i-1], line[i])
	}
	total := cum[len(cum)-1]
	if total == 0 {
		return nil
	}
	out := make([]LatLng, 0, len(fracs))
	seg := 1
	for _, fr := range fracs {
		target := total * math.Max(0, math.Min(1, fr))
		for seg < len(cum)-1 && cum[seg] < target {
			seg++
		}
		p, q := line[seg-1], line[seg]
		span := cum[seg] - cum[seg-1]
		t := 0.0
		if span > 0 {
			t = (target - cum[seg-1]) / span
		}
		out = append(out, LatLng{Lat: p.Lat + (q.Lat-p.Lat)*t, Lng: p.Lng + (q.Lng-p.Lng)*t})
	}
	return out
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

// placeKind collapses OSM tags into the handful of place types the UI knows.
func placeKind(key, value string) string {
	switch key + ":" + value {
	case "place:city":
		return "city"
	case "place:town":
		return "town"
	case "place:village", "place:hamlet":
		return "village"
	case "place:island":
		return "island"
	case "boundary:national_park", "leisure:nature_reserve", "boundary:protected_area":
		return "park"
	case "place:locality":
		return "locality"
	}
	return ""
}
