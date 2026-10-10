import assert from 'node:assert/strict'
import test from 'node:test'
import { destinationTransport, roadStops, showTransportItem } from '../src/lib/transport.ts'
import { planDataWarnings, planRealData } from '../src/lib/dataWarnings.ts'

test('driving brings an own car; flying preserves requested rental', () => {
  assert.equal(destinationTransport({recommended_mode: 'drive', local_transport: 'rental_car'}), 'own car')
  assert.equal(destinationTransport({recommended_mode: 'fly', local_transport: 'rental_car'}), 'rental car')
  assert.equal(destinationTransport({recommended_mode: 'fly', local_transport: 'auto'}), 'transit/rideshare')
  assert.equal(destinationTransport({recommended_mode: 'drive', local_transport: 'walking'}), 'walking')
})

test('own-car setup is hidden in saved plans without hiding departure or rental items', () => {
  const plan = {recommended_mode: 'drive', local_transport: 'auto'}
  const item = {category: 'transport', title: 'Arrange destination transportation: own car', cost_usd: 0}
  assert.equal(showTransportItem(plan, item), false)
  assert.equal(showTransportItem(plan, {...item, title: 'Depart Kansas City'}), true)
  assert.equal(showTransportItem({recommended_mode: 'fly', local_transport: 'rental_car'},
    {...item, title: 'Arrange destination transportation: rental car'}), true)
})

test('legacy flight plans exclude road stops and their price/source disclosures', () => {
  const plan = {
    recommended_mode: 'fly', sources: {}, drive: {source: 'eia'}, ai_generated: true,
    flights: [], hotels: [], activities: [], food: [],
    stops: [{source: 'mock', price_unknown: true}, {source: 'geoapify', price_unknown: true}],
  }
  assert.deepEqual(roadStops(plan), [])
  assert.ok(!planDataWarnings(plan).some(w => /Rest stops|Geoapify place prices/.test(w)))
  assert.ok(!planRealData(plan).some(w => w.startsWith('Rest stops:')))
  assert.equal(roadStops({...plan, recommended_mode: 'drive'}).length, 2)
})

test('driving disclosures do not suggest renting another car', () => {
  const plan = {
    recommended_mode: 'drive', local_transport: 'rental_car', sources: {},
    drive: {source: 'eia'}, flights: [], hotels: [], activities: [], food: [],
    stops: [], ai_generated: true,
  }
  assert.ok(planDataWarnings(plan).some(w => w.includes('parking and local fuel')))
  assert.ok(!planDataWarnings(plan).some(w => w.includes('rental')))
})
