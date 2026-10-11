import { useCallback, useEffect, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { api, type Trip } from '../lib/api'
import { usePlanJob } from '../lib/usePlanJob'
import PlanView from '../components/PlanView'
import PlanProgress from '../components/PlanProgress'
import Icon from '../components/Icon'

export default function TripView() {
  const { id = '' } = useParams()
  const [params, setParams] = useSearchParams()
  const [trip, setTrip] = useState<Trip | null>(null)
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState('')
  const [copied, setCopied] = useState(false)

  const load = useCallback(() => api.trip(id).then(setTrip).catch(e => setErr((e as Error).message)), [id])
  useEffect(() => { load() }, [load])

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

  const share = async () => {
    if (!trip) return
    const url = `${location.origin}/share/${trip.id}`
    try { await navigator.clipboard.writeText(url); setCopied(true); setTimeout(() => setCopied(false), 2000) } catch { prompt('Share link', url) }
  }

  const selectFlight = async (index: number, returnToken?: string) => {
    setBusy('Saving flight selection'); setErr('')
    try {
      const plan = await api.selectTripFlight(id, index, returnToken)
      setTrip(current => current ? { ...current, plan } : current)
    } finally { setBusy('') }
  }

  if (err && !trip) return <div className="text-rose">{err}</div>
  if (!trip) return <div className="text-muted">Loading…</div>

  const planning = job.running
  // While a job runs, show its latest draft in place of the saved plan.
  const shownPlan = planning && job.plan ? job.plan : trip.plan
  const locked = !!busy || planning
  const actions = (
    <>
      {trip.plan && <button className="btn-ghost !py-1.5" onClick={share}><Icon name="share" size={14} />{copied ? 'Copied' : 'Share'}</button>}
      <button className="btn-ghost !py-1.5" onClick={() => replan()} disabled={locked}>{trip.plan ? 'Re-plan' : 'Plan'}</button>
    </>
  )

  return (
    <div className="grid gap-6">
      <div className="flex items-center justify-between print:hidden">
        <Link to="/app" className="text-[13px] text-muted hover:text-ink">← My trips</Link>
        {busy && <div className="text-sm text-muted">{busy}<span className="caret" /></div>}
      </div>
      {planning && <PlanProgress active events={job.events} stage={job.job?.stage} compact={!!job.plan} />}
      {err && <div className="text-sm text-rose">{err}</div>}
      {shownPlan ? (
        <PlanView plan={shownPlan} actions={actions} busy={locked} refining={planning} onChooseHotel={planning ? undefined : chooseHotel} onReplan={m => replan(m)}
          onSelectFlight={planning ? undefined : selectFlight} disabled={locked}
          meta={{ travelers: trip.travelers, budget_usd: trip.budget_usd, status: trip.status }} />
      ) : (
        <div className="panel p-10 text-center grid gap-3 justify-items-center">
          <h1 className="text-[32px]">{trip.title}</h1>
          <div className="text-[13px] text-muted">{trip.start_date} → {trip.end_date} · {trip.travelers} traveler{trip.travelers > 1 ? 's' : ''}</div>
          {planning ? <div className="text-muted">Pricing flights, stays and gas<span className="caret" /></div> : (
            <>
              <div className="text-muted">No plan yet.</div>
              <button className="btn-primary" onClick={() => replan()} disabled={locked}>Price &amp; plan it<Icon name="arrow" size={15} /></button>
            </>
          )}
        </div>
      )}
    </div>
  )
}
