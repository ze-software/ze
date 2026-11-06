// Design: docs/architecture/plugin/feature-gates.md -- exposed semantic sink capabilities refuse.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalDirectSinkCapability treats the qualified registration
// API as the same semantic operation whether retained syntax calls it or opaque
// code receives it as a callable value. Empty fixture bodies do not remove that
// operation. This tests arbitrary reachable capability use, not sort's behavior.
func TestCommittedExternalDirectSinkCapability(t *testing.T) {
	for _, sink := range []struct {
		kind string
		pkg  string
		path string
		name string
	}{
		{kind: "plugin", pkg: "registry", path: registrationPackage, name: "Register"},
		{kind: "yang", pkg: "yang", path: "internal/component/config/yang", name: "RegisterModule"},
	} {
		t.Run(sink.kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, sink.kind)
			fixture.goFile(t, sink.path+"/external_method_control.go", "package "+sink.pkg+"\ntype SafeReceiver struct{}\nfunc (SafeReceiver) "+sink.name+"() {}\n")
			call := "_ = api." + sink.name + "\nsafe := api.SafeReceiver{}\n_ = safe\norder.Slice([]any{TRANSFERRED}, func(i, j int) bool { return false })"
			source := externalEffectsSource("", call)
			source = strings.Replace(source, "import (", "import (\n api \"github.com/ze-software/ze/"+sink.path+"\"", 1)
			fixture.goFile(t, callableOwnerCaller, strings.Replace(source, "TRANSFERRED", "func() {}", 1))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)

			// Same package and spelling do not make a concrete receiver method
			// the package-level semantic sink. Its retained body is inert.
			fixture.goFile(t, callableOwnerCaller, strings.Replace(source, "TRANSFERRED", "safe."+sink.name, 1))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)

			fixture.goFile(t, callableOwnerCaller, strings.Replace(source, "TRANSFERRED", "api."+sink.name, 1))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "registration")
		})
	}
}
