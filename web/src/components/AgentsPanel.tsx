import { useState } from 'react'
import type { AgentReport, Plan, Research } from '../lib/api'
import Icon from './Icon'

const STATUS: Record<AgentReport['status'], string> = { ok: 'text-accent', skipped: 'text-dim', failed: 'text-rose' }
const NAME: Record<string, string> = {
  'curator:activities': 'Curator · things to do', 'curator:food': 'Curator · restaurants', 'curator:stops': 'Curator · road stops',
  researcher: 'Researcher', writer: 'Writer', 'reviewer:audit': 'Reviewer · data audit', reviewer: 'Reviewer · critique',
}

/** The Researcher agent's briefing: what a local friend would tell you. Model knowledge, labeled as such. */
export function ResearchCard({ r }: { r?: Research }) {
  if (!r?.overview && !r?.tips?.length) return null
  return (
    <section className="panel px-6 py-6 print:break-inside-avoid" aria-label="destination research">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h3 className="text-[22px]">Local briefing</h3>
        <span className="tag text-accent">Researched by AI on Akamai</span>
      </div>
      {r.overview && <p className="text-[15px] text-muted mt-3 leading-relaxed">{r.overview}</p>}
      {r.seasonal && <p className="text-sm text-muted mt-2"><span className="text-ink font-medium">These dates:</span> {r.seasonal}</p>}
      <div className="grid sm:grid-cols-2 gap-5 mt-5 text-sm">
        {!!r.neighborhoods?.length && <List title="Where to base yourself" items={r.neighborhoods} />}
        {!!r.tips?.length && <List title="Tips" items={r.tips} />}
        {!!r.avoid?.length && <List title="Skip" items={r.avoid} />}
        {!!r.place_notes?.length && (
          <div>
            <h4 className="label">About the places in this plan</h4>
            <ul className="mt-1.5 grid gap-1.5 text-muted">
              {r.place_notes.map(n => <li key={n.name}><span className="text-ink">{n.name}</span> — {n.note}</li>)}
            </ul>
          </div>
        )}
      </div>
      <p className="text-[11px] text-dim mt-5">{r.source}. No prices or opening hours come from this briefing.</p>
    </section>
  )
}

function List({ title, items }: { title: string; items: string[] }) {
  return (
    <div>
      <h4 className="label">{title}</h4>
      <ul className="mt-1.5 grid gap-1.5 text-muted list-disc pl-4">
        {items.map((t, i) => <li key={i}>{t}</li>)}
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
    <div className="panel p-5 print:hidden">
      <div className="flex items-center justify-between gap-3">
        <h3 className="text-[19px]">How the AI built this</h3>
        <button className="tag hover:text-ink inline-flex items-center gap-1 shrink-0" onClick={() => setOpen(o => !o)} aria-expanded={open}>
          {reports.length} agents · {issues} note{issues === 1 ? '' : 's'} <Icon name="chevron" size={12} className={open ? 'rotate-180' : ''} />
        </button>
      </div>
      {open && (
        <ol className="mt-3 grid gap-3 text-sm">
          {reports.map((r, i) => (
            <li key={i}>
              <div className="flex items-baseline gap-2">
                <span className={`text-[10px] uppercase tracking-wider font-semibold ${STATUS[r.status] ?? 'text-muted'}`}>{r.status}</span>
                <span className="text-ink">{NAME[r.agent] ?? r.agent}</span>
              </div>
              <div className="text-muted text-[13px]">{r.summary}</div>
              {!!r.notes?.length && (
                <ul className="mt-1 grid gap-0.5 text-[12px] text-dim list-disc pl-4">
                  {r.notes.map((n, j) => <li key={j}>{n}</li>)}
                </ul>
              )}
            </li>
          ))}
          <li className="text-[11px] text-dim">Curator and Reviewer may only choose from or remove real provider places; they never add places or prices.</li>
        </ol>
      )}
    </div>
  )
}
