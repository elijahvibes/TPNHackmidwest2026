package config

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config is read once from the environment at startup. Every external
// flight and hotel search requires SERPAPI_KEY; other integrations are optional.
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
	SerpAPIKey  string
	EIAKey      string
	PlacesKey   string // Google Places
	GeoapifyKey string
	MapboxToken string

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

// loadDotEnv reads KEY=VALUE lines from the first .env found in the working
// directory or its parent (repo root when running from server/). Real
// environment variables always win, so Docker/Linode config is unaffected.
func loadDotEnv() {
	for _, p := range []string{".env", filepath.Join("..", ".env")} {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			k, v = strings.TrimSpace(k), strings.Trim(strings.TrimSpace(v), `"'`)
			if os.Getenv(k) == "" {
				os.Setenv(k, v)
			}
		}
		log.Printf("loaded %s", p)
		return
	}
}

func Load() Config {
	loadDotEnv()
	c := Config{
		Port:             env("PORT", "8080"),
		DatabaseURL:      env("DATABASE_URL", "mysql://trailmix:trailmix@localhost:3306/trailmix"),
		JWTSecret:        env("JWT_SECRET", ""),
		StaticDir:        env("STATIC_DIR", ""),
		CORSOrigin:       env("CORS_ORIGIN", "http://localhost:5173"),
		AIBaseURL:        env("AI_BASE_URL", ""),
		AIAPIKey:         env("AI_API_KEY", ""),
		AIModel:          env("AI_MODEL", "hackathon-chat"),
		SerpAPIKey:       env("SERPAPI_KEY", ""),
		EIAKey:           env("EIA_KEY", ""),
		PlacesKey:        env("GOOGLE_PLACES_KEY", ""),
		GeoapifyKey:      env("GEOAPIFY_KEY", ""),
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
