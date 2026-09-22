import { Outlet, NavLink, useLocation, useNavigate } from 'react-router-dom'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useAuth } from '@/contexts/AuthContext'
import {
  LayoutDashboard,
  Table2,
  Server,
  Terminal,
  ScrollText,
  Database,
  Menu,
  X,
  Users,
  Shield,
  LogOut,
} from 'lucide-react'
import { useState, useEffect } from 'react'

export function Layout() {
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [healthStatus, setHealthStatus] = useState<'healthy' | 'unhealthy' | 'loading'>('loading')
  const location = useLocation()
  const navigate = useNavigate()
  const { user, isAdmin, logout } = useAuth()

  // Filter nav items based on user role
  const baseNavItems = [
    { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
    { to: '/tables', label: 'Tables', icon: Table2 },
    { to: '/servers', label: 'Servers', icon: Server },
    { to: '/explorer', label: 'Data Explorer', icon: Terminal },
    { to: '/logs', label: 'Logs', icon: ScrollText },
  ]

  const adminNavItems = [
    { to: '/users', label: 'Users', icon: Users },
    { to: '/permissions', label: 'Permissions', icon: Shield },
  ]

  const navItems = isAdmin ? [...baseNavItems, ...adminNavItems] : baseNavItems

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  useEffect(() => {
    const checkHealth = async () => {
      try {
        const res = await fetch('/api/health')
        if (res.ok) {
          setHealthStatus('healthy')
        } else {
          setHealthStatus('unhealthy')
        }
      } catch {
        setHealthStatus('unhealthy')
      }
    }
    checkHealth()
    const interval = setInterval(checkHealth, 10000)
    return () => clearInterval(interval)
  }, [])

  useEffect(() => {
    setSidebarOpen(false)
  }, [location.pathname])

  return (
    <div className="min-h-screen bg-background">
      {/* Mobile menu button */}
      <div className="lg:hidden fixed top-0 left-0 right-0 z-50 flex items-center justify-between border-b bg-background px-4 py-3">
        <button onClick={() => setSidebarOpen(!sidebarOpen)} className="text-foreground">
          {sidebarOpen ? <X size={24} /> : <Menu size={24} />}
        </button>
        <div className="flex items-center gap-2">
          <Database size={20} className="text-primary" />
          <span className="font-bold text-foreground">GoThinkDB</span>
        </div>
        <Badge variant={healthStatus === 'healthy' ? 'default' : 'destructive'}>
          {healthStatus === 'loading' ? '...' : healthStatus}
        </Badge>
      </div>

      {/* Sidebar overlay for mobile */}
      {sidebarOpen && (
        <div
          className="lg:hidden fixed inset-0 z-40 bg-black/50"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      {/* Sidebar */}
      <aside
        className={cn(
          'fixed top-0 left-0 z-40 h-screen w-64 border-r bg-card transition-transform lg:translate-x-0',
          sidebarOpen ? 'translate-x-0' : '-translate-x-full'
        )}
      >
        <div className="flex h-16 items-center gap-2 border-b px-6">
          <Database size={24} className="text-primary" />
          <div>
            <h1 className="text-lg font-bold text-foreground">GoThinkDB</h1>
            <p className="text-xs text-muted-foreground">Admin Console</p>
          </div>
        </div>

        <nav className="flex flex-col gap-1 p-4">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-primary text-primary-foreground'
                    : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
                )
              }
            >
              <item.icon size={18} />
              {item.label}
            </NavLink>
          ))}
        </nav>

        <div className="absolute bottom-0 left-0 right-0 border-t p-4 space-y-3">
          {/* User info */}
          {user && (
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <div className="rounded-full bg-primary/10 p-1.5">
                  <Users className="h-3 w-3 text-primary" />
                </div>
                <div>
                  <p className="text-xs font-medium">{user.username}</p>
                  <Badge variant={user.role === 'admin' ? 'default' : 'secondary'} className="text-[9px] px-1 py-0">
                    {user.role}
                  </Badge>
                </div>
              </div>
              <Button
                variant="ghost"
                size="icon"
                onClick={handleLogout}
                className="h-8 w-8 text-muted-foreground hover:text-destructive"
                title="Logout"
              >
                <LogOut className="h-4 w-4" />
              </Button>
            </div>
          )}
          
          {/* Health status */}
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <div
                className={cn(
                  'h-2 w-2 rounded-full',
                  healthStatus === 'healthy' ? 'bg-green-500' : 'bg-red-500'
                )}
              />
              <span className="text-xs text-muted-foreground">
                {healthStatus === 'healthy' ? 'Connected' : 'Disconnected'}
              </span>
            </div>
            <Badge variant={healthStatus === 'healthy' ? 'default' : 'destructive'} className="text-[10px]">
              {healthStatus === 'loading' ? '...' : healthStatus}
            </Badge>
          </div>
        </div>
      </aside>

      {/* Main content */}
      <main className="lg:ml-64 min-h-screen pt-16 lg:pt-0">
        <div className="p-6 lg:p-8">
          <Outlet />
        </div>
      </main>
    </div>
  )
}
