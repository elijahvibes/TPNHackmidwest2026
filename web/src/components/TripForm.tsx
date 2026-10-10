import { useMemo, useState } from 'react'
import { api, type LatLng, type Placemark, type PlanRequest } from '../lib/api'
import PlaceInput from './PlaceInput'

export type TripFormValue = PlanRequest & { title?: string }

const INTERESTS = ['outdoors', 'museums', 'food', 'nightlife', 'family', 'history'] as const

const EXAMPLES: { from: string; to: string }[] = [
  { from: 'Kansas City', to: 'Denver' },
  { from: 'Lawrence, KS', to: 'Estes Park, CO' },
  { from: 'Omaha', to: 'Nashville' },
  { from: 'Chicago', to: 'Door County, WI' },
  { from: 'St. Louis', to: 'New Orleans' },
]

const today = new Date()
const iso = (d: Date) => d.toISOString().slice(0, 10)
const plus = (n: number) => { const d = new Date(today); d.setDate(d.getDate() + n); return d }
const nightsBetween = (a: string, b: string) => Math.round((Date.parse(b) - Date.parse(a)) / 86400000)

export default function TripForm({ onSubmit, busy, submitLabel = 'plan it', defaultOrigin }: {
  onSubmit: (v: TripFormValue) => void
  busy?: boolean
  submitLabel?: string
  defaultOrigin?: string | null
}) {
  const [v, setV] = useState<TripFormValue>({
    origin: defaultOrigin || 'Kansas City', destination: 'Denver',
    start_date: iso(plus(14)), end_date: iso(plus(17)),
    travelers: 2, budget_usd: 1200, mode: 'auto', interests: ['outdoors', 'food'],
    local_transport: 'auto', activity_radius_miles: 25,
  })
  const [originPin, setOriginPin] = useState<Placemark | null>(null)
  const [destPin, setDestPin] = useState<Placemark | null>(null)
  const [car, setCar] = useState({ year: 2019, make: 'Honda', model: 'Civic' })
  const [mpgNote, setMpgNote] = useState<string>('')
  const [geo, setGeo] = useState<LatLng | null>(null)
  const set = <K extends keyof TripFormValue>(k: K, val: TripFormValue[K]) => setV(s => ({ ...s, [k]: val }))

  const nights = useMemo(() => nightsBetween(v.start_date, v.end_date), [v.start_date, v.end_date])
  const dateErr = Number.isNaN(nights) ? 'pick both dates' : nights < 0 ? 'return is before departure' : nights > 30 ? 'trips over 30 nights are not priced' : ''

  const lookupMPG = async () => {
    setMpgNote('looking up…')
    try {
      const r = await api.vehicleMPG(car.year, car.make, car.model)
      set('vehicle_mpg', r.mpg)
      setMpgNote(`${r.mpg} mpg · ${r.source}`)
    } catch { setMpgNote('lookup failed, using 28 mpg') }
  }

  const useMyLocation = () => {
    if (!navigator.geolocation) { setMpgNote('geolocation unsupported'); return }
    navigator.geolocation.getCurrentPosition(
      p => {
        const ll = { lat: p.coords.latitude, lng: p.coords.longitude }
        setGeo(ll); setOriginPin(null)
        set('origin_loc', ll); set('origin', `My location (${ll.lat.toFixed(2)}, ${ll.lng.toFixed(2)})`)
      },
      () => setMpgNote('location denied'),
      { timeout: 8000 },
    )
  }

  const swap = () => {
    const o = v.origin, oPin = originPin
    set('origin', v.destination); set('destination', o)
    setOriginPin(destPin); setDestPin(oPin); setGeo(null)
    set('origin_loc', destPin ? { lat: destPin.lat, lng: destPin.lng } : undefined)
    set('dest_loc', oPin ? { lat: oPin.lat, lng: oPin.lng } : undefined)
  }

  const submit = (e: React.FormEvent) => {
    e.preventDefault()
    if (dateErr) return
    onSubmit({
      ...v,
      origin_loc: geo ?? (originPin ? { lat: originPin.lat, lng: originPin.lng } : undefined),
      dest_loc: destPin ? { lat: destPin.lat, lng: destPin.lng } : undefined,
    })
  }

  return (
    <form className="panel p-5 grid gap-5" onSubmit={submit}>
      <div className="grid sm:grid-cols-[1fr_auto_1fr] gap-3 sm:gap-2 items-end">
        <div>
          <div className="flex items-center justify-between">
            <label htmlFor="from" className="label">from</label>
            <button type="button" onClick={useMyLocation} className="mono text-[11px] text-cyan hover:underline">
              {geo ? '◉ using gps' : '◎ use my location'}
            </button>
          </div>
          <div className="mt-1">
            <PlaceInput id="from" value={v.origin} pinned={originPin} placeholder="any city, e.g. Lawrence, KS"
              onChange={t => { set('origin', t); set('origin_loc', undefined); setGeo(null) }}
              onPick={p => { setOriginPin(p); set('origin_loc', p ? { lat: p.lat, lng: p.lng } : undefined) }} />
          </div>
        </div>
        <button type="button" onClick={swap} title="swap origin and destination" aria-label="swap origin and destination"
          className="btn-ghost !px-3 justify-self-center sm:mb-0.5">⇄</button>
        <div>
          <label htmlFor="to" className="label">to</label>
          <div className="mt-1">
            <PlaceInput id="to" value={v.destination} pinned={destPin} placeholder="any city, town or park"
              onChange={t => { set('destination', t); set('dest_loc', undefined) }}
              onPick={p => { setDestPin(p); set('dest_loc', p ? { lat: p.lat, lng: p.lng } : undefined) }} />
          </div>
        </div>
      </div>

      <div className="flex flex-wrap gap-1.5 -mt-2">
        <span className="mono text-[11px] text-dim mr-1">try:</span>
        {EXAMPLES.map(ex => (
          <button type="button" key={ex.from + ex.to}
            onClick={() => { set('origin', ex.from); set('destination', ex.to); setOriginPin(null); setDestPin(null); setGeo(null); set('origin_loc', undefined); set('dest_loc', undefined) }}
            className="mono text-[11px] px-2 py-0.5 rounded border border-line text-muted hover:border-accent hover:text-accent transition-colors">
            {ex.from} → {ex.to}
          </button>
        ))}
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
        <div><label className="label">depart</label><input type="date" className="input mt-1" value={v.start_date} onChange={e => set('start_date', e.target.value)} required /></div>
        <div>
          <div className="flex items-center justify-between">
            <label className="label">return</label>
            <span className={`mono text-[11px] ${dateErr ? 'text-rose' : 'text-dim'}`}>{dateErr || (nights === 0 ? 'day trip' : `${nights} night${nights === 1 ? '' : 's'}`)}</span>
          </div>
          <input type="date" className={`input mt-1 ${dateErr ? '!border-rose' : ''}`} value={v.end_date} min={v.start_date} onChange={e => set('end_date', e.target.value)} required />
        </div>
        <div><label className="label">travelers</label><input type="number" min={1} max={12} className="input mt-1" value={v.travelers} onChange={e => set('travelers', +e.target.value)} /></div>
        <div><label className="label">budget $</label><input type="number" min={0} step={50} className="input mt-1" value={v.budget_usd ?? ''} placeholder="no limit" onChange={e => set('budget_usd', e.target.value ? +e.target.value : undefined)} /></div>
      </div>

      <div className="grid sm:grid-cols-[auto_1fr] gap-4 items-end">
        <div>
          <label className="label">mode</label>
          <div className="mt-1 flex rounded-md border border-line overflow-hidden mono text-sm">
            {(['auto', 'drive', 'fly'] as const).map(m => (
              <button type="button" key={m} onClick={() => set('mode', m)}
                className={`px-4 py-2 transition-colors ${v.mode === m ? 'bg-accent text-bg' : 'text-muted hover:text-ink'}`}>{m === 'auto' ? 'best value' : m}</button>
            ))}
          </div>
        </div>
        <div>
          <label className="label">your car (for gas math)</label>
          <div className="mt-1 flex gap-2">
            <input className="input !w-20" type="number" value={car.year} onChange={e => setCar({ ...car, year: +e.target.value })} aria-label="year" />
            <input className="input" value={car.make} onChange={e => setCar({ ...car, make: e.target.value })} placeholder="make" aria-label="make" />
            <input className="input" value={car.model} onChange={e => setCar({ ...car, model: e.target.value })} placeholder="model" aria-label="model" />
            <button type="button" className="btn-ghost whitespace-nowrap" onClick={lookupMPG}>mpg</button>
          </div>
          {mpgNote && <div className="mono text-[11px] text-muted mt-1">{mpgNote}</div>}
        </div>
      </div>

      <div className="grid sm:grid-cols-2 gap-4">
        <div>
          <label htmlFor="local-transport" className="label">getting around at destination</label>
          <select id="local-transport" className="input mt-1" value={v.local_transport}
            onChange={e => {
              const transport = e.target.value
              if (transport === 'auto' || transport === 'rental_car' || transport === 'transit_rideshare' || transport === 'walking') {
                set('local_transport', transport)
                if (transport === 'walking') set('activity_radius_miles', 2)
              }
            }}>
            <option value="auto">Own car if driving; transit/rideshare if flying</option>
            <option value="rental_car" disabled={v.mode === 'drive'}>Rental car if flying; own car if driving</option>
            <option value="transit_rideshare">Transit / rideshare</option>
            <option value="walking">Walking</option>
          </select>
        </div>
        <div>
          <label htmlFor="activity-radius" className="label">explore within (miles of destination)</label>
          <input id="activity-radius" className="input mt-1" type="number" min={1} max={150} required
            value={v.activity_radius_miles} onChange={e => set('activity_radius_miles', +e.target.value)} />
        </div>
        <p className="text-xs text-muted sm:col-span-2">
          City and out-of-city activities within your range. This is straight-line distance, not travel time.
          Getting-around costs are estimated per day and included in the total. {v.local_transport === 'walking' && (v.activity_radius_miles ?? 0) > 3
            && 'This range may require additional transportation beyond walking.'}
        </p>
      </div>

      <div>
        <label className="label">interests (shapes things to do)</label>
        <div className="mt-1 flex flex-wrap gap-2">
          {INTERESTS.map(i => {
            const on = v.interests?.includes(i)
            return (
              <button type="button" key={i} onClick={() => set('interests', on ? (v.interests ?? []).filter(x => x !== i) : [...(v.interests ?? []), i])}
                className={`mono text-xs px-3 py-1 rounded-full border transition-colors ${on ? 'bg-accent text-bg border-accent' : 'border-line text-muted hover:border-accent hover:text-accent'}`}>
                {on ? '✓ ' : ''}{i}
              </button>
            )
          })}
        </div>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3 pt-1">
        <span className="mono text-[11px] text-dim">any US city or park works · flights priced from the nearest airport · live flight, hotel, gas, route and crime data; place prices marked ~ are estimates</span>
        <button className="btn-primary" disabled={busy || !!dateErr}>{busy ? 'planning…' : submitLabel} →</button>
      </div>
    </form>
  )
}
