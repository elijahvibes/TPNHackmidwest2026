import type { DayItem, Place, Plan } from './api'

export function placeMapLink(place: Place): string | undefined {
  if (!['google', 'geoapify'].includes(place.source)) return undefined
  const loc = place.location
  if (!loc || !Number.isFinite(loc.lat) || !Number.isFinite(loc.lng) ||
    Math.abs(loc.lat) > 90 || Math.abs(loc.lng) > 180) return undefined
  return `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(`${loc.lat},${loc.lng}`)}`
}

const normalize = (name: string) => name.trim().toLocaleLowerCase()

export function itineraryMapLink(plan: Plan, item: DayItem): string | undefined {
  const lists: Record<string, Place[] | undefined> = {
    activity: plan.activities, food: plan.food, stop: plan.stops,
  }
  const name = normalize(item.title.replace(/^(?:Dinner at|Lunch at|Breakfast at|Visit|Explore|Stop at|Check in:)\s+/i, ''))
  if (item.category === 'lodging') {
    const hotels = plan.hotels.filter(h => normalize(h.name) === name)
    if (hotels.length !== 1 || !['serpapi', 'google', 'amadeus'].includes(hotels[0].source)) return undefined
    const destination = plan.destination_name || plan.title.split('→')[1]?.trim()
    if (!destination) return undefined
    return `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(`${hotels[0].name}, ${destination}`)}`
  }
  const matches = (lists[item.category] ?? []).filter(p => normalize(p.name) === name)
  const links = matches.map(placeMapLink)
  if (links.length === 0 || links.some(link => !link) || new Set(links).size !== 1) return undefined
  return links[0]
}
