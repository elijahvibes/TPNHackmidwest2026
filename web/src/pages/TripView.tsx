import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, usd, type Trip } from '../lib/api'
import PlanView from '../components/PlanView'
import PlanProgress from '../components/PlanProgress'

export default function TripView() {
  const { id = '' } = useParams()
  const [trip, setTrip] = useState<Trip | null>(null)
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState('')
  const [receipt, setReceipt] = useState<{ ref: string | null; mock: boolean } | null>(null)
  const [copied, setCopied] = useState(false)

  const load = () => api.trip(id).then(setTrip).catch(e => setErr((e as Error).message))
  useEffect(() => { load() }, [id])

  const replan = async (mode?: string) => {
    setBusy(mode ? `re-planning as ${mode}…` : 're-planning…'); setErr('')
    try { await api.planTrip(id, mode ? { mode } : undefined); await load() } catch (e) { setErr((e as Error).message) } finally { setBusy('') }
  }

  const chooseHotel = async (name: string) => {
    setErr('')
    try {
      const plan = await api.chooseHotel(id, name)
      setTrip(t => t ? { ...t, plan } : t)
    } catch (e) { setErr((e as Error).message) }
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

  const share = async () => {
    if (!trip) return
    const url = `${location.origin}/share/${trip.id}`
    try { await navigator.clipboard.writeText(url); setCopied(true); setTimeout(() => setCopied(false), 2000) } catch { prompt('Share link', url) }
  }

  if (err && !trip) return <div className="mono text-rose">{err}</div>
  if (!trip) return <div className="mono text-muted">loading…</div>

  const planning = busy.startsWith('re-planning')
  const actions = (
    <>
      {trip.plan && <button className="btn-ghost !py-1" onClick={share}>{copied ? '✓ copied' : 'share'}</button>}
      <button className="btn-ghost !py-1" onClick={() => replan()} disabled={!!busy}>{trip.plan ? 're-plan' : 'plan'}</button>
      {trip.plan && trip.status !== 'booked' && (
        <button className="btn-primary !py-1" onClick={book} disabled={!!busy}>book · {usd(trip.plan.total_usd)}</button>
      )}
    </>
  )

  return (
    <div className="grid gap-5">
      <div className="flex items-center justify-between print:hidden">
        <Link to="/app" className="mono text-xs text-muted hover:text-ink">← trips</Link>
        {busy && !planning && <div className="mono text-sm text-cyan">{busy}<span className="caret" /></div>}
      </div>
      {planning && <PlanProgress active />}
      {err && <div className="mono text-sm text-rose">error: {err}</div>}
      {receipt && (
        <div className="panel p-4 border-accent mono text-sm">
          <span className="text-accent">✓ payment settled</span> · ref {receipt.ref}
          {receipt.mock && <span className="text-dim"> · mock mode (set JACKHENRY_BASE_URL to go live)</span>}
        </div>
      )}
      {trip.plan ? (
        <PlanView plan={trip.plan} actions={actions} busy={!!busy} onChooseHotel={chooseHotel} onReplan={m => replan(m)}
          meta={{ travelers: trip.travelers, budget_usd: trip.budget_usd, status: trip.status }} />
      ) : (
        <div className="panel p-8 text-center grid gap-3 justify-items-center">
          <h1 className="mono text-2xl font-bold">{trip.title}</h1>
          <div className="mono text-xs text-muted">{trip.start_date} → {trip.end_date} · {trip.travelers} traveler{trip.travelers > 1 ? 's' : ''}</div>
          <div className="mono text-muted">no plan yet.</div>
          <button className="btn-primary" onClick={() => replan()} disabled={!!busy}>price &amp; plan it →</button>
        </div>
      )}
    </div>
  )
}
