// Design: docs/architecture/api/ipc_protocol.md — RPC wire message types
// Related: conn.go — Conn uses line format for RPC framing
// Related: framing.go — newline-delimited frame reader/writer
// Related: types.go — domain-specific RPC input/output types

package rpc

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Request represents a parsed incoming RPC request line: #<id> <method> [<json>].
type Request struct {
	ID     uint64          // Correlation ID from #<id> prefix
	Method string          // module:rpc-name
	Params json.RawMessage // JSON payload (may be nil)
}

// RPCCallError represents an error returned by the remote side via #<id> error [<json>].
type RPCCallError struct {
	Code    string // Short kebab-case identifier (may be empty)
	Message string // Human-readable detail
}

func (e *RPCCallError) Error() string {
	if e.Message != "" {
		return "rpc error: " + e.Message
	}
	if e.Code != "" {
		return "rpc error: " + e.Code
	}
	return "rpc error: (no message)"
}

// CodedError is a Go error that carries a short machine-readable code.
// Used to pass structured error information through the dispatch chain
// so that Dispatch can construct an error response with a proper code.
type CodedError struct {
	Code    string // Short kebab-case identifier (e.g., "unknown-command")
	message string
}

// NewCodedError creates an error with a code and human-readable message.
func NewCodedError(code, message string) *CodedError {
	return &CodedError{Code: code, message: message}
}

func (e *CodedError) Error() string { return e.message }

// ExtractErrorMessage extracts the human-readable message from error payload JSON.
// Returns the message if present, or empty string.
func ExtractErrorMessage(payload json.RawMessage) string {
	if len(payload) == 0 {
		return ""
	}
	var detail struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(payload, &detail) == nil {
		return detail.Message
	}
	return ""
}

// ParseLine parses a wire line into id, verb, and payload.
// Format: #<id> <verb> [<payload>].
func ParseLine(line []byte) (id uint64, verb string, payload []byte, err error) {
	s := string(line)
	if !strings.HasPrefix(s, "#") {
		return 0, "", nil, fmt.Errorf("line missing # prefix: %q", truncate(s, 80))
	}
	s = s[1:] // strip #

	// Extract ID
	idStr, rest, hasRest := strings.Cut(s, " ")
	id, err = strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, "", nil, fmt.Errorf("invalid id %q: %w", idStr, err)
	}

	if !hasRest || rest == "" {
		return 0, "", nil, fmt.Errorf("line has no verb after #%d", id)
	}

	// Extract verb and optional payload
	verb, payloadStr, _ := strings.Cut(rest, " ")
	if payloadStr != "" {
		payload = []byte(payloadStr)
	}

	return id, verb, payload, nil
}

// FormatRequest formats a request line: #<id> <method> [<json>].
func FormatRequest(id uint64, method string, params json.RawMessage) []byte {
	if len(params) == 0 || string(params) == "null" {
		return fmt.Appendf(nil, "#%d %s", id, method)
	}
	buf := make([]byte, 0, 2+20+1+len(method)+1+len(params))
	buf = append(buf, '#')
	buf = strconv.AppendUint(buf, id, 10)
	buf = append(buf, ' ')
	buf = append(buf, method...)
	buf = append(buf, ' ')
	buf = append(buf, params...)
	return buf
}

// FormatResult formats a success response: #<id> ok [<json>].
func FormatResult(id uint64, result json.RawMessage) []byte {
	if len(result) == 0 || string(result) == "null" {
		return FormatOK(id)
	}
	buf := make([]byte, 0, 2+20+4+len(result))
	buf = append(buf, '#')
	buf = strconv.AppendUint(buf, id, 10)
	buf = append(buf, ' ', 'o', 'k', ' ')
	buf = append(buf, result...)
	return buf
}

// FormatOK formats an empty success response: #<id> ok.
func FormatOK(id uint64) []byte {
	buf := make([]byte, 0, 2+20+3)
	buf = append(buf, '#')
	buf = strconv.AppendUint(buf, id, 10)
	buf = append(buf, ' ', 'o', 'k')
	return buf
}

// FormatError formats an error response: #<id> error [<json>].
func FormatError(id uint64, errPayload json.RawMessage) []byte {
	if len(errPayload) == 0 {
		buf := make([]byte, 0, 2+20+6)
		buf = append(buf, '#')
		buf = strconv.AppendUint(buf, id, 10)
		buf = append(buf, " error"...)
		return buf
	}
	buf := make([]byte, 0, 2+20+7+len(errPayload))
	buf = append(buf, '#')
	buf = strconv.AppendUint(buf, id, 10)
	buf = append(buf, " error "...)
	buf = append(buf, errPayload...)
	return buf
}

// NewErrorPayload creates a JSON error payload with code and message fields.
func NewErrorPayload(code, message string) json.RawMessage {
	data, _ := json.Marshal(struct {
		Code    string `json:"code,omitempty"`
		Message string `json:"message"`
	}{Code: code, Message: message})
	return data
}

// parseRPCError parses an error payload JSON into an RPCCallError.
func parseRPCError(payload []byte) *RPCCallError {
	if len(payload) == 0 {
		return &RPCCallError{}
	}
	var detail struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(payload, &detail) == nil {
		return &RPCCallError{Code: detail.Code, Message: detail.Message}
	}
	// Payload is not JSON — use it as the message directly.
	return &RPCCallError{Message: string(payload)}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
