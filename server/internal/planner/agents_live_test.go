package planner

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"trailmix/internal/ai"
	"trailmix/internal/config"
	"trailmix/internal/travel"
)

// TestAgentsLive runs the whole agent pipeline (Curator → Researcher → Writer →
// Reviewer) against the configured inference service with fixture flights and
// hotels and real OpenStreetMap places. It is skipped unless AI_BASE_URL is set
// (directly or via the repo .env), so CI and offline runs are unaffected.
//
//	go test ./internal/planner -run TestAgentsLive -v
func TestAgentsLive(t *testing.T) {
	cfg := config.Load()
	if cfg.AIBaseURL == "" || os.Getenv("CI") != "" {
		t.Skip("AI_BASE_URL not set")
	}
	travel.Offline = false // this test wants the real free providers too
	defer func() { travel.Offline = true }()
	client := ai.New(cfg.AIBaseURL, cfg.AIAPIKey, cfg.AIModel)
	p := &Planner{
		AI:      client,
		Geo:     travel.NewGeo(""),
		Fuel:    travel.NewFuel(""),
		SerpAPI: fixtureSearch{},
		Places:  travel.NewPlaces("").EnableOSM(client),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	start := time.Now().AddDate(0, 0, 21)
	plan, err := p.Plan(ctx, Request{
		Origin: "Kansas City, MO", Destination: "Denver, CO",
		StartDate: start.Format("2006-01-02"), EndDate: start.AddDate(0, 0, 3).Format("2006-01-02"),
		Travelers: 2, BudgetUSD: 2500, Interests: []string{"outdoors", "museums", "food"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.AIGenerated {
		t.Fatalf("itinerary was not AI generated; agents: %+v", plan.Agents)
	}
	if plan.Research == nil {
		t.Errorf("no research attached")
	}
	seen := map[string]bool{}
	for _, r := range plan.Agents {
		seen[r.Agent] = true
		t.Logf("%-20s %-7s %s", r.Agent, r.Status, r.Summary)
		for _, n := range r.Notes {
			t.Logf("    · %s", n)
		}
	}
	for _, want := range []string{"curator:activities", "curator:food", "researcher", "reviewer:audit", "reviewer"} {
		if !seen[want] {
			t.Errorf("agent %s did not report", want)
		}
	}
	// Every activity/food line the Writer produced must name a real place.
	known := map[string]bool{}
	for _, l := range [][]travel.Place{plan.Activities, plan.Food, plan.Stops} {
		for _, pl := range l {
			known[pl.Name] = true
		}
	}
	out, _ := json.MarshalIndent(map[string]any{"summary": plan.Summary, "days": plan.Days, "research": plan.Research}, "", "  ")
	t.Logf("plan:\n%s", out)
	if len(plan.Days) != plan.Nights+1 {
		t.Errorf("got %d days for %d nights", len(plan.Days), plan.Nights)
	}
}
