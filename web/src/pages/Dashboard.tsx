import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, fmtDate, usd, type Trip } from '../lib/api'
import { useAuth } from '../lib/auth'

const statusColor: Record<Trip['status'], string> = { draft: 'text-dim', planned: 'text-cyan', booked: 'text-accent' }
const MODE_ICON: Record<string, string> = { drive: '🚗', fly: '✈' }
const ICON: Record<string, string> = { sun: '☀', cloud: '☁', rain: '🌧', snow: '❄', storm: '⛈', wind: '💨', fog: '🌫' }

const nightsOf = (t: Trip) => Math.max(0, Math.round((Date.parse(t.end_date) - Date.parse(t.start_date)) / 86400000))
const daysUntil = (iso: string) => Math.ceil((Date.parse(iso) - Date.now()) / 86400000)

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

  const upcoming = (trips ?? []).filter(t => daysUntil(t.end_date) >= 0).sort((a, b) => a.start_date.localeCompare(b.start_date))
  const past = (trips ?? []).filter(t => daysUntil(t.end_date) < 0).sort((a, b) => b.start_date.localeCompare(a.start_date))
  const spend = upcoming.reduce((s, t) => s + (t.plan?.total_usd ?? 0), 0)

  return (
    <div className="grid gap-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <div className="mono text-xs text-muted prompt">ls ~/trips</div>
          <h1 className="mono text-2xl font-bold mt-1">hey {user?.name.split(' ')[0]}</h1>
          {trips && trips.length > 0 && (
            <div className="mono text-xs text-muted mt-1">{upcoming.length} upcoming · {past.length} past{spend > 0 && <> · <span className="text-ink">{usd(spend)}</span> planned</>}</div>
          )}
        </div>
        <Link to="/app/new" className="btn-primary">+ new trip</Link>
      </div>
      {err && <div className="mono text-sm text-rose">{err}</div>}
      {trips === null ? <div className="mono text-muted">loading…</div> : trips.length === 0 ? (
        <div className="panel p-10 text-center grid gap-3 justify-items-center">
          <div className="mono text-muted">no trips yet.</div>
          <div className="text-sm text-dim max-w-sm">Pick any two places, your dates and a budget. You get a priced, day-by-day itinerary in about ten seconds.</div>
          <Link to="/app/new" className="btn-primary">plan your first one →</Link>
        </div>
      ) : (
        <>
          <TripGrid trips={upcoming} onRemove={remove} />
          {past.length > 0 && (
            <details className="group">
              <summary className="mono text-xs text-muted cursor-pointer hover:text-ink">past trips ({past.length})</summary>
              <div className="mt-3"><TripGrid trips={past} onRemove={remove} dim /></div>
            </details>
          )}
        </>
      )}
    </div>
  )
}

function TripGrid({ trips, onRemove, dim }: { trips: Trip[]; onRemove: (id: string) => void; dim?: boolean }) {
  if (!trips.length) return null
  return (
    <ul className="grid sm:grid-cols-2 gap-4">
      {trips.map(t => {
        const p = t.plan
        const w = p?.weather?.days?.[0]
        const until = daysUntil(t.start_date)
        const n = nightsOf(t)
        return (
          <li key={t.id} className={`panel hover:border-accent transition-colors flex flex-col ${dim ? 'opacity-70 hover:opacity-100' : ''}`}>
            <Link to={`/app/trips/${t.id}`} className="block p-4 flex-1">
              <div className="flex justify-between items-start gap-2">
                <h3 className="mono font-bold leading-snug">
                  {p ? <>{p.origin_place?.name ?? t.origin} <span className="text-accent">→</span> {p.dest_place?.label ?? t.destination}</> : t.title}
                </h3>
                <span className={`tag shrink-0 ${statusColor[t.status]}`}>{t.status}</span>
              </div>
              <div className="mono text-xs text-muted mt-1 flex flex-wrap gap-x-2">
                <span>{fmtDate(t.start_date)} – {fmtDate(t.end_date)}</span>
                <span>· {n} night{n === 1 ? '' : 's'}</span>
                <span>· {t.travelers} traveler{t.travelers > 1 ? 's' : ''}</span>
              </div>
              {p ? (
                <div className="mt-3 grid gap-2">
                  <div className="flex items-center justify-between mono text-sm">
                    <span className="text-muted">{MODE_ICON[p.recommended_mode]} {p.recommended_mode}{p.recommended_mode === 'fly' && p.origin_airport && p.dest_airport ? ` ${p.origin_airport.code}→${p.dest_airport.code}` : ''}</span>
                    <span className={p.under_budget ? 'text-accent' : 'text-rose'}>{usd(p.total_usd)}</span>
                  </div>
                  <div className="flex items-center justify-between mono text-[11px] text-dim">
                    <span className="truncate">⌂ {p.hotels[0]?.name ?? 'no stay'}</span>
                    {w && <span className="shrink-0">{ICON[w.icon] ?? '☀'} {w.high_f}° on arrival</span>}
                  </div>
                  <div className="h-1 rounded bg-line overflow-hidden flex">
                    {(() => {
                      const o = p.options.find(x => x.mode === p.recommended_mode) ?? p.options[0]
                      const cap = Math.max(t.budget_usd ?? 0, o.total_usd, 1)
                      return [['var(--color-cyan)', o.transport_usd], ['var(--color-rose)', o.lodging_usd], ['var(--color-accent)', o.activities_usd]].map(([c, v], i) => (
                        <span key={i} style={{ width: `${(Number(v) / cap) * 100}%`, background: String(c) }} />
                      ))
                    })()}
                  </div>
                </div>
              ) : (
                <div className="mono text-sm mt-3 flex justify-between">
                  <span className="text-muted">not planned yet</span>
                  <span className="text-dim">{t.budget_usd ? `budget ${usd(t.budget_usd)}` : ''}</span>
                </div>
              )}
            </Link>
            <div className="px-4 pb-3 flex items-center justify-between mono text-[11px]">
              <span className="text-dim">{until > 0 ? `in ${until} day${until === 1 ? '' : 's'}` : until === 0 ? 'today' : 'done'}</span>
              <button onClick={() => onRemove(t.id)} className="text-dim hover:text-rose">rm</button>
            </div>
          </li>
        )
      })}
    </ul>
  )
}
