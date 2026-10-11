import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, fmtDate, usd, type Trip } from '../lib/api'
import { useAuth } from '../lib/auth'
import Hero from '../components/Hero'
import Icon, { WeatherIcon } from '../components/Icon'
import { PlacePicture } from '../components/TripGlance'

const statusStyle: Record<Trip['status'], string> = { draft: 'text-muted', planned: 'text-cyan border-cyan/40', booked: 'text-accent border-accent/50 bg-accent-soft' }

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
    <div className="grid gap-8">
      <Hero eyebrow="My trips" title={<>Hello, {user?.name.split(' ')[0]}.</>}
        lede={trips && trips.length > 0
          ? <>{upcoming.length} upcoming · {past.length} past{spend > 0 && <> · {usd(spend)} planned</>}</>
          : 'Every trip you save shows up here with its price and itinerary.'}
        aside={<Link to="/app/new" className="btn-primary">Plan a trip<Icon name="arrow" size={15} /></Link>} />
      {err && <div className="text-sm text-rose">{err}</div>}
      {trips === null ? <div className="text-muted">Loading…</div> : trips.length === 0 ? (
        <div className="panel p-12 text-center grid gap-3 justify-items-center">
          <h2 className="text-[26px]">No trips yet.</h2>
          <p className="text-sm text-muted max-w-sm">Pick any two places, your dates and a budget. You get a priced, day-by-day itinerary in about a minute.</p>
          <Link to="/app/new" className="btn-primary mt-2">Plan your first one<Icon name="arrow" size={15} /></Link>
        </div>
      ) : (
        <>
          <TripGrid trips={upcoming} onRemove={remove} />
          {past.length > 0 && (
            <details className="group">
              <summary className="text-sm text-muted cursor-pointer hover:text-ink">Past trips ({past.length})</summary>
              <div className="mt-4"><TripGrid trips={past} onRemove={remove} dim /></div>
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
    <ul className="grid sm:grid-cols-2 lg:grid-cols-3 gap-5">
      {trips.map(t => {
        const p = t.plan
        const w = p?.weather?.days?.[0]
        const until = daysUntil(t.start_date)
        const n = nightsOf(t)
        const destName = p?.dest_place?.name ?? t.destination.split(',')[0]
        const originName = p?.origin_place?.name ?? t.origin.split(',')[0]
        const o = p ? (p.options.find(x => x.mode === p.recommended_mode) ?? p.options[0]) : null
        return (
          <li key={t.id} className={`panel overflow-hidden hover:border-ink/40 transition-colors flex flex-col ${dim ? 'opacity-70 hover:opacity-100' : ''}`}>
            <Link to={`/app/trips/${t.id}`} className="block flex-1">
              <PlacePicture name={p?.dest_place?.label ?? t.destination} kind={p?.dest_place?.kind} className="aspect-[16/9]" rounded="rounded-none" credit={false} lazy />
              <div className="p-5">
                <div className="flex justify-between items-start gap-3">
                  <h3 className="text-[24px] leading-tight truncate">{destName} Trip</h3>
                  <span className={`tag shrink-0 capitalize mt-1 ${statusStyle[t.status]}`}>{t.status}</span>
                </div>
                <div className="text-[13px] text-muted mt-0.5 flex items-center gap-1.5 min-w-0">
                  <span className="truncate">{originName}</span><Icon name="arrow" size={12} className="shrink-0" /><span className="truncate">{destName}</span>
                </div>
                <dl className="mt-3 grid gap-1.5 text-[13px]">
                  <div className="flex items-center justify-between gap-3">
                    <dt className="flex items-center gap-1.5 text-ink"><Icon name="calendar" size={14} className="text-muted" />{fmtDate(t.start_date, { month: 'short', day: 'numeric' })}–{fmtDate(t.end_date, { month: 'short', day: 'numeric' })}</dt>
                    <dd className="text-muted">{n} night{n === 1 ? '' : 's'}</dd>
                  </div>
                  <div className="flex items-center justify-between gap-3">
                    <dt className="flex items-center gap-1.5 text-ink"><Icon name="users" size={14} className="text-muted" />{t.travelers} traveler{t.travelers > 1 ? 's' : ''}</dt>
                    {p && <dd className="text-muted flex items-center gap-1 capitalize"><Icon name={p.recommended_mode === 'fly' ? 'plane' : 'car'} size={14} />{p.recommended_mode}</dd>}
                  </div>
                </dl>
                {p && o ? (
                  <div className="mt-4 rounded-lg bg-accent-soft/70 px-3.5 py-3">
                    <div className="flex items-baseline justify-between gap-3">
                      <div className="serif text-[24px] leading-none tabular-nums text-ink">{usd(p.total_usd)}</div>
                      <div className={`text-[12px] ${p.under_budget ? 'text-muted' : 'text-rose'}`}>{t.budget_usd ? (p.under_budget ? `of ${usd(t.budget_usd)}` : `over ${usd(t.budget_usd)}`) : 'all-in'}</div>
                    </div>
                    <div className="mt-2 h-1.5 rounded-full bg-panel/80 overflow-hidden flex">
                      {(() => {
                        const cap = Math.max(t.budget_usd ?? 0, o.total_usd, 1)
                        return [['var(--color-cyan)', o.transport_usd], ['var(--color-rose)', o.lodging_usd], ['var(--color-accent)', o.activities_usd]].map(([c, v], i) => (
                          <span key={i} style={{ width: `${(Number(v) / cap) * 100}%`, background: String(c) }} />
                        ))
                      })()}
                    </div>
                    <div className="mt-2 flex items-center justify-between text-[12px] text-muted">
                      <span className="truncate flex items-center gap-1.5"><Icon name="bed" size={13} />{p.hotels[0]?.name ?? 'No stay'}</span>
                      {w && <span className="shrink-0 flex items-center gap-1"><WeatherIcon icon={w.icon} size={13} />{w.high_f}°</span>}
                    </div>
                  </div>
                ) : (
                  <div className="mt-4 rounded-lg bg-panel-2 px-3.5 py-3 text-[13px] flex justify-between">
                    <span className="text-muted">Not planned yet</span>
                    <span className="text-dim">{t.budget_usd ? `Budget ${usd(t.budget_usd)}` : ''}</span>
                  </div>
                )}
              </div>
            </Link>
            <div className="px-5 pb-4 flex items-center justify-between text-[12px]">
              <span className="text-dim">{until > 0 ? `In ${until} day${until === 1 ? '' : 's'}` : until === 0 ? 'Today' : 'Done'}</span>
              <button onClick={() => onRemove(t.id)} className="text-dim hover:text-rose">Delete</button>
            </div>
          </li>
        )
      })}
    </ul>
  )
}
