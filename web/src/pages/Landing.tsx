import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, chooseHotelLocally, type Plan, type PlanRequest } from '../lib/api'
import { usePlanJob } from '../lib/usePlanJob'
import TripForm, { type TripFormValue } from '../components/TripForm'
import PlanView from '../components/PlanView'
import PlanProgress from '../components/PlanProgress'
import { useAuth } from '../lib/auth'

const PERKS = [
  ['any city', 'type a town, a park or a lake. Flights come from its nearest airport'],
  ['your car', 'gas priced from your MPG and this week’s pump price'],
  ['real stays', 'hotels near the destination. Swap one and the total updates'],
  ['drive vs fly', 'one value score weighs price against travel time'],
]

export default function Landing() {
  const { user } = useAuth()
  const [plan, setPlan] = useState<Plan | null>(null)
  const [req, setReq] = useState<PlanRequest | null>(null)
  const [jobId, setJobId] = useState<string | null>(null)
  const [starting, setStarting] = useState(false)
  const [err, setErr] = useState('')
  const resultRef = useRef<HTMLDivElement>(null)

  // Planning is a background job: the rule-based draft (real prices) shows
  // within seconds, and the AI agents' version replaces it when they finish.
  const job = usePlanJob(jobId, final => { setPlan(final); setJobId(null) })
  useEffect(() => { if (job.plan && job.running) setPlan(job.plan) }, [job.plan, job.running])
  useEffect(() => { if (job.error) { setErr(job.error); setJobId(null) } }, [job.error])
  useEffect(() => {
    if (plan) setTimeout(() => resultRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' }), 50)
  }, [plan === null]) // eslint-disable-line react-hooks/exhaustive-deps -- scroll once when the first draft lands
  const busy = starting || job.running

  const run = async (v: TripFormValue) => {
    setStarting(true); setErr(''); setPlan(null); setReq(v)
    try {
      const { job_id } = await api.startPreview(v)
      setJobId(job_id)
    } catch (e) { setErr((e as Error).message) } finally { setStarting(false) }
  }

  const [selecting, setSelecting] = useState(false)
  const selectFlight = async (index: number, returnToken?: string) => {
    if (!plan) return
    setSelecting(true)
    try { setPlan(await api.selectPreviewFlight(plan, index, returnToken)) }
    finally { setSelecting(false) }
  }

  return (
    <div className="grid gap-8">
      <section className="pt-4 grid lg:grid-cols-[1.2fr_1fr] gap-8 items-end">
        <div>
          <div className="mono text-xs text-muted prompt">tpn plan --from "anywhere" --to "anywhere" --optimize value</div>
          <h1 className="mono text-4xl sm:text-5xl font-bold mt-3 leading-tight">
            Plan the whole trip<br /><span className="text-accent">for the least money.</span><span className="caret" />
          </h1>
          <p className="text-muted mt-4 max-w-xl">
            TPN prices flights, gas for <em>your</em> car, hotels, activities and stops on the way together,
            then ranks the options by value. An AI running on Akamai writes the day-by-day itinerary.
          </p>
          {!user && (
            <div className="mt-4 flex gap-3">
              <Link to="/register" className="btn-primary">create account</Link>
              <Link to="/login" className="btn-ghost">login</Link>
            </div>
          )}
        </div>
        <ul className="grid grid-cols-2 gap-3">
          {PERKS.map(([k, v]) => (
            <li key={k} className="panel p-3">
              <div className="mono text-xs text-accent">{k}</div>
              <div className="text-[13px] text-muted mt-1 leading-snug">{v}</div>
            </li>
          ))}
        </ul>
      </section>

      <TripForm onSubmit={run} busy={busy} submitLabel={user ? 'preview plan' : 'try it'} defaultOrigin={user?.home_city} />
      <PlanProgress active={busy} events={job.events} stage={job.job?.stage} compact={!!plan} />
      {err && <div className="mono text-sm text-rose">error: {err}</div>}
      {plan && (
        <div ref={resultRef} className="grid gap-6 scroll-mt-20">
          <PlanView plan={plan} meta={{ travelers: req?.travelers, budget_usd: req?.budget_usd ?? null }} refining={job.running}
            onSelectFlight={job.running ? undefined : selectFlight} disabled={busy || selecting}
            onChooseHotel={job.running ? undefined : name => setPlan(p => p ? chooseHotelLocally(p, name) : p)} />
          {!user && <div className="panel p-4 mono text-sm text-muted">Like it? <Link to="/register" className="text-accent">create an account</Link> to save trips, swap hotels for good, and book with Jack Henry.</div>}
          {user && <div className="panel p-4 mono text-sm text-muted">Save this as a trip from <Link to="/app/new" className="text-accent">+ new</Link>.</div>}
        </div>
      )}
    </div>
  )
}
