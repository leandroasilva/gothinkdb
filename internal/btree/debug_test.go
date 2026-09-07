package btree

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/leandroasilva/gothinkdb/internal/storage"
)

// TestSimpleBTree tests a simple insert and get operation
func TestSimpleBTree(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gothinkdb-simple-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	config := &storage.SerializerConfig{
		DataDir:     tmpDir,
		CacheSizeMB: 1,
	}

	ctx := context.Background()

	serializer, err := storage.NewFileSerializer(config)
	if err != nil {
		t.Fatalf("failed to create serializer: %v", err)
	}
	defer serializer.Close()

	cache := storage.NewPageCache(serializer, 100)
	defer cache.Close()

	tree, err := New(cache)
	if err != nil {
		t.Fatalf("failed to create tree: %v", err)
	}

	// Insert a single key-value pair
	key := []byte("testkey")
	value := []byte("testvalue")

	t.Logf("Inserting key=%s, value=%s", key, value)
	if err := tree.Put(ctx, key, value); err != nil {
		t.Fatalf("failed to put: %v", err)
	}

	// Flush cache to ensure data is persisted
	if err := cache.Flush(ctx); err != nil {
		t.Fatalf("failed to flush cache: %v", err)
	}

	// Try to get it back
	t.Logf("Getting key=%s", key)
	retrieved, err := tree.Get(ctx, key)
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}

	if retrieved == nil {
		t.Fatal("retrieved value is nil")
	}

	t.Logf("Retrieved value=%s", retrieved)
	if !bytes.Equal(retrieved, value) {
		t.Errorf("expected %s, got %s", value, retrieved)
	}
}

// TestLeafNodeDirect tests leaf node operations directly
func TestLeafNodeDirect(t *testing.T) {
	node := NewLeafNode(1)

	// Insert
	key := []byte("key1")
	value := []byte("value1")
	if err := node.Insert(key, value); err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Get
	retrieved, found := node.Get(key)
	if !found {
		t.Fatal("key not found")
	}
	if !bytes.Equal(retrieved, value) {
		t.Errorf("expected %s, got %s", value, retrieved)
	}

	// Serialize
	data := node.Serialize()
	t.Logf("Serialized %d bytes", len(data))

	// Deserialize
	node2, err := DeserializeLeaf(1, data)
	if err != nil {
		t.Fatalf("failed to deserialize: %v", err)
	}

	// Get from deserialized node
	retrieved2, found := node2.Get(key)
	if !found {
		t.Fatal("key not found after deserialize")
	}
	if !bytes.Equal(retrieved2, value) {
		t.Errorf("expected %s, got %s", value, retrieved2)
	}
}

// TestBlockRoundTrip tests writing and reading a block
func TestBlockRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gothinkdb-block-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	config := &storage.SerializerConfig{
		DataDir:     tmpDir,
		CacheSizeMB: 1,
	}

	ctx := context.Background()

	serializer, err := storage.NewFileSerializer(config)
	if err != nil {
		t.Fatalf("failed to create serializer: %v", err)
	}
	defer serializer.Close()

	// Create a leaf node
	node := NewLeafNode(storage.InvalidBlockID)
	node.Insert([]byte("key1"), []byte("value1"))
	node.Insert([]byte("key2"), []byte("value2"))

	// Serialize
	data := node.Serialize()
	t.Logf("Node serialized to %d bytes", len(data))

	// Write to block
	block := storage.NewBlockFromData(data)
	blockID, err := serializer.BlockWrite(ctx, storage.InvalidBlockID, block)
	if err != nil {
		t.Fatalf("failed to write block: %v", err)
	}
	t.Logf("Block written with ID %d", blockID)

	// Flush
	if err := serializer.Flush(ctx); err != nil {
		t.Fatalf("failed to flush: %v", err)
	}

	// Read back
	readBlock, err := serializer.BlockRead(ctx, blockID)
	if err != nil {
		t.Fatalf("failed to read block: %v", err)
	}

	// Deserialize
	node2, err := DeserializeLeaf(blockID, readBlock[:])
	if err != nil {
		t.Fatalf("failed to deserialize: %v", err)
	}

	// Verify
	value, found := node2.Get([]byte("key1"))
	if !found {
		t.Fatal("key1 not found")
	}
	if !bytes.Equal(value, []byte("value1")) {
		t.Errorf("expected value1, got %s", value)
	}

	value, found = node2.Get([]byte("key2"))
	if !found {
		t.Fatal("key2 not found")
	}
	if !bytes.Equal(value, []byte("value2")) {
		t.Errorf("expected value2, got %s", value)
	}
}

// TestCacheRoundTrip tests writing and reading through cache
func TestCacheRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gothinkdb-cache-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	config := &storage.SerializerConfig{
		DataDir:     tmpDir,
		CacheSizeMB: 1,
	}

	ctx := context.Background()

	serializer, err := storage.NewFileSerializer(config)
	if err != nil {
		t.Fatalf("failed to create serializer: %v", err)
	}
	defer serializer.Close()

	cache := storage.NewPageCache(serializer, 100)
	defer cache.Close()

	// Create a leaf node
	node := NewLeafNode(storage.InvalidBlockID)
	node.Insert([]byte("key1"), []byte("value1"))

	// Serialize
	data := node.Serialize()
	t.Logf("Node serialized to %d bytes", len(data))

	// Write to cache
	block := storage.NewBlockFromData(data)
	blockID, err := cache.Allocate(ctx, block)
	if err != nil {
		t.Fatalf("failed to allocate: %v", err)
	}
	t.Logf("Block allocated with ID %d", blockID)

	// Flush
	if err := cache.Flush(ctx); err != nil {
		t.Fatalf("failed to flush: %v", err)
	}

	// Read back
	readBlock, err := cache.Get(ctx, blockID)
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}

	t.Logf("Read block: first byte = %d (should be %d for leaf)", readBlock[0], byte(NodeTypeLeaf))

	// Deserialize
	node2, err := DeserializeLeaf(blockID, readBlock[:])
	if err != nil {
		t.Fatalf("failed to deserialize: %v", err)
	}

	// Verify
	value, found := node2.Get([]byte("key1"))
	if !found {
		t.Fatal("key1 not found")
	}
	if !bytes.Equal(value, []byte("value1")) {
		t.Errorf("expected value1, got %s", value)
	}
}

func TestDebugSerialization(t *testing.T) {
	node := NewLeafNode(1)
	node.Insert([]byte("abc"), []byte("123"))

	data := node.Serialize()

	t.Logf("Serialized data length: %d", len(data))
	t.Logf("First 20 bytes: %v", data[:20])
	t.Logf("Node type byte: %d (expected %d)", data[0], byte(NodeTypeLeaf))
	t.Logf("Num keys (bytes 1-2): %v", data[1:3])

	// Try to deserialize
	node2, err := DeserializeLeaf(1, data)
	if err != nil {
		t.Fatalf("deserialize failed: %v", err)
	}

	t.Logf("Deserialized node has %d keys", len(node2.keys))
	if len(node2.keys) > 0 {
		t.Logf("First key: %s", node2.keys[0])
		t.Logf("First value: %s", node2.values[0])
	}

	value, found := node2.Get([]byte("abc"))
	if !found {
		t.Fatal("key not found")
	}
	if string(value) != "123" {
		t.Errorf("expected 123, got %s", value)
	}
}
