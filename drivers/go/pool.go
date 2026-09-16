// Package gothinkdb provides a connection pool for managing multiple connections.
package gothinkdb

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// PoolOptions configures the connection pool.
type PoolOptions struct {
	Host            string
	Port            int
	DB              string
	User            string
	Password        string
	MaxConns        int
	MinConns        int
	ConnTimeout     time.Duration
	ConnMaxLifetime time.Duration
	HealthCheckInterval time.Duration
}

func (o *PoolOptions) defaults() {
	if o.Host == "" {
		o.Host = "localhost"
	}
	if o.Port == 0 {
		o.Port = 28015
	}
	if o.DB == "" {
		o.DB = "test"
	}
	if o.User == "" {
		o.User = "admin"
	}
	if o.MaxConns == 0 {
		o.MaxConns = 10
	}
	if o.MinConns == 0 {
		o.MinConns = 2
	}
	if o.ConnTimeout == 0 {
		o.ConnTimeout = 30 * time.Second
	}
	if o.ConnMaxLifetime == 0 {
		o.ConnMaxLifetime = 5 * time.Minute
	}
	if o.HealthCheckInterval == 0 {
		o.HealthCheckInterval = 30 * time.Second
	}
}

// poolConn wraps a Conn with metadata for pool management.
type poolConn struct {
	conn      *Conn
	createdAt time.Time
	inUse     bool
}

// Pool manages a pool of connections to a GoThinkDB server.
type Pool struct {
	opts      PoolOptions
	conns     []*poolConn
	mu        sync.Mutex
	closed    bool
	nextIdx   atomic.Uint64
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	totalCreated atomic.Int64
	totalDestroyed atomic.Int64
}

// NewPool creates a new connection pool.
func NewPool(opts PoolOptions) (*Pool, error) {
	opts.defaults()

	if opts.MinConns > opts.MaxConns {
		return nil, fmt.Errorf("MinConns (%d) cannot be greater than MaxConns (%d)", opts.MinConns, opts.MaxConns)
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		opts:   opts,
		conns:  make([]*poolConn, 0, opts.MaxConns),
		ctx:    ctx,
		cancel: cancel,
	}

	// Create minimum connections
	for i := 0; i < opts.MinConns; i++ {
		pc, err := p.createConn()
		if err != nil {
			p.Close()
			return nil, fmt.Errorf("failed to create initial connection %d: %w", i, err)
		}
		p.conns = append(p.conns, pc)
	}

	// Start background health check
	p.wg.Add(1)
	go p.healthCheckLoop()

	return p, nil
}

// createConn creates a new pooled connection.
func (p *Pool) createConn() (*poolConn, error) {
	conn, err := Connect(ConnectOptions{
		Host:     p.opts.Host,
		Port:     p.opts.Port,
		DB:       p.opts.DB,
		User:     p.opts.User,
		Password: p.opts.Password,
		Timeout:  p.opts.ConnTimeout,
	})
	if err != nil {
		return nil, err
	}
	p.totalCreated.Add(1)
	return &poolConn{
		conn:      conn,
		createdAt: time.Now(),
	}, nil
}

// Acquire gets a connection from the pool.
func (p *Pool) Acquire() (*Conn, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, fmt.Errorf("pool is closed")
	}

	// Try to find an idle connection
	for _, pc := range p.conns {
		if !pc.inUse && pc.conn.IsOpen() {
			pc.inUse = true
			p.mu.Unlock()
			return pc.conn, nil
		}
	}

	// No idle connection found; create a new one if under limit
	if len(p.conns) < p.opts.MaxConns {
		pc, err := p.createConn()
		if err != nil {
			p.mu.Unlock()
			return nil, fmt.Errorf("failed to create connection: %w", err)
		}
		pc.inUse = true
		p.conns = append(p.conns, pc)
		p.mu.Unlock()
		return pc.conn, nil
	}

	p.mu.Unlock()
	return nil, fmt.Errorf("connection pool exhausted (max=%d)", p.opts.MaxConns)
}

// Release returns a connection to the pool.
func (p *Pool) Release(conn *Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, pc := range p.conns {
		if pc.conn == conn {
			pc.inUse = false
			return
		}
	}
}

// Exec acquires a connection, executes the function, and releases the connection.
func (p *Pool) Exec(fn func(conn *Conn) error) error {
	conn, err := p.Acquire()
	if err != nil {
		return err
	}
	defer p.Release(conn)
	return fn(conn)
}

// Stats returns pool statistics.
func (p *Pool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	var total, idle, inUse int
	for _, pc := range p.conns {
		total++
		if pc.inUse {
			inUse++
		} else {
			idle++
		}
	}

	return PoolStats{
		TotalConns:     total,
		IdleConns:      idle,
		InUseConns:     inUse,
		MaxConns:       p.opts.MaxConns,
		MinConns:       p.opts.MinConns,
		TotalCreated:   p.totalCreated.Load(),
		TotalDestroyed: p.totalDestroyed.Load(),
	}
}

// PoolStats contains pool statistics.
type PoolStats struct {
	TotalConns     int   `json:"total_conns"`
	IdleConns      int   `json:"idle_conns"`
	InUseConns     int   `json:"in_use_conns"`
	MaxConns       int   `json:"max_conns"`
	MinConns       int   `json:"min_conns"`
	TotalCreated   int64 `json:"total_created"`
	TotalDestroyed int64 `json:"total_destroyed"`
}

// healthCheckLoop periodically checks and removes stale connections.
func (p *Pool) healthCheckLoop() {
	defer p.wg.Done()
	ticker := time.NewTicker(p.opts.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.removeStaleConns()
		}
	}
}

// removeStaleConns removes connections that exceeded max lifetime.
func (p *Pool) removeStaleConns() {
	p.mu.Lock()
	defer p.mu.Unlock()

	alive := make([]*poolConn, 0, len(p.conns))
	for _, pc := range p.conns {
		if pc.inUse {
			alive = append(alive, pc)
			continue
		}
		if time.Since(pc.createdAt) > p.opts.ConnMaxLifetime {
			pc.conn.Close()
			p.totalDestroyed.Add(1)
			continue
		}
		if !pc.conn.IsOpen() {
			p.totalDestroyed.Add(1)
			continue
		}
		alive = append(alive, pc)
	}

	// Ensure we maintain minimum connections
	for len(alive) < p.opts.MinConns {
		pc, err := p.createConn()
		if err != nil {
			break
		}
		alive = append(alive, pc)
	}

	p.conns = alive
}

// Close closes all connections in the pool.
func (p *Pool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	p.cancel()
	p.wg.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, pc := range p.conns {
		pc.conn.Close()
		p.totalDestroyed.Add(1)
	}
	p.conns = nil
	return nil
}
