package ipc_test

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestConnectBackCodeMatchesDiagnostic holds the one copy ipc keeps of a
// diagnostic code to its declaration: ipc cannot import diagnostic (diagnostic
// imports ipc), so the code a refused connect-back raises is spelled here and
// compared with diagnostic.CodeDoctorPluginConnectBackRefused, which describes
// it to `ze show diagnostic` and registers it as an event code.
func TestConnectBackCodeMatchesDiagnostic(t *testing.T) {
	if got, want := ipc.ConnectBackRefusedCode, diagnostic.CodeDoctorPluginConnectBackRefused; got != want {
		t.Fatalf("ipc raises %q, diagnostic declares %q", got, want)
	}
}
