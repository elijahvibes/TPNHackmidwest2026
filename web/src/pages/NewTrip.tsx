import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import TripForm, { type TripFormValue } from '../components/TripForm'
import PlanProgress from '../components/PlanProgress'
import { useAuth } from '../lib/auth'

export default function NewTrip() {
  const nav = useNavigate()
  const { user } = useAuth()
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const [step, setStep] = useState('')

  const submit = async (v: TripFormValue) => {
    setBusy(true); setErr('')
    try {
      setStep('saving trip…')
      const trip = await api.createTrip({
        origin: v.origin, destination: v.destination, start_date: v.start_date, end_date: v.end_date,
        travelers: v.travelers, budget_usd: v.budget_usd, mode: v.mode,
      })
      setStep('pricing flights, gas, hotels… asking the AI…')
      // Pinned points from the autocomplete ride along so the planner never re-guesses the place.
      await api.planTrip(trip.id, { vehicle_mpg: v.vehicle_mpg, interests: v.interests, origin_loc: v.origin_loc, dest_loc: v.dest_loc,
        local_transport: v.local_transport, activity_radius_miles: v.activity_radius_miles })
      nav(`/app/trips/${trip.id}`)
    } catch (e) { setErr((e as Error).message); setBusy(false) }
  }

  return (
    <div className="grid gap-6">
      <div>
        <div className="mono text-xs text-muted prompt">tpn new</div>
        <h1 className="mono text-2xl font-bold mt-1">new trip</h1>
        <p className="text-sm text-muted mt-1">Any city, town or park. Flights are priced from the nearest airport; the drive is priced for your car.</p>
      </div>
      <TripForm onSubmit={submit} busy={busy} submitLabel="save & plan" defaultOrigin={user?.home_city} />
      {busy && step === 'saving trip…' && <div className="mono text-sm text-cyan">{step}<span className="caret" /></div>}
      <PlanProgress active={busy && step !== 'saving trip…'} />
      {err && <div className="mono text-sm text-rose">error: {err}</div>}
    </div>
  )
}
