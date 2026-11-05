// Design: docs/architecture/plugin/feature-gates.md -- unavailable executable bodies must refuse, never panic or omit.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedBodylessInitClosureRefusesUnknownExecution keeps an ordinary
// registered plugin while init invokes an external bodyless function through
// an immediate closure. Its effects are unavailable, so a complete inventory
// must explicitly refuse rather than panic or publish the ordinary subset.
func TestCommittedBodylessInitClosureRefusesUnknownExecution(t *testing.T) {
	root := retainedControlFixture(t)
	write(t, root, "internal/plugins/example/external.go", `package example
func externalInstall()
func init() { func() { externalInstall() }() }
`)
	commitInventoryFixture(t, root)
	plugins, err := Plugins(root)
	if err == nil {
		t.Fatalf("unknown external initialization published incomplete plugins: %+v", plugins)
	}
	if plugins != nil {
		t.Fatalf("unknown external initialization leaked partial plugins: %+v", plugins)
	}
	if !strings.Contains(err.Error(), "externalInstall") || !strings.Contains(err.Error(), "body") {
		t.Fatalf("refusal does not identify unavailable function body: %v", err)
	}
}
