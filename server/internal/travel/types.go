// Package travel wraps every third-party price/data source behind a small
// interface. Flights and hotels require SerpApi; other providers may use
// labeled mock data or estimates when their API keys are absent.
package travel

type LatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type FlightQuote struct {
	Carrier        string   `json:"carrier"`
	Origin         string   `json:"origin"`
	Destination    string   `json:"destination"`
	Depart         string   `json:"depart"`
	Return         string   `json:"return,omitempty"`
	PriceUSD       float64  `json:"price_usd"` // per traveler, round trip
	Stops          int      `json:"stops"`
	DurationMin    int      `json:"duration_min"`
	Source         string   `json:"source"` // serpapi
	DepartureTime  string   `json:"departure_time,omitempty"`
	ArrivalTime    string   `json:"arrival_time,omitempty"`
	FlightNumbers  []string `json:"flight_numbers,omitempty"`
	DepartureToken string   `json:"departure_token,omitempty"`
	BookingToken   string   `json:"booking_token,omitempty"`
}

type FlightSearch struct {
	Origin         string `json:"origin"`
	Destination    string `json:"destination"`
	Depart         string `json:"depart"`
	Return         string `json:"return"`
	Travelers      int    `json:"travelers"`
	DepartureToken string `json:"departure_token,omitempty"`
	BookingToken   string `json:"booking_token,omitempty"`
}

type BookingAction struct {
	URL      string `json:"url"`
	PostData string `json:"post_data,omitempty"`
}

type BookingSeller struct {
	Name       string        `json:"book_with"`
	Price      float64       `json:"price"`
	Title      string        `json:"option_title"`
	Extensions []string      `json:"extensions"`
	Baggage    []string      `json:"baggage_prices"`
	Action     BookingAction `json:"booking_request"`
	Phone      string        `json:"booking_phone,omitempty"`
}

type BookingOption struct {
	SeparateTickets bool           `json:"separate_tickets"`
	Together        *BookingSeller `json:"together,omitempty"`
	Departing       *BookingSeller `json:"departing,omitempty"`
	Returning       *BookingSeller `json:"returning,omitempty"`
}

type HotelQuote struct {
	Name       string  `json:"name"`
	Rating     float64 `json:"rating"`
	NightlyUSD float64 `json:"nightly_usd"`
	Nights     int     `json:"nights"`
	TotalUSD   float64 `json:"total_usd"`
	Kind       string  `json:"kind"` // hotel | rental
	DistanceKm float64 `json:"distance_km"`
	Source     string  `json:"source"`
}

type DriveEstimate struct {
	DistanceMiles float64 `json:"distance_miles"`
	DurationMin   int     `json:"duration_min"`
	GallonsNeeded float64 `json:"gallons_needed"`
	GasPriceUSD   float64 `json:"gas_price_usd"` // per gallon, regional
	FuelCostUSD   float64 `json:"fuel_cost_usd"` // one way
	RoundTripUSD  float64 `json:"round_trip_usd"`
	VehicleMPG    float64 `json:"vehicle_mpg"`
	Source        string  `json:"source"`
}

type Place struct {
	Name     string  `json:"name"`
	Category string  `json:"category"` // attraction | food | rest_stop | scenic
	Rating   float64 `json:"rating"`
	PriceUSD float64 `json:"price_usd"` // estimated per person, 0 = free
	Location LatLng  `json:"location"`
	Note     string  `json:"note,omitempty"`
	Source   string  `json:"source"`
}
