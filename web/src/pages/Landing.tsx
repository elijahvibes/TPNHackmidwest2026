import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type Plan } from '../lib/api'
import TripForm, { type TripFormValue } from '../components/TripForm'
import PlanView from '../components/PlanView'
import { useAuth } from '../lib/auth'

export default function Landing() {
  const { user } = useAuth()
  const [plan, setPlan] = useState<Plan | null>(null)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')

  const run = async (v: TripFormValue) => {
    setBusy(true); setErr(''); setPlan(null)
    try { setPlan(await api.planPreview(v)) } catch (e) { setErr((e as Error).message) } finally { setBusy(false) }
  }

  const selectFlight = async (index: number, returnToken?: string) => {
    if (!plan) return
    setBusy(true)
    try { setPlan(await api.selectPreviewFlight(plan, index, returnToken)) }
    finally { setBusy(false) }
  }

  return (
    <div className="grid gap-8">
      <section className="pt-6">
        <div className="mono text-xs text-muted prompt">trailmix plan --from "home" --to "anywhere" --optimize value</div>
        <h1 className="mono text-4xl sm:text-5xl font-bold mt-3 leading-tight">
          Plan the whole trip.<br /><span className="text-accent">Pay the least for it.</span><span className="caret" />
        </h1>
        <p className="text-muted mt-4 max-w-xl">
          Flights, gas for <em>your</em> car, hotels, things to do, and stops on the way, priced together and
          ranked by value. The itinerary is written by an AI running on Akamai.
        </p>
        {!user && (
          <div className="mt-4 flex gap-3">
            <Link to="/register" className="btn-primary">create account</Link>
            <Link to="/login" className="btn-ghost">login</Link>
          </div>
        )}
      </section>

      <TripForm onSubmit={run} busy={busy} submitLabel={user ? 'preview plan' : 'try it'} />
      {err && <div className="mono text-sm text-rose">error: {err}</div>}
      {plan && (
        <>
          <PlanView plan={plan} onSelectFlight={selectFlight} disabled={busy} />
          {!user && <div className="panel p-4 mono text-sm text-muted">Like it? <Link to="/register" className="text-accent">create an account</Link> to save trips and book with Jack Henry.</div>}
          {user && <div className="panel p-4 mono text-sm text-muted">Save this as a trip from <Link to="/app/new" className="text-accent">+ new</Link>.</div>}
        </>
      )}
    </div>
  )
}
