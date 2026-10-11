import { useState } from 'react'
import { fmtDate, type Safety, type Weather } from '../lib/api'
import Icon, { WeatherIcon } from './Icon'

export function WeatherStrip({ w }: { w: Weather }) {
  if (!w?.days?.length) return null
  return (
    <div className="panel p-5">
      <div className="flex items-baseline justify-between gap-3">
        <h3 className="text-[19px] truncate">Weather · {w.place}</h3>
        <span className={`tag shrink-0 ${w.source === 'nws' ? 'text-cyan' : 'text-dim'}`}>{w.source === 'nws' ? 'NWS forecast' : 'Typical conditions'}</span>
      </div>
      <div className="mt-3 flex gap-2 overflow-x-auto min-w-0 scrollbar-none">
        {w.days.map(d => (
          <div key={d.date} className={`w-[92px] shrink-0 rounded-lg border p-2.5 text-center ${d.precip_pct >= 50 ? 'border-cyan/40 bg-cyan/5' : 'border-line'}`}>
            <div className="text-[11px] text-muted">{fmtDate(d.date, { weekday: 'short' })} <span className="text-dim">{fmtDate(d.date, { day: 'numeric' })}</span></div>
            <div className="my-1.5 flex justify-center text-ink" aria-label={d.short}><WeatherIcon icon={d.icon} size={22} /></div>
            <div className="text-sm tabular-nums"><span className="text-ink">{d.high_f}°</span><span className="text-dim"> / {d.low_f}°</span></div>
            <div className="text-[11px] text-muted truncate" title={d.short}>{d.short}</div>
            {d.precip_pct > 0 && <div className="text-[11px] text-cyan">{d.precip_pct}% rain</div>}
          </div>
        ))}
      </div>
      {w.note && <div className="text-[12px] text-dim mt-2">{w.note}</div>}
    </div>
  )
}

const levelStyle: Record<Safety['level'], string> = {
  low: 'text-accent border-accent/50', moderate: 'text-amber border-amber/50', elevated: 'text-rose border-rose/50',
}

export function SafetyCard({ s }: { s: Safety }) {
  const [open, setOpen] = useState(s?.level === 'elevated')
  if (!s?.level) return null
  return (
    <div className="panel p-5">
      <div className="flex items-center justify-between gap-3">
        <h3 className="text-[19px] truncate">Safety · {s.city}</h3>
        <button className={`tag shrink-0 inline-flex items-center gap-1 capitalize ${levelStyle[s.level]}`} onClick={() => setOpen(o => !o)} aria-expanded={open}>
          {s.level} · {s.score}/100 <Icon name="chevron" size={12} className={open ? 'rotate-180' : ''} />
        </button>
      </div>
      <div className="mt-3 h-1.5 rounded-full bg-line overflow-hidden">
        <div className={`h-full ${s.level === 'low' ? 'bg-accent' : s.level === 'moderate' ? 'bg-amber' : 'bg-rose'}`} style={{ width: `${s.score}%` }} />
      </div>
      {open && (
        <ul className="mt-3 grid gap-1.5 text-sm text-muted list-disc pl-4">
          {s.tips.map((t, i) => <li key={i}>{t}</li>)}
          <li className="list-none -ml-4 text-[11px] text-dim mt-1">Source: {s.source}</li>
        </ul>
      )}
    </div>
  )
}
