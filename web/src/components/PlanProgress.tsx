import { useEffect, useState } from 'react'

const STEPS = [
  'geocoding origin + destination',
  'routing the drive, pricing gas for your car',
  'searching round-trip flights',
  'checking hotels near the destination',
  'finding things to do, food, and stops on the way',
  'scoring drive vs fly for value',
  'asking the AI on Akamai to write the itinerary',
]

/** Terminal-style step log shown while a plan request is in flight. */
export default function PlanProgress({ active }: { active: boolean }) {
  const [n, setN] = useState(0)
  useEffect(() => {
    if (!active) { setN(0); return }
    const id = setInterval(() => setN(i => Math.min(i + 1, STEPS.length - 1)), 900)
    return () => clearInterval(id)
  }, [active])
  if (!active) return null
  return (
    <div className="panel p-4 mono text-sm grid gap-1">
      {STEPS.slice(0, n + 1).map((s, i) => (
        <div key={s} className={i === n ? 'text-cyan' : 'text-dim'}>
          <span className="text-accent">{i === n ? '›' : '✓'}</span> {s}{i === n && <span className="caret" />}
        </div>
      ))}
    </div>
  )
}
