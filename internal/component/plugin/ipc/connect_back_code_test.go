package ipc_test

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	_ "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestConnectBackCodeMatchesDiagnostic verifies that ipc.ConnectBackRefusedCode
// is registered with diagnostic metadata (via plugin/server, because diagnostic
// imports plugin/ipc through internal/component/plugin).
func TestConnectBackCodeMatchesDiagnostic(t *testing.T) {
	if meta := diagnostic.Lookup(ipc.ConnectBackRefusedCode); meta == nil {
		t.Fatalf("ipc raises %q, which has no diagnostic metadata registered", ipc.ConnectBackRefusedCode)
	}
}
