import { useState } from 'react'
import { api } from '../lib/api'
import { useAuth } from '../lib/auth'

/** Home base: saved once, then prefilled as the origin of every new trip. */
export default function Profile() {
  const { user, setUser } = useAuth()
  const [city, setCity] = useState(user?.home_city ?? '')
  const [loc, setLoc] = useState<{ lat: number; lng: number } | null>(
    user?.home_lat != null && user?.home_lng != null ? { lat: user.home_lat, lng: user.home_lng } : null,
  )
  const [msg, setMsg] = useState('')
  const [busy, setBusy] = useState(false)

  const locate = () => {
    if (!navigator.geolocation) { setMsg('geolocation unsupported in this browser'); return }
    setMsg('locating…')
    navigator.geolocation.getCurrentPosition(
      p => { setLoc({ lat: p.coords.latitude, lng: p.coords.longitude }); setMsg(`got ${p.coords.latitude.toFixed(3)}, ${p.coords.longitude.toFixed(3)}`) },
      () => setMsg('location permission denied'),
      { timeout: 8000 },
    )
  }

  const save = async (e: React.FormEvent) => {
    e.preventDefault(); setBusy(true); setMsg('')
    try {
      const u = await api.updateMe({ home_city: city || null, home_lat: loc?.lat ?? null, home_lng: loc?.lng ?? null })
      setUser(u); setMsg('saved ✓')
    } catch (ex) { setMsg((ex as Error).message) } finally { setBusy(false) }
  }

  return (
    <form onSubmit={save} className="grid gap-6 max-w-lg">
      <div>
        <div className="mono text-xs text-muted prompt">cat ~/.tpnrc</div>
        <h1 className="mono text-2xl font-bold mt-1">profile</h1>
      </div>
      <div className="panel p-5 grid gap-4">
        <div className="grid sm:grid-cols-2 gap-4">
          <div><label className="label">name</label><div className="mono mt-1">{user?.name}</div></div>
          <div><label className="label">email</label><div className="mono mt-1 text-muted">{user?.email}</div></div>
        </div>
        <div>
          <label className="label">home city (prefills "from" on new trips)</label>
          <input className="input mt-1" value={city} onChange={e => setCity(e.target.value)} placeholder="Kansas City" />
        </div>
        <div>
          <div className="flex items-center justify-between">
            <label className="label">home coordinates</label>
            <button type="button" onClick={locate} className="mono text-[11px] text-cyan hover:underline">◎ use my location</button>
          </div>
          <div className="mono text-sm mt-1 text-muted">{loc ? `${loc.lat.toFixed(4)}, ${loc.lng.toFixed(4)}` : 'not set'}</div>
        </div>
        <div className="flex items-center justify-between pt-1">
          <span className="mono text-xs text-muted">{msg}</span>
          <button className="btn-primary" disabled={busy}>{busy ? 'saving…' : 'save'}</button>
        </div>
      </div>
    </form>
  )
}
