// Design: docs/architecture/plugin/feature-gates.md -- one row per registered schema identity.
package repoinventory

import "testing"

// TestCollectDoesNotDuplicateAlreadyRegisteredPluginSchema uses both plugin.YANG
// and an init-reachable standalone registration of that schema. The report must
// retain the explicit registration filename rather than synthesizing a second row.
func TestCollectDoesNotDuplicateAlreadyRegisteredPluginSchema(t *testing.T) {
	root := committedInventoryFixture(t)
	write(t, root, "internal/component/config/yang/register.go", "package yang\nfunc RegisterModule(name, content string) {}\n")
	write(t, root, "internal/thing/register.go", `package thing
import schema "github.com/ze-software/ze/internal/component/config/yang"
func init() { schema.RegisterModule("thing.yang", "module thing { namespace urn:thing; prefix t; }") }
`)
	commitInventoryFixture(t, root)
	inv, err := Collect(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.YANGModules) != 1 || inv.YANGModules[0].Name != "thing.yang" {
		t.Fatalf("schema registration duplicated or renamed: %+v", inv.YANGModules)
	}
}
