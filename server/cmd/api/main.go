package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"trailmix/internal/ai"
	"trailmix/internal/auth"
	"trailmix/internal/config"
	"trailmix/internal/db"
	"trailmix/internal/handlers"
	"trailmix/internal/payments"
	"trailmix/internal/planner"
	"trailmix/internal/travel"
)

func main() {
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		// Degraded mode: the stateless planner still works so the landing-page
		// demo survives a dead database. Accounts/trips return 503 until it's back.
		log.Printf("WARN database unavailable, running without accounts: %v", err)
		pool = nil
	} else {
		defer pool.Close()
	}

	fuel := travel.NewFuel(cfg.EIAKey)
	deps := &handlers.Deps{
		DB:   pool,
		Auth: auth.New(cfg.JWTSecret),
		Fuel: fuel,
		Planner: &planner.Planner{
			AI:      ai.New(cfg.AIBaseURL, cfg.AIAPIKey, cfg.AIModel),
			Geo:     travel.NewGeo(cfg.MapboxToken),
			Fuel:    fuel,
			Amadeus: travel.NewAmadeus(cfg.AmadeusKey, cfg.AmadeusSecret),
			Places:  travel.NewPlaces(cfg.PlacesKey),
		},
		Payments: payments.NewJackHenry(cfg.JackHenryBaseURL, cfg.JackHenryKey),
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer)
	r.Use(middleware.Timeout(120 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   strings.Split(cfg.CORSOrigin, ","),
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Route("/api", func(r chi.Router) { handlers.Mount(r, deps) })

	if cfg.StaticDir != "" {
		r.NotFound(spaHandler(cfg.StaticDir))
	}

	log.Printf("TPN api listening on :%s (ai=%v amadeus=%v payments=%v static=%q)",
		cfg.Port, deps.Planner.AI.Enabled(), deps.Planner.Amadeus.Enabled(), deps.Payments.Live(), cfg.StaticDir)
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r, ReadHeaderTimeout: 10 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

// spaHandler serves the built frontend and falls back to index.html for
// client-side routes.
func spaHandler(dir string) http.HandlerFunc {
	fs := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")
	return func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, index)
	}
}
