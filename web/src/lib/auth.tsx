import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { api, token, type User } from './api'

type AuthState = {
  user: User | null
  loading: boolean
  login: (email: string, password: string) => Promise<void>
  register: (email: string, name: string, password: string) => Promise<void>
  logout: () => void
  setUser: (u: User) => void
}

const Ctx = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(!!token.get())

  useEffect(() => {
    if (!token.get()) return
    api.me().then(setUser).catch(() => token.clear()).finally(() => setLoading(false))
  }, [])

  const login = async (email: string, password: string) => {
    const r = await api.login(email, password)
    token.set(r.token)
    setUser(r.user)
  }
  const register = async (email: string, name: string, password: string) => {
    const r = await api.register(email, name, password)
    token.set(r.token)
    setUser(r.user)
  }
  const logout = () => { token.clear(); setUser(null) }

  return <Ctx.Provider value={{ user, loading, login, register, logout, setUser }}>{children}</Ctx.Provider>
}

export function useAuth() {
  const v = useContext(Ctx)
  if (!v) throw new Error('useAuth outside AuthProvider')
  return v
}
