import type { ReactNode } from 'react'

/** Page heading in the editorial style: tracked eyebrow, large serif title, one-line lede. */
export default function Hero({ eyebrow, title, lede, aside }: { eyebrow?: string; title: ReactNode; lede?: ReactNode; aside?: ReactNode }) {
  return (
    <section className="flex flex-wrap items-end justify-between gap-4 pt-2 pb-2">
      <div className="min-w-0">
        {eyebrow && <div className="eyebrow">{eyebrow}</div>}
        <h1 className="text-[40px] sm:text-[48px] leading-[1.05] text-ink mt-2">{title}</h1>
        {lede && <p className="text-[15px] text-muted mt-3 max-w-2xl">{lede}</p>}
      </div>
      {aside}
    </section>
  )
}
