import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../lib/auth'

export default function Register() {
  const { register } = useAuth()
  const nav = useNavigate()
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault(); setBusy(true); setErr('')
    try { await register(email, name, password); nav('/app') } catch (ex) { setErr((ex as Error).message) } finally { setBusy(false) }
  }

  return (
    <form onSubmit={submit} className="panel p-8 max-w-md mx-auto mt-8 grid gap-5">
      <div>
        <div className="eyebrow">Save your trips</div>
        <h1 className="text-[32px] mt-1">Create an account</h1>
      </div>
      <div><label htmlFor="name" className="label">Name</label><input id="name" className="input mt-1.5" value={name} onChange={e => setName(e.target.value)} required autoFocus /></div>
      <div><label htmlFor="email" className="label">Email</label><input id="email" className="input mt-1.5" type="email" value={email} onChange={e => setEmail(e.target.value)} required /></div>
      <div><label htmlFor="password" className="label">Password <span className="text-muted font-normal">(8 or more characters)</span></label><input id="password" className="input mt-1.5" type="password" minLength={8} value={password} onChange={e => setPassword(e.target.value)} required /></div>
      {err && <div className="text-sm text-rose">{err}</div>}
      <button className="btn-primary !py-2.5" disabled={busy}>{busy ? 'Creating…' : 'Sign up'}</button>
      <div className="text-[13px] text-muted">Have an account? <Link to="/login" className="text-accent underline underline-offset-2">Log in</Link></div>
    </form>
  )
}
