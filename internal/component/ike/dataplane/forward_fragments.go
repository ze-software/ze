// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- transit traffic, fragment gathering
// Related: xfrm_state_scope_linux.go -- the XFRM answer, read from the kernel's capability receipt
// Related: dataplane.go -- the other optional capability declarations

package dataplane

import "runtime"

// ForwardFragmentGatherer is implemented by a backend that states whether its forward
// policy check sees whole datagrams: whether every fragment of a packet it forwards is
// reassembled before that check reads the packet's ports.
//
// RFC 4301 Section 7.4: "All implementations MUST support stateful fragment checking to
// accommodate BYPASS traffic for which a non-trivial port range is specified." The
// engine projects a port-scoped operator BYPASS onto the forward lookup only on a
// backend that answers true, and refuses such an entry at commit on any other backend
// with a forward lookup (engine/spd_forward_bypass.go, spdForwardBypassRefusals).
//
// A backend that does not implement the interface has declared nothing, and the engine
// treats that as not gathering. A backend that cannot read the answer MUST return the
// error, never false: false is the answer "this backend does not gather". The answer
// MUST NOT change while the backend is loaded, because the engine derives an entry's
// install, its replacement and its removal from it.
type ForwardFragmentGatherer interface {
	GathersForwardFragments() (bool, error)
}

// RuntimeKernelBuildFix is the repair that builds Ze's native runtime kernel, the one
// that carries Ze's XFRM patch series, for this host's architecture, written as the
// commands an operator error's fix carries.
//
// It keeps the exact native command contract of RuntimeKernelBuild in
// internal/le/build/hostdriver/hostdriver.go. Shipped dataplane code MUST NOT import
// le's build tooling to format the repair, so this is a copy, and
// TestRuntimeKernelBuildFixMatchesHostDriver compares it with that source.
func RuntimeKernelBuildFix() string {
	return "  ./le build host-driver\n  ./ze-host appliance kernel --target runtime --arch " + runtime.GOARCH
}
