package protocol

import (
	"context"
	"log/slog"
)

// DefaultHandler is a basic query handler that returns simple responses
type DefaultHandler struct{}

// NewDefaultHandler creates a new default handler
func NewDefaultHandler() *DefaultHandler {
	return &DefaultHandler{}
}

// HandleQuery handles incoming queries
func (h *DefaultHandler) HandleQuery(ctx context.Context, conn *Connection, query *Query) (*Response, error) {
	slog.Debug("handling query", "token", query.Token, "type", query.Type)

	switch query.Type {
	case QueryStart:
		return h.handleStart(ctx, conn, query)
	case QueryContinue:
		return h.handleContinue(ctx, conn, query)
	case QueryStop:
		return h.handleStop(ctx, conn, query)
	case QueryNoReplyWait:
		return &Response{Type: ResponseWaitComplete}, nil
	case QueryServerInfo:
		return &Response{
			Type: ResponseSuccessAtom,
			Data: []interface{}{
				map[string]interface{}{
					"id":      "gothinkdb-server",
					"name":    "gothinkdb",
					"version": "0.1.0",
				},
			},
		}, nil
	default:
		return nil, ErrUnknownQueryType
	}
}

func (h *DefaultHandler) handleStart(ctx context.Context, conn *Connection, query *Query) (*Response, error) {
	// For now, return a simple response
	// In the future, this will parse and execute ReQL queries
	return &Response{
		Type: ResponseSuccessAtom,
		Data: []interface{}{"GoThinkDB is ready!"},
	}, nil
}

func (h *DefaultHandler) handleContinue(ctx context.Context, conn *Connection, query *Query) (*Response, error) {
	// Cursor continuation - not yet implemented
	return &Response{
		Type: ResponseSuccessSeq,
		Data: []interface{}{},
	}, nil
}

func (h *DefaultHandler) handleStop(ctx context.Context, conn *Connection, query *Query) (*Response, error) {
	// Stop cursor - not yet implemented
	return &Response{
		Type: ResponseWaitComplete,
	}, nil
}

// Error types
var (
	ErrUnknownQueryType = &ProtocolError{Code: ErrorParam, Message: "Unknown query type"}
	ErrDatumTooLarge    = &ProtocolError{Code: ErrorParam, Message: "Datum too large"}
)

// ProtocolError represents a protocol-level error
type ProtocolError struct {
	Code    int64
	Message string
}

func (e *ProtocolError) Error() string {
	return e.Message
}
