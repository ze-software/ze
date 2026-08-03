// Design: ai/rules/plugins.md -- ze_ssh absent (compile-out) validation
//
//go:build !ze_ssh

package hub

// VALIDATES: without the ze_ssh build tag (e.g. ze-stripped), the ssh seam
// stays nil -- the compile-out proof at the seam layer (the go tool nm symbol
// check is the binary-level proof).
// PREVENTS: a regression where ssh leaks into a hardened build via an always-on
// import or an ungated seam installation.

import "testing"

func TestBuildTag_SSH_Absent(t *testing.T) {
	if sshBuild != nil || sshWirePostStart != nil || sshBuildStandalone != nil {
		t.Fatal("non-ze_ssh build: ssh seam unexpectedly installed (ssh not compiled out)")
	}
}
