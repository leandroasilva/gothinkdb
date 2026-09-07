package reql

import (
	"fmt"
	"math"
)

// AggregationType represents the type of aggregation
type AggregationType int

const (
	AggCount AggregationType = iota
	AggSum
	AggAvg
	AggMin
	AggMax
	AggGroupBy
)

// String returns the string representation of aggregation type
func (a AggregationType) String() string {
	switch a {
	case AggCount:
		return "count"
	case AggSum:
		return "sum"
	case AggAvg:
		return "avg"
	case AggMin:
		return "min"
	case AggMax:
		return "max"
	case AggGroupBy:
		return "group_by"
	default:
		return "unknown"
	}
}

// Aggregate performs an aggregation operation on a sequence
func (e *Evaluator) Aggregate(data []map[string]interface{}, field string, aggType AggregationType) (interface{}, error) {
	switch aggType {
	case AggCount:
		return len(data), nil

	case AggSum:
		return e.sum(data, field)

	case AggAvg:
		return e.avg(data, field)

	case AggMin:
		return e.min(data, field)

	case AggMax:
		return e.max(data, field)

	default:
		return nil, fmt.Errorf("unsupported aggregation type: %v", aggType)
	}
}

// GroupBy performs a group by aggregation
func (e *Evaluator) GroupBy(data []map[string]interface{}, groupField, aggField string, aggType AggregationType) ([]map[string]interface{}, error) {
	// Group data by groupField
	groups := make(map[string][]map[string]interface{})
	for _, doc := range data {
		if groupValue, ok := doc[groupField]; ok {
			groupKey := fmt.Sprintf("%v", groupValue)
			groups[groupKey] = append(groups[groupKey], doc)
		}
	}

	// Perform aggregation on each group
	var results []map[string]interface{}
	for groupKey, groupData := range groups {
		aggResult, err := e.Aggregate(groupData, aggField, aggType)
		if err != nil {
			return nil, err
		}

		result := map[string]interface{}{
			groupField: groupKey,
			"value":    aggResult,
		}
		results = append(results, result)
	}

	return results, nil
}

// sum calculates the sum of a field
func (e *Evaluator) sum(data []map[string]interface{}, field string) (float64, error) {
	var sum float64
	for _, doc := range data {
		if value, ok := doc[field]; ok {
			if numValue, ok := toFloat64(value); ok {
				sum += numValue
			}
		}
	}
	return sum, nil
}

// avg calculates the average of a field
func (e *Evaluator) avg(data []map[string]interface{}, field string) (float64, error) {
	if len(data) == 0 {
		return 0, nil
	}

	sum, err := e.sum(data, field)
	if err != nil {
		return 0, err
	}

	return sum / float64(len(data)), nil
}

// min finds the minimum value of a field
func (e *Evaluator) min(data []map[string]interface{}, field string) (interface{}, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var minValue interface{}
	var minFloat float64
	first := true

	for _, doc := range data {
		if value, ok := doc[field]; ok {
			if first {
				minValue = value
				if numValue, ok := toFloat64(value); ok {
					minFloat = numValue
				}
				first = false
				continue
			}

			if numValue, ok := toFloat64(value); ok {
				if numValue < minFloat {
					minValue = value
					minFloat = numValue
				}
			}
		}
	}

	return minValue, nil
}

// max finds the maximum value of a field
func (e *Evaluator) max(data []map[string]interface{}, field string) (interface{}, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var maxValue interface{}
	var maxFloat float64
	first := true

	for _, doc := range data {
		if value, ok := doc[field]; ok {
			if first {
				maxValue = value
				if numValue, ok := toFloat64(value); ok {
					maxFloat = numValue
				}
				first = false
				continue
			}

			if numValue, ok := toFloat64(value); ok {
				if numValue > maxFloat {
					maxValue = value
					maxFloat = numValue
				}
			}
		}
	}

	return maxValue, nil
}

// toFloat64 converts an interface to float64
func toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

// Distinct returns distinct values from a sequence
func (e *Evaluator) Distinct(data []map[string]interface{}, field string) ([]interface{}, error) {
	seen := make(map[string]bool)
	var distinct []interface{}

	for _, doc := range data {
		if value, ok := doc[field]; ok {
			key := fmt.Sprintf("%v", value)
			if !seen[key] {
				seen[key] = true
				distinct = append(distinct, value)
			}
		}
	}

	return distinct, nil
}

// Contains checks if a sequence contains a value
func (e *Evaluator) Contains(data []map[string]interface{}, field string, value interface{}) (bool, error) {
	for _, doc := range data {
		if docValue, ok := doc[field]; ok {
			if fmt.Sprintf("%v", docValue) == fmt.Sprintf("%v", value) {
				return true, nil
			}
		}
	}
	return false, nil
}

// Reduce performs a reduce operation on a sequence
func (e *Evaluator) Reduce(data []map[string]interface{}, initial interface{}, reducer func(acc, val interface{}) interface{}) (interface{}, error) {
	acc := initial
	for _, doc := range data {
		acc = reducer(acc, doc)
	}
	return acc, nil
}

// Math operations

// Abs returns the absolute value
func Abs(x float64) float64 {
	return math.Abs(x)
}

// Ceil returns the ceiling of a number
func Ceil(x float64) float64 {
	return math.Ceil(x)
}

// Floor returns the floor of a number
func Floor(x float64) float64 {
	return math.Floor(x)
}

// Round rounds a number to the nearest integer
func Round(x float64) float64 {
	return math.Round(x)
}

// Sqrt returns the square root of a number
func Sqrt(x float64) float64 {
	return math.Sqrt(x)
}

// Pow returns x raised to the power y
func Pow(x, y float64) float64 {
	return math.Pow(x, y)
}
