package reql

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/leandroasilva/gothinkdb/pkg/datum"
)

type Evaluator struct {
	tables map[string]*Table
	admin  *AdminManager
}

type Table struct {
	Name        string
	Data        map[string]datum.Datum
	Indexes     *IndexManager
	Changefeeds []*Changefeed
	mu          sync.RWMutex
}

func NewEvaluator() *Evaluator {
	return &Evaluator{
		tables: make(map[string]*Table),
		admin:  NewAdminManager(),
	}
}

// GetAdmin returns the admin manager
func (e *Evaluator) GetAdmin() *AdminManager {
	return e.admin
}

func (e *Evaluator) GetOrCreateTable(name string) *Table {
	if table, ok := e.tables[name]; ok {
		return table
	}
	table := &Table{
		Name:        name,
		Data:        make(map[string]datum.Datum),
		Indexes:     NewIndexManager(),
		Changefeeds: make([]*Changefeed, 0),
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
	case 33: // HAS_FIELDS
		return e.evalHasFields(ctx, arr[1:])
	case 34: // WITHOUT
		return e.evalWithout(ctx, arr[1:])
	case 36: // MERGE
		return e.evalMerge(ctx, arr[1:])
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
	case 48: // INNER_JOIN
		return e.evalInnerJoin(ctx, arr[1:])
	case 49: // OUTER_JOIN
		return e.evalOuterJoin(ctx, arr[1:])
	case 70: // GET
		return e.evalGet(ctx, arr[1:])
	case 78: // GET_ALL
		return e.evalGetAll(ctx, arr[1:])
	case 75: // INDEX_CREATE
		return e.evalIndexCreate(ctx, arr[1:])
	case 76: // INDEX_DROP
		return e.evalIndexDrop(ctx, arr[1:])
	case 77: // INDEX_LIST
		return e.evalIndexList(ctx, arr[1:])
	case 86: // COUNT
		return e.evalCount(ctx, arr[1:])
	case 87: // SUM
		return e.evalSum(ctx, arr[1:])
	case 88: // AVG
		return e.evalAvg(ctx, arr[1:])
	case 89: // MIN
		return e.evalMin(ctx, arr[1:])
	case 90: // MAX
		return e.evalMax(ctx, arr[1:])
	case 91: // GROUP
		return e.evalGroup(ctx, arr[1:])
	case 92: // UNGROUP
		return e.evalUngroup(ctx, arr[1:])
	case 93: // REDUCE
		return e.evalReduce(ctx, arr[1:])
	case 172: // BETWEEN
		return e.evalBetween(ctx, arr[1:])
	case 152: // CHANGES
		return e.evalChanges(ctx, arr[1:])
	case 57: // DB_CREATE
		return e.evalDBCreate(ctx, arr[1:])
	case 58: // DB_DROP
		return e.evalDBDrop(ctx, arr[1:])
	case 59: // DB_LIST
		return e.evalDBList(ctx, arr[1:])
	case 60: // TABLE_CREATE
		return e.evalTableCreate(ctx, arr[1:])
	case 61: // TABLE_DROP
		return e.evalTableDrop(ctx, arr[1:])
	case 62: // TABLE_LIST
		return e.evalTableList(ctx, arr[1:])
	case 137: // STATUS
		return e.evalStatus(ctx, arr[1:])
	case 138: // INFO
		return e.evalInfo(ctx, arr[1:])
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
			"replaced":  datum.NewNum(float64(count)),
			"unchanged": datum.NewNum(0),
			"skipped":   datum.NewNum(0),
			"errors":    datum.NewNum(0),
		})), nil
	}

	// Handle single document: find by ID and update in-place
	if selection.Type() == datum.Object {
		if idDatum, ok := selection.Object().Get("id"); ok && idDatum.Type() == datum.Str {
			id := idDatum.Str()
			for _, table := range e.tables {
				if doc, exists := table.Data[id]; exists {
					if doc.Type() == datum.Object && updateObj.Type() == datum.Object {
						for _, key := range updateObj.Object().Keys() {
							val, _ := updateObj.Object().Get(key)
							doc.Object().Set(key, val)
						}
						table.Data[id] = doc
					}
					return datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
						"replaced":  datum.NewNum(1),
						"unchanged": datum.NewNum(0),
						"skipped":   datum.NewNum(0),
						"errors":    datum.NewNum(0),
					})), nil
				}
			}
		}
		// If not found by ID, update the datum in-place (best effort)
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

	// Handle single document: find and remove from any table by ID
	if selection.Type() == datum.Object {
		if idDatum, ok := selection.Object().Get("id"); ok && idDatum.Type() == datum.Str {
			id := idDatum.Str()
			for _, table := range e.tables {
				if _, exists := table.Data[id]; exists {
					delete(table.Data, id)
					return datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
						"deleted": datum.NewNum(1),
						"skipped": datum.NewNum(0),
						"errors":  datum.NewNum(0),
					})), nil
				}
			}
		}
	}

	return datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
		"deleted": datum.NewNum(0),
		"skipped": datum.NewNum(0),
		"errors":  datum.NewNum(0),
	})), nil
}

// evalReplace replaces documents in a table with a new document.
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

	// Handle table reference (replace all documents)
	if tableName, ok := e.getTableName(selection); ok {
		table := e.GetOrCreateTable(tableName)
		count := 0
		for id := range table.Data {
			table.Data[id] = replacement
			count++
		}
		return datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
			"replaced":  datum.NewNum(float64(count)),
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

// matchPredicate checks if a document matches an object predicate (field=value matching).
func matchPredicate(doc datum.Datum, predicate datum.Datum) bool {
	if doc.Type() != datum.Object || predicate.Type() != datum.Object {
		return false
	}
	docObj := doc.Object()
	predObj := predicate.Object()
	for _, key := range predObj.Keys() {
		predVal, _ := predObj.Get(key)
		docVal, ok := docObj.Get(key)
		if !ok {
			return false
		}
		if !docVal.Equal(predVal) {
			return false
		}
	}
	return true
}

// sequenceToArray converts a table reference or array into a flat array of documents.
func (e *Evaluator) sequenceToArray(seq datum.Datum) ([]datum.Datum, error) {
	if seq.Type() == datum.Array {
		return seq.Array(), nil
	}
	if seq.Type() == datum.Object {
		if tableName, ok := e.getTableName(seq); ok {
			table := e.GetOrCreateTable(tableName)
			result := make([]datum.Datum, 0, len(table.Data))
			for _, doc := range table.Data {
				result = append(result, doc)
			}
			return result, nil
		}
	}
	return nil, fmt.Errorf("not a sequence")
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

	predicate, err := e.evalArg(ctx, args[1])
	if err != nil {
		return datum.Datum{}, err
	}

	arr, err := e.sequenceToArray(sequence)
	if err != nil {
		return datum.Datum{}, err
	}

	result := make([]datum.Datum, 0)
	for _, doc := range arr {
		if matchPredicate(doc, predicate) {
			result = append(result, doc)
		}
	}

	return datum.NewArray(result), nil
}

// evalMap applies a transformation to each document.
// Since function serialization is not supported over the wire protocol,
// map returns the sequence as-is (drivers should do client-side mapping).
func (e *Evaluator) evalMap(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 2 {
		return datum.Datum{}, fmt.Errorf("MAP requires sequence and function")
	}

	sequence, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}

	// Function evaluation is not supported over the wire protocol;
	// return the sequence unchanged.
	arr, err := e.sequenceToArray(sequence)
	if err != nil {
		return sequence, nil
	}
	return datum.NewArray(arr), nil
}

// evalOrderBy sorts documents by the specified field(s).
func (e *Evaluator) evalOrderBy(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 2 {
		return datum.Datum{}, fmt.Errorf("ORDER_BY requires sequence and key")
	}

	sequence, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}

	arr, err := e.sequenceToArray(sequence)
	if err != nil {
		return datum.Datum{}, err
	}

	// Extract sort fields
	var fields []string
	if args[1].Type() == datum.Array {
		for _, f := range args[1].Array() {
			if f.Type() == datum.Str {
				fields = append(fields, f.Str())
			}
		}
	} else if args[1].Type() == datum.Str {
		fields = []string{args[1].Str()}
	}

	if len(fields) == 0 {
		return datum.NewArray(arr), nil
	}

	// Sort using datum.Compare on the first specified field
	sort.Slice(arr, func(i, j int) bool {
		for _, field := range fields {
			var vi, vj datum.Datum
			if arr[i].Type() == datum.Object {
				vi, _ = arr[i].Object().Get(field)
			}
			if arr[j].Type() == datum.Object {
				vj, _ = arr[j].Object().Get(field)
			}
			cmp := vi.Compare(vj)
			if cmp != 0 {
				return cmp < 0
			}
		}
		return false
	})

	return datum.NewArray(arr), nil
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

	arr, err := e.sequenceToArray(sequence)
	if err != nil {
		return datum.Datum{}, err
	}

	limit := int(count.Num())
	if limit >= len(arr) {
		return datum.NewArray(arr), nil
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

	arr, err := e.sequenceToArray(sequence)
	if err != nil {
		return datum.Datum{}, err
	}

	skip := int(count.Num())
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

// evalIndexCreate creates a secondary index
func (e *Evaluator) evalIndexCreate(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 3 {
		return datum.Datum{}, fmt.Errorf("INDEX_CREATE requires table, name, and field")
	}

	tableRef, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}

	tableName, ok := e.getTableName(tableRef)
	if !ok {
		return datum.Datum{}, fmt.Errorf("invalid table reference")
	}

	indexName := args[1]
	if indexName.Type() != datum.Str {
		return datum.Datum{}, fmt.Errorf("index name must be a string")
	}

	fieldName := args[2]
	if fieldName.Type() != datum.Str {
		return datum.Datum{}, fmt.Errorf("field name must be a string")
	}

	table := e.GetOrCreateTable(tableName)
	if err := table.Indexes.CreateIndex(indexName.Str(), fieldName.Str()); err != nil {
		return datum.Datum{}, err
	}

	// Build the index from existing data
	if err := table.Indexes.BuildIndex(indexName.Str(), table.Data); err != nil {
		return datum.Datum{}, err
	}

	return datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
		"created": datum.NewNum(1),
	})), nil
}

// evalIndexDrop drops a secondary index
func (e *Evaluator) evalIndexDrop(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 2 {
		return datum.Datum{}, fmt.Errorf("INDEX_DROP requires table and name")
	}

	tableRef, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}

	tableName, ok := e.getTableName(tableRef)
	if !ok {
		return datum.Datum{}, fmt.Errorf("invalid table reference")
	}

	indexName := args[1]
	if indexName.Type() != datum.Str {
		return datum.Datum{}, fmt.Errorf("index name must be a string")
	}

	table := e.GetOrCreateTable(tableName)
	if err := table.Indexes.DropIndex(indexName.Str()); err != nil {
		return datum.Datum{}, err
	}

	return datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
		"dropped": datum.NewNum(1),
	})), nil
}

// evalIndexList lists all secondary indexes
func (e *Evaluator) evalIndexList(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 1 {
		return datum.Datum{}, fmt.Errorf("INDEX_LIST requires table")
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
	indexes := table.Indexes.ListIndexes()

	result := make([]datum.Datum, len(indexes))
	for i, name := range indexes {
		result[i] = datum.NewStr(name)
	}

	return datum.NewArray(result), nil
}

// evalBetween queries documents in an index range
func (e *Evaluator) evalBetween(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 4 {
		return datum.Datum{}, fmt.Errorf("BETWEEN requires table, index, lower, and upper")
	}

	tableRef, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}

	tableName, ok := e.getTableName(tableRef)
	if !ok {
		return datum.Datum{}, fmt.Errorf("invalid table reference")
	}

	indexName := args[1]
	if indexName.Type() != datum.Str {
		return datum.Datum{}, fmt.Errorf("index name must be a string")
	}

	lower := args[2]
	upper := args[3]

	table := e.GetOrCreateTable(tableName)
	idx, exists := table.Indexes.GetIndex(indexName.Str())
	if !exists {
		return datum.Datum{}, fmt.Errorf("index %s does not exist", indexName.Str())
	}

	// Get document IDs from index
	docIDs := idx.Between(lower, upper)

	// Retrieve documents
	result := make([]datum.Datum, 0, len(docIDs))
	for _, docID := range docIDs {
		if doc, ok := table.Data[docID]; ok {
			result = append(result, doc)
		}
	}

	return datum.NewArray(result), nil
}

// evalChanges creates a changefeed
func (e *Evaluator) evalChanges(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 1 {
		return datum.Datum{}, fmt.Errorf("CHANGES requires table")
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
	cf := NewChangefeed(tableName)
	table.Changefeeds = append(table.Changefeeds, cf)

	// Return a changefeed reference
	return datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
		"$reql_type$": datum.NewStr("CHANGEFEED"),
		"table":       datum.NewStr(tableName),
	})), nil
}

// notifyChangefeeds notifies all changefeeds of a change
func (e *Evaluator) notifyChangefeeds(tableName string, oldDoc, newDoc datum.Datum) {
	table, ok := e.tables[tableName]
	if !ok {
		return
	}

	event := ChangeEvent{
		OldValue: oldDoc,
		NewValue: newDoc,
	}

	for _, cf := range table.Changefeeds {
		cf.Send(event)
	}
}

// evalDBCreate creates a new database
func (e *Evaluator) evalDBCreate(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 1 {
		return datum.Datum{}, fmt.Errorf("DB_CREATE requires database name")
	}

	dbName := args[0]
	if dbName.Type() != datum.Str {
		return datum.Datum{}, fmt.Errorf("database name must be a string")
	}

	if err := e.admin.CreateDatabase(dbName.Str()); err != nil {
		return datum.Datum{}, err
	}

	return datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
		"dbs_created": datum.NewNum(1),
	})), nil
}

// evalDBDrop drops a database
func (e *Evaluator) evalDBDrop(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 1 {
		return datum.Datum{}, fmt.Errorf("DB_DROP requires database name")
	}

	dbName := args[0]
	if dbName.Type() != datum.Str {
		return datum.Datum{}, fmt.Errorf("database name must be a string")
	}

	if err := e.admin.DropDatabase(dbName.Str()); err != nil {
		return datum.Datum{}, err
	}

	return datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
		"dbs_dropped": datum.NewNum(1),
	})), nil
}

// evalDBList lists all databases
func (e *Evaluator) evalDBList(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	databases := e.admin.ListDatabases()

	result := make([]datum.Datum, len(databases))
	for i, name := range databases {
		result[i] = datum.NewStr(name)
	}

	return datum.NewArray(result), nil
}

// evalTableCreate creates a new table
func (e *Evaluator) evalTableCreate(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 1 {
		return datum.Datum{}, fmt.Errorf("TABLE_CREATE requires table name")
	}

	tableName := args[0]
	if tableName.Type() != datum.Str {
		return datum.Datum{}, fmt.Errorf("table name must be a string")
	}

	// Default to "test" database if not specified
	dbName := "test"
	if len(args) > 1 {
		// Check if second arg is a database reference
		if args[1].Type() == datum.Object {
			if dbRef, ok := args[1].Object().Get("db"); ok {
				if dbRef.Type() == datum.Str {
					dbName = dbRef.Str()
				}
			}
		}
	}

	if err := e.admin.CreateTable(dbName, tableName.Str()); err != nil {
		return datum.Datum{}, err
	}

	return datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
		"tables_created": datum.NewNum(1),
	})), nil
}

// evalTableDrop drops a table
func (e *Evaluator) evalTableDrop(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 1 {
		return datum.Datum{}, fmt.Errorf("TABLE_DROP requires table name")
	}

	tableName := args[0]
	if tableName.Type() != datum.Str {
		return datum.Datum{}, fmt.Errorf("table name must be a string")
	}

	// Default to "test" database if not specified
	dbName := "test"
	if len(args) > 1 {
		// Check if second arg is a database reference
		if args[1].Type() == datum.Object {
			if dbRef, ok := args[1].Object().Get("db"); ok {
				if dbRef.Type() == datum.Str {
					dbName = dbRef.Str()
				}
			}
		}
	}

	if err := e.admin.DropTable(dbName, tableName.Str()); err != nil {
		return datum.Datum{}, err
	}

	return datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
		"tables_dropped": datum.NewNum(1),
	})), nil
}

// evalTableList lists all tables in a database
func (e *Evaluator) evalTableList(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	// Default to "test" database if not specified
	dbName := "test"
	if len(args) > 0 {
		// Check if arg is a database reference
		if args[0].Type() == datum.Object {
			if dbRef, ok := args[0].Object().Get("db"); ok {
				if dbRef.Type() == datum.Str {
					dbName = dbRef.Str()
				}
			}
		}
	}

	tables, err := e.admin.ListTables(dbName)
	if err != nil {
		return datum.Datum{}, err
	}

	result := make([]datum.Datum, len(tables))
	for i, name := range tables {
		result[i] = datum.NewStr(name)
	}

	return datum.NewArray(result), nil
}

// evalStatus gets server status
func (e *Evaluator) evalStatus(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	status := e.admin.GetServerStatus()

	result := make(map[string]datum.Datum)
	for k, v := range status {
		switch val := v.(type) {
		case string:
			result[k] = datum.NewStr(val)
		case int:
			result[k] = datum.NewNum(float64(val))
		default:
			result[k] = datum.NewStr(fmt.Sprintf("%v", val))
		}
	}

	return datum.NewObject(datum.NewObjectDataFromMap(result)), nil
}

// evalInfo gets server info
func (e *Evaluator) evalInfo(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	config := e.admin.GetConfig()

	result := make(map[string]datum.Datum)
	for k, v := range config {
		switch val := v.(type) {
		case string:
			result[k] = datum.NewStr(val)
		case int:
			result[k] = datum.NewNum(float64(val))
		default:
			result[k] = datum.NewStr(fmt.Sprintf("%v", val))
		}
	}

	return datum.NewObject(datum.NewObjectDataFromMap(result)), nil
}

// --- Aggregation functions ---

// evalCount returns the count of documents in a sequence.
func (e *Evaluator) evalCount(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 1 {
		return datum.Datum{}, fmt.Errorf("COUNT requires a sequence")
	}
	seq, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}
	arr, err := e.sequenceToArray(seq)
	if err != nil {
		return datum.Datum{}, err
	}
	return datum.NewNum(float64(len(arr))), nil
}

// evalSum returns the sum of a field across a sequence.
func (e *Evaluator) evalSum(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 1 {
		return datum.Datum{}, fmt.Errorf("SUM requires a sequence")
	}
	seq, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}
	arr, err := e.sequenceToArray(seq)
	if err != nil {
		return datum.Datum{}, err
	}
	var field string
	if len(args) > 1 && args[1].Type() == datum.Str {
		field = args[1].Str()
	}
	var sum float64
	for _, doc := range arr {
		if field != "" {
			if doc.Type() == datum.Object {
				if v, ok := doc.Object().Get(field); ok && v.Type() == datum.Num {
					sum += v.Num()
				}
			}
		} else if doc.Type() == datum.Num {
			sum += doc.Num()
		}
	}
	return datum.NewNum(sum), nil
}

// evalAvg returns the average of a field across a sequence.
func (e *Evaluator) evalAvg(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 1 {
		return datum.Datum{}, fmt.Errorf("AVG requires a sequence")
	}
	seq, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}
	arr, err := e.sequenceToArray(seq)
	if err != nil {
		return datum.Datum{}, err
	}
	var field string
	if len(args) > 1 && args[1].Type() == datum.Str {
		field = args[1].Str()
	}
	var sum float64
	var count int
	for _, doc := range arr {
		if field != "" {
			if doc.Type() == datum.Object {
				if v, ok := doc.Object().Get(field); ok && v.Type() == datum.Num {
					sum += v.Num()
					count++
				}
			}
		} else if doc.Type() == datum.Num {
			sum += doc.Num()
			count++
		}
	}
	if count == 0 {
		return datum.NewNum(0), nil
	}
	return datum.NewNum(sum / float64(count)), nil
}

// evalMin returns the minimum value of a field across a sequence.
func (e *Evaluator) evalMin(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 1 {
		return datum.Datum{}, fmt.Errorf("MIN requires a sequence")
	}
	seq, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}
	arr, err := e.sequenceToArray(seq)
	if err != nil {
		return datum.Datum{}, err
	}
	var field string
	if len(args) > 1 && args[1].Type() == datum.Str {
		field = args[1].Str()
	}
	var minVal datum.Datum
	first := true
	for _, doc := range arr {
		var v datum.Datum
		if field != "" {
			if doc.Type() == datum.Object {
				if fv, ok := doc.Object().Get(field); ok {
					v = fv
				} else {
					continue
				}
			} else {
				continue
			}
		} else {
			v = doc
		}
		if first || v.Compare(minVal) < 0 {
			minVal = v
			first = false
		}
	}
	if first {
		return datum.NewNull(), nil
	}
	return minVal, nil
}

// evalMax returns the maximum value of a field across a sequence.
func (e *Evaluator) evalMax(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 1 {
		return datum.Datum{}, fmt.Errorf("MAX requires a sequence")
	}
	seq, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}
	arr, err := e.sequenceToArray(seq)
	if err != nil {
		return datum.Datum{}, err
	}
	var field string
	if len(args) > 1 && args[1].Type() == datum.Str {
		field = args[1].Str()
	}
	var maxVal datum.Datum
	first := true
	for _, doc := range arr {
		var v datum.Datum
		if field != "" {
			if doc.Type() == datum.Object {
				if fv, ok := doc.Object().Get(field); ok {
					v = fv
				} else {
					continue
				}
			} else {
				continue
			}
		} else {
			v = doc
		}
		if first || v.Compare(maxVal) > 0 {
			maxVal = v
			first = false
		}
	}
	if first {
		return datum.NewNull(), nil
	}
	return maxVal, nil
}

// evalGroup groups documents by a field value.
func (e *Evaluator) evalGroup(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 2 {
		return datum.Datum{}, fmt.Errorf("GROUP requires sequence and field")
	}
	seq, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}
	arr, err := e.sequenceToArray(seq)
	if err != nil {
		return datum.Datum{}, err
	}
	field := ""
	if args[1].Type() == datum.Str {
		field = args[1].Str()
	}
	groups := make(map[string][]datum.Datum)
	var order []string
	for _, doc := range arr {
		key := "null"
		if field != "" && doc.Type() == datum.Object {
			if v, ok := doc.Object().Get(field); ok {
				b, _ := v.MarshalJSON()
				key = string(b)
			}
		}
		if _, seen := groups[key]; !seen {
			order = append(order, key)
		}
		groups[key] = append(groups[key], doc)
	}
	result := make([]datum.Datum, 0, len(order))
	for _, key := range order {
		group := datum.NewObject(datum.NewObjectDataFromMap(map[string]datum.Datum{
			"group":  datum.NewStr(key),
			"values": datum.NewArray(groups[key]),
		}))
		result = append(result, group)
	}
	return datum.NewArray(result), nil
}

// evalUngroup flattens grouped results back to an array.
func (e *Evaluator) evalUngroup(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 1 {
		return datum.Datum{}, fmt.Errorf("UNGROUP requires a sequence")
	}
	seq, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}
	return seq, nil
}

// evalReduce reduces a sequence to a single value.
// Since function serialization is not supported, returns the first element.
func (e *Evaluator) evalReduce(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 2 {
		return datum.Datum{}, fmt.Errorf("REDUCE requires sequence and function")
	}
	seq, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}
	arr, err := e.sequenceToArray(seq)
	if err != nil {
		return datum.Datum{}, err
	}
	if len(arr) == 0 {
		return datum.NewNull(), nil
	}
	return arr[0], nil
}

// --- Projection functions ---

// evalHasFields filters documents that have all the specified fields.
func (e *Evaluator) evalHasFields(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 2 {
		return datum.Datum{}, fmt.Errorf("HAS_FIELDS requires sequence and fields")
	}
	seq, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}
	arr, err := e.sequenceToArray(seq)
	if err != nil {
		return datum.Datum{}, err
	}
	var fields []string
	if args[1].Type() == datum.Array {
		for _, f := range args[1].Array() {
			if f.Type() == datum.Str {
				fields = append(fields, f.Str())
			}
		}
	}
	result := make([]datum.Datum, 0)
	for _, doc := range arr {
		if doc.Type() != datum.Object {
			continue
		}
		hasAll := true
		for _, f := range fields {
			if _, ok := doc.Object().Get(f); !ok {
				hasAll = false
				break
			}
		}
		if hasAll {
			result = append(result, doc)
		}
	}
	return datum.NewArray(result), nil
}

// evalWithout returns documents without the specified fields.
func (e *Evaluator) evalWithout(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 2 {
		return datum.Datum{}, fmt.Errorf("WITHOUT requires sequence and fields")
	}
	seq, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}
	arr, err := e.sequenceToArray(seq)
	if err != nil {
		return datum.Datum{}, err
	}
	var fields []string
	if args[1].Type() == datum.Array {
		for _, f := range args[1].Array() {
			if f.Type() == datum.Str {
				fields = append(fields, f.Str())
			}
		}
	}
	result := make([]datum.Datum, 0, len(arr))
	for _, doc := range arr {
		if doc.Type() != datum.Object {
			result = append(result, doc)
			continue
		}
		newObj := datum.NewObjectDataFromMap(nil)
		for _, key := range doc.Object().Keys() {
			skip := false
			for _, f := range fields {
				if key == f {
					skip = true
					break
				}
			}
			if !skip {
				v, _ := doc.Object().Get(key)
				newObj.Set(key, v)
			}
		}
		result = append(result, datum.NewObject(newObj))
	}
	return datum.NewArray(result), nil
}

// evalMerge merges additional fields into each document.
func (e *Evaluator) evalMerge(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 2 {
		return datum.Datum{}, fmt.Errorf("MERGE requires sequence and object")
	}
	seq, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}
	mergeObj, err := e.evalArg(ctx, args[1])
	if err != nil {
		return datum.Datum{}, err
	}

	// Single object merge (not a table reference)
	if seq.Type() == datum.Object {
		if _, isTable := e.getTableName(seq); !isTable {
			if mergeObj.Type() == datum.Object {
				newObj := datum.NewObjectDataFromMap(nil)
				for _, key := range seq.Object().Keys() {
					v, _ := seq.Object().Get(key)
					newObj.Set(key, v)
				}
				for _, key := range mergeObj.Object().Keys() {
					v, _ := mergeObj.Object().Get(key)
					newObj.Set(key, v)
				}
				return datum.NewObject(newObj), nil
			}
			return seq, nil
		}
	}

	arr, err := e.sequenceToArray(seq)
	if err != nil {
		return datum.Datum{}, err
	}
	result := make([]datum.Datum, 0, len(arr))
	for _, doc := range arr {
		if doc.Type() != datum.Object || mergeObj.Type() != datum.Object {
			result = append(result, doc)
			continue
		}
		newObj := datum.NewObjectDataFromMap(nil)
		for _, key := range doc.Object().Keys() {
			v, _ := doc.Object().Get(key)
			newObj.Set(key, v)
		}
		for _, key := range mergeObj.Object().Keys() {
			v, _ := mergeObj.Object().Get(key)
			newObj.Set(key, v)
		}
		result = append(result, datum.NewObject(newObj))
	}
	return datum.NewArray(result), nil
}

// --- Join functions ---

// evalInnerJoin performs an inner join between two sequences.
func (e *Evaluator) evalInnerJoin(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 3 {
		return datum.Datum{}, fmt.Errorf("INNER_JOIN requires left, right, and predicate")
	}
	leftSeq, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}
	rightSeq, err := e.evalArg(ctx, args[1])
	if err != nil {
		return datum.Datum{}, err
	}
	leftArr, err := e.sequenceToArray(leftSeq)
	if err != nil {
		return datum.Datum{}, err
	}
	rightArr, err := e.sequenceToArray(rightSeq)
	if err != nil {
		return datum.Datum{}, err
	}
	predicate := args[2]

	result := make([]datum.Datum, 0)
	for _, leftDoc := range leftArr {
		for _, rightDoc := range rightArr {
			if joinMatches(leftDoc, rightDoc, predicate) {
				merged := mergeJoinDocs(leftDoc, rightDoc)
				result = append(result, merged)
			}
		}
	}
	return datum.NewArray(result), nil
}

// evalOuterJoin performs a left outer join between two sequences.
func (e *Evaluator) evalOuterJoin(ctx context.Context, args []datum.Datum) (datum.Datum, error) {
	if len(args) < 3 {
		return datum.Datum{}, fmt.Errorf("OUTER_JOIN requires left, right, and predicate")
	}
	leftSeq, err := e.evalArg(ctx, args[0])
	if err != nil {
		return datum.Datum{}, err
	}
	rightSeq, err := e.evalArg(ctx, args[1])
	if err != nil {
		return datum.Datum{}, err
	}
	leftArr, err := e.sequenceToArray(leftSeq)
	if err != nil {
		return datum.Datum{}, err
	}
	rightArr, err := e.sequenceToArray(rightSeq)
	if err != nil {
		return datum.Datum{}, err
	}
	predicate := args[2]

	result := make([]datum.Datum, 0)
	for _, leftDoc := range leftArr {
		matched := false
		for _, rightDoc := range rightArr {
			if joinMatches(leftDoc, rightDoc, predicate) {
				merged := mergeJoinDocs(leftDoc, rightDoc)
				result = append(result, merged)
				matched = true
			}
		}
		if !matched {
			result = append(result, leftDoc)
		}
	}
	return datum.NewArray(result), nil
}

// joinMatches checks if two documents match based on a join predicate.
func joinMatches(left, right, predicate datum.Datum) bool {
	if predicate.Type() != datum.Object {
		return false
	}
	predObj := predicate.Object()
	for _, key := range predObj.Keys() {
		val, _ := predObj.Get(key)
		if val.Type() == datum.Str {
			field := val.Str()
			var lv, rv datum.Datum
			var lok, rok bool
			if left.Type() == datum.Object {
				lv, lok = left.Object().Get(field)
			}
			if right.Type() == datum.Object {
				rv, rok = right.Object().Get(field)
			}
			if !lok || !rok || !lv.Equal(rv) {
				return false
			}
		}
	}
	return true
}

// mergeJoinDocs merges two documents from a join.
func mergeJoinDocs(left, right datum.Datum) datum.Datum {
	if left.Type() != datum.Object || right.Type() != datum.Object {
		return left
	}
	merged := datum.NewObjectDataFromMap(nil)
	for _, key := range left.Object().Keys() {
		v, _ := left.Object().Get(key)
		merged.Set(key, v)
	}
	for _, key := range right.Object().Keys() {
		v, _ := right.Object().Get(key)
		if _, exists := merged.Get(key); exists {
			merged.Set("right_"+key, v)
		} else {
			merged.Set(key, v)
		}
	}
	return datum.NewObject(merged)
}
