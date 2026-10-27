// Design: docs/architecture/testing/interop.md -- bgp-panic-cease, the ze_le daemon
// Related: session_panic_inject_le.go -- the ze_le half, which can inject a panic

//go:build !ze_le

package reactor

import "net/netip"

// injectReadPanic injects nothing. Every shipped build lacks the ze_le tag, so
// this empty body is the one it compiles, the call in processMessage inlines
// away, and no file, command or peer can make the read path panic on purpose.
func injectReadPanic(netip.Addr) {}
