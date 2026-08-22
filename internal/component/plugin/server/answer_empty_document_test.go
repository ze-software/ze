package server

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// VALIDATES: an answer carrying no document leaves Data absent, so ResponseJSON
// reports nothing rather than the four bytes `null`.
// PREVENTS: an operator reading `null` where they read nothing, which the owner
// ruled against directly (returning nil is fine, printing it is not).
func TestEmptyDocumentIsAbsentNotNull(t *testing.T) {
	absent := &plugin.Response{Status: plugin.StatusDone}
	got, err := plugin.ResponseJSON(absent, nil)
	if err != nil {
		t.Fatalf("absent data: %v", err)
	}
	if got != "" {
		t.Fatalf("an answer with no document rendered %q, want nothing", got)
	}

	empty := &plugin.Response{Status: plugin.StatusDone, Data: plugin.RawJSON("")}
	wrong, err := plugin.ResponseJSON(empty, nil)
	if err != nil {
		t.Fatalf("empty RawJSON: %v", err)
	}
	if wrong != "null" {
		t.Fatalf("RawJSON(\"\") rendered %q; the guard in pluginCommandResponse exists because this is %q", wrong, "null")
	}
}
