// Design: docs/architecture/plugin/feature-gates.md -- conservative external type shapes.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalTypeShapeCapabilities requires each newly admitted shape
// to retain installer methods as possible external effects. None of these calls
// uses alias, promoted or generic methods as certain retained dispatch owners.
func TestCommittedExternalTypeShapeCapabilities(t *testing.T) {
	for _, shape := range []struct{ name, declaration, value string }{
		{"alias", "type parcelShape = parcel\n", "[]parcelShape{{}}"},
		{"defined-pointer", "type parcelShape *parcel\n", "[]parcelShape{nil}"},
		{"embedded", "type parcelShape struct { parcel }\n", "[]parcelShape{{}}"},
		{"instantiated", "type parcelShape[T any] struct { Value T }\n", "[]parcelShape[parcel]{{}}"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				declarations := "type parcel struct{}\nfunc (parcel) Fire() { BODY }\n" + shape.declaration
				call := "order.Slice(" + shape.value + ", func(i, j int) bool { return false })"
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(strings.Replace(declarations, "BODY", "", 1), call))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(strings.Replace(declarations, "BODY", "install(0)", 1), call))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "registration")
			})
		}
	}
}
