import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { X, Copy, Check, Loader2, Server } from 'lucide-react'
import { apiService } from '@/services/api'

interface AddNodeModalProps {
  isOpen: boolean
  onClose: () => void
  onSuccess: () => void
}

interface JoinToken {
  id: string
  token: string
  cluster_id: string
  created_at: string
  expires_at: string
  created_by: string
}

export function AddNodeModal({ isOpen, onClose, onSuccess }: AddNodeModalProps) {
  const [step, setStep] = useState(1)
  const [token, setToken] = useState<JoinToken | null>(null)
  const [nodeAddress, setNodeAddress] = useState('')
  const [clusterPort, setClusterPort] = useState('29015')
  const [httpPort, setHttpPort] = useState('8080')
  const [driverPort, setDriverPort] = useState('28015')
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState('')
  const [copied, setCopied] = useState(false)

  if (!isOpen) return null

  const handleGenerateToken = async () => {
    setIsLoading(true)
    setError('')
    try {
      const newToken = await apiService.generateJoinToken('24h')
      setToken(newToken)
      setStep(2)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to generate token')
    } finally {
      setIsLoading(false)
    }
  }

  const handleCopyToken = () => {
    if (token) {
      navigator.clipboard.writeText(token.token)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }
  }

  const handleAddNode = async () => {
    if (!token || !nodeAddress) return

    setIsLoading(true)
    setError('')
    try {
      await apiService.addNodeToCluster({
        token: token.token,
        address: nodeAddress,
        cluster_port: parseInt(clusterPort),
        http_port: parseInt(httpPort),
        driver_port: parseInt(driverPort),
      })
      onSuccess()
      handleClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add node')
    } finally {
      setIsLoading(false)
    }
  }

  const handleClose = () => {
    setStep(1)
    setToken(null)
    setNodeAddress('')
    setClusterPort('29015')
    setHttpPort('8080')
    setDriverPort('28015')
    setError('')
    onClose()
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <Card className="w-full max-w-2xl mx-4">
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle className="flex items-center gap-2">
              <Server className="h-5 w-5" />
              Add Node to Cluster
            </CardTitle>
            <CardDescription>Step {step} of 3</CardDescription>
          </div>
          <Button variant="ghost" size="icon" onClick={handleClose}>
            <X className="h-4 w-4" />
          </Button>
        </CardHeader>
        <CardContent>
          {error && (
            <div className="mb-4 rounded-lg bg-destructive/10 p-3 text-sm text-destructive">
              {error}
            </div>
          )}

          {/* Step 1: Generate Token */}
          {step === 1 && (
            <div className="space-y-4">
              <div>
                <h3 className="font-medium mb-2">Generate Join Token</h3>
                <p className="text-sm text-muted-foreground mb-4">
                  Generate a secure token that will allow a new node to join the cluster.
                  The token expires in 24 hours and can only be used once.
                </p>
              </div>
              <Button onClick={handleGenerateToken} disabled={isLoading} className="w-full">
                {isLoading ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Generating...
                  </>
                ) : (
                  'Generate Token'
                )}
              </Button>
            </div>
          )}

          {/* Step 2: Installation Instructions */}
          {step === 2 && token && (
            <div className="space-y-4">
              <div>
                <h3 className="font-medium mb-2">Join Token Generated</h3>
                <p className="text-sm text-muted-foreground mb-4">
                  Copy this token and use it when starting the new node.
                </p>
              </div>
              
              <div className="rounded-lg border p-4">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm font-medium">Token:</span>
                  <Button variant="ghost" size="sm" onClick={handleCopyToken}>
                    {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                  </Button>
                </div>
                <code className="block text-xs bg-muted p-2 rounded break-all">
                  {token.token}
                </code>
                <p className="text-xs text-muted-foreground mt-2">
                  Expires: {new Date(token.expires_at).toLocaleString()}
                </p>
              </div>

              <div className="rounded-lg border p-4">
                <h4 className="font-medium mb-2">Installation Instructions:</h4>
                <ol className="list-decimal list-inside space-y-2 text-sm">
                  <li>Install GoThinkDB on the new server</li>
                  <li>Start the node with the join token:</li>
                </ol>
                <code className="block text-xs bg-muted p-2 rounded mt-2 break-all">
                  ./gothinkdb -join-token {token.token.substring(0, 20)}... -join-address &lt;this-node-ip&gt;:29015
                </code>
              </div>

              <div className="flex gap-2">
                <Button variant="outline" onClick={() => setStep(1)}>
                  Back
                </Button>
                <Button onClick={() => setStep(3)} className="flex-1">
                  Continue
                </Button>
              </div>
            </div>
          )}

          {/* Step 3: Register Node */}
          {step === 3 && (
            <div className="space-y-4">
              <div>
                <h3 className="font-medium mb-2">Register Node</h3>
                <p className="text-sm text-muted-foreground mb-4">
                  Enter the address of the new node to register it in the cluster.
                </p>
              </div>

              <div className="space-y-3">
                <div>
                  <label className="text-sm font-medium">Node Address (IP or hostname)</label>
                  <input
                    type="text"
                    value={nodeAddress}
                    onChange={(e) => setNodeAddress(e.target.value)}
                    placeholder="192.168.1.101"
                    className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm mt-1"
                  />
                </div>
                <div className="grid grid-cols-3 gap-3">
                  <div>
                    <label className="text-sm font-medium">Cluster Port</label>
                    <input
                      type="number"
                      value={clusterPort}
                      onChange={(e) => setClusterPort(e.target.value)}
                      className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm mt-1"
                    />
                  </div>
                  <div>
                    <label className="text-sm font-medium">HTTP Port</label>
                    <input
                      type="number"
                      value={httpPort}
                      onChange={(e) => setHttpPort(e.target.value)}
                      className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm mt-1"
                    />
                  </div>
                  <div>
                    <label className="text-sm font-medium">Driver Port</label>
                    <input
                      type="number"
                      value={driverPort}
                      onChange={(e) => setDriverPort(e.target.value)}
                      className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm mt-1"
                    />
                  </div>
                </div>
              </div>

              <div className="flex gap-2">
                <Button variant="outline" onClick={() => setStep(2)}>
                  Back
                </Button>
                <Button onClick={handleAddNode} disabled={isLoading || !nodeAddress} className="flex-1">
                  {isLoading ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      Adding...
                    </>
                  ) : (
                    'Add Node'
                  )}
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
