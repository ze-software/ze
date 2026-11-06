// Design: docs/architecture/plugin/feature-gates.md -- field metadata retains enclosing reads.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalFieldMetadataRetainsEnclosingRead distinguishes a scalar
// field copied into an external argument from the writable enclosing object.
// The latter can change the registered description even though the field's
// initial literal carries no independent reference capability.
func TestCommittedExternalFieldMetadataRetainsEnclosingRead(t *testing.T) {
	fixture := externalEffectsFixture(t, "plugin")
	name := asmOrdinaryDir + "/register.go"
	source := strings.Replace(fixture.goSources[name], "func init() {", "var metadata = struct { Label string }{Label: \"ordinary committed control\"}\nfunc init() {", 1)
	source = strings.Replace(source, "Description: \"ordinary committed control\"", "Description: metadata.Label", 1)
	fixture.goFile(t, name, source)
	// A one-element slice is a valid sort input. The control gives opaque
	// code only a value copy; its scalar field cannot mutate metadata.
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "order.Slice([]any{metadata}, func(i, j int) bool { return false })"))
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)

	// Only the argument's ownership changes. The same opaque operation now
	// receives the registered field's writable enclosing object.
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "order.Slice([]any{&metadata}, func(i, j int) bool { return false })"))
	commitInventoryFixture(t, fixture.root)
	externalEffectsRefusal(t, fixture, "metadata")
}
