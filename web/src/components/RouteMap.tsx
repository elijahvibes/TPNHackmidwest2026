import type { LatLng, Place } from '../lib/api'

type Pin = { at: LatLng; label: string; kind: 'origin' | 'dest' | 'stop' | 'activity' }

/**
 * Dependency-free route sketch: projects lat/lng into an SVG box with padding
 * and draws origin → destination with stops along the way. Looks right at any
 * scale and needs no map token; swap for Mapbox GL later if a key shows up.
 */
export default function RouteMap({ origin, dest, stops, activities, originLabel, destLabel }: {
  origin: LatLng; dest: LatLng; stops: Place[]; activities: Place[]; originLabel: string; destLabel: string
}) {
  const pins: Pin[] = [
    { at: origin, label: originLabel, kind: 'origin' },
    ...stops.filter(s => s.location?.lat).map(s => ({ at: s.location, label: s.name, kind: 'stop' as const })),
    ...activities.slice(0, 3).filter(a => a.location?.lat).map(a => ({ at: a.location, label: a.name, kind: 'activity' as const })),
    { at: dest, label: destLabel, kind: 'dest' },
  ]
  const W = 720, H = 300, PAD = 48
  const lats = pins.map(p => p.at.lat), lngs = pins.map(p => p.at.lng)
  const minLat = Math.min(...lats), maxLat = Math.max(...lats)
  const minLng = Math.min(...lngs), maxLng = Math.max(...lngs)
  // keep aspect sane for short trips; lng degrees shrink with cos(lat)
  const midLat = (minLat + maxLat) / 2
  const kx = Math.cos((midLat * Math.PI) / 180)
  const spanLng = Math.max((maxLng - minLng) * kx, 0.5), spanLat = Math.max(maxLat - minLat, 0.5)
  const scale = Math.min((W - 2 * PAD) / spanLng, (H - 2 * PAD) / spanLat)
  const cx = ((minLng + maxLng) / 2) * kx, cy = midLat
  const X = (ll: LatLng) => W / 2 + (ll.lng * kx - cx) * scale
  const Y = (ll: LatLng) => H / 2 - (ll.lat - cy) * scale

  const color: Record<Pin['kind'], string> = { origin: 'var(--color-cyan)', dest: 'var(--color-accent)', stop: 'var(--color-muted)', activity: 'var(--color-amber)' }
  const path = `M ${X(origin)} ${Y(origin)} L ${X(dest)} ${Y(dest)}`

  return (
    <div className="panel p-3 overflow-hidden">
      <div className="label mb-2">route</div>
      <svg viewBox={`0 0 ${W} ${H}`} className="w-full h-auto font-mono" role="img" aria-label={`Route from ${originLabel} to ${destLabel}`}>
        <defs>
          <pattern id="grid" width="24" height="24" patternUnits="userSpaceOnUse">
            <path d="M 24 0 L 0 0 0 24" fill="none" stroke="var(--color-line)" strokeWidth="0.6" />
          </pattern>
        </defs>
        <rect width={W} height={H} fill="url(#grid)" />
        <path d={path} stroke="var(--color-accent)" strokeWidth="2" strokeDasharray="6 5" fill="none" opacity="0.8" />
        {pins.map((p, i) => {
          const x = X(p.at), y = Y(p.at)
          const big = p.kind === 'origin' || p.kind === 'dest'
          const anchor = x > W * 0.7 ? 'end' : 'start'
          const dx = anchor === 'end' ? -10 : 10
          return (
            <g key={i}>
              {big && <circle cx={x} cy={y} r="11" fill={color[p.kind]} opacity="0.18" />}
              <circle cx={x} cy={y} r={big ? 5 : 3.5} fill={color[p.kind]} stroke="var(--color-bg)" strokeWidth="1.5" />
              <text x={x + dx} y={y + 4} fontSize={big ? 12 : 10} fill={big ? 'var(--color-ink)' : 'var(--color-muted)'} textAnchor={anchor}>
                {p.label.length > 26 ? p.label.slice(0, 24) + '…' : p.label}
              </text>
            </g>
          )
        })}
      </svg>
      <div className="mono text-[10px] text-dim mt-1 flex gap-4">
        <span><span style={{ color: 'var(--color-cyan)' }}>●</span> origin</span>
        <span><span style={{ color: 'var(--color-accent)' }}>●</span> destination</span>
        <span><span style={{ color: 'var(--color-muted)' }}>●</span> stop</span>
        <span><span style={{ color: 'var(--color-amber)' }}>●</span> thing to do</span>
      </div>
    </div>
  )
}
