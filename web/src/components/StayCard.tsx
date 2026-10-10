import { useState } from 'react'
import { usd, type HotelQuote } from '../lib/api'

const KIND_LABEL: Record<string, string> = { hotel: 'hotel', rental: 'rental', inn: 'inn / b&b', resort: 'resort' }

function Stars({ r }: { r: number }) {
  const full = Math.round(r)
  return <span className="text-amber tracking-tighter" aria-label={`${r.toFixed(1)} stars`}>{'★'.repeat(Math.min(5, full))}<span className="text-line">{'★'.repeat(Math.max(0, 5 - full))}</span></span>
}

/**
 * The chosen stay, with the alternatives the planner priced. Choosing another
 * one re-totals the whole plan (server-side for saved trips).
 */
export default function StayCard({ hotels, budgetLeft, onChoose, source }: {
  hotels: HotelQuote[]
  budgetLeft?: number // budget minus everything but lodging; undefined = no budget
  onChoose?: (name: string) => Promise<void> | void
  source?: string
}) {
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState('')
  const h = hotels[0]
  if (!h) return (
    <div className="panel p-4"><div className="label">your stay</div><div className="mono text-sm text-dim mt-2">no lodging found near the destination</div></div>
  )
  const choose = async (name: string) => {
    if (!onChoose) return
    setBusy(name)
    try { await onChoose(name); setOpen(false) } finally { setBusy('') }
  }
  return (
    <div className="panel p-4 grid gap-3">
      <div className="flex items-baseline justify-between gap-3">
        <h4 className="label">your stay</h4>
        <span className={`tag ${source === 'amadeus' ? 'text-cyan' : 'text-dim'}`}>{source === 'amadeus' ? 'live rates · amadeus' : 'sample rates'}</span>
      </div>
      <div>
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="font-medium text-ink leading-snug">{h.name}</div>
            <div className="mono text-[11px] text-muted mt-0.5 flex flex-wrap gap-x-2">
              <Stars r={h.rating} /><span>{h.rating.toFixed(1)}</span>
              <span>· {KIND_LABEL[h.kind] ?? h.kind}</span>
              {(h.area || h.distance_km > 0) && <span>· {h.area ?? ''}{h.distance_km > 0 ? ` (${h.distance_km.toFixed(1)} km from center)` : ''}</span>}
            </div>
          </div>
          <div className="text-right shrink-0">
            <div className="mono text-xl text-ink leading-none">{usd(h.nightly_usd)}<span className="text-xs text-muted">/night</span></div>
            <div className="mono text-[11px] text-muted mt-1">× {h.nights} = <b className="text-accent">{usd(h.total_usd)}</b></div>
          </div>
        </div>
        {h.amenities && h.amenities.length > 0 && (
          <div className="flex flex-wrap gap-1.5 mt-2">
            {h.amenities.map(a => <span key={a} className="tag !text-[10px]">{a}</span>)}
          </div>
        )}
        {budgetLeft !== undefined && (
          <div className={`mono text-[11px] mt-2 ${h.total_usd <= budgetLeft ? 'text-dim' : 'text-rose'}`}>
            {h.total_usd <= budgetLeft ? `fits: ${usd(budgetLeft - h.total_usd)} of budget left after lodging` : `${usd(h.total_usd - budgetLeft)} over what the budget leaves for lodging`}
          </div>
        )}
      </div>
      <div className="flex flex-wrap gap-2 print:hidden">
        {h.url && <a href={h.url} target="_blank" rel="noreferrer noopener" className="btn-ghost !py-1 !text-xs">look up / book ↗</a>}
        {hotels.length > 1 && (
          <button className="btn-ghost !py-1 !text-xs" onClick={() => setOpen(o => !o)} aria-expanded={open}>
            {open ? 'hide' : `change stay (${hotels.length - 1} more)`}
          </button>
        )}
      </div>
      {open && (
        <ul className="grid gap-1.5 border-t border-line pt-3">
          {hotels.slice(1).map(alt => {
            const diff = alt.total_usd - h.total_usd
            return (
              <li key={alt.name} className="grid grid-cols-[1fr_auto] gap-2 items-center rounded-md border border-line px-3 py-2 hover:border-muted">
                <div className="min-w-0">
                  <div className="text-sm text-ink truncate">{alt.name}</div>
                  <div className="mono text-[11px] text-muted flex flex-wrap gap-x-2">
                    <span className="text-amber">★ {alt.rating.toFixed(1)}</span>
                    <span>{KIND_LABEL[alt.kind] ?? alt.kind}</span>
                    {alt.area && <span>· {alt.area}</span>}
                    {budgetLeft !== undefined && alt.total_usd > budgetLeft && <span className="text-rose">· over budget</span>}
                  </div>
                </div>
                <div className="text-right">
                  <div className="mono text-sm text-ink">{usd(alt.nightly_usd)}<span className="text-[10px] text-muted">/n</span></div>
                  <div className={`mono text-[10px] ${diff > 0 ? 'text-rose' : 'text-accent'}`}>{diff > 0 ? '+' : '−'}{usd(Math.abs(diff))} total</div>
                  {onChoose && (
                    <button className="mono text-[11px] text-cyan hover:underline mt-0.5" disabled={!!busy} onClick={() => choose(alt.name)}>
                      {busy === alt.name ? 'switching…' : 'choose'}
                    </button>
                  )}
                </div>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
