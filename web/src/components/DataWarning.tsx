export default function DataWarning({ details }: { details: string[] }) {
  if (details.length === 0) return null
  return (
    <section role="alert" className="rounded-lg border-2 border-rose bg-rose/10 p-5 text-rose">
      <h2 className="mono text-xl sm:text-2xl font-bold">WARNING: THIS PLAN USES MOCK OR NON-LIVE DATA</h2>
      <p className="mt-2 font-semibold">Some of this data does not come from live APIs. Check the items below before you rely on this trip.</p>
      <ul className="mt-3 list-disc pl-5 grid gap-1 text-sm">
        {details.map(detail => <li key={detail}>{detail}</li>)}
      </ul>
    </section>
  )
}
