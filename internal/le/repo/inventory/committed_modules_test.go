// Design: docs/architecture/plugin/feature-gates.md -- reachable standalone schema registrations share the snapshot.
package repoinventory

import (
	"context"
	"strings"
	"testing"
)

// TestCollectIncludesReachableStandaloneModuleRegistration proves config/yang
// registration is independent of plugin.YANG, pinned across dirty bytes and
// HEAD movement, and excludes an uncalled registration literal.
func TestCollectIncludesReachableStandaloneModuleRegistration(t *testing.T) {
	root := committedInventoryFixture(t)
	write(t, root, "internal/component/config/yang/register.go", "package yang\nfunc RegisterModule(name, content string) {}\n")
	write(t, root, "internal/component/plugin/registry/external_witness.go", "package registry\nimport _ \"embed\"\n")
	write(t, root, "internal/component/config/yang/external_witness.go", "package yang\nimport _ \"embed\"\n")
	write(t, root, "internal/extra/register.go", `package extra
import (
 _ "embed"
 schema "github.com/ze-software/ze/internal/component/config/yang"
)
//go:embed standalone.yang
var document string
const moduleName = "standalone.yang"
func init() { registerSchema() }
func registerSchema() { schema.RegisterModule(moduleName, document) }
func neverCalled() { schema.RegisterModule("unreachable.yang", "module unreachable { namespace urn:no; prefix no; }") }
`)
	write(t, root, "internal/extra/standalone.yang", "module standalone { namespace urn:standalone; prefix s; rpc standalone-show { } }\n")
	write(t, root, "cmd/ze/main.go", "package main\nimport (\n _ \"github.com/ze-software/ze/internal/thing\"\n _ \"github.com/ze-software/ze/internal/extra\"\n)\nfunc main() {}\n")
	commitInventoryFixture(t, root)
	snapshot, err := ReadCommitted(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "internal/extra/register.go", "package extra\n")
	write(t, root, "internal/extra/standalone.yang", "module dirty { }\n")
	commitInventoryFixture(t, root)
	inv, err := snapshot.collect()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, module := range inv.YANGModules {
		if module.Name == "standalone.yang" && module.Source == "infrastructure" {
			found = true
		}
		if strings.Contains(module.Name, "unreachable") || strings.Contains(module.Name, "dirty") {
			t.Errorf("unreachable or moving source entered module population: %+v", module)
		}
	}
	if !found {
		t.Fatalf("reachable standalone module missing: %+v", inv.YANGModules)
	}
	if inv.RPCsByModule["standalone.yang"] != 1 || inv.TotalRPCs != 4 {
		t.Fatalf("RPC report mixed snapshots or omitted same-line statement: %+v", inv.RPCsByModule)
	}
}
