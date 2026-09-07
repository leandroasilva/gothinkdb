package raft

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// WAL represents a Write-Ahead Log
type WAL struct {
	path    string
	file    *os.File
	encoder *json.Encoder
	mu      sync.Mutex
}

// NewWAL creates a new Write-Ahead Log
func NewWAL(path string) (*WAL, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create WAL directory: %w", err)
	}

	// Open or create the WAL file
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	return &WAL{
		path:    path,
		file:    file,
		encoder: json.NewEncoder(file),
	}, nil
}

// Append appends an entry to the WAL
func (w *WAL) Append(entry *LogEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.encoder.Encode(entry); err != nil {
		return fmt.Errorf("failed to write WAL entry: %w", err)
	}

	// Sync to disk
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync WAL: %w", err)
	}

	return nil
}

// Read reads all entries from the WAL
func (w *WAL) Read() ([]*LogEntry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Seek to beginning
	if _, err := w.file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek WAL: %w", err)
	}

	var entries []*LogEntry
	decoder := json.NewDecoder(w.file)

	for {
		var entry LogEntry
		if err := decoder.Decode(&entry); err != nil {
			break // EOF or error
		}
		entries = append(entries, &entry)
	}

	return entries, nil
}

// Truncate truncates the WAL up to the given index
func (w *WAL) Truncate(index int64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Seek to beginning
	if _, err := w.file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek WAL: %w", err)
	}

	// Read all entries
	var entries []*LogEntry
	decoder := json.NewDecoder(w.file)

	for {
		var entry LogEntry
		if err := decoder.Decode(&entry); err != nil {
			break // EOF or error
		}
		entries = append(entries, &entry)
	}

	// Filter entries
	var remaining []*LogEntry
	for _, entry := range entries {
		if entry.Index > index {
			remaining = append(remaining, entry)
		}
	}

	// Close current file
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("failed to close WAL: %w", err)
	}

	// Create new file
	file, err := os.Create(w.path)
	if err != nil {
		return fmt.Errorf("failed to create new WAL: %w", err)
	}

	w.file = file
	w.encoder = json.NewEncoder(file)

	// Write remaining entries
	for _, entry := range remaining {
		if err := w.encoder.Encode(entry); err != nil {
			return fmt.Errorf("failed to write WAL entry: %w", err)
		}
	}

	// Sync to disk
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync WAL: %w", err)
	}

	return nil
}

// Close closes the WAL
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		return w.file.Close()
	}
	return nil
}
