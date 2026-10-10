import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, usd, type Trip } from '../lib/api'
import PlanView from '../components/PlanView'

export default function TripView() {
  const { id = '' } = useParams()
  const [trip, setTrip] = useState<Trip | null>(null)
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState('')
  const [receipt, setReceipt] = useState<{ ref: string | null; mock: boolean } | null>(null)

  const load = () => api.trip(id).then(setTrip).catch(e => setErr((e as Error).message))
  useEffect(() => { load() }, [id])

  const replan = async () => {
    setBusy('re-planning…'); setErr('')
    try { await api.planTrip(id); await load() } catch (e) { setErr((e as Error).message) } finally { setBusy('') }
  }

  const book = async () => {
    if (!trip?.plan) return
    setBusy('charging via Jack Henry…'); setErr('')
    try {
      const r = await api.pay(trip.id, trip.plan.total_usd, `TPN: ${trip.title}`)
      setReceipt({ ref: r.payment.provider_ref, mock: r.mock })
      await load()
    } catch (e) { setErr((e as Error).message) } finally { setBusy('') }
  }

  if (err && !trip) return <div className="mono text-rose">{err}</div>
  if (!trip) return <div className="mono text-muted">loading…</div>

  return (
    <div className="grid gap-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <Link to="/app" className="mono text-xs text-muted hover:text-ink">← trips</Link>
          <h1 className="mono text-2xl font-bold mt-1">{trip.title} <span className="tag ml-2">{trip.status}</span></h1>
        </div>
        <div className="flex gap-2">
          <button className="btn-ghost" onClick={replan} disabled={!!busy}>{trip.plan ? 're-plan' : 'plan'}</button>
          {trip.plan && trip.status !== 'booked' && (
            <button className="btn-primary" onClick={book} disabled={!!busy}>book · {usd(trip.plan.total_usd)}</button>
          )}
        </div>
      </div>
      {busy && <div className="mono text-sm text-cyan">{busy}<span className="caret" /></div>}
      {err && <div className="mono text-sm text-rose">error: {err}</div>}
      {receipt && (
        <div className="panel p-4 border-accent mono text-sm">
          <span className="text-accent">✓ payment settled</span> · ref {receipt.ref}
          {receipt.mock && <span className="text-dim"> · mock mode (set JACKHENRY_BASE_URL to go live)</span>}
        </div>
      )}
      {trip.plan ? <PlanView plan={trip.plan} /> : (
        <div className="panel p-8 text-center mono text-muted">no plan yet. hit <b className="text-ink">plan</b> to price it.</div>
      )}
    </div>
  )
}
