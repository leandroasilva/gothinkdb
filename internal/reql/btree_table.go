package reql

import (
	"context"
	"fmt"

	"github.com/leandroasilva/gothinkdb/internal/btree"
	"github.com/leandroasilva/gothinkdb/internal/storage"
	"github.com/leandroasilva/gothinkdb/pkg/datum"
)

// BTreeTable represents a table backed by a B-Tree
type BTreeTable struct {
	Name        string
	tree        *btree.BTree
	cache       *storage.PageCache
	Indexes     *IndexManager
	Changefeeds []*Changefeed
}

// NewBTreeTable creates a new B-Tree backed table
func NewBTreeTable(name string, cache *storage.PageCache) (*BTreeTable, error) {
	tree, err := btree.New(cache)
	if err != nil {
		return nil, fmt.Errorf("failed to create B-Tree: %w", err)
	}

	return &BTreeTable{
		Name:        name,
		tree:        tree,
		cache:       cache,
		Indexes:     NewIndexManager(),
		Changefeeds: make([]*Changefeed, 0),
	}, nil
}

// LoadBTreeTable loads an existing B-Tree table
func LoadBTreeTable(name string, cache *storage.PageCache, rootID storage.BlockID) *BTreeTable {
	tree := btree.Load(cache, rootID)
	return &BTreeTable{
		Name:        name,
		tree:        tree,
		cache:       cache,
		Indexes:     NewIndexManager(),
		Changefeeds: make([]*Changefeed, 0),
	}
}

// Insert inserts a document into the table
func (t *BTreeTable) Insert(ctx context.Context, id string, doc datum.Datum) error {
	key := []byte(id)

	// Serialize document to JSON
	data, err := doc.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize document: %w", err)
	}

	// Insert into B-Tree
	if err := t.tree.Put(ctx, key, data); err != nil {
		return fmt.Errorf("failed to insert into B-Tree: %w", err)
	}

	// Update indexes
	for _, indexName := range t.Indexes.ListIndexes() {
		idx, _ := t.Indexes.GetIndex(indexName)
		if doc.Type() == datum.Object {
			if value, exists := doc.Object().Get(idx.Field); exists {
				idx.Add(id, value)
			}
		}
	}

	// Notify changefeeds
	t.notifyChangefeeds(datum.Datum{}, doc)

	return nil
}

// Get retrieves a document by ID
func (t *BTreeTable) Get(ctx context.Context, id string) (datum.Datum, error) {
	key := []byte(id)

	data, err := t.tree.Get(ctx, key)
	if err != nil {
		return datum.Datum{}, fmt.Errorf("failed to get from B-Tree: %w", err)
	}

	if data == nil {
		return datum.NewNull(), nil
	}

	// Deserialize document from JSON
	var doc datum.Datum
	if err := doc.UnmarshalJSON(data); err != nil {
		return datum.Datum{}, fmt.Errorf("failed to deserialize document: %w", err)
	}

	return doc, nil
}

// Update updates a document in the table
func (t *BTreeTable) Update(ctx context.Context, id string, updates datum.Datum) error {
	// Get old document
	oldDoc, err := t.Get(ctx, id)
	if err != nil {
		return err
	}

	if oldDoc.IsNull() {
		return fmt.Errorf("document %s not found", id)
	}

	// Apply updates
	if oldDoc.Type() == datum.Object && updates.Type() == datum.Object {
		for _, key := range updates.Object().Keys() {
			val, _ := updates.Object().Get(key)
			oldDoc.Object().Set(key, val)
		}
	}

	// Update indexes
	for _, indexName := range t.Indexes.ListIndexes() {
		idx, _ := t.Indexes.GetIndex(indexName)
		if oldDoc.Type() == datum.Object {
			oldValue, oldExists := oldDoc.Object().Get(idx.Field)
			newValue, newExists := datum.Datum{}, false
			if updates.Type() == datum.Object {
				newValue, newExists = updates.Object().Get(idx.Field)
			}

			if oldExists && newExists {
				idx.Update(id, oldValue, newValue)
			} else if oldExists {
				idx.Remove(id, oldValue)
			} else if newExists {
				idx.Add(id, newValue)
			}
		}
	}

	// Write back to B-Tree
	data, err := oldDoc.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize document: %w", err)
	}

	if err := t.tree.Put(ctx, []byte(id), data); err != nil {
		return fmt.Errorf("failed to update in B-Tree: %w", err)
	}

	// Notify changefeeds
	t.notifyChangefeeds(datum.Datum{}, oldDoc)

	return nil
}

// Delete deletes a document from the table
func (t *BTreeTable) Delete(ctx context.Context, id string) error {
	// Get old document for changefeed
	oldDoc, err := t.Get(ctx, id)
	if err != nil {
		return err
	}

	// Remove from indexes
	if !oldDoc.IsNull() && oldDoc.Type() == datum.Object {
		for _, indexName := range t.Indexes.ListIndexes() {
			idx, _ := t.Indexes.GetIndex(indexName)
			if value, exists := oldDoc.Object().Get(idx.Field); exists {
				idx.Remove(id, value)
			}
		}
	}

	// Delete from B-Tree
	if err := t.tree.Delete(ctx, []byte(id)); err != nil {
		return fmt.Errorf("failed to delete from B-Tree: %w", err)
	}

	// Notify changefeeds
	if !oldDoc.IsNull() {
		t.notifyChangefeeds(oldDoc, datum.Datum{})
	}

	return nil
}

// GetAll retrieves all documents from the table
func (t *BTreeTable) GetAll(ctx context.Context) ([]datum.Datum, error) {
	// Scan entire B-Tree
	kvPairs, err := t.tree.Scan(ctx, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to scan B-Tree: %w", err)
	}

	docs := make([]datum.Datum, 0, len(kvPairs))
	for _, kv := range kvPairs {
		var doc datum.Datum
		if err := doc.UnmarshalJSON(kv.Value); err != nil {
			return nil, fmt.Errorf("failed to deserialize document: %w", err)
		}
		docs = append(docs, doc)
	}

	return docs, nil
}

// RootID returns the root block ID of the B-Tree
func (t *BTreeTable) RootID() storage.BlockID {
	return t.tree.RootID()
}

// notifyChangefeeds notifies all changefeeds of a change
func (t *BTreeTable) notifyChangefeeds(oldDoc, newDoc datum.Datum) {
	event := ChangeEvent{
		OldValue: oldDoc,
		NewValue: newDoc,
	}

	for _, cf := range t.Changefeeds {
		cf.Send(event)
	}
}
