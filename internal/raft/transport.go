package raft

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"
)

// Transport represents the network transport layer for Raft RPC
type Transport struct {
	nodeID   string
	address  string
	listener net.Listener
	raft     *Raft
	peers    map[string]string // nodeID -> address
	mu       sync.RWMutex
	quit     chan struct{}
}

// NewTransport creates a new Raft transport
func NewTransport(nodeID, address string, raft *Raft) *Transport {
	return &Transport{
		nodeID:  nodeID,
		address: address,
		raft:    raft,
		peers:   make(map[string]string),
		quit:    make(chan struct{}),
	}
}

// AddPeer adds a peer to the transport
func (t *Transport) AddPeer(nodeID, address string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.peers[nodeID] = address
}

// RemovePeer removes a peer from the transport
func (t *Transport) RemovePeer(nodeID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.peers, nodeID)
}

// Start starts the transport listener
func (t *Transport) Start() error {
	listener, err := net.Listen("tcp", t.address)
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}

	t.listener = listener
	slog.Info("Raft transport started", "node_id", t.nodeID, "address", t.address)

	go t.acceptLoop()
	return nil
}

// Stop stops the transport
func (t *Transport) Stop() {
	close(t.quit)
	if t.listener != nil {
		t.listener.Close()
	}
}

func (t *Transport) acceptLoop() {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			select {
			case <-t.quit:
				return
			default:
				slog.Error("failed to accept connection", "error", err)
				continue
			}
		}

		go t.handleConnection(conn)
	}
}

func (t *Transport) handleConnection(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var msg RPCMessage
	if err := decoder.Decode(&msg); err != nil {
		slog.Error("failed to decode RPC message", "error", err)
		return
	}

	var reply RPCMessage

	switch msg.Type {
	case RPCRequestVote:
		args := &RequestVoteArgs{}
		if err := json.Unmarshal(msg.Payload, args); err != nil {
			slog.Error("failed to unmarshal RequestVoteArgs", "error", err)
			return
		}
		replyPayload := t.raft.RequestVote(context.Background(), args)
		payload, _ := json.Marshal(replyPayload)
		reply = RPCMessage{
			Type:    RPCRequestVoteReply,
			Payload: payload,
		}

	case RPCAppendEntries:
		args := &AppendEntriesArgs{}
		if err := json.Unmarshal(msg.Payload, args); err != nil {
			slog.Error("failed to unmarshal AppendEntriesArgs", "error", err)
			return
		}
		replyPayload := t.raft.AppendEntries(context.Background(), args)
		payload, _ := json.Marshal(replyPayload)
		reply = RPCMessage{
			Type:    RPCAppendEntriesReply,
			Payload: payload,
		}

	default:
		slog.Warn("unknown RPC message type", "type", msg.Type)
		return
	}

	if err := encoder.Encode(reply); err != nil {
		slog.Error("failed to encode RPC reply", "error", err)
	}
}

// SendRequestVote sends a RequestVote RPC to a peer
func (t *Transport) SendRequestVote(peerID string, args *RequestVoteArgs) (*RequestVoteReply, error) {
	t.mu.RLock()
	address, exists := t.peers[peerID]
	t.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("peer %s not found", peerID)
	}

	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to peer %s: %w", peerID, err)
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	payload, _ := json.Marshal(args)
	msg := RPCMessage{
		Type:    RPCRequestVote,
		Payload: payload,
	}

	if err := encoder.Encode(msg); err != nil {
		return nil, fmt.Errorf("failed to send RequestVote: %w", err)
	}

	var reply RPCMessage
	if err := decoder.Decode(&reply); err != nil {
		return nil, fmt.Errorf("failed to receive RequestVote reply: %w", err)
	}

	var replyPayload RequestVoteReply
	if err := json.Unmarshal(reply.Payload, &replyPayload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal RequestVoteReply: %w", err)
	}

	return &replyPayload, nil
}

// SendAppendEntries sends an AppendEntries RPC to a peer
func (t *Transport) SendAppendEntries(peerID string, args *AppendEntriesArgs) (*AppendEntriesReply, error) {
	t.mu.RLock()
	address, exists := t.peers[peerID]
	t.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("peer %s not found", peerID)
	}

	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to peer %s: %w", peerID, err)
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	payload, _ := json.Marshal(args)
	msg := RPCMessage{
		Type:    RPCAppendEntries,
		Payload: payload,
	}

	if err := encoder.Encode(msg); err != nil {
		return nil, fmt.Errorf("failed to send AppendEntries: %w", err)
	}

	var reply RPCMessage
	if err := decoder.Decode(&reply); err != nil {
		return nil, fmt.Errorf("failed to receive AppendEntries reply: %w", err)
	}

	var replyPayload AppendEntriesReply
	if err := json.Unmarshal(reply.Payload, &replyPayload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal AppendEntriesReply: %w", err)
	}

	return &replyPayload, nil
}

// RPCMessageType represents the type of RPC message
type RPCMessageType int

const (
	RPCRequestVote RPCMessageType = iota
	RPCRequestVoteReply
	RPCAppendEntries
	RPCAppendEntriesReply
)

// RPCMessage represents an RPC message
type RPCMessage struct {
	Type    RPCMessageType  `json:"type"`
	Payload json.RawMessage `json:"payload"`
}
