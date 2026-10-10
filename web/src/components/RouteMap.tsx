import type { LatLng, Plan } from '../lib/api'

type Kind = 'origin' | 'dest' | 'stop' | 'activity' | 'food' | 'hotel' | 'airport'
type Pin = { at: LatLng; label: string; kind: Kind; sub?: string; quiet?: boolean }

const COLOR: Record<Kind, string> = {
  origin: 'var(--color-cyan)', dest: 'var(--color-accent)', stop: 'var(--color-muted)',
  activity: 'var(--color-amber)', food: 'var(--color-amber)', hotel: 'var(--color-rose)', airport: 'var(--color-ink)',
}

/** Equirectangular projection of a pin set into a W×H box with padding. */
function projector(pins: Pin[], W: number, H: number, PAD: number, minSpan: number) {
  const lats = pins.map(p => p.at.lat), lngs = pins.map(p => p.at.lng)
  const minLat = Math.min(...lats), maxLat = Math.max(...lats)
  const minLng = Math.min(...lngs), maxLng = Math.max(...lngs)
  const midLat = (minLat + maxLat) / 2
  const kx = Math.cos((midLat * Math.PI) / 180)
  const spanLng = Math.max((maxLng - minLng) * kx, minSpan), spanLat = Math.max(maxLat - minLat, minSpan)
  const scale = Math.min((W - 2 * PAD) / spanLng, (H - 2 * PAD) / spanLat)
  const cx = ((minLng + maxLng) / 2) * kx, cy = midLat
  return {
    X: (ll: LatLng) => W / 2 + (ll.lng * kx - cx) * scale,
    Y: (ll: LatLng) => H / 2 - (ll.lat - cy) * scale,
  }
}

type Proj = ReturnType<typeof projector> & { arc: (a: LatLng, b: LatLng, bend?: number) => string }

function Sketch({ pins, W, H, minSpan, paths, legend, label }: {
  pins: Pin[]; W: number; H: number; minSpan: number
  paths: (p: Proj) => React.ReactNode
  legend: [Kind, string][]; label: string
}) {
  const pr = projector(pins, W, H, 48, minSpan)
  const arc = (a: LatLng, b: LatLng, bend = 0.12) => {
    const x1 = pr.X(a), y1 = pr.Y(a), x2 = pr.X(b), y2 = pr.Y(b)
    const mx = (x1 + x2) / 2, my = (y1 + y2) / 2
    const dx = x2 - x1, dy = y2 - y1
    return `M ${x1.toFixed(1)} ${y1.toFixed(1)} Q ${(mx - dy * bend).toFixed(1)} ${(my + dx * bend).toFixed(1)} ${x2.toFixed(1)} ${y2.toFixed(1)}`
  }
  return (
    <>
      <svg viewBox={`0 0 ${W} ${H}`} className="w-full h-auto font-mono" role="img" aria-label={label}>
        <rect width={W} height={H} fill="url(#tpn-grid)" />
        {paths({ ...pr, arc })}
        {pins.map((p, i) => {
          const x = pr.X(p.at), y = pr.Y(p.at)
          const big = p.kind === 'origin' || p.kind === 'dest'
          const anchor = x > W * 0.66 ? 'end' : 'start'
          const dx = anchor === 'end' ? -9 : 9
          const dy = p.kind === 'hotel' ? 13 : p.kind === 'activity' || p.kind === 'food' ? -5 : 4
          const text = p.label.length > 22 ? p.label.slice(0, 20) + '…' : p.label
          return (
            <g key={i}>
              <title>{p.sub ? `${p.label} · ${p.sub}` : p.label}</title>
              {big && <circle cx={x} cy={y} r="11" fill={COLOR[p.kind]} opacity="0.18" />}
              {p.kind === 'airport'
                ? <rect x={x - 4} y={y - 4} width="8" height="8" fill="var(--color-bg)" stroke={COLOR.airport} strokeWidth="1.5" transform={`rotate(45 ${x} ${y})`} />
                : p.kind === 'hotel'
                  ? <path d={`M ${x - 5} ${y + 4} v -6 l 5 -5 l 5 5 v 6 z`} fill={COLOR.hotel} stroke="var(--color-bg)" strokeWidth="1.2" />
                  : <circle cx={x} cy={y} r={big ? 5 : 3.5} fill={COLOR[p.kind]} stroke="var(--color-bg)" strokeWidth="1.5" />}
              {!p.quiet && (
                <text x={x + dx} y={y + dy} fontSize={big ? 12 : 10} fontWeight={big || p.kind === 'hotel' ? 600 : 400}
                  fill={big ? 'var(--color-ink)' : p.kind === 'hotel' ? COLOR.hotel : 'var(--color-muted)'} textAnchor={anchor}>{text}</text>
              )}
            </g>
          )
        })}
      </svg>
      <div className="mono text-[10px] text-dim mt-1 flex flex-wrap gap-x-3 gap-y-0.5">
        {legend.map(([k, t]) => <span key={k}><span style={{ color: COLOR[k] }}>{k === 'hotel' ? '⌂' : k === 'airport' ? '◆' : '●'}</span> {t}</span>)}
      </div>
    </>
  )
}

/**
 * Dependency-free trip map. Top: the route (drive = dashed curve city to city
 * with stops; fly = arc between the nearest airports plus the hops from each
 * city). Bottom: a zoomed-in sketch of the destination with the stay and the
 * things to do, since at route scale they would all sit on one pixel.
 */
export default function RouteMap({ plan, mode }: { plan: Plan; mode: string }) {
  const origin = plan.origin_loc, dest = plan.dest_loc
  if (!origin || !dest) return null
  const flying = !!(mode === 'fly' && plan.origin_airport && plan.dest_airport)
  const hotel = plan.hotels[0]
  const oName = plan.origin_place?.name || 'origin', dName = plan.dest_place?.name || 'destination'

  const routePins: Pin[] = [
    { at: origin, label: oName, kind: 'origin' },
    ...(mode === 'drive' ? plan.stops.filter(s => s.location?.lat).map(s => ({ at: s.location, label: s.name, kind: 'stop' as const })) : []),
    ...(flying ? [
      { at: plan.origin_airport!, label: plan.origin_airport!.code, kind: 'airport' as const, sub: plan.origin_airport!.name },
      { at: plan.dest_airport!, label: plan.dest_airport!.code, kind: 'airport' as const, sub: plan.dest_airport!.name },
    ] : []),
    { at: dest, label: dName, kind: 'dest' },
  ]
  const localPins: Pin[] = [
    { at: dest, label: dName, kind: 'dest' },
    ...(hotel?.location?.lat ? [{ at: hotel.location, label: hotel.name, kind: 'hotel' as const, sub: `your stay · ${hotel.area ?? ''}` }] : []),
    ...plan.activities.slice(0, 5).filter(a => a.location?.lat).map(a => ({ at: a.location, label: a.name, kind: 'activity' as const })),
    ...plan.food.slice(0, 3).filter(f => f.location?.lat).map(f => ({ at: f.location, label: f.name, kind: 'food' as const, quiet: true })),
  ]

  const miles = Math.round(plan.drive.distance_miles)
  const caption = flying
    ? `${plan.origin_airport!.code} → ${plan.dest_airport!.code} · ${miles} mi by road`
    : `${miles} mi each way · ${Math.floor(plan.drive.duration_min / 60)}h ${plan.drive.duration_min % 60}m`

  return (
    <div className="panel p-3 overflow-hidden grid gap-3">
      <svg width="0" height="0" className="absolute" aria-hidden>
        <defs>
          <pattern id="tpn-grid" width="24" height="24" patternUnits="userSpaceOnUse">
            <path d="M 24 0 L 0 0 0 24" fill="none" stroke="var(--color-line)" strokeWidth="0.6" />
          </pattern>
        </defs>
      </svg>
      <div>
        <div className="flex items-baseline justify-between">
          <div className="label">route</div>
          <div className="mono text-[11px] text-muted">{caption}</div>
        </div>
        <div className="mt-2">
          <Sketch pins={routePins} W={720} H={260} minSpan={0.5} label={`Route from ${oName} to ${dName}`}
            legend={flying ? [['origin', 'origin'], ['airport', 'airport'], ['dest', 'destination']] : [['origin', 'origin'], ...(mode === 'drive' && plan.stops.length > 0 ? [['stop', 'stop'] as [Kind, string]] : []), ['dest', 'destination']]}
            paths={(pr: Proj) => flying ? (
              <>
                <path d={pr.arc(origin, plan.origin_airport!, 0)} stroke="var(--color-muted)" strokeWidth="1.5" strokeDasharray="2 3" fill="none" />
                <path d={pr.arc(plan.origin_airport!, plan.dest_airport!, 0.18)} stroke="var(--color-cyan)" strokeWidth="2" strokeDasharray="8 5" fill="none" opacity="0.9" />
                <path d={pr.arc(plan.dest_airport!, dest, 0)} stroke="var(--color-muted)" strokeWidth="1.5" strokeDasharray="2 3" fill="none" />
              </>
            ) : (
              <path d={pr.arc(origin, dest)} stroke="var(--color-accent)" strokeWidth="2" strokeDasharray="6 5" fill="none" opacity="0.85" />
            )} />
        </div>
      </div>
      {localPins.length > 1 && (
        <div className="border-t border-line pt-3">
          <div className="flex items-baseline justify-between">
            <div className="label">around {dName}</div>
            {hotel?.distance_km > 0 && <div className="mono text-[11px] text-muted">stay {hotel.distance_km.toFixed(1)} km from center</div>}
          </div>
          <div className="mt-2">
            <Sketch pins={localPins} W={720} H={320} minSpan={0.045} label={`Around ${dName}: your stay and things to do`}
              legend={[['dest', 'center'], ['hotel', 'your stay'], ['activity', 'thing to do'], ['food', 'eat']]}
              paths={(pr: Proj) => hotel?.location?.lat ? (
                <path d={pr.arc(hotel.location, dest, 0)} stroke="var(--color-rose)" strokeWidth="1" strokeDasharray="2 3" fill="none" opacity="0.6" />
              ) : null} />
          </div>
        </div>
      )}
    </div>
  )
}
