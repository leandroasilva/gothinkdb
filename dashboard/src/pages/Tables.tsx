import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Table2, Plus, Trash2, Database, ChevronRight } from 'lucide-react'
import { useAuth } from '@/contexts/AuthContext'

function makeFetchJSON(token: string | null) {
  return async (url: string, options?: RequestInit) => {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    }
    if (token) headers['Authorization'] = `Bearer ${token}`

    const res = await fetch(url, {
      ...options,
      headers: { ...headers, ...(options?.headers as Record<string, string>) },
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    return res.json()
  }
}

export function TablesPage() {
  const { token } = useAuth()
  const fetchJSON = makeFetchJSON(token)
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [selectedDb, setSelectedDb] = useState('')
  const [showCreateDb, setShowCreateDb] = useState(false)
  const [showCreateTable, setShowCreateTable] = useState(false)
  const [newDbName, setNewDbName] = useState('')
  const [newTableName, setNewTableName] = useState('')

  const { data: databases } = useQuery({
    queryKey: ['databases'],
    queryFn: () => fetchJSON('/api/databases'),
  })

  const dbList = Array.isArray(databases) ? databases : []

  // Auto-select first database when list loads
  const effectiveDb = selectedDb || (dbList.length > 0 ? dbList[0] : '')

  const { data: tables, isLoading } = useQuery({
    queryKey: ['tables', effectiveDb],
    queryFn: () => fetchJSON(`/api/tables?db=${effectiveDb}`),
    enabled: effectiveDb !== '',
  })

  const createDbMutation = useMutation({
    mutationFn: (name: string) =>
      fetchJSON('/api/databases', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name }),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['databases'] })
      setShowCreateDb(false)
      setNewDbName('')
    },
  })

  const createTableMutation = useMutation({
    mutationFn: (name: string) =>
      fetchJSON(`/api/tables/${name}?db=${effectiveDb}`, { method: 'POST' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tables', effectiveDb] })
      setShowCreateTable(false)
      setNewTableName('')
    },
  })

  const dropTableMutation = useMutation({
    mutationFn: (name: string) =>
      fetchJSON(`/api/tables/${name}?db=${effectiveDb}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tables', effectiveDb] })
    },
  })

  const dropDbMutation = useMutation({
    mutationFn: (name: string) =>
      fetchJSON(`/api/databases/${name}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['databases'] })
    },
  })

  const tableList = Array.isArray(tables) ? tables : []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-foreground">Tables</h1>
          <p className="text-muted-foreground">
            Manage databases and tables
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => setShowCreateDb(true)}>
            <Database size={16} className="mr-2" />
            New Database
          </Button>
          <Button onClick={() => setShowCreateTable(true)}>
            <Plus size={16} className="mr-2" />
            New Table
          </Button>
        </div>
      </div>

      {/* Database selector */}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Database</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap gap-2">
            {dbList.length === 0 ? (
              <p className="text-sm text-muted-foreground">No databases yet. Create one to get started.</p>
            ) : dbList.map((db: string) => (
              <Button
                key={db}
                variant={effectiveDb === db ? 'default' : 'outline'}
                size="sm"
                onClick={() => setSelectedDb(db)}
              >
                {db}
              </Button>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Tables list */}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg flex items-center gap-2">
            <Table2 size={18} />
            {effectiveDb ? `Tables in "${effectiveDb}"` : 'Select a database'}
            <Badge variant="secondary">{tableList.length}</Badge>
          </CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <p className="text-sm text-muted-foreground">Loading...</p>
          ) : tableList.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              No tables found. Create one to get started.
            </p>
          ) : (
            <div className="space-y-2">
              {tableList.map((table: string) => (
                <div
                  key={table}
                  className="flex items-center justify-between rounded-lg border p-4 hover:bg-accent/50 transition-colors cursor-pointer"
                  onClick={() => navigate(`/tables/${effectiveDb}/${table}`)}
                >
                  <div className="flex items-center gap-3">
                    <Table2 size={18} className="text-primary" />
                    <div>
                      <p className="font-medium">{table}</p>
                      <p className="text-xs text-muted-foreground">
                        Database: {effectiveDb}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={(e) => {
                        e.stopPropagation()
                        if (confirm(`Drop table "${table}"?`)) {
                          dropTableMutation.mutate(table)
                        }
                      }}
                    >
                      <Trash2 size={14} className="text-destructive" />
                    </Button>
                    <ChevronRight size={16} className="text-muted-foreground" />
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Create Database Dialog */}
      {showCreateDb && (
        <Card className="border-primary/50">
          <CardHeader>
            <CardTitle className="text-lg">Create Database</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex gap-2">
              <input
                type="text"
                value={newDbName}
                onChange={(e) => setNewDbName(e.target.value)}
                placeholder="Database name"
                className="flex-1 rounded-md border border-input bg-background px-3 py-2 text-sm"
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && newDbName) {
                    createDbMutation.mutate(newDbName)
                  }
                }}
              />
              <Button
                onClick={() => newDbName && createDbMutation.mutate(newDbName)}
                disabled={!newDbName}
              >
                Create
              </Button>
              <Button variant="outline" onClick={() => setShowCreateDb(false)}>
                Cancel
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Create Table Dialog */}
      {showCreateTable && (
        <Card className="border-primary/50">
          <CardHeader>
            <CardTitle className="text-lg">
              Create Table in "{effectiveDb}"
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex gap-2">
              <input
                type="text"
                value={newTableName}
                onChange={(e) => setNewTableName(e.target.value)}
                placeholder="Table name"
                className="flex-1 rounded-md border border-input bg-background px-3 py-2 text-sm"
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && newTableName) {
                    createTableMutation.mutate(newTableName)
                  }
                }}
              />
              <Button
                onClick={() =>
                  newTableName && createTableMutation.mutate(newTableName)
                }
                disabled={!newTableName}
              >
                Create
              </Button>
              <Button variant="outline" onClick={() => setShowCreateTable(false)}>
                Cancel
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Databases section */}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg flex items-center gap-2">
            <Database size={18} />
            All Databases
            <Badge variant="secondary">{dbList.length}</Badge>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            {dbList.map((db: string) => (
              <div
                key={db}
                className="flex items-center justify-between rounded-lg border p-3"
              >
                <div className="flex items-center gap-3">
                  <Database size={16} className="text-primary" />
                  <span className="font-medium">{db}</span>
                </div>
                {db !== 'rethinkdb' && (
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => {
                      if (confirm(`Drop database "${db}"?`)) {
                        dropDbMutation.mutate(db)
                      }
                    }}
                  >
                    <Trash2 size={14} className="text-destructive" />
                  </Button>
                )}
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
