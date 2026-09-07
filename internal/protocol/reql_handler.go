package protocol

import (
	"context"
	"log/slog"

	"github.com/leandroasilva/gothinkdb/internal/reql"
	"github.com/leandroasilva/gothinkdb/pkg/datum"
)

// ReQLHandler handles ReQL queries using the evaluator
type ReQLHandler struct {
	evaluator *reql.Evaluator
}

// NewReQLHandler creates a new ReQL handler
func NewReQLHandler() *ReQLHandler {
	return &ReQLHandler{
		evaluator: reql.NewEvaluator(),
	}
}

// HandleQuery handles incoming ReQL queries
func (h *ReQLHandler) HandleQuery(ctx context.Context, conn *Connection, query *Query) (*Response, error) {
	slog.Debug("handling ReQL query", "token", query.Token, "type", query.Type)

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

func (h *ReQLHandler) handleStart(ctx context.Context, conn *Connection, query *Query) (*Response, error) {
	// Extract the query from the first element
	if query.Query == nil {
		return nil, &ProtocolError{Code: ErrorParam, Message: "Empty query"}
	}

	// Evaluate the query
	result, err := h.evaluator.Evaluate(ctx, query.Query)
	if err != nil {
		slog.Error("query evaluation failed", "error", err)
		return &Response{
			Type:  ResponseRuntimeError,
			Notes: []string{err.Error()},
		}, nil
	}

	// Convert result to interface{} for response
	responseData, err := datumToInterface(result)
	if err != nil {
		slog.Error("failed to convert result", "error", err)
		return &Response{
			Type:  ResponseRuntimeError,
			Notes: []string{err.Error()},
		}, nil
	}

	return &Response{
		Type: ResponseSuccessAtom,
		Data: []interface{}{responseData},
	}, nil
}

func (h *ReQLHandler) handleContinue(ctx context.Context, conn *Connection, query *Query) (*Response, error) {
	// Cursor continuation - not yet implemented
	return &Response{
		Type: ResponseSuccessSeq,
		Data: []interface{}{},
	}, nil
}

func (h *ReQLHandler) handleStop(ctx context.Context, conn *Connection, query *Query) (*Response, error) {
	// Stop cursor - not yet implemented
	return &Response{
		Type: ResponseWaitComplete,
	}, nil
}

// datumToInterface converts a datum.Datum to interface{} for JSON serialization
func datumToInterface(d datum.Datum) (interface{}, error) {
	switch d.Type() {
	case datum.Null:
		return nil, nil
	case datum.Bool:
		return d.Bool(), nil
	case datum.Num:
		return d.Num(), nil
	case datum.Str:
		return d.Str(), nil
	case datum.Array:
		arr := d.Array()
		result := make([]interface{}, len(arr))
		for i, item := range arr {
			converted, err := datumToInterface(item)
			if err != nil {
				return nil, err
			}
			result[i] = converted
		}
		return result, nil
	case datum.Object:
		obj := d.Object()
		result := make(map[string]interface{})
		for _, key := range obj.Keys() {
			val, _ := obj.Get(key)
			converted, err := datumToInterface(val)
			if err != nil {
				return nil, err
			}
			result[key] = converted
		}
		return result, nil
	case datum.Binary:
		return d.Binary(), nil
	case datum.Time:
		return d.Time(), nil
	case datum.Geometry:
		return d.Geometry(), nil
	case datum.MinVal:
		return map[string]interface{}{
			"$reql_type$": "MINVAL",
		}, nil
	case datum.MaxVal:
		return map[string]interface{}{
			"$reql_type$": "MAXVAL",
		}, nil
	default:
		return nil, nil
	}
}
