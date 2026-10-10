import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, usd, type Payment } from '../lib/api'
import DataWarning from '../components/DataWarning'
import { isMockPayment } from '../lib/dataWarnings'

const statusColor: Record<string, string> = { settled: 'text-accent', pending: 'text-amber', failed: 'text-rose' }

export default function Payments() {
  const [rows, setRows] = useState<Payment[] | null>(null)
  const [err, setErr] = useState('')
  const [live, setLive] = useState<boolean | null>(null)
  useEffect(() => {
    api.payments().then(setRows).catch(e => setErr((e as Error).message))
    api.health().then(h => setLive(h.payments)).catch(() => setLive(null))
  }, [])

  const total = (rows ?? []).filter(r => r.status === 'settled').reduce((s, r) => s + r.amount_usd, 0)

  return (
    <div className="grid gap-6">
      <div className="flex items-end justify-between">
        <div>
          <div className="mono text-xs text-muted prompt">cat ~/payments.log</div>
          <h1 className="mono text-2xl font-bold mt-1">payments</h1>
        </div>
        <div className="text-right">
          <div className="label">settled total</div>
          <div className="mono text-2xl text-accent">{usd(total)}</div>
        </div>
      </div>
      <div className="flex flex-wrap gap-2">
        <span className="tag">provider: <b className="text-ink">Jack Henry</b></span>
        {live === true && <span className="tag text-accent">sandbox live</span>}
        {live === false && <span className="tag text-amber">mock mode · set JACKHENRY_BASE_URL</span>}
      </div>
      <DataWarning details={[
        ...(live === false ? ['Payments: mock mode is on. New payments are simulated and no money moves.'] : []),
        ...((rows ?? []).some(r => isMockPayment(r.provider_ref))
          ? ['Payment history: entries marked MOCK are simulated settlements. The settled total counts them.'] : []),
      ]} />
      {err && <div className="mono text-sm text-rose">{err}</div>}
      {rows === null ? <div className="mono text-muted">loading…</div> : rows.length === 0 ? (
        <div className="panel p-8 text-center mono text-muted">no payments yet. book a trip from its page.</div>
      ) : (
        <div className="panel overflow-x-auto">
          <table className="w-full mono text-sm">
            <thead className="text-[11px] uppercase tracking-widest text-muted">
              <tr className="border-b border-line">
                <th className="text-left p-3">when</th><th className="text-left p-3">memo</th><th className="text-left p-3">ref</th><th className="text-left p-3">status</th><th className="text-right p-3">amount</th>
              </tr>
            </thead>
            <tbody>
              {rows.map(r => (
                <tr key={r.id} className="border-b border-line/60 last:border-0">
                  <td className="p-3 text-muted whitespace-nowrap">{new Date(r.created_at).toLocaleString()}</td>
                  <td className="p-3">{r.trip_id ? <Link className="hover:text-accent" to={`/app/trips/${r.trip_id}`}>{r.memo ?? 'trip'}</Link> : (r.memo ?? '—')}</td>
                  <td className="p-3 text-dim">{r.provider_ref ?? '—'}{isMockPayment(r.provider_ref) && <span className="ml-2 text-rose font-bold">MOCK</span>}</td>
                  <td className={`p-3 ${statusColor[r.status] ?? 'text-muted'}`}>{r.status}</td>
                  <td className="p-3 text-right text-accent">{usd(r.amount_usd)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
