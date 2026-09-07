package storage

import (
	"context"
	"errors"
)

// Common errors
var (
	ErrBlockNotFound    = errors.New("block not found")
	ErrInvalidBlockID   = errors.New("invalid block ID")
	ErrSerializerClosed = errors.New("serializer is closed")
	ErrIOError          = errors.New("I/O error")
)

// Serializer is the interface for block storage backends
type Serializer interface {
	// BlockRead reads a block by ID
	BlockRead(ctx context.Context, id BlockID) (*Block, error)

	// BlockWrite writes a block and returns its ID
	// If id is InvalidBlockID, a new block is allocated
	BlockWrite(ctx context.Context, id BlockID, block *Block) (BlockID, error)

	// BlockDelete marks a block as deleted
	BlockDelete(ctx context.Context, id BlockID) error

	// EndBlockID returns the next available block ID
	EndBlockID() BlockID

	// Flush ensures all pending writes are persisted
	Flush(ctx context.Context) error

	// Close closes the serializer
	Close() error
}

// SerializerConfig holds configuration for a serializer
type SerializerConfig struct {
	// DataDir is the directory where data files are stored
	DataDir string

	// CacheSizeMB is the size of the page cache in megabytes
	CacheSizeMB int

	// MaxOpenFiles is the maximum number of open file descriptors
	MaxOpenFiles int
}

// DefaultSerializerConfig returns a default configuration
func DefaultSerializerConfig() *SerializerConfig {
	return &SerializerConfig{
		DataDir:      "/data/gothinkdb",
		CacheSizeMB:  1024,
		MaxOpenFiles: 1000,
	}
}
