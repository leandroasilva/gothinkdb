package btree

import (
	"bytes"
	"context"
	"fmt"

	"github.com/leandroasilva/gothinkdb/internal/storage"
)

// BTree represents a B-Tree data structure
type BTree struct {
	cache  *storage.PageCache
	rootID storage.BlockID
}

// New creates a new B-Tree
func New(cache *storage.PageCache) (*BTree, error) {
	ctx := context.Background()

	// Create root leaf node
	rootNode := NewLeafNode(storage.InvalidBlockID)
	data := rootNode.Serialize()

	// Allocate block and write data
	block := storage.NewBlockFromData(data)
	rootID, err := cache.Allocate(ctx, block)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate root: %w", err)
	}

	return &BTree{
		cache:  cache,
		rootID: rootID,
	}, nil
}

// Load loads an existing B-Tree from storage
func Load(cache *storage.PageCache, rootID storage.BlockID) *BTree {
	return &BTree{
		cache:  cache,
		rootID: rootID,
	}
}

// Get retrieves a value by key
func (t *BTree) Get(ctx context.Context, key []byte) ([]byte, error) {
	return t.get(ctx, t.rootID, key)
}

func (t *BTree) get(ctx context.Context, nodeID storage.BlockID, key []byte) ([]byte, error) {
	block, err := t.cache.Get(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	// Determine node type
	if block[0] == byte(NodeTypeLeaf) {
		node, err := DeserializeLeaf(nodeID, block[:])
		if err != nil {
			return nil, err
		}
		value, found := node.Get(key)
		if !found {
			return nil, nil
		}
		return value, nil
	}

	// Internal node
	node, err := DeserializeInternal(nodeID, block[:])
	if err != nil {
		return nil, err
	}

	childIdx := node.FindChild(key)
	return t.get(ctx, node.children[childIdx], key)
}

// Put inserts or updates a key-value pair
func (t *BTree) Put(ctx context.Context, key, value []byte) error {
	newRoot, _, err := t.put(ctx, t.rootID, key, value)
	if err != nil {
		return err
	}

	if newRoot != storage.InvalidBlockID {
		// The root was split, update root ID
		t.rootID = newRoot
	}

	return nil
}

func (t *BTree) put(ctx context.Context, nodeID storage.BlockID, key, value []byte) (storage.BlockID, []byte, error) {
	block, err := t.cache.Get(ctx, nodeID)
	if err != nil {
		return storage.InvalidBlockID, nil, err
	}

	// Leaf node
	if block[0] == byte(NodeTypeLeaf) {
		node, err := DeserializeLeaf(nodeID, block[:])
		if err != nil {
			return storage.InvalidBlockID, nil, err
		}

		if err := node.Insert(key, value); err != nil {
			return storage.InvalidBlockID, nil, err
		}

		// Write back
		data := node.Serialize()
		newBlock := storage.NewBlockFromData(data)
		if err := t.cache.Put(ctx, nodeID, newBlock); err != nil {
			return storage.InvalidBlockID, nil, err
		}

		// Split if full
		if node.IsFull() {
			rightID, separator, err := t.splitLeaf(ctx, node)
			if err != nil {
				return storage.InvalidBlockID, nil, err
			}

			// If this is the root, create a new root
			if nodeID == t.rootID {
				newRootID, err := t.createRoot(ctx, node.blockID, rightID, separator)
				if err != nil {
					return storage.InvalidBlockID, nil, err
				}
				return newRootID, nil, nil
			}

			return rightID, separator, nil
		}

		return storage.InvalidBlockID, nil, nil
	}

	// Internal node
	node, err := DeserializeInternal(nodeID, block[:])
	if err != nil {
		return storage.InvalidBlockID, nil, err
	}

	childIdx := node.FindChild(key)
	newChild, childSeparator, err := t.put(ctx, node.children[childIdx], key, value)
	if err != nil {
		return storage.InvalidBlockID, nil, err
	}

	if newChild != storage.InvalidBlockID {
		// Child was split, insert new child with the separator
		if err := node.Insert(childSeparator, newChild); err != nil {
			return storage.InvalidBlockID, nil, err
		}

		// Write back
		data := node.Serialize()
		newBlock := storage.NewBlockFromData(data)
		if err := t.cache.Put(ctx, nodeID, newBlock); err != nil {
			return storage.InvalidBlockID, nil, err
		}

		// Split if full
		if node.IsFull() {
			rightID, separator, err := t.splitInternal(ctx, node)
			if err != nil {
				return storage.InvalidBlockID, nil, err
			}

			// If this is the root, create a new root
			if nodeID == t.rootID {
				newRootID, err := t.createRoot(ctx, node.blockID, rightID, separator)
				if err != nil {
					return storage.InvalidBlockID, nil, err
				}
				return newRootID, nil, nil
			}

			return rightID, separator, nil
		}
	}

	return storage.InvalidBlockID, nil, nil
}

// createRoot creates a new root node when the old root splits
func (t *BTree) createRoot(ctx context.Context, leftID, rightID storage.BlockID, separator []byte) (storage.BlockID, error) {
	// Create a new internal node as the root
	newRoot := NewInternalNode(storage.InvalidBlockID)
	newRoot.children = append(newRoot.children, leftID, rightID)
	newRoot.keys = append(newRoot.keys, separator)
	newRoot.numKeys = 1

	// Serialize and allocate
	data := newRoot.Serialize()
	block := storage.NewBlockFromData(data)
	rootID, err := t.cache.Allocate(ctx, block)
	if err != nil {
		return storage.InvalidBlockID, err
	}

	return rootID, nil
}

func (t *BTree) splitLeaf(ctx context.Context, node *LeafNode) (storage.BlockID, []byte, error) {
	// Allocate new block for right node
	rightBlock := storage.NewBlock()
	rightID, err := t.cache.Allocate(ctx, rightBlock)
	if err != nil {
		return storage.InvalidBlockID, nil, err
	}

	// Split the node
	right := node.Split(rightID)

	// Get the separator key (first key of right node)
	separator := make([]byte, len(right.keys[0]))
	copy(separator, right.keys[0])

	// Write both nodes
	leftData := node.Serialize()
	leftBlock := storage.NewBlockFromData(leftData)
	if err := t.cache.Put(ctx, node.blockID, leftBlock); err != nil {
		return storage.InvalidBlockID, nil, err
	}

	rightData := right.Serialize()
	rightBlock = storage.NewBlockFromData(rightData)
	if err := t.cache.Put(ctx, rightID, rightBlock); err != nil {
		return storage.InvalidBlockID, nil, err
	}

	return rightID, separator, nil
}

func (t *BTree) splitInternal(ctx context.Context, node *InternalNode) (storage.BlockID, []byte, error) {
	// Allocate new block for right node
	rightBlock := storage.NewBlock()
	rightID, err := t.cache.Allocate(ctx, rightBlock)
	if err != nil {
		return storage.InvalidBlockID, nil, err
	}

	// Split the node
	right, separator := node.Split(rightID)

	// Write both nodes
	leftData := node.Serialize()
	leftBlock := storage.NewBlockFromData(leftData)
	if err := t.cache.Put(ctx, node.blockID, leftBlock); err != nil {
		return storage.InvalidBlockID, nil, err
	}

	rightData := right.Serialize()
	rightBlock = storage.NewBlockFromData(rightData)
	if err := t.cache.Put(ctx, rightID, rightBlock); err != nil {
		return storage.InvalidBlockID, nil, err
	}

	return rightID, separator, nil
}

// Delete removes a key from the B-Tree
func (t *BTree) Delete(ctx context.Context, key []byte) error {
	// For simplicity, we'll just mark the value as deleted
	// A full implementation would handle rebalancing
	return t.delete(ctx, t.rootID, key)
}

func (t *BTree) delete(ctx context.Context, nodeID storage.BlockID, key []byte) error {
	block, err := t.cache.Get(ctx, nodeID)
	if err != nil {
		return err
	}

	if block[0] == byte(NodeTypeLeaf) {
		node, err := DeserializeLeaf(nodeID, block[:])
		if err != nil {
			return err
		}

		if !node.Delete(key) {
			return nil // Key not found
		}

		// Write back
		data := node.Serialize()
		newBlock := storage.NewBlockFromData(data)
		return t.cache.Put(ctx, nodeID, newBlock)
	}

	// Internal node - find child and recurse
	node, err := DeserializeInternal(nodeID, block[:])
	if err != nil {
		return err
	}

	childIdx := node.FindChild(key)
	return t.delete(ctx, node.children[childIdx], key)
}

// Scan performs a range scan
func (t *BTree) Scan(ctx context.Context, startKey, endKey []byte) ([]KeyValue, error) {
	var results []KeyValue
	err := t.scan(ctx, t.rootID, startKey, endKey, &results)
	return results, err
}

// KeyValue represents a key-value pair
type KeyValue struct {
	Key   []byte
	Value []byte
}

func (t *BTree) scan(ctx context.Context, nodeID storage.BlockID, startKey, endKey []byte, results *[]KeyValue) error {
	block, err := t.cache.Get(ctx, nodeID)
	if err != nil {
		return err
	}

	if block[0] == byte(NodeTypeLeaf) {
		node, err := DeserializeLeaf(nodeID, block[:])
		if err != nil {
			return err
		}

		// Find start position
		startPos := node.findKey(startKey)

		// Iterate through keys in range
		for i := startPos; i < len(node.keys); i++ {
			if endKey != nil && bytes.Compare(node.keys[i], endKey) > 0 {
				break
			}
			*results = append(*results, KeyValue{
				Key:   node.keys[i],
				Value: node.values[i],
			})
		}

		return nil
	}

	// Internal node
	node, err := DeserializeInternal(nodeID, block[:])
	if err != nil {
		return err
	}

	startIdx := node.FindChild(startKey)

	// Scan children in range
	for i := startIdx; i < len(node.children); i++ {
		if err := t.scan(ctx, node.children[i], startKey, endKey, results); err != nil {
			return err
		}
	}

	return nil
}

// RootID returns the root block ID
func (t *BTree) RootID() storage.BlockID {
	return t.rootID
}
