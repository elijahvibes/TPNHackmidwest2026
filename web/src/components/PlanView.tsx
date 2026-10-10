import { hrs, usd, type Plan } from '../lib/api'
import FlightChooser from './FlightChooser'

const catColor: Record<string, string> = {
  transport: 'text-cyan', lodging: 'text-amber', activity: 'text-accent', food: 'text-rose', stop: 'text-muted',
}

export default function PlanView({ plan, onSelectFlight, disabled }: {
  plan: Plan; onSelectFlight?: (index: number, returnToken?: string) => Promise<void>; disabled?: boolean
}) {
  const best = plan.options.find(o => o.mode === plan.recommended_mode) ?? plan.options[0]
  return (
    <div className="grid gap-6">
      {/* headline */}
      <section className="panel p-5">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h2 className="mono text-xl font-bold">{plan.title}</h2>
            <p className="text-muted mt-2 max-w-2xl">{plan.summary}</p>
          </div>
          <div className="text-right">
            <div className="label">all-in estimate</div>
            <div className={`mono text-3xl font-bold ${plan.under_budget ? 'text-accent' : 'text-rose'}`}>{usd(plan.total_usd)}</div>
            <div className="mono text-[11px] text-muted">{plan.nights} nights · {plan.under_budget ? 'under budget' : 'over budget'}</div>
          </div>
        </div>
        <div className="flex flex-wrap gap-2 mt-4">
          <span className="tag">{plan.flight_selected ? 'selected' : 'best'}: <b className="text-accent">{plan.recommended_mode}</b></span>
          {Object.entries(plan.sources).map(([k, v]) => (
            <span key={k} className="tag">{k}: <span className={v === 'mock' || v === 'fallback' || v === 'estimate' ? 'text-dim' : 'text-cyan'}>{v}</span></span>
          ))}
          {plan.ai_generated && <span className="tag text-accent">✦ itinerary by AI on Akamai</span>}
        </div>
        {(plan.sources.flights === 'serpapi' || plan.sources.hotels === 'serpapi') && (
          <p className="text-dim text-xs mt-3">Flight and stay prices are Google Travel search results via SerpApi, not guaranteed booking quotes. Optional fees and final checkout prices may differ.</p>
        )}
      </section>

      {/* value comparison */}
      <section className="grid sm:grid-cols-2 gap-4">
        {plan.options.map(o => (
          <div key={o.mode} className={`panel p-4 ${o.mode === best.mode ? 'border-accent' : ''}`}>
            <div className="flex items-center justify-between">
              <span className="mono font-bold uppercase">{o.mode}</span>
              <span className="tag">value {o.value_score}</span>
            </div>
            <div className="mono text-2xl mt-2">{usd(o.total_usd)}</div>
            <div className="text-muted text-sm mt-1">{o.why}</div>
            <dl className="mono text-xs grid grid-cols-3 gap-2 mt-3 text-muted">
              <div><dt className="text-dim">transport</dt><dd>{usd(o.transport_usd)}</dd></div>
              <div><dt className="text-dim">lodging</dt><dd>{usd(o.lodging_usd)}</dd></div>
              <div><dt className="text-dim">travel time</dt><dd>{o.travel_hours}h</dd></div>
            </dl>
          </div>
        ))}
      </section>

      {onSelectFlight && <FlightChooser key={plan.generated_at} plan={plan} onSelect={onSelectFlight} disabled={disabled} />}

      {/* day by day */}
      <section className="grid gap-3">
        <h3 className="mono prompt text-sm text-muted">itinerary</h3>
        {plan.days.map(d => (
          <div key={d.day} className="panel p-4">
            <div className="flex items-baseline gap-3">
              <span className="mono text-accent font-bold">day {d.day}</span>
              <span className="mono text-xs text-muted">{d.date}</span>
              <span className="text-sm text-ink">{d.theme}</span>
            </div>
            <ul className="mt-3 grid gap-2">
              {d.items.map((it, i) => (
                <li key={i} className="grid grid-cols-[52px_1fr_auto] gap-3 text-sm">
                  <span className="mono text-dim">{it.time}</span>
                  <span>
                    <span className={`mono text-[10px] uppercase mr-2 ${catColor[it.category] ?? 'text-muted'}`}>{it.category}</span>
                    {it.title}
                    {it.detail && <span className="text-muted"> — {it.detail}</span>}
                  </span>
                  <span className="mono text-muted">{it.cost_usd ? usd(it.cost_usd) : 'free'}</span>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </section>

      {/* raw data */}
      <section className="grid md:grid-cols-3 gap-4">
        <div className="panel p-4">
          <h4 className="label">drive</h4>
          <div className="mono text-sm mt-2 grid gap-1">
            <div>{plan.drive.distance_miles} mi · {hrs(plan.drive.duration_min)}</div>
            <div>{plan.drive.gallons_needed} gal @ ${plan.drive.gas_price_usd.toFixed(2)}/gal ({plan.drive.vehicle_mpg} mpg)</div>
            <div className="text-accent">{usd(plan.drive.round_trip_usd)} round trip</div>
          </div>
        </div>
        <div className="panel p-4">
          <h4 className="label">flights</h4>
          <p className="text-dim text-xs mt-1">Round trip per traveler</p>
          <ul className="mono text-sm mt-2 grid gap-1">
            {plan.flights.slice(plan.selected_flight ?? 0, (plan.selected_flight ?? 0) + 1).map((f, i) => (
              <li key={i} className="flex justify-between"><span>{f.carrier} · {f.stops ? `${f.stops} stop` : 'nonstop'} · {hrs(f.duration_min)}</span><span className="text-accent">{usd(plan.selected_return?.price_usd ?? f.price_usd)}</span></li>
            ))}
            {plan.flights.length === 0 && <li className="text-dim">no airport match</li>}
          </ul>
        </div>
        <div className="panel p-4">
          <h4 className="label">stay</h4>
          <ul className="mono text-sm mt-2 grid gap-1">
            {plan.hotels.slice(0, 3).map((h, i) => (
              <li key={i} className="flex justify-between gap-2"><span className="truncate">{h.name} <span className="text-dim">★{h.rating.toFixed(1)}</span></span><span className="text-accent whitespace-nowrap">{usd(h.nightly_usd)}/n</span></li>
            ))}
          </ul>
        </div>
      </section>

      <section className="grid md:grid-cols-3 gap-4">
        {([['things to do', plan.activities], ['eat', plan.food], ['stops on the way', plan.stops]] as const).map(([title, list]) => (
          <div key={title} className="panel p-4">
            <h4 className="label">{title}</h4>
            <ul className="text-sm mt-2 grid gap-1.5">
              {list.map((p, i) => (
                <li key={i} className="flex justify-between gap-2">
                  <span>{p.name}{p.note && <span className="text-dim"> · {p.note}</span>}</span>
                  <span className="mono text-muted whitespace-nowrap">{p.price_usd ? usd(p.price_usd) : 'free'}</span>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </section>
    </div>
  )
}
