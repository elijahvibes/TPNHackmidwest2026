package travel

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
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

// Hotels lists hotel offers near a point. Falls back to mock quotes.
func (a *Amadeus) Hotels(ctx context.Context, at LatLng, checkIn, checkOut string, nights int, dest string) []HotelQuote {
	if a.Enabled() {
		var list struct {
			Data []struct {
				HotelID string `json:"hotelId"`
				Name    string `json:"name"`
			} `json:"data"`
		}
		q := url.Values{"latitude": {fmt.Sprint(at.Lat)}, "longitude": {fmt.Sprint(at.Lng)}, "radius": {"15"}, "radiusUnit": {"KM"}}
		if err := a.get(ctx, "/v1/reference-data/locations/hotels/by-geocode", q, &list); err == nil && len(list.Data) > 0 {
			ids := make([]string, 0, 10)
			for i, h := range list.Data {
				if i >= 10 {
					break
				}
				ids = append(ids, h.HotelID)
			}
			var offers struct {
				Data []struct {
					Hotel struct {
						Name   string `json:"name"`
						Rating string `json:"rating"`
					} `json:"hotel"`
					Offers []struct {
						Price struct {
							Total string `json:"total"`
						} `json:"price"`
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
					n := nights
					if n < 1 {
						n = 1
					}
					quotes = append(quotes, HotelQuote{
						Name: d.Hotel.Name, Rating: rating, Nights: nights,
						TotalUSD: round2(total), NightlyUSD: round2(total / float64(n)),
						Kind: "hotel", Source: "amadeus",
					})
				}
				if len(quotes) > 0 {
					return quotes
				}
			}
		}
	}
	return mockHotels(dest, nights)
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

func mockHotels(dest string, nights int) []HotelQuote {
	s := seed(dest)
	names := []string{"The Marquee Hotel", "Riverside Inn", "Loft & Co. Rental", "Summit Suites", "Old Town B&B"}
	kinds := []string{"hotel", "hotel", "rental", "hotel", "rental"}
	out := make([]HotelQuote, 0, len(names))
	for i, n := range names {
		nightly := 79 + float64((s>>uint(i))%160)
		out = append(out, HotelQuote{
			Name: n, Rating: 3.4 + float64((s+uint32(i))%16)/10, NightlyUSD: round2(nightly),
			Nights: nights, TotalUSD: round2(nightly * float64(nights)), Kind: kinds[i],
			DistanceKm: round1(float64((s>>uint(i+2))%80) / 10), Source: "mock",
		})
	}
	return out
}
