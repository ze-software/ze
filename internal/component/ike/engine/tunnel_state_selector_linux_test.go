//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- backend declarations survive test wrappers.
// Related: child_forward_policy_linux_test.go -- the refusal wrapper preserves the real backend contract.

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// InboundStateSelectorSupported forwards the wrapped backend's optional contract.
// Embedding Dataplane alone exposes only mandatory interface methods.
func (d *forwardKernelRefusalDP) InboundStateSelectorSupported() bool {
	capable, ok := d.Dataplane.(dataplane.InboundStateSelectorInstaller)
	if !ok {
		return false
	}
	return capable.InboundStateSelectorSupported()
}

// TestTunnelStateSelectorCapabilitySurvivesRefusalWrapper keeps the kernel TDD
// refusal fixture on the same inbound-state contract as its underlying backend.
func TestTunnelStateSelectorCapabilitySurvivesRefusalWrapper(t *testing.T) {
	declared := &forwardKernelRefusalDP{Dataplane: &tunnelBindingDP{}}
	if !declared.InboundStateSelectorSupported() {
		t.Fatal("refusal wrapper hid the real backend's inbound-selector declaration")
	}
	undeclared := &forwardKernelRefusalDP{Dataplane: &mockDP{}}
	if undeclared.InboundStateSelectorSupported() {
		t.Fatal("refusal wrapper invented a capability the underlying backend did not declare")
	}
}
