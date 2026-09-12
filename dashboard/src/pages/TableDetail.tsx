import { useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Table2, Database, Key, BarChart3, ArrowLeft } from 'lucide-react'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'

async function fetchJSON(url: string) {
  const token = localStorage.getItem('auth_token')
  const headers: Record<string, string> = {}
  if (token) headers['Authorization'] = `Bearer ${token}`
  const res = await fetch(url, { headers })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  return res.json()
}

export function TableDetailPage() {
  const { db, table } = useParams<{ db: string; table: string }>()
  const navigate = useNavigate()
  const [activeTab, setActiveTab] = useState<'overview' | 'data' | 'indexes'>('overview')

  const { data: tableData } = useQuery({
    queryKey: ['table', db, table],
    queryFn: () => fetchJSON(`/api/tables/${table}?db=${db}`),
  })

  const { data: documents } = useQuery({
    queryKey: ['tableDocs', db, table],
    queryFn: () => fetchJSON(`/api/tables/${db}/${table}/docs`),
    enabled: activeTab === 'data',
  })

  const { data: indexes } = useQuery({
    queryKey: ['tableIndexes', db, table],
    queryFn: () => fetchJSON(`/api/tables/${db}/${table}/indexes`),
    enabled: activeTab === 'indexes',
  })

  const tabs = [
    { id: 'overview' as const, label: 'Overview', icon: BarChart3 },
    { id: 'data' as const, label: 'Data', icon: Table2 },
    { id: 'indexes' as const, label: 'Indexes', icon: Key },
  ]

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" onClick={() => navigate('/tables')}>
          <ArrowLeft size={20} />
        </Button>
        <div>
          <h1 className="text-3xl font-bold text-foreground flex items-center gap-2">
            <Table2 className="text-primary" />
            {table}
          </h1>
          <p className="text-muted-foreground flex items-center gap-2">
            <Database size={14} />
            {db}
          </p>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`flex items-center gap-2 px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              activeTab === tab.id
                ? 'border-primary text-primary'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            }`}
          >
            <tab.icon size={16} />
            {tab.label}
          </button>
        ))}
      </div>

      {/* Overview Tab */}
      {activeTab === 'overview' && (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm text-muted-foreground">Documents</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-2xl font-bold">
                {tableData?.doc_count ?? 'N/A'}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm text-muted-foreground">Indexes</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-2xl font-bold">
                {tableData?.indexes?.length ?? 0}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm text-muted-foreground">Shards</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-2xl font-bold">
                {tableData?.shards ?? 1}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm text-muted-foreground">Replicas</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-2xl font-bold">
                {tableData?.replicas ?? 1}
              </p>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Data Tab */}
      {activeTab === 'data' && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Documents</CardTitle>
          </CardHeader>
          <CardContent>
            {Array.isArray(documents) && documents.length > 0 ? (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b">
                      {Object.keys(documents[0] || {}).map((key) => (
                        <th key={key} className="text-left py-2 px-3 font-medium text-muted-foreground">
                          {key}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {documents.map((doc: Record<string, unknown>, i: number) => (
                      <tr key={i} className="border-b hover:bg-accent/50">
                        {Object.values(doc).map((val, j) => (
                          <td key={j} className="py-2 px-3 text-muted-foreground max-w-[200px] truncate">
                            {typeof val === 'object' ? JSON.stringify(val) : String(val)}
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">No documents found.</p>
            )}
          </CardContent>
        </Card>
      )}

      {/* Indexes Tab */}
      {activeTab === 'indexes' && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg flex items-center gap-2">
              <Key size={18} />
              Indexes
              <Badge variant="secondary">
                {Array.isArray(indexes) ? indexes.length : 0}
              </Badge>
            </CardTitle>
          </CardHeader>
          <CardContent>
            {Array.isArray(indexes) && indexes.length > 0 ? (
              <div className="space-y-2">
                {indexes.map((idx: string | { name: string }) => {
                  const name = typeof idx === 'string' ? idx : idx.name
                  return (
                    <div key={name} className="flex items-center gap-3 rounded-lg border p-3">
                      <Key size={16} className="text-primary" />
                      <span className="font-medium">{name}</span>
                      {name === 'id' && (
                        <Badge variant="outline">Primary</Badge>
                      )}
                    </div>
                  )
                })}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">No indexes found.</p>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  )
}
