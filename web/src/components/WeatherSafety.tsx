import { useState } from 'react'
import type { Safety, Weather } from '../lib/api'

const ICON: Record<string, string> = { sun: '☀', cloud: '☁', rain: '🌧', snow: '❄', storm: '⛈', wind: '💨', fog: '🌫' }

export function WeatherStrip({ w }: { w: Weather }) {
  if (!w?.days?.length) return null
  return (
    <div className="panel p-4">
      <div className="flex items-baseline justify-between">
        <h4 className="label">weather · {w.place}</h4>
        <span className={`tag ${w.source === 'nws' ? 'text-cyan' : 'text-dim'}`}>{w.source === 'nws' ? 'NWS forecast' : 'typical conditions'}</span>
      </div>
      <div className="mt-3 flex gap-2 overflow-x-auto">
        {w.days.map(d => (
          <div key={d.date} className={`min-w-[96px] rounded-md border p-2 text-center ${d.precip_pct >= 50 ? 'border-cyan/50 bg-cyan/5' : 'border-line'}`}>
            <div className="mono text-[10px] text-muted">{d.label?.slice(0, 3) || d.date.slice(5)}</div>
            <div className="text-2xl leading-tight" aria-label={d.short}>{ICON[d.icon] ?? '☀'}</div>
            <div className="mono text-sm"><span className="text-ink">{d.high_f}°</span><span className="text-dim"> / {d.low_f}°</span></div>
            <div className="mono text-[10px] text-muted truncate" title={d.short}>{d.short}</div>
            {d.precip_pct > 0 && <div className="mono text-[10px] text-cyan">{d.precip_pct}% rain</div>}
          </div>
        ))}
      </div>
      {w.note && <div className="mono text-[11px] text-dim mt-2">{w.note}</div>}
    </div>
  )
}

const levelStyle: Record<Safety['level'], string> = {
  low: 'text-accent border-accent/60', moderate: 'text-amber border-amber/60', elevated: 'text-rose border-rose/60',
}

export function SafetyCard({ s }: { s: Safety }) {
  const [open, setOpen] = useState(s?.level === 'elevated')
  if (!s?.level) return null
  return (
    <div className="panel p-4">
      <div className="flex items-center justify-between gap-3">
        <h4 className="label">safety · {s.city}</h4>
        <button className={`tag ${levelStyle[s.level]}`} onClick={() => setOpen(o => !o)} aria-expanded={open}>
          {s.level} · score {s.score}/100 {open ? '▾' : '▸'}
        </button>
      </div>
      <div className="mt-2 h-1.5 rounded bg-line overflow-hidden">
        <div className={`h-full ${s.level === 'low' ? 'bg-accent' : s.level === 'moderate' ? 'bg-amber' : 'bg-rose'}`} style={{ width: `${s.score}%` }} />
      </div>
      {open && (
        <ul className="mt-3 grid gap-1 text-sm text-muted">
          {s.tips.map((t, i) => <li key={i}>· {t}</li>)}
          <li className="mono text-[10px] text-dim mt-1">source: {s.source}</li>
        </ul>
      )}
    </div>
  )
}
