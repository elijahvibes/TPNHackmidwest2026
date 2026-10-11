import { useState, type ReactNode } from 'react'
import { fmtDate, usd, type Plan } from '../lib/api'
import Itinerary from './Itinerary'
import StayCard from './StayCard'
import CostBreakdown from './CostBreakdown'
import RouteMap from './RouteMap'
import { SafetyCard, WeatherStrip } from './WeatherSafety'
import FlightChooser from './FlightChooser'
import DataWarning, { EstimatesNote } from './DataWarning'
import { AgentsCard, ResearchCard } from './AgentsPanel'
import Icon from './Icon'
import { PlacePicture } from './TripGlance'
import { planDataWarnings, planEstimates, planRealData } from '../lib/dataWarnings'
import { placeMapLink } from '../lib/placeLinks'
import { destinationTransport, roadStops } from '../lib/transport'

export type PlanMeta = { travelers?: number; budget_usd?: number | null; status?: string }

export default function PlanView({ plan, meta, actions, onChooseHotel, onReplan, onSelectFlight, disabled, busy, refining }: {
  plan: Plan
  meta?: PlanMeta
  actions?: ReactNode
  onChooseHotel?: (name: string) => Promise<void> | void
  onReplan?: (mode: string) => void
  onSelectFlight?: (index: number, returnToken?: string) => Promise<void>
  disabled?: boolean
  busy?: boolean
  /** a draft: real prices are final, the AI agents are still working on places and the itinerary */
  refining?: boolean
}) {
  const realData = planRealData(plan)
  const rec = plan.options.find(o => o.mode === plan.recommended_mode) ?? plan.options[0]
  const first = plan.days[0]?.date, last = plan.days[plan.days.length - 1]?.date
  const budget = meta?.budget_usd ?? undefined
  const budgetLeft = budget ? budget - rec.transport_usd - rec.activities_usd - (rec.local_usd ?? 0) : undefined
  const live = Object.entries(plan.sources).filter(([, v]) => !['mock', 'fallback', 'estimate', 'none'].includes(v)).length
  const [showSources, setShowSources] = useState(false)
  const stops = roadStops(plan)
  const unknownCosts = [...plan.activities, ...plan.food, ...stops].some(p => p.price_unknown)
  const estimates = planEstimates(plan)
  // While the AI is still working, "rule-based itinerary" is a transient state, not a warning.
  const warnings = planDataWarnings(plan).filter(w => !(refining && w.startsWith('Itinerary:')))
  const destName = (plan.dest_place?.name ?? plan.title.split('→')[1] ?? '').trim()

  return (
    <div className="grid gap-6">
      <DataWarning details={warnings} />
      <EstimatesNote details={estimates} />
      <section aria-label="Real provider data" className="rounded-xl border-2 border-accent/60 bg-accent-soft/50 p-5 print:hidden">
        <h2 className="text-[20px] text-accent">Real provider data in this plan</h2>
        {realData.length > 0 ? (
          <ul className="mt-3 list-disc pl-5 grid gap-1 text-sm">
            {realData.map(detail => <li key={detail}>{detail}</li>)}
          </ul>
        ) : <p className="mt-2 text-muted">No real provider results are identified in this plan.</p>}
        <p className="mt-3 text-xs text-muted">
          This list comes from the sources recorded when the plan was generated. TPN did not recheck
          availability, so saved results may be out of date, and search prices are not booking quotes.
        </p>
      </section>
      <header className="panel px-6 sm:px-8 py-7 print:border-0 print:p-0">
        <div className="eyebrow">Your plan</div>
        <h2 className="text-[34px] sm:text-[40px] leading-tight mt-1">{destName ? `${destName} Trip` : plan.title}</h2>
        <div className="text-[14px] text-muted mt-1 flex items-center gap-2">
          <span>{plan.origin_place?.label ?? plan.title.split('→')[0]}</span>
          <Icon name="arrow" size={14} />
          <span>{plan.dest_place?.label ?? plan.title.split('→')[1]}</span>
          {meta?.status && <span className={`tag capitalize ml-1 ${meta.status === 'booked' ? 'text-accent border-accent/50 bg-accent-soft' : ''}`}>{meta.status}</span>}
        </div>
        <p className="text-[15px] text-muted mt-4 max-w-3xl leading-relaxed">{plan.summary}</p>
        <div className="flex flex-wrap items-center gap-2 mt-4 print:hidden">
          {plan.local_transport && <span className="tag">Getting around: {destinationTransport(plan)} · explore within {plan.activity_radius_miles} miles</span>}
          {plan.ai_generated
            ? <span className="tag text-accent">Itinerary by {plan.agents?.length ? `${plan.agents.length} AI agents` : 'AI'} on Akamai</span>
            : refining
              ? <span className="tag text-cyan">Draft · AI agents on Akamai are refining this<span className="caret" /></span>
              : <span className="tag text-amber">Itinerary: rule-based (AI offline)</span>}
          <button className="tag hover:text-ink inline-flex items-center gap-1" onClick={() => setShowSources(s => !s)} aria-expanded={showSources}>
            {live}/{Object.keys(plan.sources).length} live data sources <Icon name="chevron" size={12} className={showSources ? 'rotate-180' : ''} />
          </button>
          {showSources && Object.entries(plan.sources).map(([k, v]) => (
            <span key={k} className="tag">{k}: <span className={['mock', 'fallback', 'estimate', 'none'].includes(v) ? 'text-dim' : 'text-cyan'}>{v}</span></span>
          ))}
        </div>
        {(plan.sources.flights === 'serpapi' || plan.sources.hotels === 'serpapi') && (
          <p className="text-dim text-xs mt-3">Flight and stay prices are Google Travel search results via SerpApi. They are not booking quotes, and fees and final checkout prices may differ.</p>
        )}
        {actions && (
          <div className="lg:hidden flex flex-wrap items-center justify-between gap-3 mt-5 pt-4 border-t border-line print:hidden">
            <span className={`serif text-[26px] tabular-nums ${plan.under_budget ? 'text-ink' : 'text-rose'}`}>{usd(plan.total_usd)}</span>
            <div className="flex flex-wrap gap-2">{actions}</div>
          </div>
        )}
      </header>

      <div className="grid lg:grid-cols-[minmax(0,1fr)_340px] xl:grid-cols-[minmax(0,1fr)_380px] gap-6 items-start">
        <div className="grid gap-6 min-w-0">
          {onSelectFlight && <FlightChooser key={plan.generated_at} plan={plan} onSelect={onSelectFlight} disabled={disabled || busy} />}
          <Itinerary plan={plan} />
          <ResearchCard r={plan.research} />
          <Explore plan={plan} />
        </div>
        <aside className="grid gap-4 min-w-0 [&>*]:min-w-0 lg:sticky lg:top-24 print:hidden">
          <section className="panel p-5 grid gap-4" aria-label="Trip at a glance">
            <PlacePicture name={plan.dest_place?.label ?? destName} kind={plan.dest_place?.kind} />
            <dl className="grid gap-2 text-[14px]">
              {first && last && (
                <div className="flex items-center justify-between gap-3">
                  <dt className="flex items-center gap-2 text-ink"><Icon name="calendar" size={16} className="text-muted" />{fmtDate(first, { month: 'short', day: 'numeric' })}–{fmtDate(last, { month: 'short', day: 'numeric' })}</dt>
                  <dd className="text-[13px] text-muted">{plan.nights} night{plan.nights === 1 ? '' : 's'}</dd>
                </div>
              )}
              <div className="flex items-center justify-between gap-3">
                <dt className="flex items-center gap-2 text-ink"><Icon name="users" size={16} className="text-muted" />{meta?.travelers ?? plan.travelers ?? 1} traveler{(meta?.travelers ?? plan.travelers ?? 1) > 1 ? 's' : ''}</dt>
                <dd className="text-[13px] text-muted flex items-center gap-1.5 capitalize"><Icon name={plan.recommended_mode === 'fly' ? 'plane' : 'car'} size={14} />{plan.recommended_mode}
                  {plan.recommended_mode === 'fly' && plan.origin_airport && plan.dest_airport && <span className="uppercase">{plan.origin_airport.code}–{plan.dest_airport.code}</span>}</dd>
              </div>
            </dl>
            <div className="rounded-lg bg-accent-soft/70 px-4 py-3.5">
              <div className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">{unknownCosts ? 'All-in estimate · some places unpriced' : 'All-in estimate'}</div>
              <div className="flex items-baseline justify-between gap-3 mt-1">
                <div className={`serif text-[32px] leading-none tabular-nums ${plan.under_budget ? 'text-ink' : 'text-rose'}`}>{usd(plan.total_usd)}</div>
                <div className="text-[13px] text-muted">{usd(plan.total_usd / Math.max(plan.nights, 1))} / night</div>
              </div>
              <div className="text-[13px] text-muted mt-1.5">{budget ? (plan.under_budget ? `${usd(budget - plan.total_usd)} under the ${usd(budget)} budget` : `${usd(plan.total_usd - budget)} over the ${usd(budget)} budget`) : 'No budget set'}{estimates.length > 0 && ' · includes estimates'}</div>
            </div>
            {actions && <div className="grid grid-cols-2 gap-2 [&>*]:w-full [&>*:last-child:nth-child(odd)]:col-span-2">{actions}</div>}
          </section>
          <StayCard hotels={plan.hotels} budgetLeft={budgetLeft} onChoose={onChooseHotel} source={plan.sources.hotels} />
          <CostBreakdown plan={plan} budget={budget} onReplan={onReplan} busy={busy} />
          <AgentsCard plan={plan} />
          <RouteMap plan={plan} mode={plan.recommended_mode} />
          <WeatherStrip w={plan.weather} />
          <SafetyCard s={plan.safety} />
        </aside>
      </div>
      {[...plan.activities, ...plan.food, ...stops].some(p => p.source === 'geoapify' || p.source === 'wikipedia' || p.popularity) && (
        <p className="text-xs text-muted">
          Powered by <a className="underline" href="https://www.geoapify.com/" target="_blank" rel="noreferrer">Geoapify</a>
          {' · '}© <a className="underline" href="https://www.openstreetmap.org/copyright" target="_blank" rel="noreferrer">OpenStreetMap contributors</a>
          {' · '}"well known" and descriptions from <a className="underline" href="https://en.wikipedia.org/" target="_blank" rel="noreferrer">Wikipedia</a> (CC BY-SA).
          Place prices marked ~ are typical-price estimates, not provider data.
        </p>
      )}
    </div>
  )
}

function Explore({ plan }: { plan: Plan }) {
  const groups = [
    { title: 'Things to do', list: plan.activities },
    { title: 'Where to eat', list: plan.food },
    { title: 'Stops on the way', list: roadStops(plan) },
  ].filter(g => g.list.length)
  if (!groups.length) return null
  return (
    <section className="grid sm:grid-cols-3 gap-4 print:break-inside-avoid">
      {groups.map(g => (
        <div key={g.title} className="panel p-5">
          <h3 className="text-[19px]">{g.title}</h3>
          <ul className="text-sm mt-3 grid gap-2.5">
            {g.list.map((p, i) => {
              const mapLink = placeMapLink(p)
              return (
                <li key={i} className="flex justify-between gap-2">
                  <span className="min-w-0">
                    <span className="text-ink">{p.name}</span>
                    {p.rating > 0 && <span className="text-[11px] ml-1.5 text-amber">★ {p.rating.toFixed(1)}</span>}
                    {(p.popularity ?? 0) >= 100 && <span className="text-[11px] ml-1.5 text-cyan" title={`${p.popularity} Wikipedia readers a day`}>{(p.popularity ?? 0) >= 1000 ? 'famous' : 'well known'}</span>}
                    {p.note && <div className="text-[12px] text-muted">{p.note}</div>}
                    {mapLink && <a href={mapLink} target="_blank" rel="noopener noreferrer" className="text-[12px] text-cyan underline underline-offset-2"
                      aria-label={`View map for ${p.name} (opens in a new tab)`}>View map</a>}
                  </span>
                  <span className="text-[12px] text-muted whitespace-nowrap tabular-nums" title={p.price_estimated ? 'typical price, estimated' : undefined}>
                    {p.price_unknown ? 'price unknown' : p.price_usd ? `${p.price_estimated ? '~' : ''}${usd(p.price_usd)}` : p.price_estimated ? '~free' : 'free'}
                  </span>
                </li>
              )
            })}
          </ul>
        </div>
      ))}
    </section>
  )
}
