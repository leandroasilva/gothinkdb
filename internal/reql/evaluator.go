package reql

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/leandroasilva/gothinkdb/pkg/datum"
)

type Evaluator struct {
	tables map[string]*Table
}

type Table struct {
	Name string
	Data map[string]datum.Datum
}

func NewEvaluator() *Evaluator {
	return &Evaluator{
		tables: make(map[string]*Table),
	}
}

func (e *Evaluator) GetOrCreateTable(name string) *Table {
	if table, ok := e.tables[name]; ok {
		return table
	}
	table := &Table{
		Name: name,
		Data: make(map[string]datum.Datum),
	}
	e.tables[name] = table
	return table
}

func (e *Evaluator) Evaluate(ctx context.Context, query interface{}) (datum.Datum, error) {
	// If query is already a datum.Datum, use it directly
	if qd, ok := query.(datum.Datum); ok {
		return e.evaluateDatum(ctx, qd)
	}

	var queryDatum datum.Datum
	if err := queryDatum.FromInterface(query); err != nil {
		return datum.Datum{}, fmt.Errorf("failed to convert query: %w", err)
	}
	return e.evaluateDatum(ctx, queryDatum)
}

func (e *Evaluator) evaluateDatum(ctx context.Context, query datum.Datum) (datum.Datum, error) {
	if query.Type() != datum.Array {
		return datum.Datum{}, fmt.Errorf("query must be an array")
	}

	arr := query.Array()
	if len(arr) == 0 {
		return datum.Datum{}, fmt.Errorf("empty query")
	}

	term := int(arr[0].Num())

	switch term {
	case 10: // TABLE
		return e.evalTable(ctx, arr[1:])
	case 17: // INSERT
		return e.evalInsert(ctx, arr[1:])
	case 18: // UPDATE
		return e.evalUpdate(ctx, arr[1:])
	case 19: // DELETE
		return e.evalDelete(ctx, arr[1:])
	case 20: // REPLACE
		return e.evalReplace(ctx, arr[1:])
	case 39: // FILTER
		return e.evalFilter(ctx, arr[1:])
	case 40: // MAP
		return e.evalMap(ctx, arr[1:])
	case 41: // ORDER_BY
		return e.evalOrderBy(ctx, arr[1:])
	case 42: // LIMIT
		return e.evalLimit(ctx, arr[1:])
	case 43: // SKIP
		return e.evalSkip(ctx, arr[1:])
	case 70: // GET
		return e.evalGet(ctx, arr[1:])
	case 78: // GET_ALL
		return e.evalGetAll(ctx, arr[1:])
	default:
		return datum.Datum{}, fmt.Errorf("unsupported term: %d", term)
	}
}

func (e *Evaluator) evalArg(ctx context.Context, arg datum.Datum) (datum.Datum, error) {
	// If it's an array starting with a number, it's a query
	if arg.Type() == datum.Array {
		arr := arg.Array()
		if len(arr) > 0 && arr[0].Type() == datum.Num {
			return e.evaluateDatum(ctx, arg)
		}
	}
	// Otherwise, return it as-is
	return arg, nil
}

func (e *Evaluator) evalTable(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) == 0 || args[0].Type() != datum.Str {
		return datum.Datum{}, fmt.Errorf("TABLE requires table name")
	}

	tableName := args[0].Str()
	e.GetOrCreateTable(tableName)

	// Return a table reference
	obj := datum.NewObjectDataFromMap(map[string]datum.Datum{
		"$reql_type$": datum.NewStr("TABLE"),
		"table":       datum.NewStr(tableName),
	})
	return datum.NewObject(obj), nil
}

func (e *Evaluator) evalInsert(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 2 {
		return datum.Datum{}, fmt.Errorf("INSERT requires table and document")
	}

	tableRef, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}

	tableName, ok := e.getTableName(tableRef)
	if !ok {
		return datum.Datum{}, fmt.Errorf("invalid table reference")
	}

	table := e.GetOrCreateTable(tableName)

	doc, err := e.evalArg(ctx, args[1])
	if err != nil {
		return datum.Datum{}, err
	}

	id := fmt.Sprintf("auto_%d", len(table.Data)+1)
	if doc.Type() == datum.Object {
		obj := doc.Object()
		if idDatum, ok := obj.Get("id"); ok && idDatum.Type() == datum.Str {
			id = idDatum.Str()
		} else {
			obj.Set("id", datum.NewStr(id))
		}
	}

	table.Data[id] = doc

	result := datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
		"inserted":       datum.NewNum(1),
		"generated_keys": datum.NewArray([]datum.Datum{datum.NewStr(id)}),
	}))

	return result, nil
}

func (e *Evaluator) evalGet(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 2 {
		return datum.Datum{}, fmt.Errorf("GET requires table and key")
	}

	tableRef, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}

	tableName, ok := e.getTableName(tableRef)
	if !ok {
		return datum.Datum{}, fmt.Errorf("invalid table reference")
	}

	table := e.GetOrCreateTable(tableName)

	keyDatum := args[1]
	if keyDatum.Type() != datum.Str {
		return datum.Datum{}, fmt.Errorf("key must be a string")
	}

	if doc, ok := table.Data[keyDatum.Str()]; ok {
		return doc, nil
	}

	return datum.NewNull(), nil
}

func (e *Evaluator) getTableName(tableRef datum.Datum) (string, bool) {
	if tableRef.Type() != datum.Object {
		return "", false
	}
	obj := tableRef.Object()
	tableNameDatum, ok := obj.Get("table")
	if !ok || tableNameDatum.Type() != datum.Str {
		return "", false
	}
	return tableNameDatum.Str(), true
}

func (e *Evaluator) ExecuteJSON(ctx context.Context, queryJSON []byte) ([]byte, error) {
	var query interface{}
	if err := json.Unmarshal(queryJSON, &query); err != nil {
		return nil, fmt.Errorf("failed to parse query JSON: %w", err)
	}

	result, err := e.Evaluate(ctx, query)
	if err != nil {
		return nil, err
	}

	return result.MarshalJSON()
}

// evalUpdate updates documents in a table
func (e *Evaluator) evalUpdate(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 2 {
		return datum.Datum{}, fmt.Errorf("UPDATE requires selection and update object")
	}

	selection, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}

	updateObj, err := e.evalArg(ctx, args[1])
	if err != nil {
		return datum.Datum{}, err
	}

	// Handle table reference (update all documents)
	if tableName, ok := e.getTableName(selection); ok {
		table := e.GetOrCreateTable(tableName)
		count := 0
		for id, doc := range table.Data {
			if doc.Type() == datum.Object {
				if updateObj.Type() == datum.Object {
					for _, key := range updateObj.Object().Keys() {
						val, _ := updateObj.Object().Get(key)
						doc.Object().Set(key, val)
					}
					table.Data[id] = doc
					count++
				}
			}
		}
		return datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
			"replaced":  datum.NewNum(0),
			"unchanged": datum.NewNum(0),
			"skipped":   datum.NewNum(0),
			"errors":    datum.NewNum(0),
		})), nil
	}

	// Handle single document
	if selection.Type() == datum.Object {
		if updateObj.Type() == datum.Object {
			for _, key := range updateObj.Object().Keys() {
				val, _ := updateObj.Object().Get(key)
				selection.Object().Set(key, val)
			}
		}
		return datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
			"replaced":  datum.NewNum(1),
			"unchanged": datum.NewNum(0),
			"skipped":   datum.NewNum(0),
			"errors":    datum.NewNum(0),
		})), nil
	}

	return datum.Datum{}, fmt.Errorf("UPDATE: invalid selection type")
}

// evalDelete deletes documents from a table
func (e *Evaluator) evalDelete(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 1 {
		return datum.Datum{}, fmt.Errorf("DELETE requires selection")
	}

	selection, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}

	// Handle table reference (delete all documents)
	if tableName, ok := e.getTableName(selection); ok {
		table := e.GetOrCreateTable(tableName)
		count := len(table.Data)
		table.Data = make(map[string]datum.Datum)
		return datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
			"deleted": datum.NewNum(float64(count)),
			"skipped": datum.NewNum(0),
			"errors":  datum.NewNum(0),
		})), nil
	}

	return datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
		"deleted": datum.NewNum(0),
		"skipped": datum.NewNum(0),
		"errors":  datum.NewNum(0),
	})), nil
}

// evalReplace replaces documents in a table
func (e *Evaluator) evalReplace(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 2 {
		return datum.Datum{}, fmt.Errorf("REPLACE requires selection and replacement object")
	}

	selection, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}

	replacement, err := e.evalArg(ctx, args[1])
	if err != nil {
		return datum.Datum{}, err
	}

	// Handle single document
	if selection.Type() == datum.Object {
		_ = replacement // TODO: implement full replacement
		return datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
			"replaced":  datum.NewNum(1),
			"unchanged": datum.NewNum(0),
			"skipped":   datum.NewNum(0),
			"errors":    datum.NewNum(0),
		})), nil
	}

	return datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
		"replaced":  datum.NewNum(0),
		"unchanged": datum.NewNum(0),
		"skipped":   datum.NewNum(0),
		"errors":    datum.NewNum(0),
	})), nil
}

// evalFilter filters documents based on a predicate
func (e *Evaluator) evalFilter(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 2 {
		return datum.Datum{}, fmt.Errorf("FILTER requires sequence and predicate")
	}

	sequence, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}

	_ = sequence // TODO: implement predicate evaluation
	predicate, err := e.evalArg(ctx, args[1])
	if err != nil {
		return datum.Datum{}, err
	}

	_ = predicate // For now, return empty array (predicate evaluation is complex)
	return datum.NewArray([]datum.Datum{}), nil
}

// evalMap applies a function to each document
func (e *Evaluator) evalMap(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 2 {
		return datum.Datum{}, fmt.Errorf("MAP requires sequence and function")
	}

	sequence, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}

	_ = sequence // TODO: implement function evaluation
	// For now, return empty array (function evaluation is complex)
	return datum.NewArray([]datum.Datum{}), nil
}

// evalOrderBy sorts documents
func (e *Evaluator) evalOrderBy(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 2 {
		return datum.Datum{}, fmt.Errorf("ORDER_BY requires sequence and key")
	}

	sequence, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}

	// For now, return the sequence as-is
	return sequence, nil
}

// evalLimit limits the number of documents
func (e *Evaluator) evalLimit(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 2 {
		return datum.Datum{}, fmt.Errorf("LIMIT requires sequence and count")
	}

	sequence, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}

	count, err := e.evalArg(ctx, args[1])
	if err != nil {
		return datum.Datum{}, err
	}

	if sequence.Type() != datum.Array {
		return datum.Datum{}, fmt.Errorf("LIMIT: sequence must be an array")
	}

	limit := int(count.Num())
	arr := sequence.Array()
	if limit >= len(arr) {
		return sequence, nil
	}

	return datum.NewArray(arr[:limit]), nil
}

// evalSkip skips a number of documents
func (e *Evaluator) evalSkip(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 2 {
		return datum.Datum{}, fmt.Errorf("SKIP requires sequence and count")
	}

	sequence, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}

	count, err := e.evalArg(ctx, args[1])
	if err != nil {
		return datum.Datum{}, err
	}

	if sequence.Type() != datum.Array {
		return datum.Datum{}, fmt.Errorf("SKIP: sequence must be an array")
	}

	skip := int(count.Num())
	arr := sequence.Array()
	if skip >= len(arr) {
		return datum.NewArray([]datum.Datum{}), nil
	}

	return datum.NewArray(arr[skip:]), nil
}

// evalGetAll retrieves all documents matching keys
func (e *Evaluator) evalGetAll(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 1 {
		return datum.Datum{}, fmt.Errorf("GET_ALL requires table")
	}

	tableRef, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}

	tableName, ok := e.getTableName(tableRef)
	if !ok {
		return datum.Datum{}, fmt.Errorf("invalid table reference")
	}

	table := e.GetOrCreateTable(tableName)

	// If no keys specified, return all documents
	if len(args) == 1 {
		result := datum.NewArray([]datum.Datum{})
		for _, doc := range table.Data {
			result = datum.NewArray(append(result.Array(), doc))
		}
		return result, nil
	}

	// Return documents matching the specified keys
	result := datum.NewArray([]datum.Datum{})
	for i := 1; i < len(args); i++ {
		keyDatum := args[i]
		if keyDatum.Type() == datum.Str {
			if doc, ok := table.Data[keyDatum.Str()]; ok {
				result = datum.NewArray(append(result.Array(), doc))
			}
		}
	}

	return result, nil
}
