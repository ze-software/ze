package rpc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewError verifies NewError produces RPCError with explicit code and human-readable detail.
//
// VALIDATES: Error field uses the explicit code, Params.message preserves original text.
// PREVENTS: Unreadable error messages reaching the CLI display.
func TestNewError(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		message     string
		wantMessage string
	}{
		{"simple", "peer-not-found", "peer not found", "peer not found"},
		{"command error", "command-not-available", `command "bgp rib routes" not available`, `command "bgp rib routes" not available`},
		{"single word", "unauthorized", "unauthorized", "unauthorized"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := json.RawMessage(`42`)
			err := NewError(id, tt.code, tt.message)

			assert.Equal(t, tt.code, err.Error)
			assert.Equal(t, id, err.ID)

			var detail struct {
				Message string `json:"message"`
			}
			require.NoError(t, json.Unmarshal(err.Params, &detail))
			assert.Equal(t, tt.wantMessage, detail.Message)
		})
	}
}

// TestNewErrorNilID verifies NewError handles nil ID gracefully.
//
// VALIDATES: nil ID produces valid RPCError with null/empty id.
// PREVENTS: Nil pointer dereference in error construction.
func TestNewErrorNilID(t *testing.T) {
	err := NewError(nil, "some-error", "some error")
	assert.Equal(t, "some-error", err.Error)
	assert.Nil(t, err.ID)
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

// TestExtractMessage verifies human-readable message extraction from RPCError params.
//
// VALIDATES: ExtractMessage returns Params.message when present, empty string otherwise.
// PREVENTS: Consumers falling through to kebab-case Error code for display.
func TestExtractMessage(t *testing.T) {
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
			got := ExtractMessage(tt.params)
			assert.Equal(t, tt.want, got)
		})
	}
}
