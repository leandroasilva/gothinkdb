import { useState, useEffect } from 'react'
import { apiService, User, Permission } from '@/services/api'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Shield, Plus, Trash2, Loader2, Database } from 'lucide-react'

interface UserPermissions {
  user: User
  permissions: Permission[]
}

export function PermissionsPage() {
  const [userPermissions, setUserPermissions] = useState<UserPermissions[]>([])
  const [databases, setDatabases] = useState<string[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [showGrantForm, setShowGrantForm] = useState<string | null>(null)
  const [selectedDb, setSelectedDb] = useState('')
  const [canRead, setCanRead] = useState(true)
  const [canWrite, setCanWrite] = useState(false)
  const [canCreate, setCanCreate] = useState(false)
  const [canDrop, setCanDrop] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    loadData()
  }, [])

  const loadData = async () => {
    try {
      const [users, dbs] = await Promise.all([
        apiService.listUsers(),
        apiService.listDatabases(),
      ])
      setDatabases(dbs)

      // Load permissions for each user
      const permsData: UserPermissions[] = []
      for (const user of users) {
        const perms = await apiService.listUserPermissions(user.id)
        permsData.push({ user, permissions: perms })
      }
      setUserPermissions(permsData)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load data')
    } finally {
      setIsLoading(false)
    }
  }

  const handleGrantPermission = async (userId: string) => {
    if (!selectedDb) return

    try {
      await apiService.grantPermission(userId, selectedDb, canRead, canWrite, canCreate, canDrop)
      setShowGrantForm(null)
      setSelectedDb('')
      setCanRead(true)
      setCanWrite(false)
      setCanCreate(false)
      setCanDrop(false)
      loadData()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to grant permission')
    }
  }

  const handleRevokePermission = async (userId: string, database: string) => {
    if (!confirm(`Revoke all permissions for database "${database}"?`)) return

    try {
      await apiService.revokePermission(userId, database)
      loadData()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to revoke permission')
    }
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">Permissions</h1>
        <p className="text-muted-foreground">Manage database access permissions for users</p>
      </div>

      {error && (
        <div className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {userPermissions.map(({ user, permissions }) => (
        <Card key={user.id}>
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className="rounded-full bg-primary/10 p-2">
                  <Shield className="h-5 w-5 text-primary" />
                </div>
                <div>
                  <CardTitle className="text-lg">{user.username}</CardTitle>
                  <CardDescription>
                    <Badge variant={user.role === 'admin' ? 'default' : 'secondary'} className="mr-2">
                      {user.role}
                    </Badge>
                    {user.role === 'admin' ? 'Full access to all databases' : `${permissions.length} database(s)`}
                  </CardDescription>
                </div>
              </div>
              {user.role !== 'admin' && (
                <Button
                  size="sm"
                  onClick={() => setShowGrantForm(showGrantForm === user.id ? null : user.id)}
                >
                  <Plus className="mr-2 h-4 w-4" />
                  Grant Permission
                </Button>
              )}
            </div>
          </CardHeader>
          <CardContent>
            {showGrantForm === user.id && (
              <div className="mb-4 rounded-lg border p-4 space-y-3">
                <div className="space-y-2">
                  <label className="text-sm font-medium">Database</label>
                  <select
                    value={selectedDb}
                    onChange={(e) => setSelectedDb(e.target.value)}
                    className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                  >
                    <option value="">Select a database</option>
                    {databases.map((db) => (
                      <option key={db} value={db}>
                        {db}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <label className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={canRead}
                      onChange={(e) => setCanRead(e.target.checked)}
                      className="rounded"
                    />
                    <span className="text-sm">Read</span>
                  </label>
                  <label className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={canWrite}
                      onChange={(e) => setCanWrite(e.target.checked)}
                      className="rounded"
                    />
                    <span className="text-sm">Write</span>
                  </label>
                  <label className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={canCreate}
                      onChange={(e) => setCanCreate(e.target.checked)}
                      className="rounded"
                    />
                    <span className="text-sm">Create Tables</span>
                  </label>
                  <label className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={canDrop}
                      onChange={(e) => setCanDrop(e.target.checked)}
                      className="rounded"
                    />
                    <span className="text-sm">Drop Tables</span>
                  </label>
                </div>
                <div className="flex gap-2">
                  <Button size="sm" onClick={() => handleGrantPermission(user.id)}>
                    Grant Permission
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setShowGrantForm(null)}
                  >
                    Cancel
                  </Button>
                </div>
              </div>
            )}

            {user.role === 'admin' ? (
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Database className="h-4 w-4" />
                Admin has full access to all databases
              </div>
            ) : permissions.length === 0 ? (
              <div className="text-sm text-muted-foreground">
                No database permissions granted
              </div>
            ) : (
              <div className="space-y-2">
                {permissions.map((perm) => (
                  <div
                    key={perm.id}
                    className="flex items-center justify-between rounded-lg border p-3"
                  >
                    <div className="flex items-center gap-3">
                      <Database className="h-4 w-4 text-muted-foreground" />
                      <span className="font-medium">{perm.database}</span>
                    </div>
                    <div className="flex items-center gap-2">
                      {perm.can_read && <Badge variant="outline">Read</Badge>}
                      {perm.can_write && <Badge variant="outline">Write</Badge>}
                      {perm.can_create && <Badge variant="outline">Create</Badge>}
                      {perm.can_drop && <Badge variant="outline">Drop</Badge>}
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => handleRevokePermission(user.id, perm.database)}
                        className="text-destructive hover:text-destructive"
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
