import assert from 'node:assert/strict'
import test from 'node:test'
import { isMockPayment, planDataWarnings, planEstimates, planRealData } from '../src/lib/dataWarnings.ts'

function livePlan() {
  return {
    sources: { flights: 'serpapi', hotels: 'serpapi', gas: 'eia', route: 'mapbox', places: 'google', weather: 'nws' },
    drive: { source: 'eia' }, flights: [{ source: 'serpapi' }], hotels: [{ source: 'serpapi' }],
    activities: [], food: [], stops: [], weather: { source: 'nws' }, ai_generated: true,
  }
}

test('no warning for live sources without static or estimated data', () => {
  assert.deepEqual(planDataWarnings(livePlan()), [])
})

test('destination transport is an estimate, disclosed but not a warning', () => {
  const plan = {...livePlan(), local_transport: 'rental_car', activity_radius_miles: 50}
  assert.deepEqual(planDataWarnings(plan), [])
  assert.ok(planEstimates(plan).some(w => w.startsWith('Getting around (rental car)') && w.includes('included in the total')))
  assert.ok(!planRealData(plan).some(w => w.startsWith('Destination transportation')))
})

test('estimated place prices are disclosed with their count', () => {
  const plan = livePlan()
  plan.activities = [{ source: 'geoapify', price_estimated: true, price_usd: 18 }, { source: 'geoapify', price_estimated: true, price_usd: 0 }]
  plan.food = [{ source: 'google', price_usd: 25 }]
  assert.deepEqual(planDataWarnings(plan), [])
  assert.ok(planEstimates(plan).some(w => w.startsWith('Place prices: 2 of 3 places use typical prices')))
})

test('mixed live/mock places are detected even when the summary source is Google', () => {
  const plan = livePlan()
  plan.activities = [{ source: 'google' }]
  plan.food = [{ source: 'mock' }]
  plan.stops = [{ source: 'mock' }]
  const warnings = planDataWarnings(plan)
  assert.ok(warnings.includes('Restaurants: mock data is being used.'))
  assert.ok(warnings.includes('Rest stops: mock data is being used.'))
  assert.ok(planEstimates(plan).some(w => w.startsWith('Place costs:')))
  assert.ok(!warnings.some(w => w === 'Attractions: mock data is being used.'))
})

test('legacy mock flights/hotels and selected return are detected from individual quotes', () => {
  const plan = livePlan()
  plan.flights = [{ source: 'mock' }, { source: 'mock' }]
  plan.hotels = [{ source: 'mock' }]
  plan.selected_return = { source: 'mock' }
  const warnings = planDataWarnings(plan)
  assert.equal(warnings.filter(w => w === 'Flights: mock data is being used.').length, 1)
  assert.ok(warnings.includes('Hotels: mock data is being used.'))
})

test('fallback gas, estimated route, mock weather, static safety and rule-based itinerary are identified', () => {
  const plan = livePlan()
  plan.sources.route = 'estimate'
  plan.drive.source = 'fallback'
  plan.weather.source = 'mock'
  plan.sources.safety = 'fbi-ucr'
  plan.safety = { source: 'national average (city not in table)' }
  plan.ai_generated = false
  const warnings = planDataWarnings(plan)
  for (const prefix of ['Gas price:', 'Weather:', 'Safety:', 'Itinerary:']) {
    assert.ok(warnings.some(w => w.startsWith(prefix)), prefix)
  }
  // A great-circle route is an estimate, not fake data.
  assert.ok(!warnings.some(w => w.startsWith('Driving route:')))
  assert.ok(planEstimates(plan).some(w => w.startsWith('Driving distance and time: straight-line estimate')))
})

test('free live sources are credited as real data', () => {
  const plan = livePlan()
  plan.sources.route = 'osrm'
  plan.sources.safety = 'fbi-cde'
  plan.safety = { source: 'fbi-cde (live, CO statewide 2025)' }
  assert.deepEqual(planDataWarnings(plan), [])
  const real = planRealData(plan)
  assert.ok(real.some(w => w.startsWith('Driving distance and travel time: OSRM')))
  assert.ok(real.some(w => w.startsWith('Safety: FBI Crime Data Explorer')))
})

test('old plans without weather or safety still detect summary-only mock sources', () => {
  const plan = livePlan()
  delete plan.weather
  plan.sources.places = 'mock'
  plan.sources.safety = 'fbi-ucr'
  assert.ok(planDataWarnings(plan).includes('Places: mock data is being used.'))
  assert.ok(planDataWarnings(plan).some(w => w.startsWith('Safety:')))
})

test('saved mock payments remain identifiable after live mode is enabled', () => {
  assert.equal(isMockPayment('JH-MOCK-ab12'), true)
  assert.equal(isMockPayment('real-payment-id'), false)
  assert.equal(isMockPayment(null), false)
})

test('real-data disclosure identifies actual provider results and excludes empty weather', () => {
  const plan = livePlan()
  plan.ai_generated = false
  assert.deepEqual(planRealData(plan), [
    'Flights: 1 of 1 results from Google Flights via SerpApi.',
    'Hotels: 1 of 1 results from Google Hotels via SerpApi.',
    'Gas price: EIA weekly US regular gasoline price, not a local station quote.',
    'Driving distance and travel time: Mapbox Directions.',
  ])
})

test('mixed categories disclose real counts while retaining mock warnings', () => {
  const plan = livePlan()
  plan.flights.push({ source: 'mock' })
  plan.activities = [{ source: 'google' }, { source: 'mock' }]
  plan.food = [{ source: 'mock' }]
  plan.stops = [{ source: 'google' }]
  const real = planRealData(plan)
  assert.ok(real.includes('Flights: 1 of 2 results from Google Flights via SerpApi.'))
  assert.ok(real.some(detail => detail.startsWith('Attractions: 1 of 2 places from Google Places')))
  assert.ok(real.some(detail => detail.startsWith('Rest stops: 1 of 1 places from Google Places')))
  assert.ok(!real.some(detail => detail.startsWith('Restaurants:')))
  assert.ok(planDataWarnings(plan).includes('Attractions: mock data is being used.'))
})

test('summary tags alone do not claim empty or mock results are real', () => {
  const plan = livePlan()
  plan.flights = [{ source: 'mock' }]
  plan.hotels = []
  plan.drive.source = 'fallback'
  plan.sources.route = 'estimate'
  plan.weather = { source: 'mock', days: [{}] }
  plan.safety = { source: 'fbi-ucr (built-in table)' }
  plan.ai_generated = false
  assert.deepEqual(planRealData(plan), [])
})

test('real return selection, forecast coverage and AI narrative are explicitly identified', () => {
  const plan = livePlan()
  plan.selected_return = { source: 'serpapi' }
  plan.weather.days = [{}, {}]
  const real = planRealData(plan)
  assert.ok(real.includes('Selected return flight: Google Flights via SerpApi.'))
  assert.ok(real.includes('Weather: 2 trip days with National Weather Service forecasts.'))
  assert.ok(real.some(detail => detail.startsWith('Itinerary narrative: AI-generated')))
})

test('Geoapify real locations are disclosed without claiming ratings or prices', () => {
  const plan = livePlan()
  plan.activities = [{ source: 'geoapify', price_unknown: true }, { source: 'mock' }]
  const real = planRealData(plan)
  assert.ok(real.some(w => w.includes('Attractions: 1 of 2 places from Geoapify / OpenStreetMap')))
  assert.ok(planDataWarnings(plan).includes('Attractions: mock data is being used.'))
  assert.ok(planEstimates(plan).some(w => w.includes('no price and count as $0')))
})
