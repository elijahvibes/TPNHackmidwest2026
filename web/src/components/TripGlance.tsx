import { useEffect, useRef, useState, type ReactNode } from 'react'
import { fmtDate, usd } from '../lib/api'
import { commonsFileURL, placePhoto, type PlacePhoto } from '../lib/placeImages'
import Icon from './Icon'

/** Illustrated fallback: an autumn trail toward the mountains, in the page palette. */
export function TrailScene({ className = '' }: { className?: string }) {
  return (
    <svg viewBox="0 0 400 220" className={`w-full h-full block ${className}`} preserveAspectRatio="xMidYMid slice" role="img" aria-label="A trail winding toward mountains">
      <defs>
        <linearGradient id="sky" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#dfe7ee" /><stop offset="1" stopColor="#f3efe6" /></linearGradient>
        <linearGradient id="ground" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#b9b58a" /><stop offset="1" stopColor="#8d8d5e" /></linearGradient>
      </defs>
      <rect width="400" height="220" fill="url(#sky)" />
      <path d="M0 120 L60 70 L110 100 L160 55 L215 95 L265 60 L320 100 L370 75 L400 95 V220 H0 Z" fill="#9aa5ad" />
      <path d="M150 65 L160 55 L172 66 Z M255 70 L265 60 L278 71 Z M50 78 L60 70 L70 80 Z" fill="#eef1f3" />
      <path d="M0 140 L70 110 L140 135 L230 105 L310 130 L400 110 V220 H0 Z" fill="#6f7f66" />
      <path d="M0 165 L90 140 L180 160 L280 135 L400 155 V220 H0 Z" fill="url(#ground)" />
      {[[30, 150, 1.1], [70, 160, 1.3], [120, 150, 1], [300, 150, 1.2], [340, 165, 1.4], [375, 150, 1]].map(([x, y, s], i) => (
        <g key={i} transform={`translate(${x} ${y}) scale(${s})`}>
          <rect x="-1.2" y="0" width="2.4" height="18" fill="#6b5b45" />
          <ellipse cx="0" cy="-6" rx="9" ry="13" fill={i % 2 ? '#d9a441' : '#c98a2e'} />
          <ellipse cx="-3" cy="-9" rx="4" ry="6" fill="#e8bd5a" opacity="0.7" />
        </g>
      ))}
      {[[200, 150, 1], [230, 160, 1.3], [255, 148, 0.9]].map(([x, y, s], i) => (
        <g key={`p${i}`} transform={`translate(${x} ${y}) scale(${s})`}>
          <path d="M0 -26 L10 0 H-10 Z" fill="#3f5a3d" /><path d="M0 -18 L12 8 H-12 Z" fill="#4a6a45" /><rect x="-1.2" y="8" width="2.4" height="8" fill="#5a4a38" />
        </g>
      ))}
      <path d="M190 220 C 200 190, 160 180, 175 165 C 190 150, 215 150, 205 135" stroke="#cbb48c" strokeWidth="14" fill="none" strokeLinecap="round" />
      <path d="M190 220 C 200 190, 160 180, 175 165 C 190 150, 215 150, 205 135" stroke="#b89d73" strokeWidth="3" strokeDasharray="4 10" fill="none" strokeLinecap="round" opacity="0.6" />
    </svg>
  )
}

/**
 * The photo for a destination, chosen by what kind of place it is. The drawn
 * trail scene sits underneath, so it shows while the photo streams in and
 * stays if the photo cannot be fetched. The photographer is credited in the
 * corner.
 */
export function PlacePicture({ name, kind, className = 'aspect-[16/10]', credit = true, rounded = 'rounded-lg', lazy = false }: {
  name: string; kind?: string; className?: string; credit?: boolean; rounded?: string
  /** defer fetching until near the viewport (for grids of cards; the hero image of a page should load at once) */
  lazy?: boolean
}) {
  const photo: PlacePhoto = placePhoto(name, kind)
  const [failed, setFailed] = useState(false)
  const img = useRef<HTMLImageElement>(null)
  useEffect(() => {
    const el = img.current
    // A cached image can finish (or fail) before React attaches onError, so ask the element.
    setFailed(!!el && el.complete && el.naturalWidth === 0)
  }, [photo.src])
  return (
    <div className={`relative overflow-hidden border border-line bg-panel-2 ${rounded} ${className}`}>
      <div className="absolute inset-0"><TrailScene /></div>
      {!failed && (
        <img ref={img} src={photo.src} alt={photo.alt} loading={lazy ? 'lazy' : 'eager'} decoding="async"
          onError={() => setFailed(true)} className="absolute inset-0 w-full h-full object-cover" />
      )}
      {credit && !failed && (
        <a href={commonsFileURL(photo)} target="_blank" rel="noreferrer noopener"
          className="absolute bottom-1.5 right-2 text-[10px] text-white/80 hover:text-white bg-ink/40 rounded px-1.5 py-0.5 backdrop-blur-sm"
          title={`${photo.author} · ${photo.license} · Wikimedia Commons`}>© {photo.author}</a>
      )}
    </div>
  )
}

/**
 * The sticky "your trip at a glance" card beside the planner: a live mirror
 * of what the form says, plus the one button that starts the plan.
 */
export default function TripGlance({ origin, destination, destKind, start, end, nights, travelers, budget, style, dateErr, action }: {
  origin: string; destination: string; destKind?: string; start: string; end: string; nights: number; travelers: number
  budget?: number; style: string[]; dateErr?: string; action: ReactNode
}) {
  const dest = (destination || 'Your').split(',')[0].trim()
  const title = `${dest} Trip`
  const dates = Number.isNaN(nights) ? 'Pick your dates'
    : fmtDate(start, { month: 'short', day: 'numeric' }) === fmtDate(end, { month: 'short', day: 'numeric' })
      ? fmtDate(start, { month: 'short', day: 'numeric', year: 'numeric' })
      : `${fmtDate(start, { month: 'short', day: 'numeric' })}–${fmtDate(end, { day: 'numeric' })}, ${new Date(end + 'T12:00:00').getFullYear()}`
  const nightsLabel = dateErr ? dateErr : nights === 0 ? 'Day trip' : `${nights} night${nights === 1 ? '' : 's'}`
  return (
    <aside className="panel p-6 grid gap-5 lg:sticky lg:top-24">
      <h2 className="text-[22px] text-ink">Your trip at a glance</h2>
      <PlacePicture name={destination} kind={destKind} />
      <div>
        <div className="flex items-start justify-between gap-3">
          <h3 className="text-[30px] leading-tight text-ink truncate">{title}</h3>
          <Icon name="pencil" size={18} className="text-muted mt-2 shrink-0" />
        </div>
        <div className="text-[14px] text-muted mt-1 flex items-center gap-2 min-w-0">
          <span className="truncate">{origin.split(',')[0] || '…'}</span>
          <Icon name="arrow" size={14} className="shrink-0" />
          <span className="truncate">{dest}</span>
        </div>
      </div>
      <dl className="grid gap-2.5 text-[14px]">
        <div className="flex items-center justify-between gap-3">
          <dt className="flex items-center gap-2 text-ink"><Icon name="calendar" size={16} className="text-muted" />{dates}</dt>
          <dd className={`text-[13px] ${dateErr ? 'text-rose' : 'text-muted'}`}>{nightsLabel}</dd>
        </div>
        <div className="flex items-center justify-between gap-3">
          <dt className="flex items-center gap-2 text-ink"><Icon name="users" size={16} className="text-muted" />{travelers} {travelers === 1 ? 'adult' : 'adults'}</dt>
          <dd className="text-[13px] text-muted">1 getaway</dd>
        </div>
      </dl>
      <div className="rounded-lg bg-accent-soft/70 px-4 py-3.5">
        <div className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Whole-trip budget</div>
        <div className="flex items-baseline justify-between gap-3 mt-1">
          <div className="serif text-[32px] leading-none text-ink">{budget ? usd(budget) : 'No limit'}</div>
          {budget ? <div className="text-[13px] text-muted">{usd(budget / Math.max(travelers, 1))} / person</div> : null}
        </div>
        <div className="text-[13px] text-muted mt-1.5">Travel + stays + activities</div>
      </div>
      <div>
        <div className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Your travel style</div>
        <div className="text-[14px] text-ink mt-1">{style.join(' · ')}</div>
      </div>
      {action}
      <p className="text-[12px] text-muted text-center leading-relaxed -mt-1">We'll compare whole-trip costs and build your itinerary. Nothing is booked yet.</p>
    </aside>
  )
}
