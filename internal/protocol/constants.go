// Package protocol implements the RethinkDB wire protocol for GoThinkDB.
// This package provides compatibility with existing RethinkDB drivers.
package protocol

// Protocol version magic numbers
const (
	// Version 1.0 (current, with SCRAM authentication)
	Version1_0 uint32 = 0x3f61bcd1

	// Legacy versions (for compatibility)
	Version0_4 uint32 = 0x5f75687a
	Version0_3 uint32 = 0x5f75687a
	Version0_2 uint32 = 0x5f75687a
	Version0_1 uint32 = 0x5f75687a
)

// Query types
const (
	QueryStart       int64 = 1
	QueryContinue    int64 = 2
	QueryStop        int64 = 3
	QueryNoReplyWait int64 = 4
	QueryServerInfo  int64 = 5
)

// Response types
const (
	ResponseSuccessAtom    int64 = 1
	ResponseSuccessSeq     int64 = 2
	ResponseSuccessPartial int64 = 3
	ResponseWaitComplete   int64 = 4
	ResponseClientError    int64 = 16
	ResponseCompileError   int64 = 17
	ResponseRuntimeError   int64 = 18
)

// Error types
const (
	ErrorInternal    int64 = 10000
	ErrorDuplicate   int64 = 20000
	ErrorNonExist    int64 = 30000
	ErrorPermission  int64 = 40000
	ErrorParam       int64 = 50000
	ErrorResource    int64 = 60000
	ErrorOpFailed    int64 = 70000
	ErrorOpInterrupt int64 = 80000
)

// Max concurrent queries per connection
const MaxQueriesPerConn = 1024
