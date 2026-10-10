import { useEffect, useState } from 'react'

/**
 * Terminal-style step log shown while a plan request is in flight.
 *
 * A real plan takes 60-120 s on the live site: SerpAPI flight + hotel search,
 * places, weather, then two model calls on Akamai (research, then the writer).
 * Steps advance on a realistic schedule (cumulative seconds) so the log keeps
 * moving for the whole wait instead of stalling on the last line after 6 s.
 */
const STEPS: { at: number; text: string }[] = [
  { at: 0, text: 'geocoding origin + destination' },
  { at: 2, text: 'routing the drive, pricing gas for your car' },
  { at: 5, text: 'searching round-trip flights (live)' },
  { at: 20, text: 'checking hotels near the destination (live)' },
  { at: 35, text: 'finding things to do, food, and stops on the way' },
  { at: 45, text: 'pulling the NWS forecast and safety data' },
  { at: 50, text: 'scoring drive vs fly for value' },
  { at: 55, text: 'AI on Akamai: researching the destination' },
  { at: 80, text: 'AI on Akamai: writing your day-by-day itinerary' },
]

export default function PlanProgress({ active }: { active: boolean }) {
  const [elapsed, setElapsed] = useState(0)
  useEffect(() => {
    if (!active) { setElapsed(0); return }
    const started = Date.now()
    const id = setInterval(() => setElapsed(Math.floor((Date.now() - started) / 1000)), 500)
    return () => clearInterval(id)
  }, [active])
  if (!active) return null

  const n = STEPS.reduce((acc, s, i) => (elapsed >= s.at ? i : acc), 0)
  const mm = String(Math.floor(elapsed / 60)).padStart(1, '0')
  const ss = String(elapsed % 60).padStart(2, '0')

  return (
    <div className="panel p-4 mono text-sm grid gap-1" aria-live="polite">
      <div className="flex items-center justify-between text-[11px] text-muted mb-1">
        <span>planning · real prices + AI, usually 1–2 min</span>
        <span className="text-dim tabular-nums">{mm}:{ss}</span>
      </div>
      {STEPS.slice(0, n + 1).map((s, i) => (
        <div key={s.text} className={i === n ? 'text-cyan' : 'text-dim'}>
          <span className="text-accent">{i === n ? '›' : '✓'}</span> {s.text}{i === n && <span className="caret" />}
        </div>
      ))}
      {elapsed >= 100 && (
        <div className="text-[11px] text-amber mt-1">still working: the model is finishing the itinerary. Hang tight, this is the slow part.</div>
      )}
      {elapsed >= 170 && (
        <div className="text-[11px] text-rose">this is taking longer than usual. If it errors out, try once more.</div>
      )}
    </div>
  )
}
