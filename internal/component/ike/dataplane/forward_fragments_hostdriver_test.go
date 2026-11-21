// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- transit traffic, fragment gathering
// Related: forward_fragments.go -- RuntimeKernelBuildFix, the copy this test compares

// The test lives in the external package because hostdriver's own imports reach
// component packages, and only an external test package can import them without a
// cycle back to dataplane.
package dataplane_test

import (
	"runtime"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	buildhostdriver "github.com/ze-software/ze/internal/le/build/hostdriver"
)

// TestRuntimeKernelBuildFixMatchesHostDriver compares the dataplane's copy of the
// runtime kernel build commands with their declaration.
//
// VALIDATES: RuntimeKernelBuildFix answers exactly what RuntimeKernelBuild in
// internal/le/build/hostdriver answers for this host's architecture with no builder.
// Shipped dataplane code cannot import le's build tooling, so the copy is
// unavoidable, and this test is the check that compares it with its source.
// PREVENTS: an operator error that sends the operator to a build command the
// tooling no longer accepts, after either side changes alone.
func TestRuntimeKernelBuildFixMatchesHostDriver(t *testing.T) {
	want := buildhostdriver.RuntimeKernelBuild(runtime.GOARCH, "")
	if got := dataplane.RuntimeKernelBuildFix(); got != want {
		t.Fatalf("RuntimeKernelBuildFix() = %q, want hostdriver's %q", got, want)
	}
}
