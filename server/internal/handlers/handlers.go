package handlers

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"

	"trailmix/internal/auth"
	"trailmix/internal/httpx"
	"trailmix/internal/planner"
	"trailmix/internal/travel"
)

// Deps is everything a handler can reach. Built once in main.
type Deps struct {
	DB      *sql.DB
	Auth    *auth.Service
	Planner *planner.Planner
	Fuel    *travel.Fuel
}

// requireDB returns 503 for routes that need Postgres when it is unavailable.
func (d *Deps) requireDB(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if d.DB == nil {
			httpx.Error(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Mount attaches every route under /api.
func Mount(r chi.Router, d *Deps) {
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, 200, map[string]any{
			"ok":      true,
			"db":      d.DB != nil,
			"ai":      d.Planner.AI.Enabled(),
			"serpapi": d.Planner.SerpAPI.Enabled(),
		})
	})

	// No login needed: sign-up, sign-in and the read-only share page.
	r.Group(func(r chi.Router) {
		r.Use(d.requireDB)
		r.Post("/auth/register", d.register)
		r.Post("/auth/login", d.login)
		r.Get("/share/{id}", d.shareTrip) // public read-only plan by unguessable id
	})

	// Everything else, planning included, needs an account.
	r.Group(func(r chi.Router) {
		r.Use(d.requireDB, d.Auth.Middleware)
		r.Get("/auth/me", d.me)
		r.Patch("/auth/me", d.updateMe)

		r.Post("/plan/preview", d.planPreview)
		r.Post("/plan/preview/start", d.startPreview) // background job; poll /plan/jobs/{id}
		r.Get("/plan/jobs/{id}", d.planJobStatus)
		r.Post("/plan/flight", d.selectPreviewFlight)
		r.Post("/flights/returns", d.returnFlights)
		r.Post("/flights/bookings", d.flightBookings)
		r.Post("/flights/booking-redirect", resolveFlightBooking)
		r.Get("/vehicles/mpg", d.vehicleMPG)
		r.Get("/geo/suggest", d.geoSuggest) // city autocomplete (any city, not just the demo list)

		r.Get("/trips", d.listTrips)
		r.Post("/trips", d.createTrip)
		r.Get("/trips/{id}", d.getTrip)
		r.Delete("/trips/{id}", d.deleteTrip)
		r.Post("/trips/{id}/plan", d.planTrip)
		r.Post("/trips/{id}/plan/start", d.startTripPlan) // background job; saves on completion
		r.Post("/trips/{id}/hotel", d.chooseHotel)        // swap the stay, re-total the plan
		r.Post("/trips/{id}/flight", d.selectTripFlight)
	})
}
