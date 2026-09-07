package protocol

import (
	"encoding/json"
	"fmt"
)

// Query represents a ReQL query
type Query struct {
	Token int64       `json:"token"`
	Type  int64       `json:"type"`
	Query interface{} `json:"query,omitempty"`
}

// ParseQuery parses a query from JSON data
func ParseQuery(data []byte) (*Query, error) {
	var query Query
	if err := json.Unmarshal(data, &query); err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}
	return &query, nil
}

// Response represents a ReQL response
type Response struct {
	Type      int64       `json:"t"`
	Token     int64       `json:"token,omitempty"`
	Data      interface{} `json:"r"`
	Notes     []string    `json:"n,omitempty"`
	Profile   interface{} `json:"p,omitempty"`
	Backtrace interface{} `json:"b,omitempty"`
}

// Marshal marshals the response to JSON
func (r *Response) Marshal() ([]byte, error) {
	// Create response map
	resp := map[string]interface{}{
		"t": r.Type,
		"r": r.Data,
	}

	if len(r.Notes) > 0 {
		resp["n"] = r.Notes
	}
	if r.Profile != nil {
		resp["p"] = r.Profile
	}
	if r.Backtrace != nil {
		resp["b"] = r.Backtrace
	}

	return json.Marshal(resp)
}

// NewSuccessAtomResponse creates a SUCCESS_ATOM response
func NewSuccessAtomResponse(data interface{}) *Response {
	return &Response{
		Type: ResponseSuccessAtom,
		Data: []interface{}{data},
	}
}

// NewSuccessSeqResponse creates a SUCCESS_SEQUENCE response
func NewSuccessSeqResponse(data []interface{}) *Response {
	return &Response{
		Type: ResponseSuccessSeq,
		Data: data,
	}
}

// NewSuccessPartialResponse creates a SUCCESS_PARTIAL response
func NewSuccessPartialResponse(data []interface{}) *Response {
	return &Response{
		Type: ResponseSuccessPartial,
		Data: data,
	}
}

// NewErrorResponse creates an error response
func NewErrorResponse(errType int64, message string) *Response {
	return &Response{
		Type: errType,
		Data: []interface{}{message},
	}
}
