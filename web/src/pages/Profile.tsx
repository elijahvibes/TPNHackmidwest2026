import { useState } from 'react'
import { api } from '../lib/api'
import { useAuth } from '../lib/auth'
import Hero from '../components/Hero'
import Icon from '../components/Icon'

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
    if (!navigator.geolocation) { setMsg('Geolocation is not supported in this browser'); return }
    setMsg('Locating…')
    navigator.geolocation.getCurrentPosition(
      p => { setLoc({ lat: p.coords.latitude, lng: p.coords.longitude }); setMsg(`Got ${p.coords.latitude.toFixed(3)}, ${p.coords.longitude.toFixed(3)}`) },
      () => setMsg('Location permission denied'),
      { timeout: 8000 },
    )
  }

  const save = async (e: React.FormEvent) => {
    e.preventDefault(); setBusy(true); setMsg('')
    try {
      const u = await api.updateMe({ home_city: city || null, home_lat: loc?.lat ?? null, home_lng: loc?.lng ?? null })
      setUser(u); setMsg('Saved')
    } catch (ex) { setMsg((ex as Error).message) } finally { setBusy(false) }
  }

  return (
    <form onSubmit={save} className="grid gap-8 max-w-xl">
      <Hero eyebrow="Account" title="Your profile" lede="Your home city prefills the starting point of every new trip." />
      <div className="panel p-6 grid gap-5">
        <div className="grid sm:grid-cols-2 gap-4">
          <div><div className="label">Name</div><div className="mt-1 text-[15px]">{user?.name}</div></div>
          <div><div className="label">Email</div><div className="mt-1 text-[15px] text-muted">{user?.email}</div></div>
        </div>
        <div>
          <label htmlFor="home" className="label">Home city</label>
          <input id="home" className="input mt-1.5" value={city} onChange={e => setCity(e.target.value)} placeholder="Kansas City" />
        </div>
        <div>
          <div className="flex items-center justify-between">
            <div className="label">Home coordinates</div>
            <button type="button" onClick={locate} className="text-[12px] text-cyan hover:underline flex items-center gap-1"><Icon name="locate" size={13} />Use my location</button>
          </div>
          <div className="text-sm mt-1 text-muted tabular-nums">{loc ? `${loc.lat.toFixed(4)}, ${loc.lng.toFixed(4)}` : 'Not set'}</div>
        </div>
        <div className="flex items-center justify-between pt-1">
          <span className="text-[13px] text-muted">{msg}</span>
          <button className="btn-primary" disabled={busy}>{busy ? 'Saving…' : 'Save'}</button>
        </div>
      </div>
    </form>
  )
}
