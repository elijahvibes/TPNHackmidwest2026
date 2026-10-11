import { useMemo, useState, type ReactNode } from 'react'
import { api, type LatLng, type Placemark, type PlanRequest } from '../lib/api'
import PlaceInput from './PlaceInput'
import Icon, { type IconName } from './Icon'
import TripGlance from './TripGlance'

export type TripFormValue = PlanRequest & { title?: string }

const INTERESTS: { key: string; label: string }[] = [
  { key: 'outdoors', label: 'Outdoors' }, { key: 'food', label: 'Food' }, { key: 'museums', label: 'Museums' },
  { key: 'history', label: 'History' }, { key: 'nightlife', label: 'Nightlife' }, { key: 'family', label: 'Family' },
]
const MODES: { key: 'auto' | 'drive' | 'fly'; label: string }[] = [
  { key: 'auto', label: 'Best value' }, { key: 'drive', label: 'Drive' }, { key: 'fly', label: 'Fly' },
]
const TRANSPORT: { key: NonNullable<PlanRequest['local_transport']>; label: string }[] = [
  { key: 'auto', label: 'Own car or rideshare' }, { key: 'rental_car', label: 'Rental car' },
  { key: 'transit_rideshare', label: 'Transit & rideshare' }, { key: 'walking', label: 'Walking' },
]
const RADII = [2, 5, 10, 25, 60, 100, 150]

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

export default function TripForm({ onSubmit, busy, submitLabel = 'Plan my trip', defaultOrigin }: {
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
  const [more, setMore] = useState(false)
  const set = <K extends keyof TripFormValue>(k: K, val: TripFormValue[K]) => setV(s => ({ ...s, [k]: val }))

  const nights = useMemo(() => nightsBetween(v.start_date, v.end_date), [v.start_date, v.end_date])
  const dateErr = Number.isNaN(nights) ? 'Pick both dates' : nights < 0 ? 'Return is before departure' : nights > 30 ? 'Trips over 30 nights are not priced' : ''
  const nightsLabel = nights === 0 ? 'Day trip' : `${nights} night${nights === 1 ? '' : 's'}`

  const lookupMPG = async () => {
    setMpgNote('Looking up…')
    try {
      const r = await api.vehicleMPG(car.year, car.make, car.model)
      set('vehicle_mpg', r.mpg)
      setMpgNote(`${r.mpg} mpg · ${r.source}`)
    } catch { setMpgNote('Lookup failed, using 28 mpg') }
  }

  const useMyLocation = () => {
    if (!navigator.geolocation) { setMpgNote('Geolocation is not supported here'); return }
    navigator.geolocation.getCurrentPosition(
      p => {
        const ll = { lat: p.coords.latitude, lng: p.coords.longitude }
        setGeo(ll); setOriginPin(null)
        set('origin_loc', ll); set('origin', `My location (${ll.lat.toFixed(2)}, ${ll.lng.toFixed(2)})`)
      },
      () => setMpgNote('Location permission denied'),
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

  const toggleInterest = (i: string) => {
    const on = v.interests?.includes(i)
    set('interests', on ? (v.interests ?? []).filter(x => x !== i) : [...(v.interests ?? []), i])
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

  const styleLine = [MODES.find(m => m.key === v.mode)?.label ?? 'Best value', ...INTERESTS.filter(i => v.interests?.includes(i.key)).map(i => i.label)]

  return (
    <form className="grid lg:grid-cols-[minmax(0,1fr)_380px] gap-6 items-start" onSubmit={submit}>
      <div className="panel px-6 sm:px-8 py-7 grid gap-8">
        {/* 01 */}
        <Section n="01" title="Where & when">
          <div className="grid sm:grid-cols-2 gap-4">
            <Field label="From" htmlFor="from" hint={
              <button type="button" onClick={useMyLocation} className="text-[12px] text-cyan hover:underline flex items-center gap-1">
                <Icon name="locate" size={13} />{geo ? 'Using my location' : 'Use my location'}
              </button>}>
              <IconField icon="pin">
                <PlaceInput id="from" value={v.origin} pinned={originPin} placeholder="Any city, e.g. Lawrence, KS"
                  onChange={t => { set('origin', t); set('origin_loc', undefined); setGeo(null) }}
                  onPick={p => { setOriginPin(p); set('origin_loc', p ? { lat: p.lat, lng: p.lng } : undefined) }} />
              </IconField>
            </Field>
            <Field label="To" htmlFor="to" hint={
              <button type="button" onClick={swap} className="text-[12px] text-cyan hover:underline flex items-center gap-1" aria-label="Swap origin and destination">
                <Icon name="swap" size={13} />Swap
              </button>}>
              <IconField icon="pin">
                <PlaceInput id="to" value={v.destination} pinned={destPin} placeholder="Any city, town or park"
                  onChange={t => { set('destination', t); set('dest_loc', undefined) }}
                  onPick={p => { setDestPin(p); set('dest_loc', p ? { lat: p.lat, lng: p.lng } : undefined) }} />
              </IconField>
            </Field>
            <Field label="Departure" htmlFor="depart">
              <IconField icon="calendar">
                <input id="depart" type="date" className="input pl-10" value={v.start_date} onChange={e => set('start_date', e.target.value)} required />
              </IconField>
            </Field>
            <Field label="Return" htmlFor="return" hint={<span className={`text-[12px] ${dateErr ? 'text-rose' : 'text-muted'}`}>{dateErr || nightsLabel}</span>}>
              <IconField icon="calendar">
                <input id="return" type="date" className={`input pl-10 ${dateErr ? '!border-rose' : ''}`} value={v.end_date} min={v.start_date} onChange={e => set('end_date', e.target.value)} required />
              </IconField>
            </Field>
          </div>
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[12px] text-dim">
            <span>Try</span>
            {EXAMPLES.map(ex => (
              <button type="button" key={ex.from + ex.to} className="text-muted hover:text-ink underline-offset-2 hover:underline"
                onClick={() => { set('origin', ex.from); set('destination', ex.to); setOriginPin(null); setDestPin(null); setGeo(null); set('origin_loc', undefined); set('dest_loc', undefined) }}>
                {ex.from} → {ex.to}
              </button>
            ))}
          </div>
        </Section>

        {/* 02 */}
        <Section n="02" title="Your people & budget">
          <div className="grid sm:grid-cols-2 gap-4">
            <Field label="Travelers" htmlFor="travelers">
              <IconField icon="users">
                <select id="travelers" className="input pl-10" value={v.travelers} onChange={e => set('travelers', +e.target.value)}>
                  {Array.from({ length: 12 }, (_, i) => i + 1).map(n => <option key={n} value={n}>{n} {n === 1 ? 'adult' : 'adults'}</option>)}
                </select>
              </IconField>
            </Field>
            <Field label="Total trip budget" htmlFor="budget" hint={<span className="text-[12px] text-muted">USD</span>}>
              <IconField icon="wallet">
                <input id="budget" type="number" min={0} step={50} className="input pl-10" value={v.budget_usd ?? ''} placeholder="No limit"
                  onChange={e => set('budget_usd', e.target.value ? +e.target.value : undefined)} />
              </IconField>
            </Field>
          </div>
          <p className="text-[13px] text-muted">One budget for travel, stays and things to do. Not per person.</p>
        </Section>

        {/* 03 */}
        <Section n="03" title="Your kind of getaway" last>
          <div>
            <div className="label">How would you like to travel?</div>
            <div className="mt-2 flex flex-wrap gap-2">
              {MODES.map(m => <Chip key={m.key} on={v.mode === m.key} onClick={() => set('mode', m.key)}>{m.label}</Chip>)}
            </div>
            <p className="text-[13px] text-muted mt-2">
              {v.mode === 'auto' ? 'Compare driving and flying to find the better-value trip.'
                : v.mode === 'drive' ? 'Gas is priced for your car, a tank at a time, with refuel stops on the way.'
                  : 'Flights are priced from the nearest airport to each place.'}
            </p>
          </div>
          <div>
            <div className="label">Make time for</div>
            <div className="mt-2 flex flex-wrap gap-2">
              {INTERESTS.map(i => <Chip key={i.key} on={!!v.interests?.includes(i.key)} onClick={() => toggleInterest(i.key)}>{i.label}</Chip>)}
            </div>
          </div>

          <div className="border-t border-line pt-4">
            <button type="button" onClick={() => setMore(m => !m)} aria-expanded={more}
              className="w-full flex items-center justify-between text-[13px] text-muted hover:text-ink">
              <span>Car details &amp; more preferences</span>
              <Icon name={more ? 'minus' : 'plus'} size={14} />
            </button>
            {more && (
              <div className="grid gap-5 mt-4">
                <div>
                  <div className="label">Car details</div>
                  <div className="grid grid-cols-2 gap-4 mt-2">
                    <Field label="Year" htmlFor="car-year" small>
                      <input id="car-year" className="input" type="number" value={car.year} onChange={e => setCar({ ...car, year: +e.target.value })} />
                    </Field>
                    <Field label="Make" htmlFor="car-make" small>
                      <input id="car-make" className="input" value={car.make} onChange={e => setCar({ ...car, make: e.target.value })} placeholder="Honda" />
                    </Field>
                    <Field label="Model" htmlFor="car-model" small>
                      <input id="car-model" className="input" value={car.model} onChange={e => setCar({ ...car, model: e.target.value })} placeholder="Civic" />
                    </Field>
                    <Field label="MPG" htmlFor="car-mpg" small hint={<button type="button" className="text-[12px] text-cyan hover:underline" onClick={lookupMPG}>Look up</button>}>
                      <input id="car-mpg" className="input" type="number" min={5} max={120} step={0.5} value={v.vehicle_mpg ?? ''} placeholder="28"
                        onChange={e => set('vehicle_mpg', e.target.value ? +e.target.value : undefined)} />
                    </Field>
                    <Field label="Fuel tank (gallons)" htmlFor="car-tank" small>
                      <input id="car-tank" className="input" type="number" min={5} max={60} step={0.5} value={v.tank_gallons ?? ''} placeholder="14"
                        onChange={e => set('tank_gallons', e.target.value ? +e.target.value : undefined)} />
                    </Field>
                  </div>
                  <p className="text-[12px] text-muted mt-2">
                    {mpgNote ? <span className="text-ink">{mpgNote} · </span> : null}
                    You leave on a full tank and the plan adds a refuel stop each time it runs low.
                  </p>
                </div>

                <div>
                  <div className="label">More preferences</div>
                  <div className="mt-2 text-[13px] text-muted">Getting around at the destination</div>
                  <div className="mt-2 flex flex-wrap gap-2">
                    {TRANSPORT.map(t => (
                      <Chip key={t.key} on={v.local_transport === t.key} disabled={t.key === 'rental_car' && v.mode === 'drive'}
                        onClick={() => { set('local_transport', t.key); if (t.key === 'walking') set('activity_radius_miles', 2) }}>{t.label}</Chip>
                    ))}
                  </div>
                  <Field label="Exploration radius" htmlFor="radius" className="mt-4">
                    <select id="radius" className="input" value={v.activity_radius_miles} onChange={e => set('activity_radius_miles', +e.target.value)}>
                      {RADII.map(r => <option key={r} value={r}>{r} miles</option>)}
                    </select>
                  </Field>
                  <p className="text-[12px] text-muted mt-2">
                    Straight-line distance from the destination, not travel time. Getting-around costs are estimated per day and included in the total.
                    {v.local_transport === 'walking' && (v.activity_radius_miles ?? 0) > 3 && ' This range may need more than walking.'}
                  </p>
                </div>
              </div>
            )}
          </div>
        </Section>
      </div>

      <TripGlance
        origin={originPin?.name ?? v.origin} destination={destPin?.name ?? v.destination}
        start={v.start_date} end={v.end_date} nights={nights} travelers={v.travelers} budget={v.budget_usd}
        style={styleLine} dateErr={dateErr}
        action={
          <button className="btn-primary w-full !py-3 text-[15px]" disabled={busy || !!dateErr}>
            {busy ? <>Planning<span className="caret" /></> : <>{submitLabel}<Icon name="arrow" size={16} /></>}
          </button>
        } />
    </form>
  )
}

function Section({ n, title, children, last }: { n: string; title: string; children: ReactNode; last?: boolean }) {
  return (
    <section className={`grid gap-4 ${last ? '' : 'pb-8 border-b border-line'}`}>
      <h2 className="text-[22px] text-ink flex items-baseline">
        <span className="font-sans text-[12px] text-dim tabular-nums mr-3">{n}</span>{title}
      </h2>
      {children}
    </section>
  )
}

function Field({ label, htmlFor, hint, children, small, className = '' }: {
  label: string; htmlFor?: string; hint?: ReactNode; children: ReactNode; small?: boolean; className?: string
}) {
  return (
    <div className={className}>
      <div className="flex items-end justify-between mb-1.5">
        <label htmlFor={htmlFor} className={small ? 'text-[13px] text-ink' : 'label'}>{label}</label>
        {hint}
      </div>
      {children}
    </div>
  )
}

function IconField({ icon, children }: { icon: IconName; children: ReactNode }) {
  return (
    <div className="relative [&>div>input]:pl-10">
      <Icon name={icon} size={16} className="absolute left-3.5 top-1/2 -translate-y-1/2 text-muted pointer-events-none z-[1]" />
      {children}
    </div>
  )
}

function Chip({ on, onClick, children, disabled }: { on: boolean; onClick: () => void; children: ReactNode; disabled?: boolean }) {
  return (
    <button type="button" onClick={onClick} disabled={disabled} aria-pressed={on}
      className={`chip ${on ? 'chip-on' : ''} disabled:opacity-40 disabled:cursor-not-allowed`}>
      {on && <Icon name="check" size={14} />}{children}
    </button>
  )
}

