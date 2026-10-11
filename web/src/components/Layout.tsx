import { Link, NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useAuth } from '../lib/auth'

const initials = (name: string) => name.split(' ').filter(Boolean).slice(0, 2).map(s => s[0]?.toUpperCase()).join('') || '?'

export default function Layout() {
  const { user, logout } = useAuth()
  const nav = useNavigate()
  const link = ({ isActive }: { isActive: boolean }) => `text-sm transition-colors ${isActive ? 'text-ink font-medium' : 'text-muted hover:text-ink'}`
  return (
    <div className="min-h-full flex flex-col">
      <header className="border-b border-line bg-bg/90 backdrop-blur sticky top-0 z-10">
        <div className="max-w-6xl mx-auto px-5 h-16 flex items-center justify-between">
          <Link to="/" className="serif text-xl text-ink">TPN</Link>
          <nav className="flex items-center gap-5 sm:gap-7">
            {user ? (
              <>
                <NavLink to="/app/new" className={link}>Plan a trip</NavLink>
                <NavLink to="/app" end className={link}>My trips</NavLink>
                <NavLink to="/app/profile" title={user.name}
                  className={({ isActive }) => `w-8 h-8 rounded-full bg-panel border text-[12px] font-medium flex items-center justify-center transition-colors ${isActive ? 'border-ink text-ink' : 'border-line text-muted hover:border-ink hover:text-ink'}`}>
                  {initials(user.name)}
                </NavLink>
                <button className="text-sm text-muted hover:text-ink" onClick={() => { logout(); nav('/') }}>Log out</button>
              </>
            ) : (
              <>
                <NavLink to="/" end className={link}>Plan a trip</NavLink>
                <NavLink to="/login" className={link}>Log in</NavLink>
                <Link to="/register" className="btn-primary !py-1.5">Sign up</Link>
              </>
            )}
          </nav>
        </div>
      </header>
      <main className="flex-1 max-w-6xl w-full mx-auto px-5 py-10">
        <Outlet />
      </main>
      <footer className="border-t border-line">
        <div className="max-w-6xl mx-auto px-5 py-5 text-[12px] text-dim flex flex-wrap gap-x-6 gap-y-1">
          <span>TPN · HackMidwest 2026</span>
          <span>Hosted on Akamai · AI on Akamai</span>
        </div>
      </footer>
    </div>
  )
}
