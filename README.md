# trailmix — HackMidwest 2026

AI vacation planner that prices the whole trip (flights, gas for *your* car, hotels, things to do,
stops along the way) and picks the best value. Go backend, TypeScript/React frontend, Postgres,
AI inference and hosting on Akamai, payments through Jack Henry.

```
web/      Vite + React + TypeScript + Tailwind (the UI)
server/   Go API (chi + pgx), serves the built UI in production
```

## Run it locally

Requirements: Go 1.24+, Node 20+, Docker (for Postgres).

```bash
cp .env.example .env          # fill in any keys you have; everything else mocks itself
docker compose up -d db       # Postgres on :5432
```

Backend (terminal 1):

```bash
cd server && go run ./cmd/api
```

Frontend (terminal 2):

```bash
cd web && npm install && npm run dev
```

Open http://localhost:5173. Vite proxies `/api` to the Go server on :8080.
The Go server reads `.env` only through the environment, so on Windows use
`Get-Content ..\.env | ForEach-Object { if ($_ -match '^(\w+)=(.*)$') { [Environment]::SetEnvironmentVariable($matches[1], $matches[2]) } }`
in PowerShell before `go run`, or just set the variables you need.

## API

| Method | Path | Auth | What |
|---|---|---|---|
| GET | `/api/health` | no | which integrations are live |
| POST | `/api/auth/register` `/login` | no | returns a JWT |
| GET/PATCH | `/api/auth/me` | yes | profile; PATCH stores home geolocation |
| GET | `/api/vehicles/mpg?year&make&model` | yes | combined MPG from fueleconomy.gov |
| POST | `/api/plan/preview` | no | price + plan a trip without saving (landing page demo) |
| GET/POST | `/api/trips` | yes | list / create |
| GET/DELETE | `/api/trips/{id}` | yes | one trip |
| POST | `/api/trips/{id}/plan` | yes | run the planner, store the itinerary |
| GET/POST | `/api/payments` | yes | Jack Henry charge; marks the trip `booked` |

## How the planner works (`server/internal/planner`)

1. Geocode origin and destination (browser GPS, built-in gazetteer, or Mapbox).
2. Fetch in parallel-friendly steps: driving route + gas cost (EIA price × your car's MPG), round-trip
   flights (Amadeus), hotels near the destination (Amadeus), attractions and food (Google Places),
   rest stops interpolated along the route.
3. Score `drive` vs `fly`: `value = 10000 / (cost per night + 50) − 0.6 × travel hours`.
4. Hand the real numbers to the LLM on Akamai and ask for a day-by-day JSON itinerary.
   Prices are never invented by the model; it only arranges what was fetched.
5. If any key is missing, that provider returns deterministic mock data and the plan's
   `sources` map says so, so the demo never breaks.

## Deploy on Akamai / Linode

One Linode (a 2 GB shared instance is enough for the app; the model runs elsewhere):

```bash
# on the Linode (Ubuntu)
apt update && apt install -y docker.io docker-compose-v2 git
git clone <this repo> trailmix && cd trailmix
cp .env.example .env && nano .env       # JWT_SECRET, AI_BASE_URL, keys…
docker compose --profile prod up -d --build
```

That builds the UI, compiles the Go binary, and serves both on port 80 with Postgres alongside.
Point a domain at the Linode IP and put Caddy or Nginx in front for TLS if there is time.

**AI on Akamai**: set `AI_BASE_URL` to your Akamai Inference Cloud endpoint (OpenAI-compatible, ends in `/v1`)
plus `AI_API_KEY` and `AI_MODEL`. Alternative: a Linode GPU instance running `ollama serve` with
`AI_BASE_URL=http://<gpu-ip>:11434/v1` and `AI_MODEL=llama3.1`.

**Jack Henry**: `server/internal/payments/jackhenry.go` posts to `${JACKHENRY_BASE_URL}/payments`.
Adjust the path and payload to match the sandbox the sponsor hands out. Without it, payments settle in mock mode.

## Where to get keys (all free)

- Amadeus Self-Service (flights, hotels): https://developers.amadeus.com
- EIA (weekly US gas price): https://www.eia.gov/opendata/
- Google Places API (New): https://console.cloud.google.com
- Mapbox (geocoding, directions): https://account.mapbox.com
- fueleconomy.gov vehicle MPG: no key needed
