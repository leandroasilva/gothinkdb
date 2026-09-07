package storage

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileSerializer is a simple file-based block storage
type FileSerializer struct {
	mu         sync.RWMutex
	file       *os.File
	dataDir    string
	nextID     BlockID
	closed     bool
	blockCount uint32
}

// NewFileSerializer creates a new file-based serializer
func NewFileSerializer(config *SerializerConfig) (*FileSerializer, error) {
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	filePath := filepath.Join(config.DataDir, "blocks.db")
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Get file size to determine next block ID
	fileInfo, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	nextID := BlockID(fileInfo.Size() / BlockSize)
	if nextID == 0 {
		nextID = 1 // Start from 1 (0 is invalid)
	}

	return &FileSerializer{
		file:    file,
		dataDir: config.DataDir,
		nextID:  nextID,
	}, nil
}

// BlockRead reads a block by ID
func (s *FileSerializer) BlockRead(ctx context.Context, id BlockID) (*Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrSerializerClosed
	}

	if id == InvalidBlockID || id >= s.nextID {
		return nil, ErrInvalidBlockID
	}

	// Seek to block position
	offset := int64(id) * BlockSize
	if _, err := s.file.Seek(offset, 0); err != nil {
		return nil, fmt.Errorf("failed to seek: %w", err)
	}

	// Read block
	block := NewBlock()
	if _, err := s.file.Read(block[:]); err != nil {
		return nil, fmt.Errorf("failed to read block: %w", err)
	}

	return block, nil
}

// BlockWrite writes a block and returns its ID
func (s *FileSerializer) BlockWrite(ctx context.Context, id BlockID, block *Block) (BlockID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return InvalidBlockID, ErrSerializerClosed
	}

	// Allocate new block ID if needed
	if id == InvalidBlockID {
		id = s.nextID
		s.nextID++
	}

	// Seek to block position
	offset := int64(id) * BlockSize
	if _, err := s.file.Seek(offset, 0); err != nil {
		return InvalidBlockID, fmt.Errorf("failed to seek: %w", err)
	}

	// Write block
	if _, err := s.file.Write(block[:]); err != nil {
		return InvalidBlockID, fmt.Errorf("failed to write block: %w", err)
	}

	// Update block count
	if id >= BlockID(s.blockCount) {
		s.blockCount = uint32(id) + 1
	}

	return id, nil
}

// BlockDelete marks a block as deleted (fills with zeros)
func (s *FileSerializer) BlockDelete(ctx context.Context, id BlockID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrSerializerClosed
	}

	if id == InvalidBlockID || id >= s.nextID {
		return ErrInvalidBlockID
	}

	// Seek to block position
	offset := int64(id) * BlockSize
	if _, err := s.file.Seek(offset, 0); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}

	// Write zeros
	zeros := make([]byte, BlockSize)
	if _, err := s.file.Write(zeros); err != nil {
		return fmt.Errorf("failed to delete block: %w", err)
	}

	return nil
}

// EndBlockID returns the next available block ID
func (s *FileSerializer) EndBlockID() BlockID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nextID
}

// Flush ensures all pending writes are persisted
func (s *FileSerializer) Flush(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrSerializerClosed
	}

	return s.file.Sync()
}

// Close closes the serializer
func (s *FileSerializer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	return s.file.Close()
}

// WriteHeader writes a header to the file (for metadata)
func (s *FileSerializer) WriteHeader(ctx context.Context, header []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrSerializerClosed
	}

	// Write header size
	if _, err := s.file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}

	headerSize := uint32(len(header))
	if err := binary.Write(s.file, binary.LittleEndian, headerSize); err != nil {
		return fmt.Errorf("failed to write header size: %w", err)
	}

	// Write header data
	if _, err := s.file.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	return nil
}

// ReadHeader reads the header from the file
func (s *FileSerializer) ReadHeader(ctx context.Context) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrSerializerClosed
	}

	// Read header size
	if _, err := s.file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek: %w", err)
	}

	var headerSize uint32
	if err := binary.Read(s.file, binary.LittleEndian, &headerSize); err != nil {
		return nil, fmt.Errorf("failed to read header size: %w", err)
	}

	if headerSize == 0 || headerSize > 1024*1024 { // Max 1MB header
		return nil, fmt.Errorf("invalid header size: %d", headerSize)
	}

	// Read header data
	header := make([]byte, headerSize)
	if _, err := s.file.Read(header); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	return header, nil
}
