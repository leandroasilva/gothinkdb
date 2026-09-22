import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import {
  Database,
  Server,
  Table2,
  Activity,
  HardDrive,
  Cpu,
  Zap,
  AlertCircle,
} from 'lucide-react'
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'
import { useAuth } from '@/contexts/AuthContext'

export function DashboardPage() {
  const navigate = useNavigate()
  const { token } = useAuth()

  const fetchJSON = async (url: string) => {
    const headers: Record<string, string> = {}
    if (token) headers['Authorization'] = `Bearer ${token}`
    const res = await fetch(url, { headers })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    return res.json()
  }

  const { data: health } = useQuery({
    queryKey: ['health', token],
    queryFn: () => fetchJSON('/api/health'),
    refetchInterval: 5000,
  })

  const { data: serverInfo } = useQuery({
    queryKey: ['serverInfo', token],
    queryFn: () => fetchJSON('/api/server/info'),
    refetchInterval: 5000,
  })

  const { data: serverStats } = useQuery({
    queryKey: ['serverStats', token],
    queryFn: () => fetchJSON('/api/server/stats'),
    refetchInterval: 2000,
  })

  const { data: databases } = useQuery({
    queryKey: ['databases', token],
    queryFn: () => fetchJSON('/api/databases'),
    refetchInterval: 5000,
  })

  const { data: clusterStatus } = useQuery({
    queryKey: ['clusterStatus'],
    queryFn: () => fetchJSON('/api/cluster/status'),
    refetchInterval: 5000,
  })

  const { data: metrics } = useQuery({
    queryKey: ['metrics'],
    queryFn: () => fetchJSON('/api/metrics'),
    refetchInterval: 1000,
  })

  // Calculate total tables across all databases
  const dbList = Array.isArray(databases) ? databases : []
  const dbCount = dbList.length

  const { data: allTables } = useQuery({
    queryKey: ['allTables'],
    queryFn: async () => {
      const tables: string[] = []
      for (const db of dbList) {
        const dbTables = await fetchJSON(`/api/tables?db=${db}`)
        if (Array.isArray(dbTables)) {
          tables.push(...dbTables)
        }
      }
      return tables
    },
    enabled: dbList.length > 0,
    refetchInterval: 5000,
  })

  const tableCount = Array.isArray(allTables) ? allTables.length : 0

  // Chart data from real metrics
  const chartData = metrics?.history || []

  const memberCount = clusterStatus?.members
    ? Object.keys(clusterStatus.members).length
    : 0

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold text-foreground">Dashboard</h1>
        <p className="text-muted-foreground">
          Overview of your GoThinkDB cluster
        </p>
      </div>

      {/* Status Cards */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Server Status</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {health?.status || 'Unknown'}
            </div>
            <p className="text-xs text-muted-foreground">
              {serverInfo?.version || 'v0.0.0'} - {serverInfo?.server || 'unknown'}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Databases</CardTitle>
            <Database className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{dbCount}</div>
            <p className="text-xs text-muted-foreground">
              Active databases
            </p>
          </CardContent>
        </Card>

        <Card className="cursor-pointer hover:border-primary/50 transition-colors" onClick={() => navigate('/tables')}>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Tables</CardTitle>
            <Table2 className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{tableCount}</div>
            <p className="text-xs text-muted-foreground">
              {dbCount > 0 ? `${dbCount} database${dbCount > 1 ? 's' : ''}` : 'No databases'}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Cluster</CardTitle>
            <Server className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{memberCount}</div>
            <p className="text-xs text-muted-foreground">
              {memberCount <= 1 ? 'Standalone' : 'Cluster'} mode
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Performance Charts */}
      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Zap size={18} className="text-yellow-500" />
              Queries/sec
            </CardTitle>
          </CardHeader>
          <CardContent>
            {chartData.length === 0 ? (
              <div className="h-[200px] flex items-center justify-center text-sm text-muted-foreground">
                Waiting for query data...
              </div>
            ) : (
              <ResponsiveContainer width="100%" height={200}>
                <AreaChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                  <XAxis dataKey="time" className="text-xs" />
                  <YAxis className="text-xs" />
                  <Tooltip />
                  <Area
                    type="monotone"
                    dataKey="queries"
                    stroke="hsl(var(--primary))"
                    fill="hsl(var(--primary))"
                    fillOpacity={0.2}
                  />
                </AreaChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Cpu size={18} className="text-blue-500" />
              Latency (ms)
            </CardTitle>
          </CardHeader>
          <CardContent>
            {chartData.length === 0 ? (
              <div className="h-[200px] flex items-center justify-center text-sm text-muted-foreground">
                Waiting for latency data...
              </div>
            ) : (
              <ResponsiveContainer width="100%" height={200}>
                <AreaChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                  <XAxis dataKey="time" className="text-xs" />
                  <YAxis className="text-xs" />
                  <Tooltip />
                  <Area
                    type="monotone"
                    dataKey="latency"
                    stroke="hsl(var(--chart-2))"
                    fill="hsl(var(--chart-2))"
                    fillOpacity={0.2}
                  />
                </AreaChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Server Stats & Issues */}
      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <HardDrive size={18} />
              Server Statistics
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Queries Total</span>
                <span className="text-sm font-medium">{metrics?.queries_total ?? serverStats?.queries_total ?? 0}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Queries/sec</span>
                <span className="text-sm font-medium">{metrics?.queries_per_sec ?? serverStats?.queries_per_second ?? 0}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Connections</span>
                <span className="text-sm font-medium">{metrics?.connections ?? serverStats?.connections ?? 0}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Memory Used</span>
                <span className="text-sm font-medium">{metrics?.memory_used_mb ?? serverStats?.memory_used ?? 0} MB</span>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <AlertCircle size={18} className="text-yellow-500" />
              Issues & Alerts
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {memberCount <= 1 && (
                <div className="flex items-start gap-3 rounded-lg border border-yellow-500/20 bg-yellow-500/5 p-3">
                  <AlertCircle size={16} className="mt-0.5 text-yellow-500" />
                  <div>
                    <p className="text-sm font-medium text-yellow-700 dark:text-yellow-400">
                      Running in standalone mode
                    </p>
                    <p className="text-xs text-muted-foreground">
                      Add more nodes for high availability
                    </p>
                  </div>
                </div>
              )}
              {memberCount <= 1 ? null : (
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Badge variant="outline" className="bg-green-500/10 text-green-600 border-green-500/20">
                    All clear
                  </Badge>
                  <span>No issues detected</span>
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
