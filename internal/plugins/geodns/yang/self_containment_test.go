package yang

import (
	"strings"
	"testing"
)

// VALIDATES: the geodns command schema package owns the `show geodns` command
// node (the owner half of the self-containment invariant).
// PREVENTS: the command silently disappearing, which would let the central show
// guard pass vacuously, if the owner schema is removed or renamed.
func TestGeodnsCmdSchemaOwnsShowGeodns(t *testing.T) {
	if !strings.Contains(ZeGeodnsCmdYANG, `ze:command "ze-show:geodns"`) {
		t.Error(`geodns command schema must declare ze:command "ze-show:geodns" (see ai/rules/plugins.md)`)
	}
}
