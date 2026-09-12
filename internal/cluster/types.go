package cluster

import "time"

// NodeStatus represents the status of a cluster node
type NodeStatus string

const (
	NodeStatusActive   NodeStatus = "active"
	NodeStatusDraining NodeStatus = "draining"
	NodeStatusOffline  NodeStatus = "offline"
	NodeStatusJoining  NodeStatus = "joining"
	NodeStatusLeaving  NodeStatus = "leaving"
)

// NodeInfo represents information about a cluster node
type NodeInfo struct {
	NodeID      string     `json:"node_id"`
	Address     string     `json:"address"`
	ClusterPort int        `json:"cluster_port"`
	HTTPPort    int        `json:"http_port"`
	DriverPort  int        `json:"driver_port"`
	Status      NodeStatus `json:"status"`
	IsLeader    bool       `json:"is_leader"`
	JoinedAt    time.Time  `json:"joined_at"`
	LastSeen    time.Time  `json:"last_seen"`
	DataSizeMB  int64      `json:"data_size_mb"`
	TablesCount int        `json:"tables_count"`
}

// TransferStatus represents the status of a data transfer
type TransferStatus string

const (
	TransferStatusPending    TransferStatus = "pending"
	TransferStatusInProgress TransferStatus = "in_progress"
	TransferStatusCompleted  TransferStatus = "completed"
	TransferStatusFailed     TransferStatus = "failed"
)

// TransferTask represents a data transfer task between nodes
type TransferTask struct {
	ID           string         `json:"id"`
	SourceNodeID string         `json:"source_node_id"`
	TargetNodeID string         `json:"target_node_id"`
	Database     string         `json:"database"`
	Table        string         `json:"table"`
	Status       TransferStatus `json:"status"`
	Progress     int            `json:"progress"` // 0-100
	StartedAt    time.Time      `json:"started_at"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
	Error        string         `json:"error,omitempty"`
}

// RemoveOperation represents a node removal operation
type RemoveOperation struct {
	NodeID      string          `json:"node_id"`
	Status      string          `json:"status"` // draining, transferring, shutting_down, completed, failed
	Transfers   []*TransferTask `json:"transfers"`
	StartedAt   time.Time       `json:"started_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	Error       string          `json:"error,omitempty"`
}

// JoinRequest represents a request to join the cluster
type JoinRequest struct {
	Token       string `json:"token"`
	NodeID      string `json:"node_id"`
	Address     string `json:"address"`
	ClusterPort int    `json:"cluster_port"`
	HTTPPort    int    `json:"http_port"`
	DriverPort  int    `json:"driver_port"`
}

// JoinResponse represents the response to a join request
type JoinResponse struct {
	Success   bool        `json:"success"`
	NodeID    string      `json:"node_id,omitempty"`
	ClusterID string      `json:"cluster_id,omitempty"`
	LeaderID  string      `json:"leader_id,omitempty"`
	Members   []*NodeInfo `json:"members,omitempty"`
	Error     string      `json:"error,omitempty"`
}
