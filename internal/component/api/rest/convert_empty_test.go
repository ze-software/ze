// Design: docs/architecture/api/architecture.md -- lossless REST parameters.
package rest

import (
	"testing"

	"github.com/ze-software/ze/internal/component/api"
)

// TestRESTExecutePreservesExplicitEmptyContent checks JSON empty strings are
// carried to the shared command builder instead of becoming missing arguments.
func TestRESTExecutePreservesExplicitEmptyContent(t *testing.T) {
	request, err := fromRESTExecuteRequest(api.CallerIdentity{}, "show data encode", map[string]any{"content": ""})
	if err != nil {
		t.Fatal(err)
	}
	got := convertedExecuteCommand(t, request)
	if want := `show data encode content ""`; got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}
}
