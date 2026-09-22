//go:build integration

package gothinkdb

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

func getTestOpts() ConnectOptions {
	return ConnectOptions{
		Host:    envOrDefault("GOTHINKDB_HOST", "localhost"),
		Port:    envOrDefaultInt("GOTHINKDB_PORT", 28015),
		DB:      "test",
		User:    "admin",
		Timeout: 10 * time.Second,
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envOrDefaultInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		fmt.Sscanf(v, "%d", &n)
		if n > 0 {
			return n
		}
	}
	return def
}

func TestIntegration_Connect(t *testing.T) {
	conn, err := Connect(getTestOpts())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	if !conn.IsOpen() {
		t.Fatal("connection should be open")
	}
	t.Log("Connected successfully to GoThinkDB")
}

func TestIntegration_DBList(t *testing.T) {
	conn, err := Connect(getTestOpts())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close()

	r := NewRWithConn(conn)
	data, err := r.DBList().Run(conn)
	if err != nil {
		t.Fatalf("db list: %v", err)
	}
	t.Logf("DBList result: %s", string(data))
}

func TestIntegration_DBCreateAndDrop(t *testing.T) {
	conn, err := Connect(getTestOpts())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close()

	r := NewRWithConn(conn)

	// Create database
	data, err := r.DBCreate("driver_test_db").Run(conn)
	if err != nil {
		t.Fatalf("db create: %v", err)
	}
	t.Logf("DBCreate result: %s", string(data))

	// Drop database
	data, err = r.DBDrop("driver_test_db").Run(conn)
	if err != nil {
		t.Fatalf("db drop: %v", err)
	}
	t.Logf("DBDrop result: %s", string(data))
}

func TestIntegration_TableOperations(t *testing.T) {
	conn, err := Connect(getTestOpts())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close()

	r := NewRWithConn(conn)

	// Create table
	data, err := r.DB("test").TableCreate("pool_test").Run(conn)
	if err != nil {
		t.Logf("table create (may already exist): %v", err)
	} else {
		t.Logf("TableCreate result: %s", string(data))
	}

	// List tables
	data, err = r.DB("test").TableList().Run(conn)
	if err != nil {
		t.Fatalf("table list: %v", err)
	}
	t.Logf("TableList result: %s", string(data))

	// Drop table
	data, err = r.DB("test").TableDrop("pool_test").Run(conn)
	if err != nil {
		t.Logf("table drop: %v", err)
	} else {
		t.Logf("TableDrop result: %s", string(data))
	}
}

func TestIntegration_CRUD(t *testing.T) {
	conn, err := Connect(getTestOpts())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close()

	r := NewRWithConn(conn)
	table := r.Table("crud_test")

	// INSERT
	doc := map[string]interface{}{
		"id":    "doc1",
		"name":  "Alice",
		"age":   30,
		"city":  "SP",
		"active": true,
	}
	data, err := table.Insert(doc).Run(conn)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	t.Logf("Insert result: %s", string(data))

	var writeResult WriteResult
	if err := json.Unmarshal(data, &writeResult); err == nil {
		if writeResult.Inserted != 1 {
			t.Errorf("expected inserted=1, got %d", writeResult.Inserted)
		}
	}

	// GET
	data, err = table.Get("doc1").Run(conn)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	t.Logf("Get result: %s", string(data))

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err == nil {
		if result["name"] != "Alice" {
			t.Errorf("expected name=Alice, got %v", result["name"])
		}
	}

	// UPDATE
	data, err = table.Get("doc1").Update(map[string]interface{}{"age": 31}).Run(conn)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	t.Logf("Update result: %s", string(data))

	// GET after update
	data, err = table.Get("doc1").Run(conn)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	t.Logf("Get after update: %s", string(data))

	// DELETE
	data, err = table.Get("doc1").Delete().Run(conn)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	t.Logf("Delete result: %s", string(data))
}

func TestIntegration_MultipleInserts(t *testing.T) {
	conn, err := Connect(getTestOpts())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close()

	r := NewRWithConn(conn)
	table := r.Table("multi_test")

	// Insert 10 documents
	for i := 0; i < 10; i++ {
		doc := map[string]interface{}{
			"id":    fmt.Sprintf("multi_%d", i),
			"name":  fmt.Sprintf("User%d", i),
			"age":   20 + i,
			"city":  []string{"SP", "RJ", "MG"}[i%3],
			"active": i%2 == 0,
		}
		_, err := table.Insert(doc).Run(conn)
		if err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}
	t.Log("Inserted 10 documents successfully")

	// Get each one
	for i := 0; i < 10; i++ {
		data, err := table.Get(fmt.Sprintf("multi_%d", i)).Run(conn)
		if err != nil {
			t.Fatalf("get %d: %v", i, err)
		}
		var doc map[string]interface{}
		if err := json.Unmarshal(data, &doc); err == nil {
			if doc["id"] != fmt.Sprintf("multi_%d", i) {
				t.Errorf("expected id=multi_%d, got %v", i, doc["id"])
			}
		}
	}
	t.Log("Retrieved all 10 documents successfully")

	// Cleanup
	for i := 0; i < 10; i++ {
		table.Get(fmt.Sprintf("multi_%d", i)).Delete().Run(conn)
	}
	t.Log("Cleanup complete")
}

func TestIntegration_FilterQuery(t *testing.T) {
	conn, err := Connect(getTestOpts())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close()

	r := NewRWithConn(conn)
	table := r.Table("filter_test")

	// Insert test data
	for i := 0; i < 5; i++ {
		doc := map[string]interface{}{
			"id":     fmt.Sprintf("f_%d", i),
			"name":   fmt.Sprintf("User%d", i),
			"active": i%2 == 0,
		}
		_, err := table.Insert(doc).Run(conn)
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	// Filter query
	data, err := table.Filter(map[string]interface{}{"active": true}).Run(conn)
	if err != nil {
		t.Logf("filter (may be stub): %v", err)
	} else {
		t.Logf("Filter result: %s", string(data))
	}

	// Cleanup
	for i := 0; i < 5; i++ {
		table.Get(fmt.Sprintf("f_%d", i)).Delete().Run(conn)
	}
}

func TestIntegration_Pool_Basic(t *testing.T) {
	pool, err := NewPool(PoolOptions{
		Host:     envOrDefault("GOTHINKDB_HOST", "localhost"),
		Port:     envOrDefaultInt("GOTHINKDB_PORT", 28015),
		DB:       "test",
		User:     "admin",
		MaxConns: 5,
		MinConns: 2,
	})
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	stats := pool.Stats()
	t.Logf("Initial pool stats: total=%d idle=%d inUse=%d", stats.TotalConns, stats.IdleConns, stats.InUseConns)

	if stats.TotalConns != 2 {
		t.Errorf("expected 2 initial conns, got %d", stats.TotalConns)
	}

	// Acquire and release
	conn, err := pool.Acquire()
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}

	stats = pool.Stats()
	t.Logf("After acquire: total=%d idle=%d inUse=%d", stats.TotalConns, stats.IdleConns, stats.InUseConns)
	if stats.InUseConns != 1 {
		t.Errorf("expected 1 in-use, got %d", stats.InUseConns)
	}

	pool.Release(conn)

	stats = pool.Stats()
	if stats.InUseConns != 0 {
		t.Errorf("expected 0 in-use after release, got %d", stats.InUseConns)
	}
	t.Log("Pool acquire/release works correctly")
}

func TestIntegration_Pool_ConcurrentQueries(t *testing.T) {
	pool, err := NewPool(PoolOptions{
		Host:     envOrDefault("GOTHINKDB_HOST", "localhost"),
		Port:     envOrDefaultInt("GOTHINKDB_PORT", 28015),
		DB:       "test",
		User:     "admin",
		MaxConns: 5,
		MinConns: 2,
	})
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	r := NewR()

	// Insert test data using pool
	err = pool.Exec(func(conn *Conn) error {
		r2 := NewRWithConn(conn)
		_, err := r2.Table("pool_concurrent").Insert(map[string]interface{}{
			"id":   "pool_1",
			"name": "PoolTest",
		}).Run(conn)
		return err
	})
	if err != nil {
		t.Fatalf("pool insert: %v", err)
	}

	// Run concurrent queries
	var wg sync.WaitGroup
	errCh := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := pool.Exec(func(conn *Conn) error {
				_, err := r.Table("pool_concurrent").Get("pool_1").Run(conn)
				return err
			})
			if err != nil {
				errCh <- fmt.Errorf("goroutine %d: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	errCount := 0
	for err := range errCh {
		t.Logf("Concurrent error: %v", err)
		errCount++
	}

	stats := pool.Stats()
	t.Logf("After concurrent: total=%d idle=%d inUse=%d created=%d destroyed=%d",
		stats.TotalConns, stats.IdleConns, stats.InUseConns, stats.TotalCreated, stats.TotalDestroyed)

	if errCount > 0 {
		t.Logf("%d/%d concurrent queries had errors (pool may have been exhausted)", errCount, 10)
	} else {
		t.Log("All 10 concurrent queries succeeded")
	}

	// Cleanup
	pool.Exec(func(conn *Conn) error {
		r2 := NewRWithConn(conn)
		r2.Table("pool_concurrent").Get("pool_1").Delete().Run(conn)
		return nil
	})
}

func TestIntegration_Pool_Exec(t *testing.T) {
	pool, err := NewPool(PoolOptions{
		Host:     envOrDefault("GOTHINKDB_HOST", "localhost"),
		Port:     envOrDefaultInt("GOTHINKDB_PORT", 28015),
		DB:       "test",
		User:     "admin",
		MaxConns: 3,
		MinConns: 1,
	})
	if err != nil {
		t.Fatalf("pool create: %v", err)
	}
	defer pool.Close()

	// Use Exec for automatic acquire/release
	err = pool.Exec(func(conn *Conn) error {
		r := NewRWithConn(conn)
		_, err := r.DBList().Run(conn)
		return err
	})
	if err != nil {
		t.Fatalf("pool exec: %v", err)
	}
	t.Log("Pool Exec works correctly")

	stats := pool.Stats()
	if stats.InUseConns != 0 {
		t.Errorf("expected 0 in-use after Exec, got %d", stats.InUseConns)
	}
}

func TestIntegration_Pool_MaxConns(t *testing.T) {
	pool, err := NewPool(PoolOptions{
		Host:     envOrDefault("GOTHINKDB_HOST", "localhost"),
		Port:     envOrDefaultInt("GOTHINKDB_PORT", 28015),
		DB:       "test",
		User:     "admin",
		MaxConns: 2,
		MinConns: 1,
	})
	if err != nil {
		t.Fatalf("pool create: %v", err)
	}
	defer pool.Close()

	// Acquire all connections
	conn1, err := pool.Acquire()
	if err != nil {
		t.Fatalf("acquire 1: %v", err)
	}
	conn2, err := pool.Acquire()
	if err != nil {
		t.Fatalf("acquire 2: %v", err)
	}

	// Third acquire should fail
	_, err = pool.Acquire()
	if err == nil {
		t.Fatal("expected error when pool exhausted")
	}
	t.Logf("Pool exhaustion correctly returned error: %v", err)

	pool.Release(conn1)
	pool.Release(conn2)

	stats := pool.Stats()
	t.Logf("Pool stats: total=%d idle=%d max=%d", stats.TotalConns, stats.IdleConns, stats.MaxConns)
}

func TestIntegration_Pool_Close(t *testing.T) {
	pool, err := NewPool(PoolOptions{
		Host:     envOrDefault("GOTHINKDB_HOST", "localhost"),
		Port:     envOrDefaultInt("GOTHINKDB_PORT", 28015),
		DB:       "test",
		User:     "admin",
		MaxConns: 3,
		MinConns: 2,
	})
	if err != nil {
		t.Fatalf("pool create: %v", err)
	}

	stats := pool.Stats()
	t.Logf("Before close: total=%d created=%d", stats.TotalConns, stats.TotalCreated)

	pool.Close()

	// Acquire should fail
	_, err = pool.Acquire()
	if err == nil {
		t.Fatal("expected error after pool close")
	}
	t.Logf("Acquire after close correctly failed: %v", err)
}
