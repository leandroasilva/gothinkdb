import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useAuth } from '@/contexts/AuthContext'
import { AddNodeModal } from '@/components/cluster/AddNodeModal'
import { RemoveNodeModal } from '@/components/cluster/RemoveNodeModal'
import { Server, Activity, HardDrive, Cpu, Clock, CheckCircle, Plus, Trash2, Loader2 } from 'lucide-react'
import { apiService } from '@/services/api'

async function fetchJSON(url: string) {
  const res = await fetch(url)
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  return res.json()
}

interface ClusterMember {
  node_id: string
  address: string
  cluster_port: number
  http_port: number
  driver_port: number
  status: string
  is_leader: boolean
  joined_at: string
  last_seen: string
  data_size_mb: number
  tables_count: number
}

export function ServersPage() {
  const { isAdmin } = useAuth()
  const queryClient = useQueryClient()
  const [showAddNode, setShowAddNode] = useState(false)
  const [nodeToRemove, setNodeToRemove] = useState<ClusterMember | null>(null)

  const { data: serverInfo } = useQuery({
    queryKey: ['serverInfo'],
    queryFn: () => fetchJSON('/api/server/info'),
  })

  const { data: serverStats } = useQuery({
    queryKey: ['serverStats'],
    queryFn: () => fetchJSON('/api/server/stats'),
  })

  const { data: clusterStatus } = useQuery({
    queryKey: ['clusterStatus'],
    queryFn: () => fetchJSON('/api/cluster/status'),
  })

  const { data: clusterMembers, isLoading: isLoadingMembers } = useQuery({
    queryKey: ['clusterMembers'],
    queryFn: () => apiService.getClusterMembers(),
    refetchInterval: 5000, // Refresh every 5 seconds
  })

  const members = Array.isArray(clusterMembers) ? clusterMembers as ClusterMember[] : []
  const isStandalone = members.length === 0

  const handleNodeAdded = () => {
    queryClient.invalidateQueries({ queryKey: ['clusterMembers'] })
    queryClient.invalidateQueries({ queryKey: ['clusterStatus'] })
  }

  const handleNodeRemoved = () => {
    setNodeToRemove(null)
    queryClient.invalidateQueries({ queryKey: ['clusterMembers'] })
    queryClient.invalidateQueries({ queryKey: ['clusterStatus'] })
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-foreground">Servers</h1>
          <p className="text-muted-foreground">
            Monitor and manage cluster servers
          </p>
        </div>
        {isAdmin && (
          <Button onClick={() => setShowAddNode(true)}>
            <Plus className="mr-2 h-4 w-4" />
            Add Node
          </Button>
        )}
      </div>

      {/* Current Server */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-lg">
            <Server size={18} className="text-primary" />
            Current Server
            <Badge className="bg-green-500/10 text-green-600 border-green-500/20">
              <CheckCircle size={12} className="mr-1" />
              Running
            </Badge>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
            <div className="flex items-center gap-3">
              <div className="rounded-lg bg-primary/10 p-2">
                <Server size={20} className="text-primary" />
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Name</p>
                <p className="font-medium">{serverInfo?.server || 'Unknown'}</p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <div className="rounded-lg bg-blue-500/10 p-2">
                <Activity size={20} className="text-blue-500" />
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Version</p>
                <p className="font-medium">{serverInfo?.version || 'N/A'}</p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <div className="rounded-lg bg-green-500/10 p-2">
                <CheckCircle size={20} className="text-green-500" />
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Status</p>
                <p className="font-medium">{serverInfo?.status || 'Unknown'}</p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <div className="rounded-lg bg-yellow-500/10 p-2">
                <Clock size={20} className="text-yellow-500" />
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Uptime</p>
                <p className="font-medium">{serverInfo?.uptime || 'N/A'}</p>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Performance */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Queries Total</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{serverStats?.queries_total || 0}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Queries/sec</CardTitle>
            <Cpu className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{serverStats?.queries_per_second || 0}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Connections</CardTitle>
            <Server className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{serverStats?.connections || 0}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Memory</CardTitle>
            <HardDrive className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{serverStats?.memory_used || 0} MB</div>
          </CardContent>
        </Card>
      </div>

      {/* Cluster Members */}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg flex items-center gap-2">
            <Server size={18} />
            Cluster Members
            <Badge variant="secondary">
              {isStandalone ? 'Standalone' : `${members.length} nodes`}
            </Badge>
          </CardTitle>
        </CardHeader>
        <CardContent>
          {isLoadingMembers ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-8 w-8 animate-spin text-primary" />
            </div>
          ) : isStandalone ? (
            <div className="text-center py-8">
              <Server size={48} className="mx-auto mb-4 text-muted-foreground/50" />
              <p className="text-sm text-muted-foreground">
                Running in standalone mode. Click "Add Node" to add nodes to the cluster.
              </p>
            </div>
          ) : (
            <div className="space-y-2">
              {members.map((member: ClusterMember, i: number) => (
                <div key={member.node_id || i} className="flex items-center justify-between rounded-lg border p-4">
                  <div className="flex items-center gap-3">
                    <div className={`h-2 w-2 rounded-full ${
                      member.status === 'active' ? 'bg-green-500' :
                      member.status === 'draining' ? 'bg-yellow-500' :
                      'bg-red-500'
                    }`} />
                    <div>
                      <p className="font-medium flex items-center gap-2">
                        {member.node_id || `Node ${i + 1}`}
                        {member.is_leader && (
                          <Badge variant="outline" className="text-xs">Leader</Badge>
                        )}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        {member.address}:{member.cluster_port} • {member.status}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-3">
                    <div className="text-right text-xs text-muted-foreground">
                      <p>{member.data_size_mb || 0} MB</p>
                      <p>{member.tables_count || 0} tables</p>
                    </div>
                    <Badge variant="outline">{member.status || 'active'}</Badge>
                    {isAdmin && !member.is_leader && (
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => setNodeToRemove(member)}
                        className="text-destructive hover:text-destructive"
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Cluster Info */}
      {clusterStatus && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Cluster Information</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Cluster ID</span>
                <span className="text-sm font-mono">{clusterStatus.cluster_id || 'N/A'}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Raft Term</span>
                <span className="text-sm font-medium">{clusterStatus.term || 0}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Mode</span>
                <Badge variant="secondary">
                  {isStandalone ? 'Standalone' : 'Cluster'}
                </Badge>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Modals */}
      <AddNodeModal
        isOpen={showAddNode}
        onClose={() => setShowAddNode(false)}
        onSuccess={handleNodeAdded}
      />
      <RemoveNodeModal
        isOpen={!!nodeToRemove}
        onClose={() => setNodeToRemove(null)}
        onSuccess={handleNodeRemoved}
        node={nodeToRemove}
      />
    </div>
  )
}
