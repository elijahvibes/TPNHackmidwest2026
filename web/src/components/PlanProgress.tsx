import { useEffect, useState } from 'react'
import type { JobEvent } from '../lib/api'

/**
 * Terminal-style log shown while a plan is being built.
 *
 * With `events` (from a background planning job) it shows the real milestones
 * as the server reports them: provider searches, then each AI agent on Akamai.
 * Without events it falls back to a scheduled script so the log keeps moving.
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

const STAGE_LABEL: Record<string, string> = {
  search: 'searching real prices', draft: 'draft ready', agents: 'AI agents · curator + researcher',
  writer: 'AI agents · writer', review: 'AI agents · reviewer', done: 'done', error: 'failed',
}

export default function PlanProgress({ active, events, stage, compact }: { active: boolean; events?: JobEvent[]; stage?: string; compact?: boolean }) {
  const [elapsed, setElapsed] = useState(0)
  useEffect(() => {
    if (!active) { setElapsed(0); return }
    const started = Date.now()
    const id = setInterval(() => setElapsed(Math.floor((Date.now() - started) / 1000)), 500)
    return () => clearInterval(id)
  }, [active])
  if (!active) return null

  const mm = String(Math.floor(elapsed / 60))
  const ss = String(elapsed % 60).padStart(2, '0')
  const live = events && events.length > 0
  const shown = live ? events.slice(-(compact ? 4 : 12)) : []
  const n = STEPS.reduce((acc, s, i) => (elapsed >= s.at ? i : acc), 0)

  return (
    <div className="panel p-4 mono text-sm grid gap-1" aria-live="polite">
      <div className="flex items-center justify-between text-[11px] text-muted mb-1">
        <span>{live ? (STAGE_LABEL[stage ?? ''] ?? 'planning') : 'planning · real prices + AI, usually 1–2 min'}</span>
        <span className="text-dim tabular-nums">{mm}:{ss}</span>
      </div>
      {live ? shown.map((e, i) => {
        const last = i === shown.length - 1
        const color = e.status === 'failed' ? 'text-rose' : last ? 'text-cyan' : 'text-dim'
        return (
          <div key={`${e.at}-${i}`} className={color}>
            <span className="text-accent">{last ? '›' : '✓'}</span> {e.agent ? <span className="text-ink">{agentName(e.agent)}: </span> : null}{e.message}{last && <span className="caret" />}
          </div>
        )
      }) : STEPS.slice(0, n + 1).map((s, i) => (
        <div key={s.text} className={i === n ? 'text-cyan' : 'text-dim'}>
          <span className="text-accent">{i === n ? '›' : '✓'}</span> {s.text}{i === n && <span className="caret" />}
        </div>
      ))}
      {!live && elapsed >= 100 && (
        <div className="text-[11px] text-amber mt-1">still working: the model is finishing the itinerary.</div>
      )}
    </div>
  )
}

function agentName(agent: string): string {
  const [role, kind] = agent.split(':')
  const name = role.charAt(0).toUpperCase() + role.slice(1)
  return kind ? `${name} (${kind})` : name
}
