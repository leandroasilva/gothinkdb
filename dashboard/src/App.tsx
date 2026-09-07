import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { Layout } from '@/components/layout'
import { DashboardPage } from '@/pages/Dashboard'
import { TablesPage } from '@/pages/Tables'
import { TableDetailPage } from '@/pages/TableDetail'
import { ServersPage } from '@/pages/Servers'
import { ExplorerPage } from '@/pages/Explorer'
import { LogsPage } from '@/pages/Logs'

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<Navigate to="/dashboard" replace />} />
          <Route path="dashboard" element={<DashboardPage />} />
          <Route path="tables" element={<TablesPage />} />
          <Route path="tables/:db/:table" element={<TableDetailPage />} />
          <Route path="servers" element={<ServersPage />} />
          <Route path="explorer" element={<ExplorerPage />} />
          <Route path="logs" element={<LogsPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}

export default App
