// VALIDATES: spec-ospfv3-2-wire AC-19, R-1 -- the codec imports only
// internal/plugins/ospf/v3/types plus the standard library: never the OSPFv2
// engine, an OSPFv3 sibling runtime package (transport, lsdb, spf, config), a
// component, or the RIB. A reverse dependency would hide the OSPFv3 wire
// divergences this leaf exists to isolate.
// PREVENTS: the codec growing a dependency on runtime code or the OSPFv2 codec.
//
// The v6 leaves were nested under internal/plugins/ospf/v3/ so the single OSPF plugin
// is self-contained (docs/architecture/ospf/ospf-af-unify.md). Because the codec now lives
// inside the OSPF tree, "no OSPF-tree import" is expressed as a single allow-list check:
// the only permitted OSPF-tree import is the v6 types leaf.

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
		"internal/component/", // no component dependency
		"internal/core/rib",   // no RIB dependency
	}
	// Within the OSPF tree the codec may import ONLY the v6 types leaf; anything
	// else (the OSPFv2 engine, or a v6 sibling runtime package such as transport)
	// is a forbidden reverse dependency.
	allowedOSPFTree := "internal/plugins/ospf/v3/types"

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
					t.Errorf("%s imports forbidden %q (leaf codec must not import a component or the RIB)", name, p)
				}
			}
			if strings.Contains(p, "internal/plugins/ospf/") && !strings.HasSuffix(p, allowedOSPFTree) {
				t.Errorf("%s imports OSPF runtime %q (the v6 codec may import only the v6 types leaf)", name, p)
			}
		}
	}
}
