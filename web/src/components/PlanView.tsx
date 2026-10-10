import { useState, type ReactNode } from 'react'
import { fmtDate, usd, type Plan } from '../lib/api'
import Itinerary from './Itinerary'
import StayCard from './StayCard'
import CostBreakdown from './CostBreakdown'
import RouteMap from './RouteMap'
import { SafetyCard, WeatherStrip } from './WeatherSafety'
import FlightChooser from './FlightChooser'
import DataWarning from './DataWarning'
import { planDataWarnings, planRealData } from '../lib/dataWarnings'

export type PlanMeta = { travelers?: number; budget_usd?: number | null; status?: string }

const MODE_ICON: Record<string, string> = { drive: '🚗', fly: '✈' }

/**
 * Itinerary-first plan page: the day-by-day schedule is the main column; the
 * rail carries the stay, the money, the map and the conditions. Everything
 * else (the raw lists, the data sources) sits below the fold.
 */
export default function PlanView({ plan, meta, actions, onChooseHotel, onReplan, onSelectFlight, disabled, busy }: {
  plan: Plan
  meta?: PlanMeta
  actions?: ReactNode
  onChooseHotel?: (name: string) => Promise<void> | void
  onReplan?: (mode: string) => void
  onSelectFlight?: (index: number, returnToken?: string) => Promise<void>
  disabled?: boolean
  busy?: boolean
}) {
  const realData = planRealData(plan)
  const rec = plan.options.find(o => o.mode === plan.recommended_mode) ?? plan.options[0]
  const first = plan.days[0]?.date, last = plan.days[plan.days.length - 1]?.date
  const budget = meta?.budget_usd ?? undefined
  const budgetLeft = budget ? budget - rec.transport_usd - rec.activities_usd : undefined
  const live = Object.entries(plan.sources).filter(([, v]) => !['mock', 'fallback', 'estimate'].includes(v)).length
  const [showSources, setShowSources] = useState(false)

  return (
    <div className="grid gap-6">
      <DataWarning details={planDataWarnings(plan)} />
      <section aria-label="Real provider data" className="rounded-lg border-2 border-accent bg-accent/5 p-5 print:hidden">
        <h2 className="mono text-xl font-bold text-accent">REAL PROVIDER DATA IN THIS PLAN</h2>
        {realData.length > 0 ? (
          <ul className="mt-3 list-disc pl-5 grid gap-1 text-sm">
            {realData.map(detail => <li key={detail}>{detail}</li>)}
          </ul>
        ) : (
          <p className="mt-2 text-muted">This plan has no identified real provider results.</p>
        )}
        <p className="mt-3 text-xs text-muted">
          This list comes from the sources recorded when the plan was generated. TPN did not recheck
          availability, so saved results may be out of date, and search prices are not booking quotes.
        </p>
      </section>
      <header className="panel p-5 print:border-0 print:p-0">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="min-w-0">
            <div className="mono text-xs text-muted prompt">tpn plan</div>
            <h2 className="mono text-2xl sm:text-3xl font-bold mt-1 leading-tight">
              <span>{plan.origin_place?.label ?? plan.title.split('→')[0]}</span>
              <span className="text-accent mx-2">→</span>
              <span>{plan.dest_place?.label ?? plan.title.split('→')[1]}</span>
            </h2>
            <div className="mono text-xs text-muted mt-2 flex flex-wrap gap-x-3 gap-y-1">
              {first && last && <span>{fmtDate(first)} – {fmtDate(last)}</span>}
              <span>{plan.nights} night{plan.nights === 1 ? '' : 's'}</span>
              {meta?.travelers && <span>{meta.travelers} traveler{meta.travelers > 1 ? 's' : ''}</span>}
              <span className="text-ink">{MODE_ICON[plan.recommended_mode]} {plan.recommended_mode}</span>
              {plan.recommended_mode === 'fly' && plan.origin_airport && plan.dest_airport && <span>{plan.origin_airport.code} → {plan.dest_airport.code}</span>}
              {meta?.status && <span className={`tag ${meta.status === 'booked' ? 'text-accent' : ''}`}>{meta.status}</span>}
            </div>
          </div>
          <div className="text-right">
            <div className="label">all-in estimate</div>
            <div className={`mono text-4xl font-bold tabular-nums ${plan.under_budget ? 'text-accent' : 'text-rose'}`}>{usd(plan.total_usd)}</div>
            <div className="mono text-[11px] text-muted">{usd(plan.total_usd / Math.max(plan.nights, 1))}/night · {budget ? (plan.under_budget ? 'under budget' : 'over budget') : 'no budget set'}</div>
            {actions && <div className="flex flex-wrap justify-end gap-2 mt-3 print:hidden">{actions}</div>}
          </div>
        </div>
        <p className="text-muted mt-4 max-w-3xl leading-relaxed">{plan.summary}</p>
        <div className="flex flex-wrap items-center gap-2 mt-3 print:hidden">
          {plan.ai_generated
            ? <span className="tag text-accent">✦ itinerary by AI on Akamai</span>
            : <span className="tag text-amber">itinerary: rule-based (AI offline)</span>}
          <button className="tag hover:text-ink" onClick={() => setShowSources(s => !s)} aria-expanded={showSources}>
            {live}/{Object.keys(plan.sources).length} live data sources {showSources ? '▾' : '▸'}
          </button>
          {showSources && Object.entries(plan.sources).map(([k, v]) => (
            <span key={k} className="tag">{k}: <span className={['mock', 'fallback', 'estimate'].includes(v) ? 'text-dim' : 'text-cyan'}>{v}</span></span>
          ))}
        </div>
        {(plan.sources.flights === 'serpapi' || plan.sources.hotels === 'serpapi') && (
          <p className="text-dim text-xs mt-3">Flight and stay prices are Google Travel search results via SerpApi. They are not booking quotes, and fees and final checkout prices may differ.</p>
        )}
      </header>

      <div className="grid lg:grid-cols-[minmax(0,1fr)_340px] xl:grid-cols-[minmax(0,1fr)_380px] gap-6 items-start">
        <div className="grid gap-6 min-w-0">
          {onSelectFlight && <FlightChooser key={plan.generated_at} plan={plan} onSelect={onSelectFlight} disabled={disabled || busy} />}
          <Itinerary plan={plan} />
          <Explore plan={plan} />
        </div>
        <aside className="grid gap-4 min-w-0 [&>*]:min-w-0 lg:sticky lg:top-16 print:hidden">
          <StayCard hotels={plan.hotels} budgetLeft={budgetLeft} onChoose={onChooseHotel} source={plan.sources.hotels} />
          <CostBreakdown plan={plan} budget={budget} onReplan={onReplan} busy={busy} />
          <RouteMap plan={plan} mode={plan.recommended_mode} />
          <WeatherStrip w={plan.weather} />
          <SafetyCard s={plan.safety} />
        </aside>
      </div>
    </div>
  )
}

function Explore({ plan }: { plan: Plan }) {
  const groups = [
    { title: 'things to do', list: plan.activities, color: 'text-accent' },
    { title: 'eat', list: plan.food, color: 'text-amber' },
    { title: 'stops on the way', list: plan.stops, color: 'text-muted' },
  ].filter(g => g.list.length)
  if (!groups.length) return null
  return (
    <section className="grid sm:grid-cols-3 gap-4 print:break-inside-avoid">
      {groups.map(g => (
        <div key={g.title} className="panel p-4">
          <h4 className="label">{g.title}</h4>
          <ul className="text-sm mt-2 grid gap-1.5">
            {g.list.map((p, i) => (
              <li key={i} className="flex justify-between gap-2">
                <span className="min-w-0">
                  <span className="text-ink">{p.name}</span>
                  {p.rating > 0 && <span className={`mono text-[10px] ml-1.5 ${g.color}`}>★{p.rating.toFixed(1)}</span>}
                  {p.note && <div className="text-[12px] text-dim">{p.note}</div>}
                </span>
                <span className="mono text-xs text-muted whitespace-nowrap">{p.price_usd ? usd(p.price_usd) : 'free'}</span>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </section>
  )
}
