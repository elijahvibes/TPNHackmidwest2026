import type { Plan } from './api'
import { destinationTransport, roadStops } from './transport.ts'

const sourceLabels: Record<string, string> = {
  route: 'Driving route', gas: 'Gas price', flights: 'Flights', hotels: 'Hotels',
  places: 'Places', weather: 'Weather', safety: 'Safety', ai: 'Itinerary',
  local_transport: 'Destination transportation', place_prices: 'Place prices',
}

/**
 * Hard warnings: data that is fake (mock), a hard-coded constant (fallback)
 * or a static table where a live API was expected. Honest estimates are not
 * warnings; they are listed by planEstimates.
 */
export function planDataWarnings(plan: Plan): string[] {
  const warnings = new Set<string>()
  const check = (label: string, source: string | undefined) => {
    if (!source) return
    if (/mock/i.test(source)) warnings.add(`${label}: mock data is being used.`)
    else if (/fallback/i.test(source)) warnings.add(`${label}: fallback constant, not a live price.`)
    else if (/built-in|national average/i.test(source)) warnings.add(`${label}: static built-in data is being used, not a live API.`)
  }

  for (const [key, source] of Object.entries(plan.sources)) {
    check(sourceLabels[key] ?? key, source)
  }
  check('Gas price', plan.drive.source)
  for (const flight of plan.flights) check('Flights', flight.source)
  check('Flights', plan.selected_return?.source)
  for (const hotel of plan.hotels) check('Hotels', hotel.source)
  for (const [label, places] of [
    ['Attractions', plan.activities], ['Restaurants', plan.food], ['Rest stops', roadStops(plan)],
  ] as const) {
    for (const place of places ?? []) check(label, place.source)
  }
  check('Weather', plan.weather?.source)
  check('Safety', plan.safety?.source)
  if (plan.sources.safety === 'fbi-ucr' && !/built-in|national average/i.test(plan.safety?.source ?? '')) {
    warnings.add('Safety: static built-in data is being used, not a live API.')
  }
  if (!plan.ai_generated) {
    warnings.add('Itinerary: rule-based, not written by AI.')
  }
  return [...warnings]
}

/**
 * Estimates: numbers that are real-world typical values rather than quotes.
 * They are included in totals and disclosed here, not shouted as warnings.
 */
export function planEstimates(plan: Plan): string[] {
  const notes: string[] = []
  const places = [...(plan.activities ?? []), ...(plan.food ?? []), ...roadStops(plan)]
  if (/estimate/i.test(plan.sources.route ?? '')) {
    notes.push('Driving distance and time: straight-line estimate with a road factor (routing service unavailable).')
  }
  const estimated = places.filter(p => p.price_estimated).length
  if (estimated > 0) {
    notes.push(`Place prices: ${estimated} of ${places.length} places use typical prices (admission or an average meal) estimated by category and the AI Curator, marked with ~. Providers do not publish prices.`)
  } else if (places.some(p => p.source === 'google')) {
    notes.push('Place costs: estimated from Google price levels, not actual admission or menu prices.')
  }
  if (places.some(p => p.price_unknown)) {
    notes.push('Some places have no price and count as $0 in the total.')
  }
  if (plan.local_transport || plan.sources.local_transport) {
    const t = destinationTransport(plan)
    const costs = t === 'own car' ? 'parking and local fuel' : t === 'rental car' ? 'a mid-size rental, parking and fuel'
      : t === 'walking' ? 'nothing (walking)' : 'transit day passes and about one rideshare a day'
    notes.push(`Getting around (${t}): ${costs}, estimated per day from national averages and included in the total. Routes and travel times are not verified.`)
  }
  return notes
}

export function isMockPayment(ref: string | null): boolean {
  return ref?.startsWith('JH-MOCK-') ?? false
}

export function planRealData(plan: Plan): string[] {
  const real: string[] = []
  const quotes = (label: string, items: { source: string }[], provider: string, source: string) => {
    const count = items.filter(item => item.source === source).length
    if (count > 0) real.push(`${label}: ${count} of ${items.length} results from ${provider}.`)
  }
  quotes('Flights', plan.flights, 'Google Flights via SerpApi', 'serpapi')
  if (plan.selected_return?.source === 'serpapi') {
    real.push('Selected return flight: Google Flights via SerpApi.')
  }
  quotes('Hotels', plan.hotels, 'Google Hotels via SerpApi', 'serpapi')
  for (const [label, places] of [
    ['Attractions', plan.activities], ['Restaurants', plan.food], ['Rest stops', roadStops(plan)],
  ] as const) {
    const count = (places ?? []).filter(place => place.source === 'google').length
    if (count > 0) {
      real.push(`${label}: ${count} of ${places.length} places from Google Places (names, locations and available ratings; costs are estimates).`)
    }
    const geoCount = (places ?? []).filter(place => place.source === 'geoapify').length
    if (geoCount > 0) {
      real.push(`${label}: ${geoCount} of ${places.length} places from Geoapify / OpenStreetMap (real names and locations; prices and ratings unavailable).`)
    }
    const osm = (places ?? []).filter(place => place.source === 'osm').length
    if (osm > 0) {
      real.push(`${label}: ${osm} of ${places.length} places from OpenStreetMap (names and locations; costs are estimates).`)
    }
  }
  if (plan.drive.source === 'eia') real.push('Gas price: EIA weekly US regular gasoline price, not a local station quote.')
  if (plan.sources.route === 'mapbox') real.push('Driving distance and travel time: Mapbox Directions.')
  if (plan.sources.route === 'osrm') real.push('Driving distance and travel time: OSRM road routing on OpenStreetMap.')
  if (plan.sources.safety === 'fbi-cde') real.push('Safety: FBI Crime Data Explorer, live statewide violent-crime rate.')
  if (plan.weather?.source === 'nws' && plan.weather.days?.length > 0) {
    real.push(`Weather: ${plan.weather.days.length} trip days with National Weather Service forecasts.`)
  }
  if (plan.ai_generated) real.push('Itinerary narrative: AI-generated, not rule-based. This does not verify prices or availability.')
  return real
}
