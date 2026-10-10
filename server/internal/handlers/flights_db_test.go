package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"trailmix/internal/auth"
	"trailmix/internal/planner"
	"trailmix/internal/travel"
)

type selectionTestSearch struct{ *travel.SerpAPI }

func (selectionTestSearch) ReturnFlights(context.Context, travel.FlightSearch) ([]travel.FlightQuote, error) {
	return []travel.FlightQuote{{Carrier: "Test return", PriceUSD: 300, DurationMin: 150, BookingToken: "test-return"}}, nil
}

func TestSavedFlightSelectionPersistsAndChecksOwnership(t *testing.T) {
	databaseURL := os.Getenv("FLIGHT_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set FLIGHT_TEST_DATABASE_URL to run saved-flight integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var userID, otherID string
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,name,password_hash)
		VALUES(gen_random_uuid()::text || '@flight-test.invalid','Flight selection test','unused') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := pool.Exec(ctx, "DELETE FROM users WHERE id=$1", userID); err != nil {
			t.Error("test user cleanup failed:", err)
		}
	}()
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,name,password_hash)
		VALUES(gen_random_uuid()::text || '@flight-test.invalid','Flight ownership test','unused') RETURNING id`).Scan(&otherID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := pool.Exec(ctx, "DELETE FROM users WHERE id=$1", otherID); err != nil {
			t.Error("ownership test user cleanup failed:", err)
		}
	}()
	plan := planner.Plan{
		Nights: 1, Days: []planner.Day{{Day: 1}, {Day: 2}}, Travelers: 2, BudgetUSD: 1000,
		OriginName: "Kansas City", DestinationName: "Chicago", Sources: map[string]string{"flights": "serpapi"},
		Flights: []travel.FlightQuote{
			{Carrier: "First", Origin: "MCI", Destination: "ORD", Depart: "2026-11-10", Return: "2026-11-11", PriceUSD: 200, DurationMin: 120},
			{Carrier: "Second", Origin: "MCI", Destination: "ORD", Depart: "2026-11-10", Return: "2026-11-11", PriceUSD: 250, DurationMin: 140, DepartureToken: "departure"},
		},
		Options: []planner.Option{{Mode: "fly", LodgingUSD: 100}},
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	var tripID string
	if err := pool.QueryRow(ctx, `INSERT INTO trips(user_id,title,origin,destination,start_date,end_date,travelers,status,plan)
		VALUES($1,'Selection test','Kansas City','Chicago','2026-11-10','2026-11-11',2,'planned',$2) RETURNING id`, userID, raw).Scan(&tripID); err != nil {
		t.Fatal(err)
	}
	d := &Deps{DB: pool, Auth: auth.New("local-flight-integration-test"), Planner: &planner.Planner{SerpAPI: selectionTestSearch{travel.NewSerpAPI("fixture")}}}
	router := chi.NewRouter()
	router.Use(d.Auth.Middleware)
	router.Post("/trips/{id}/flight", d.selectTripFlight)
	request := func(uid, body string) *httptest.ResponseRecorder {
		t.Helper()
		token, err := d.Auth.Issue(uid, "flight-test.invalid")
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodPost, "/trips/"+tripID+"/flight", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	if w := request(otherID, `{"flight_index":1}`); w.Code != 404 {
		t.Fatalf("ownership check = %d", w.Code)
	}
	if w := request(userID, `{"flight_index":1}`); w.Code != 200 {
		t.Fatalf("selection = %d: %s", w.Code, w.Body.String())
	}
	if w := request(userID, `{"flight_index":1,"return_token":"test-return"}`); w.Code != 200 {
		t.Fatalf("return selection = %d: %s", w.Code, w.Body.String())
	}
	var saved []byte
	if err := pool.QueryRow(ctx, "SELECT plan FROM trips WHERE id=$1", tripID).Scan(&saved); err != nil {
		t.Fatal(err)
	}
	var out planner.Plan
	if err := json.Unmarshal(saved, &out); err != nil {
		t.Fatal(err)
	}
	if out.SelectedFlight != 1 || out.SelectedReturn == nil || out.SelectedReturn.BookingToken != "test-return" || out.TotalUSD != 700 {
		t.Fatalf("selection not persisted: %+v", out)
	}
	if _, err := pool.Exec(ctx, "UPDATE trips SET status='booked' WHERE id=$1", tripID); err != nil {
		t.Fatal(err)
	}
	if w := request(userID, `{"flight_index":0}`); w.Code != 409 {
		t.Fatalf("booked trip protection = %d", w.Code)
	}
}
