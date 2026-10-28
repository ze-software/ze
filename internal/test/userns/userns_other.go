// Design: docs/architecture/testing/qemu-integration.md -- Build Tags, a namespace of the test's own
// Related: userns_linux.go -- Enter on Linux, which re-runs the test in its own namespaces

//go:build !linux

package userns

import "testing"

// Enter answers true at once off Linux, where no user namespace exists: the
// test body runs in place, in the calling process. A test that needs a Linux
// namespace already carries a linux build tag, so the callers that reach this
// version are portable tests (the traceroute command tests) that call Enter to
// get their own namespace when they run on Linux.
func Enter(t *testing.T) bool {
	t.Helper()
	return true
}
