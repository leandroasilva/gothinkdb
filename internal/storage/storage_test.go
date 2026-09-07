package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBlock(t *testing.T) {
	t.Run("NewBlock", func(t *testing.T) {
		block := NewBlock()
		if block == nil {
			t.Fatal("expected non-nil block")
		}
		if !block.IsZero() {
			t.Error("expected zero block")
		}
	})

	t.Run("NewBlockFromData", func(t *testing.T) {
		data := []byte("hello world")
		block := NewBlockFromData(data)
		if block == nil {
			t.Fatal("expected non-nil block")
		}
		if block.IsZero() {
			t.Error("expected non-zero block")
		}
		// Check that data was copied
		for i, b := range data {
			if block[i] != b {
				t.Errorf("byte %d: expected %d, got %d", i, b, block[i])
			}
		}
	})

	t.Run("Clone", func(t *testing.T) {
		block := NewBlockFromData([]byte("test data"))
		clone := block.Clone()
		if clone == block {
			t.Error("expected different pointer")
		}
		// Modify original
		block[0] = 99
		// Clone should not be affected
		if clone[0] == 99 {
			t.Error("clone should not be affected by original modification")
		}
	})
}

func TestFileSerializer(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "gothinkdb-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	config := &SerializerConfig{
		DataDir:     tmpDir,
		CacheSizeMB: 1,
	}

	ctx := context.Background()

	t.Run("Create", func(t *testing.T) {
		serializer, err := NewFileSerializer(config)
		if err != nil {
			t.Fatalf("failed to create serializer: %v", err)
		}
		defer serializer.Close()

		if serializer.EndBlockID() != 1 {
			t.Errorf("expected next ID 1, got %d", serializer.EndBlockID())
		}
	})

	t.Run("WriteAndRead", func(t *testing.T) {
		serializer, err := NewFileSerializer(config)
		if err != nil {
			t.Fatalf("failed to create serializer: %v", err)
		}
		defer serializer.Close()

		// Write a block
		block := NewBlockFromData([]byte("test data"))
		id, err := serializer.BlockWrite(ctx, InvalidBlockID, block)
		if err != nil {
			t.Fatalf("failed to write block: %v", err)
		}

		if id == InvalidBlockID {
			t.Error("expected valid block ID")
		}

		// Read it back
		readBlock, err := serializer.BlockRead(ctx, id)
		if err != nil {
			t.Fatalf("failed to read block: %v", err)
		}

		// Verify data
		for i := 0; i < 9; i++ {
			if readBlock[i] != block[i] {
				t.Errorf("byte %d: expected %d, got %d", i, block[i], readBlock[i])
			}
		}
	})

	t.Run("Delete", func(t *testing.T) {
		serializer, err := NewFileSerializer(config)
		if err != nil {
			t.Fatalf("failed to create serializer: %v", err)
		}
		defer serializer.Close()

		// Write a block
		block := NewBlockFromData([]byte("to be deleted"))
		id, err := serializer.BlockWrite(ctx, InvalidBlockID, block)
		if err != nil {
			t.Fatalf("failed to write block: %v", err)
		}

		// Delete it
		if err := serializer.BlockDelete(ctx, id); err != nil {
			t.Fatalf("failed to delete block: %v", err)
		}

		// Read it back (should be zeros)
		readBlock, err := serializer.BlockRead(ctx, id)
		if err != nil {
			t.Fatalf("failed to read block: %v", err)
		}

		if !readBlock.IsZero() {
			t.Error("expected zero block after delete")
		}
	})

	t.Run("Flush", func(t *testing.T) {
		serializer, err := NewFileSerializer(config)
		if err != nil {
			t.Fatalf("failed to create serializer: %v", err)
		}

		// Write a block
		block := NewBlockFromData([]byte("flush test"))
		_, err = serializer.BlockWrite(ctx, InvalidBlockID, block)
		if err != nil {
			t.Fatalf("failed to write block: %v", err)
		}

		// Flush
		if err := serializer.Flush(ctx); err != nil {
			t.Fatalf("failed to flush: %v", err)
		}

		// Close and reopen
		serializer.Close()

		serializer2, err := NewFileSerializer(config)
		if err != nil {
			t.Fatalf("failed to reopen serializer: %v", err)
		}
		defer serializer2.Close()

		// Should be able to read the block
		readBlock, err := serializer2.BlockRead(ctx, 1)
		if err != nil {
			t.Fatalf("failed to read block after reopen: %v", err)
		}

		if readBlock.IsZero() {
			t.Error("expected non-zero block after reopen")
		}
	})
}

func TestPageCache(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "gothinkdb-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	config := &SerializerConfig{
		DataDir:     tmpDir,
		CacheSizeMB: 1,
	}

	ctx := context.Background()

	t.Run("CacheHit", func(t *testing.T) {
		serializer, err := NewFileSerializer(config)
		if err != nil {
			t.Fatalf("failed to create serializer: %v", err)
		}
		defer serializer.Close()

		cache := NewPageCache(serializer, 10)
		defer cache.Close()

		// Write a block
		block := NewBlockFromData([]byte("cache test"))
		id, err := cache.Allocate(ctx, block)
		if err != nil {
			t.Fatalf("failed to allocate block: %v", err)
		}

		// Get it (should be cache miss)
		_, err = cache.Get(ctx, id)
		if err != nil {
			t.Fatalf("failed to get block: %v", err)
		}

		hits1, misses1, _ := cache.Stats()

		// Get it again (should be cache hit)
		_, err = cache.Get(ctx, id)
		if err != nil {
			t.Fatalf("failed to get block: %v", err)
		}

		hits2, misses2, _ := cache.Stats()

		if hits2 != hits1+1 {
			t.Errorf("expected %d hits, got %d", hits1+1, hits2)
		}
		if misses2 != misses1 {
			t.Errorf("expected %d misses, got %d", misses1, misses2)
		}
	})

	t.Run("CacheEviction", func(t *testing.T) {
		serializer, err := NewFileSerializer(config)
		if err != nil {
			t.Fatalf("failed to create serializer: %v", err)
		}
		defer serializer.Close()

		cache := NewPageCache(serializer, 3) // Small cache
		defer cache.Close()

		// Write 5 blocks (more than cache size)
		ids := make([]BlockID, 5)
		for i := 0; i < 5; i++ {
			block := NewBlockFromData([]byte{byte(i)})
			id, err := cache.Allocate(ctx, block)
			if err != nil {
				t.Fatalf("failed to allocate block %d: %v", i, err)
			}
			ids[i] = id
		}

		// Cache should only have 3 blocks
		if cache.Size() > 3 {
			t.Errorf("expected cache size <= 3, got %d", cache.Size())
		}

		// All blocks should still be readable from serializer
		for i, id := range ids {
			block, err := cache.Get(ctx, id)
			if err != nil {
				t.Fatalf("failed to get block %d: %v", i, err)
			}
			if block[0] != byte(i) {
				t.Errorf("block %d: expected %d, got %d", i, i, block[0])
			}
		}
	})

	t.Run("DirtyBlocks", func(t *testing.T) {
		serializer, err := NewFileSerializer(config)
		if err != nil {
			t.Fatalf("failed to create serializer: %v", err)
		}
		defer serializer.Close()

		cache := NewPageCache(serializer, 10)

		// Allocate a block
		block1 := NewBlockFromData([]byte("original"))
		id, err := cache.Allocate(ctx, block1)
		if err != nil {
			t.Fatalf("failed to allocate block: %v", err)
		}

		// Modify it in cache
		block2 := NewBlockFromData([]byte("modified"))
		if err := cache.Put(ctx, id, block2); err != nil {
			t.Fatalf("failed to put block: %v", err)
		}

		// Flush to serializer
		if err := cache.Flush(ctx); err != nil {
			t.Fatalf("failed to flush: %v", err)
		}

		cache.Close()

		// Reopen and verify
		serializer2, err := NewFileSerializer(config)
		if err != nil {
			t.Fatalf("failed to reopen serializer: %v", err)
		}
		defer serializer2.Close()

		readBlock, err := serializer2.BlockRead(ctx, id)
		if err != nil {
			t.Fatalf("failed to read block: %v", err)
		}

		// Should have modified data
		if string(readBlock[:8]) != "modified" {
			t.Errorf("expected 'modified', got '%s'", string(readBlock[:8]))
		}
	})
}

func TestIntegration(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "gothinkdb-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	config := &SerializerConfig{
		DataDir:     tmpDir,
		CacheSizeMB: 1,
	}

	ctx := context.Background()

	t.Run("FullWorkflow", func(t *testing.T) {
		// Create serializer and cache
		serializer, err := NewFileSerializer(config)
		if err != nil {
			t.Fatalf("failed to create serializer: %v", err)
		}

		cache := NewPageCache(serializer, 100)

		// Write multiple blocks
		ids := make([]BlockID, 10)
		for i := 0; i < 10; i++ {
			block := NewBlockFromData([]byte{byte(i), byte(i + 1), byte(i + 2)})
			id, err := cache.Allocate(ctx, block)
			if err != nil {
				t.Fatalf("failed to allocate block %d: %v", i, err)
			}
			ids[i] = id
		}

		// Flush
		if err := cache.Flush(ctx); err != nil {
			t.Fatalf("failed to flush: %v", err)
		}

		// Close everything
		cache.Close()
		serializer.Close()

		// Reopen
		serializer2, err := NewFileSerializer(config)
		if err != nil {
			t.Fatalf("failed to reopen serializer: %v", err)
		}
		defer serializer2.Close()

		cache2 := NewPageCache(serializer2, 100)
		defer cache2.Close()

		// Read all blocks
		for i, id := range ids {
			block, err := cache2.Get(ctx, id)
			if err != nil {
				t.Fatalf("failed to get block %d: %v", i, err)
			}
			if block[0] != byte(i) || block[1] != byte(i+1) || block[2] != byte(i+2) {
				t.Errorf("block %d: data mismatch", i)
			}
		}

		// Delete some blocks
		for i := 0; i < 5; i++ {
			if err := cache2.Delete(ctx, ids[i]); err != nil {
				t.Fatalf("failed to delete block %d: %v", i, err)
			}
		}

		// Verify deleted blocks are zero
		for i := 0; i < 5; i++ {
			block, err := cache2.Get(ctx, ids[i])
			if err != nil {
				t.Fatalf("failed to get deleted block %d: %v", i, err)
			}
			if !block.IsZero() {
				t.Errorf("deleted block %d should be zero", i)
			}
		}

		// Verify remaining blocks are still there
		for i := 5; i < 10; i++ {
			block, err := cache2.Get(ctx, ids[i])
			if err != nil {
				t.Fatalf("failed to get block %d: %v", i, err)
			}
			if block.IsZero() {
				t.Errorf("block %d should not be zero", i)
			}
		}
	})
}

func BenchmarkBlockWrite(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "gothinkdb-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	config := &SerializerConfig{
		DataDir:     tmpDir,
		CacheSizeMB: 1,
	}

	ctx := context.Background()
	serializer, err := NewFileSerializer(config)
	if err != nil {
		b.Fatal(err)
	}
	defer serializer.Close()

	block := NewBlockFromData([]byte("benchmark data"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := serializer.BlockWrite(ctx, InvalidBlockID, block)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPageCacheGet(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "gothinkdb-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	config := &SerializerConfig{
		DataDir:     tmpDir,
		CacheSizeMB: 1,
	}

	ctx := context.Background()
	serializer, err := NewFileSerializer(config)
	if err != nil {
		b.Fatal(err)
	}
	defer serializer.Close()

	cache := NewPageCache(serializer, 1000)
	defer cache.Close()

	// Pre-populate cache
	block := NewBlockFromData([]byte("benchmark data"))
	id, err := cache.Allocate(ctx, block)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cache.Get(ctx, id)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestFileStructure verifies the file structure on disk
func TestFileStructure(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gothinkdb-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	config := &SerializerConfig{
		DataDir:     tmpDir,
		CacheSizeMB: 1,
	}

	ctx := context.Background()
	serializer, err := NewFileSerializer(config)
	if err != nil {
		t.Fatalf("failed to create serializer: %v", err)
	}

	// Write a block
	block := NewBlockFromData([]byte("test"))
	_, err = serializer.BlockWrite(ctx, InvalidBlockID, block)
	if err != nil {
		t.Fatalf("failed to write block: %v", err)
	}

	serializer.Close()

	// Verify file exists
	filePath := filepath.Join(tmpDir, "blocks.db")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("blocks.db file should exist")
	}

	// Verify file size
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		t.Fatal(err)
	}

	// Should have at least one block (4096 bytes)
	if fileInfo.Size() < BlockSize {
		t.Errorf("expected file size >= %d, got %d", BlockSize, fileInfo.Size())
	}
}
