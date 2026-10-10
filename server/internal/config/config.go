package config

import (
	"log"
	"os"
	"strconv"
)

// Config is read once from the environment at startup. Every external
// integration is optional: when its key is missing the app falls back to
// deterministic mock data so the demo never breaks.
type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	StaticDir   string // built frontend to serve; empty = API only
	CORSOrigin  string

	// AI (OpenAI-compatible chat completions: Akamai Inference Cloud, Ollama, etc.)
	AIBaseURL string
	AIAPIKey  string
	AIModel   string

	// Travel data
	AmadeusKey    string
	AmadeusSecret string
	EIAKey        string
	PlacesKey     string // Google Places
	MapboxToken   string

	// Payments
	JackHenryBaseURL string
	JackHenryKey     string

	MockMode bool
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Load() Config {
	c := Config{
		Port:             env("PORT", "8080"),
		DatabaseURL:      env("DATABASE_URL", "postgres://trailmix:trailmix@localhost:5432/trailmix?sslmode=disable"),
		JWTSecret:        env("JWT_SECRET", ""),
		StaticDir:        env("STATIC_DIR", ""),
		CORSOrigin:       env("CORS_ORIGIN", "http://localhost:5173"),
		AIBaseURL:        env("AI_BASE_URL", ""),
		AIAPIKey:         env("AI_API_KEY", ""),
		AIModel:          env("AI_MODEL", "llama-3.1-8b-instruct"),
		AmadeusKey:       env("AMADEUS_KEY", ""),
		AmadeusSecret:    env("AMADEUS_SECRET", ""),
		EIAKey:           env("EIA_KEY", ""),
		PlacesKey:        env("GOOGLE_PLACES_KEY", ""),
		MapboxToken:      env("MAPBOX_TOKEN", ""),
		JackHenryBaseURL: env("JACKHENRY_BASE_URL", ""),
		JackHenryKey:     env("JACKHENRY_KEY", ""),
	}
	c.MockMode, _ = strconv.ParseBool(env("MOCK_MODE", "false"))
	if c.JWTSecret == "" {
		log.Println("WARN: JWT_SECRET not set; using insecure dev secret")
		c.JWTSecret = "dev-secret-change-me"
	}
	return c
}
