import { useEffect, useState } from 'react'
import type { JobEvent } from '../lib/api'
import Icon from './Icon'

/**
 * Progress log shown while a plan is being built.
 *
 * With `events` (from a background planning job) it shows the real milestones
 * as the server reports them: provider searches, then each AI agent on Akamai.
 * Without events it falls back to a scheduled script so the log keeps moving.
 */
const STEPS: { at: number; text: string }[] = [
  { at: 0, text: 'Locating origin and destination' },
  { at: 2, text: 'Routing the drive and pricing gas for your car' },
  { at: 5, text: 'Searching round-trip flights (live)' },
  { at: 20, text: 'Checking hotels near the destination (live)' },
  { at: 35, text: 'Finding things to do, food, and stops on the way' },
  { at: 45, text: 'Pulling the NWS forecast and safety data' },
  { at: 50, text: 'Scoring drive vs fly for value' },
  { at: 55, text: 'AI on Akamai: researching the destination' },
  { at: 80, text: 'AI on Akamai: writing your day-by-day itinerary' },
]

const STAGE_LABEL: Record<string, string> = {
  search: 'Searching real prices', draft: 'Draft ready', agents: 'AI agents · curator + researcher',
  writer: 'AI agents · writer', review: 'AI agents · reviewer', done: 'Done', error: 'Failed',
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

  const Row = ({ done, text, failed }: { done: boolean; text: React.ReactNode; failed?: boolean }) => (
    <div className={`flex items-start gap-2.5 ${failed ? 'text-rose' : done ? 'text-dim' : 'text-ink'}`}>
      <span className={`mt-0.5 w-4 h-4 rounded-full border flex items-center justify-center shrink-0 ${done ? 'border-accent/50 text-accent' : 'border-accent bg-accent-soft'}`}>
        {done ? <Icon name="check" size={10} /> : <span className="w-1.5 h-1.5 rounded-full bg-accent animate-pulse" />}
      </span>
      <span>{text}</span>
    </div>
  )

  return (
    <div className="panel px-5 py-4 text-sm grid gap-1.5" aria-live="polite">
      <div className="flex items-center justify-between text-[12px] text-muted mb-1">
        <span>{live ? (STAGE_LABEL[stage ?? ''] ?? 'Planning') : 'Planning · real prices + AI, usually 1–2 min'}</span>
        <span className="text-dim tabular-nums">{mm}:{ss}</span>
      </div>
      {live ? shown.map((e, i) => {
        const last = i === shown.length - 1
        return <Row key={`${e.at}-${i}`} done={!last} failed={e.status === 'failed'}
          text={<>{e.agent ? <span className="font-medium">{agentName(e.agent)}: </span> : null}{e.message}</>} />
      }) : STEPS.slice(0, n + 1).map((s, i) => <Row key={s.text} done={i !== n} text={s.text} />)}
      {!live && elapsed >= 100 && (
        <div className="text-[12px] text-amber mt-1">Still working: the model is finishing the itinerary.</div>
      )}
    </div>
  )
}

function agentName(agent: string): string {
  const [role, kind] = agent.split(':')
  const name = role.charAt(0).toUpperCase() + role.slice(1)
  return kind ? `${name} (${kind})` : name
}
