import { useState } from 'react'
import { api, hrs, usd, type BookingOption, type BookingSeller, type FlightQuote, type FlightSearch, type Plan } from '../lib/api'

type Props = {
  plan: Plan
  onSelect: (index: number, returnToken?: string) => Promise<void>
  disabled?: boolean
}

function FlightDetails({ flight }: { flight: FlightQuote }) {
  return (
    <div>
      <div className="mono font-bold">{flight.carrier}</div>
      <div className="text-sm text-muted mt-1">
        {flight.origin} → {flight.destination} · {hrs(flight.duration_min)} · {flight.stops === 0 ? 'nonstop' : `${flight.stops} stop${flight.stops === 1 ? '' : 's'}`}
      </div>
      {flight.departure_time && <div className="mono text-xs mt-2">{flight.departure_time} → {flight.arrival_time} <span className="text-dim">(airport local times)</span></div>}
      {!!flight.flight_numbers?.length && <div className="text-xs text-dim mt-1">{flight.flight_numbers.join(' · ')}</div>}
    </div>
  )
}

function BookingLink({ seller }: { seller: BookingSeller }) {
  const action = seller.booking_request
  let safeURL = ''
  let postAllowed = false
  try {
    const url = new URL(action?.url ?? '')
    if (url.protocol === 'https:' && !url.username && !url.password) {
      safeURL = url.href
      postAllowed = url.host === 'www.google.com' && url.pathname === '/travel/clk/f'
    }
  } catch {
    // Some provider options are phone-only and have no booking URL.
  }
  return (
    <div className="panel p-4 grid gap-2">
      <div className="flex flex-wrap justify-between gap-2">
        <span className="font-bold">{seller.book_with} {seller.option_title && <span className="text-muted font-normal">· {seller.option_title}</span>}</span>
        {seller.price > 0 && <span className="mono text-accent">{usd(seller.price)} group total</span>}
      </div>
      <ul className="text-xs text-muted grid gap-1">
        {[...(seller.extensions ?? []), ...(seller.baggage_prices ?? [])].map((text, i) => <li key={i}>{text}</li>)}
      </ul>
      {action?.post_data && postAllowed ? (
        <form method="post" action="/api/flights/booking-redirect" target="_blank" rel="noopener noreferrer">
          <input type="hidden" name="url" value={safeURL} />
          <input type="hidden" name="post_data" value={action.post_data} />
          <button className="btn-primary" type="submit">continue to {seller.book_with} ↗</button>
        </form>
      ) : safeURL && !action?.post_data ? (
        <a className="btn-primary justify-self-start" href={safeURL} target="_blank" rel="noopener noreferrer">continue to {seller.book_with} ↗</a>
      ) : <p className="text-muted text-sm">{seller.booking_phone ? `Book by phone: ${seller.booking_phone}` : 'No supported booking link returned for this option.'}</p>}
    </div>
  )
}

export default function FlightChooser({ plan, onSelect, disabled = false }: Props) {
  const [sort, setSort] = useState('recommended')
  const [nonstop, setNonstop] = useState(false)
  const [showAll, setShowAll] = useState(false)
  const [showAllReturns, setShowAllReturns] = useState(false)
  const [returns, setReturns] = useState<FlightQuote[]>([])
  const [bookings, setBookings] = useState<BookingOption[]>([])
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const selectedIndex = plan.selected_flight ?? 0
  const selected = plan.flights[selectedIndex]
  const locked = disabled || !!busy
  const search: FlightSearch | null = selected && plan.travelers ? {
    origin: selected.origin, destination: selected.destination, depart: selected.depart,
    return: selected.return ?? '', travelers: plan.travelers, departure_token: selected.departure_token,
  } : null
  const choices = plan.flights.map((flight, index) => ({ flight, index }))
    .filter(({ flight }) => !nonstop || flight.stops === 0)
    .sort((a, b) => sort === 'price' ? a.flight.price_usd - b.flight.price_usd
      : sort === 'duration' ? a.flight.duration_min - b.flight.duration_min || a.flight.price_usd - b.flight.price_usd
        : a.index - b.index)

  const select = async (index: number, returnToken?: string) => {
    setBusy('updating your flight'); setError(''); setBookings([])
    try {
      await onSelect(index, returnToken)
      if (index !== selectedIndex) { setReturns([]); setShowAllReturns(false) }
    } catch (e) { setError((e as Error).message) } finally { setBusy('') }
  }

  const loadReturns = async () => {
    if (!search) return
    setBusy('finding return flights'); setError('')
    try { setReturns(await api.returnFlights(search)); setBookings([]) }
    catch (e) { setError((e as Error).message) } finally { setBusy('') }
  }

  const loadBookings = async () => {
    if (!search || !plan.selected_return?.booking_token) return
    setBusy('finding booking options'); setError(''); setBookings([])
    try { setBookings(await api.flightBookings({ ...search, booking_token: plan.selected_return.booking_token })) }
    catch (e) { setError((e as Error).message) } finally { setBusy('') }
  }

  return (
    <section className="panel p-5 grid gap-4" aria-label="Choose your flights" aria-busy={!!busy}>
      <div>
        <h3 className="mono text-lg font-bold">Choose your flight</h3>
        <p className="text-sm text-muted mt-1">Best value first: fare + $25 per outbound travel hour + $50 per connection. Choose a departure, then a return and booking provider.</p>
        <p className="text-xs text-dim mt-1">Prices are round trip per traveler. Choosing a return may change the fare. TPN prices all travelers as adults.</p>
      </div>
      {!plan.travelers && <p className="text-amber">Re-plan this older trip to enable flight selection.</p>}
      <div className="flex flex-wrap items-center gap-4">
        <label className="text-sm">Sort by
          <select className="input mt-1" value={sort} onChange={e => setSort(e.target.value)}>
            <option value="recommended">Recommended</option><option value="price">Lowest price</option><option value="duration">Shortest flight</option>
          </select>
        </label>
        <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={nonstop} onChange={e => setNonstop(e.target.checked)} /> Nonstop only</label>
        <span className="text-xs text-muted">{choices.length} departure options</span>
      </div>
      <div className="grid gap-3" role="group" aria-label="Departure flight options">
        {(showAll ? choices : choices.slice(0, 5)).map(({ flight: f, index }) => (
          <div key={index} className={`panel p-4 ${index === selectedIndex ? 'border-accent' : ''}`}>
            <div className="flex flex-wrap justify-between gap-3">
              <div>
                <div className="flex gap-2 mb-2">
                  {index === 0 && <span className="tag text-accent">recommended · best value</span>}
                  {index === selectedIndex && <span className="tag">selected departure</span>}
                </div>
                <FlightDetails flight={f} />
              </div>
              <div className="text-right">
                <div className="mono text-xl text-accent">{usd(f.price_usd)}</div>
                <div className="text-xs text-muted">round trip / traveler</div>
                <button className="btn-ghost mt-3" disabled={locked || !plan.travelers || (index === selectedIndex && plan.recommended_mode === 'fly')}
                  onClick={() => select(index)} aria-pressed={index === selectedIndex}>choose departure</button>
              </div>
            </div>
          </div>
        ))}
        {choices.length === 0 && <p className="text-muted">No flights match this filter. Turn off “Nonstop only” to see connecting flights.</p>}
      </div>
      {choices.length > 5 && <button className="btn-ghost justify-self-start" onClick={() => setShowAll(!showAll)}>{showAll ? 'show fewer' : `show all ${choices.length} departure options`}</button>}
      {selected && search && (
        <div className="border-t border-line pt-4 grid gap-3">
          <h4 className="mono font-bold">Return flight</h4>
          <p className="text-sm text-muted">For your selected {selected.carrier} departure {selected.departure_time || selected.depart}.</p>
          {!selected.departure_token ? <p className="text-amber">This result has no return-selection token. Re-plan or choose another departure.</p> : (
            <button className="btn-ghost justify-self-start" onClick={loadReturns} disabled={locked}> {returns.length ? 'refresh return options' : 'find return flights'}</button>
          )}
          {plan.selected_return && <div className="panel border-accent p-3"><span className="tag text-accent">selected return</span><FlightDetails flight={plan.selected_return} /><p className="mono mt-2">{usd(plan.selected_return.price_usd)} round trip / traveler</p></div>}
          {(showAllReturns ? returns : returns.slice(0, 5)).map((f, i) => (
            <div className="panel p-4 flex flex-wrap justify-between gap-3" key={i}>
              <div>{i === 0 && <span className="tag text-accent">recommended return</span>}<FlightDetails flight={f} /></div>
              <div className="text-right"><div className="mono text-accent">{usd(f.price_usd)} / traveler</div>
                <button className="btn-ghost mt-2" disabled={locked || !f.booking_token} onClick={() => select(selectedIndex, f.booking_token)}>choose return</button>
                {!f.booking_token && <p className="text-xs text-amber mt-1">Provider did not return a selectable booking token.</p>}
              </div>
            </div>
          ))}
          {returns.length > 5 && <button className="btn-ghost justify-self-start" onClick={() => setShowAllReturns(!showAllReturns)}>{showAllReturns ? 'show fewer returns' : `show all ${returns.length} return options`}</button>}
          {plan.selected_return && (
            <div className="grid gap-3">
              <button className="btn-primary justify-self-start" disabled={locked || !plan.selected_return.booking_token} onClick={loadBookings}>find booking options</button>
              {!plan.selected_return.booking_token && <p className="text-amber">No booking token returned. Choose another return or refresh the search.</p>}
              {bookings.map((option, i) => (
                <div key={i} className="grid gap-2">
                  {option.separate_tickets && <p className="text-amber text-sm">Separate tickets: book both legs separately. Connection protection may differ.</p>}
                  {option.together && <BookingLink seller={option.together} />}
                  {option.departing && <><p className="label">departure ticket</p><BookingLink seller={option.departing} /></>}
                  {option.returning && <><p className="label">return ticket</p><BookingLink seller={option.returning} /></>}
                </div>
              ))}
              {bookings.length > 0 && <p className="text-xs text-muted">Continue on the provider’s website to complete booking. TrailMix does not purchase these tickets. Confirm fare, baggage, fees and cancellation terms there.</p>}
            </div>
          )}
        </div>
      )}
      {busy && <p role="status" className="mono text-cyan text-sm">{busy}…</p>}
      {error && <p role="alert" className="text-rose text-sm">{error} Try refreshing options or re-planning if the search has expired.</p>}
    </section>
  )
}
