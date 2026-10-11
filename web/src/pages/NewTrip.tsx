import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import TripForm, { type TripFormValue } from '../components/TripForm'
import { useAuth } from '../lib/auth'

export default function NewTrip() {
  const nav = useNavigate()
  const { user } = useAuth()
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')

  const submit = async (v: TripFormValue) => {
    setBusy(true); setErr('')
    try {
      const trip = await api.createTrip({
        origin: v.origin, destination: v.destination, start_date: v.start_date, end_date: v.end_date,
        travelers: v.travelers, budget_usd: v.budget_usd, mode: v.mode,
      })
      // Planning runs in the background; the trip page shows the draft within
      // seconds and streams the AI agents' progress while they refine it.
      // Pinned points from the autocomplete ride along so the planner never re-guesses the place.
      const { job_id } = await api.startTripPlan(trip.id, { vehicle_mpg: v.vehicle_mpg, tank_gallons: v.tank_gallons, interests: v.interests, origin_loc: v.origin_loc, dest_loc: v.dest_loc,
        local_transport: v.local_transport, activity_radius_miles: v.activity_radius_miles })
      nav(`/app/trips/${trip.id}?job=${job_id}`)
    } catch (e) { setErr((e as Error).message); setBusy(false) }
  }

  return (
    <div className="grid gap-6">
      <div>
        <div className="mono text-xs text-muted prompt">tpn new</div>
        <h1 className="mono text-2xl font-bold mt-1">new trip</h1>
        <p className="text-sm text-muted mt-1">Enter any city, town or park. TPN prices flights from the nearest airport and the drive for your car.</p>
      </div>
      <TripForm onSubmit={submit} busy={busy} submitLabel="save & plan" defaultOrigin={user?.home_city} />
      {busy && <div className="mono text-sm text-cyan">saving trip and starting the planner…<span className="caret" /></div>}
      {err && <div className="mono text-sm text-rose">error: {err}</div>}
    </div>
  )
}
