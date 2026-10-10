package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"trailmix/internal/auth"
	"trailmix/internal/httpx"
	"trailmix/internal/planner"
)

type Trip struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Origin      string          `json:"origin"`
	Destination string          `json:"destination"`
	StartDate   string          `json:"start_date"`
	EndDate     string          `json:"end_date"`
	Travelers   int             `json:"travelers"`
	BudgetUSD   *float64        `json:"budget_usd"`
	Mode        string          `json:"mode"`
	Status      string          `json:"status"`
	Plan        json.RawMessage `json:"plan,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

const tripCols = "id, title, origin, destination, start_date::text, end_date::text, travelers, budget_usd, mode, status, plan, created_at"

func scanTrip(row pgx.Row) (Trip, error) {
	var t Trip
	var plan []byte
	err := row.Scan(&t.ID, &t.Title, &t.Origin, &t.Destination, &t.StartDate, &t.EndDate, &t.Travelers, &t.BudgetUSD, &t.Mode, &t.Status, &plan, &t.CreatedAt)
	if len(plan) > 0 {
		t.Plan = plan
	}
	return t, err
}

func (d *Deps) listTrips(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	rows, err := d.DB.Query(r.Context(), "SELECT "+tripCols+" FROM trips WHERE user_id=$1 ORDER BY created_at DESC", c.UserID)
	if err != nil {
		httpx.Error(w, 500, "query failed")
		return
	}
	defer rows.Close()
	trips := []Trip{}
	for rows.Next() {
		t, err := scanTrip(rows)
		if err != nil {
			httpx.Error(w, 500, "scan failed")
			return
		}
		trips = append(trips, t)
	}
	httpx.JSON(w, 200, trips)
}

type tripInput struct {
	Title       string   `json:"title"`
	Origin      string   `json:"origin"`
	Destination string   `json:"destination"`
	StartDate   string   `json:"start_date"`
	EndDate     string   `json:"end_date"`
	Travelers   int      `json:"travelers"`
	BudgetUSD   *float64 `json:"budget_usd"`
	Mode        string   `json:"mode"`
}

func (d *Deps) createTrip(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var in tripInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, 400, "invalid json")
		return
	}
	if in.Origin == "" || in.Destination == "" || in.StartDate == "" || in.EndDate == "" {
		httpx.Error(w, 400, "origin, destination, start_date and end_date are required")
		return
	}
	if in.Travelers < 1 {
		in.Travelers = 1
	}
	if in.Mode == "" {
		in.Mode = "auto"
	}
	if in.Title == "" {
		in.Title = in.Origin + " to " + in.Destination
	}
	t, err := scanTrip(d.DB.QueryRow(r.Context(),
		`INSERT INTO trips (user_id, title, origin, destination, start_date, end_date, travelers, budget_usd, mode)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+tripCols,
		c.UserID, in.Title, in.Origin, in.Destination, in.StartDate, in.EndDate, in.Travelers, in.BudgetUSD, in.Mode))
	if err != nil {
		httpx.Error(w, 400, "could not create trip: "+err.Error())
		return
	}
	httpx.JSON(w, 201, t)
}

func (d *Deps) getTrip(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	t, err := scanTrip(d.DB.QueryRow(r.Context(), "SELECT "+tripCols+" FROM trips WHERE id=$1 AND user_id=$2", chi.URLParam(r, "id"), c.UserID))
	if err != nil {
		httpx.Error(w, 404, "trip not found")
		return
	}
	httpx.JSON(w, 200, t)
}

func (d *Deps) deleteTrip(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	tag, err := d.DB.Exec(r.Context(), "DELETE FROM trips WHERE id=$1 AND user_id=$2", chi.URLParam(r, "id"), c.UserID)
	if err != nil || tag.RowsAffected() == 0 {
		httpx.Error(w, 404, "trip not found")
		return
	}
	w.WriteHeader(204)
}

// planTrip runs the planner for a saved trip and stores the result.
func (d *Deps) planTrip(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	t, err := scanTrip(d.DB.QueryRow(r.Context(), "SELECT "+tripCols+" FROM trips WHERE id=$1 AND user_id=$2", chi.URLParam(r, "id"), c.UserID))
	if err != nil {
		httpx.Error(w, 404, "trip not found")
		return
	}
	var extra struct {
		VehicleMPG float64  `json:"vehicle_mpg"`
		Interests  []string `json:"interests"`
	}
	_ = httpx.Decode(r, &extra) // body optional
	req := planner.Request{
		Origin: t.Origin, Destination: t.Destination, StartDate: t.StartDate, EndDate: t.EndDate,
		Travelers: t.Travelers, Mode: t.Mode, VehicleMPG: extra.VehicleMPG, Interests: extra.Interests,
	}
	if t.BudgetUSD != nil {
		req.BudgetUSD = *t.BudgetUSD
	}
	plan, err := d.Planner.Plan(r.Context(), req)
	if err != nil {
		httpx.Error(w, 400, err.Error())
		return
	}
	raw, _ := json.Marshal(plan)
	if _, err := d.DB.Exec(r.Context(), "UPDATE trips SET plan=$1, status='planned', updated_at=now() WHERE id=$2", raw, t.ID); err != nil {
		httpx.Error(w, 500, "save failed")
		return
	}
	httpx.JSON(w, 200, plan)
}

// planPreview is the stateless version used by the landing page.
func (d *Deps) planPreview(w http.ResponseWriter, r *http.Request) {
	var req planner.Request
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, 400, "invalid json")
		return
	}
	plan, err := d.Planner.Plan(r.Context(), req)
	if err != nil {
		httpx.Error(w, 400, err.Error())
		return
	}
	httpx.JSON(w, 200, plan)
}

// vehicleMPG: GET /api/vehicles/mpg?year=2019&make=Honda&model=Civic
func (d *Deps) vehicleMPG(w http.ResponseWriter, r *http.Request) {
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	make, model := r.URL.Query().Get("make"), r.URL.Query().Get("model")
	if year == 0 || make == "" || model == "" {
		httpx.Error(w, 400, "year, make and model are required")
		return
	}
	mpg, err := d.Fuel.VehicleMPG(r.Context(), year, make, model)
	if err != nil {
		httpx.JSON(w, 200, map[string]any{"mpg": 28, "source": "fallback", "note": err.Error()})
		return
	}
	httpx.JSON(w, 200, map[string]any{"mpg": mpg, "source": "fueleconomy.gov"})
}
