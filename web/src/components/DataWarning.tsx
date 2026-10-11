export default function DataWarning({ details }: { details: string[] }) {
  if (details.length === 0) return null
  return (
    <section role="alert" className="rounded-xl border-2 border-rose/60 bg-rose/5 p-5 text-rose">
      <h2 className="text-[20px] sm:text-[22px]">Warning: this plan uses mock or non-live data</h2>
      <p className="mt-1.5 text-sm font-medium">Some of this data does not come from live APIs. Check the items below before you rely on this trip.</p>
      <ul className="mt-3 list-disc pl-5 grid gap-1 text-sm">
        {details.map(detail => <li key={detail}>{detail}</li>)}
      </ul>
    </section>
  )
}

/** Honest estimates that are part of the total: shown as a note, not an alarm. */
export function EstimatesNote({ details }: { details: string[] }) {
  if (details.length === 0) return null
  return (
    <details className="rounded-xl border border-amber/40 bg-amber/5 px-5 py-3.5 text-sm text-muted print:hidden">
      <summary className="text-[11px] font-semibold uppercase tracking-[0.1em] text-amber cursor-pointer">
        Estimates in this total · {details.length}
      </summary>
      <ul className="mt-2 list-disc pl-5 grid gap-1">
        {details.map(detail => <li key={detail}>{detail}</li>)}
      </ul>
    </details>
  )
}
