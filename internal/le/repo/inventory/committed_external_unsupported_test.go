// Design: docs/architecture/plugin/feature-gates.md -- explicit external authority refusals.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalNamedReturnRefuses prevents a naked return from silently
// losing its named callback slot. This bounded model does not certify the shape,
// even when the assigned callback is inert.
func TestCommittedExternalNamedReturnRefuses(t *testing.T) {
	for _, body := range []string{"", "install(0)"} {
		t.Run("callback-"+body, func(t *testing.T) {
			fixture := externalEffectsFixture(t, "plugin")
			call := "_ = clock.AfterFunc(0, callback())"
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource("func callback() func() { return func() {} }\n", call))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
			declaration := "func callback() (result func()) { result = func() { " + body + " }; return }\n"
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declaration, call))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "named")
		})
	}
}

// TestCommittedExternalRangeTransferRefuses prevents the parser's synthetic
// range binding from becoming an empty capability set. Until element ownership
// is modeled, both inert and installer-bearing transfers explicitly refuse.
func TestCommittedExternalRangeTransferRefuses(t *testing.T) {
	for _, body := range []string{"", "install(0)"} {
		t.Run("callback-"+body, func(t *testing.T) {
			fixture := externalEffectsFixture(t, "plugin")
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "_ = clock.AfterFunc(0, func() {})"))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
			call := "for _, callback := range []func(){func() { " + body + " }} { _ = clock.AfterFunc(0, callback) }"
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", call))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "range")
		})
	}
}

// TestCommittedExternalLinknameRefuses gives an apparently uninitialized local
// function variable the identity of a private installer-bearing variable in a
// retained package. Import-cycle witnesses cannot establish this symbol's value.
func TestCommittedExternalLinknameRefuses(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			fixture.goFile(t, callableOwnerFile, fixture.goSources[callableOwnerFile]+"\nvar linkedCallback = func() { Install(0) }\n")
			call := "_ = clock.AfterFunc(0, callback)"
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource("var callback = func() {}\n", call))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
			declaration := "//go:linkname callback github.com/ze-software/ze/" + callableOwnerDir + ".linkedCallback\nvar callback func()\n"
			source := externalEffectsSource(declaration, call)
			source = strings.Replace(source, "import (", "import (_ \"unsafe\";", 1)
			fixture.goFile(t, callableOwnerCaller, source)
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "linkname")
		})
	}
}
