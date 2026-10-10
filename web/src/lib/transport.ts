import type { DayItem, Plan } from './api'

export function destinationTransport(plan: Pick<Plan, 'recommended_mode' | 'local_transport'>): string {
  if (plan.local_transport === 'walking') return 'walking'
  if (plan.local_transport === 'transit_rideshare') return 'transit/rideshare'
  if (plan.recommended_mode === 'drive') return 'own car'
  return plan.local_transport === 'rental_car' ? 'rental car' : 'transit/rideshare'
}

export function roadStops(plan: Pick<Plan, 'recommended_mode' | 'stops'>) {
  return plan.recommended_mode === 'fly' ? [] : plan.stops ?? []
}

export function showTransportItem(plan: Pick<Plan, 'recommended_mode' | 'local_transport'>, item: DayItem): boolean {
  if (plan.recommended_mode === 'fly' && item.category === 'stop') return false
  return !(destinationTransport(plan) === 'own car' && item.category === 'transport'
    && item.title === 'Arrange destination transportation: own car')
}
