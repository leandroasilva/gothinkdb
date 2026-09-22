import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider } from '@/contexts/AuthContext'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { AdminRoute } from '@/components/AdminRoute'
import { Layout } from '@/components/layout'
import { LoginPage } from '@/pages/Login'
import { DashboardPage } from '@/pages/Dashboard'
import { TablesPage } from '@/pages/Tables'
import { TableDetailPage } from '@/pages/TableDetail'
import { ServersPage } from '@/pages/Servers'
import { ExplorerPage } from '@/pages/Explorer'
import { LogsPage } from '@/pages/Logs'
import { UsersPage } from '@/pages/Users'
import { PermissionsPage } from '@/pages/Permissions'

function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          {/* Public routes */}
          <Route path="/login" element={<LoginPage />} />
          
          {/* Protected routes */}
          <Route path="/" element={
            <ProtectedRoute>
              <Layout />
            </ProtectedRoute>
          }>
            <Route index element={<Navigate to="/dashboard" replace />} />
            <Route path="dashboard" element={<DashboardPage />} />
            <Route path="tables" element={<TablesPage />} />
            <Route path="tables/:db/:table" element={<TableDetailPage />} />
            <Route path="servers" element={<ServersPage />} />
            <Route path="explorer" element={<ExplorerPage />} />
            <Route path="logs" element={<LogsPage />} />
          </Route>
          
          {/* Admin-only routes */}
          <Route path="/" element={
            <AdminRoute>
              <Layout />
            </AdminRoute>
          }>
            <Route path="users" element={<UsersPage />} />
            <Route path="permissions" element={<PermissionsPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  )
}

export default App
