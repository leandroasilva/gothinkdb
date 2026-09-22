import { useState, useCallback } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Play, History, Trash2, Copy, Download } from 'lucide-react'

interface QueryResult {
  status: string
  result: unknown
  error?: string
}

interface HistoryItem {
  query: string
  result: QueryResult
  timestamp: Date
}

export function ExplorerPage() {
  const [query, setQuery] = useState('r.table("test")')
  const [result, setResult] = useState<QueryResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [history, setHistory] = useState<HistoryItem[]>([])
  const [viewMode, setViewMode] = useState<'table' | 'json'>('json')

  const executeQuery = useCallback(async () => {
    if (!query.trim()) return
    setLoading(true)
    try {
      const token = localStorage.getItem('auth_token')
      const headers: Record<string, string> = { 'Content-Type': 'application/json' }
      if (token) headers['Authorization'] = `Bearer ${token}`

      const res = await fetch('/api/query', {
        method: 'POST',
        headers,
        body: JSON.stringify({ query }),
      })
      const data = await res.json()
      const queryResult: QueryResult = {
        status: data.status || (res.ok ? 'success' : 'error'),
        result: data.result ?? data,
        error: data.error,
      }
      setResult(queryResult)
      setHistory((prev) => [
        { query, result: queryResult, timestamp: new Date() },
        ...prev.slice(0, 49),
      ])
    } catch (err) {
      setResult({
        status: 'error',
        result: null,
        error: err instanceof Error ? err.message : 'Unknown error',
      })
    } finally {
      setLoading(false)
    }
  }, [query])

  const loadFromHistory = (item: HistoryItem) => {
    setQuery(item.query)
    setResult(item.result)
  }

  const copyResult = () => {
    if (result) {
      navigator.clipboard.writeText(JSON.stringify(result.result, null, 2))
    }
  }

  const downloadResult = () => {
    if (result) {
      const blob = new Blob([JSON.stringify(result.result, null, 2)], {
        type: 'application/json',
      })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'query-result.json'
      a.click()
      URL.revokeObjectURL(url)
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold text-foreground">Data Explorer</h1>
        <p className="text-muted-foreground">
          Execute ReQL queries interactively
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        {/* Query Editor */}
        <div className="lg:col-span-2 space-y-4">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle className="text-lg">Query</CardTitle>
              <div className="flex gap-2">
                <Button
                  size="sm"
                  onClick={executeQuery}
                  disabled={loading || !query.trim()}
                >
                  <Play size={14} className="mr-1" />
                  {loading ? 'Running...' : 'Run'}
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              <textarea
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                className="w-full h-40 rounded-md border border-input bg-background px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-ring resize-none"
                placeholder="Enter ReQL query...&#10;Example: r.table('users').filter({name: 'John'})"
                onKeyDown={(e) => {
                  if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
                    executeQuery()
                  }
                }}
              />
              <div className="flex items-center justify-between mt-2">
                <p className="text-xs text-muted-foreground">
                  Press Cmd/Ctrl+Enter to run
                </p>
                <div className="flex gap-2">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setQuery('')}
                  >
                    Clear
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Result */}
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle className="text-lg flex items-center gap-2">
                Result
                {result && (
                  <Badge
                    variant={result.status === 'success' ? 'default' : 'destructive'}
                    className="text-xs"
                  >
                    {result.status}
                  </Badge>
                )}
              </CardTitle>
              <div className="flex gap-2">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setViewMode(viewMode === 'json' ? 'table' : 'json')}
                >
                  {viewMode === 'json' ? 'Table' : 'JSON'}
                </Button>
                <Button variant="ghost" size="icon" onClick={copyResult} disabled={!result}>
                  <Copy size={14} />
                </Button>
                <Button variant="ghost" size="icon" onClick={downloadResult} disabled={!result}>
                  <Download size={14} />
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              {!result ? (
                <p className="text-sm text-muted-foreground">
                  Run a query to see results here.
                </p>
              ) : result.error ? (
                <div className="rounded-lg bg-destructive/10 border border-destructive/20 p-4">
                  <p className="text-sm text-destructive font-mono">{result.error}</p>
                </div>
              ) : viewMode === 'json' ? (
                <pre className="rounded-lg bg-muted p-4 text-sm font-mono overflow-auto max-h-96">
                  {JSON.stringify(result.result, null, 2)}
                </pre>
              ) : Array.isArray(result.result) && result.result.length > 0 ? (
                <div className="overflow-x-auto max-h-96">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b">
                        {Object.keys(result.result[0] as Record<string, unknown>).map((key) => (
                          <th key={key} className="text-left py-2 px-3 font-medium text-muted-foreground">
                            {key}
                          </th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {(result.result as Record<string, unknown>[]).map((row, i) => (
                        <tr key={i} className="border-b hover:bg-accent/50">
                          {Object.values(row).map((val, j) => (
                            <td key={j} className="py-2 px-3 max-w-[200px] truncate">
                              {typeof val === 'object' ? JSON.stringify(val) : String(val)}
                            </td>
                          ))}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <pre className="rounded-lg bg-muted p-4 text-sm font-mono">
                  {JSON.stringify(result.result, null, 2)}
                </pre>
              )}
            </CardContent>
          </Card>
        </div>

        {/* History */}
        <div>
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle className="text-lg flex items-center gap-2">
                <History size={18} />
                History
                <Badge variant="secondary" className="text-xs">
                  {history.length}
                </Badge>
              </CardTitle>
              {history.length > 0 && (
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => setHistory([])}
                >
                  <Trash2 size={14} />
                </Button>
              )}
            </CardHeader>
            <CardContent>
              {history.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  No query history yet.
                </p>
              ) : (
                <div className="space-y-2 max-h-[600px] overflow-y-auto">
                  {history.map((item, i) => (
                    <button
                      key={i}
                      onClick={() => loadFromHistory(item)}
                      className="w-full text-left rounded-lg border p-3 hover:bg-accent/50 transition-colors"
                    >
                      <p className="text-xs font-mono text-muted-foreground truncate">
                        {item.query}
                      </p>
                      <div className="flex items-center justify-between mt-1">
                        <Badge
                          variant={item.result.status === 'success' ? 'default' : 'destructive'}
                          className="text-[10px]"
                        >
                          {item.result.status}
                        </Badge>
                        <span className="text-[10px] text-muted-foreground">
                          {item.timestamp.toLocaleTimeString()}
                        </span>
                      </div>
                    </button>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}
