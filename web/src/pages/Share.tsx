import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, type Trip } from '../lib/api'
import PlanView from '../components/PlanView'
import Icon from '../components/Icon'

/** Public, read-only view of a planned trip. No login required. */
export default function Share() {
  const { id = '' } = useParams()
  const [trip, setTrip] = useState<Trip | null>(null)
  const [err, setErr] = useState('')
  useEffect(() => { api.share(id).then(setTrip).catch(e => setErr((e as Error).message)) }, [id])

  if (err) return <div className="panel p-8 text-center text-rose">{err}</div>
  if (!trip?.plan) return <div className="text-muted">Loading…</div>
  return (
    <div className="grid gap-6">
      <div className="flex items-center justify-between print:hidden">
        <div className="text-[13px] text-muted">Shared trip · read only</div>
        <Link to="/" className="btn-primary !py-1.5">Plan your own<Icon name="arrow" size={14} /></Link>
      </div>
      <PlanView plan={trip.plan} meta={{ travelers: trip.travelers, budget_usd: trip.budget_usd, status: trip.status }} />
    </div>
  )
}
