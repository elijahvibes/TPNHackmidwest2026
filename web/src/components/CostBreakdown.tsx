import { hrs, usd, type Plan, type PlanOption } from '../lib/api'

const MODE_ICON: Record<string, string> = { drive: '🚗', fly: '✈' }

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
    { k: 'transport', v: rec.transport_usd, c: 'var(--color-cyan)' },
    { k: 'lodging', v: rec.lodging_usd, c: 'var(--color-rose)' },
    { k: 'things to do & food', v: rec.activities_usd, c: 'var(--color-accent)' },
  ]
  const over = budget ? rec.total_usd - budget : 0
  return (
    <div className="panel p-4 grid gap-4">
      <div>
        <div className="flex items-baseline justify-between">
          <h4 className="label">cost</h4>
          <span className="mono text-[11px] text-muted">
            {budget ? (over > 0 ? <span className="text-rose">{usd(over)} over the {usd(budget)} budget</span> : <span>{usd(budget - rec.total_usd)} under the {usd(budget)} budget</span>) : 'no budget set'}
          </span>
        </div>
        <div className="mt-2 h-3 rounded bg-line/60 overflow-hidden flex" role="img" aria-label={`total ${usd(rec.total_usd)}`}>
          {seg.map(s => <div key={s.k} style={{ width: `${(s.v / cap) * 100}%`, background: s.c }} title={`${s.k}: ${usd(s.v)}`} />)}
        </div>
        {budget && budget < cap ? <div className="relative h-2"><span className="absolute top-0 w-px h-2 bg-ink" style={{ left: `${(budget / cap) * 100}%` }} title="budget" /></div> : null}
        <dl className="mono text-xs grid grid-cols-3 gap-2 mt-2">
          {seg.map(s => (
            <div key={s.k}><dt className="text-dim flex items-center gap-1"><span className="inline-block w-2 h-2 rounded-sm" style={{ background: s.c }} />{s.k}</dt><dd className="text-ink">{usd(s.v)}</dd></div>
          ))}
        </dl>
      </div>

      <div className="grid gap-2">
        <h4 className="label">drive vs fly</h4>
        {plan.options.map(o => <OptionRow key={o.mode} o={o} best={o.mode === plan.recommended_mode} plan={plan} onReplan={onReplan} busy={busy} />)}
        {plan.options.length === 1 && <div className="mono text-[11px] text-dim">no airport pair within reach, so only driving is priced</div>}
      </div>
    </div>
  )
}

function OptionRow({ o, best, plan, onReplan, busy }: { o: PlanOption; best: boolean; plan: Plan; onReplan?: (m: string) => void; busy?: boolean }) {
  const f = plan.flights[0]
  const detail = o.mode === 'drive'
    ? `${Math.round(plan.drive.distance_miles)} mi · ${hrs(plan.drive.duration_min)} each way · ${plan.drive.gallons_needed} gal @ $${plan.drive.gas_price_usd.toFixed(2)} (${plan.drive.vehicle_mpg} mpg)`
    : f ? `${f.carrier} ${f.origin}→${f.destination} · ${f.stops ? `${f.stops} stop` : 'nonstop'} · ${hrs(f.duration_min)} · ${usd(f.price_usd)}/person` : ''
  return (
    <div className={`rounded-md border px-3 py-2 ${best ? 'border-accent/70 bg-accent/5' : 'border-line'}`}>
      <div className="flex items-center justify-between gap-2">
        <div className="mono text-sm"><span className="mr-1.5">{MODE_ICON[o.mode]}</span><b className="uppercase">{o.mode}</b>{best && <span className="text-accent text-[10px] ml-2">● best value</span>}</div>
        <div className="mono text-sm text-ink tabular-nums">{usd(o.total_usd)}</div>
      </div>
      <div className="mono text-[11px] text-muted mt-1">{detail}</div>
      <div className="mono text-[11px] text-dim mt-1 flex flex-wrap gap-x-3">
        <span>{o.travel_hours}h round trip</span>
        <span>value {o.value_score}</span>
        {o.factors.filter(x => x.includes('airport') || x.includes('rental')).map((x, i) => <span key={i} className="text-amber">{x}</span>)}
      </div>
      {!best && onReplan && (
        <button className="mono text-[11px] text-cyan hover:underline mt-1 print:hidden" disabled={busy} onClick={() => onReplan(o.mode)}>re-plan the itinerary as {o.mode} →</button>
      )}
    </div>
  )
}
