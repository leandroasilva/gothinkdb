package btree

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/leandroasilva/gothinkdb/internal/storage"
)

// TestMultipleInserts tests inserting multiple keys
func TestMultipleInserts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gothinkdb-multi-test-*")
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

	// Insert 10 keys
	for i := 0; i < 10; i++ {
		key := []byte(fmt.Sprintf("key%02d", i))
		value := []byte(fmt.Sprintf("value%02d", i))

		t.Logf("Inserting key=%s, value=%s", key, value)
		if err := tree.Put(ctx, key, value); err != nil {
			t.Fatalf("failed to put key %d: %v", i, err)
		}
	}

	// Flush cache
	if err := cache.Flush(ctx); err != nil {
		t.Fatalf("failed to flush cache: %v", err)
	}

	// Try to get them back
	for i := 0; i < 10; i++ {
		key := []byte(fmt.Sprintf("key%02d", i))
		expected := fmt.Sprintf("value%02d", i)

		t.Logf("Getting key=%s", key)
		retrieved, err := tree.Get(ctx, key)
		if err != nil {
			t.Fatalf("failed to get key %d: %v", i, err)
		}

		if retrieved == nil {
			t.Errorf("key %d: retrieved value is nil", i)
			continue
		}

		t.Logf("Retrieved value=%s", retrieved)
		if !bytes.Equal(retrieved, []byte(expected)) {
			t.Errorf("key %d: expected %s, got %s", i, expected, retrieved)
		}
	}
}

// TestInsertAndUpdate tests inserting and updating keys
func TestInsertAndUpdate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gothinkdb-update-test-*")
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

	// Insert a key
	key := []byte("key1")
	value1 := []byte("value1")
	t.Logf("Inserting key=%s, value=%s", key, value1)
	if err := tree.Put(ctx, key, value1); err != nil {
		t.Fatalf("failed to put: %v", err)
	}

	// Get it back
	retrieved, err := tree.Get(ctx, key)
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}
	if !bytes.Equal(retrieved, value1) {
		t.Errorf("expected %s, got %s", value1, retrieved)
	}

	// Update it
	value2 := []byte("value2")
	t.Logf("Updating key=%s, value=%s", key, value2)
	if err := tree.Put(ctx, key, value2); err != nil {
		t.Fatalf("failed to update: %v", err)
	}

	// Get it back again
	retrieved, err = tree.Get(ctx, key)
	if err != nil {
		t.Fatalf("failed to get after update: %v", err)
	}
	if !bytes.Equal(retrieved, value2) {
		t.Errorf("expected %s, got %s", value2, retrieved)
	}
}
