// VALIDATES: spec-ospfv3-2-wire AC-19, R-1 -- the codec imports only
// internal/plugins/ospfv3/types plus the standard library: never the OSPFv2
// implementation, an OSPFv3 sibling runtime package (transport, lsdb, spf,
// config), a component, or the RIB. A reverse dependency would hide the OSPFv3
// wire divergences this leaf exists to isolate.
// PREVENTS: the codec growing a dependency on runtime code or the OSPFv2 codec.

package packet

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestOSPFv3PacketNoRuntimeImports(t *testing.T) {
	forbidden := []string{
		"internal/plugins/ospf/", // OSPFv2 implementation (must stay separate)
		"internal/component/",    // no component dependency
		"internal/core/rib",      // no RIB dependency
	}
	// Sibling OSPFv3 runtime packages must not be imported; the only allowed
	// ospfv3 import is the types leaf.
	allowedOSPFv3 := "internal/plugins/ospfv3/types"

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbidden {
				if strings.Contains(p, bad) {
					t.Errorf("%s imports forbidden %q (leaf codec must not import runtime or OSPFv2)", name, p)
				}
			}
			if strings.Contains(p, "internal/plugins/ospfv3/") && !strings.HasSuffix(p, allowedOSPFv3) {
				t.Errorf("%s imports OSPFv3 sibling %q (only the types leaf is allowed)", name, p)
			}
		}
	}
}
