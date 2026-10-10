import type { Plan } from './api'

const sourceLabels: Record<string, string> = {
  route: 'Driving route', gas: 'Gas price', flights: 'Flights', hotels: 'Hotels',
  places: 'Places', weather: 'Weather', safety: 'Safety', ai: 'Itinerary',
}

export function planDataWarnings(plan: Plan): string[] {
  const warnings = new Set<string>()
  const check = (label: string, source: string | undefined) => {
    if (!source) return
    if (/mock/i.test(source)) warnings.add(`${label}: mock data in use.`)
    else if (/fallback|estimate/i.test(source)) warnings.add(`${label}: fallback data or estimates, not live results.`)
    else if (/built-in|national average/i.test(source)) warnings.add(`${label}: static built-in data, not a live API.`)
  }

  for (const [key, source] of Object.entries(plan.sources)) {
    check(sourceLabels[key] ?? key, source)
  }
  check('Gas price', plan.drive.source)
  for (const flight of plan.flights) check('Flights', flight.source)
  check('Flights', plan.selected_return?.source)
  for (const hotel of plan.hotels) check('Hotels', hotel.source)
  for (const [label, places] of [
    ['Attractions', plan.activities], ['Restaurants', plan.food], ['Rest stops', plan.stops],
  ] as const) {
    for (const place of places ?? []) check(label, place.source)
  }
  check('Weather', plan.weather?.source)
  check('Safety', plan.safety?.source)
  if (plan.sources.safety === 'fbi-ucr' && !/built-in|national average/i.test(plan.safety?.source ?? '')) {
    warnings.add('Safety: static built-in data, not a live API.')
  }
  if ([...(plan.activities ?? []), ...(plan.food ?? []), ...(plan.stops ?? [])].some(p => p.source === 'google')) {
    warnings.add('Place costs: estimated from Google price levels, not admission or menu prices. A cost of zero can mean the price is unknown.')
  }
  if (!plan.ai_generated) {
    warnings.add('Itinerary: rule-based, not written by AI.')
  }
  return [...warnings]
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
    ['Attractions', plan.activities], ['Restaurants', plan.food], ['Rest stops', plan.stops],
  ] as const) {
    const count = (places ?? []).filter(place => place.source === 'google').length
    if (count > 0) {
      real.push(`${label}: ${count} of ${places.length} places from Google Places (names, locations and available ratings; costs are estimates).`)
    }
  }
  if (plan.drive.source === 'eia') real.push('Gas price: EIA weekly US regular gasoline price, not a local station quote.')
  if (plan.sources.route === 'mapbox') real.push('Driving distance and travel time: Mapbox Directions.')
  if (plan.weather?.source === 'nws' && plan.weather.days?.length > 0) {
    real.push(`Weather: ${plan.weather.days.length} trip days with National Weather Service forecasts.`)
  }
  if (plan.ai_generated) real.push('Itinerary narrative: AI-generated, not rule-based. This does not verify prices or availability.')
  return real
}
