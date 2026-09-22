//go:build integration

package gothinkdb

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"
)

// ============================================================
// Helpers
// ============================================================

func testConnect(t *testing.T) *Conn {
	t.Helper()
	conn, err := Connect(ConnectOptions{
		Host:    envOrDefault("GOTHINKDB_HOST", "localhost"),
		Port:    envOrDefaultInt("GOTHINKDB_PORT", 28015),
		DB:      "test",
		User:    "admin",
		Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	return conn
}

func cleanupTable(t *testing.T, r *R, conn *Conn, table string) {
	t.Helper()
	r.Table(table).Delete().Run(conn)
}

// ============================================================
// 1. Database Operations
// ============================================================

func TestComprehensive_DatabaseOperations(t *testing.T) {
	conn := testConnect(t)
	defer conn.Close()
	r := NewRWithConn(conn)

	// Create database
	data, err := r.DBCreate("comprehensive_test_db").Run(conn)
	if err != nil {
		t.Fatalf("DBCreate: %v", err)
	}
	t.Logf("DBCreate: %s", string(data))

	// List databases
	data, err = r.DBList().Run(conn)
	if err != nil {
		t.Fatalf("DBList: %v", err)
	}
	t.Logf("DBList: %s", string(data))
	var dbs []string
	json.Unmarshal(data, &dbs)
	found := false
	for _, db := range dbs {
		if db == "comprehensive_test_db" {
			found = true
			break
		}
	}
	if !found {
		t.Error("created database not found in list")
	}

	// Drop database
	data, err = r.DBDrop("comprehensive_test_db").Run(conn)
	if err != nil {
		t.Fatalf("DBDrop: %v", err)
	}
	t.Logf("DBDrop: %s", string(data))
}

// ============================================================
// 2. Table Operations
// ============================================================

func TestComprehensive_TableOperations(t *testing.T) {
	conn := testConnect(t)
	defer conn.Close()
	r := NewRWithConn(conn)

	// Create tables
	for _, name := range []string{"test_users", "test_orders", "test_products"} {
		_, err := r.DB("test").TableCreate(name).Run(conn)
		if err != nil {
			t.Logf("TableCreate %s (may exist): %v", name, err)
		}
	}

	// List tables
	data, err := r.DB("test").TableList().Run(conn)
	if err != nil {
		t.Fatalf("TableList: %v", err)
	}
	t.Logf("TableList: %s", string(data))

	// Drop tables
	for _, name := range []string{"test_users", "test_orders", "test_products"} {
		_, err := r.DB("test").TableDrop(name).Run(conn)
		if err != nil {
			t.Logf("TableDrop %s: %v", name, err)
		}
	}
	t.Log("Table operations completed")
}

// ============================================================
// 3. Insert 1000+ Records
// ============================================================

func TestComprehensive_Insert1000Records(t *testing.T) {
	conn := testConnect(t)
	defer conn.Close()
	r := NewRWithConn(conn)

	table := r.Table("bulk_users")
	cleanupTable(t, r, conn, "bulk_users")

	cities := []string{"São Paulo", "Rio de Janeiro", "Belo Horizonte", "Curitiba", "Porto Alegre", "Salvador", "Fortaleza", "Brasília"}
	departments := []string{"Engineering", "Marketing", "Sales", "HR", "Finance"}
	statuses := []string{"active", "inactive", "pending"}

	start := time.Now()
	for i := 0; i < 1100; i++ {
		doc := map[string]interface{}{
			"id":         fmt.Sprintf("user_%04d", i),
			"name":       fmt.Sprintf("User %d", i),
			"email":      fmt.Sprintf("user%d@example.com", i),
			"age":        18 + rand.Intn(50),
			"salary":     3000 + rand.Float64()*7000,
			"city":       cities[rand.Intn(len(cities))],
			"department": departments[rand.Intn(len(departments))],
			"status":     statuses[rand.Intn(len(statuses))],
			"active":     rand.Intn(2) == 1,
			"score":      rand.Float64() * 100,
			"created_at": time.Now().Unix(),
		}
		_, err := table.Insert(doc).Run(conn)
		if err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}
	t.Logf("Inserted 1100 records in %v", time.Since(start))

	// Verify count via GetAll
	data, err := table.GetAll().Run(conn)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	var docs []interface{}
	json.Unmarshal(data, &docs)
	if len(docs) != 1100 {
		t.Errorf("expected 1100 docs, got %d", len(docs))
	}
	t.Logf("Verified %d documents via GetAll", len(docs))

	// Cleanup
	cleanupTable(t, r, conn, "bulk_users")
}

// ============================================================
// 4. Multi-Table CRUD with Relationships
// ============================================================

func TestComprehensive_MultiTableCRUD(t *testing.T) {
	conn := testConnect(t)
	defer conn.Close()
	r := NewRWithConn(conn)

	// Clean up
	cleanupTable(t, r, conn, "mt_users")
	cleanupTable(t, r, conn, "mt_orders")
	cleanupTable(t, r, conn, "mt_products")

	users := r.Table("mt_users")
	orders := r.Table("mt_orders")
	products := r.Table("mt_products")

	// Insert users
	for i := 0; i < 50; i++ {
		_, err := users.Insert(map[string]interface{}{
			"id":   fmt.Sprintf("u%d", i),
			"name": fmt.Sprintf("User%d", i),
			"city": []string{"SP", "RJ", "MG"}[i%3],
			"age":  20 + i,
		}).Run(conn)
		if err != nil {
			t.Fatalf("insert user %d: %v", i, err)
		}
	}

	// Insert products
	for i := 0; i < 20; i++ {
		_, err := products.Insert(map[string]interface{}{
			"id":    fmt.Sprintf("p%d", i),
			"name":  fmt.Sprintf("Product%d", i),
			"price": 10.0 + float64(i)*5.5,
			"stock": 100 + i*10,
		}).Run(conn)
		if err != nil {
			t.Fatalf("insert product %d: %v", i, err)
		}
	}

	// Insert orders (relating users and products)
	for i := 0; i < 100; i++ {
		_, err := orders.Insert(map[string]interface{}{
			"id":         fmt.Sprintf("o%d", i),
			"user_id":    fmt.Sprintf("u%d", i%50),
			"product_id": fmt.Sprintf("p%d", i%20),
			"quantity":   1 + rand.Intn(10),
			"total":      50.0 + float64(i)*3.25,
			"status":     []string{"pending", "shipped", "delivered"}[i%3],
		}).Run(conn)
		if err != nil {
			t.Fatalf("insert order %d: %v", i, err)
		}
	}
	t.Log("Inserted 50 users, 20 products, 100 orders")

	// GET user by ID
	data, err := users.Get("u0").Run(conn)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	var user map[string]interface{}
	json.Unmarshal(data, &user)
	if user["name"] != "User0" {
		t.Errorf("expected User0, got %v", user["name"])
	}

	// UPDATE user
	_, err = users.Get("u0").Update(map[string]interface{}{"age": 99}).Run(conn)
	if err != nil {
		t.Fatalf("update user: %v", err)
	}
	data, _ = users.Get("u0").Run(conn)
	json.Unmarshal(data, &user)
	if user["age"] != float64(99) {
		t.Errorf("expected age=99, got %v", user["age"])
	}

	// DELETE single order
	_, err = orders.Get("o0").Delete().Run(conn)
	if err != nil {
		t.Fatalf("delete order: %v", err)
	}
	data, err = orders.Get("o0").Run(conn)
	if err != nil {
		t.Fatalf("get deleted: %v", err)
	}
	if string(data) != "null" {
		t.Logf("deleted doc: %s (expected null)", string(data))
	}

	// Verify counts
	data, _ = users.GetAll().Run(conn)
	var uDocs []interface{}
	json.Unmarshal(data, &uDocs)
	if len(uDocs) != 50 {
		t.Errorf("expected 50 users, got %d", len(uDocs))
	}

	data, _ = orders.GetAll().Run(conn)
	var oDocs []interface{}
	json.Unmarshal(data, &oDocs)
	if len(oDocs) != 99 {
		t.Errorf("expected 99 orders, got %d", len(oDocs))
	}

	data, _ = products.GetAll().Run(conn)
	var pDocs []interface{}
	json.Unmarshal(data, &pDocs)
	if len(pDocs) != 20 {
		t.Errorf("expected 20 products, got %d", len(pDocs))
	}

	t.Logf("Users=%d, Orders=%d, Products=%d", len(uDocs), len(oDocs), len(pDocs))

	// Cleanup
	cleanupTable(t, r, conn, "mt_users")
	cleanupTable(t, r, conn, "mt_orders")
	cleanupTable(t, r, conn, "mt_products")
}

// ============================================================
// 5. Filter, OrderBy, Limit, Skip
// ============================================================

func TestComprehensive_FilterOrderByLimitSkip(t *testing.T) {
	conn := testConnect(t)
	defer conn.Close()
	r := NewRWithConn(conn)

	table := r.Table("query_test")
	cleanupTable(t, r, conn, "query_test")

	// Insert 100 records
	for i := 0; i < 100; i++ {
		_, err := table.Insert(map[string]interface{}{
			"id":     fmt.Sprintf("q%03d", i),
			"name":   fmt.Sprintf("Item%d", i),
			"city":   []string{"SP", "RJ", "MG", "PR"}[i%4],
			"active": i%2 == 0,
			"score":  float64(i),
			"price":  10.0 + float64(i)*1.5,
		}).Run(conn)
		if err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	// FILTER by active=true
	data, err := table.Filter(map[string]interface{}{"active": true}).Run(conn)
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	var filtered []interface{}
	json.Unmarshal(data, &filtered)
	t.Logf("Filter active=true: %d results", len(filtered))
	if len(filtered) != 50 {
		t.Errorf("expected 50 active, got %d", len(filtered))
	}

	// FILTER by city=SP
	data, err = table.Filter(map[string]interface{}{"city": "SP"}).Run(conn)
	if err != nil {
		t.Fatalf("filter city: %v", err)
	}
	json.Unmarshal(data, &filtered)
	t.Logf("Filter city=SP: %d results", len(filtered))
	if len(filtered) != 25 {
		t.Errorf("expected 25 SP, got %d", len(filtered))
	}

	// ORDER BY score
	data, err = table.OrderBy("score").Run(conn)
	if err != nil {
		t.Fatalf("orderBy: %v", err)
	}
	var ordered []map[string]interface{}
	json.Unmarshal(data, &ordered)
	t.Logf("OrderBy score: %d results", len(ordered))
	if len(ordered) > 1 {
		first := ordered[0]["score"].(float64)
		last := ordered[len(ordered)-1]["score"].(float64)
		if first > last {
			t.Errorf("orderBy failed: first=%.0f last=%.0f", first, last)
		}
	}

	// LIMIT
	data, err = table.Limit(10).Run(conn)
	if err != nil {
		t.Fatalf("limit: %v", err)
	}
	var limited []interface{}
	json.Unmarshal(data, &limited)
	if len(limited) != 10 {
		t.Errorf("expected 10, got %d", len(limited))
	}
	t.Logf("Limit 10: %d results", len(limited))

	// SKIP
	data, err = table.Skip(90).Run(conn)
	if err != nil {
		t.Fatalf("skip: %v", err)
	}
	var skipped []interface{}
	json.Unmarshal(data, &skipped)
	if len(skipped) != 10 {
		t.Errorf("expected 10 after skip 90, got %d", len(skipped))
	}
	t.Logf("Skip 90: %d results", len(skipped))

	// Combined: Filter + OrderBy + Limit
	data, err = table.Filter(map[string]interface{}{"active": true}).OrderBy("score").Limit(5).Run(conn)
	if err != nil {
		t.Fatalf("combined: %v", err)
	}
	var combined []interface{}
	json.Unmarshal(data, &combined)
	t.Logf("Filter+OrderBy+Limit: %d results", len(combined))
	if len(combined) > 5 {
		t.Errorf("expected at most 5, got %d", len(combined))
	}

	cleanupTable(t, r, conn, "query_test")
}

// ============================================================
// 6. Aggregations (Count, Sum, Avg, Min, Max)
// ============================================================

func TestComprehensive_Aggregations(t *testing.T) {
	conn := testConnect(t)
	defer conn.Close()
	r := NewRWithConn(conn)

	table := r.Table("agg_test")
	cleanupTable(t, r, conn, "agg_test")

	// Insert 200 records with numeric fields
	for i := 0; i < 200; i++ {
		_, err := table.Insert(map[string]interface{}{
			"id":    fmt.Sprintf("a%d", i),
			"value": float64(i + 1),
			"group": []string{"A", "B", "C", "D"}[i%4],
		}).Run(conn)
		if err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	// COUNT
	data, err := table.Count().Run(conn)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	var count float64
	json.Unmarshal(data, &count)
	if count != 200 {
		t.Errorf("expected count=200, got %.0f", count)
	}
	t.Logf("Count: %.0f", count)

	// SUM of value
	data, err = table.Sum("value").Run(conn)
	if err != nil {
		t.Fatalf("sum: %v", err)
	}
	var sum float64
	json.Unmarshal(data, &sum)
	expectedSum := float64(200 * 201 / 2) // 1+2+...+200 = 20100
	if sum != expectedSum {
		t.Errorf("expected sum=%.0f, got %.0f", expectedSum, sum)
	}
	t.Logf("Sum(value): %.0f", sum)

	// AVG of value
	data, err = table.Avg("value").Run(conn)
	if err != nil {
		t.Fatalf("avg: %v", err)
	}
	var avg float64
	json.Unmarshal(data, &avg)
	expectedAvg := expectedSum / 200.0
	if avg != expectedAvg {
		t.Errorf("expected avg=%.1f, got %.1f", expectedAvg, avg)
	}
	t.Logf("Avg(value): %.1f", avg)

	// MIN of value
	data, err = table.Min("value").Run(conn)
	if err != nil {
		t.Fatalf("min: %v", err)
	}
	var minVal float64
	json.Unmarshal(data, &minVal)
	if minVal != 1 {
		t.Errorf("expected min=1, got %.0f", minVal)
	}
	t.Logf("Min(value): %.0f", minVal)

	// MAX of value
	data, err = table.Max("value").Run(conn)
	if err != nil {
		t.Fatalf("max: %v", err)
	}
	var maxVal float64
	json.Unmarshal(data, &maxVal)
	if maxVal != 200 {
		t.Errorf("expected max=200, got %.0f", maxVal)
	}
	t.Logf("Max(value): %.0f", maxVal)

	// GROUP by group field
	data, err = table.Group("group").Run(conn)
	if err != nil {
		t.Fatalf("group: %v", err)
	}
	var groups []interface{}
	json.Unmarshal(data, &groups)
	t.Logf("Group(group): %d groups", len(groups))
	if len(groups) != 4 {
		t.Errorf("expected 4 groups, got %d", len(groups))
	}

	cleanupTable(t, r, conn, "agg_test")
}

// ============================================================
// 7. Index Operations + Between
// ============================================================

func TestComprehensive_IndexAndBetween(t *testing.T) {
	conn := testConnect(t)
	defer conn.Close()
	r := NewRWithConn(conn)

	table := r.Table("idx_test")
	cleanupTable(t, r, conn, "idx_test")

	// Insert 100 records
	for i := 0; i < 100; i++ {
		_, err := table.Insert(map[string]interface{}{
			"id":   fmt.Sprintf("i%03d", i),
			"name": fmt.Sprintf("Item%d", i),
			"age":  fmt.Sprintf("%d", 18+i%50),
		}).Run(conn)
		if err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	// Create index on "age"
	data, err := table.IndexCreate("age_idx", "age").Run(conn)
	if err != nil {
		t.Fatalf("index create: %v", err)
	}
	t.Logf("IndexCreate: %s", string(data))

	// List indexes
	data, err = table.IndexList().Run(conn)
	if err != nil {
		t.Fatalf("index list: %v", err)
	}
	var indexes []string
	json.Unmarshal(data, &indexes)
	t.Logf("IndexList: %v", indexes)
	found := false
	for _, idx := range indexes {
		if idx == "age_idx" {
			found = true
			break
		}
	}
	if !found {
		t.Error("age_idx not found in index list")
	}

	// BETWEEN using index
	data, err = table.Between("age_idx", "20", "30").Run(conn)
	if err != nil {
		t.Logf("Between (may need index): %v", err)
	} else {
		var between []interface{}
		json.Unmarshal(data, &between)
		t.Logf("Between 20-30: %d results", len(between))
	}

	// Drop index
	data, err = table.IndexDrop("age_idx").Run(conn)
	if err != nil {
		t.Fatalf("index drop: %v", err)
	}
	t.Logf("IndexDrop: %s", string(data))

	cleanupTable(t, r, conn, "idx_test")
}

// ============================================================
// 8. Projection (HasFields, Without)
// ============================================================

func TestComprehensive_Projection(t *testing.T) {
	conn := testConnect(t)
	defer conn.Close()
	r := NewRWithConn(conn)

	table := r.Table("proj_test")
	cleanupTable(t, r, conn, "proj_test")

	// Insert records with varying fields
	for i := 0; i < 10; i++ {
		doc := map[string]interface{}{
			"id":    fmt.Sprintf("pr%d", i),
			"name":  fmt.Sprintf("Name%d", i),
			"email": fmt.Sprintf("email%d@test.com", i),
		}
		if i%2 == 0 {
			doc["phone"] = "111-1111"
		}
		_, err := table.Insert(doc).Run(conn)
		if err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	// HasFields: only docs with "phone"
	data, err := table.HasFields("phone").Run(conn)
	if err != nil {
		t.Fatalf("hasFields: %v", err)
	}
	var hasPhone []interface{}
	json.Unmarshal(data, &hasPhone)
	t.Logf("HasFields(phone): %d results", len(hasPhone))
	if len(hasPhone) != 5 {
		t.Errorf("expected 5 with phone, got %d", len(hasPhone))
	}

	// Without: remove "email" field
	data, err = table.Without("email").Run(conn)
	if err != nil {
		t.Fatalf("without: %v", err)
	}
	var withoutEmail []map[string]interface{}
	json.Unmarshal(data, &withoutEmail)
	t.Logf("Without(email): %d results", len(withoutEmail))
	if len(withoutEmail) > 0 {
		if _, ok := withoutEmail[0]["email"]; ok {
			t.Error("email field should have been removed")
		}
	}

	cleanupTable(t, r, conn, "proj_test")
}

// ============================================================
// 9. Cross-Table Join
// ============================================================

func TestComprehensive_Join(t *testing.T) {
	conn := testConnect(t)
	defer conn.Close()
	r := NewRWithConn(conn)

	users := r.Table("join_users")
	ordersT := r.Table("join_orders")
	cleanupTable(t, r, conn, "join_users")
	cleanupTable(t, r, conn, "join_orders")

	// Insert users
	for i := 0; i < 10; i++ {
		users.Insert(map[string]interface{}{
			"id":      fmt.Sprintf("ju%d", i),
			"user_id": fmt.Sprintf("uid%d", i),
			"name":    fmt.Sprintf("User%d", i),
			"city":    "SP",
		}).Run(conn)
	}

	// Insert orders with matching user_id
	for i := 0; i < 15; i++ {
		ordersT.Insert(map[string]interface{}{
			"id":      fmt.Sprintf("jo%d", i),
			"user_id": fmt.Sprintf("uid%d", i%10),
			"product": fmt.Sprintf("Prod%d", i%5),
			"amount":  float64(100 + i*10),
		}).Run(conn)
	}

	// Inner join on user_id
	data, err := users.InnerJoin(ordersT.Query, map[string]interface{}{"match": "user_id"}).Run(conn)
	if err != nil {
		t.Logf("InnerJoin: %v", err)
	} else {
		var joined []interface{}
		json.Unmarshal(data, &joined)
		t.Logf("InnerJoin: %d results", len(joined))
	}

	// Outer join
	data, err = users.OuterJoin(ordersT.Query, map[string]interface{}{"match": "user_id"}).Run(conn)
	if err != nil {
		t.Logf("OuterJoin: %v", err)
	} else {
		var joined []interface{}
		json.Unmarshal(data, &joined)
		t.Logf("OuterJoin: %d results", len(joined))
	}

	cleanupTable(t, r, conn, "join_users")
	cleanupTable(t, r, conn, "join_orders")
}

// ============================================================
// 10. Connection Pool Control
// ============================================================

func TestComprehensive_PoolControl(t *testing.T) {
	pool, err := NewPool(PoolOptions{
		Host:     envOrDefault("GOTHINKDB_HOST", "localhost"),
		Port:     envOrDefaultInt("GOTHINKDB_PORT", 28015),
		DB:       "test",
		User:     "admin",
		MaxConns: 10,
		MinConns: 3,
	})
	if err != nil {
		t.Fatalf("pool create: %v", err)
	}
	defer pool.Close()

	// Check initial stats
	stats := pool.Stats()
	t.Logf("Initial pool stats: total=%d idle=%d inUse=%d max=%d",
		stats.TotalConns, stats.IdleConns, stats.InUseConns, stats.MaxConns)
	if stats.TotalConns != 3 {
		t.Errorf("expected 3 initial conns, got %d", stats.TotalConns)
	}

	// Acquire multiple connections
	conns := make([]*Conn, 0)
	for i := 0; i < 5; i++ {
		c, err := pool.Acquire()
		if err != nil {
			t.Fatalf("acquire %d: %v", i, err)
		}
		conns = append(conns, c)
	}

	stats = pool.Stats()
	t.Logf("After 5 acquires: total=%d idle=%d inUse=%d", stats.TotalConns, stats.IdleConns, stats.InUseConns)
	if stats.InUseConns != 5 {
		t.Errorf("expected 5 in-use, got %d", stats.InUseConns)
	}

	// Use each connection for a query
	r := NewR()
	for i, c := range conns {
		_, err := r.DBList().Run(c)
		if err != nil {
			t.Errorf("query on conn %d: %v", i, err)
		}
	}

	// Release all
	for _, c := range conns {
		pool.Release(c)
	}

	stats = pool.Stats()
	if stats.InUseConns != 0 {
		t.Errorf("expected 0 in-use after release, got %d", stats.InUseConns)
	}

	// Test pool exhaustion
	for i := 0; i < 10; i++ {
		_, err := pool.Acquire()
		if err != nil {
			t.Fatalf("acquire %d: %v", i, err)
		}
	}
	_, err = pool.Acquire()
	if err == nil {
		t.Error("expected pool exhaustion error")
	} else {
		t.Logf("Pool exhaustion correctly detected: %v", err)
	}

	t.Log("Pool control test completed")
}

// ============================================================
// 11. Concurrent Pool Operations
// ============================================================

func TestComprehensive_ConcurrentPoolOps(t *testing.T) {
	pool, err := NewPool(PoolOptions{
		Host:     envOrDefault("GOTHINKDB_HOST", "localhost"),
		Port:     envOrDefaultInt("GOTHINKDB_PORT", 28015),
		DB:       "test",
		User:     "admin",
		MaxConns: 10,
		MinConns: 3,
	})
	if err != nil {
		t.Fatalf("pool create: %v", err)
	}
	defer pool.Close()

	r := NewR()
	table := r.Table("concurrent_test")

	// Insert 100 records using pool
	for i := 0; i < 100; i++ {
		err := pool.Exec(func(conn *Conn) error {
			_, err := table.Insert(map[string]interface{}{
				"id":    fmt.Sprintf("c%d", i),
				"value": float64(i),
			}).Run(conn)
			return err
		})
		if err != nil {
			t.Fatalf("pool insert %d: %v", i, err)
		}
	}

	// Concurrent reads
	var wg sync.WaitGroup
	errCh := make(chan error, 50)
	start := time.Now()

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := pool.Exec(func(conn *Conn) error {
				_, err := table.Get(fmt.Sprintf("c%d", idx%100)).Run(conn)
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

	t.Logf("50 concurrent reads in %v, errors: %d", time.Since(start), errCount)

	stats := pool.Stats()
	t.Logf("Pool stats after concurrent: total=%d idle=%d inUse=%d created=%d",
		stats.TotalConns, stats.IdleConns, stats.InUseConns, stats.TotalCreated)

	// Cleanup
	pool.Exec(func(conn *Conn) error {
		r2 := NewRWithConn(conn)
		r2.Table("concurrent_test").Delete().Run(conn)
		return nil
	})
}

// ============================================================
// 12. Status and Info
// ============================================================

func TestComprehensive_StatusInfo(t *testing.T) {
	conn := testConnect(t)
	defer conn.Close()
	r := NewRWithConn(conn)

	// Status
	data, err := NewQuery([]interface{}{137}, conn).Run(conn)
	if err != nil {
		t.Logf("Status: %v", err)
	} else {
		t.Logf("Status: %s", string(data))
	}

	// Info
	data, err = NewQuery([]interface{}{138}, conn).Run(conn)
	if err != nil {
		t.Logf("Info: %v", err)
	} else {
		t.Logf("Info: %s", string(data))
	}

	_ = r
}

// ============================================================
// Main test runner
// ============================================================

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
