import { useEffect, useRef } from 'react'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import type { LatLng, Plan } from '../lib/api'

type Kind = 'origin' | 'dest' | 'stop' | 'activity' | 'food' | 'hotel' | 'airport'
type Pin = { at: LatLng; label: string; kind: Kind; sub?: string; quiet?: boolean }
type Path = { pts: LatLng[]; color: string; width?: number; dash?: string; opacity?: number }

// Leaflet writes these straight into SVG attributes, so they are the theme hexes, not var()s.
const COLOR: Record<Kind, string> = {
  origin: '#3e5f7e', dest: '#56674a', stop: '#656a63',
  activity: '#a3661f', food: '#a3661f', hotel: '#b23a3a', airport: '#1f2420',
}
const PANEL = '#ffffff'

// OpenStreetMap's standard tiles: free with attribution and no key. The CSS desaturates
// them a little so they sit with the paper palette.
const TILES = 'https://tile.openstreetmap.org/{z}/{x}/{y}.png'
const ATTRIBUTION = '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'

/** Points along a quadratic curve from a to b, bowed sideways by `bend` of its length (0 = straight). */
function arc(a: LatLng, b: LatLng, bend = 0, n = 32): LatLng[] {
  if (!bend) return [a, b]
  const kx = Math.cos((((a.lat + b.lat) / 2) * Math.PI) / 180)
  const x1 = a.lng * kx, y1 = a.lat, x2 = b.lng * kx, y2 = b.lat
  const dx = x2 - x1, dy = y2 - y1
  const cx = (x1 + x2) / 2 - dy * bend, cy = (y1 + y2) / 2 + dx * bend
  return Array.from({ length: n + 1 }, (_, i) => {
    const t = i / n, u = 1 - t
    return { lng: (u * u * x1 + 2 * u * t * cx + t * t * x2) / kx, lat: u * u * y1 + 2 * u * t * cy + t * t * y2 }
  })
}

const glyph = (kind: Kind) => kind === 'hotel'
  ? `<svg viewBox="0 0 16 16" width="16" height="16"><path d="M3 13V7.5L8 3l5 4.5V13z" fill="${COLOR.hotel}" stroke="${PANEL}" stroke-width="1.3"/></svg>`
  : `<svg viewBox="0 0 16 16" width="16" height="16"><rect x="4" y="4" width="8" height="8" transform="rotate(45 8 8)" fill="${PANEL}" stroke="${COLOR.airport}" stroke-width="1.5"/></svg>`

function TileMap({ pins, paths, height, maxZoom, label, legend }: {
  pins: Pin[]; paths: Path[]; height: number; maxZoom: number; label: string; legend: [Kind, string][]
}) {
  const ref = useRef<HTMLDivElement>(null)
  const key = JSON.stringify([pins, paths])

  useEffect(() => {
    const el = ref.current
    if (!el) return
    const map = L.map(el, { zoomControl: false, scrollWheelZoom: false, attributionControl: true })
    map.attributionControl.setPrefix(false)
    L.tileLayer(TILES, { attribution: ATTRIBUTION, maxZoom: 19 }).addTo(map)

    for (const p of paths) {
      L.polyline(p.pts.map(ll => [ll.lat, ll.lng]), { color: p.color, weight: p.width ?? 2, dashArray: p.dash, opacity: p.opacity ?? 0.9, interactive: false }).addTo(map)
    }
    for (const p of pins) {
      const big = p.kind === 'origin' || p.kind === 'dest'
      const at: [number, number] = [p.at.lat, p.at.lng]
      if (big) L.circleMarker(at, { radius: 11, stroke: false, fillColor: COLOR[p.kind], fillOpacity: 0.18, interactive: false }).addTo(map)
      const marker = p.kind === 'hotel' || p.kind === 'airport'
        ? L.marker(at, { icon: L.divIcon({ className: 'tpn-pin', html: glyph(p.kind), iconSize: [16, 16], iconAnchor: [8, 8] }), keyboard: false })
        : L.circleMarker(at, { radius: big ? 5 : 3.5, color: PANEL, weight: 1.5, fillColor: COLOR[p.kind], fillOpacity: 1 })
      const text = p.label.length > 22 ? p.label.slice(0, 20) + '…' : p.label
      marker.bindTooltip(p.quiet ? (p.sub ? `${p.label} · ${p.sub}` : p.label) : text, {
        permanent: !p.quiet, direction: 'auto', offset: [8, 0], opacity: 1,
        className: `tpn-tip ${big ? 'tpn-tip-big' : p.kind === 'hotel' ? 'tpn-tip-hotel' : ''}`,
      })
      if (!p.quiet && p.sub) marker.bindPopup(`${p.label}<br><span class="text-muted">${p.sub}</span>`)
      marker.addTo(map)
    }

    const bounds = L.latLngBounds(pins.map(p => [p.at.lat, p.at.lng]))
    const fit = () => pins.length > 1
      ? map.fitBounds(bounds, { padding: [24, 24], maxZoom })
      : map.setView(bounds.getCenter(), Math.min(maxZoom, 12))
    fit()
    // The aside is laid out after mount (sticky column, breakpoints), and a fit done at
    // zero width lands on the wrong zoom: refit when the box first gets a real size.
    let fitted = el.clientWidth > 0
    const ro = new ResizeObserver(() => {
      map.invalidateSize()
      if (!fitted && el.clientWidth > 0) { fitted = true; fit() }
    })
    ro.observe(el)
    return () => { ro.disconnect(); map.remove() }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, height, maxZoom])

  return (
    <>
      <div ref={ref} role="img" aria-label={label} className="isolate relative rounded-lg overflow-hidden border border-line bg-panel-2" style={{ height }} />
      <div className="text-[11px] text-dim mt-1.5 flex flex-wrap gap-x-3 gap-y-0.5">
        {legend.map(([k, t]) => <span key={k}><span style={{ color: COLOR[k] }}>{k === 'hotel' ? '⌂' : k === 'airport' ? '◆' : '●'}</span> {t}</span>)}
      </div>
    </>
  )
}

/**
 * Trip map on OpenStreetMap tiles. Top: the route (drive = dashed curve city to
 * city with stops; fly = arc between the nearest airports plus the hops from
 * each city). Bottom: the destination zoomed in with the stay and the things to
 * do, since at route scale they would all sit on one pixel.
 */
export default function RouteMap({ plan, mode }: { plan: Plan; mode: string }) {
  const origin = plan.origin_loc, dest = plan.dest_loc
  if (!origin || !dest) return null
  const flying = !!(mode === 'fly' && plan.origin_airport && plan.dest_airport)
  const hotel = plan.hotels[0]
  const oName = plan.origin_place?.name || 'origin', dName = plan.dest_place?.name || 'destination'

  const routePins: Pin[] = [
    { at: origin, label: oName, kind: 'origin' },
    ...(mode === 'drive' ? plan.stops.filter(s => s.location?.lat).map(s => ({ at: s.location, label: s.name, kind: 'stop' as const, quiet: true })) : []),
    ...(flying ? [
      { at: plan.origin_airport!, label: plan.origin_airport!.code, kind: 'airport' as const, sub: plan.origin_airport!.name },
      { at: plan.dest_airport!, label: plan.dest_airport!.code, kind: 'airport' as const, sub: plan.dest_airport!.name },
    ] : []),
    { at: dest, label: dName, kind: 'dest' },
  ]
  const routePaths: Path[] = flying ? [
    { pts: arc(origin, plan.origin_airport!), color: COLOR.stop, width: 1.5, dash: '2 4' },
    { pts: arc(plan.origin_airport!, plan.dest_airport!, 0.18), color: COLOR.origin, dash: '8 6' },
    { pts: arc(plan.dest_airport!, dest), color: COLOR.stop, width: 1.5, dash: '2 4' },
  ] : [
    { pts: arc(origin, dest, 0.12), color: COLOR.dest, dash: '6 6', opacity: 0.85 },
  ]

  const localPins: Pin[] = [
    { at: dest, label: dName, kind: 'dest' },
    ...(hotel?.location?.lat ? [{ at: hotel.location, label: hotel.name, kind: 'hotel' as const, sub: `your stay · ${hotel.area ?? ''}` }] : []),
    ...plan.activities.slice(0, 5).filter(a => a.location?.lat).map(a => ({ at: a.location, label: a.name, kind: 'activity' as const })),
    ...plan.food.slice(0, 3).filter(f => f.location?.lat).map(f => ({ at: f.location, label: f.name, kind: 'food' as const, quiet: true })),
  ]
  const localPaths: Path[] = hotel?.location?.lat ? [{ pts: [hotel.location, dest], color: COLOR.hotel, width: 1, dash: '2 4', opacity: 0.6 }] : []

  const miles = Math.round(plan.drive.distance_miles)
  const caption = flying
    ? `${plan.origin_airport!.code} → ${plan.dest_airport!.code} · ${miles} mi by road`
    : `${miles} mi each way · ${Math.floor(plan.drive.duration_min / 60)}h ${plan.drive.duration_min % 60}m`

  return (
    <div className="panel p-5 overflow-hidden grid gap-4">
      <div>
        <div className="flex items-baseline justify-between">
          <h3 className="text-[19px]">Route</h3>
          <div className="text-[12px] text-muted shrink-0">{caption}</div>
        </div>
        <div className="mt-2">
          <TileMap pins={routePins} paths={routePaths} height={240} maxZoom={9} label={`Route from ${oName} to ${dName}`}
            legend={flying ? [['origin', 'origin'], ['airport', 'airport'], ['dest', 'destination']] : [['origin', 'origin'], ...(mode === 'drive' && plan.stops.length > 0 ? [['stop', 'stop'] as [Kind, string]] : []), ['dest', 'destination']]} />
        </div>
      </div>
      {localPins.length > 1 && (
        <div className="border-t border-line pt-4">
          <div className="flex items-baseline justify-between">
            <h3 className="text-[19px] truncate">Around {dName}</h3>
            {hotel?.distance_km > 0 && <div className="text-[12px] text-muted shrink-0">stay {hotel.distance_km.toFixed(1)} km from center</div>}
          </div>
          <div className="mt-2">
            <TileMap pins={localPins} paths={localPaths} height={300} maxZoom={14} label={`Around ${dName}: your stay and things to do`}
              legend={[['dest', 'center'], ['hotel', 'your stay'], ['activity', 'thing to do'], ['food', 'eat']]} />
          </div>
        </div>
      )}
    </div>
  )
}
