package reql

import (
	"fmt"
)

// JoinType represents the type of join operation
type JoinType int

const (
	InnerJoin JoinType = iota
	LeftOuterJoin
	RightOuterJoin
	FullOuterJoin
)

// String returns the string representation of join type
func (j JoinType) String() string {
	switch j {
	case InnerJoin:
		return "inner"
	case LeftOuterJoin:
		return "left_outer"
	case RightOuterJoin:
		return "right_outer"
	case FullOuterJoin:
		return "full_outer"
	default:
		return "unknown"
	}
}

// JoinOperation represents a join operation
type JoinOperation struct {
	Type       JoinType
	LeftTable  string
	RightTable string
	LeftKey    string
	RightKey   string
	Predicate  func(left, right map[string]interface{}) bool
}

// Join performs a join operation between two tables
func (e *Evaluator) Join(leftTable, rightTable, leftKey, rightKey string, joinType JoinType) ([]map[string]interface{}, error) {
	// Get left table data
	leftData, err := e.getTableData(leftTable)
	if err != nil {
		return nil, fmt.Errorf("failed to get left table: %w", err)
	}

	// Get right table data
	rightData, err := e.getTableData(rightTable)
	if err != nil {
		return nil, fmt.Errorf("failed to get right table: %w", err)
	}

	// Build index on right table
	rightIndex := make(map[string][]map[string]interface{})
	for _, doc := range rightData {
		if key, ok := doc[rightKey]; ok {
			keyStr := fmt.Sprintf("%v", key)
			rightIndex[keyStr] = append(rightIndex[keyStr], doc)
		}
	}

	// Perform join
	var results []map[string]interface{}

	for _, leftDoc := range leftData {
		leftKeyValue, ok := leftDoc[leftKey]
		if !ok {
			continue
		}

		leftKeyStr := fmt.Sprintf("%v", leftKeyValue)
		matchingRightDocs := rightIndex[leftKeyStr]

		if len(matchingRightDocs) > 0 {
			// Match found
			for _, rightDoc := range matchingRightDocs {
				merged := mergeDocuments(leftDoc, rightDoc)
				results = append(results, merged)
			}
		} else if joinType == LeftOuterJoin || joinType == FullOuterJoin {
			// No match, but include left document with nulls for right
			merged := mergeDocumentsWithNulls(leftDoc, rightData)
			results = append(results, merged)
		}
	}

	// For right outer and full outer joins, include unmatched right documents
	if joinType == RightOuterJoin || joinType == FullOuterJoin {
		matchedRightKeys := make(map[string]bool)
		for _, leftDoc := range leftData {
			if leftKeyValue, ok := leftDoc[leftKey]; ok {
				matchedRightKeys[fmt.Sprintf("%v", leftKeyValue)] = true
			}
		}

		for _, rightDoc := range rightData {
			if rightKeyValue, ok := rightDoc[rightKey]; ok {
				rightKeyStr := fmt.Sprintf("%v", rightKeyValue)
				if !matchedRightKeys[rightKeyStr] {
					merged := mergeDocumentsWithNulls(rightDoc, leftData)
					results = append(results, merged)
				}
			}
		}
	}

	return results, nil
}

// getTableData gets all documents from a table
func (e *Evaluator) getTableData(tableName string) ([]map[string]interface{}, error) {
	table, exists := e.tables[tableName]
	if !exists {
		return nil, fmt.Errorf("table %s not found", tableName)
	}

	var docs []map[string]interface{}
	for _, doc := range table.Data {
		if docMap, ok := datumToMap(doc); ok {
			docs = append(docs, docMap)
		}
	}

	return docs, nil
}

// datumToMap converts a datum to a map
func datumToMap(doc interface{}) (map[string]interface{}, bool) {
	if m, ok := doc.(map[string]interface{}); ok {
		return m, true
	}
	return nil, false
}

// mergeDocuments merges two documents
func mergeDocuments(left, right map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})

	// Add left document fields
	for k, v := range left {
		merged["left_"+k] = v
	}

	// Add right document fields
	for k, v := range right {
		merged["right_"+k] = v
	}

	return merged
}

// mergeDocumentsWithNulls merges a document with nulls for missing fields
func mergeDocumentsWithNulls(doc map[string]interface{}, otherDocs []map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})

	// Add document fields
	for k, v := range doc {
		merged[k] = v
	}

	// Add null fields for other document structure
	if len(otherDocs) > 0 {
		for k := range otherDocs[0] {
			if _, exists := merged[k]; !exists {
				merged[k] = nil
			}
		}
	}

	return merged
}
