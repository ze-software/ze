package server

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// internalBuildTestWireToPath maps each wire method to its YANG path for the
// benchmarks. It fails the benchmark when the schema does not resolve: a nil
// loader maps no RPC, so the benchmark would measure a dispatcher with no
// handlers instead of naming the module this binary does not link.
func internalBuildTestWireToPath(tb testing.TB) map[string]string {
	tb.Helper()
	loader, err := yang.DefaultLoader()
	if err != nil {
		tb.Fatalf("resolve the YANG schema this binary links: %v", err)
	}
	return yang.WireMethodToPath(loader)
}
