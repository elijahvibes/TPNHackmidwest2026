import { useCallback, useEffect, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { api, usd, type Trip } from '../lib/api'
import { usePlanJob } from '../lib/usePlanJob'
import PlanView from '../components/PlanView'
import PlanProgress from '../components/PlanProgress'
import DataWarning from '../components/DataWarning'

export default function TripView() {
  const { id = '' } = useParams()
  const [params, setParams] = useSearchParams()
  const [trip, setTrip] = useState<Trip | null>(null)
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState('')
  const [receipt, setReceipt] = useState<{ ref: string | null; mock: boolean } | null>(null)
  const [copied, setCopied] = useState(false)
  const [paymentsLive, setPaymentsLive] = useState<boolean | null>(null)

  const load = useCallback(() => api.trip(id).then(setTrip).catch(e => setErr((e as Error).message)), [id])
  useEffect(() => { load() }, [load])
  useEffect(() => {
    api.health().then(h => setPaymentsLive(h.payments)).catch(e => setErr((e as Error).message))
  }, [])

  // Planning is a background job (?job=…): the draft shows within seconds and
  // the AI agents' progress streams in; on completion the saved trip reloads.
  const jobId = params.get('job')
  const job = usePlanJob(jobId, () => {
    load().then(() => { params.delete('job'); setParams(params, { replace: true }) })
  })
  useEffect(() => { if (job.error) setErr(job.error) }, [job.error])

  const replan = async (mode?: string) => {
    setErr('')
    try {
      const { job_id } = await api.startTripPlan(id, mode ? { mode } : undefined)
      params.set('job', job_id); setParams(params, { replace: true })
    } catch (e) { setErr((e as Error).message) }
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

  const selectFlight = async (index: number, returnToken?: string) => {
    setBusy('saving flight selection…'); setErr('')
    try {
      const plan = await api.selectTripFlight(id, index, returnToken)
      setTrip(current => current ? { ...current, plan } : current)
    } finally { setBusy('') }
  }

  if (err && !trip) return <div className="mono text-rose">{err}</div>
  if (!trip) return <div className="mono text-muted">loading…</div>

  const planning = job.running
  // While a job runs, show its latest draft in place of the saved plan.
  const shownPlan = planning && job.plan ? job.plan : trip.plan
  const locked = !!busy || planning
  const actions = (
    <>
      {trip.plan && <button className="btn-ghost !py-1" onClick={share}>{copied ? '✓ copied' : 'share'}</button>}
      <button className="btn-ghost !py-1" onClick={() => replan()} disabled={locked}>{trip.plan ? 're-plan' : 'plan'}</button>
      {trip.plan && trip.status !== 'booked' && (
        <button className="btn-primary !py-1" onClick={book} disabled={locked}>book · {usd(trip.plan.total_usd)}</button>
      )}
    </>
  )

  return (
    <div className="grid gap-5">
      <div className="flex items-center justify-between print:hidden">
        <Link to="/app" className="mono text-xs text-muted hover:text-ink">← trips</Link>
        {busy && <div className="mono text-sm text-cyan">{busy}<span className="caret" /></div>}
      </div>
      {planning && <PlanProgress active events={job.events} stage={job.job?.stage} compact={!!job.plan} />}
      {err && <div className="mono text-sm text-rose">error: {err}</div>}
      <DataWarning details={receipt?.mock
        ? ['Payment: simulated settlement. No money moved, and no flight or hotel was reserved.']
        : paymentsLive === false && trip.status !== 'booked'
          ? ['Payments: mock mode is on. The book button simulates a payment. It moves no money and reserves no travel.']
          : []} />
      {receipt && (
        <div className="panel p-4 border-accent mono text-sm">
          <span className="text-accent">✓ payment settled</span> · ref {receipt.ref}
          {receipt.mock && <span className="text-rose font-bold"> · simulated payment, no real charge</span>}
        </div>
      )}
      {shownPlan ? (
        <PlanView plan={shownPlan} actions={actions} busy={locked} refining={planning} onChooseHotel={planning ? undefined : chooseHotel} onReplan={m => replan(m)}
          onSelectFlight={planning ? undefined : selectFlight} disabled={locked || trip.status === 'booked'}
          meta={{ travelers: trip.travelers, budget_usd: trip.budget_usd, status: trip.status }} />
      ) : (
        <div className="panel p-8 text-center grid gap-3 justify-items-center">
          <h1 className="mono text-2xl font-bold">{trip.title}</h1>
          <div className="mono text-xs text-muted">{trip.start_date} → {trip.end_date} · {trip.travelers} traveler{trip.travelers > 1 ? 's' : ''}</div>
          {planning ? <div className="mono text-muted">pricing flights, stays and gas…</div> : (
            <>
              <div className="mono text-muted">no plan yet.</div>
              <button className="btn-primary" onClick={() => replan()} disabled={locked}>price &amp; plan it →</button>
            </>
          )}
        </div>
      )}
    </div>
  )
}
