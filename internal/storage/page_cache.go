package storage

import (
	"container/list"
	"context"
	"sync"
)

// PageCache is an in-memory cache for blocks
type PageCache struct {
	mu         sync.RWMutex
	cache      map[BlockID]*list.Element
	lru        *list.List
	maxSize    int // Maximum number of blocks to cache
	serializer Serializer
	hits       uint64
	misses     uint64
}

// cacheEntry represents an entry in the cache
type cacheEntry struct {
	id    BlockID
	block *Block
	dirty bool // True if block has been modified
}

// NewPageCache creates a new page cache
func NewPageCache(serializer Serializer, maxSize int) *PageCache {
	return &PageCache{
		cache:      make(map[BlockID]*list.Element),
		lru:        list.New(),
		maxSize:    maxSize,
		serializer: serializer,
	}
}

// Get retrieves a block from the cache or serializer
func (c *PageCache) Get(ctx context.Context, id BlockID) (*Block, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if block is in cache
	if elem, ok := c.cache[id]; ok {
		// Move to front (most recently used)
		c.lru.MoveToFront(elem)
		c.hits++
		entry := elem.Value.(*cacheEntry)
		return entry.block.Clone(), nil
	}

	// Cache miss - read from serializer
	c.misses++
	block, err := c.serializer.BlockRead(ctx, id)
	if err != nil {
		return nil, err
	}

	// Add to cache
	c.addToCache(id, block, false)

	return block.Clone(), nil
}

// Put writes a block to the cache and marks it as dirty
func (c *PageCache) Put(ctx context.Context, id BlockID, block *Block) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if block is already in cache
	if elem, ok := c.cache[id]; ok {
		// Update existing entry
		c.lru.MoveToFront(elem)
		entry := elem.Value.(*cacheEntry)
		entry.block = block.Clone()
		entry.dirty = true
		return nil
	}

	// Add new entry to cache
	c.addToCache(id, block, true)

	return nil
}

// Allocate allocates a new block and returns its ID
func (c *PageCache) Allocate(ctx context.Context, block *Block) (BlockID, error) {
	// Write to serializer to get new ID
	id, err := c.serializer.BlockWrite(ctx, InvalidBlockID, block)
	if err != nil {
		return InvalidBlockID, err
	}

	// Add to cache
	c.mu.Lock()
	c.addToCache(id, block, false)
	c.mu.Unlock()

	return id, nil
}

// Flush writes all dirty blocks to the serializer
func (c *PageCache) Flush(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Write all dirty blocks
	for elem := c.lru.Front(); elem != nil; elem = elem.Next() {
		entry := elem.Value.(*cacheEntry)
		if entry.dirty {
			if _, err := c.serializer.BlockWrite(ctx, entry.id, entry.block); err != nil {
				return err
			}
			entry.dirty = false
		}
	}

	return c.serializer.Flush(ctx)
}

// Close flushes and closes the cache
func (c *PageCache) Close() error {
	ctx := context.Background()
	if err := c.Flush(ctx); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = nil
	c.lru = nil
	return nil
}

// Stats returns cache statistics
func (c *PageCache) Stats() (hits, misses uint64, size int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits, c.misses, len(c.cache)
}

// addToCache adds a block to the cache (must be called with lock held)
func (c *PageCache) addToCache(id BlockID, block *Block, dirty bool) {
	// Evict if necessary
	for len(c.cache) >= c.maxSize {
		c.evict()
	}

	// Add to cache
	entry := &cacheEntry{
		id:    id,
		block: block.Clone(),
		dirty: dirty,
	}
	elem := c.lru.PushFront(entry)
	c.cache[id] = elem
}

// evict removes the least recently used block from the cache (must be called with lock held)
func (c *PageCache) evict() {
	// Get least recently used element
	elem := c.lru.Back()
	if elem == nil {
		return
	}

	entry := elem.Value.(*cacheEntry)

	// Write back if dirty
	if entry.dirty {
		ctx := context.Background()
		_, _ = c.serializer.BlockWrite(ctx, entry.id, entry.block)
	}

	// Remove from cache
	delete(c.cache, entry.id)
	c.lru.Remove(elem)
}

// Delete removes a block from the cache and serializer
func (c *PageCache) Delete(ctx context.Context, id BlockID) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove from cache
	if elem, ok := c.cache[id]; ok {
		c.lru.Remove(elem)
		delete(c.cache, id)
	}

	// Delete from serializer
	return c.serializer.BlockDelete(ctx, id)
}

// Size returns the number of blocks in the cache
func (c *PageCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}
