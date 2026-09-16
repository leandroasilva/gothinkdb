package gothinkdb

import "encoding/json"

// ResponseType represents the type of a server response.
type ResponseType int

const (
	// ResponseTypeSuccessAtom - single value response
	ResponseTypeSuccessAtom ResponseType = 1
	// ResponseTypeSuccessSequence - sequence response
	ResponseTypeSuccessSequence ResponseType = 2
	// ResponseTypeSuccessPartial - partial sequence
	ResponseTypeSuccessPartial ResponseType = 3
	// ResponseTypeWaitComplete - changefeed confirmation
	ResponseTypeWaitComplete ResponseType = 4
	// ResponseTypeClientError - client-side error
	ResponseTypeClientError ResponseType = 16
	// ResponseTypeCompileError - query compilation error
	ResponseTypeCompileError ResponseType = 17
	// ResponseTypeRuntimeError - runtime error
	ResponseTypeRuntimeError ResponseType = 18
)

// Response represents a server response to a query.
type Response struct {
	Type    int             `json:"t"`
	Token   uint64          `json:"token,omitempty"`
	Data    json.RawMessage `json:"r,omitempty"`
	Notes   []string        `json:"n,omitempty"`
	Profile interface{}     `json:"p,omitempty"`
	Error   string          `json:"e,omitempty"`
}

// ChangeEvent represents a changefeed event.
type ChangeEvent struct {
	NewVal interface{} `json:"new_val"`
	OldVal interface{} `json:"old_val"`
}

// WriteResult represents the result of an insert/update/delete operation.
type WriteResult struct {
	Inserted int `json:"inserted"`
	Updated  int `json:"updated"`
	Deleted  int `json:"deleted"`
	Replaced int `json:"replaced"`
	Errors   int `json:"errors"`
}

// TermType constants matching the GoThinkDB evaluator term numbers.
const (
	TermDatum       = 1
	TermMakeArray   = 2
	TermMakeObj     = 3
	TermHasFields   = 33
	TermWithout     = 34
	TermMerge       = 36
	TermFilter      = 39
	TermMap         = 40
	TermOrderBy     = 41
	TermLimit       = 42
	TermSkip        = 43
	TermInnerJoin   = 48
	TermOuterJoin   = 49
	TermTable       = 10
	TermInsert      = 17
	TermUpdate      = 18
	TermDelete      = 19
	TermReplace     = 20
	TermCount       = 86
	TermSum         = 87
	TermAvg         = 88
	TermMin         = 89
	TermMax         = 90
	TermGroup       = 91
	TermUngroup     = 92
	TermReduce      = 93
	TermGet         = 70
	TermGetAll      = 78
	TermIndexCreate = 75
	TermIndexDrop   = 76
	TermIndexList   = 77
	TermDB          = 14
	TermDBCreate    = 57
	TermDBDrop      = 58
	TermDBList      = 59
	TermTableCreate = 60
	TermTableDrop   = 61
	TermTableList   = 62
	TermChanges     = 152
	TermBetween     = 172
)
