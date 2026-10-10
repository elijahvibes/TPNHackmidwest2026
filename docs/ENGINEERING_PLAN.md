# trailmix — Engineering Plan

HackMidwest 2026 · Go backend + TypeScript frontend · hosted and inferencing on Akamai · payments by Jack Henry

This is the working plan for the team. Sections 1–2 describe what exists today. Section 3 is the
work, broken into streams you can pick up independently. Section 4 is the priority order; Section 5
is a suggested schedule; Section 6 is the demo we are building toward.

---

## 1. What we have today

About 2,900 lines across both halves, pushed to `main`. Status key: ✅ verified working · 🟡 built but
untested (needs Postgres or an API key) · ⬜ not started.

### Backend (`server/`, Go 1.24, chi + pgx + JWT)

| Area | File | Status | Notes |
|---|---|---|---|
| HTTP server, CORS, static SPA serving | `cmd/api/main.go` | ✅ | Runs in degraded mode if Postgres is down |
| Config from env | `internal/config` | ✅ | Every integration optional |
| MySQL (Akamai managed) + auto-migrations | `internal/db`, `migrations/001_init.sql` | ✅ | Tables: users, vehicles, trips, payments |
| Register / login / me / update home location | `internal/handlers/auth.go` | 🟡 | bcrypt + 7-day JWT |
| Trips CRUD + plan + preview | `internal/handlers/trips.go` | 🟡 preview ✅ | Plan stored as JSONB on the trip |
| Payments (Jack Henry) | `internal/handlers/payments.go`, `internal/payments` | 🟡 mock ✅ | Generic REST shape, needs sandbox docs |
| Planner + value scoring + fallback itinerary | `internal/planner` | ✅ | 4 unit tests pass |
| AI client (OpenAI-compatible) | `internal/ai` | 🟡 | Needs `AI_BASE_URL` |
| Geocoding + routing | `internal/travel/geo.go` | ✅ gazetteer / 🟡 Mapbox | 22 cities built in |
| Gas price + vehicle MPG | `internal/travel/fuel.go` | ✅ MPG live / 🟡 EIA | fueleconomy.gov needs no key |
| Flights + hotels | `internal/travel/amadeus.go` | 🟡 | Mock fallback ✅ |
| Things to do / food / rest stops | `internal/travel/places.go` | 🟡 | Mock fallback ✅ |

### Frontend (`web/`, Vite + React 19 + TypeScript + Tailwind 4)

| Page / component | Status | Notes |
|---|---|---|
| Landing with live "try it" planner | ✅ | No login needed |
| Login / Register | 🟡 | Needs Postgres to test |
| Dashboard (trip list) | 🟡 | |
| New trip (save → plan → redirect) | 🟡 | |
| Trip view (plan, re-plan, book) | 🟡 | |
| `TripForm` (GPS button, MPG lookup, mode toggle) | ✅ | |
| `PlanView` (options, itinerary, raw data) | ✅ | |
| Typed API client + auth context | ✅ | |

### Ops

| Item | Status |
|---|---|
| `Dockerfile` (multi-stage: UI → Go → alpine) | 🟡 not built yet |
| `docker-compose.yml` (db + prod app profile) | 🟡 Docker Desktop would not start on the dev laptop |
| `.env.example`, README with Linode steps | ✅ |
| Linode instance, domain, TLS | ⬜ |
| Akamai inference endpoint | ⬜ |
| Jack Henry sandbox credentials | ⬜ |

---

## 2. Architecture

```
 browser (React SPA)
   │  /api/*  (JWT in Authorization header)
   ▼
 Go API  ──────────────────────────────┐
   ├─ handlers  (auth, trips, payments)│
   ├─ planner   (orchestrates below)   │   Akamai Managed MySQL (users, vehicles, trips.plan JSON, payments)
   │    ├─ travel.Geo      → Mapbox            (geocode, directions)
   │    ├─ travel.Fuel     → fueleconomy.gov    (MPG, no key)   + EIA (gas $/gal)
   │    ├─ travel.Amadeus  → Amadeus test API   (flights, hotels)
   │    ├─ travel.Places   → Google Places New  (attractions, food, rest stops)
   │    └─ ai.Client       → Akamai Inference   (OpenAI-compatible chat completions)
   └─ payments.Client      → Jack Henry sandbox
```

Request flow for "plan my trip":

1. Geocode origin (browser GPS if given) and destination.
2. Route + gas cost, flights, hotels, places, rest stops. Each provider returns mock data if its key is missing and tags its `source`.
3. Build `drive` and `fly` options. `value = 10000 / (cost_per_night + 50) − 0.6 × travel_hours`. Best value that fits the budget wins; user can force a mode.
4. Send the real numbers to the LLM with a strict JSON schema. The model arranges, never invents prices. On any failure, the deterministic fallback itinerary is used.
5. Store the plan on the trip; UI renders it.

Prod topology: **one Linode** running `docker compose --profile prod` (Postgres + app on :80). The Go binary serves the built SPA, so there is one container to deploy and no CORS in prod. The model runs on Akamai Inference Cloud (or a GPU Linode with Ollama) and is reached over HTTPS.

---

## 3. Workstreams

Each task has an acceptance test. Size: S ≈ 1 h, M ≈ 2–3 h, L ≈ half a day.

### A. Infra & deploy (owner: ____)

| # | Task | Size | Done when |
|---|---|---|---|
| A1 | Get Postgres running locally for everyone (Docker Desktop, or `winget install PostgreSQL.PostgreSQL.16` on Windows) | S | `curl localhost:8080/api/health` shows `"db":true` |
| A2 | Provision Linode (Ubuntu 24.04, 2 GB shared is enough), install Docker, clone repo | S | `ssh` works, `docker compose version` prints |
| A3 | First prod deploy: `.env` with real `JWT_SECRET`, `docker compose --profile prod up -d --build` | M | Landing page loads on `http://<linode-ip>` |
| A4 | Domain + TLS (Caddy in front, or Linode NodeBalancer) | M | `https://` works, HTTP redirects |
| A5 | Deploy script: `deploy.sh` that pulls main, rebuilds, restarts; document in README | S | One command redeploys |
| A6 | Nightly-safe: `restart: unless-stopped` verified, Postgres volume survives `down/up` | S | Data persists across restart |

Risk: Docker Desktop on the Windows laptop would not start its engine today. If it keeps failing, use native Postgres locally and only use Docker on the Linode.

### B. AI on Akamai (owner: ____)

| # | Task | Size | Done when |
|---|---|---|---|
| B1 | Get the Akamai inference endpoint + key from the sponsor; confirm it is OpenAI-compatible (`POST /v1/chat/completions`) | S | `curl` returns a completion |
| B2 | Set `AI_BASE_URL`, `AI_API_KEY`, `AI_MODEL`; verify `/api/health` shows `"ai":true` and a plan has `ai_generated: true` | S | Plan summary is clearly model-written |
| B3 | Prompt hardening: test 10 city pairs; if the model returns invalid JSON, add a repair pass (ask it to fix its own output) in `ai.Chat` | M | ≥ 9/10 plans parse first try |
| B4 | Enforce schema server-side: validate `days[].items[]` categories and that costs ≤ fetched prices; drop bad items instead of failing | M | No plan ever 500s because of the model |
| B5 | Interests: pass `interests[]` (food, outdoors, museums, nightlife) from the form through to the prompt and to Places type selection | M | Changing interests changes the itinerary |
| B6 | Fallback: if the endpoint is slow (> 60 s) return the deterministic itinerary and flag `ai_generated:false` | S | Timeout never blocks the demo |
| B7 | Stretch: stream the itinerary to the UI with SSE so it "types out" | L | Visible typing effect |

### C. Data integrations (owner: ____)

| # | Task | Size | Done when |
|---|---|---|---|
| C1 | Sign up for Amadeus Self-Service, set `AMADEUS_KEY/SECRET`; test MCI→DEN flights and Denver hotels | S | `sources.flights == "amadeus"` |
| C2 | Replace the hard-coded `IATA()` map with Amadeus `/v1/reference-data/locations?subType=AIRPORT,CITY` lookup, cached in memory | M | Any US city gets an airport |
| C3 | Mapbox token: geocode arbitrary places, real driving distance/time | S | "Branson, MO" works |
| C4 | Google Places (New) key; verify attractions/food/rest stops; tune `includedTypes` per interest | S | Real place names in plan |
| C5 | EIA key for the real weekly gas price; stretch: use the regional series (PADD) matching the route | S | `sources.gas == "eia"` |
| C6 | Rest stops along the real route: use Mapbox route geometry instead of straight-line interpolation (3–4 points) | M | Stops are on the highway |
| C7 | Rate-limit/caching: in-memory cache keyed by (origin, dest, dates) for 10 min so re-plans and demos don't burn quota | M | Second identical plan returns in < 200 ms |

### D. Planner & value engine (owner: ____)

| # | Task | Size | Done when |
|---|---|---|---|
| D1 | Hotel choice honors budget: pick the best-rated hotel whose total keeps the plan under budget; otherwise cheapest | M | Budget 800 picks a cheaper hotel than budget 2000 |
| D2 | Value score explainability: return `why` strings for each factor (cost/night, travel hours, hotel rating) and show them in the UI | S | UI shows "why this is best value" |
| D3 | Travelers × car: if `travelers > 5` disable drive unless a second car is noted | S | Edge case handled |
| D4 | Multi-day drives: if one-way drive > 9 h, insert an overnight stop city (midpoint from gazetteer/Mapbox) with its own hotel | L | KC→LA plan has a stop night |
| D5 | Per-day budget bar: `total_usd` per day in the plan payload | S | UI can draw a bar |
| D6 | Unit tests for D1 and D4 | S | `go test ./...` green |

### E. Accounts & database (owner: ____)

| # | Task | Size | Done when |
|---|---|---|---|
| E1 | End-to-end: register → login → create trip → plan → view, against real Postgres | S | Manual pass + screenshot |
| E2 | Vehicles table: save the user's car (year/make/model/mpg) from the form; prefill next time | M | Second trip remembers the car |
| E3 | Profile page: name, home city, "use my location" saves `home_lat/lng`; new trip defaults origin to home | M | Origin prefilled |
| E4 | Trip edit (dates/travelers/budget) + re-plan | M | Edit saves and re-plans |
| E5 | Indexes + `updated_at` trigger; `002_*.sql` migration for anything new | S | Migration applies on boot |
| E6 | Security pass: rate-limit auth endpoints (chi `httprate`), JWT secret required in prod (fail boot if default) | S | Default secret refuses to start when `ENV=prod` |

### F. Frontend (owner: ____)

| # | Task | Size | Done when |
|---|---|---|---|
| F1 | Map: Mapbox GL with origin, destination, route line, stop pins, hotel pin (`origin_loc`, `dest_loc`, `stops[].location` already in the payload) | L | Map renders on trip page |
| F2 | Loading experience: step-by-step "pricing flights… asking the AI…" log that advances with real timing | S | Feels alive during the 5–20 s plan |
| F3 | Plan comparison: side-by-side drive vs fly with the value breakdown from D2 | M | |
| F4 | Mobile pass: form and plan readable at 375 px | S | Phone screenshot |
| F5 | Share link: `/share/:id` public read-only plan (new unauthenticated GET endpoint) | M | Judges can open a link |
| F6 | Empty/error states: DB down, AI down, over budget — each has a clear message | S | |
| F7 | Polish: favicon, OG tags, 404 page, keyboard focus states | S | |

### G. Payments — Jack Henry (owner: ____)

| # | Task | Size | Done when |
|---|---|---|---|
| G1 | Get sandbox docs + credentials from the sponsor; identify the exact endpoint for "create payment/transfer" and its auth scheme | S | Written down in `docs/jackhenry.md` |
| G2 | Update `payments/jackhenry.go`: real path, payload, response parsing; keep mock fallback | M | `/api/health` shows `"payments":true`, a charge appears in the sandbox |
| G3 | Linked account: if the sandbox exposes accounts/balances, show the balance on the booking confirm step | M | Balance visible before booking |
| G4 | Booking flow: confirm modal (total, line items, account) → charge → receipt panel → trip status `booked` | M | Demoable end to end |
| G5 | Payments history page (`/app/payments`, endpoint already exists) | S | |
| G6 | Webhook/status poll if the sandbox settles asynchronously | M | `pending` → `settled` updates |

### H. QA, demo, submission (owner: ____)

| # | Task | Size | Done when |
|---|---|---|---|
| H1 | Seed script: demo user + 2 planned trips (`server/cmd/seed`) | S | `go run ./cmd/seed` |
| H2 | Smoke test script hitting every endpoint (`scripts/smoke.sh`) | S | Runs green against prod |
| H3 | Demo script (Section 6) rehearsed twice; timings noted | S | |
| H4 | Fallback plan: if Wi-Fi/APIs die on stage, `MOCK_MODE=true` build ready on a second laptop | S | |
| H5 | Devpost/submission: README, architecture diagram, screenshots, 2-min video | M | Submitted |

---

## 4. Priority

**P0 — the demo does not work without these**
A1, A2, A3, B1, B2, E1, G1, G2, G4, H1, H3

**P1 — makes it convincing**
C1, C3, C4, B3, B4, D1, D2, F1, F2, F3, A4, A5, H2, H5

**P2 — stretch, only if P0/P1 are green**
B5, B7, C2, C5, C6, C7, D3, D4, D5, E2, E3, E4, E6, F4, F5, F6, F7, G3, G5, G6, A6

Rule: nobody starts a P2 while a P0 is red.

---

## 5. Schedule: everything ships today

One day. Split the work by what blocks on outside input (keys, sponsor access, a Linode account) and
what does not. The "no-input" column is being worked by Claude in this session; humans own the
"needs a human" column, which is almost entirely getting access and deploying.

### Needs a human (do these first, they unblock everything)

| Order | Task | Who | Minutes |
|---|---|---|---|
| 1 | **Sponsor tables**: Akamai inference endpoint + key; Jack Henry sandbox URL, auth, "create payment" docs (B1, G1) | ____ | 30 |
| 2 | **Keys**: Amadeus Self-Service, Mapbox, Google Places (New), EIA. Paste into a shared note and into `.env` (C1, C3, C4, C5) | ____ | 30 |
| 3 | **Linode**: create Ubuntu 24.04 2 GB instance, `apt install docker.io docker-compose-v2 git`, clone repo, copy `.env` (A2) | ____ | 30 |
| 4 | First deploy: `docker compose --profile prod up -d --build` on the Linode, open `http://<ip>` (A3) | same | 20 |
| 5 | Run `.\scripts\dev-db.ps1` (Windows) or `docker compose up -d db` locally so you can test accounts (A1) | everyone | 10 |
| 6 | Once G1 docs are in hand: adapt `payments/jackhenry.go` with Claude (G2), test a charge (G4) | ____ | 60 |
| 7 | Rehearse the demo twice (H3), record the video, submit (H5) | all | 90 |

### No outside input needed (in progress now)

| Task | What changes |
|---|---|
| A1 | `scripts/dev-db.ps1`: portable MariaDB, no Docker, no admin |
| E1 | End-to-end account → trip → plan → book verified against real Postgres |
| H1 | `go run ./cmd/seed`: demo user + planned trips |
| H2 | `scripts/smoke.sh`: hits every endpoint, green or red |
| B3, B4 | JSON repair pass + server-side validation so the model can never 500 a plan |
| D1, D2 | Hotel choice honors budget; every option carries a "why" breakdown |
| F2 | Live step log while planning |
| F3 | Side-by-side value breakdown in the UI |
| A5 | `scripts/deploy.sh` for the Linode: pull, rebuild, restart |
| F6 | Clear states for DB down / AI down / over budget |

### Cut for today

D4 overnight stops, B7 streaming, C2 airport lookup (hard-coded map covers the demo cities), C6 route-geometry stops,
F5 share link, G6 webhooks, E2–E4 profile/vehicle/edit. Revisit only if everything above is green.

**Feature freeze two hours before submission.** After that: bugs, copy, rehearsal.

---

## 6. The demo (what we are building toward)

1. **Landing**: type "Kansas City → Nashville, 2 people, $900, Oct 24–27", click *use my location*, look up a 2019 Civic's MPG live. Hit *try it*. Point out the source tags flipping from `mock` to `amadeus` / `google` / `eia`.
2. **The value call**: show drive vs fly cards, the value score, and the "why". Flip the mode to *fly* and show the budget warning.
3. **AI on Akamai**: scroll the itinerary, call out `✦ itinerary by AI on Akamai`, that the model only arranged real prices.
4. **Map**: route line with rest stops and the hotel pin.
5. **Account**: sign up, save the trip, show it on the dashboard.
6. **Book with Jack Henry**: confirm modal → charge in the sandbox → receipt → trip marked `booked`.
7. **Close**: one Linode, one container, model on Akamai, every provider degrades gracefully.

Target: under 4 minutes.

---

## 7. Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Jack Henry sandbox is not a simple REST "pay" call | High | G1 first thing; mock mode already demoable; show the sandbox call in a terminal if the UI path is blocked |
| Akamai inference endpoint not OpenAI-compatible | Medium | `ai.Client` is 150 lines; adapt request/response shapes; Ollama on a GPU Linode as backup |
| Amadeus test env lacks small-airport routes | Medium | C2 city lookup + mock fallback with `source` tag; pick demo cities that work (KC, Denver, Nashville, Chicago all tested in test env) |
| Model returns bad JSON | High | B3 repair pass, B4 validation, fallback itinerary |
| Docker Desktop flaky on Windows | Known | Native Postgres locally; Docker only on Linode |
| Google Places billing/quota | Low | Cache (C7); 20 km radius, ≤ 6 results per call |
| Wi-Fi at the venue | Medium | H4 `MOCK_MODE` backup; everything works offline except AI and live prices |

---

## 8. Conventions

- Branch per task: `feat/<stream><#>-short-name`, e.g. `feat/F1-map`. PR into `main`; one approval; squash.
- `go vet ./... && go test ./...` and `npm run build` must pass before merge.
- Env vars documented in `.env.example` the moment they are added.
- New SQL goes in `server/internal/db/migrations/00N_name.sql`, idempotent (`IF NOT EXISTS`).
- API changes: update the table in `README.md` and the types in `web/src/lib/api.ts` in the same PR.
- No secrets in git. `.env` is ignored; keys live in the Linode `.env` and in a shared password note.
