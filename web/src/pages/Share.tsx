import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, type Trip } from '../lib/api'
import PlanView from '../components/PlanView'

/** Public, read-only view of a planned trip. No login required. */
export default function Share() {
  const { id = '' } = useParams()
  const [trip, setTrip] = useState<Trip | null>(null)
  const [err, setErr] = useState('')
  useEffect(() => { api.share(id).then(setTrip).catch(e => setErr((e as Error).message)) }, [id])

  if (err) return <div className="panel p-8 text-center mono text-rose">{err}</div>
  if (!trip?.plan) return <div className="mono text-muted">loading…</div>
  return (
    <div className="grid gap-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <div className="mono text-xs text-muted prompt">tpn share</div>
          <h1 className="mono text-2xl font-bold mt-1">{trip.title}</h1>
          <div className="mono text-xs text-muted mt-1">{trip.start_date} → {trip.end_date} · {trip.travelers} traveler{trip.travelers > 1 ? 's' : ''} · shared read-only</div>
        </div>
        <Link to="/" className="btn-primary">plan your own →</Link>
      </div>
      <PlanView plan={trip.plan} />
    </div>
  )
}
