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
	if want := `show data encode content ""`; request.Command != want {
		t.Fatalf("command = %q, want %q", request.Command, want)
	}
}
