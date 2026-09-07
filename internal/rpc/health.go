package rpc

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// HealthMonitor monitors the health of cluster nodes
type HealthMonitor struct {
	server   *Server
	cluster  *ClusterManager
	interval time.Duration
	timeout  time.Duration
	quit     chan struct{}
	wg       sync.WaitGroup
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(server *Server, cluster *ClusterManager, interval, timeout time.Duration) *HealthMonitor {
	return &HealthMonitor{
		server:   server,
		cluster:  cluster,
		interval: interval,
		timeout:  timeout,
		quit:     make(chan struct{}),
	}
}

// Start starts the health monitor
func (hm *HealthMonitor) Start() {
	hm.wg.Add(1)
	go func() {
		defer hm.wg.Done()
		hm.monitorLoop()
	}()

	slog.Info("health monitor started", "interval", hm.interval, "timeout", hm.timeout)
}

// Stop stops the health monitor
func (hm *HealthMonitor) Stop() {
	close(hm.quit)
	hm.wg.Wait()
	slog.Info("health monitor stopped")
}

func (hm *HealthMonitor) monitorLoop() {
	ticker := time.NewTicker(hm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hm.checkHealth()
		case <-hm.quit:
			return
		}
	}
}

func (hm *HealthMonitor) checkHealth() {
	ctx, cancel := context.WithTimeout(context.Background(), hm.timeout)
	defer cancel()

	members := hm.cluster.GetMembers()
	for _, member := range members {
		if member.NodeID == hm.server.nodeID {
			continue // Skip self
		}

		address := member.Address
		if member.ClusterPort > 0 {
			address = fmt.Sprintf("%s:%d", member.Address, member.ClusterPort)
		}

		err := hm.server.SendHeartbeat(ctx, address)
		if err != nil {
			slog.Warn("heartbeat failed", "node_id", member.NodeID, "error", err)
			hm.cluster.UpdateNodeStatus(member.NodeID, "inactive")
		} else {
			hm.cluster.UpdateNodeStatus(member.NodeID, "active")
		}
	}

	// Mark nodes as inactive if they haven't been seen recently
	hm.cluster.MarkInactiveNodes(hm.timeout * 3)
}

// GetClusterHealth returns the health status of the cluster
func (hm *HealthMonitor) GetClusterHealth() map[string]interface{} {
	members := hm.cluster.GetMembers()
	activeCount := 0
	inactiveCount := 0

	for _, member := range members {
		if member.Status == "active" {
			activeCount++
		} else {
			inactiveCount++
		}
	}

	return map[string]interface{}{
		"cluster_id":     hm.cluster.GetClusterID(),
		"total_nodes":    len(members),
		"active_nodes":   activeCount,
		"inactive_nodes": inactiveCount,
		"leader":         hm.cluster.GetLeader(),
		"term":           hm.cluster.GetTerm(),
		"healthy":        activeCount > len(members)/2,
	}
}
