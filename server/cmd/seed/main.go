// Seed creates a demo account with two planned trips so the dashboard is never
// empty on stage. Safe to re-run: the user is upserted and trips are only
// created when the account has none.
//
//	go run ./cmd/seed
//	login: demo@trailmix.app / demo1234
package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"

	"trailmix/internal/ai"
	"trailmix/internal/auth"
	"trailmix/internal/config"
	"trailmix/internal/db"
	"trailmix/internal/planner"
	"trailmix/internal/travel"
)

func main() {
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	hash, _ := auth.HashPassword("demo1234")
	_, err = pool.ExecContext(ctx, `
		INSERT INTO users (id, email, name, password_hash, home_city, home_lat, home_lng)
		VALUES (?, ?, ?, ?, 'Kansas City', 39.0997, -94.5786)
		ON DUPLICATE KEY UPDATE password_hash = VALUES(password_hash)`,
		uuid.NewString(), "demo@trailmix.app", "Demo Traveler", hash)
	if err != nil {
		log.Fatalf("upsert user: %v", err)
	}
	var userID string
	if err := pool.QueryRowContext(ctx, "SELECT id FROM users WHERE email=?", "demo@trailmix.app").Scan(&userID); err != nil {
		log.Fatalf("load user: %v", err)
	}

	var n int
	_ = pool.QueryRowContext(ctx, "SELECT count(*) FROM trips WHERE user_id=?", userID).Scan(&n)
	if n > 0 {
		log.Printf("demo user %s already has %d trips; nothing to do", userID, n)
		return
	}

	fuel := travel.NewFuel(cfg.EIAKey)
	aiClient := ai.New(cfg.AIBaseURL, cfg.AIAPIKey, cfg.AIModel)
	p := &planner.Planner{
		AI:      aiClient,
		Geo:     travel.NewGeo(cfg.MapboxToken),
		Fuel:    fuel,
		SerpAPI: travel.NewSerpAPI(cfg.SerpAPIKey),
		Places:  travel.NewPlacesWithGeoapify(cfg.PlacesKey, cfg.GeoapifyKey).EnableOSM(aiClient),
		Weather: travel.NewNWS(),
		Wiki:    travel.NewWiki(),
	}

	start := time.Now().AddDate(0, 0, 14)
	trips := []planner.Request{
		{Origin: "Kansas City", Destination: "Nashville", StartDate: date(start), EndDate: date(start.AddDate(0, 0, 3)), Travelers: 2, BudgetUSD: 900, Mode: "auto", VehicleMPG: 33},
		{Origin: "Kansas City", Destination: "Denver", StartDate: date(start.AddDate(0, 0, 30)), EndDate: date(start.AddDate(0, 0, 34)), Travelers: 4, BudgetUSD: 2500, Mode: "auto", VehicleMPG: 24},
	}
	for _, req := range trips {
		plan, err := p.Plan(ctx, req)
		if err != nil {
			log.Fatalf("plan %s→%s: %v", req.Origin, req.Destination, err)
		}
		raw, _ := json.Marshal(plan)
		_, err = pool.ExecContext(ctx, `
			INSERT INTO trips (id, user_id, title, origin, destination, start_date, end_date, travelers, budget_usd, mode, status, plan)
			VALUES (?,?,?,?,?,?,?,?,?,?,'planned',?)`,
			uuid.NewString(), userID, req.Origin+" to "+req.Destination, req.Origin, req.Destination, req.StartDate, req.EndDate, req.Travelers, req.BudgetUSD, req.Mode, string(raw))
		if err != nil {
			log.Fatalf("insert trip: %v", err)
		}
		log.Printf("seeded %s → %s: %s, $%.0f", req.Origin, req.Destination, plan.RecommendedMode, plan.TotalUSD)
	}
	log.Println("done. login with demo@trailmix.app / demo1234")
}

func date(t time.Time) string { return t.Format("2006-01-02") }
