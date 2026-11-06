// Design: docs/architecture/plugin/feature-gates.md -- range kind comes from retained declarations.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalRangeKindExecution pairs retained non-invoking kinds
// with function iterator operands reached through the same source resolver.
func TestCommittedExternalRangeKindExecution(t *testing.T) {
	const each = "func each(yield func(uint32) bool) { helper.Install(0); _ = yield(0) }\n"
	for _, kind := range []string{"plugin", "yang"} {
		for _, scenario := range []struct {
			name, declarations, setup, ordinary, iterator string
		}{
			{"function variable", "", "loop := each; _ = loop; ", "[]uint32{0}", "loop"},
			{"typed function alias", "type Iterator = func(func(uint32) bool)\n", "var loop Iterator = each; _ = loop; ", "[]uint32{0}", "loop"},
			{"function result", "func values() []uint32 { return []uint32{0} }\nfunc iterator() func(func(uint32) bool) { return each }\n", "", "values()", "iterator()"},
			{"named container", "type Numbers []uint32\ntype Alias = Numbers\n", "", "Alias{0}", "each"},
			{"integer", "", "", "2", "each"},
			{"string", "", "", "\"ab\"", "each"},
			{"pointer array", "", "", "&[1]uint32{0}", "each"},
			{"make slice", "", "", "make([]uint32, 1)", "each"},
			{"new array", "", "", "new([1]uint32)", "each"},
			{"retained field", "type Holder struct { Values []uint32 }\nvar holder Holder\n", "", "holder.Values", "each"},
		} {
			t.Run(kind+"/"+scenario.name, func(t *testing.T) {
				fixture := externalPublicEntryFixture(t, kind)
				body := each + scenario.declarations + "func Fire() { " + scenario.setup + "for item := range OPERAND { _ = item } }"
				ordinary := strings.Replace(body, "OPERAND", scenario.ordinary, 1)
				fixture.goFile(t, externalPublicEntryDir+"/entry.go", externalPublicHelperSource("publicentry", ordinary))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)

				changed := strings.Replace(body, "OPERAND", scenario.iterator, 1)
				fixture.goFile(t, externalPublicEntryDir+"/entry.go", externalPublicHelperSource("publicentry", changed))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "range")
			})
		}
	}
}
