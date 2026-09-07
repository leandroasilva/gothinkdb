package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"
)

// Server represents an RPC server
type Server struct {
	nodeID      string
	address     string
	listener    net.Listener
	cluster     *ClusterManager
	quit        chan struct{}
	wg          sync.WaitGroup
	mu          sync.RWMutex
}

// NewServer creates a new RPC server
func NewServer(nodeID, address string, cluster *ClusterManager) *Server {
	return &Server{
		nodeID:  nodeID,
		address: address,
		cluster: cluster,
		quit:    make(chan struct{}),
	}
}

// Start starts the RPC server
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.address, err)
	}
	s.listener = listener

	slog.Info("RPC server starting", "address", s.address, "node_id", s.nodeID)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.acceptLoop()
	}()

	return nil
}

// Stop stops the RPC server
func (s *Server) Stop() error {
	close(s.quit)

	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			return err
		}
	}

	s.wg.Wait()
	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return
			default:
				slog.Error("accept error", "error", err)
				continue
			}
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConnection(conn)
		}()
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {
		var msg Message
		if err := decoder.Decode(&msg); err != nil {
			if err.Error() != "EOF" {
				slog.Error("failed to decode message", "error", err)
			}
			return
		}

		slog.Debug("received RPC message", "type", msg.Type, "from", msg.From)

		response, err := s.handleMessage(&msg)
		if err != nil {
			slog.Error("failed to handle message", "error", err)
			return
		}

		if response != nil {
			if err := encoder.Encode(response); err != nil {
				slog.Error("failed to encode response", "error", err)
				return
			}
		}
	}
}

func (s *Server) handleMessage(msg *Message) (*Message, error) {
	switch msg.Type {
	case MsgHeartbeat:
		return s.handleHeartbeat(msg)
	case MsgJoinRequest:
		return s.handleJoinRequest(msg)
	case MsgLeaveRequest:
		return s.handleLeaveRequest(msg)
	default:
		return nil, fmt.Errorf("unknown message type: %d", msg.Type)
	}
}

func (s *Server) handleHeartbeat(msg *Message) (*Message, error) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid heartbeat payload")
	}

	nodeID, _ := payload["node_id"].(string)
	s.cluster.UpdateNodeStatus(nodeID, "active")

	return &Message{
		Type:      MsgHeartbeatResponse,
		From:      s.nodeID,
		To:        msg.From,
		Timestamp: time.Now().Unix(),
		Payload: map[string]interface{}{
			"node_id":   s.nodeID,
			"timestamp": time.Now().Unix(),
			"status":    "active",
		},
	}, nil
}

func (s *Server) handleJoinRequest(msg *Message) (*Message, error) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid join request payload")
	}

	nodeID, _ := payload["node_id"].(string)
	address, _ := payload["address"].(string)
	clusterPort := int(payload["cluster_port"].(float64))

	// Add node to cluster
	err := s.cluster.AddNode(nodeID, address, clusterPort)
	if err != nil {
		return &Message{
			Type:      MsgJoinResponse,
			From:      s.nodeID,
			To:        msg.From,
			Timestamp: time.Now().Unix(),
			Payload: JoinResponsePayload{
				Success: false,
				Error:   err.Error(),
			},
		}, nil
	}

	// Get current members
	members := s.cluster.GetMembers()
	memberIDs := make([]string, 0, len(members))
	for _, member := range members {
		memberIDs = append(memberIDs, member.NodeID)
	}

	return &Message{
		Type:      MsgJoinResponse,
		From:      s.nodeID,
		To:        msg.From,
		Timestamp: time.Now().Unix(),
		Payload: JoinResponsePayload{
			Success:   true,
			NodeID:    s.nodeID,
			ClusterID: s.cluster.GetClusterID(),
			Members:   memberIDs,
		},
	}, nil
}

func (s *Server) handleLeaveRequest(msg *Message) (*Message, error) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid leave request payload")
	}

	nodeID, _ := payload["node_id"].(string)
	reason, _ := payload["reason"].(string)

	slog.Info("node leaving cluster", "node_id", nodeID, "reason", reason)

	// Remove node from cluster
	err := s.cluster.RemoveNode(nodeID)
	if err != nil {
		return &Message{
			Type:      MsgLeaveResponse,
			From:      s.nodeID,
			To:        msg.From,
			Timestamp: time.Now().Unix(),
			Payload: LeaveResponsePayload{
				Success: false,
				Error:   err.Error(),
			},
		}, nil
	}

	return &Message{
		Type:      MsgLeaveResponse,
		From:      s.nodeID,
		To:        msg.From,
		Timestamp: time.Now().Unix(),
		Payload: LeaveResponsePayload{
			Success: true,
		},
	}, nil
}

// SendHeartbeat sends a heartbeat to a specific node
func (s *Server) SendHeartbeat(ctx context.Context, address string) error {
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	msg := &Message{
		Type:      MsgHeartbeat,
		From:      s.nodeID,
		Timestamp: time.Now().Unix(),
		Payload: HeartbeatPayload{
			NodeID:    s.nodeID,
			Timestamp: time.Now().Unix(),
			Status:    "active",
		},
	}

	if err := encoder.Encode(msg); err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}

	var response Message
	if err := decoder.Decode(&response); err != nil {
		return fmt.Errorf("failed to read heartbeat response: %w", err)
	}

	return nil
}

// JoinCluster sends a join request to an existing cluster
func (s *Server) JoinCluster(ctx context.Context, seedAddress string) error {
	conn, err := net.DialTimeout("tcp", seedAddress, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to seed %s: %w", seedAddress, err)
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	msg := &Message{
		Type:      MsgJoinRequest,
		From:      s.nodeID,
		Timestamp: time.Now().Unix(),
		Payload: JoinRequestPayload{
			NodeID:      s.nodeID,
			Address:     s.address,
			ClusterPort: 29015,
		},
	}

	if err := encoder.Encode(msg); err != nil {
		return fmt.Errorf("failed to send join request: %w", err)
	}

	var response Message
	if err := decoder.Decode(&response); err != nil {
		return fmt.Errorf("failed to read join response: %w", err)
	}

	payload, ok := response.Payload.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid join response payload")
	}

	success, _ := payload["success"].(bool)
	if !success {
		errMsg, _ := payload["error"].(string)
		return fmt.Errorf("join failed: %s", errMsg)
	}

	slog.Info("successfully joined cluster", "cluster_id", payload["cluster_id"])
	return nil
}
