import { useEffect, useId, useRef, useState } from 'react'
import { api, type Placemark } from '../lib/api'

/**
 * City / town / park autocomplete. Any place works: suggestions come from the
 * API (OpenStreetMap-backed, with the built-in gazetteer as offline fallback).
 * Picking one pins its coordinates so the planner never has to guess.
 */
export default function PlaceInput({ value, pinned, onChange, onPick, placeholder, id, autoFocus }: {
  value: string
  pinned: Placemark | null
  onChange: (text: string) => void
  onPick: (p: Placemark | null) => void
  placeholder?: string
  id?: string
  autoFocus?: boolean
}) {
  const [items, setItems] = useState<Placemark[]>([])
  const [open, setOpen] = useState(false)
  const [hi, setHi] = useState(0)
  const [loading, setLoading] = useState(false)
  const listId = useId()
  const boxRef = useRef<HTMLDivElement>(null)
  const skipNext = useRef(false)
  const typed = useRef(false) // no suggestions until the user actually types

  useEffect(() => {
    if (skipNext.current) { skipNext.current = false; return }
    const q = value.trim()
    if (!typed.current || q.length < 2 || pinned) return
    const ctrl = new AbortController()
    const t = setTimeout(async () => {
      setLoading(true)
      try {
        const res = await api.geoSuggest(q, ctrl.signal)
        setItems(res); setHi(0); setOpen(res.length > 0)
      } catch { /* aborted or offline: keep what we have */ } finally { setLoading(false) }
    }, 220)
    return () => { clearTimeout(t); ctrl.abort() }
  }, [value, pinned])

  useEffect(() => {
    const onDoc = (e: MouseEvent) => { if (!boxRef.current?.contains(e.target as Node)) setOpen(false) }
    document.addEventListener('mousedown', onDoc)
    return () => document.removeEventListener('mousedown', onDoc)
  }, [])

  const pick = (p: Placemark) => {
    skipNext.current = true
    onChange(p.label); onPick(p); setOpen(false); setItems([])
  }

  const onKey = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (!open || items.length === 0) return
    if (e.key === 'ArrowDown') { e.preventDefault(); setHi(h => (h + 1) % items.length) }
    else if (e.key === 'ArrowUp') { e.preventDefault(); setHi(h => (h - 1 + items.length) % items.length) }
    else if (e.key === 'Enter') { e.preventDefault(); pick(items[hi]) }
    else if (e.key === 'Escape') setOpen(false)
  }

  return (
    <div ref={boxRef} className="relative">
      <input
        id={id} autoFocus={autoFocus}
        className={`input pr-8 ${pinned ? '!border-accent/60' : ''}`}
        value={value} placeholder={placeholder} required autoComplete="off"
        role="combobox" aria-expanded={open} aria-controls={listId} aria-autocomplete="list"
        onChange={e => {
          const t = e.target.value
          typed.current = true
          onChange(t); onPick(null)
          if (t.trim().length < 2) { setItems([]); setOpen(false) }
        }}
        onFocus={() => items.length && setOpen(true)}
        onKeyDown={onKey}
      />
      <span className="absolute right-2.5 top-1/2 -translate-y-1/2 mono text-[11px] pointer-events-none" aria-hidden>
        {loading ? <span className="text-dim">…</span> : pinned ? <span className="text-accent" title={`pinned: ${pinned.lat.toFixed(3)}, ${pinned.lng.toFixed(3)}`}>◉</span> : <span className="text-dim">◎</span>}
      </span>
      {open && items.length > 0 && (
        <ul id={listId} role="listbox" className="absolute z-20 mt-1 w-full panel shadow-xl shadow-black/40 overflow-hidden">
          {items.map((p, i) => (
            <li key={p.label + i} role="option" aria-selected={i === hi}
              className={`px-3 py-2 text-sm cursor-pointer flex items-center justify-between gap-3 ${i === hi ? 'bg-panel-2 text-ink' : 'text-muted'}`}
              onMouseEnter={() => setHi(i)} onMouseDown={e => { e.preventDefault(); pick(p) }}>
              <span><span className="text-ink">{p.name}</span>{p.label !== p.name && <span className="text-muted">, {p.label.slice(p.name.length + 2)}</span>}</span>
              <span className="mono text-[10px] text-dim">{p.source === 'gazetteer' ? 'popular' : p.country}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
