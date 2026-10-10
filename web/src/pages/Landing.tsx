import { useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, chooseHotelLocally, type Plan, type PlanRequest } from '../lib/api'
import TripForm, { type TripFormValue } from '../components/TripForm'
import PlanView from '../components/PlanView'
import PlanProgress from '../components/PlanProgress'
import { useAuth } from '../lib/auth'

const PERKS = [
  ['any city', 'type a town, a park, a lake — flights come from its nearest airport'],
  ['your car', 'gas priced from your actual MPG and this week’s pump price'],
  ['real stays', 'hotels near the destination, swappable, re-totaled live'],
  ['drive vs fly', 'one value score so the cheap option is also the sane one'],
]

export default function Landing() {
  const { user } = useAuth()
  const [plan, setPlan] = useState<Plan | null>(null)
  const [req, setReq] = useState<PlanRequest | null>(null)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const resultRef = useRef<HTMLDivElement>(null)

  const run = async (v: TripFormValue) => {
    setBusy(true); setErr(''); setPlan(null); setReq(v)
    try {
      const p = await api.planPreview(v)
      setPlan(p)
      setTimeout(() => resultRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' }), 50)
    } catch (e) { setErr((e as Error).message) } finally { setBusy(false) }
  }

  const selectFlight = async (index: number, returnToken?: string) => {
    if (!plan) return
    setBusy(true)
    try { setPlan(await api.selectPreviewFlight(plan, index, returnToken)) }
    finally { setBusy(false) }
  }

  return (
    <div className="grid gap-8">
      <section className="pt-4 grid lg:grid-cols-[1.2fr_1fr] gap-8 items-end">
        <div>
          <div className="mono text-xs text-muted prompt">tpn plan --from "anywhere" --to "anywhere" --optimize value</div>
          <h1 className="mono text-4xl sm:text-5xl font-bold mt-3 leading-tight">
            Plan the whole trip.<br /><span className="text-accent">Pay the least for it.</span><span className="caret" />
          </h1>
          <p className="text-muted mt-4 max-w-xl">
            Flights, gas for <em>your</em> car, hotels, things to do, and stops on the way, priced together and
            ranked by value. The day-by-day itinerary is written by an AI running on Akamai.
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
      <PlanProgress active={busy} />
      {err && <div className="mono text-sm text-rose">error: {err}</div>}
      {plan && (
        <div ref={resultRef} className="grid gap-6 scroll-mt-20">
          <PlanView plan={plan} meta={{ travelers: req?.travelers, budget_usd: req?.budget_usd ?? null }} onSelectFlight={selectFlight} disabled={busy}
            onChooseHotel={name => setPlan(p => p ? chooseHotelLocally(p, name) : p)} />
          {!user && <div className="panel p-4 mono text-sm text-muted">Like it? <Link to="/register" className="text-accent">create an account</Link> to save trips, swap hotels for good, and book with Jack Henry.</div>}
          {user && <div className="panel p-4 mono text-sm text-muted">Save this as a trip from <Link to="/app/new" className="text-accent">+ new</Link>.</div>}
        </div>
      )}
    </div>
  )
}
