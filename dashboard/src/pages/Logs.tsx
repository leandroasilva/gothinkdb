import { useState, useEffect, useRef } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ScrollText, Filter, Pause, Play, Trash2 } from 'lucide-react'

interface LogEntry {
  id: string
  timestamp: string
  level: 'info' | 'warn' | 'error' | 'debug'
  message: string
  server?: string
}

const LEVEL_COLORS: Record<string, string> = {
  info: 'bg-blue-500/10 text-blue-600 border-blue-500/20',
  warn: 'bg-yellow-500/10 text-yellow-600 border-yellow-500/20',
  error: 'bg-red-500/10 text-red-600 border-red-500/20',
  debug: 'bg-gray-500/10 text-gray-600 border-gray-500/20',
}

export function LogsPage() {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [filter, setFilter] = useState<string>('all')
  const [paused, setPaused] = useState(false)
  const [search, setSearch] = useState('')
  const scrollRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (paused) return

    const fetchLogs = async () => {
      try {
        const token = localStorage.getItem('auth_token')
        const headers: Record<string, string> = {}
        if (token) headers['Authorization'] = `Bearer ${token}`

        const res = await fetch('/api/logs', { headers })
        if (res.ok) {
          const data = await res.json()
          if (Array.isArray(data)) {
            setLogs(data)
          }
        }
      } catch {
        // ignore
      }
    }

    fetchLogs()
    const interval = setInterval(fetchLogs, 3000)
    return () => clearInterval(interval)
  }, [paused])

  useEffect(() => {
    if (scrollRef.current && !paused) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [logs, paused])

  const filteredLogs = logs.filter((log) => {
    if (filter !== 'all' && log.level !== filter) return false
    if (search && !log.message.toLowerCase().includes(search.toLowerCase())) return false
    return true
  })

  const levelCounts = logs.reduce(
    (acc, log) => {
      acc[log.level] = (acc[log.level] || 0) + 1
      return acc
    },
    {} as Record<string, number>
  )

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-foreground">Logs</h1>
          <p className="text-muted-foreground">
            Real-time server logs
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            variant={paused ? 'default' : 'outline'}
            size="sm"
            onClick={() => setPaused(!paused)}
          >
            {paused ? <Play size={14} className="mr-1" /> : <Pause size={14} className="mr-1" />}
            {paused ? 'Resume' : 'Pause'}
          </Button>
          <Button variant="outline" size="sm" onClick={() => setLogs([])}>
            <Trash2 size={14} className="mr-1" />
            Clear
          </Button>
        </div>
      </div>

      {/* Stats */}
      <div className="grid gap-4 md:grid-cols-4">
        {(['info', 'warn', 'error', 'debug'] as const).map((level) => (
          <Card key={level} className="cursor-pointer" onClick={() => setFilter(filter === level ? 'all' : level)}>
            <CardContent className="pt-4">
              <div className="flex items-center justify-between">
                <Badge className={LEVEL_COLORS[level]}>{level.toUpperCase()}</Badge>
                <span className="text-2xl font-bold">{levelCounts[level] || 0}</span>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Filters */}
      <Card>
        <CardContent className="pt-4">
          <div className="flex gap-2 items-center">
            <Filter size={16} className="text-muted-foreground" />
            <div className="flex gap-1">
              {['all', 'info', 'warn', 'error', 'debug'].map((level) => (
                <Button
                  key={level}
                  variant={filter === level ? 'default' : 'outline'}
                  size="sm"
                  onClick={() => setFilter(level)}
                >
                  {level}
                </Button>
              ))}
            </div>
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search logs..."
              className="flex-1 rounded-md border border-input bg-background px-3 py-1.5 text-sm"
            />
          </div>
        </CardContent>
      </Card>

      {/* Log entries */}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg flex items-center gap-2">
            <ScrollText size={18} />
            Log Entries
            <Badge variant="secondary">{filteredLogs.length}</Badge>
            {paused && (
              <Badge variant="outline" className="text-yellow-600">
                Paused
              </Badge>
            )}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div
            ref={scrollRef}
            className="space-y-1 max-h-[500px] overflow-y-auto font-mono text-sm"
          >
            {filteredLogs.length === 0 ? (
              <p className="text-muted-foreground text-center py-8">
                No log entries found.
              </p>
            ) : (
              filteredLogs.map((log) => (
                <div
                  key={log.id}
                  className="flex items-start gap-3 rounded px-3 py-2 hover:bg-accent/50"
                >
                  <Badge className={`${LEVEL_COLORS[log.level]} text-[10px] shrink-0 mt-0.5`}>
                    {log.level}
                  </Badge>
                  <span className="text-xs text-muted-foreground shrink-0 mt-0.5 w-20">
                    {new Date(log.timestamp).toLocaleTimeString()}
                  </span>
                  <span className="text-foreground break-all">{log.message}</span>
                </div>
              ))
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
