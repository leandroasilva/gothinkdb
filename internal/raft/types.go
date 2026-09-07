package raft

import (
	"time"
)

// State represents the state of a Raft node
type State int

const (
	Follower State = iota
	Candidate
	Leader
)

// String returns the string representation of a state
func (s State) String() string {
	switch s {
	case Follower:
		return "Follower"
	case Candidate:
		return "Candidate"
	case Leader:
		return "Leader"
	default:
		return "Unknown"
	}
}

// LogEntry represents a single entry in the Raft log
type LogEntry struct {
	Term      int64       `json:"term"`
	Index     int64       `json:"index"`
	Command   interface{} `json:"command"`
	Timestamp int64       `json:"timestamp"`
}

// RequestVoteArgs represents the arguments for a RequestVote RPC
type RequestVoteArgs struct {
	Term         int64  `json:"term"`
	CandidateID  string `json:"candidate_id"`
	LastLogIndex int64  `json:"last_log_index"`
	LastLogTerm  int64  `json:"last_log_term"`
}

// RequestVoteReply represents the reply for a RequestVote RPC
type RequestVoteReply struct {
	Term        int64 `json:"term"`
	VoteGranted bool  `json:"vote_granted"`
}

// AppendEntriesArgs represents the arguments for an AppendEntries RPC
type AppendEntriesArgs struct {
	Term         int64       `json:"term"`
	LeaderID     string      `json:"leader_id"`
	PrevLogIndex int64       `json:"prev_log_index"`
	PrevLogTerm  int64       `json:"prev_log_term"`
	Entries      []*LogEntry `json:"entries"`
	LeaderCommit int64       `json:"leader_commit"`
}

// AppendEntriesReply represents the reply for an AppendEntries RPC
type AppendEntriesReply struct {
	Term    int64 `json:"term"`
	Success bool  `json:"success"`
}

// Config represents the configuration for a Raft node
type Config struct {
	NodeID            string        `json:"node_id"`
	Peers             []string      `json:"peers"`
	ElectionTimeout   time.Duration `json:"election_timeout"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
}

// DefaultConfig returns a default Raft configuration
func DefaultConfig(nodeID string, peers []string) *Config {
	return &Config{
		NodeID:            nodeID,
		Peers:             peers,
		ElectionTimeout:   150 * time.Millisecond,
		HeartbeatInterval: 50 * time.Millisecond,
	}
}
