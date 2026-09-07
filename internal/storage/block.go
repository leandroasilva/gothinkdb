// Package storage implements the storage engine for GoThinkDB.
// This includes the page cache, serializer, and block management.
package storage

// Block size constants
const (
	// BlockSize is the size of a single block in bytes (4KB)
	BlockSize = 4096

	// MaxBlockID is the maximum number of blocks
	MaxBlockID = 1<<32 - 1

	// InvalidBlockID represents an invalid block ID
	InvalidBlockID BlockID = 0
)

// BlockID represents a unique identifier for a block
type BlockID uint32

// Block represents a fixed-size block of data
type Block [BlockSize]byte

// BlockRef is a reference to a block with its ID
type BlockRef struct {
	ID   BlockID
	Data *Block
}

// NewBlock creates a new empty block
func NewBlock() *Block {
	return &Block{}
}

// NewBlockFromData creates a new block from existing data
func NewBlockFromData(data []byte) *Block {
	block := NewBlock()
	copy(block[:], data)
	return block
}

// IsZero returns true if the block is all zeros
func (b *Block) IsZero() bool {
	for _, byte := range b {
		if byte != 0 {
			return false
		}
	}
	return true
}

// Clone creates a copy of the block
func (b *Block) Clone() *Block {
	clone := NewBlock()
	copy(clone[:], b[:])
	return clone
}
