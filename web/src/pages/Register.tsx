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
    <form onSubmit={submit} className="panel p-6 max-w-sm mx-auto mt-10 grid gap-4">
      <h1 className="mono prompt font-bold">create account</h1>
      <div><label className="label">name</label><input className="input mt-1" value={name} onChange={e => setName(e.target.value)} required autoFocus /></div>
      <div><label className="label">email</label><input className="input mt-1" type="email" value={email} onChange={e => setEmail(e.target.value)} required /></div>
      <div><label className="label">password (8+)</label><input className="input mt-1" type="password" minLength={8} value={password} onChange={e => setPassword(e.target.value)} required /></div>
      {err && <div className="mono text-sm text-rose">{err}</div>}
      <button className="btn-primary justify-center" disabled={busy}>{busy ? '…' : 'sign up'}</button>
      <div className="mono text-xs text-muted">have an account? <Link to="/login" className="text-accent">login</Link></div>
    </form>
  )
}
