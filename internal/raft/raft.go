package raft

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"
)

// Raft represents a Raft consensus node
type Raft struct {
	config *Config

	// Persistent state
	currentTerm int64
	votedFor    string
	log         []*LogEntry

	// Volatile state
	state       State
	commitIndex int64
	lastApplied int64

	// Leader state
	nextIndex  map[string]int64
	matchIndex map[string]int64

	// Election state
	electionTimeout time.Duration
	lastHeartbeat   time.Time

	// Transport layer
	transport *Transport

	mu   sync.RWMutex
	quit chan struct{}
	wg   sync.WaitGroup
}

// New creates a new Raft node
func New(config *Config) *Raft {
	r := &Raft{
		config:          config,
		currentTerm:     0,
		votedFor:        "",
		log:             make([]*LogEntry, 0),
		state:           Follower,
		commitIndex:     0,
		lastApplied:     0,
		nextIndex:       make(map[string]int64),
		matchIndex:      make(map[string]int64),
		electionTimeout: config.ElectionTimeout,
		lastHeartbeat:   time.Now(),
		quit:            make(chan struct{}),
	}

	// Initialize nextIndex and matchIndex for all peers
	for _, peer := range config.Peers {
		r.nextIndex[peer] = 1
		r.matchIndex[peer] = 0
	}

	return r
}

// SetTransport sets the transport layer
func (r *Raft) SetTransport(transport *Transport) {
	r.transport = transport
}

// Start starts the Raft node
func (r *Raft) Start() {
	slog.Info("starting Raft node", "node_id", r.config.NodeID, "state", r.state)

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.run()
	}()
}

// Stop stops the Raft node
func (r *Raft) Stop() {
	close(r.quit)
	r.wg.Wait()
	slog.Info("Raft node stopped", "node_id", r.config.NodeID)
}

func (r *Raft) run() {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.tick()
		case <-r.quit:
			return
		}
	}
}

func (r *Raft) tick() {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch r.state {
	case Follower:
		r.tickFollower()
	case Candidate:
		r.tickCandidate()
	case Leader:
		r.tickLeader()
	}
}

func (r *Raft) tickFollower() {
	// Check if election timeout has elapsed
	if time.Since(r.lastHeartbeat) > r.electionTimeout {
		slog.Info("election timeout, becoming candidate", "node_id", r.config.NodeID)
		r.becomeCandidate()
	}
}

func (r *Raft) tickCandidate() {
	// Start election
	r.startElection()
}

func (r *Raft) tickLeader() {
	// Send heartbeats to all peers
	r.sendHeartbeats()
}

func (r *Raft) becomeCandidate() {
	r.state = Candidate
	r.currentTerm++
	r.votedFor = r.config.NodeID
	r.lastHeartbeat = time.Now()

	// Reset election timeout with jitter
	r.electionTimeout = r.config.ElectionTimeout + time.Duration(rand.Intn(150))*time.Millisecond

	slog.Info("became candidate", "node_id", r.config.NodeID, "term", r.currentTerm)
}

func (r *Raft) startElection() {
	// Request votes from all peers
	votes := 1 // Vote for self

	for _, peer := range r.config.Peers {
		go func(peerID string) {
			reply := r.requestVoteFromPeer(peerID)
			if reply != nil && reply.VoteGranted {
				r.mu.Lock()
				votes++

				// Check if we have majority
				if votes > (len(r.config.Peers)+1)/2 {
					if r.state == Candidate {
						r.becomeLeader()
					}
				}
				r.mu.Unlock()
			}
		}(peer)
	}

	// Reset election timeout
	r.lastHeartbeat = time.Now()
}

func (r *Raft) requestVoteFromPeer(peerID string) *RequestVoteReply {
	if r.transport == nil {
		return nil
	}

	args := &RequestVoteArgs{
		Term:         r.currentTerm,
		CandidateID:  r.config.NodeID,
		LastLogIndex: r.getLastLogIndex(),
		LastLogTerm:  r.getLastLogTerm(),
	}

	reply, err := r.transport.SendRequestVote(peerID, args)
	if err != nil {
		slog.Debug("failed to send RequestVote to peer", "peer", peerID, "error", err)
		return nil
	}

	return reply
}

func (r *Raft) becomeLeader() {
	slog.Info("became leader", "node_id", r.config.NodeID, "term", r.currentTerm)
	r.state = Leader

	// Initialize nextIndex and matchIndex
	lastLogIndex := r.getLastLogIndex()
	for _, peer := range r.config.Peers {
		r.nextIndex[peer] = lastLogIndex + 1
		r.matchIndex[peer] = 0
	}
}

func (r *Raft) sendHeartbeats() {
	for _, peer := range r.config.Peers {
		go r.sendAppendEntriesToPeer(peer)
	}
}

func (r *Raft) sendAppendEntriesToPeer(peerID string) {
	if r.transport == nil {
		return
	}

	r.mu.RLock()
	prevLogIndex := r.nextIndex[peerID] - 1
	prevLogTerm := int64(0)
	if prevLogIndex > 0 && prevLogIndex <= int64(len(r.log)) {
		prevLogTerm = r.log[prevLogIndex-1].Term
	}

	// Get entries to send
	var entries []*LogEntry
	if r.nextIndex[peerID] <= int64(len(r.log)) {
		entries = r.log[r.nextIndex[peerID]-1:]
	}

	args := &AppendEntriesArgs{
		Term:         r.currentTerm,
		LeaderID:     r.config.NodeID,
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  prevLogTerm,
		Entries:      entries,
		LeaderCommit: r.commitIndex,
	}
	r.mu.RUnlock()

	reply, err := r.transport.SendAppendEntries(peerID, args)
	if err != nil {
		slog.Debug("failed to send AppendEntries to peer", "peer", peerID, "error", err)
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if reply.Term > r.currentTerm {
		r.currentTerm = reply.Term
		r.state = Follower
		r.votedFor = ""
		return
	}

	if reply.Success {
		// Update nextIndex and matchIndex
		if len(entries) > 0 {
			r.nextIndex[peerID] = entries[len(entries)-1].Index + 1
			r.matchIndex[peerID] = entries[len(entries)-1].Index
		}
	} else {
		// Decrement nextIndex and retry
		if r.nextIndex[peerID] > 1 {
			r.nextIndex[peerID]--
		}
	}
}

// RequestVote handles a RequestVote RPC
func (r *Raft) RequestVote(ctx context.Context, args *RequestVoteArgs) *RequestVoteReply {
	r.mu.Lock()
	defer r.mu.Unlock()

	reply := &RequestVoteReply{
		Term:        r.currentTerm,
		VoteGranted: false,
	}

	// If term < currentTerm, reject
	if args.Term < r.currentTerm {
		return reply
	}

	// If term > currentTerm, become follower
	if args.Term > r.currentTerm {
		r.currentTerm = args.Term
		r.state = Follower
		r.votedFor = ""
	}

	// Check if we can grant the vote
	if r.votedFor == "" || r.votedFor == args.CandidateID {
		// Check if candidate's log is at least as up-to-date as ours
		lastLogIndex := r.getLastLogIndex()
		lastLogTerm := r.getLastLogTerm()

		if args.LastLogTerm > lastLogTerm ||
			(args.LastLogTerm == lastLogTerm && args.LastLogIndex >= lastLogIndex) {
			r.votedFor = args.CandidateID
			reply.VoteGranted = true
			r.lastHeartbeat = time.Now()
		}
	}

	return reply
}

// AppendEntries handles an AppendEntries RPC
func (r *Raft) AppendEntries(ctx context.Context, args *AppendEntriesArgs) *AppendEntriesReply {
	r.mu.Lock()
	defer r.mu.Unlock()

	reply := &AppendEntriesReply{
		Term:    r.currentTerm,
		Success: false,
	}

	// If term < currentTerm, reject
	if args.Term < r.currentTerm {
		return reply
	}

	// If term > currentTerm or we're a candidate, become follower
	if args.Term > r.currentTerm || r.state == Candidate {
		r.currentTerm = args.Term
		r.state = Follower
		r.votedFor = ""
	}

	r.lastHeartbeat = time.Now()

	// Check log consistency
	if args.PrevLogIndex > 0 {
		if args.PrevLogIndex > int64(len(r.log)) {
			return reply
		}
		if r.log[args.PrevLogIndex-1].Term != args.PrevLogTerm {
			// Delete conflicting entries
			r.log = r.log[:args.PrevLogIndex-1]
			return reply
		}
	}

	// Append new entries
	for _, entry := range args.Entries {
		if entry.Index <= int64(len(r.log)) {
			if r.log[entry.Index-1].Term != entry.Term {
				r.log = r.log[:entry.Index-1]
				r.log = append(r.log, entry)
			}
		} else {
			r.log = append(r.log, entry)
		}
	}

	// Update commit index
	if args.LeaderCommit > r.commitIndex {
		lastLogIndex := r.getLastLogIndex()
		if args.LeaderCommit < lastLogIndex {
			r.commitIndex = args.LeaderCommit
		} else {
			r.commitIndex = lastLogIndex
		}
	}

	reply.Success = true
	return reply
}

// Propose proposes a new command to the Raft cluster
func (r *Raft) Propose(ctx context.Context, command interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state != Leader {
		return fmt.Errorf("not leader")
	}

	// Append entry to log
	entry := &LogEntry{
		Term:      r.currentTerm,
		Index:     r.getLastLogIndex() + 1,
		Command:   command,
		Timestamp: time.Now().Unix(),
	}

	r.log = append(r.log, entry)

	// Replicate to peers
	r.sendHeartbeats()

	return nil
}

// Helper methods

func (r *Raft) getLastLogIndex() int64 {
	if len(r.log) == 0 {
		return 0
	}
	return r.log[len(r.log)-1].Index
}

func (r *Raft) getLastLogTerm() int64 {
	if len(r.log) == 0 {
		return 0
	}
	return r.log[len(r.log)-1].Term
}

// GetState returns the current state of the Raft node
func (r *Raft) GetState() State {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

// GetCurrentTerm returns the current term
func (r *Raft) GetCurrentTerm() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentTerm
}

// IsLeader returns true if this node is the leader
func (r *Raft) IsLeader() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state == Leader
}
