package travel

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func TestReturnFlightsRetainsTokensAndSchedule(t *testing.T) {
	client := testSerpAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("departure_token") != "outbound-token" || r.URL.Query().Get("booking_token") != "" {
			t.Errorf("incorrect return search tokens")
		}
		fmt.Fprint(w, `{"search_metadata":{"status":"Success"},"best_flights":[{
			"price":900,"total_duration":150,"booking_token":"return-token",
			"flights":[{"airline":"United","flight_number":"UA 123",
			"departure_airport":{"time":"2026-11-13 12:00"},"arrival_airport":{"time":"2026-11-13 14:30"}}]
		}]}`)
	})
	in := FlightSearch{Origin: "MCI", Destination: "ORD", Depart: "2026-11-10", Return: "2026-11-13", Travelers: 2, DepartureToken: "outbound-token"}
	flights, err := client.ReturnFlights(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	f := flights[0]
	if f.Origin != "ORD" || f.Destination != "MCI" || f.PriceUSD != 450 || f.BookingToken != "return-token" ||
		f.DepartureTime != "2026-11-13 12:00" || f.ArrivalTime != "2026-11-13 14:30" || f.FlightNumbers[0] != "UA 123" {
		t.Fatalf("return details incorrect: %+v", f)
	}
}

func TestBookingOptionsPreservesOpaquePOSTAndSeparateTickets(t *testing.T) {
	client := testSerpAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("booking_token") != "booking" || r.URL.Query().Get("departure_token") != "" {
			t.Error("incorrect booking tokens")
		}
		fmt.Fprint(w, `{"search_metadata":{"status":"Success"},"booking_options":[
			{"together":{"book_with":"United","price":900,"option_title":"Economy",
			 "extensions":["No refunds"],"baggage_prices":["1 free carry-on"],
			 "booking_request":{"url":"https://www.google.com/travel/clk/f","post_data":"u=A%2BB%3D&v=1"}}},
			{"separate_tickets":true,"departing":{"book_with":"Airline A","price":400},"returning":{"book_with":"Airline B","price":500}}
		]}`)
	})
	options, err := client.BookingOptions(context.Background(), FlightSearch{
		Origin: "MCI", Destination: "ORD", Depart: "2026-11-10", Return: "2026-11-13", Travelers: 2, BookingToken: "booking",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 2 || options[0].Together.Action.PostData != "u=A%2BB%3D&v=1" ||
		options[0].Together.Baggage[0] != "1 free carry-on" || !options[1].SeparateTickets || options[1].Returning.Name != "Airline B" {
		t.Fatalf("booking options = %+v", options)
	}
}

func TestFlightFollowupRejectsMissingTokens(t *testing.T) {
	client := NewSerpAPI("test-secret")
	if _, err := client.ReturnFlights(context.Background(), FlightSearch{}); err == nil {
		t.Fatal("missing departure token accepted")
	}
	if _, err := client.BookingOptions(context.Background(), FlightSearch{}); err == nil {
		t.Fatal("missing booking token accepted")
	}
	for _, in := range []FlightSearch{
		{Origin: "MCI", Destination: "ORD", Depart: "2026-11-10", Return: "2026-11-13", Travelers: 0},
		{Origin: "MCI", Destination: "ORD", Depart: "2026-11-10", Return: "2026-11-09", Travelers: 1},
		{Origin: "MCI", Destination: "ORD", Depart: "invalid", Return: "2026-11-13", Travelers: 1},
	} {
		if _, err := flightQuery(in); err == nil {
			t.Fatalf("invalid query accepted: %+v", in)
		}
	}
}
