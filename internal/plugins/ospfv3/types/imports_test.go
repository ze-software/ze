// VALIDATES: spec-ospfv3-1-types R-1 -- the leaf type package imports only the standard
// library: never the OSPFv2 implementation, any OSPFv3 sibling runtime package, a
// component, or the RIB. Accidentally sharing OSPFv2 identifiers would hide the OSPFv3
// wire differences.
// PREVENTS: the leaf package growing a reverse dependency on runtime code.
package types

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestOSPFv3TypesNoRuntimeImports(t *testing.T) {
	forbidden := []string{
		"internal/plugins/ospf/",   // OSPFv2 implementation (must stay separate)
		"internal/plugins/ospfv3/", // no OSPFv3 sibling runtime package (this is a leaf)
		"internal/component/",      // no component dependency
		"internal/core/rib",        // no RIB dependency
	}
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
					t.Errorf("%s imports forbidden %q (leaf package must not import runtime or OSPFv2)", name, p)
				}
			}
		}
	}
}
