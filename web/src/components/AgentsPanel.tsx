import { useState } from 'react'
import type { AgentReport, Plan, Research } from '../lib/api'

const STATUS: Record<AgentReport['status'], string> = { ok: 'text-accent', skipped: 'text-dim', failed: 'text-rose' }
const NAME: Record<string, string> = {
  'curator:activities': 'Curator · things to do', 'curator:food': 'Curator · restaurants', 'curator:stops': 'Curator · road stops',
  researcher: 'Researcher', writer: 'Writer', 'reviewer:audit': 'Reviewer · data audit', reviewer: 'Reviewer · critique',
}

/** The Researcher agent's briefing: what a local friend would tell you. Model knowledge, labeled as such. */
export function ResearchCard({ r }: { r?: Research }) {
  if (!r?.overview && !r?.tips?.length) return null
  return (
    <section className="panel p-5 print:break-inside-avoid" aria-label="destination research">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h3 className="label">local briefing</h3>
        <span className="tag text-accent">✦ researched by AI on Akamai</span>
      </div>
      {r.overview && <p className="text-muted mt-3 leading-relaxed">{r.overview}</p>}
      {r.seasonal && <p className="text-sm text-muted mt-2"><span className="text-ink">These dates:</span> {r.seasonal}</p>}
      <div className="grid sm:grid-cols-2 gap-4 mt-4 text-sm">
        {!!r.neighborhoods?.length && <List title="where to base yourself" items={r.neighborhoods} />}
        {!!r.tips?.length && <List title="tips" items={r.tips} />}
        {!!r.avoid?.length && <List title="skip" items={r.avoid} color="text-amber" />}
        {!!r.place_notes?.length && (
          <div>
            <h4 className="label">about the places in this plan</h4>
            <ul className="mt-1.5 grid gap-1 text-muted">
              {r.place_notes.map(n => <li key={n.name}><span className="text-ink">{n.name}</span> — {n.note}</li>)}
            </ul>
          </div>
        )}
      </div>
      <p className="mono text-[10px] text-dim mt-4">{r.source}. No prices or opening hours come from this briefing.</p>
    </section>
  )
}

function List({ title, items, color = 'text-ink' }: { title: string; items: string[]; color?: string }) {
  return (
    <div>
      <h4 className="label">{title}</h4>
      <ul className="mt-1.5 grid gap-1 text-muted">
        {items.map((t, i) => <li key={i} className="flex gap-2"><span className={`${color} shrink-0`}>·</span><span>{t}</span></li>)}
      </ul>
    </div>
  )
}

/** What each agent did to this plan, so the AI's work is inspectable rather than a black box. */
export function AgentsCard({ plan }: { plan: Plan }) {
  const [open, setOpen] = useState(false)
  const reports = plan.agents ?? []
  if (!reports.length) return null
  const issues = reports.filter(r => r.status === 'ok' && r.notes?.length).reduce((n, r) => n + (r.notes?.length ?? 0), 0)
  return (
    <div className="panel p-4 print:hidden">
      <div className="flex items-center justify-between gap-3">
        <h4 className="label">how the AI built this</h4>
        <button className="tag hover:text-ink" onClick={() => setOpen(o => !o)} aria-expanded={open}>
          {reports.length} agents · {issues} note{issues === 1 ? '' : 's'} {open ? '▾' : '▸'}
        </button>
      </div>
      {open && (
        <ol className="mt-3 grid gap-2.5 text-sm">
          {reports.map((r, i) => (
            <li key={i}>
              <div className="flex items-baseline gap-2">
                <span className={`mono text-[10px] uppercase tracking-wider ${STATUS[r.status] ?? 'text-muted'}`}>{r.status}</span>
                <span className="text-ink">{NAME[r.agent] ?? r.agent}</span>
              </div>
              <div className="text-muted text-[13px]">{r.summary}</div>
              {!!r.notes?.length && (
                <ul className="mt-1 grid gap-0.5 text-[12px] text-dim">
                  {r.notes.map((n, j) => <li key={j}>· {n}</li>)}
                </ul>
              )}
            </li>
          ))}
          <li className="mono text-[10px] text-dim">Curator and Reviewer may only choose from or remove real provider places; they never add places or prices.</li>
        </ol>
      )}
    </div>
  )
}
