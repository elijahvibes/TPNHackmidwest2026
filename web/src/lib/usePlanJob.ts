import { useEffect, useRef, useState } from 'react'
import { api, type Plan, type PlanJob } from './api'

/**
 * Polls a background planning job. The server hands back a rule-based draft
 * within a few seconds and streams agent milestones; `plan` is the latest
 * version (draft, then final), `done` flips when the AI itinerary is in.
 */
export function usePlanJob(jobId: string | null, onDone?: (plan: Plan) => void) {
  const [job, setJob] = useState<PlanJob | null>(null)
  const [error, setError] = useState('')
  const doneRef = useRef<typeof onDone>(onDone)
  doneRef.current = onDone

  useEffect(() => {
    setJob(null); setError('')
    if (!jobId) return
    let stop = false
    let timer: ReturnType<typeof setTimeout> | undefined
    const tick = async () => {
      try {
        const j = await api.planJob(jobId)
        if (stop) return
        setJob(j)
        if (j.status === 'error') { setError(j.error || 'planning failed'); return }
        if (j.status === 'done' && j.plan) { doneRef.current?.(j.plan); return }
      } catch (e) {
        if (stop) return
        setError((e as Error).message); return
      }
      timer = setTimeout(tick, 1500)
    }
    tick()
    return () => { stop = true; if (timer) clearTimeout(timer) }
  }, [jobId])

  const running = !!jobId && !!job && job.status === 'running'
  return { job, plan: job?.plan ?? null, events: job?.events ?? [], running: !!jobId && !error && (!job || job.status === 'running'), done: job?.status === 'done', error, draft: running && !!job?.plan }
}
