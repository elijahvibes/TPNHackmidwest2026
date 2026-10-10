import { useState } from 'react'
import { api, type LatLng, type PlanRequest } from '../lib/api'

export type TripFormValue = PlanRequest & { title?: string }

const INTERESTS = ['outdoors', 'museums', 'food', 'nightlife', 'family', 'history'] as const

const today = new Date()
const iso = (d: Date) => d.toISOString().slice(0, 10)
const plus = (n: number) => { const d = new Date(today); d.setDate(d.getDate() + n); return d }

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
  })
  const [car, setCar] = useState({ year: 2019, make: 'Honda', model: 'Civic' })
  const [mpgNote, setMpgNote] = useState<string>('')
  const [geo, setGeo] = useState<LatLng | null>(null)
  const set = <K extends keyof TripFormValue>(k: K, val: TripFormValue[K]) => setV(s => ({ ...s, [k]: val }))

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
      p => { const ll = { lat: p.coords.latitude, lng: p.coords.longitude }; setGeo(ll); set('origin_loc', ll); set('origin', `My location (${ll.lat.toFixed(2)}, ${ll.lng.toFixed(2)})`) },
      () => setMpgNote('location denied'),
      { timeout: 8000 },
    )
  }

  return (
    <form className="panel p-5 grid gap-4" onSubmit={e => { e.preventDefault(); onSubmit(v) }}>
      <div className="grid sm:grid-cols-2 gap-4">
        <div>
          <div className="flex items-center justify-between">
            <label className="label">from</label>
            <button type="button" onClick={useMyLocation} className="mono text-[11px] text-cyan hover:underline">
              {geo ? '◉ using gps' : '◎ use my location'}
            </button>
          </div>
          <input className="input mt-1" value={v.origin} onChange={e => { set('origin', e.target.value); set('origin_loc', undefined); setGeo(null) }} placeholder="Kansas City" required />
        </div>
        <div>
          <label className="label">to</label>
          <input className="input mt-1" value={v.destination} onChange={e => set('destination', e.target.value)} placeholder="Denver" required />
        </div>
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
        <div><label className="label">depart</label><input type="date" className="input mt-1" value={v.start_date} onChange={e => set('start_date', e.target.value)} required /></div>
        <div><label className="label">return</label><input type="date" className="input mt-1" value={v.end_date} onChange={e => set('end_date', e.target.value)} required /></div>
        <div><label className="label">travelers</label><input type="number" min={1} max={12} className="input mt-1" value={v.travelers} onChange={e => set('travelers', +e.target.value)} /></div>
        <div><label className="label">budget $</label><input type="number" min={0} step={50} className="input mt-1" value={v.budget_usd ?? ''} onChange={e => set('budget_usd', e.target.value ? +e.target.value : undefined)} /></div>
      </div>

      <div className="grid sm:grid-cols-[auto_1fr] gap-4 items-end">
        <div>
          <label className="label">mode</label>
          <div className="mt-1 flex rounded-md border border-line overflow-hidden mono text-sm">
            {(['auto', 'drive', 'fly'] as const).map(m => (
              <button type="button" key={m} onClick={() => set('mode', m)}
                className={`px-4 py-2 ${v.mode === m ? 'bg-accent text-bg' : 'text-muted hover:text-ink'}`}>{m === 'auto' ? 'best value' : m}</button>
            ))}
          </div>
        </div>
        <div>
          <label className="label">your car (for gas math)</label>
          <div className="mt-1 flex gap-2">
            <input className="input !w-20" type="number" value={car.year} onChange={e => setCar({ ...car, year: +e.target.value })} />
            <input className="input" value={car.make} onChange={e => setCar({ ...car, make: e.target.value })} placeholder="make" />
            <input className="input" value={car.model} onChange={e => setCar({ ...car, model: e.target.value })} placeholder="model" />
            <button type="button" className="btn-ghost whitespace-nowrap" onClick={lookupMPG}>mpg</button>
          </div>
          {mpgNote && <div className="mono text-[11px] text-muted mt-1">{mpgNote}</div>}
        </div>
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

      <div className="flex items-center justify-between pt-1">
        <span className="mono text-[11px] text-dim">prices pulled live where keys are configured; mock otherwise</span>
        <button className="btn-primary" disabled={busy}>{busy ? 'planning…' : submitLabel} →</button>
      </div>
    </form>
  )
}
