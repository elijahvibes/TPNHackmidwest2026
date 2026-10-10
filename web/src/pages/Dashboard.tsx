import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, usd, type Trip } from '../lib/api'
import { useAuth } from '../lib/auth'

const statusColor: Record<Trip['status'], string> = { draft: 'text-dim', planned: 'text-cyan', booked: 'text-accent' }

export default function Dashboard() {
  const { user } = useAuth()
  const [trips, setTrips] = useState<Trip[] | null>(null)
  const [err, setErr] = useState('')

  const load = () => api.trips().then(setTrips).catch(e => setErr((e as Error).message))
  useEffect(() => { load() }, [])

  const remove = async (id: string) => {
    if (!confirm('Delete this trip?')) return
    await api.deleteTrip(id); load()
  }

  return (
    <div className="grid gap-6">
      <div className="flex items-end justify-between">
        <div>
          <div className="mono text-xs text-muted prompt">ls ~/trips</div>
          <h1 className="mono text-2xl font-bold mt-1">hey {user?.name.split(' ')[0]}</h1>
        </div>
        <Link to="/app/new" className="btn-primary">+ new trip</Link>
      </div>
      {err && <div className="mono text-sm text-rose">{err}</div>}
      {trips === null ? <div className="mono text-muted">loading…</div> : trips.length === 0 ? (
        <div className="panel p-8 text-center">
          <div className="mono text-muted">no trips yet.</div>
          <Link to="/app/new" className="btn-primary mt-4">plan your first one →</Link>
        </div>
      ) : (
        <ul className="grid sm:grid-cols-2 gap-4">
          {trips.map(t => (
            <li key={t.id} className="panel p-4 hover:border-accent transition-colors">
              <Link to={`/app/trips/${t.id}`} className="block">
                <div className="flex justify-between items-start gap-2">
                  <h3 className="mono font-bold">{t.title}</h3>
                  <span className={`tag ${statusColor[t.status]}`}>{t.status}</span>
                </div>
                <div className="mono text-xs text-muted mt-1">{t.start_date} → {t.end_date} · {t.travelers} traveler{t.travelers > 1 ? 's' : ''}</div>
                <div className="mono text-sm mt-3 flex justify-between">
                  <span className="text-muted">{t.plan ? `best: ${t.plan.recommended_mode}` : 'not planned yet'}</span>
                  <span className="text-accent">{t.plan ? usd(t.plan.total_usd) : t.budget_usd ? `budget ${usd(t.budget_usd)}` : ''}</span>
                </div>
              </Link>
              <button onClick={() => remove(t.id)} className="mono text-[11px] text-dim hover:text-rose mt-3">rm</button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
