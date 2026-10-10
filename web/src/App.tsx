import { Navigate, Route, Routes } from 'react-router-dom'
import { useAuth } from './lib/auth'
import Layout from './components/Layout'
import Landing from './pages/Landing'
import Login from './pages/Login'
import Register from './pages/Register'
import Dashboard from './pages/Dashboard'
import NewTrip from './pages/NewTrip'
import TripView from './pages/TripView'
import Share from './pages/Share'
import Payments from './pages/Payments'

function Private({ children }: { children: React.ReactElement }) {
  const { user, loading } = useAuth()
  if (loading) return <div className="mono text-muted p-8">loading session…</div>
  return user ? children : <Navigate to="/login" replace />
}

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<Landing />} />
        <Route path="/login" element={<Login />} />
        <Route path="/register" element={<Register />} />
        <Route path="/share/:id" element={<Share />} />
        <Route path="/app" element={<Private><Dashboard /></Private>} />
        <Route path="/app/new" element={<Private><NewTrip /></Private>} />
        <Route path="/app/trips/:id" element={<Private><TripView /></Private>} />
        <Route path="/app/payments" element={<Private><Payments /></Private>} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  )
}
