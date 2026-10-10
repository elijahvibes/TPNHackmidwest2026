// Package travel wraps every third-party price/data source behind a small
// interface. Each provider degrades to realistic mock data when its API key is
// absent, so the planner always produces a priced itinerary.
package travel

type LatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type FlightQuote struct {
	Carrier     string  `json:"carrier"`
	Origin      string  `json:"origin"`
	Destination string  `json:"destination"`
	Depart      string  `json:"depart"`
	Return      string  `json:"return,omitempty"`
	PriceUSD    float64 `json:"price_usd"` // per traveler, round trip
	Stops       int     `json:"stops"`
	DurationMin int     `json:"duration_min"`
	Source      string  `json:"source"` // amadeus | mock
}

type HotelQuote struct {
	Name       string   `json:"name"`
	Rating     float64  `json:"rating"`
	NightlyUSD float64  `json:"nightly_usd"`
	Nights     int      `json:"nights"`
	TotalUSD   float64  `json:"total_usd"`
	Kind       string   `json:"kind"` // hotel | rental | inn | resort
	DistanceKm float64  `json:"distance_km"`
	Location   LatLng   `json:"location"`            // where it sits, for the map
	Area       string   `json:"area,omitempty"`      // "Downtown", "near the airport"
	Amenities  []string `json:"amenities,omitempty"` // short tags: "free breakfast", "pool"
	URL        string   `json:"url,omitempty"`       // where to actually book / look it up
	Source     string   `json:"source"`
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
