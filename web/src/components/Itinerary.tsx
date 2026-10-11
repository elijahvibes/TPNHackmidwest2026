import { useEffect, useMemo, useRef, useState } from 'react'
import { fmtDate, fmtTime, usd, type Day, type DayItem, type ItemCategory, type Plan } from '../lib/api'
import { itineraryMapLink } from '../lib/placeLinks'
import { roadStops, showTransportItem } from '../lib/transport'
import Icon, { WeatherIcon, type IconName } from './Icon'

const CAT: Record<ItemCategory, { label: string; color: string; icon: IconName }> = {
  transport: { label: 'Travel', color: 'var(--color-cyan)', icon: 'arrow' },
  lodging: { label: 'Stay', color: 'var(--color-rose)', icon: 'bed' },
  activity: { label: 'Do', color: 'var(--color-accent)', icon: 'star' },
  food: { label: 'Eat', color: 'var(--color-amber)', icon: 'fork' },
  stop: { label: 'Stop', color: 'var(--color-muted)', icon: 'flag' },
}

const dayTotal = (d: Day) => d.items.reduce((s, it) => s + (it.cost_usd || 0), 0)

/**
 * The itinerary is the product. Day chips up top jump to each day; every day
 * is a vertical timeline with the weather for that date and its running cost.
 */
export default function Itinerary({ plan }: { plan: Plan }) {
  const [active, setActive] = useState(1)
  const refs = useRef<Record<number, HTMLElement | null>>({})
  const weatherByDate = useMemo(() => Object.fromEntries((plan.weather?.days ?? []).map(d => [d.date, d])), [plan.weather])
  const days = plan.days.map(d => ({...d, items: d.items.filter(it => showTransportItem(plan, it))}))
  const unknownPlaceCosts = [...plan.activities, ...plan.food, ...roadStops(plan)].some(p => p.price_unknown)

  // Highlight the day currently in view as the reader scrolls.
  useEffect(() => {
    const els = Object.values(refs.current).filter(Boolean) as HTMLElement[]
    if (!els.length || typeof IntersectionObserver === 'undefined') return
    const io = new IntersectionObserver(entries => {
      const vis = entries.filter(e => e.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)
      if (vis[0]) setActive(Number((vis[0].target as HTMLElement).dataset.day))
    }, { rootMargin: '-120px 0px -60% 0px', threshold: 0 })
    els.forEach(el => io.observe(el))
    return () => io.disconnect()
  }, [plan.days])

  const jump = (n: number) => {
    setActive(n)
    refs.current[n]?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  const total = days.reduce((s, d) => s + dayTotal(d), 0)

  return (
    <section className="grid gap-4" aria-label="itinerary">
      <div className="sticky top-16 z-[5] -mx-1 px-1 py-2 bg-bg/95 backdrop-blur print:static print:bg-transparent">
        <div className="flex items-center gap-2 overflow-x-auto scrollbar-none">
          <h3 className="text-[19px] shrink-0 mr-2">Itinerary</h3>
          {days.map(d => {
            const w = weatherByDate[d.date]
            const on = d.day === active
            return (
              <button key={d.day} onClick={() => jump(d.day)}
                className={`chip shrink-0 !py-1 !text-[12px] ${on ? 'chip-on' : ''}`}
                aria-current={on ? 'true' : undefined}>
                <span className="font-medium">Day {d.day}</span> · {fmtDate(d.date, { weekday: 'short' })} {fmtDate(d.date, { day: 'numeric' })}
                {w && <span className="ml-1 text-dim flex items-center gap-0.5"><WeatherIcon icon={w.icon} size={12} />{w.high_f}°</span>}
              </button>
            )
          })}
          <button onClick={() => window.print()} className="ml-auto shrink-0 text-[12px] text-muted hover:text-ink print:hidden flex items-center gap-1" title="Print or save as PDF"><Icon name="print" size={14} />Print</button>
        </div>
      </div>

      {days.map(d => {
        const w = weatherByDate[d.date]
        const sub = dayTotal(d)
        return (
          <article key={d.day} ref={el => { refs.current[d.day] = el }} data-day={d.day} id={`day-${d.day}`}
            className="panel overflow-hidden scroll-mt-32 print:break-inside-avoid">
            <header className="flex flex-wrap items-center gap-x-4 gap-y-1 px-6 pt-5 pb-4 border-b border-line bg-panel-2/60">
              <div className="serif text-ink text-[28px] leading-none tabular-nums w-10">{String(d.day).padStart(2, '0')}</div>
              <div className="min-w-0 flex-1">
                <div className="text-[17px] text-ink serif">{d.theme}</div>
                <div className="text-[12px] text-muted">{fmtDate(d.date, { weekday: 'long', month: 'long', day: 'numeric' })}</div>
              </div>
              {w && (
                <div className={`text-[12px] px-2 py-1 rounded-md border flex items-center gap-1.5 ${w.precip_pct >= 50 ? 'border-cyan/40 text-cyan' : 'border-line text-muted'}`} title={w.short}>
                  <WeatherIcon icon={w.icon} size={14} />{w.high_f}° / {w.low_f}°{w.precip_pct > 0 ? ` · ${w.precip_pct}% rain` : ''}
                </div>
              )}
              <div className="text-sm text-ink tabular-nums">{sub ? usd(sub) : <span className="text-dim">{unknownPlaceCosts || d.items.some(it => it.cost_unknown) ? 'Unquoted costs' : 'Free day'}</span>}</div>
            </header>
            <ol className="px-6 py-2">
              {d.items.map((it, i) => <Item key={i} it={it} last={i === d.items.length - 1} mapLink={itineraryMapLink(plan, it)} unknownPlaceCosts={unknownPlaceCosts} />)}
            </ol>
          </article>
        )
      })}

      <div className="flex items-center justify-between text-[13px] text-muted px-1">
        <span>{plan.days.length} days · {plan.nights} nights</span>
        <span>Scheduled costs <b className="text-ink font-medium tabular-nums">{usd(total)}</b> · plan total <b className="text-ink font-medium tabular-nums">{usd(plan.total_usd)}</b></span>
      </div>
    </section>
  )
}

function Item({ it, last, mapLink, unknownPlaceCosts }: { it: DayItem; last: boolean; mapLink: string | undefined; unknownPlaceCosts: boolean }) {
  const c = CAT[it.category] ?? CAT.activity
  return (
    <li className="grid grid-cols-[64px_22px_1fr_auto] gap-x-3 relative">
      <span className="text-[12px] text-muted pt-3.5 tabular-nums whitespace-nowrap">{it.time === '--:--' ? '' : fmtTime(it.time)}</span>
      <span className="relative flex justify-center pt-3">
        {!last && <span className="absolute top-8 bottom-0 w-px bg-line" aria-hidden />}
        <span className="relative z-[1] w-[22px] h-[22px] rounded-full border bg-panel flex items-center justify-center"
          style={{ borderColor: c.color, color: c.color }} aria-label={c.label}><Icon name={c.icon} size={11} /></span>
      </span>
      <div className={`py-3 ${last ? '' : 'border-b border-line/60'}`}>
        <div className="text-[15px] text-ink">{it.title}</div>
        {it.detail && <div className="text-[13px] text-muted mt-0.5">{it.detail}</div>}
        <div className="flex items-center gap-3 mt-1">
          <span className="text-[11px] uppercase tracking-wider" style={{ color: c.color }}>{c.label}</span>
          {mapLink && <a href={mapLink} target="_blank" rel="noopener noreferrer" className="text-[12px] text-cyan underline underline-offset-2"
            aria-label={`View map for ${it.title} (opens in a new tab)`}>{it.category === 'lodging' ? 'Search map' : 'View map'}</a>}
        </div>
      </div>
      <span className={`text-sm pt-3 tabular-nums ${it.cost_usd ? 'text-ink' : 'text-dim'}`}>{it.cost_unknown ? 'price unknown' : it.cost_usd ? usd(it.cost_usd) : unknownPlaceCosts && ['activity', 'food', 'stop'].includes(it.category) ? 'price unknown' : 'free'}</span>
    </li>
  )
}
