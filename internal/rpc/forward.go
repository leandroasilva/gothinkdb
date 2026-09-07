package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"time"
)

// ForwardQueryPayload represents a forwarded query
type ForwardQueryPayload struct {
	QueryID   int64       `json:"query_id"`
	Query     interface{} `json:"query"`
	Token     int64       `json:"token"`
	Timestamp int64       `json:"timestamp"`
}

// ForwardResponsePayload represents a forwarded query response
type ForwardResponsePayload struct {
	QueryID   int64       `json:"query_id"`
	Result    interface{} `json:"result"`
	Error     string      `json:"error,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

// ForwardQuery forwards a query to another node
func (s *Server) ForwardQuery(ctx context.Context, targetNodeID string, queryID int64, query interface{}, token int64) (interface{}, error) {
	// Get target node address
	node, exists := s.cluster.GetNode(targetNodeID)
	if !exists {
		return nil, fmt.Errorf("node %s not found in cluster", targetNodeID)
	}

	address := node.Address
	if node.ClusterPort > 0 {
		address = fmt.Sprintf("%s:%d", node.Address, node.ClusterPort)
	}

	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	msg := &Message{
		Type:      MsgForwardQuery,
		From:      s.nodeID,
		To:        targetNodeID,
		Timestamp: time.Now().Unix(),
		Payload: ForwardQueryPayload{
			QueryID:   queryID,
			Query:     query,
			Token:     token,
			Timestamp: time.Now().Unix(),
		},
	}

	if err := encoder.Encode(msg); err != nil {
		return nil, fmt.Errorf("failed to send forward query: %w", err)
	}

	var response Message
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to read forward response: %w", err)
	}

	if response.Type != MsgForwardResponse {
		return nil, fmt.Errorf("unexpected response type: %d", response.Type)
	}

	payload, ok := response.Payload.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid forward response payload")
	}

	if errMsg, ok := payload["error"].(string); ok && errMsg != "" {
		return nil, fmt.Errorf("forward query failed: %s", errMsg)
	}

	return payload["result"], nil
}

// HandleForwardQuery handles a forwarded query
func (s *Server) handleForwardQuery(msg *Message) (*Message, error) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid forward query payload")
	}

	queryID, _ := payload["query_id"].(float64)

	slog.Debug("handling forwarded query", "query_id", queryID, "from", msg.From)

	// Execute the query using the ReQL evaluator
	// This would need to be integrated with the actual evaluator
	// For now, return a placeholder response
	result := map[string]interface{}{
		"status": "executed",
		"node":   s.nodeID,
	}

	return &Message{
		Type:      MsgForwardResponse,
		From:      s.nodeID,
		To:        msg.From,
		Timestamp: time.Now().Unix(),
		Payload: ForwardResponsePayload{
			QueryID:   int64(queryID),
			Result:    result,
			Timestamp: time.Now().Unix(),
		},
	}, nil
}
