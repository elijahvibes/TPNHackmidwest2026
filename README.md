# TPN — HackMidwest 2026

TPN is an AI vacation planner that prices the whole trip (flights, gas for *your* car, hotels, things to do,
stops along the way) and picks the best value. Go backend, TypeScript/React frontend, MySQL,
AI inference and hosting on Akamai, payments through Jack Henry.

```
web/      Vite + React + TypeScript + Tailwind (the UI)
server/   Go API (chi + database/sql), serves the built UI in production
```

## Run it locally

Requirements: Go 1.26+, Node 20+, MySQL (Docker or the Windows helper below).

```bash
cp .env.example .env          # set SERPAPI_KEY, DATABASE_URL and JWT_SECRET
```

MySQL, pick one:

```bash
docker compose --profile local up -d db     # Docker (mysql:8.4)
```

```powershell
.\scripts\dev-db.ps1                        # Windows, no Docker: portable MariaDB in .mariadb/
```

Prod uses the team's Akamai Managed MySQL database. Its `private-…` hostname only resolves inside
Akamai, so laptops use a local database and the Linode uses the managed one (`.env.linode` → `.env` on the server).

The Go server loads `.env` from the repo root automatically; real environment variables win.

Demo data: `cd server && go run ./cmd/seed` creates `demo@trailmix.app` / `demo1234` with two planned trips.
Seeding requires `SERPAPI_KEY` and fetches real flight and hotel prices.
Smoke test: `scripts/smoke.sh [base-url]` exercises every endpoint.

Backend (terminal 1):

```bash
# From the repository root; the server loads .env automatically.
cd server && go run ./cmd/api
```

Frontend (terminal 2):

```bash
cd web && npm install && npm run dev
```

Open http://localhost:5173. Vite proxies `/api` to the Go server on :8080.

## API

| Method | Path | Auth | What |
|---|---|---|---|
| GET | `/api/health` | no | database availability and which integrations are configured |
| POST | `/api/auth/register` `/login` | no | returns a JWT |
| GET/PATCH | `/api/auth/me` | yes | profile; PATCH stores home geolocation |
| GET | `/api/vehicles/mpg?year&make&model` | no | combined MPG from fueleconomy.gov |
| GET | `/api/geo/suggest?q=law` | no | city / town / park autocomplete (any place, US first) |
| POST | `/api/plan/preview` | no | price + plan a trip without saving (landing page demo) |
| GET/POST | `/api/trips` | yes | list / create |
| GET/DELETE | `/api/trips/{id}` | yes | one trip |
| POST | `/api/trips/{id}/plan` | yes | run the planner, store the itinerary; body may carry `mode`, `origin_loc`, `dest_loc`, `vehicle_mpg`, `interests` |
| POST | `/api/trips/{id}/hotel` | yes | `{"name"}` swaps the stay and re-totals the stored plan |
| GET/POST | `/api/payments` | yes | Jack Henry charge; marks the trip `booked` |

## How the planner works (`server/internal/planner`)

1. Resolve origin and destination. Any city works: a point pinned by the form's autocomplete or
   browser GPS wins; otherwise the name goes through the built-in gazetteer (~300 US cities and parks,
   offline), then Mapbox when `MAPBOX_TOKEN` is set, then Photon/Nominatim (OpenStreetMap, no key).
2. Fetch in parallel-friendly steps: driving route + gas cost (EIA price × your car's MPG), round-trip
   flights and hotels (SerpApi Google Flights / Google Hotels) between the **nearest major airports** to
   each place (the drive to the terminal counts toward door-to-door time), attractions and food
   (Google Places), rest stops interpolated along the route.
3. Score `drive` vs `fly`: `value = 10000 / (cost per night + 50) − 0.6 × travel hours`.
4. Hand the real numbers to the LLM on Akamai and ask for a day-by-day JSON itinerary.
   Prices are never invented by the model; it only arranges what was fetched.
5. Flights and hotels require SerpApi and never fall back to mock prices:
   missing credentials, provider failures or missing priced results return an explicit
   HTTP 502. Gas, routes and places retain their labeled fallbacks and estimates.

## Mock and non-live data warnings

Every displayed plan (preview, saved trip, or shared trip) shows a prominent red
warning whenever its source tags or individual results indicate mock data,
fallbacks, or estimates. The warning names the affected categories, including
mixed live/mock attractions, restaurants, and rest stops. Static safety data,
Google Places price-level cost estimates, and rule-based itineraries are identified
separately rather than presented as live API results. Existing fallbacks remain enabled.

Alongside the warning, a green "Real provider data used in this plan" section
identifies actual flight/hotel results, Google Places names/locations/ratings,
EIA gas prices, Mapbox directions, available NWS forecast days, and AI narration.
Mixed live/mock categories show real-result counts as well as mock warnings.
Empty results and static safety data are not labeled live. Provider provenance is
recorded at plan generation; saved results are not newly checked, and Google
Places costs remain estimates rather than real menu or admission prices.

Payment screens also warn about configured mock mode and persisted `JH-MOCK-`
receipts. Mock settlement does not move money or reserve travel.

Warning detection tests: `node --test web/tests/dataWarnings.test.mjs` (Node 22.18+).

## Real flights and hotels with SerpApi

Amadeus Self-Service was decommissioned in July 2026. Its integration has been replaced
with [SerpApi](https://serpapi.com), using one server-side `SERPAPI_KEY` for both
[Google Flights](https://serpapi.com/google-flights-api) and
[Google Hotels](https://serpapi.com/google-hotels-api).

1. Add `SERPAPI_KEY` to your local `.env`. Never put it in a frontend variable or commit it.
2. Restart the API with the environment loaded as shown above.
3. `/api/health` reports `serpapi: true` when the key is configured (not proof of a successful search).
4. Test without login or Postgres:

```bash
curl --max-time 120 http://localhost:8080/api/plan/preview \
  -H 'Content-Type: application/json' \
  -d '{"origin":"Kansas City","destination":"Chicago","start_date":"2026-11-10","end_date":"2026-11-13","travelers":2,"mode":"fly","budget_usd":2000}'
```

Successful results have `sources.flights` and `sources.hotels` set to `serpapi`.
Flight quotes are round-trip prices per traveler; the planner multiplies them by the
traveler count. Hotels are searched for that guest count and use the returned full-stay
total rather than multiplying a nightly rate or using an explicitly pre-tax rate.
All travelers are currently treated as adults.

### Choosing flights

All returned departure options are retained. Recommended order balances price,
outbound duration and stops: `fare per traveler + $25/hour + $50/connection`
(lowest score first). This is a transparent ranking heuristic, not an added fee.
The recommended flight is first and selected by default. The UI offers lowest-price
and shortest-duration sorts, a nonstop filter, and expandable lists.

Choose a departure, then fetch return options on demand. Selecting a return uses
the provider's updated round-trip fare and the actual outbound + return duration.
Trip totals, budget status and the deterministic itinerary update together.
Saved-trip selections are persisted; booked trips cannot change flights. Older saved
plans must be re-planned to add selection tokens and traveler metadata.

Booking options are fetched only after return selection. They include seller links,
fare conditions and baggage information when provided. Google booking actions
requiring POST are resolved server-side using the opaque provider payload.
Separate-ticket options are labeled. Following a seller link does not buy a ticket;
complete and confirm the booking on the seller's site. Jack Henry payment alone
does not reserve airline tickets.

Endpoints: `POST /api/flights/returns`, `/api/flights/bookings`,
`/api/plan/flight` (preview selection), and `/api/trips/{id}/flight`
(authenticated saved selection). Return and booking lookups consume additional
SerpApi searches when not cached; initial planning still makes only two searches.
Selection updates the itinerary without another AI call.

These are real Google Travel search results, not bookable or guaranteed quotes.
Final prices, optional baggage and other fees may differ. Each plan normally needs two
searches, with SerpApi's default cache enabled (up to one hour). Check
[current quotas and pricing](https://serpapi.com/pricing) before testing repeatedly.
Without `SERPAPI_KEY`, trip planning returns an explicit error; there are no mock
flight or hotel quotes. Existing saved plans are not rewritten and may still contain
older mock quotes; re-plan them to fetch real prices.

## Deploy on Akamai / Linode

One Linode (a 2 GB shared instance is enough for the app; the model runs elsewhere):

```bash
# on the Linode (Ubuntu)
apt update && apt install -y docker.io docker-compose-v2 git
git clone https://github.com/elijahvibes/TPNHackmidwest2026.git trailmix && cd trailmix
scp .env.linode <linode>:trailmix/.env     # from a laptop that has it; then add AI_BASE_URL + keys
docker compose --profile prod up -d --build
```

That builds the UI, compiles the Go binary, and serves both behind Caddy with automatic HTTPS at
`https://172-239-49-30.sslip.io` (an `sslip.io` name resolves to the IP it encodes, so no DNS setup).
Plain `http://172.239.49.30` keeps working. For a real domain: point its A record at the Linode,
set `SITE_HOST=yourdomain.com` in `.env`, redeploy; Caddy fetches the certificate itself.

The database is the Akamai Managed MySQL; the Linode must be in the same region/VLAN for the
`private-` host, or add the Linode's IP to the database's access controls and use the public host.
Redeploys from a laptop: `scripts/ship.sh` (the repo is private, so it uploads the tree instead of `git pull`).

**AI on Akamai**: set `AI_BASE_URL` to your Akamai Inference Cloud endpoint (OpenAI-compatible, ends in `/v1`)
plus `AI_API_KEY` and `AI_MODEL`. Alternative: a Linode GPU instance running `ollama serve` with
`AI_BASE_URL=http://<gpu-ip>:11434/v1` and `AI_MODEL=llama3.1`.

**Jack Henry**: `server/internal/payments/jackhenry.go` posts to `${JACKHENRY_BASE_URL}/payments`.
This is a generic placeholder, not a verified Jack Henry API contract. Without configuration, payments settle in mock mode.

## Where to get keys (all free)

- SerpApi (Google Flights, Google Hotels; limited free tier): https://serpapi.com
- EIA (weekly US gas price): https://www.eia.gov/opendata/
- Google Places API (New): https://console.cloud.google.com
- Mapbox (geocoding, directions): https://account.mapbox.com
- fueleconomy.gov vehicle MPG: no key needed
