import assert from 'node:assert/strict'
import test from 'node:test'
import { itineraryMapLink, placeMapLink } from '../src/lib/placeLinks.ts'

const cafe = { name: 'Cafe & Grill', source: 'geoapify', location: { lat: 41.88, lng: -87.63 } }
const plan = { title: 'Kansas City → Chicago', activities: [], food: [cafe], stops: [], hotels: [] }

test('real providers link to exact coordinates without a key', () => {
  const url = new URL(placeMapLink(cafe))
  assert.equal(url.origin, 'https://www.google.com')
  assert.equal(url.searchParams.get('api'), '1')
  assert.equal(url.searchParams.get('query'), '41.88,-87.63')
  assert.equal(placeMapLink({ ...cafe, source: 'google' }), url.href)
})

test('mock places, unknown sources and invalid coordinates never receive links', () => {
  for (const p of [
    { ...cafe, source: 'mock' }, { ...cafe, source: '' },
    { ...cafe, location: undefined }, { ...cafe, location: { lat: NaN, lng: 0 } },
    { ...cafe, location: { lat: 91, lng: 0 } },
  ]) assert.equal(placeMapLink(p), undefined)
})

test('itinerary links handle dinner prefixes and exact names, not arbitrary event text', () => {
  assert.equal(itineraryMapLink(plan, { category: 'food', title: 'Dinner at Cafe & Grill' }), placeMapLink(cafe))
  assert.equal(itineraryMapLink({ ...plan, activities: [cafe] }, { category: 'activity', title: 'Visit Cafe & Grill' }), placeMapLink(cafe))
  assert.equal(itineraryMapLink(plan, { category: 'activity', title: 'Concert at Cafe & Grill' }), undefined)
  assert.equal(itineraryMapLink(plan, { category: 'transport', title: cafe.name }), undefined)
})

test('ambiguous duplicate names are not linked to a potentially wrong location', () => {
  assert.equal(itineraryMapLink({ ...plan, food: [cafe, { ...cafe, location: { lat: 42, lng: -88 } }] },
    { category: 'food', title: cafe.name }), undefined)
})

test('real hotel gets an encoded destination search, mock hotel does not', () => {
  const hotel = { name: 'Hotel & Spa', source: 'serpapi' }
  const item = { category: 'lodging', title: 'Check in: Hotel & Spa' }
  const url = new URL(itineraryMapLink({ ...plan, hotels: [hotel] }, item))
  assert.equal(url.searchParams.get('query'), 'Hotel & Spa, Chicago')
  assert.equal(itineraryMapLink({ ...plan, hotels: [{ ...hotel, source: 'mock' }] }, item), undefined)
})
