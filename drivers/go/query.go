package gothinkdb

import (
	"encoding/json"
	"fmt"
)

// Query represents a ReQL query term.
type Query struct {
	term []interface{}
	conn *Conn
}

// NewQuery creates a new query from a term.
func NewQuery(term []interface{}, conn *Conn) *Query {
	return &Query{term: term, conn: conn}
}

// ToTerm returns the raw query term.
func (q *Query) ToTerm() []interface{} {
	return q.term
}

// Run executes the query on the given connection (or the bound connection).
func (q *Query) Run(conn ...*Conn) (json.RawMessage, error) {
	c := q.conn
	if len(conn) > 0 && conn[0] != nil {
		c = conn[0]
	}
	if c == nil {
		return nil, fmt.Errorf("no connection provided")
	}
	resp, err := c.Query(q.term)
	if err != nil {
		return nil, err
	}
	// The server wraps all responses in an array (RethinkDB convention).
	// Unwrap: if data is a single-element array, return the inner element.
	return unwrapResponse(resp.Data), nil
}

// unwrapResponse removes the outer array wrapper from server responses.
func unwrapResponse(data json.RawMessage) json.RawMessage {
	if len(data) == 0 {
		return data
	}
	// Check if it's an array
	trimmed := data[0]
	if trimmed != '[' {
		return data
	}
	// Try to parse as array
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		return data
	}
	// If single element, return it directly
	if len(arr) == 1 {
		return arr[0]
	}
	return data
}

// Run executes the query and decodes the result into the given value.
func (q *Query) RunResult(result interface{}, conn ...*Conn) error {
	data, err := q.Run(conn...)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, result)
}

// --- Transformations ---

// Filter adds a filter operation to the query.
func (q *Query) Filter(predicate interface{}) *Query {
	return NewQuery([]interface{}{TermFilter, q.term, predicate}, q.conn)
}

// Map adds a map operation to the query.
func (q *Query) Map(fn interface{}) *Query {
	return NewQuery([]interface{}{TermMap, q.term, fn}, q.conn)
}

// OrderBy adds an orderBy operation to the query.
func (q *Query) OrderBy(fields ...string) *Query {
	return NewQuery([]interface{}{TermOrderBy, q.term, fields}, q.conn)
}

// Limit adds a limit operation to the query.
func (q *Query) Limit(n int) *Query {
	return NewQuery([]interface{}{TermLimit, q.term, n}, q.conn)
}

// Skip adds a skip operation to the query.
func (q *Query) Skip(n int) *Query {
	return NewQuery([]interface{}{TermSkip, q.term, n}, q.conn)
}

// Between adds a between operation to the query (requires index name).
func (q *Query) Between(index string, lower, upper interface{}) *Query {
	return NewQuery([]interface{}{TermBetween, q.term, index, lower, upper}, q.conn)
}

// Pluck selects specific fields from the result.
func (q *Query) Pluck(fields ...string) *Query {
	return NewQuery([]interface{}{TermHasFields, q.term, fields}, q.conn)
}

// Without excludes specific fields from the result.
func (q *Query) Without(fields ...string) *Query {
	return NewQuery([]interface{}{TermWithout, q.term, fields}, q.conn)
}

// Merge merges additional fields into the result.
func (q *Query) Merge(other map[string]interface{}) *Query {
	return NewQuery([]interface{}{TermMerge, q.term, other}, q.conn)
}

// HasFields filters documents that have the specified fields.
func (q *Query) HasFields(fields ...string) *Query {
	return NewQuery([]interface{}{TermHasFields, q.term, fields}, q.conn)
}

// --- Mutations ---

// Update adds an update operation to the query.
func (q *Query) Update(changes map[string]interface{}) *Query {
	return NewQuery([]interface{}{TermUpdate, q.term, changes}, q.conn)
}

// Delete adds a delete operation to the query.
func (q *Query) Delete() *Query {
	return NewQuery([]interface{}{TermDelete, q.term}, q.conn)
}

// Replace adds a replace operation to the query.
func (q *Query) Replace(doc interface{}) *Query {
	return NewQuery([]interface{}{TermReplace, q.term, doc}, q.conn)
}

// --- Aggregations ---

// Count adds a count operation.
func (q *Query) Count() *Query {
	return NewQuery([]interface{}{TermCount, q.term}, q.conn)
}

// Sum adds a sum operation.
func (q *Query) Sum(field ...string) *Query {
	term := []interface{}{TermSum, q.term}
	if len(field) > 0 {
		term = append(term, field[0])
	}
	return NewQuery(term, q.conn)
}

// Avg adds an average operation.
func (q *Query) Avg(field ...string) *Query {
	term := []interface{}{TermAvg, q.term}
	if len(field) > 0 {
		term = append(term, field[0])
	}
	return NewQuery(term, q.conn)
}

// Min adds a min operation.
func (q *Query) Min(field ...string) *Query {
	term := []interface{}{TermMin, q.term}
	if len(field) > 0 {
		term = append(term, field[0])
	}
	return NewQuery(term, q.conn)
}

// Max adds a max operation.
func (q *Query) Max(field ...string) *Query {
	term := []interface{}{TermMax, q.term}
	if len(field) > 0 {
		term = append(term, field[0])
	}
	return NewQuery(term, q.conn)
}

// Group adds a group operation.
func (q *Query) Group(field string) *Query {
	return NewQuery([]interface{}{TermGroup, q.term, field}, q.conn)
}

// Ungroup adds an ungroup operation.
func (q *Query) Ungroup() *Query {
	return NewQuery([]interface{}{TermUngroup, q.term}, q.conn)
}

// Reduce adds a reduce operation.
func (q *Query) Reduce(fn interface{}) *Query {
	return NewQuery([]interface{}{TermReduce, q.term, fn}, q.conn)
}

// --- Joins ---

// InnerJoin adds an inner join operation.
func (q *Query) InnerJoin(other *Query, predicate interface{}) *Query {
	return NewQuery([]interface{}{TermInnerJoin, q.term, other.ToTerm(), predicate}, q.conn)
}

// OuterJoin adds an outer join operation.
func (q *Query) OuterJoin(other *Query, predicate interface{}) *Query {
	return NewQuery([]interface{}{TermOuterJoin, q.term, other.ToTerm(), predicate}, q.conn)
}

// --- Changefeeds ---

// Changes subscribes to changes on the query.
func (q *Query) Changes() *Query {
	return NewQuery([]interface{}{TermChanges, q.term}, q.conn)
}

// --- Table operations ---

// TableQuery represents a table reference with additional operations.
type TableQuery struct {
	*Query
}

// Table creates a new table query on a specific database.
func (d *DbQuery) Table(name string) *TableQuery {
	term := []interface{}{TermTable, name}
	return &TableQuery{Query: NewQuery(term, d.conn)}
}

// Get gets a document by primary key.
func (t *TableQuery) Get(id string) *Query {
	return NewQuery([]interface{}{TermGet, t.term, id}, t.conn)
}

// GetAll gets documents by secondary index keys.
func (t *TableQuery) GetAll(keys ...interface{}) *Query {
	term := append([]interface{}{TermGetAll, t.term}, keys...)
	return NewQuery(term, t.conn)
}

// Insert inserts a document or documents into the table.
func (t *TableQuery) Insert(doc interface{}) *Query {
	return NewQuery([]interface{}{TermInsert, t.term, doc}, t.conn)
}

// IndexCreate creates a secondary index.
func (t *TableQuery) IndexCreate(name string, indexFn ...interface{}) *Query {
	term := []interface{}{TermIndexCreate, t.term, name}
	if len(indexFn) > 0 {
		term = append(term, indexFn[0])
	}
	return NewQuery(term, t.conn)
}

// IndexDrop drops a secondary index.
func (t *TableQuery) IndexDrop(name string) *Query {
	return NewQuery([]interface{}{TermIndexDrop, t.term, name}, t.conn)
}

// IndexList lists all secondary indexes.
func (t *TableQuery) IndexList() *Query {
	return NewQuery([]interface{}{TermIndexList, t.term}, t.conn)
}

// --- Database operations ---

// DbQuery represents a database reference.
type DbQuery struct {
	term []interface{}
	conn *Conn
}

// TableCreate creates a new table in the database.
func (d *DbQuery) TableCreate(name string) *Query {
	return NewQuery([]interface{}{TermTableCreate, name, d.term}, d.conn)
}

// TableDrop drops a table from the database.
func (d *DbQuery) TableDrop(name string) *Query {
	return NewQuery([]interface{}{TermTableDrop, name, d.term}, d.conn)
}

// TableList lists all tables in the database.
func (d *DbQuery) TableList() *Query {
	return NewQuery([]interface{}{TermTableList, d.term}, d.conn)
}

// --- R (top-level query object) ---

// R is the top-level query builder, similar to rethinkdb's `r`.
type R struct {
	conn *Conn
	db   string
}

// NewR creates a new R query builder.
func NewR() *R {
	return &R{}
}

// NewRWithConn creates a new R bound to a connection.
func NewRWithConn(conn *Conn) *R {
	return &R{conn: conn, db: conn.DB()}
}

// Use sets the default database.
func (r *R) Use(db string) *R {
	r.db = db
	return r
}

// DB returns a database reference.
func (r *R) DB(name string) *DbQuery {
	return &DbQuery{
		term: []interface{}{TermDB, name},
		conn: r.conn,
	}
}

// Table returns a table reference using the default database.
func (r *R) Table(name string) *TableQuery {
	term := []interface{}{TermTable, name}
	return &TableQuery{Query: NewQuery(term, r.conn)}
}

// DBCreate creates a new database.
func (r *R) DBCreate(name string) *Query {
	return NewQuery([]interface{}{TermDBCreate, name}, r.conn)
}

// DBDrop drops a database.
func (r *R) DBDrop(name string) *Query {
	return NewQuery([]interface{}{TermDBDrop, name}, r.conn)
}

// DBList lists all databases.
func (r *R) DBList() *Query {
	return NewQuery([]interface{}{TermDBList}, r.conn)
}

// Expr wraps a value as a ReQL expression.
func (r *R) Expr(value interface{}) *Query {
	return NewQuery([]interface{}{TermDatum, value}, r.conn)
}
