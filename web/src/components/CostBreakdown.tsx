import { hrs, usd, type Plan, type PlanOption } from '../lib/api'
import { roadStops } from '../lib/transport'
import Icon from './Icon'

/** Where the money goes, against the budget, plus the drive-vs-fly call. */
export default function CostBreakdown({ plan, budget, onReplan, busy }: {
  plan: Plan
  budget?: number | null
  onReplan?: (mode: string) => void
  busy?: boolean
}) {
  const rec = plan.options.find(o => o.mode === plan.recommended_mode) ?? plan.options[0]
  const cap = Math.max(budget ?? 0, rec.total_usd, 1)
  const seg = [
    { k: 'Transport', v: rec.transport_usd, c: 'var(--color-cyan)' },
    { k: 'Lodging', v: rec.lodging_usd, c: 'var(--color-rose)' },
    { k: 'Things to do & food', v: rec.activities_usd, c: 'var(--color-accent)' },
    ...(rec.local_usd ? [{ k: 'Getting around ~', v: rec.local_usd, c: 'var(--color-amber)' }] : []),
  ]
  const over = budget ? rec.total_usd - budget : 0
  const places = [...plan.activities, ...plan.food, ...roadStops(plan)]
  const unknownCosts = places.some(p => p.price_unknown)
  const estimated = !!rec.local_usd || places.some(p => p.price_estimated)
  return (
    <div className="panel p-5 grid gap-5">
      <div>
        <div className="flex items-baseline justify-between gap-3">
          <h3 className="text-[19px]">Cost</h3>
          <span className="text-[12px] text-muted text-right">
            {budget ? (over > 0 ? <span className="text-rose">{usd(over)} over the {usd(budget)} budget</span> : <span>{usd(budget - rec.total_usd)} under the {usd(budget)} budget</span>) : 'No budget set'}
          </span>
        </div>
        <div className="mt-3 h-2.5 rounded-full bg-line/70 overflow-hidden flex" role="img" aria-label={`total ${usd(rec.total_usd)}`}>
          {seg.map(s => <div key={s.k} style={{ width: `${(s.v / cap) * 100}%`, background: s.c }} title={`${s.k}: ${usd(s.v)}`} />)}
        </div>
        {budget && budget < cap ? <div className="relative h-2"><span className="absolute top-0 w-px h-2 bg-ink" style={{ left: `${(budget / cap) * 100}%` }} title="budget" /></div> : null}
        <dl className={`text-[12px] grid ${seg.length > 3 ? 'grid-cols-2' : 'grid-cols-3'} gap-2 mt-3`}>
          {seg.map(s => (
            <div key={s.k}><dt className="text-muted flex items-center gap-1.5"><span className="inline-block w-2 h-2 rounded-sm" style={{ background: s.c }} />{s.k}</dt><dd className="text-ink tabular-nums mt-0.5">{usd(s.v)}</dd></div>
          ))}
        </dl>
        {estimated && <p className="text-xs text-muted mt-3">Flights, stay and gas are live prices. Place prices marked ~ and getting around are typical-price estimates, included so the total is all-in.</p>}
        {unknownCosts && <p className="text-xs text-muted mt-1">Some places have no price and count as $0.</p>}
      </div>

      <div className="grid gap-2">
        <h3 className="text-[19px]">Drive vs fly</h3>
        {plan.options.map(o => <OptionRow key={o.mode} o={o} best={o.mode === plan.recommended_mode} plan={plan} onReplan={onReplan} busy={busy} />)}
        {plan.options.length === 1 && <div className="text-[12px] text-dim">No airport pair within reach, so only driving is priced.</div>}
      </div>
    </div>
  )
}

function OptionRow({ o, best, plan, onReplan, busy }: { o: PlanOption; best: boolean; plan: Plan; onReplan?: (m: string) => void; busy?: boolean }) {
  const f = plan.flights[0]
  const d = plan.drive
  const refuels = d.refuels?.length ?? 0
  const detail = o.mode === 'drive'
    ? `${Math.round(d.distance_miles)} mi · ${hrs(d.duration_min)} each way · ${d.gallons_needed} gal @ $${d.gas_price_usd.toFixed(2)} (${d.vehicle_mpg} mpg)`
    : f ? `${f.carrier} ${f.origin}→${f.destination} · ${f.stops ? `${f.stops} stop` : 'nonstop'} · ${hrs(f.duration_min)} · ${usd(f.price_usd)}/person` : ''
  const fuel = o.mode === 'drive' && d.start_tank_usd !== undefined
    ? `Per direction: full tank ${usd(d.start_tank_usd)} (${d.tank_gallons} gal, ~${Math.round(d.range_miles ?? 0)} mi)` +
      (refuels ? ` + ${refuels} refuel${refuels > 1 ? 's' : ''} at ${usd(d.refuels![0].cost_usd)}${refuels > 1 && d.refuels![refuels - 1].cost_usd !== d.refuels![0].cost_usd ? ` and ${usd(d.refuels![refuels - 1].cost_usd)}` : ''}` : ', no refuel needed')
    : ''
  return (
    <div className={`rounded-lg border px-3.5 py-3 ${best ? 'border-accent/60 bg-accent-soft/50' : 'border-line'}`}>
      <div className="flex items-center justify-between gap-2">
        <div className="text-sm flex items-center gap-2"><Icon name={o.mode === 'fly' ? 'plane' : 'car'} size={15} className="text-muted" /><b className="font-medium capitalize">{o.mode}</b>{best && <span className="text-accent text-[11px]">Best value</span>}</div>
        <div className="text-sm text-ink tabular-nums">{usd(o.total_usd)}</div>
      </div>
      <div className="text-[12px] text-muted mt-1">{detail}</div>
      {fuel && <div className="text-[12px] text-amber mt-0.5">{fuel}</div>}
      <div className="text-[12px] text-dim mt-1 flex flex-wrap gap-x-3">
        <span>{o.travel_hours}h round trip</span>
        <span>value {o.value_score}</span>
        {o.factors.filter(x => x.includes('airport') || x.includes('rental')).map((x, i) => <span key={i} className="text-amber">{x}</span>)}
      </div>
      {!best && onReplan && (
        <button className="text-[12px] text-cyan hover:underline mt-1.5 print:hidden" disabled={busy} onClick={() => onReplan(o.mode)}>Re-plan the itinerary as {o.mode} →</button>
      )}
    </div>
  )
}
