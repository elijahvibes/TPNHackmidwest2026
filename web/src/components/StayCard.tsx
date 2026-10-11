import { useState } from 'react'
import { usd, type HotelQuote } from '../lib/api'

const KIND_LABEL: Record<string, string> = { hotel: 'Hotel', rental: 'Rental', inn: 'Inn / B&B', resort: 'Resort' }

function Stars({ r }: { r: number }) {
  const full = Math.round(r)
  return <span className="text-amber tracking-tight" aria-label={`${r.toFixed(1)} stars`}>{'★'.repeat(Math.min(5, full))}<span className="text-line">{'★'.repeat(Math.max(0, 5 - full))}</span></span>
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
    <div className="panel p-5"><h3 className="text-[19px]">Your stay</h3><div className="text-sm text-dim mt-2">No lodging found near the destination.</div></div>
  )
  const choose = async (name: string) => {
    if (!onChoose) return
    setBusy(name)
    try { await onChoose(name); setOpen(false) } finally { setBusy('') }
  }
  return (
    <div className="panel p-5 grid gap-4">
      <div className="flex items-baseline justify-between gap-3">
        <h3 className="text-[19px]">Your stay</h3>
        <span className={`tag ${source === 'serpapi' ? 'text-cyan' : 'text-dim'}`}>{source === 'serpapi' ? 'Live rates · SerpApi' : 'Sample rates'}</span>
      </div>
      <div>
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="text-[15px] text-ink leading-snug">{h.name}</div>
            <div className="text-[12px] text-muted mt-1 flex flex-wrap gap-x-2">
              <Stars r={h.rating} /><span>{h.rating.toFixed(1)}</span>
              <span>· {KIND_LABEL[h.kind] ?? h.kind}</span>
              {(h.area || h.distance_km > 0) && <span>· {h.area ?? ''}{h.distance_km > 0 ? ` (${h.distance_km.toFixed(1)} km from center)` : ''}</span>}
            </div>
          </div>
          <div className="text-right shrink-0">
            <div className="serif text-[22px] text-ink leading-none tabular-nums">{usd(h.nightly_usd)}<span className="font-sans text-xs text-muted"> /night</span></div>
            <div className="text-[12px] text-muted mt-1">× {h.nights} = <b className="text-ink font-medium tabular-nums">{usd(h.total_usd)}</b></div>
          </div>
        </div>
        {h.amenities && h.amenities.length > 0 && (
          <div className="flex flex-wrap gap-1.5 mt-2.5">
            {h.amenities.map(a => <span key={a} className="tag !text-[10px]">{a}</span>)}
          </div>
        )}
        {budgetLeft !== undefined && (
          <div className={`text-[12px] mt-2.5 ${h.total_usd <= budgetLeft ? 'text-muted' : 'text-rose'}`}>
            {h.total_usd <= budgetLeft ? `Fits: ${usd(budgetLeft - h.total_usd)} of budget left after lodging` : `${usd(h.total_usd - budgetLeft)} over what the budget leaves for lodging`}
          </div>
        )}
      </div>
      <div className="flex flex-wrap gap-2 print:hidden">
        {h.url && <a href={h.url} target="_blank" rel="noreferrer noopener" className="btn-ghost !py-1.5 !text-[13px]">Look up / book ↗</a>}
        {hotels.length > 1 && (
          <button className="btn-ghost !py-1.5 !text-[13px]" onClick={() => setOpen(o => !o)} aria-expanded={open}>
            {open ? 'Hide options' : `Change stay (${hotels.length - 1} more)`}
          </button>
        )}
      </div>
      {open && (
        <ul className="grid gap-2 border-t border-line pt-4">
          {hotels.slice(1).map(alt => {
            const diff = alt.total_usd - h.total_usd
            return (
              <li key={alt.name} className="grid grid-cols-[1fr_auto] gap-2 items-center rounded-lg border border-line px-3.5 py-2.5 hover:border-ink/40">
                <div className="min-w-0">
                  <div className="text-sm text-ink truncate">{alt.name}</div>
                  <div className="text-[12px] text-muted flex flex-wrap gap-x-2">
                    <span className="text-amber">★ {alt.rating.toFixed(1)}</span>
                    <span>{KIND_LABEL[alt.kind] ?? alt.kind}</span>
                    {alt.area && <span>· {alt.area}</span>}
                    {budgetLeft !== undefined && alt.total_usd > budgetLeft && <span className="text-rose">· over budget</span>}
                  </div>
                </div>
                <div className="text-right">
                  <div className="text-sm text-ink tabular-nums">{usd(alt.nightly_usd)}<span className="text-[10px] text-muted"> /n</span></div>
                  <div className={`text-[11px] tabular-nums ${diff > 0 ? 'text-rose' : 'text-accent'}`}>{diff > 0 ? '+' : '−'}{usd(Math.abs(diff))} total</div>
                  {onChoose && (
                    <button className="text-[12px] text-cyan hover:underline mt-0.5" disabled={!!busy} onClick={() => choose(alt.name)}>
                      {busy === alt.name ? 'Switching…' : 'Choose'}
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
