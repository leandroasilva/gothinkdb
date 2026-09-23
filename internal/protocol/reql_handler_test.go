package protocol

import (
	"context"
	"testing"

	"github.com/leandroasilva/gothinkdb/internal/reql"
	"github.com/leandroasilva/gothinkdb/pkg/datum"
)

func TestReQLHandler_HandleQuery(t *testing.T) {
	handler := NewReQLHandler(reql.NewEvaluator())
	ctx := context.Background()

	// Test TABLE query
	query := &Query{
		Token: 1,
		Type:  QueryStart,
		Query: []interface{}{float64(10), "test_table"},
	}

	response, err := handler.HandleQuery(ctx, nil, query)
	if err != nil {
		t.Fatalf("HandleQuery failed: %v", err)
	}

	if response.Type != ResponseSuccessAtom {
		t.Errorf("expected ResponseSuccessAtom, got %d", response.Type)
	}

	if dataArr, ok := response.Data.([]interface{}); !ok || len(dataArr) == 0 {
		t.Error("expected non-empty response data")
	}
}

func TestReQLHandler_InsertAndGet(t *testing.T) {
	handler := NewReQLHandler(reql.NewEvaluator())
	ctx := context.Background()

	// Insert document
	insertQuery := &Query{
		Token: 1,
		Type:  QueryStart,
		Query: []interface{}{
			float64(17), // INSERT
			[]interface{}{float64(10), "users"},
			map[string]interface{}{
				"id":   "user1",
				"name": "Alice",
				"age":  float64(30),
			},
		},
	}

	response, err := handler.HandleQuery(ctx, nil, insertQuery)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if response.Type != ResponseSuccessAtom {
		t.Errorf("expected ResponseSuccessAtom, got %d", response.Type)
	}

	// Get document
	getQuery := &Query{
		Token: 2,
		Type:  QueryStart,
		Query: []interface{}{
			float64(70), // GET
			[]interface{}{float64(10), "users"},
			"user1",
		},
	}

	response, err = handler.HandleQuery(ctx, nil, getQuery)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if response.Type != ResponseSuccessAtom {
		t.Errorf("expected ResponseSuccessAtom, got %d", response.Type)
	}

	dataArr, ok := response.Data.([]interface{})
	if !ok || len(dataArr) == 0 {
		t.Fatal("expected non-empty response data")
	}

	// Verify the document
	data := dataArr[0]
	if dataMap, ok := data.(map[string]interface{}); ok {
		if name, ok := dataMap["name"].(string); !ok || name != "Alice" {
			t.Errorf("expected name=Alice, got %v", dataMap["name"])
		}
		if age, ok := dataMap["age"].(float64); !ok || age != 30 {
			t.Errorf("expected age=30, got %v", dataMap["age"])
		}
	} else {
		t.Error("expected map response")
	}
}

func TestDatumToInterface(t *testing.T) {
	tests := []struct {
		name     string
		input    datum.Datum
		expected interface{}
	}{
		{
			name:     "null",
			input:    datum.NewNull(),
			expected: nil,
		},
		{
			name:     "bool",
			input:    datum.NewBool(true),
			expected: true,
		},
		{
			name:     "num",
			input:    datum.NewNum(42.5),
			expected: 42.5,
		},
		{
			name:     "str",
			input:    datum.NewStr("hello"),
			expected: "hello",
		},
		{
			name: "array",
			input: datum.NewArray([]datum.Datum{
				datum.NewNum(1),
				datum.NewNum(2),
			}),
			expected: []interface{}{1.0, 2.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := datumToInterface(tt.input)
			if err != nil {
				t.Fatalf("datumToInterface failed: %v", err)
			}

			if result == nil && tt.expected == nil {
				return
			}

			// For arrays, compare length
			if expectedArr, ok := tt.expected.([]interface{}); ok {
				if resultArr, ok := result.([]interface{}); ok {
					if len(resultArr) != len(expectedArr) {
						t.Errorf("expected array length %d, got %d", len(expectedArr), len(resultArr))
					}
					return
				}
			}

			// For simple types, direct comparison
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
