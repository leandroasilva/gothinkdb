package btree

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/leandroasilva/gothinkdb/internal/storage"
)

func TestLeafNode(t *testing.T) {
	t.Run("Insert and Get", func(t *testing.T) {
		node := NewLeafNode(1)

		// Insert some keys
		for i := 0; i < 10; i++ {
			key := []byte(fmt.Sprintf("key%02d", i))
			value := []byte(fmt.Sprintf("value%02d", i))
			if err := node.Insert(key, value); err != nil {
				t.Fatalf("failed to insert: %v", err)
			}
		}

		// Get them back
		for i := 0; i < 10; i++ {
			key := []byte(fmt.Sprintf("key%02d", i))
			value, found := node.Get(key)
			if !found {
				t.Errorf("key not found: %s", key)
			}
			expected := fmt.Sprintf("value%02d", i)
			if string(value) != expected {
				t.Errorf("expected %s, got %s", expected, value)
			}
		}
	})

	t.Run("Delete", func(t *testing.T) {
		node := NewLeafNode(1)

		// Insert
		node.Insert([]byte("key1"), []byte("value1"))
		node.Insert([]byte("key2"), []byte("value2"))

		// Delete
		if !node.Delete([]byte("key1")) {
			t.Error("expected delete to succeed")
		}

		// Verify deleted
		_, found := node.Get([]byte("key1"))
		if found {
			t.Error("key1 should be deleted")
		}

		// Verify other key still exists
		value, found := node.Get([]byte("key2"))
		if !found || string(value) != "value2" {
			t.Error("key2 should still exist")
		}
	})

	t.Run("Serialize and Deserialize", func(t *testing.T) {
		node := NewLeafNode(1)
		node.Insert([]byte("key1"), []byte("value1"))
		node.Insert([]byte("key2"), []byte("value2"))

		// Serialize
		data := node.Serialize()

		// Deserialize
		node2, err := DeserializeLeaf(1, data)
		if err != nil {
			t.Fatalf("failed to deserialize: %v", err)
		}

		// Verify
		value, found := node2.Get([]byte("key1"))
		if !found || string(value) != "value1" {
			t.Error("key1 not found after deserialize")
		}
	})

	t.Run("Split", func(t *testing.T) {
		node := NewLeafNode(1)

		// Fill the node
		for i := 0; i < MaxLeafKeys; i++ {
			key := []byte(fmt.Sprintf("key%03d", i))
			value := []byte(fmt.Sprintf("value%03d", i))
			node.Insert(key, value)
		}

		if !node.IsFull() {
			t.Error("node should be full")
		}

		// Split
		right := node.Split(2)

		// Verify both nodes have keys
		if len(node.keys) == 0 || len(right.keys) == 0 {
			t.Error("both nodes should have keys after split")
		}

		// Verify total keys
		totalKeys := len(node.keys) + len(right.keys)
		if totalKeys != MaxLeafKeys {
			t.Errorf("expected %d total keys, got %d", MaxLeafKeys, totalKeys)
		}
	})
}

func TestBTree(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "gothinkdb-btree-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	config := &storage.SerializerConfig{
		DataDir:     tmpDir,
		CacheSizeMB: 1,
	}

	ctx := context.Background()

	t.Run("Create and Insert", func(t *testing.T) {
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

		// Insert some keys
		for i := 0; i < 100; i++ {
			key := []byte(fmt.Sprintf("key%03d", i))
			value := []byte(fmt.Sprintf("value%03d", i))
			if err := tree.Put(ctx, key, value); err != nil {
				t.Fatalf("failed to put: %v", err)
			}
		}

		// Get them back
		for i := 0; i < 100; i++ {
			key := []byte(fmt.Sprintf("key%03d", i))
			value, err := tree.Get(ctx, key)
			if err != nil {
				t.Fatalf("failed to get: %v", err)
			}
			expected := fmt.Sprintf("value%03d", i)
			if string(value) != expected {
				t.Errorf("expected %s, got %s", expected, value)
			}
		}
	})

	t.Run("Delete", func(t *testing.T) {
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

		// Insert
		tree.Put(ctx, []byte("key1"), []byte("value1"))
		tree.Put(ctx, []byte("key2"), []byte("value2"))

		// Delete
		if err := tree.Delete(ctx, []byte("key1")); err != nil {
			t.Fatalf("failed to delete: %v", err)
		}

		// Verify deleted
		value, err := tree.Get(ctx, []byte("key1"))
		if err != nil {
			t.Fatalf("failed to get: %v", err)
		}
		if value != nil {
			t.Error("key1 should be deleted")
		}

		// Verify other key still exists
		value, err = tree.Get(ctx, []byte("key2"))
		if err != nil {
			t.Fatalf("failed to get: %v", err)
		}
		if string(value) != "value2" {
			t.Error("key2 should still exist")
		}
	})

	t.Run("Range Scan", func(t *testing.T) {
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

		// Insert keys
		for i := 0; i < 50; i++ {
			key := []byte(fmt.Sprintf("key%03d", i))
			value := []byte(fmt.Sprintf("value%03d", i))
			tree.Put(ctx, key, value)
		}

		// Scan range
		results, err := tree.Scan(ctx, []byte("key010"), []byte("key020"))
		if err != nil {
			t.Fatalf("failed to scan: %v", err)
		}

		// Should have 11 keys (010-020 inclusive)
		if len(results) != 11 {
			t.Errorf("expected 11 results, got %d", len(results))
		}

		// Verify first and last
		if string(results[0].Key) != "key010" {
			t.Errorf("expected first key to be key010, got %s", results[0].Key)
		}
		if string(results[10].Key) != "key020" {
			t.Errorf("expected last key to be key020, got %s", results[10].Key)
		}
	})

	t.Run("Large Dataset", func(t *testing.T) {
		serializer, err := storage.NewFileSerializer(config)
		if err != nil {
			t.Fatalf("failed to create serializer: %v", err)
		}
		defer serializer.Close()

		cache := storage.NewPageCache(serializer, 1000)
		defer cache.Close()

		tree, err := New(cache)
		if err != nil {
			t.Fatalf("failed to create tree: %v", err)
		}

		// Insert 1000 keys
		for i := 0; i < 1000; i++ {
			key := []byte(fmt.Sprintf("key%05d", i))
			value := []byte(fmt.Sprintf("value%05d", i))
			if err := tree.Put(ctx, key, value); err != nil {
				t.Fatalf("failed to put key %d: %v", i, err)
			}
		}

		// Verify all keys
		for i := 0; i < 1000; i++ {
			key := []byte(fmt.Sprintf("key%05d", i))
			value, err := tree.Get(ctx, key)
			if err != nil {
				t.Fatalf("failed to get key %d: %v", i, err)
			}
			expected := fmt.Sprintf("value%05d", i)
			if string(value) != expected {
				t.Errorf("key %d: expected %s, got %s", i, expected, value)
			}
		}
	})
}

func BenchmarkBTreePut(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "gothinkdb-btree-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	config := &storage.SerializerConfig{
		DataDir:     tmpDir,
		CacheSizeMB: 10,
	}

	ctx := context.Background()
	serializer, err := storage.NewFileSerializer(config)
	if err != nil {
		b.Fatal(err)
	}
	defer serializer.Close()

	cache := storage.NewPageCache(serializer, 10000)
	defer cache.Close()

	tree, err := New(cache)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := []byte(fmt.Sprintf("key%010d", i))
		value := []byte(fmt.Sprintf("value%010d", i))
		tree.Put(ctx, key, value)
	}
}

func BenchmarkBTreeGet(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "gothinkdb-btree-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	config := &storage.SerializerConfig{
		DataDir:     tmpDir,
		CacheSizeMB: 10,
	}

	ctx := context.Background()
	serializer, err := storage.NewFileSerializer(config)
	if err != nil {
		b.Fatal(err)
	}
	defer serializer.Close()

	cache := storage.NewPageCache(serializer, 10000)
	defer cache.Close()

	tree, err := New(cache)
	if err != nil {
		b.Fatal(err)
	}

	// Pre-populate
	for i := 0; i < 10000; i++ {
		key := []byte(fmt.Sprintf("key%010d", i))
		value := []byte(fmt.Sprintf("value%010d", i))
		tree.Put(ctx, key, value)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := []byte(fmt.Sprintf("key%010d", i%10000))
		tree.Get(ctx, key)
	}
}
