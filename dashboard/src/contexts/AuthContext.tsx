import { createContext, useContext, useState, useEffect, ReactNode } from 'react'
import { apiService, User, Permission } from '@/services/api'

interface AuthContextType {
  user: User | null
  permissions: Permission[]
  token: string | null
  isLoading: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => void
  isAdmin: boolean
  hasDatabaseAccess: (database: string, access: 'read' | 'write' | 'create' | 'drop') => boolean
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [permissions, setPermissions] = useState<Permission[]>([])
  const [token, setToken] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    // Check for existing token on mount
    const storedToken = localStorage.getItem('auth_token')
    if (storedToken) {
      apiService.setToken(storedToken)
      setToken(storedToken)
      
      // Fetch user info
      apiService.me()
        .then((data) => {
          setUser(data.user)
          setPermissions(data.permissions)
        })
        .catch(() => {
          // Token invalid, clear it
          localStorage.removeItem('auth_token')
          apiService.setToken(null)
          setToken(null)
        })
        .finally(() => {
          setIsLoading(false)
        })
    } else {
      setIsLoading(false)
    }
  }, [])

  const login = async (username: string, password: string) => {
    const data = await apiService.login(username, password)
    localStorage.setItem('auth_token', data.token)
    setToken(data.token)
    setUser(data.user)
    
    // Fetch permissions
    const meData = await apiService.me()
    setPermissions(meData.permissions)
  }

  const logout = () => {
    localStorage.removeItem('auth_token')
    apiService.setToken(null)
    setToken(null)
    setUser(null)
    setPermissions([])
  }

  const isAdmin = user?.role === 'admin'

  const hasDatabaseAccess = (database: string, access: 'read' | 'write' | 'create' | 'drop'): boolean => {
    // Admin has full access
    if (isAdmin) return true

    // Check permissions
    const perm = permissions.find(p => p.database === database)
    if (!perm) return false

    switch (access) {
      case 'read': return perm.can_read
      case 'write': return perm.can_write
      case 'create': return perm.can_create
      case 'drop': return perm.can_drop
      default: return false
    }
  }

  return (
    <AuthContext.Provider
      value={{
        user,
        permissions,
        token,
        isLoading,
        login,
        logout,
        isAdmin,
        hasDatabaseAccess,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}
