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
    <div className="grid gap-5">
      <div className="flex items-center justify-between print:hidden">
        <div className="mono text-xs text-muted">shared read-only · <span className="prompt">tpn share</span></div>
        <Link to="/" className="btn-primary !py-1">plan your own →</Link>
      </div>
      <PlanView plan={trip.plan} meta={{ travelers: trip.travelers, budget_usd: trip.budget_usd, status: trip.status }} />
    </div>
  )
}
