import { useEffect, useMemo, useRef, useState } from 'react'
import { fmtDate, usd, type Day, type DayItem, type ItemCategory, type Plan } from '../lib/api'
import { itineraryMapLink } from '../lib/placeLinks'
import { roadStops, showTransportItem } from '../lib/transport'

const ICON: Record<string, string> = { sun: '☀', cloud: '☁', rain: '🌧', snow: '❄', storm: '⛈', wind: '💨', fog: '🌫' }

const CAT: Record<ItemCategory, { label: string; color: string; glyph: string }> = {
  transport: { label: 'travel', color: 'var(--color-cyan)', glyph: '→' },
  lodging: { label: 'stay', color: 'var(--color-rose)', glyph: '⌂' },
  activity: { label: 'do', color: 'var(--color-accent)', glyph: '★' },
  food: { label: 'eat', color: 'var(--color-amber)', glyph: '◆' },
  stop: { label: 'stop', color: 'var(--color-muted)', glyph: '⚑' },
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
      <div className="sticky top-14 z-[5] -mx-1 px-1 py-2 bg-bg/90 backdrop-blur print:static print:bg-transparent">
        <div className="flex items-center gap-2 overflow-x-auto scrollbar-none">
          <span className="mono text-xs text-muted prompt shrink-0">itinerary</span>
          {days.map(d => {
            const w = weatherByDate[d.date]
            const on = d.day === active
            return (
              <button key={d.day} onClick={() => jump(d.day)}
                className={`shrink-0 mono text-[11px] px-2.5 py-1 rounded-full border transition-colors ${on ? 'border-accent text-accent bg-accent/10' : 'border-line text-muted hover:text-ink hover:border-muted'}`}
                aria-current={on ? 'true' : undefined}>
                <span className="font-bold">d{d.day}</span> · {fmtDate(d.date, { weekday: 'short' })} {fmtDate(d.date, { day: 'numeric' })}
                {w && <span className="ml-1.5 text-dim">{ICON[w.icon] ?? '☀'} {w.high_f}°</span>}
              </button>
            )
          })}
          <button onClick={() => window.print()} className="ml-auto shrink-0 mono text-[11px] text-dim hover:text-ink print:hidden" title="print or save as PDF">⎙ print</button>
        </div>
      </div>

      {days.map(d => {
        const w = weatherByDate[d.date]
        const sub = dayTotal(d)
        return (
          <article key={d.day} ref={el => { refs.current[d.day] = el }} data-day={d.day} id={`day-${d.day}`}
            className="panel overflow-hidden scroll-mt-28 print:break-inside-avoid">
            <header className="flex flex-wrap items-center gap-x-4 gap-y-1 px-5 pt-4 pb-3 border-b border-line bg-panel-2/40">
              <div className="mono text-accent font-bold text-2xl leading-none tabular-nums w-10">{String(d.day).padStart(2, '0')}</div>
              <div className="min-w-0 flex-1">
                <div className="font-medium text-ink">{d.theme}</div>
                <div className="mono text-[11px] text-muted">{fmtDate(d.date, { weekday: 'long', month: 'long', day: 'numeric' })}</div>
              </div>
              {w && (
                <div className={`mono text-[11px] px-2 py-1 rounded border ${w.precip_pct >= 50 ? 'border-cyan/50 text-cyan' : 'border-line text-muted'}`} title={w.short}>
                  {ICON[w.icon] ?? '☀'} {w.high_f}° / {w.low_f}°{w.precip_pct > 0 ? ` · ${w.precip_pct}% rain` : ''}
                </div>
              )}
              <div className="mono text-sm text-ink tabular-nums">{sub ? usd(sub) : <span className="text-dim">{unknownPlaceCosts || d.items.some(it => it.cost_unknown) ? 'unquoted costs' : 'free day'}</span>}</div>
            </header>
            <ol className="px-5 py-2">
              {d.items.map((it, i) => <Item key={i} it={it} last={i === d.items.length - 1} mapLink={itineraryMapLink(plan, it)} unknownPlaceCosts={unknownPlaceCosts} />)}
            </ol>
          </article>
        )
      })}

      <div className="flex items-center justify-between mono text-xs text-muted px-1">
        <span>{plan.days.length} days · {plan.nights} nights</span>
        <span>scheduled costs <b className="text-ink">{usd(total)}</b> · plan total <b className="text-accent">{usd(plan.total_usd)}</b></span>
      </div>
    </section>
  )
}

function Item({ it, last, mapLink, unknownPlaceCosts }: { it: DayItem; last: boolean; mapLink: string | undefined; unknownPlaceCosts: boolean }) {
  const c = CAT[it.category] ?? CAT.activity
  return (
    <li className="grid grid-cols-[44px_20px_1fr_auto] gap-x-3 relative">
      <span className="mono text-[11px] text-dim pt-3 tabular-nums">{it.time === '--:--' ? '' : it.time}</span>
      <span className="relative flex justify-center pt-3">
        {!last && <span className="absolute top-7 bottom-0 w-px bg-line" aria-hidden />}
        <span className="relative z-[1] w-5 h-5 rounded-full border bg-bg flex items-center justify-center text-[10px] leading-none"
          style={{ borderColor: c.color, color: c.color }} aria-label={c.label}>{c.glyph}</span>
      </span>
      <div className={`py-2.5 ${last ? '' : 'border-b border-line/60'}`}>
        <div className="text-sm text-ink">{it.title}</div>
        {it.detail && <div className="text-[13px] text-muted mt-0.5">{it.detail}</div>}
        {mapLink && <a href={mapLink} target="_blank" rel="noopener noreferrer" className="text-xs text-cyan underline"
          aria-label={`View map for ${it.title} (opens in a new tab)`}>{it.category === 'lodging' ? 'Search map' : 'View map'}</a>}
        <div className="mono text-[10px] uppercase tracking-wider mt-1" style={{ color: c.color }}>{c.label}</div>
      </div>
      <span className={`mono text-sm pt-2.5 tabular-nums ${it.cost_usd ? 'text-ink' : 'text-dim'}`}>{it.cost_unknown ? 'price unknown' : it.cost_usd ? usd(it.cost_usd) : unknownPlaceCosts && ['activity', 'food', 'stop'].includes(it.category) ? 'price unknown' : 'free'}</span>
    </li>
  )
}
