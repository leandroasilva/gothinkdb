package reql

import (
	"context"
	"fmt"
	"testing"
)

func TestEvaluator_InsertAndGet(t *testing.T) {
	eval := NewEvaluator()
	ctx := context.Background()

	// Insert a document
	insertQuery := []interface{}{
		float64(17), // INSERT
		[]interface{}{
			float64(10), // TABLE
			"users",
		},
		map[string]interface{}{
			"id":   "user1",
			"name": "Alice",
			"age":  float64(30),
		},
	}

	result, err := eval.Evaluate(ctx, insertQuery)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if result.Object() == nil {
		t.Fatal("result should be an object")
	}

	inserted, ok := result.Object().Get("inserted")
	if !ok || inserted.Num() != 1 {
		t.Errorf("expected inserted=1, got %v", inserted)
	}

	// Get the document
	getQuery := []interface{}{
		float64(70), // GET
		[]interface{}{
			float64(10), // TABLE
			"users",
		},
		"user1",
	}

	doc, err := eval.Evaluate(ctx, getQuery)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if doc.Object() == nil {
		t.Fatal("document should be an object")
	}

	name, ok := doc.Object().Get("name")
	if !ok || name.Str() != "Alice" {
		t.Errorf("expected name=Alice, got %v", name)
	}

	age, ok := doc.Object().Get("age")
	if !ok || age.Num() != 30 {
		t.Errorf("expected age=30, got %v", age)
	}
}

func TestEvaluator_MultipleInserts(t *testing.T) {
	eval := NewEvaluator()
	ctx := context.Background()

	// Insert multiple documents
	for i := 0; i < 3; i++ {
		insertQuery := []interface{}{
			float64(17),
			[]interface{}{
				float64(10),
				"test_table",
			},
			map[string]interface{}{
				"id":    fmt.Sprintf("doc%d", i),
				"value": i * 10,
			},
		}

		result, err := eval.Evaluate(ctx, insertQuery)
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}

		if result.Object() == nil {
			t.Fatalf("result %d should be an object", i)
		}
	}

	// Verify we can get each document
	for i := 0; i < 3; i++ {
		getQuery := []interface{}{
			float64(70),
			[]interface{}{
				float64(10),
				"test_table",
			},
			fmt.Sprintf("doc%d", i),
		}

		doc, err := eval.Evaluate(ctx, getQuery)
		if err != nil {
			t.Fatalf("get %d failed: %v", i, err)
		}

		if doc.Object() == nil {
			t.Fatalf("document %d should be an object", i)
		}

		value, ok := doc.Object().Get("value")
		if !ok || value.Num() != float64(i*10) {
			t.Errorf("document %d: expected value=%d, got %v", i, i*10, value)
		}
	}
}

func TestEvaluator_GetNonExistent(t *testing.T) {
	eval := NewEvaluator()
	ctx := context.Background()

	// Get non-existent document
	getQuery := []interface{}{
		float64(70),
		[]interface{}{
			float64(10),
			"empty_table",
		},
		"nonexistent",
	}

	doc, err := eval.Evaluate(ctx, getQuery)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if !doc.IsNull() {
		t.Errorf("expected null for non-existent document, got %v", doc)
	}
}

func TestEvaluator_Update(t *testing.T) {
	eval := NewEvaluator()
	ctx := context.Background()

	// Insert a document
	insertQuery := []interface{}{
		float64(17),
		[]interface{}{float64(10), "users"},
		map[string]interface{}{
			"id":   "user1",
			"name": "Alice",
			"age":  float64(30),
		},
	}
	_, err := eval.Evaluate(ctx, insertQuery)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	// Update the document
	updateQuery := []interface{}{
		float64(18),
		[]interface{}{float64(10), "users"},
		map[string]interface{}{
			"age": float64(31),
		},
	}
	result, err := eval.Evaluate(ctx, updateQuery)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if result.Object() == nil {
		t.Fatal("result should be an object")
	}

	// Get the updated document
	getQuery := []interface{}{
		float64(70),
		[]interface{}{float64(10), "users"},
		"user1",
	}
	doc, err := eval.Evaluate(ctx, getQuery)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	age, ok := doc.Object().Get("age")
	if !ok || age.Num() != 31 {
		t.Errorf("expected age=31, got %v", age)
	}
}

func TestEvaluator_Delete(t *testing.T) {
	eval := NewEvaluator()
	ctx := context.Background()

	// Insert documents
	for i := 0; i < 3; i++ {
		insertQuery := []interface{}{
			float64(17),
			[]interface{}{float64(10), "test_table"},
			map[string]interface{}{
				"id":    fmt.Sprintf("doc%d", i),
				"value": i,
			},
		}
		_, err := eval.Evaluate(ctx, insertQuery)
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	// Delete all documents
	deleteQuery := []interface{}{
		float64(19),
		[]interface{}{float64(10), "test_table"},
	}
	result, err := eval.Evaluate(ctx, deleteQuery)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if result.Object() == nil {
		t.Fatal("result should be an object")
	}

	deleted, ok := result.Object().Get("deleted")
	if !ok || deleted.Num() != 3 {
		t.Errorf("expected deleted=3, got %v", deleted)
	}

	// Verify table is empty
	getAllQuery := []interface{}{
		float64(78),
		[]interface{}{float64(10), "test_table"},
	}
	docs, err := eval.Evaluate(ctx, getAllQuery)
	if err != nil {
		t.Fatalf("get_all failed: %v", err)
	}

	if len(docs.Array()) != 0 {
		t.Errorf("expected 0 documents after delete, got %d", len(docs.Array()))
	}
}

func TestEvaluator_GetAll(t *testing.T) {
	eval := NewEvaluator()
	ctx := context.Background()

	// Insert documents
	for i := 0; i < 5; i++ {
		insertQuery := []interface{}{
			float64(17),
			[]interface{}{float64(10), "test_table"},
			map[string]interface{}{
				"id":    fmt.Sprintf("doc%d", i),
				"value": i * 10,
			},
		}
		_, err := eval.Evaluate(ctx, insertQuery)
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	// Get all documents
	getAllQuery := []interface{}{
		float64(78),
		[]interface{}{float64(10), "test_table"},
	}
	docs, err := eval.Evaluate(ctx, getAllQuery)
	if err != nil {
		t.Fatalf("get_all failed: %v", err)
	}

	if len(docs.Array()) != 5 {
		t.Errorf("expected 5 documents, got %d", len(docs.Array()))
	}
}

func TestEvaluator_Limit(t *testing.T) {
	eval := NewEvaluator()
	ctx := context.Background()

	// Insert documents
	for i := 0; i < 10; i++ {
		insertQuery := []interface{}{
			float64(17),
			[]interface{}{float64(10), "test_table"},
			map[string]interface{}{
				"id":    fmt.Sprintf("doc%d", i),
				"value": i,
			},
		}
		_, err := eval.Evaluate(ctx, insertQuery)
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	// Get all and limit to 3
	getAllQuery := []interface{}{
		float64(78),
		[]interface{}{float64(10), "test_table"},
	}
	allDocs, err := eval.Evaluate(ctx, getAllQuery)
	if err != nil {
		t.Fatalf("get_all failed: %v", err)
	}

	limitQuery := []interface{}{
		float64(42),
		allDocs,
		float64(3),
	}
	limitedDocs, err := eval.Evaluate(ctx, limitQuery)
	if err != nil {
		t.Fatalf("limit failed: %v", err)
	}

	if len(limitedDocs.Array()) != 3 {
		t.Errorf("expected 3 documents after limit, got %d", len(limitedDocs.Array()))
	}
}

func TestEvaluator_Skip(t *testing.T) {
	eval := NewEvaluator()
	ctx := context.Background()

	// Insert documents
	for i := 0; i < 10; i++ {
		insertQuery := []interface{}{
			float64(17),
			[]interface{}{float64(10), "test_table"},
			map[string]interface{}{
				"id":    fmt.Sprintf("doc%d", i),
				"value": i,
			},
		}
		_, err := eval.Evaluate(ctx, insertQuery)
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	// Get all and skip 5
	getAllQuery := []interface{}{
		float64(78),
		[]interface{}{float64(10), "test_table"},
	}
	allDocs, err := eval.Evaluate(ctx, getAllQuery)
	if err != nil {
		t.Fatalf("get_all failed: %v", err)
	}

	skipQuery := []interface{}{
		float64(43),
		allDocs,
		float64(5),
	}
	skippedDocs, err := eval.Evaluate(ctx, skipQuery)
	if err != nil {
		t.Fatalf("skip failed: %v", err)
	}

	if len(skippedDocs.Array()) != 5 {
		t.Errorf("expected 5 documents after skip, got %d", len(skippedDocs.Array()))
	}
}

func TestEvaluator_IndexCreate(t *testing.T) {
	eval := NewEvaluator()
	ctx := context.Background()

	// Insert documents
	for i := 0; i < 5; i++ {
		insertQuery := []interface{}{
			float64(17),
			[]interface{}{float64(10), "users"},
			map[string]interface{}{
				"id":   fmt.Sprintf("user%d", i),
				"name": fmt.Sprintf("User%d", i),
				"age":  float64(20 + i),
			},
		}
		_, err := eval.Evaluate(ctx, insertQuery)
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	// Create index on age field
	indexCreateQuery := []interface{}{
		float64(75),
		[]interface{}{float64(10), "users"},
		"age_index",
		"age",
	}
	result, err := eval.Evaluate(ctx, indexCreateQuery)
	if err != nil {
		t.Fatalf("index_create failed: %v", err)
	}

	created, ok := result.Object().Get("created")
	if !ok || created.Num() != 1 {
		t.Errorf("expected created=1, got %v", created)
	}
}

func TestEvaluator_IndexList(t *testing.T) {
	eval := NewEvaluator()
	ctx := context.Background()

	// Create indexes
	for i := 0; i < 3; i++ {
		indexCreateQuery := []interface{}{
			float64(75),
			[]interface{}{float64(10), "test_table"},
			fmt.Sprintf("index%d", i),
			fmt.Sprintf("field%d", i),
		}
		_, err := eval.Evaluate(ctx, indexCreateQuery)
		if err != nil {
			t.Fatalf("index_create %d failed: %v", i, err)
		}
	}

	// List indexes
	indexListQuery := []interface{}{
		float64(77),
		[]interface{}{float64(10), "test_table"},
	}
	result, err := eval.Evaluate(ctx, indexListQuery)
	if err != nil {
		t.Fatalf("index_list failed: %v", err)
	}

	if len(result.Array()) != 3 {
		t.Errorf("expected 3 indexes, got %d", len(result.Array()))
	}
}

func TestEvaluator_IndexDrop(t *testing.T) {
	eval := NewEvaluator()
	ctx := context.Background()

	// Create index
	indexCreateQuery := []interface{}{
		float64(75),
		[]interface{}{float64(10), "test_table"},
		"test_index",
		"field",
	}
	_, err := eval.Evaluate(ctx, indexCreateQuery)
	if err != nil {
		t.Fatalf("index_create failed: %v", err)
	}

	// Drop index
	indexDropQuery := []interface{}{
		float64(76),
		[]interface{}{float64(10), "test_table"},
		"test_index",
	}
	result, err := eval.Evaluate(ctx, indexDropQuery)
	if err != nil {
		t.Fatalf("index_drop failed: %v", err)
	}

	dropped, ok := result.Object().Get("dropped")
	if !ok || dropped.Num() != 1 {
		t.Errorf("expected dropped=1, got %v", dropped)
	}
}

func TestEvaluator_Between(t *testing.T) {
	eval := NewEvaluator()
	ctx := context.Background()

	// Insert documents
	for i := 0; i < 10; i++ {
		insertQuery := []interface{}{
			float64(17),
			[]interface{}{float64(10), "users"},
			map[string]interface{}{
				"id":   fmt.Sprintf("user%d", i),
				"name": fmt.Sprintf("User%d", i),
				"age":  float64(20 + i),
			},
		}
		_, err := eval.Evaluate(ctx, insertQuery)
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	// Create index on age
	indexCreateQuery := []interface{}{
		float64(75),
		[]interface{}{float64(10), "users"},
		"age_index",
		"age",
	}
	_, err := eval.Evaluate(ctx, indexCreateQuery)
	if err != nil {
		t.Fatalf("index_create failed: %v", err)
	}

	// Query with BETWEEN (age 23-26)
	betweenQuery := []interface{}{
		float64(172),
		[]interface{}{float64(10), "users"},
		"age_index",
		float64(23),
		float64(26),
	}
	result, err := eval.Evaluate(ctx, betweenQuery)
	if err != nil {
		t.Fatalf("between failed: %v", err)
	}

	if len(result.Array()) != 4 {
		t.Errorf("expected 4 documents, got %d", len(result.Array()))
	}
}

func TestEvaluator_Changes(t *testing.T) {
	eval := NewEvaluator()
	ctx := context.Background()

	// Create changefeed
	changesQuery := []interface{}{
		float64(152),
		[]interface{}{float64(10), "test_table"},
	}
	result, err := eval.Evaluate(ctx, changesQuery)
	if err != nil {
		t.Fatalf("changes failed: %v", err)
	}

	if result.Object() == nil {
		t.Fatal("result should be an object")
	}

	reqlType, ok := result.Object().Get("$reql_type$")
	if !ok || reqlType.Str() != "CHANGEFEED" {
		t.Errorf("expected CHANGEFEED type, got %v", reqlType)
	}
}
