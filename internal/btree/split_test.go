package btree

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/leandroasilva/gothinkdb/internal/storage"
)

// TestManyInserts tests inserting many keys (triggers split)
func TestManyInserts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gothinkdb-many-test-*")
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

	// Insert 60 keys (should trigger split since MaxLeafKeys = 50)
	numKeys := 60
	for i := 0; i < numKeys; i++ {
		key := []byte(fmt.Sprintf("key%03d", i))
		value := []byte(fmt.Sprintf("value%03d", i))

		if err := tree.Put(ctx, key, value); err != nil {
			t.Fatalf("failed to put key %d: %v", i, err)
		}
	}

	t.Logf("Inserted %d keys", numKeys)

	// Flush cache
	if err := cache.Flush(ctx); err != nil {
		t.Fatalf("failed to flush cache: %v", err)
	}

	// Try to get them back
	failCount := 0
	for i := 0; i < numKeys; i++ {
		key := []byte(fmt.Sprintf("key%03d", i))
		expected := fmt.Sprintf("value%03d", i)

		retrieved, err := tree.Get(ctx, key)
		if err != nil {
			t.Errorf("failed to get key %d: %v", i, err)
			failCount++
			continue
		}

		if retrieved == nil {
			t.Errorf("key %d: retrieved value is nil", i)
			failCount++
			continue
		}

		if !bytes.Equal(retrieved, []byte(expected)) {
			t.Errorf("key %d: expected %s, got %s", i, expected, retrieved)
			failCount++
		}
	}

	if failCount > 0 {
		t.Errorf("Failed to retrieve %d out of %d keys", failCount, numKeys)
	} else {
		t.Logf("Successfully retrieved all %d keys", numKeys)
	}
}
