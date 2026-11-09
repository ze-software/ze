// Design: docs/architecture/plugin/feature-gates.md -- a range header stores its collection's iteration values.
package repoinventory

import "testing"

// TestCommittedExternalRangeAssignmentStoresElements requires a range header
// that assigns to existing variables to store the collection's contents, as
// gokrazy's packer ranges extraFilesArchiveCandidates into effectivePath.
// Method: candidates returns a []string or a []func(); the init assigns each
// element to a declared variable with = and hands it to sort.Slice. The
// strings are inert and publish, an inert callback publishes, and a callback
// that installs refuses as registration, so the stored value is the
// elements and not an empty transfer.
//
// MUTATION: (internal/le/repo/inventory/externalexpressions.go,
// `			return e.rangeAssignedValue(source, value, possible, depth)`,
// `			return externalValue{unknown: "range assignment capability ownership is unsupported"}`):
// every shape refuses as registration capability is unknown, the actual-root
// refusal at gokrazy packer.go:210.
// MUTATION: (internal/le/repo/inventory/externalrangestores.go,
// `	result := externalFlat(e.value(collection, possible, depth+1), 0)`,
// `	result := externalValue{}`):
// the stored callback is lost, and the installing shape publishes.
func TestCommittedExternalRangeAssignmentStoresElements(t *testing.T) {
	for _, shape := range []struct{ name, declarations, call, reason string }{
		{"strings", "func candidates() []string { return []string{\"a\"} }\n",
			"var path string; for _, path = range candidates() { " + callResultSliced("path") + " }", ""},
		{"inert callback", "func candidates() []func() { return []func(){func() {}} }\n",
			"var hook func(); for _, hook = range candidates() { " + callResultSliced("hook") + " }", ""},
		{"installing callback", "func candidates() []func() { return []func(){func() { install(0) }} }\n",
			"var hook func(); for _, hook = range candidates() { " + callResultSliced("hook") + " }", "registration"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(shape.declarations, shape.call))
				commitInventoryFixture(t, fixture.root)
				if shape.reason == "" {
					externalEffectsPositive(t, fixture)
					return
				}
				externalEffectsRefusal(t, fixture, shape.reason)
			})
		}
	}
}

// TestCommittedExternalRangeAssignmentRefusesIterators keeps refusing a range
// header that assigns from a function iterator or a channel, whose iteration
// values are what the iterator yields or what is sent, not the collection.
//
// MUTATION: (internal/le/repo/inventory/externalrangestores.go,
// `	if reason := externalRangeCollectionShape(datatype); reason != "" {`,
// `	if reason := externalRangeCollectionShape(datatype); reason == "-" {`):
// the channel shape stores the collection value and publishes, and the
// iterator shape loses its cause, refusing only on the iterator's own yield
// formal.
func TestCommittedExternalRangeAssignmentRefusesIterators(t *testing.T) {
	for _, shape := range []struct{ name, declarations, call, reason string }{
		{"function iterator", "func each(yield func(func()) bool) { yield(func() {}) }\n",
			"var hook func(); for hook = range each { " + callResultSliced("hook") + " }", "range callback capability transfer is unsupported"},
		{"channel", "func feed() chan func() { return make(chan func()) }\n",
			"var hook func(); for hook = range feed() { " + callResultSliced("hook") + " }", "range channel capability transfer is unsupported"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(shape.declarations, shape.call))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, shape.reason)
			})
		}
	}
}
