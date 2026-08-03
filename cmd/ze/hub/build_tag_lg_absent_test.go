// Design: ai/rules/plugins.md -- ze_lg absent (compile-out) validation
//
//go:build !ze_lg

package hub

// VALIDATES: without the ze_lg build tag (e.g. ze-stripped), the looking-glass
// service factory is NOT registered -- the compile-out proof at the
// registration layer (the go tool nm symbol check is the binary-level proof).
// PREVENTS: a regression where lg leaks into a hardened build via an always-on
// import or an ungated registration.

import "testing"

func TestBuildTag_LG_Absent(t *testing.T) {
	if registeredServiceName("looking-glass") {
		t.Fatal("non-ze_lg build: looking-glass factory unexpectedly registered (not compiled out)")
	}
}
