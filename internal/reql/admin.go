package reql

import (
	"fmt"
	"sort"
	"sync"

	"github.com/leandroasilva/gothinkdb/pkg/datum"
)

// Database represents a database containing tables
type Database struct {
	Name   string
	Tables map[string]*Table
	mu     sync.RWMutex
}

// NewDatabase creates a new database
func NewDatabase(name string) *Database {
	return &Database{
		Name:   name,
		Tables: make(map[string]*Table),
	}
}

// AdminManager manages databases and server configuration
type AdminManager struct {
	databases map[string]*Database
	config    map[string]interface{}
	mu        sync.RWMutex
}

// NewAdminManager creates a new admin manager
func NewAdminManager() *AdminManager {
	am := &AdminManager{
		databases: make(map[string]*Database),
		config: map[string]interface{}{
			"server_name": "gothinkdb-server",
			"version":     "0.1.0",
		},
	}

	// Create default database
	am.databases["test"] = NewDatabase("test")

	return am
}

// CreateDatabase creates a new database
func (am *AdminManager) CreateDatabase(name string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.databases[name]; exists {
		return fmt.Errorf("database %s already exists", name)
	}

	am.databases[name] = NewDatabase(name)
	return nil
}

// DropDatabase drops a database
func (am *AdminManager) DropDatabase(name string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.databases[name]; !exists {
		return fmt.Errorf("database %s does not exist", name)
	}

	if name == "test" {
		return fmt.Errorf("cannot drop default database")
	}

	delete(am.databases, name)
	return nil
}

// ListDatabases lists all databases
func (am *AdminManager) ListDatabases() []string {
	am.mu.RLock()
	defer am.mu.RUnlock()

	names := make([]string, 0, len(am.databases))
	for name := range am.databases {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetDatabase gets a database by name
func (am *AdminManager) GetDatabase(name string) (*Database, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	db, ok := am.databases[name]
	return db, ok
}

// CreateTable creates a new table in a database
func (am *AdminManager) CreateTable(dbName, tableName string) error {
	am.mu.RLock()
	db, exists := am.databases[dbName]
	am.mu.RUnlock()

	if !exists {
		return fmt.Errorf("database %s does not exist", dbName)
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.Tables[tableName]; exists {
		return fmt.Errorf("table %s already exists in database %s", tableName, dbName)
	}

	// Create placeholder - will be replaced with evaluator's table via SyncTableRef
	db.Tables[tableName] = &Table{
		Name:        tableName,
		Data:        make(map[string]datum.Datum),
		Indexes:     NewIndexManager(),
		Changefeeds: make([]*Changefeed, 0),
	}

	return nil
}

// SyncTableRef replaces the admin's table placeholder with the evaluator's actual table instance.
// This ensures data written via the evaluator is visible through the admin API.
// It also removes the table from other databases to prevent duplicates.
func (am *AdminManager) SyncTableRef(dbName, tableName string, evaluatorTable *Table) bool {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Remove from all other databases first
	for name, db := range am.databases {
		if name != dbName {
			db.mu.Lock()
			delete(db.Tables, tableName)
			db.mu.Unlock()
		}
	}

	// Set in the target database
	db, exists := am.databases[dbName]
	if !exists {
		return false
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	db.Tables[tableName] = evaluatorTable
	return true
}

// DropTable drops a table from a database
func (am *AdminManager) DropTable(dbName, tableName string) error {
	am.mu.RLock()
	db, exists := am.databases[dbName]
	am.mu.RUnlock()

	if !exists {
		return fmt.Errorf("database %s does not exist", dbName)
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.Tables[tableName]; !exists {
		return fmt.Errorf("table %s does not exist in database %s", tableName, dbName)
	}

	delete(db.Tables, tableName)
	return nil
}

// ListTables lists all tables in a database
func (am *AdminManager) ListTables(dbName string) ([]string, error) {
	am.mu.RLock()
	db, exists := am.databases[dbName]
	am.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("database %s does not exist", dbName)
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	names := make([]string, 0, len(db.Tables))
	for name := range db.Tables {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// GetTable gets a table from a database
func (am *AdminManager) GetTable(dbName, tableName string) (*Table, error) {
	am.mu.RLock()
	db, exists := am.databases[dbName]
	am.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("database %s does not exist", dbName)
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	table, exists := db.Tables[tableName]
	if !exists {
		return nil, fmt.Errorf("table %s does not exist in database %s", tableName, dbName)
	}

	return table, nil
}

// DatabaseSizeBytes returns the approximate on-memory size of a database by
// summing the JSON-encoded length of every document in every table (plus the
// document key). It is used by the HTTP API to report per-database usage for
// quota enforcement; the value is an approximation, not a precise disk footprint.
func (am *AdminManager) DatabaseSizeBytes(dbName string) (int64, error) {
	am.mu.RLock()
	db, exists := am.databases[dbName]
	am.mu.RUnlock()
	if !exists {
		return 0, fmt.Errorf("database %s does not exist", dbName)
	}

	db.mu.RLock()
	tables := make([]*Table, 0, len(db.Tables))
	for _, t := range db.Tables {
		tables = append(tables, t)
	}
	db.mu.RUnlock()

	var total int64
	for _, t := range tables {
		t.RLock()
		for key, doc := range t.Data {
			total += int64(len(key))
			if b, err := doc.MarshalJSON(); err == nil {
				total += int64(len(b))
			}
		}
		t.RUnlock()
	}
	return total, nil
}

// GetConfig gets server configuration
func (am *AdminManager) GetConfig() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	config := make(map[string]interface{})
	for k, v := range am.config {
		config[k] = v
	}
	return config
}

// SetConfig sets a configuration value
func (am *AdminManager) SetConfig(key string, value interface{}) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.config[key] = value
}

// GetServerStatus gets server status information
func (am *AdminManager) GetServerStatus() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	status := map[string]interface{}{
		"id":        "gothinkdb-server",
		"name":      am.config["server_name"],
		"version":   am.config["version"],
		"databases": len(am.databases),
	}

	// Count total tables
	totalTables := 0
	for _, db := range am.databases {
		totalTables += len(db.Tables)
	}
	status["tables"] = totalTables

	return status
}
