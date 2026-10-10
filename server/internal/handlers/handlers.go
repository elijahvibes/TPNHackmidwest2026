package handlers

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"

	"trailmix/internal/auth"
	"trailmix/internal/httpx"
	"trailmix/internal/payments"
	"trailmix/internal/planner"
	"trailmix/internal/travel"
)

// Deps is everything a handler can reach. Built once in main.
type Deps struct {
	DB       *sql.DB
	Auth     *auth.Service
	Planner  *planner.Planner
	Fuel     *travel.Fuel
	Payments *payments.Client
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
			"ok":       true,
			"db":       d.DB != nil,
			"ai":       d.Planner.AI.Enabled(),
			"amadeus":  d.Planner.Amadeus.Enabled(),
			"payments": d.Payments.Live(),
		})
	})

	// No login needed: the landing-page demo uses both of these.
	r.Post("/plan/preview", d.planPreview)
	r.Get("/vehicles/mpg", d.vehicleMPG)

	r.Group(func(r chi.Router) {
		r.Use(d.requireDB)
		r.Post("/auth/register", d.register)
		r.Post("/auth/login", d.login)
	})

	r.Group(func(r chi.Router) {
		r.Use(d.requireDB, d.Auth.Middleware)
		r.Get("/auth/me", d.me)
		r.Patch("/auth/me", d.updateMe)

		r.Get("/trips", d.listTrips)
		r.Post("/trips", d.createTrip)
		r.Get("/trips/{id}", d.getTrip)
		r.Delete("/trips/{id}", d.deleteTrip)
		r.Post("/trips/{id}/plan", d.planTrip)

		r.Get("/payments", d.listPayments)
		r.Post("/payments", d.createPayment)
	})
}
