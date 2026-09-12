package cluster

import (
	"log/slog"
	"sort"
	"sync"
)

// Rebalancer handles data rebalancing across cluster nodes
type Rebalancer struct {
	joinManager     *JoinManager
	transferManager *TransferManager
	mu              sync.RWMutex
}

// NewRebalancer creates a new rebalancer
func NewRebalancer(joinManager *JoinManager, transferManager *TransferManager) *Rebalancer {
	return &Rebalancer{
		joinManager:     joinManager,
		transferManager: transferManager,
	}
}

// NodeLoad represents the load on a node
type NodeLoad struct {
	NodeID     string
	DataSizeMB int64
	TablesCount int
	Score      float64 // Calculated load score
}

// RebalancePlan represents a plan for rebalancing data
type RebalancePlan struct {
	Transfers []*TransferTask
}

// CalculateRebalancePlan calculates a plan to rebalance data across nodes
func (r *Rebalancer) CalculateRebalancePlan() *RebalancePlan {
	r.mu.RLock()
	defer r.mu.RUnlock()

	members := r.joinManager.GetMembers()
	if len(members) <= 1 {
		return &RebalancePlan{Transfers: make([]*TransferTask, 0)}
	}

	// Calculate load for each node
	loads := make([]*NodeLoad, 0, len(members))
	for _, m := range members {
		if m.Status != NodeStatusActive {
			continue
		}
		loads = append(loads, &NodeLoad{
			NodeID:      m.NodeID,
			DataSizeMB:  m.DataSizeMB,
			TablesCount: m.TablesCount,
		})
	}

	if len(loads) == 0 {
		return &RebalancePlan{Transfers: make([]*TransferTask, 0)}
	}

	// Calculate load scores
	r.calculateLoadScores(loads)

	// Find most loaded and least loaded nodes
	sort.Slice(loads, func(i, j int) bool {
		return loads[i].Score > loads[j].Score
	})

	// Create transfer plan
	plan := &RebalancePlan{
		Transfers: make([]*TransferTask, 0),
	}

	// In a real implementation, this would:
	// 1. Identify tables/databases on overloaded nodes
	// 2. Select targets on underloaded nodes
	// 3. Create transfer tasks
	
	// For now, return empty plan (no rebalancing needed in simulation)
	slog.Debug("rebalance plan calculated",
		"nodes", len(loads),
		"transfers", len(plan.Transfers),
	)

	return plan
}

// calculateLoadScores calculates load scores for nodes
func (r *Rebalancer) calculateLoadScores(loads []*NodeLoad) {
	if len(loads) == 0 {
		return
	}

	// Calculate total and average
	var totalData int64
	var totalTables int
	for _, l := range loads {
		totalData += l.DataSizeMB
		totalTables += l.TablesCount
	}

	avgData := float64(totalData) / float64(len(loads))
	avgTables := float64(totalTables) / float64(len(loads))

	// Calculate score for each node (deviation from average)
	for _, l := range loads {
		dataDeviation := 0.0
		if avgData > 0 {
			dataDeviation = (float64(l.DataSizeMB) - avgData) / avgData
		}
		
		tablesDeviation := 0.0
		if avgTables > 0 {
			tablesDeviation = (float64(l.TablesCount) - avgTables) / avgTables
		}

		// Weighted score: 70% data, 30% tables
		l.Score = dataDeviation*0.7 + tablesDeviation*0.3
	}
}

// ExecuteRebalance executes a rebalance plan
func (r *Rebalancer) ExecuteRebalance(plan *RebalancePlan) error {
	if plan == nil || len(plan.Transfers) == 0 {
		slog.Debug("no transfers to execute")
		return nil
	}

	slog.Info("executing rebalance plan",
		"transfers", len(plan.Transfers),
	)

	for _, transfer := range plan.Transfers {
		r.transferManager.StartTransfer(transfer)
	}

	return nil
}

// TriggerRebalance triggers a rebalance operation
func (r *Rebalancer) TriggerRebalance() error {
	plan := r.CalculateRebalancePlan()
	return r.ExecuteRebalance(plan)
}

// GetClusterBalance returns balance information about the cluster
func (r *Rebalancer) GetClusterBalance() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	members := r.joinManager.GetMembers()
	
	var totalData int64
	var totalTables int
	activeNodes := 0

	for _, m := range members {
		if m.Status == NodeStatusActive {
			totalData += m.DataSizeMB
			totalTables += m.TablesCount
			activeNodes++
		}
	}

	avgData := int64(0)
	avgTables := 0
	if activeNodes > 0 {
		avgData = totalData / int64(activeNodes)
		avgTables = totalTables / activeNodes
	}

	return map[string]interface{}{
		"total_nodes":    len(members),
		"active_nodes":   activeNodes,
		"total_data_mb":  totalData,
		"total_tables":   totalTables,
		"avg_data_mb":    avgData,
		"avg_tables":     avgTables,
	}
}
