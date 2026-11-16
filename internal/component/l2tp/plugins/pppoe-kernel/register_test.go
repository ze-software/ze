// Design: docs/architecture/l2tp/bng-5-pppoe.md -- discoverable kernel provider
// Related: register.go -- composition-root side effect

package pppoekernel

import (
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/kernel"
)

// TestKernelProviderRegisters proves importing the composition wrapper installs
// the exact native provider without a parent transport import.
func TestKernelProviderRegisters(t *testing.T) {
	backend, err := dataplane.LookupSessionDataplane(kernel.Name)
	if err != nil { t.Fatal(err) }
	if backend.Name() != kernel.Name { t.Fatalf("provider=%q", backend.Name()) }
	if opened, err := backend.Open(nil, dataplane.Request{}); err == nil || opened != nil {
		t.Fatalf("registry context validation bypassed: session=%v error=%v", opened, err)
	}
}
