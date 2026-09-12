const API_BASE = ''

export interface User {
  id: string
  username: string
  role: 'admin' | 'user'
  created_at: string
  updated_at: string
}

export interface Permission {
  id: string
  database: string
  can_read: boolean
  can_write: boolean
  can_create: boolean
  can_drop: boolean
}

export interface LoginResponse {
  token: string
  user: User
}

export interface MeResponse {
  user: User
  permissions: Permission[]
}

class ApiService {
  private token: string | null = null

  setToken(token: string | null) {
    this.token = token
  }

  getToken(): string | null {
    return this.token
  }

  private async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    }

    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`
    }

    const response = await fetch(`${API_BASE}${endpoint}`, {
      ...options,
      headers,
    })

    if (response.status === 401) {
      // Token expired or invalid
      this.setToken(null)
      localStorage.removeItem('auth_token')
      window.location.href = '/login'
      throw new Error('Unauthorized')
    }

    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: 'Unknown error' }))
      throw new Error(error.error || `HTTP ${response.status}`)
    }

    return response.json()
  }

  // Auth
  async login(username: string, password: string): Promise<LoginResponse> {
    const data = await this.request<LoginResponse>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    })
    this.setToken(data.token)
    return data
  }

  async me(): Promise<MeResponse> {
    return this.request<MeResponse>('/api/auth/me')
  }

  // Databases
  async listDatabases(): Promise<string[]> {
    return this.request<string[]>('/api/databases')
  }

  async createDatabase(name: string): Promise<{ status: string }> {
    return this.request<{ status: string }>('/api/databases', {
      method: 'POST',
      body: JSON.stringify({ name }),
    })
  }

  async dropDatabase(name: string): Promise<{ status: string }> {
    return this.request<{ status: string }>(`/api/databases/${name}`, {
      method: 'DELETE',
    })
  }

  // Tables
  async listTables(db: string): Promise<string[]> {
    return this.request<string[]>(`/api/tables?db=${encodeURIComponent(db)}`)
  }

  async createTable(db: string, name: string): Promise<{ status: string }> {
    return this.request<{ status: string }>(`/api/tables/${name}?db=${encodeURIComponent(db)}`, {
      method: 'POST',
    })
  }

  async dropTable(db: string, name: string): Promise<{ status: string }> {
    return this.request<{ status: string }>(`/api/tables/${name}?db=${encodeURIComponent(db)}`, {
      method: 'DELETE',
    })
  }

  // Users (admin only)
  async listUsers(): Promise<User[]> {
    return this.request<User[]>('/api/users')
  }

  async createUser(username: string, password: string, role: 'admin' | 'user'): Promise<User> {
    return this.request<User>('/api/users', {
      method: 'POST',
      body: JSON.stringify({ username, password, role }),
    })
  }

  async deleteUser(id: string): Promise<{ status: string }> {
    return this.request<{ status: string }>(`/api/users/${id}`, {
      method: 'DELETE',
    })
  }

  // Permissions (admin only)
  async listUserPermissions(userId: string): Promise<Permission[]> {
    return this.request<Permission[]>(`/api/users/${userId}/permissions`)
  }

  async grantPermission(
    userId: string,
    database: string,
    canRead: boolean,
    canWrite: boolean,
    canCreate: boolean,
    canDrop: boolean
  ): Promise<Permission> {
    return this.request<Permission>(`/api/users/${userId}/permissions`, {
      method: 'POST',
      body: JSON.stringify({
        database,
        can_read: canRead,
        can_write: canWrite,
        can_create: canCreate,
        can_drop: canDrop,
      }),
    })
  }

  async revokePermission(userId: string, database: string): Promise<{ status: string }> {
    return this.request<{ status: string }>(`/api/users/${userId}/permissions/${database}`, {
      method: 'DELETE',
    })
  }

  // Cluster
  async getClusterStatus(): Promise<unknown> {
    return this.request('/api/cluster/status')
  }

  async getClusterMembers(): Promise<unknown[]> {
    return this.request<unknown[]>('/api/cluster/members')
  }

  // Cluster Tokens
  async generateJoinToken(expiresIn: string = '24h'): Promise<{
    id: string
    token: string
    cluster_id: string
    created_at: string
    expires_at: string
    created_by: string
  }> {
    return this.request('/api/cluster/tokens', {
      method: 'POST',
      body: JSON.stringify({ expires_in: expiresIn }),
    })
  }

  async listJoinTokens(): Promise<Array<{
    id: string
    token: string
    cluster_id: string
    created_at: string
    expires_at: string
    created_by: string
  }>> {
    return this.request('/api/cluster/tokens')
  }

  async revokeJoinToken(tokenId: string): Promise<{ status: string }> {
    return this.request(`/api/cluster/tokens/${tokenId}`, {
      method: 'DELETE',
    })
  }

  // Cluster Nodes
  async addNodeToCluster(params: {
    token: string
    address: string
    cluster_port: number
    http_port: number
    driver_port: number
  }): Promise<{ success: boolean; node_id?: string; error?: string }> {
    return this.request('/api/cluster/join', {
      method: 'POST',
      body: JSON.stringify({
        token: params.token,
        node_id: `node-${Date.now()}`,
        address: params.address,
        cluster_port: params.cluster_port,
        http_port: params.http_port,
        driver_port: params.driver_port,
      }),
    })
  }

  async startNodeRemoval(nodeId: string): Promise<{ status: string; node_id: string }> {
    return this.request(`/api/cluster/nodes/${nodeId}/remove`, {
      method: 'POST',
    })
  }

  async getNodeRemovalStatus(nodeId: string): Promise<{
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
  }> {
    return this.request(`/api/cluster/nodes/${nodeId}/status`)
  }

  async getClusterBalance(): Promise<{
    total_nodes: number
    active_nodes: number
    total_data_mb: number
    total_tables: number
    avg_data_mb: number
    avg_tables: number
  }> {
    return this.request('/api/cluster/balance')
  }

  async getClusterTransfers(): Promise<Array<{
    id: string
    source_node_id: string
    target_node_id: string
    database: string
    table: string
    status: string
    progress: number
    started_at: string
    completed_at?: string
    error?: string
  }>> {
    return this.request('/api/cluster/transfers')
  }

  // Server
  async getServerInfo(): Promise<unknown> {
    return this.request('/api/server/info')
  }

  async getServerStats(): Promise<unknown> {
    return this.request('/api/server/stats')
  }

  // Logs
  async getLogs(): Promise<unknown[]> {
    return this.request<unknown[]>('/api/logs')
  }
}

export const apiService = new ApiService()
