package travel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

var ErrProvider = errors.New("travel provider unavailable")

type SerpAPI struct {
	key     string
	baseURL string
	http    *http.Client
}

func NewSerpAPI(key string) *SerpAPI {
	return &SerpAPI{
		key: strings.TrimSpace(key), baseURL: "https://serpapi.com/search.json",
		http: &http.Client{Timeout: 45 * time.Second},
	}
}

func (s *SerpAPI) Enabled() bool { return s.key != "" }

type serpFlight struct {
	Flights []struct {
		Airline          string `json:"airline"`
		FlightNumber     string `json:"flight_number"`
		DepartureAirport struct {
			Time string `json:"time"`
		} `json:"departure_airport"`
		ArrivalAirport struct {
			Time string `json:"time"`
		} `json:"arrival_airport"`
	} `json:"flights"`
	Price          float64 `json:"price"`
	TotalDuration  int     `json:"total_duration"`
	DepartureToken string  `json:"departure_token"`
	BookingToken   string  `json:"booking_token"`
}

type serpResponse struct {
	Error          string `json:"error"`
	SearchMetadata struct {
		Status string `json:"status"`
	} `json:"search_metadata"`
	BestFlights    []serpFlight    `json:"best_flights"`
	OtherFlights   []serpFlight    `json:"other_flights"`
	BookingOptions []BookingOption `json:"booking_options"`
	Properties     []struct {
		Type          string  `json:"type"`
		Name          string  `json:"name"`
		OverallRating float64 `json:"overall_rating"`
		TotalRate     struct {
			Lowest float64 `json:"extracted_lowest"`
		} `json:"total_rate"`
	} `json:"properties"`
}

func (s *SerpAPI) search(ctx context.Context, q url.Values) (serpResponse, error) {
	var out serpResponse
	if !s.Enabled() {
		return out, fmt.Errorf("%w: SERPAPI_KEY is required for flight and hotel searches", ErrProvider)
	}
	q.Set("api_key", s.key)
	q.Set("currency", "USD")
	q.Set("hl", "en")
	q.Set("gl", "us")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"?"+q.Encode(), nil)
	if err != nil {
		return out, fmt.Errorf("%w: could not construct SerpApi request", ErrProvider)
	}
	res, err := s.http.Do(req)
	if err != nil {
		// net/http errors include the request URL, which contains the API key.
		return out, fmt.Errorf("%w: SerpApi request failed (check connectivity or timeout)", ErrProvider)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return out, fmt.Errorf("%w: SerpApi HTTP %d (check API key and search quota)", ErrProvider, res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return out, fmt.Errorf("%w: invalid SerpApi JSON response", ErrProvider)
	}
	if out.Error != "" {
		message := strings.ReplaceAll(out.Error, s.key, "[redacted]")
		return out, fmt.Errorf("%w: SerpApi: %s", ErrProvider, message)
	}
	if out.SearchMetadata.Status != "Success" {
		return out, fmt.Errorf("%w: SerpApi search did not complete successfully", ErrProvider)
	}
	return out, nil
}

func (s *SerpAPI) Flights(ctx context.Context, origin, dest, depart, ret string, adults int) ([]FlightQuote, error) {
	if adults < 1 {
		return nil, fmt.Errorf("travelers must be positive")
	}
	if origin == "" || dest == "" {
		return nil, fmt.Errorf("no airport match for origin or destination; use a supported city")
	}
	return s.flightQuotes(ctx, FlightSearch{Origin: origin, Destination: dest, Depart: depart, Return: ret, Travelers: adults})
}

func flightQuery(in FlightSearch) (url.Values, error) {
	if in.Origin == "" || in.Destination == "" || in.Travelers < 1 || in.Travelers > 9 {
		return nil, fmt.Errorf("valid airports and 1-9 adult travelers are required")
	}
	start, err := time.Parse("2006-01-02", in.Depart)
	end, endErr := time.Parse("2006-01-02", in.Return)
	if err != nil || endErr != nil || !end.After(start) {
		return nil, fmt.Errorf("valid departure and return dates are required")
	}
	q := url.Values{
		"engine": {"google_flights"}, "type": {"1"},
		"departure_id": {in.Origin}, "arrival_id": {in.Destination},
		"outbound_date": {in.Depart}, "return_date": {in.Return},
		"adults": {strconv.Itoa(in.Travelers)},
	}
	return q, nil
}

func (s *SerpAPI) ReturnFlights(ctx context.Context, in FlightSearch) ([]FlightQuote, error) {
	if in.DepartureToken == "" {
		return nil, fmt.Errorf("departure_token is required; re-plan if this flight has expired")
	}
	return s.flightQuotes(ctx, in)
}

func (s *SerpAPI) flightQuotes(ctx context.Context, in FlightSearch) ([]FlightQuote, error) {
	q, err := flightQuery(in)
	if err != nil {
		return nil, err
	}
	if in.DepartureToken != "" {
		q.Set("departure_token", in.DepartureToken)
	}
	out, err := s.search(ctx, q)
	if err != nil {
		return nil, err
	}
	quotes := []FlightQuote{}
	for _, f := range append(out.BestFlights, out.OtherFlights...) {
		if f.Price <= 0 || f.TotalDuration <= 0 || len(f.Flights) == 0 || f.Flights[0].Airline == "" {
			continue
		}
		carriers := []string{}
		numbers := []string{}
		for _, segment := range f.Flights {
			if segment.Airline != "" && !slices.Contains(carriers, segment.Airline) {
				carriers = append(carriers, segment.Airline)
			}
			if segment.FlightNumber != "" {
				numbers = append(numbers, segment.FlightNumber)
			}
		}
		origin, dest, depart := in.Origin, in.Destination, in.Depart
		if in.DepartureToken != "" {
			origin, dest = dest, origin
			depart = in.Return
		}
		quotes = append(quotes, FlightQuote{
			Carrier: strings.Join(carriers, " / "), Origin: origin, Destination: dest,
			Depart: depart, Return: in.Return,
			// Google Flights prices cover the requested passenger group and round trip.
			PriceUSD: round2(f.Price / float64(in.Travelers)),
			Stops:    len(f.Flights) - 1, DurationMin: f.TotalDuration, Source: "serpapi",
			DepartureTime: f.Flights[0].DepartureAirport.Time,
			ArrivalTime:   f.Flights[len(f.Flights)-1].ArrivalAirport.Time,
			FlightNumbers: numbers, DepartureToken: f.DepartureToken, BookingToken: f.BookingToken,
		})
	}
	if len(quotes) == 0 {
		return nil, fmt.Errorf("%w: SerpApi returned no priced flights for this route and dates", ErrProvider)
	}
	return quotes, nil
}

func (s *SerpAPI) BookingOptions(ctx context.Context, in FlightSearch) ([]BookingOption, error) {
	if in.BookingToken == "" {
		return nil, fmt.Errorf("booking_token is required; choose a return flight first")
	}
	q, err := flightQuery(in)
	if err != nil {
		return nil, err
	}
	q.Set("booking_token", in.BookingToken)
	out, err := s.search(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(out.BookingOptions) == 0 {
		return nil, fmt.Errorf("%w: no booking options available; try another flight or re-plan", ErrProvider)
	}
	return out.BookingOptions, nil
}

// FlightValueCost trades $25 per travel hour and $50 per connection against the fare.
func FlightValueCost(f FlightQuote) float64 {
	return f.PriceUSD + float64(f.DurationMin)*25/60 + float64(f.Stops)*50
}

func RankFlights(flights []FlightQuote) {
	slices.SortStableFunc(flights, func(a, b FlightQuote) int {
		if FlightValueCost(a) < FlightValueCost(b) {
			return -1
		}
		if FlightValueCost(a) > FlightValueCost(b) {
			return 1
		}
		if a.PriceUSD < b.PriceUSD {
			return -1
		}
		if a.PriceUSD > b.PriceUSD {
			return 1
		}
		return 0
	})
}

func (s *SerpAPI) Hotels(ctx context.Context, dest, checkIn, checkOut string, nights, adults int) ([]HotelQuote, error) {
	if nights < 1 || adults < 1 {
		return nil, fmt.Errorf("nights and travelers must be positive")
	}
	out, err := s.search(ctx, url.Values{
		"engine": {"google_hotels"}, "q": {dest + " hotels"},
		"check_in_date": {checkIn}, "check_out_date": {checkOut},
		"adults": {strconv.Itoa(adults)},
	})
	if err != nil {
		return nil, err
	}
	quotes := []HotelQuote{}
	for _, h := range out.Properties {
		// Use the complete stay rate, not a nightly or explicitly pre-tax price.
		if h.Name == "" || h.TotalRate.Lowest <= 0 {
			continue
		}
		kind := "hotel"
		if h.Type == "vacation rental" {
			kind = "rental"
		}
		quotes = append(quotes, HotelQuote{
			Name: h.Name, Rating: h.OverallRating, Nights: nights,
			TotalUSD: round2(h.TotalRate.Lowest), NightlyUSD: round2(h.TotalRate.Lowest / float64(nights)),
			Kind: kind, Source: "serpapi",
		})
	}
	if len(quotes) == 0 {
		return nil, fmt.Errorf("%w: SerpApi returned no priced stays for this destination, dates and guest count", ErrProvider)
	}
	return quotes, nil
}
