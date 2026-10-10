package travel

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Amadeus covers flights and hotels through the free Self-Service test API.
// https://developers.amadeus.com returns cached but realistic prices in its
// test environment, which is ideal for a demo.
type Amadeus struct {
	key, secret string
	http        *http.Client

	mu    sync.Mutex
	token string
	exp   time.Time
}

func NewAmadeus(key, secret string) *Amadeus {
	return &Amadeus{key: key, secret: secret, http: &http.Client{Timeout: 20 * time.Second}}
}

func (a *Amadeus) Enabled() bool { return a.key != "" && a.secret != "" }

const amadeusBase = "https://test.api.amadeus.com"

func (a *Amadeus) auth(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.token != "" && time.Now().Before(a.exp) {
		return a.token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {a.key}, "client_secret": {a.secret}}
	req, _ := http.NewRequestWithContext(ctx, "POST", amadeusBase+"/v1/security/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := a.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil || out.AccessToken == "" {
		return "", fmt.Errorf("amadeus auth failed")
	}
	a.token = out.AccessToken
	a.exp = time.Now().Add(time.Duration(out.ExpiresIn-60) * time.Second)
	return a.token, nil
}

func (a *Amadeus) get(ctx context.Context, path string, q url.Values, v any) error {
	tok, err := a.auth(ctx)
	if err != nil {
		return err
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", amadeusBase+path+"?"+q.Encode(), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	res, err := a.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("amadeus %s: status %d", path, res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(v)
}

// Flights searches round-trip offers between IATA codes. Falls back to mock quotes.
func (a *Amadeus) Flights(ctx context.Context, originIATA, destIATA, depart, ret string, adults int) []FlightQuote {
	if a.Enabled() && originIATA != "" && destIATA != "" {
		q := url.Values{
			"originLocationCode": {originIATA}, "destinationLocationCode": {destIATA},
			"departureDate": {depart}, "adults": {strconv.Itoa(adults)},
			"currencyCode": {"USD"}, "max": {"5"},
		}
		if ret != "" {
			q.Set("returnDate", ret)
		}
		var out struct {
			Data []struct {
				Price struct {
					GrandTotal string `json:"grandTotal"`
				} `json:"price"`
				ValidatingAirlineCodes []string `json:"validatingAirlineCodes"`
				Itineraries            []struct {
					Duration string `json:"duration"`
					Segments []any  `json:"segments"`
				} `json:"itineraries"`
			} `json:"data"`
		}
		if err := a.get(ctx, "/v2/shopping/flight-offers", q, &out); err == nil && len(out.Data) > 0 {
			quotes := make([]FlightQuote, 0, len(out.Data))
			for _, d := range out.Data {
				total, _ := strconv.ParseFloat(d.Price.GrandTotal, 64)
				carrier := "n/a"
				if len(d.ValidatingAirlineCodes) > 0 {
					carrier = d.ValidatingAirlineCodes[0]
				}
				stops, dur := 0, 0
				if len(d.Itineraries) > 0 {
					stops = len(d.Itineraries[0].Segments) - 1
					dur = parseISODurationMin(d.Itineraries[0].Duration)
				}
				quotes = append(quotes, FlightQuote{
					Carrier: carrier, Origin: originIATA, Destination: destIATA,
					Depart: depart, Return: ret, PriceUSD: round2(total / float64(adults)),
					Stops: stops, DurationMin: dur, Source: "amadeus",
				})
			}
			return quotes
		}
	}
	return mockFlights(originIATA, destIATA, depart, ret)
}

// Hotels lists hotel offers near a point (the city the user asked for, not
// its airport). Falls back to mock quotes that are still placed on the map.
func (a *Amadeus) Hotels(ctx context.Context, at LatLng, checkIn, checkOut string, nights int, dest string) []HotelQuote {
	if a.Enabled() {
		var list struct {
			Data []struct {
				HotelID string `json:"hotelId"`
				Name    string `json:"name"`
				GeoCode struct {
					Latitude  float64 `json:"latitude"`
					Longitude float64 `json:"longitude"`
				} `json:"geoCode"`
				Distance struct {
					Value float64 `json:"value"`
					Unit  string  `json:"unit"`
				} `json:"distance"`
			} `json:"data"`
		}
		q := url.Values{"latitude": {fmt.Sprint(at.Lat)}, "longitude": {fmt.Sprint(at.Lng)}, "radius": {"15"}, "radiusUnit": {"KM"}}
		if err := a.get(ctx, "/v1/reference-data/locations/hotels/by-geocode", q, &list); err == nil && len(list.Data) > 0 {
			type meta struct {
				at LatLng
				km float64
			}
			where := map[string]meta{}
			ids := make([]string, 0, 10)
			for i, h := range list.Data {
				if i >= 10 {
					break
				}
				ids = append(ids, h.HotelID)
				km := h.Distance.Value
				if strings.EqualFold(h.Distance.Unit, "MILE") {
					km *= 1.609
				}
				where[h.HotelID] = meta{at: LatLng{Lat: h.GeoCode.Latitude, Lng: h.GeoCode.Longitude}, km: km}
			}
			var offers struct {
				Data []struct {
					Hotel struct {
						HotelID string `json:"hotelId"`
						Name    string `json:"name"`
						Rating  string `json:"rating"`
					} `json:"hotel"`
					Offers []struct {
						Price struct {
							Total string `json:"total"`
						} `json:"price"`
						Room struct {
							TypeEstimated struct {
								Category string `json:"category"`
							} `json:"typeEstimated"`
						} `json:"room"`
					} `json:"offers"`
				} `json:"data"`
			}
			q2 := url.Values{"hotelIds": {strings.Join(ids, ",")}, "checkInDate": {checkIn}, "checkOutDate": {checkOut}, "currency": {"USD"}, "bestRateOnly": {"true"}}
			if err := a.get(ctx, "/v3/shopping/hotel-offers", q2, &offers); err == nil && len(offers.Data) > 0 {
				quotes := []HotelQuote{}
				for _, d := range offers.Data {
					if len(d.Offers) == 0 {
						continue
					}
					total, _ := strconv.ParseFloat(d.Offers[0].Price.Total, 64)
					rating, _ := strconv.ParseFloat(d.Hotel.Rating, 64)
					if rating == 0 {
						rating = 3.8 // Amadeus test data often omits stars
					}
					n := nights
					if n < 1 {
						n = 1
					}
					m := where[d.Hotel.HotelID]
					hq := HotelQuote{
						Name: titleWords(d.Hotel.Name), Rating: rating, Nights: nights,
						TotalUSD: round2(total), NightlyUSD: round2(total / float64(n)),
						Kind: "hotel", DistanceKm: round1(m.km), Location: m.at, Source: "amadeus",
						Area: areaLabel(m.km), URL: hotelSearchURL(d.Hotel.Name, dest, checkIn, checkOut),
					}
					if c := d.Offers[0].Room.TypeEstimated.Category; c != "" {
						hq.Amenities = []string{strings.ToLower(strings.ReplaceAll(c, "_", " "))}
					}
					quotes = append(quotes, hq)
				}
				if len(quotes) > 0 {
					return quotes
				}
			}
		}
	}
	return mockHotels(at, dest, nights, checkIn, checkOut)
}

// hotelSearchURL is a plain web search for the stay; we do not pretend to be
// a booking engine. Google's hotel search understands "<name> <city>" well.
func hotelSearchURL(name, dest, checkIn, checkOut string) string {
	q := url.Values{"q": {name + " " + dest}}
	if checkIn != "" && checkOut != "" {
		q.Set("checkin", checkIn)
		q.Set("checkout", checkOut)
	}
	return "https://www.google.com/travel/hotels?" + q.Encode()
}

func areaLabel(km float64) string {
	switch {
	case km <= 1.5:
		return "downtown"
	case km <= 4:
		return "near downtown"
	case km <= 10:
		return "in town"
	default:
		return "outskirts"
	}
}

// titleWords fixes the SHOUTING names Amadeus returns.
func titleWords(s string) string {
	if s != strings.ToUpper(s) {
		return s
	}
	w := strings.Fields(strings.ToLower(s))
	for i := range w {
		if len(w[i]) > 0 {
			w[i] = strings.ToUpper(w[i][:1]) + w[i][1:]
		}
	}
	return strings.Join(w, " ")
}

func parseISODurationMin(s string) int { // PT5H30M
	s = strings.TrimPrefix(s, "PT")
	h, m := 0, 0
	if i := strings.Index(s, "H"); i >= 0 {
		h, _ = strconv.Atoi(s[:i])
		s = s[i+1:]
	}
	if i := strings.Index(s, "M"); i >= 0 {
		m, _ = strconv.Atoi(s[:i])
	}
	return h*60 + m
}

// --- deterministic mocks (stable per input so demos are repeatable) ---

func seed(parts ...string) uint32 {
	h := fnv.New32a()
	for _, p := range parts {
		h.Write([]byte(strings.ToLower(p)))
	}
	return h.Sum32()
}

func mockFlights(o, d, depart, ret string) []FlightQuote {
	s := seed(o, d, depart)
	base := 140 + float64(s%260)
	carriers := []string{"Southwest", "Delta", "United", "American", "Frontier"}
	out := make([]FlightQuote, 0, 4)
	for i := 0; i < 4; i++ {
		out = append(out, FlightQuote{
			Carrier: carriers[(int(s)+i)%len(carriers)], Origin: o, Destination: d,
			Depart: depart, Return: ret,
			PriceUSD:    round2(base + float64(i)*37 + float64((s>>uint(i))%40)),
			Stops:       i % 2,
			DurationMin: 110 + int(s%90) + i*45,
			Source:      "mock",
		})
	}
	return out
}

func mockHotels(at LatLng, dest string, nights int, checkIn, checkOut string) []HotelQuote {
	s := seed(dest)
	short := shortCity(dest)
	type tmpl struct {
		name, kind, area string
		km               float64
		amen             []string
	}
	ts := []tmpl{
		{"The " + short + " Grand", "hotel", "downtown", 0.6, []string{"rooftop bar", "gym", "valet"}},
		{short + " Riverside Inn", "inn", "near downtown", 2.1, []string{"free breakfast", "free parking"}},
		{"Loft on Main", "rental", "downtown", 1.1, []string{"full kitchen", "sleeps 4", "washer"}},
		{"Summit Suites " + short, "hotel", "in town", 5.4, []string{"pool", "free parking", "suites"}},
		{"Old Town B&B", "inn", "near downtown", 1.8, []string{"breakfast included", "garden"}},
		{short + " Lodge & Cabins", "resort", "outskirts", 9.5, []string{"hot tub", "kitchenette", "free parking"}},
	}
	out := make([]HotelQuote, 0, len(ts))
	for i, t := range ts {
		nightly := 79 + float64((s>>uint(i))%160)
		if t.area == "downtown" {
			nightly += 40
		}
		// Scatter around the city centre at roughly the stated distance.
		ang := float64((s>>uint(i*3))%360) * math.Pi / 180
		dLat := t.km / 111 * math.Cos(ang)
		dLng := t.km / (111 * math.Cos(at.Lat*math.Pi/180)) * math.Sin(ang)
		out = append(out, HotelQuote{
			Name: t.name, Rating: 3.4 + float64((s+uint32(i))%16)/10, NightlyUSD: round2(nightly),
			Nights: nights, TotalUSD: round2(nightly * float64(nights)), Kind: t.kind,
			DistanceKm: t.km, Location: LatLng{Lat: round4(at.Lat + dLat), Lng: round4(at.Lng + dLng)},
			Area: t.area, Amenities: t.amen, URL: hotelSearchURL(t.name, dest, checkIn, checkOut), Source: "mock",
		})
	}
	return out
}

// shortCity turns "Kansas City, MO" into "Kansas City" for hotel names.
func shortCity(dest string) string {
	d := strings.TrimSpace(dest)
	if i := strings.Index(d, ","); i > 0 {
		d = d[:i]
	}
	if i := strings.Index(d, "("); i > 0 {
		d = strings.TrimSpace(d[:i])
	}
	if d == "" {
		return "City"
	}
	return titleWords(strings.ToUpper(d))
}

func round4(v float64) float64 { return math.Round(v*10000) / 10000 }
