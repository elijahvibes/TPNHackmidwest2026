import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, chooseHotelLocally, type Plan, type PlanRequest } from '../lib/api'
import { usePlanJob } from '../lib/usePlanJob'
import TripForm, { type TripFormValue } from '../components/TripForm'
import PlanView from '../components/PlanView'
import PlanProgress from '../components/PlanProgress'
import Hero from '../components/Hero'
import { useAuth } from '../lib/auth'

export default function Landing() {
  const { user, loading } = useAuth()
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

  if (loading) return <div className="text-muted p-8">Loading your session…</div>
  if (!user) return <Welcome />

  return (
    <div className="grid gap-8">
      <Hero eyebrow="A good trip starts here" title="A plan that fits."
        lede="The whole getaway, thoughtfully planned around your budget. Flights, gas for your car, stays and things to do, priced together and ranked by value." />

      <TripForm onSubmit={run} busy={busy} submitLabel="Preview my trip" defaultOrigin={user.home_city} />
      <PlanProgress active={busy} events={job.events} stage={job.job?.stage} compact={!!plan} />
      {err && <div className="text-sm text-rose">{err}</div>}
      {plan && (
        <div ref={resultRef} className="grid gap-6 scroll-mt-24">
          <PlanView plan={plan} meta={{ travelers: req?.travelers, budget_usd: req?.budget_usd ?? null }} refining={job.running}
            onSelectFlight={job.running ? undefined : selectFlight} disabled={busy || selecting}
            onChooseHotel={job.running ? undefined : name => setPlan(p => p ? chooseHotelLocally(p, name) : p)} />
          <div className="panel p-5 text-sm text-muted">Save this as a trip from <Link to="/app/new" className="text-accent underline underline-offset-2">Plan a trip</Link>.</div>
        </div>
      )}
    </div>
  )
}

/** Signed-out front door: planning needs an account, so the only way in is to sign in or sign up. */
function Welcome() {
  return (
    <div className="grid gap-8">
      <Hero eyebrow="A good trip starts here" title="A plan that fits."
        lede="The whole getaway, thoughtfully planned around your budget. Flights, gas for your car, stays and things to do, priced together and ranked by value." />
      <div className="panel p-8 sm:p-10 grid gap-5 max-w-xl">
        <div>
          <h2 className="text-[26px]">Sign in to start planning</h2>
          <p className="text-[15px] text-muted mt-2">Trips, stays and flight picks are saved to your account, so every plan is yours to come back to.</p>
        </div>
        <div className="flex flex-wrap gap-3">
          <Link to="/register" className="btn-primary">Create an account</Link>
          <Link to="/login" className="btn-ghost">Log in</Link>
        </div>
      </div>
    </div>
  )
}
