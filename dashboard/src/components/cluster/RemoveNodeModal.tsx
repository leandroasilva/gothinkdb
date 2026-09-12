import { useState, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { X, AlertTriangle, Loader2, CheckCircle } from 'lucide-react'
import { apiService } from '@/services/api'

interface RemoveNodeModalProps {
  isOpen: boolean
  onClose: () => void
  onSuccess: () => void
  node: {
    node_id: string
    address: string
    status: string
  } | null
}

interface RemoveOperation {
  node_id: string
  status: string
  transfers: Array<{
    id: string
    status: string
    progress: number
    database: string
    table: string
  }>
  started_at: string
  completed_at?: string
  error?: string
}

export function RemoveNodeModal({ isOpen, onClose, onSuccess, node }: RemoveNodeModalProps) {
  const [step, setStep] = useState<'confirm' | 'removing' | 'completed'>('confirm')
  const [operation, setOperation] = useState<RemoveOperation | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!isOpen || !node) return

    // Poll for removal status
    const pollStatus = async () => {
      try {
        const status = await apiService.getNodeRemovalStatus(node.node_id)
        setOperation(status)
        
        if (status.status === 'completed') {
          setStep('completed')
        } else if (status.status === 'failed') {
          setError(status.error || 'Removal failed')
        }
      } catch (err) {
        // Node might have been removed already
        if (err instanceof Error && err.message.includes('not found')) {
          setStep('completed')
        }
      }
    }

    const interval = setInterval(pollStatus, 2000)
    return () => clearInterval(interval)
  }, [isOpen, node])

  if (!isOpen || !node) return null

  const handleStartRemoval = async () => {
    setIsLoading(true)
    setError('')
    setStep('removing')
    
    try {
      await apiService.startNodeRemoval(node.node_id)
      // Start polling for status
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to start removal')
      setStep('confirm')
    } finally {
      setIsLoading(false)
    }
  }

  const handleClose = () => {
    if (step === 'completed') {
      onSuccess()
    }
    setStep('confirm')
    setOperation(null)
    setError('')
    onClose()
  }

  const getStatusMessage = (status: string) => {
    switch (status) {
      case 'draining':
        return 'Marking node as draining...'
      case 'identifying_data':
        return 'Identifying data to transfer...'
      case 'transferring':
        return 'Transferring data to other nodes...'
      case 'shutting_down':
        return 'Sending shutdown command...'
      case 'removing':
        return 'Removing node from cluster...'
      case 'completed':
        return 'Node removed successfully'
      case 'failed':
        return 'Removal failed'
      default:
        return status
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <Card className="w-full max-w-lg mx-4">
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle className="flex items-center gap-2">
              {step === 'completed' ? (
                <CheckCircle className="h-5 w-5 text-green-500" />
              ) : (
                <AlertTriangle className="h-5 w-5 text-destructive" />
              )}
              Remove Node
            </CardTitle>
            <CardDescription>
              {step === 'confirm' && 'This action will gracefully remove the node from the cluster'}
              {step === 'removing' && 'Removal in progress...'}
              {step === 'completed' && 'Node has been removed'}
            </CardDescription>
          </div>
          {step !== 'removing' && (
            <Button variant="ghost" size="icon" onClick={handleClose}>
              <X className="h-4 w-4" />
            </Button>
          )}
        </CardHeader>
        <CardContent>
          {error && (
            <div className="mb-4 rounded-lg bg-destructive/10 p-3 text-sm text-destructive">
              {error}
            </div>
          )}

          {/* Confirm Step */}
          {step === 'confirm' && (
            <div className="space-y-4">
              <div className="rounded-lg border border-destructive/50 p-4">
                <p className="font-medium mb-2">Node to remove:</p>
                <p className="text-sm font-mono">{node.node_id}</p>
                <p className="text-sm text-muted-foreground">{node.address}</p>
              </div>

              <div className="rounded-lg bg-muted p-4">
                <h4 className="font-medium mb-2">What will happen:</h4>
                <ul className="list-disc list-inside space-y-1 text-sm text-muted-foreground">
                  <li>Node will be marked as "draining"</li>
                  <li>Data unique to this node will be transferred to other nodes</li>
                  <li>Tables will be redistributed across remaining nodes</li>
                  <li>Node will receive shutdown command</li>
                  <li>Node will be removed from cluster</li>
                </ul>
              </div>

              <div className="flex gap-2">
                <Button variant="outline" onClick={handleClose}>
                  Cancel
                </Button>
                <Button 
                  variant="destructive" 
                  onClick={handleStartRemoval} 
                  disabled={isLoading}
                  className="flex-1"
                >
                  {isLoading ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      Starting...
                    </>
                  ) : (
                    'Remove Node'
                  )}
                </Button>
              </div>
            </div>
          )}

          {/* Removing Step */}
          {step === 'removing' && (
            <div className="space-y-4">
              <div className="flex items-center justify-center py-4">
                <Loader2 className="h-8 w-8 animate-spin text-primary" />
              </div>

              {operation && (
                <div className="space-y-3">
                  <div className="text-center">
                    <p className="font-medium">{getStatusMessage(operation.status)}</p>
                  </div>

                  {operation.transfers && operation.transfers.length > 0 && (
                    <div className="rounded-lg border p-3">
                      <p className="text-sm font-medium mb-2">Data Transfers:</p>
                      <div className="space-y-2">
                        {operation.transfers.map((transfer) => (
                          <div key={transfer.id} className="flex items-center justify-between">
                            <span className="text-xs text-muted-foreground">
                              {transfer.database}.{transfer.table}
                            </span>
                            <div className="flex items-center gap-2">
                              <div className="w-24 h-2 bg-muted rounded-full overflow-hidden">
                                <div 
                                  className="h-full bg-primary transition-all"
                                  style={{ width: `${transfer.progress}%` }}
                                />
                              </div>
                              <span className="text-xs">{transfer.progress}%</span>
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              )}
            </div>
          )}

          {/* Completed Step */}
          {step === 'completed' && (
            <div className="space-y-4">
              <div className="text-center py-4">
                <CheckCircle className="h-12 w-12 text-green-500 mx-auto mb-2" />
                <p className="font-medium">Node Removed Successfully</p>
                <p className="text-sm text-muted-foreground">
                  The node has been gracefully removed from the cluster.
                </p>
              </div>

              <Button onClick={handleClose} className="w-full">
                Close
              </Button>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
