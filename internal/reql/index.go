package reql

import (
	"fmt"
	"sort"
	"sync"

	"github.com/leandroasilva/gothinkdb/pkg/datum"
)

// Index represents a secondary index on a table
type Index struct {
	Name      string
	Field     string
	Data      map[string][]string // index value -> list of document IDs
	mu        sync.RWMutex
}

// NewIndex creates a new secondary index
func NewIndex(name, field string) *Index {
	return &Index{
		Name:  name,
		Field: field,
		Data:  make(map[string][]string),
	}
}

// Add adds a document to the index
func (idx *Index) Add(docID string, value datum.Datum) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	key := value.String()
	idx.Data[key] = append(idx.Data[key], docID)
}

// Remove removes a document from the index
func (idx *Index) Remove(docID string, value datum.Datum) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	key := value.String()
	if ids, ok := idx.Data[key]; ok {
		for i, id := range ids {
			if id == docID {
				idx.Data[key] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
		if len(idx.Data[key]) == 0 {
			delete(idx.Data, key)
		}
	}
}

// Update updates a document in the index
func (idx *Index) Update(docID string, oldValue, newValue datum.Datum) {
	idx.Remove(docID, oldValue)
	idx.Add(docID, newValue)
}

// Get retrieves document IDs by index value
func (idx *Index) Get(value datum.Datum) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	key := value.String()
	if ids, ok := idx.Data[key]; ok {
		result := make([]string, len(ids))
		copy(result, ids)
		return result
	}
	return nil
}

// Between retrieves document IDs in a range
func (idx *Index) Between(lower, upper datum.Datum) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var result []string
	lowerKey := lower.String()
	upperKey := upper.String()

	// Get all keys and sort them
	keys := make([]string, 0, len(idx.Data))
	for k := range idx.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Find keys in range
	for _, k := range keys {
		if k >= lowerKey && k <= upperKey {
			result = append(result, idx.Data[k]...)
		}
	}

	return result
}

// GetAll retrieves all document IDs in the index
func (idx *Index) GetAll() []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var result []string
	for _, ids := range idx.Data {
		result = append(result, ids...)
	}
	return result
}

// IndexManager manages indexes for a table
type IndexManager struct {
	indexes map[string]*Index
	mu      sync.RWMutex
}

// NewIndexManager creates a new index manager
func NewIndexManager() *IndexManager {
	return &IndexManager{
		indexes: make(map[string]*Index),
	}
}

// CreateIndex creates a new index
func (im *IndexManager) CreateIndex(name, field string) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	if _, exists := im.indexes[name]; exists {
		return fmt.Errorf("index %s already exists", name)
	}

	im.indexes[name] = NewIndex(name, field)
	return nil
}

// DropIndex drops an index
func (im *IndexManager) DropIndex(name string) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	if _, exists := im.indexes[name]; !exists {
		return fmt.Errorf("index %s does not exist", name)
	}

	delete(im.indexes, name)
	return nil
}

// GetIndex gets an index by name
func (im *IndexManager) GetIndex(name string) (*Index, bool) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	idx, ok := im.indexes[name]
	return idx, ok
}

// ListIndexes lists all indexes
func (im *IndexManager) ListIndexes() []string {
	im.mu.RLock()
	defer im.mu.RUnlock()

	names := make([]string, 0, len(im.indexes))
	for name := range im.indexes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// BuildIndex builds an index from existing data
func (im *IndexManager) BuildIndex(name string, data map[string]datum.Datum) error {
	im.mu.RLock()
	idx, ok := im.indexes[name]
	im.mu.RUnlock()

	if !ok {
		return fmt.Errorf("index %s does not exist", name)
	}

	// Build the index
	for docID, doc := range data {
		if doc.Type() == datum.Object {
			if value, exists := doc.Object().Get(idx.Field); exists {
				idx.Add(docID, value)
			}
		}
	}

	return nil
}
