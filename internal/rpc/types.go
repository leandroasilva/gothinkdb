package rpc

import (
	"time"
)

// MessageType represents the type of RPC message
type MessageType int

const (
	// Heartbeat messages
	MsgHeartbeat MessageType = iota
	MsgHeartbeatResponse

	// Cluster membership messages
	MsgJoinRequest
	MsgJoinResponse
	MsgLeaveRequest
	MsgLeaveResponse

	// Data replication messages
	MsgReplicateRequest
	MsgReplicateResponse
	MsgSyncRequest
	MsgSyncResponse

	// Query forwarding messages
	MsgForwardQuery
	MsgForwardResponse
)

// Message represents an RPC message
type Message struct {
	Type      MessageType `json:"type"`
	From      string      `json:"from"`
	To        string      `json:"to,omitempty"`
	Timestamp int64       `json:"timestamp"`
	Payload   interface{} `json:"payload"`
}

// HeartbeatPayload represents a heartbeat message
type HeartbeatPayload struct {
	NodeID    string `json:"node_id"`
	Timestamp int64  `json:"timestamp"`
	Status    string `json:"status"`
}

// JoinRequestPayload represents a join request
type JoinRequestPayload struct {
	NodeID      string `json:"node_id"`
	Address     string `json:"address"`
	ClusterPort int    `json:"cluster_port"`
}

// JoinResponsePayload represents a join response
type JoinResponsePayload struct {
	Success   bool     `json:"success"`
	NodeID    string   `json:"node_id,omitempty"`
	ClusterID string   `json:"cluster_id,omitempty"`
	Members   []string `json:"members,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// LeaveRequestPayload represents a leave request
type LeaveRequestPayload struct {
	NodeID string `json:"node_id"`
	Reason string `json:"reason"`
}

// LeaveResponsePayload represents a leave response
type LeaveResponsePayload struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// NodeInfo represents information about a cluster node
type NodeInfo struct {
	NodeID      string    `json:"node_id"`
	Address     string    `json:"address"`
	ClusterPort int       `json:"cluster_port"`
	Status      string    `json:"status"` // "active", "inactive", "joining", "leaving"
	LastSeen    time.Time `json:"last_seen"`
	JoinedAt    time.Time `json:"joined_at"`
}

// ClusterState represents the current state of the cluster
type ClusterState struct {
	ClusterID string                `json:"cluster_id"`
	Members   map[string]*NodeInfo  `json:"members"`
	Leader    string                `json:"leader,omitempty"`
	Term      int64                 `json:"term"`
	UpdatedAt time.Time             `json:"updated_at"`
}
