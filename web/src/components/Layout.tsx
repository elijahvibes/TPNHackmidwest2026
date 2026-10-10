import { Link, NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useAuth } from '../lib/auth'

export default function Layout() {
  const { user, logout } = useAuth()
  const nav = useNavigate()
  return (
    <div className="min-h-full flex flex-col">
      <header className="border-b border-line bg-bg/80 backdrop-blur sticky top-0 z-10">
        <div className="max-w-6xl mx-auto px-4 h-14 flex items-center justify-between">
          <Link to="/" className="mono font-bold text-ink flex items-center gap-2">
            <span className="text-accent">~/</span>tpn
          </Link>
          <nav className="mono text-xs sm:text-sm flex items-center gap-3 sm:gap-5">
            {user ? (
              <>
                <NavLink to="/app" className={({ isActive }) => isActive ? 'text-accent' : 'text-muted hover:text-ink'}>trips</NavLink>
                <NavLink to="/app/new" className={({ isActive }) => isActive ? 'text-accent' : 'text-muted hover:text-ink'}>+ new</NavLink>
                <NavLink to="/app/payments" className={({ isActive }) => isActive ? 'text-accent' : 'text-muted hover:text-ink'}>payments</NavLink>
                <NavLink to="/app/profile" className={({ isActive }) => isActive ? 'text-accent' : 'text-dim hover:text-ink'} title="profile">{user.name.split(' ')[0].toLowerCase()}@tpn</NavLink>
                <button className="btn-ghost !py-1" onClick={() => { logout(); nav('/') }}>logout</button>
              </>
            ) : (
              <>
                <NavLink to="/login" className="text-muted hover:text-ink">login</NavLink>
                <Link to="/register" className="btn-primary !py-1">sign up</Link>
              </>
            )}
          </nav>
        </div>
      </header>
      <main className="flex-1 max-w-6xl w-full mx-auto px-4 py-8">
        <Outlet />
      </main>
      <footer className="border-t border-line">
        <div className="max-w-6xl mx-auto px-4 py-4 mono text-[11px] text-dim flex flex-wrap gap-4">
          <span>// HackMidwest 2026</span>
          <span>hosted on Akamai · AI on Akamai · payments by Jack Henry</span>
        </div>
      </footer>
    </div>
  )
}
