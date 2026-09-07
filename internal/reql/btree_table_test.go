package reql

import (
	"context"
	"os"
	"testing"

	"github.com/leandroasilva/gothinkdb/internal/storage"
	"github.com/leandroasilva/gothinkdb/pkg/datum"
)

func TestBTreeTable_BasicOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "btree_table_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	serializer, err := storage.NewFileSerializer(&storage.SerializerConfig{
		DataDir:     tmpDir,
		CacheSizeMB: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	cache := storage.NewPageCache(serializer, 100)
	ctx := context.Background()

	// Create table
	table, err := NewBTreeTable("users", cache)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert documents
	for i := 0; i < 10; i++ {
		doc := datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
			"id":   datum.NewStr("user" + string(rune('0'+i))),
			"name": datum.NewStr("User" + string(rune('0'+i))),
			"age":  datum.NewNum(float64(20 + i)),
		}))

		err := table.Insert(ctx, "user"+string(rune('0'+i)), doc)
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	// Get documents
	for i := 0; i < 10; i++ {
		doc, err := table.Get(ctx, "user"+string(rune('0'+i)))
		if err != nil {
			t.Fatalf("get %d failed: %v", i, err)
		}

		if doc.IsNull() {
			t.Errorf("document %d should not be null", i)
		}

		name, ok := doc.Object().Get("name")
		if !ok {
			t.Errorf("document %d should have name field", i)
		}

		expectedName := "User" + string(rune('0'+i))
		if name.Str() != expectedName {
			t.Errorf("expected name %s, got %s", expectedName, name.Str())
		}
	}

	// Get all documents
	docs, err := table.GetAll(ctx)
	if err != nil {
		t.Fatalf("get all failed: %v", err)
	}

	if len(docs) != 10 {
		t.Errorf("expected 10 documents, got %d", len(docs))
	}

	// Update document
	updates := datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
		"age": datum.NewNum(99),
	}))

	err = table.Update(ctx, "user0", updates)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	// Verify update
	doc, err := table.Get(ctx, "user0")
	if err != nil {
		t.Fatalf("get after update failed: %v", err)
	}

	age, ok := doc.Object().Get("age")
	if !ok {
		t.Error("document should have age field")
	}

	if age.Num() != 99 {
		t.Errorf("expected age 99, got %f", age.Num())
	}

	// Delete document
	err = table.Delete(ctx, "user0")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// Verify deletion
	doc, err = table.Get(ctx, "user0")
	if err != nil {
		t.Fatalf("get after delete failed: %v", err)
	}

	if !doc.IsNull() {
		t.Error("document should be null after deletion")
	}
}

func TestBTreeTable_LargeDataset(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "btree_table_large_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	serializer, err := storage.NewFileSerializer(&storage.SerializerConfig{
		DataDir:     tmpDir,
		CacheSizeMB: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	cache := storage.NewPageCache(serializer, 100)
	ctx := context.Background()

	// Create table
	table, err := NewBTreeTable("large_table", cache)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert 100 documents
	for i := 0; i < 100; i++ {
		doc := datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
			"id":    datum.NewStr("doc" + string(rune('0'+i/10)) + string(rune('0'+i%10))),
			"value": datum.NewNum(float64(i)),
		}))

		err := table.Insert(ctx, "doc"+string(rune('0'+i/10))+string(rune('0'+i%10)), doc)
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	// Verify all documents
	for i := 0; i < 100; i++ {
		doc, err := table.Get(ctx, "doc"+string(rune('0'+i/10))+string(rune('0'+i%10)))
		if err != nil {
			t.Fatalf("get %d failed: %v", i, err)
		}

		if doc.IsNull() {
			t.Errorf("document %d should not be null", i)
		}

		value, ok := doc.Object().Get("value")
		if !ok {
			t.Errorf("document %d should have value field", i)
		}

		if value.Num() != float64(i) {
			t.Errorf("expected value %d, got %f", i, value.Num())
		}
	}

	// Get all documents
	docs, err := table.GetAll(ctx)
	if err != nil {
		t.Fatalf("get all failed: %v", err)
	}

	if len(docs) != 100 {
		t.Errorf("expected 100 documents, got %d", len(docs))
	}
}

func TestBTreeTable_WithIndexes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "btree_table_index_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	serializer, err := storage.NewFileSerializer(&storage.SerializerConfig{
		DataDir:     tmpDir,
		CacheSizeMB: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	cache := storage.NewPageCache(serializer, 100)
	ctx := context.Background()

	// Create table
	table, err := NewBTreeTable("users", cache)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Create index
	err = table.Indexes.CreateIndex("age_index", "age")
	if err != nil {
		t.Fatalf("create index failed: %v", err)
	}

	// Insert documents
	for i := 0; i < 5; i++ {
		doc := datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
			"id":   datum.NewStr("user" + string(rune('0'+i))),
			"name": datum.NewStr("User" + string(rune('0'+i))),
			"age":  datum.NewNum(float64(20 + i)),
		}))

		err := table.Insert(ctx, "user"+string(rune('0'+i)), doc)
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	// Verify index was updated
	idx, ok := table.Indexes.GetIndex("age_index")
	if !ok {
		t.Fatal("index should exist")
	}

	// Query by index
	docIDs := idx.Get(datum.NewNum(22))
	if len(docIDs) != 1 {
		t.Errorf("expected 1 document with age 22, got %d", len(docIDs))
	}
}
