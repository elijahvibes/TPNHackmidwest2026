// Thin typed client for the Go API. Same origin in prod; Vite proxies /api in dev.

export type User = {
  id: string
  email: string
  name: string
  home_city: string | null
  home_lat: number | null
  home_lng: number | null
}

export type LatLng = { lat: number; lng: number }

/** A resolved place: what the user typed turned into a point plus a label to show back. */
export type Placemark = LatLng & {
  label: string; name: string; region: string; country: string; source: string
}

export type Airport = LatLng & { code: string; name: string; city: string; distance_miles: number }

export type FlightQuote = {
  carrier: string; origin: string; destination: string; depart: string; return?: string
  price_usd: number; stops: number; duration_min: number; source: string
  departure_time?: string; arrival_time?: string; flight_numbers?: string[]
  departure_token?: string; booking_token?: string
}
export type FlightSearch = {
  origin: string; destination: string; depart: string; return: string; travelers: number
  departure_token?: string; booking_token?: string
}
export type BookingSeller = {
  book_with: string; price: number; option_title: string; extensions: string[] | null
  baggage_prices: string[] | null; booking_phone?: string
  booking_request: { url: string; post_data?: string }
}
export type BookingOption = {
  separate_tickets: boolean
  together?: BookingSeller; departing?: BookingSeller; returning?: BookingSeller
}
export type HotelQuote = {
  name: string; rating: number; nightly_usd: number; nights: number; total_usd: number
  kind: string; distance_km: number; location: LatLng; area?: string; amenities?: string[]; url?: string; source: string
}
export type DriveEstimate = {
  distance_miles: number; duration_min: number; gallons_needed: number; gas_price_usd: number
  fuel_cost_usd: number; round_trip_usd: number; vehicle_mpg: number; source: string
}
export type Place = {
  name: string; category: string; categories?: string[]; rating: number; price_usd: number; price_unknown?: boolean; location: LatLng; note?: string; source: string
}
export type PlanOption = {
  mode: string; transport_usd: number; lodging_usd: number; activities_usd: number
  total_usd: number; travel_hours: number; value_score: number; why: string; factors: string[]
}
export type DayWeather = { date: string; label: string; high_f: number; low_f: number; short: string; precip_pct: number; icon: string }
export type Weather = { place: string; days: DayWeather[]; note?: string; source: string }
export type Safety = { city: string; violent_per_100k: number; level: 'low' | 'moderate' | 'elevated'; score: number; tips: string[]; source: string }
export type ItemCategory = 'transport' | 'lodging' | 'activity' | 'food' | 'stop'
export type DayItem = { time: string; title: string; detail?: string; cost_usd: number; cost_unknown?: boolean; category: ItemCategory }
export type Day = { day: number; date: string; theme: string; items: DayItem[] }

export type Plan = {
  title: string; summary: string; recommended_mode: string; options: PlanOption[]
  drive: DriveEstimate; flights: FlightQuote[]; hotels: HotelQuote[]
  stops: Place[]; activities: Place[]; food: Place[]; days: Day[]
  total_usd: number; under_budget: boolean; nights: number
  origin_loc: LatLng; dest_loc: LatLng
  origin_place: Placemark; dest_place: Placemark
  origin_airport?: Airport; dest_airport?: Airport
  ai_generated: boolean
  weather: Weather; safety: Safety
  sources: Record<string, string>; generated_at: string
  travelers?: number; budget_usd?: number; selected_flight?: number; selected_return?: FlightQuote
  flight_selected?: boolean; origin_name?: string; destination_name?: string
  local_transport?: 'auto' | 'rental_car' | 'transit_rideshare' | 'walking'; activity_radius_miles?: number
}

export type Trip = {
  id: string; title: string; origin: string; destination: string
  start_date: string; end_date: string; travelers: number; budget_usd: number | null
  mode: string; status: 'draft' | 'planned' | 'booked'; plan?: Plan; created_at: string
}

export type Payment = {
  id: string; trip_id: string | null; provider: string; provider_ref: string | null
  amount_usd: number; status: string; memo: string | null; created_at: string
}

export type PlanRequest = {
  origin: string; destination: string; start_date: string; end_date: string
  travelers: number; budget_usd?: number; mode?: string; vehicle_mpg?: number
  interests?: string[]; origin_loc?: LatLng; dest_loc?: LatLng
  local_transport?: 'auto' | 'rental_car' | 'transit_rideshare' | 'walking'; activity_radius_miles?: number
}

export type PlanExtra = Pick<PlanRequest, 'vehicle_mpg' | 'interests' | 'mode' | 'origin_loc' | 'dest_loc' | 'local_transport' | 'activity_radius_miles'>

const TOKEN_KEY = 'tpn.token'
export const token = {
  get: () => { try { return localStorage.getItem(TOKEN_KEY) } catch { return null } },
  set: (t: string) => { try { localStorage.setItem(TOKEN_KEY, t) } catch { /* ignore */ } },
  clear: () => { try { localStorage.removeItem(TOKEN_KEY) } catch { /* ignore */ } },
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) { super(message); this.status = status }
}

async function request<T>(method: string, path: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  const t = token.get()
  if (t) headers.Authorization = `Bearer ${t}`
  const res = await fetch(`/api${path}`, { method, headers, body: body === undefined ? undefined : JSON.stringify(body), signal })
  if (res.status === 204) return undefined as T
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    const msg = (data as { error?: string }).error ?? res.statusText
    // Friendlier copy for the two outages a demo can hit.
    if (res.status === 503) throw new ApiError(503, 'accounts are offline right now (database unreachable). The planner on the home page still works.')
    throw new ApiError(res.status, msg)
  }
  return data as T
}

export const api = {
  health: () => request<{ ok: boolean; db: boolean; ai: boolean; serpapi: boolean; payments: boolean }>('GET', '/health'),

  register: (email: string, name: string, password: string) =>
    request<{ token: string; user: User }>('POST', '/auth/register', { email, name, password }),
  login: (email: string, password: string) =>
    request<{ token: string; user: User }>('POST', '/auth/login', { email, password }),
  me: () => request<User>('GET', '/auth/me'),
  updateMe: (patch: Partial<Pick<User, 'home_city' | 'home_lat' | 'home_lng'>>) =>
    request<User>('PATCH', '/auth/me', patch),

  vehicleMPG: (year: number, make: string, model: string) =>
    request<{ mpg: number; source: string; note?: string }>(
      'GET', `/vehicles/mpg?year=${year}&make=${encodeURIComponent(make)}&model=${encodeURIComponent(model)}`),

  /** City/town/park autocomplete. Any place, not just the demo list. */
  geoSuggest: (q: string, signal?: AbortSignal) =>
    request<Placemark[]>('GET', `/geo/suggest?q=${encodeURIComponent(q)}`, undefined, signal),

  planPreview: (req: PlanRequest) => request<Plan>('POST', '/plan/preview', req),
  selectPreviewFlight: (plan: Plan, flight_index: number, return_token?: string) =>
    request<Plan>('POST', '/plan/flight', { plan, flight_index, return_token }),
  returnFlights: (search: FlightSearch) => request<FlightQuote[]>('POST', '/flights/returns', search),
  flightBookings: (search: FlightSearch) => request<BookingOption[]>('POST', '/flights/bookings', search),
  selectTripFlight: (id: string, flight_index: number, return_token?: string) =>
    request<Plan>('POST', `/trips/${id}/flight`, { flight_index, return_token }),

  share: (id: string) => request<Trip>('GET', `/share/${id}`),

  trips: () => request<Trip[]>('GET', '/trips'),
  trip: (id: string) => request<Trip>('GET', `/trips/${id}`),
  createTrip: (t: Omit<PlanRequest, 'origin_loc' | 'dest_loc' | 'vehicle_mpg' | 'interests'> & { title?: string }) =>
    request<Trip>('POST', '/trips', t),
  deleteTrip: (id: string) => request<void>('DELETE', `/trips/${id}`),
  planTrip: (id: string, extra?: PlanExtra) =>
    request<Plan>('POST', `/trips/${id}/plan`, extra ?? {}),
  chooseHotel: (id: string, name: string) =>
    request<Plan>('POST', `/trips/${id}/hotel`, { name }),

  payments: () => request<Payment[]>('GET', '/payments'),
  pay: (trip_id: string, amount_usd: number, memo: string) =>
    request<{ payment: Payment; mock: boolean }>('POST', '/payments', { trip_id, amount_usd, memo }),
}

export const usd = (n: number) => n.toLocaleString('en-US', { style: 'currency', currency: 'USD', maximumFractionDigits: 0 })
export const hrs = (min: number) => `${Math.floor(min / 60)}h ${min % 60}m`

/** "2026-11-06" → "Fri, Nov 6" (no timezone surprises: parse as local date). */
export const fmtDate = (iso: string, opts: Intl.DateTimeFormatOptions = { weekday: 'short', month: 'short', day: 'numeric' }) => {
  const [y, m, d] = iso.split('-').map(Number)
  if (!y || !m || !d) return iso
  return new Date(y, m - 1, d).toLocaleDateString('en-US', opts)
}

/**
 * Client-side version of the server's ChooseHotel, for the stateless landing
 * preview (nothing to persist there). Saved trips call api.chooseHotel instead.
 */
export function chooseHotelLocally(plan: Plan, name: string): Plan {
  const idx = plan.hotels.findIndex(h => h.name === name)
  if (idx < 0) return plan
  const h = plan.hotels[idx]
  const hotels = [h, ...plan.hotels.filter((_, i) => i !== idx)]
  const options = plan.options.map(o => {
    const total = Math.round((o.transport_usd + h.total_usd + o.activities_usd) * 100) / 100
    const perNight = total / plan.nights
    return {
      ...o, lodging_usd: h.total_usd, total_usd: total,
      value_score: Math.round((10000 / (perNight + 50) - o.travel_hours * 0.6) * 10) / 10,
      factors: o.factors.map(f => f.endsWith('per night all-in') ? `$${perNight.toFixed(0)} per night all-in`
        : f.includes('/night') ? `${h.name} ★${h.rating.toFixed(1)} at $${h.nightly_usd.toFixed(0)}/night` : f),
    }
  })
  const rec = options.find(o => o.mode === plan.recommended_mode) ?? options[0]
  const days = plan.days.map(d => ({
    ...d,
    items: d.items.map(it => it.category === 'lodging'
      ? { ...it, title: `Check in: ${h.name}`, detail: `${h.nights} nights at $${h.nightly_usd.toFixed(0)}/night · ${h.area ?? ''}`, cost_usd: h.total_usd }
      : it),
  }))
  const base = plan.summary.split(' Stay changed to ')[0]
  const summary = `${base} Stay changed to ${h.name}; total now $${rec.total_usd.toFixed(0)}.`
  return { ...plan, hotels, options, days, summary, total_usd: rec.total_usd }
}
