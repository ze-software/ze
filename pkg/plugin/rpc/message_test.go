package rpc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseLine verifies parsing of the #<id> <verb> [<payload>] wire format.
//
// VALIDATES: ParseLine correctly extracts id, verb, and optional payload.
// PREVENTS: Incorrect parsing of the unified line format.
func TestParseLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		line        string
		wantID      uint64
		wantVerb    string
		wantPayload string
		wantErr     bool
	}{
		{"request with params", `#1 test-method {"key":"value"}`, 1, "test-method", `{"key":"value"}`, false},
		{"request no params", "#42 ping", 42, "ping", "", false},
		{"ok with payload", `#5 ok {"result":"done"}`, 5, "ok", `{"result":"done"}`, false},
		{"ok no payload", "#3 ok", 3, "ok", "", false},
		{"error with payload", `#7 error {"code":"not-found","message":"peer not found"}`, 7, "error", `{"code":"not-found","message":"peer not found"}`, false},
		{"error no payload", "#9 error", 9, "error", "", false},
		{"large id", "#18446744073709551615 method", 18446744073709551615, "method", "", false},
		{"missing hash prefix", "1 method", 0, "", "", true},
		{"no verb", "#1", 0, "", "", true},
		{"invalid id", "#abc method", 0, "", "", true},
		{"empty after hash", "#", 0, "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			id, verb, payload, err := ParseLine([]byte(tt.line))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
			assert.Equal(t, tt.wantVerb, verb)
			if tt.wantPayload == "" {
				assert.Nil(t, payload)
			} else {
				assert.Equal(t, tt.wantPayload, string(payload))
			}
		})
	}
}

// TestFormatRequest verifies request line formatting: #<id> <method> [<json>]
//
// VALIDATES: FormatRequest produces correct wire format for requests.
// PREVENTS: Malformed request lines on the wire.
func TestFormatRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		id     uint64
		method string
		params json.RawMessage
		want   string
	}{
		{"with params", 1, "test-method", json.RawMessage(`{"key":"value"}`), `#1 test-method {"key":"value"}`},
		{"no params", 42, "ping", nil, "#42 ping"},
		{"null params", 5, "ping", json.RawMessage("null"), "#5 ping"},
		{"empty params", 3, "method", json.RawMessage(""), "#3 method"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatRequest(tt.id, tt.method, tt.params)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

// TestFormatResult verifies success response formatting: #<id> ok [<json>]
//
// VALIDATES: FormatResult produces correct wire format for success responses.
// PREVENTS: Malformed ok responses on the wire.
func TestFormatResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		id     uint64
		result json.RawMessage
		want   string
	}{
		{"with result", 1, json.RawMessage(`{"data":"value"}`), `#1 ok {"data":"value"}`},
		{"nil result", 2, nil, "#2 ok"},
		{"null result", 3, json.RawMessage("null"), "#3 ok"},
		{"empty result", 4, json.RawMessage(""), "#4 ok"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatResult(tt.id, tt.result)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

// TestFormatOK verifies empty success response formatting: #<id> ok
//
// VALIDATES: FormatOK produces correct wire format.
// PREVENTS: Malformed empty ok responses.
func TestFormatOK(t *testing.T) {
	t.Parallel()

	got := FormatOK(42)
	assert.Equal(t, "#42 ok", string(got))
}

// TestFormatError verifies error response formatting: #<id> error [<json>]
//
// VALIDATES: FormatError produces correct wire format for error responses.
// PREVENTS: Malformed error responses on the wire.
func TestFormatError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      uint64
		payload json.RawMessage
		want    string
	}{
		{"with payload", 1, json.RawMessage(`{"code":"not-found","message":"peer not found"}`), `#1 error {"code":"not-found","message":"peer not found"}`},
		{"empty payload", 2, nil, "#2 error"},
		{"empty bytes", 3, json.RawMessage(""), "#3 error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatError(tt.id, tt.payload)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

// TestNewErrorPayload verifies error payload construction.
//
// VALIDATES: NewErrorPayload produces valid JSON with code and message fields.
// PREVENTS: Unreadable error payloads reaching the CLI display.
func TestNewErrorPayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		code        string
		message     string
		wantCode    string
		wantMessage string
	}{
		{"with code", "peer-not-found", "peer not found", "peer-not-found", "peer not found"},
		{"empty code", "", "some error", "", "some error"},
		{"command error", "command-not-available", `command "bgp rib routes" not available`, "command-not-available", `command "bgp rib routes" not available`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload := NewErrorPayload(tt.code, tt.message)

			var detail struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			require.NoError(t, json.Unmarshal(payload, &detail))
			assert.Equal(t, tt.wantCode, detail.Code)
			assert.Equal(t, tt.wantMessage, detail.Message)
		})
	}
}

// TestRPCCallError verifies RPCCallError.Error() message formatting.
//
// VALIDATES: RPCCallError implements error interface with informative messages.
// PREVENTS: Uninformative error messages when RPC calls fail.
func TestRPCCallError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		err     RPCCallError
		wantMsg string
	}{
		{"message only", RPCCallError{Message: "peer not found"}, "rpc error: peer not found"},
		{"code only", RPCCallError{Code: "not-found"}, "rpc error: not-found"},
		{"both", RPCCallError{Code: "not-found", Message: "peer not found"}, "rpc error: peer not found"},
		{"neither", RPCCallError{}, "rpc error: (no message)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.wantMsg, tt.err.Error())
		})
	}
}

// TestCodedError verifies CodedError carries code through error chains.
//
// VALIDATES: CodedError implements error interface and preserves code.
// PREVENTS: Loss of error codes when errors pass through dispatch layers.
func TestCodedError(t *testing.T) {
	err := NewCodedError("unknown-command", "command not found")
	assert.Equal(t, "unknown-command", err.Code)
	assert.Equal(t, "command not found", err.Error())
}

// TestExtractErrorMessage verifies human-readable message extraction from error payload JSON.
//
// VALIDATES: ExtractErrorMessage returns message when present, empty string otherwise.
// PREVENTS: Consumers falling through to kebab-case code for display.
func TestExtractErrorMessage(t *testing.T) {
	tests := []struct {
		name   string
		params json.RawMessage
		want   string
	}{
		{"with_message", json.RawMessage(`{"message":"peer not found"}`), "peer not found"},
		{"empty_message", json.RawMessage(`{"message":""}`), ""},
		{"no_message_field", json.RawMessage(`{"code":"err"}`), ""},
		{"nil_params", nil, ""},
		{"empty_params", json.RawMessage(``), ""},
		{"invalid_json", json.RawMessage(`{broken`), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractErrorMessage(tt.params)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseRPCError verifies parseRPCError handles empty, valid JSON,
// non-JSON, and partial field payloads.
//
// VALIDATES: parseRPCError correctly populates RPCCallError from various payloads.
// PREVENTS: Lost error details when remote side sends structured or unstructured errors.
func TestParseRPCError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		payload     string
		wantCode    string
		wantMessage string
	}{
		{
			name:        "empty payload",
			payload:     "",
			wantCode:    "",
			wantMessage: "",
		},
		{
			name:        "valid json",
			payload:     `{"code":"x","message":"y"}`,
			wantCode:    "x",
			wantMessage: "y",
		},
		{
			name:        "non-json",
			payload:     "plain text",
			wantCode:    "",
			wantMessage: "plain text",
		},
		{
			name:        "partial fields message only",
			payload:     `{"message":"only msg"}`,
			wantCode:    "",
			wantMessage: "only msg",
		},
		{
			name:        "partial fields code only",
			payload:     `{"code":"only-code"}`,
			wantCode:    "only-code",
			wantMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var input []byte
			if tt.payload != "" {
				input = []byte(tt.payload)
			}
			got := parseRPCError(input)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantCode, got.Code)
			assert.Equal(t, tt.wantMessage, got.Message)
		})
	}
}

// TestParseLineFormatRoundTrip verifies that Format*/ParseLine round-trip correctly.
//
// VALIDATES: Lines formatted with Format* can be parsed back by ParseLine.
// PREVENTS: Formatting/parsing mismatch causing protocol errors.
func TestParseLineFormatRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("request round-trip", func(t *testing.T) {
		t.Parallel()
		params := json.RawMessage(`{"key":"value"}`)
		line := FormatRequest(42, "test-method", params)

		id, verb, payload, err := ParseLine(line)
		require.NoError(t, err)
		assert.Equal(t, uint64(42), id)
		assert.Equal(t, "test-method", verb)
		assert.Equal(t, string(params), string(payload))
	})

	t.Run("ok round-trip", func(t *testing.T) {
		t.Parallel()
		result := json.RawMessage(`{"status":"done"}`)
		line := FormatResult(7, result)

		id, verb, payload, err := ParseLine(line)
		require.NoError(t, err)
		assert.Equal(t, uint64(7), id)
		assert.Equal(t, "ok", verb)
		assert.Equal(t, string(result), string(payload))
	})

	t.Run("error round-trip", func(t *testing.T) {
		t.Parallel()
		errPayload := NewErrorPayload("not-found", "peer not found")
		line := FormatError(3, errPayload)

		id, verb, payload, err := ParseLine(line)
		require.NoError(t, err)
		assert.Equal(t, uint64(3), id)
		assert.Equal(t, "error", verb)
		assert.JSONEq(t, string(errPayload), string(payload))
	})

	t.Run("ok no payload round-trip", func(t *testing.T) {
		t.Parallel()
		line := FormatOK(99)

		id, verb, payload, err := ParseLine(line)
		require.NoError(t, err)
		assert.Equal(t, uint64(99), id)
		assert.Equal(t, "ok", verb)
		assert.Nil(t, payload)
	})
}
