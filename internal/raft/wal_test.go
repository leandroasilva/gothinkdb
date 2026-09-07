package raft

import (
	"path/filepath"
	"testing"
)

func TestWAL_AppendRead(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	wal, err := NewWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer wal.Close()

	// Append entries
	entries := []*LogEntry{
		{Term: 1, Index: 1, Command: "cmd1"},
		{Term: 1, Index: 2, Command: "cmd2"},
		{Term: 2, Index: 3, Command: "cmd3"},
	}

	for _, entry := range entries {
		if err := wal.Append(entry); err != nil {
			t.Fatalf("Failed to append entry: %v", err)
		}
	}

	// Read entries
	readEntries, err := wal.Read()
	if err != nil {
		t.Fatalf("Failed to read WAL: %v", err)
	}

	if len(readEntries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(readEntries))
	}
}

func TestWAL_Truncate(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	wal, err := NewWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer wal.Close()

	// Append entries
	for i := int64(1); i <= 5; i++ {
		entry := &LogEntry{Term: 1, Index: i, Command: "cmd"}
		if err := wal.Append(entry); err != nil {
			t.Fatalf("Failed to append entry: %v", err)
		}
	}

	// Truncate up to index 3
	if err := wal.Truncate(3); err != nil {
		t.Fatalf("Failed to truncate WAL: %v", err)
	}

	// Read remaining entries
	entries, err := wal.Read()
	if err != nil {
		t.Fatalf("Failed to read WAL: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("Expected 2 entries after truncation, got %d", len(entries))
	}

	if entries[0].Index != 4 {
		t.Errorf("Expected first entry index 4, got %d", entries[0].Index)
	}
}

func TestSnapshot_CreateLoad(t *testing.T) {
	tmpDir := t.TempDir()
	snapshotPath := filepath.Join(tmpDir, "snapshot.json")

	sm, err := NewSnapshotManager(snapshotPath)
	if err != nil {
		t.Fatalf("Failed to create snapshot manager: %v", err)
	}

	// Create snapshot
	data := []*LogEntry{
		{Term: 1, Index: 1, Command: "cmd1"},
		{Term: 1, Index: 2, Command: "cmd2"},
	}

	if err := sm.Create(2, 1, data); err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	// Load snapshot
	snapshot, err := sm.Load()
	if err != nil {
		t.Fatalf("Failed to load snapshot: %v", err)
	}

	if snapshot == nil {
		t.Fatal("Snapshot should not be nil")
	}

	if snapshot.LastIndex != 2 {
		t.Errorf("Expected LastIndex 2, got %d", snapshot.LastIndex)
	}

	if snapshot.LastTerm != 1 {
		t.Errorf("Expected LastTerm 1, got %d", snapshot.LastTerm)
	}
}

func TestSnapshot_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	snapshotPath := filepath.Join(tmpDir, "snapshot.json")

	sm, err := NewSnapshotManager(snapshotPath)
	if err != nil {
		t.Fatalf("Failed to create snapshot manager: %v", err)
	}

	// Create snapshot
	if err := sm.Create(1, 1, nil); err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	// Check exists
	if !sm.Exists() {
		t.Error("Snapshot should exist")
	}

	// Delete snapshot
	if err := sm.Delete(); err != nil {
		t.Fatalf("Failed to delete snapshot: %v", err)
	}

	// Check not exists
	if sm.Exists() {
		t.Error("Snapshot should not exist after deletion")
	}
}

func TestSnapshotManager_NoSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	snapshotPath := filepath.Join(tmpDir, "snapshot.json")

	sm, err := NewSnapshotManager(snapshotPath)
	if err != nil {
		t.Fatalf("Failed to create snapshot manager: %v", err)
	}

	// Load non-existent snapshot
	snapshot, err := sm.Load()
	if err != nil {
		t.Fatalf("Failed to load snapshot: %v", err)
	}

	if snapshot != nil {
		t.Error("Snapshot should be nil when file doesn't exist")
	}
}
