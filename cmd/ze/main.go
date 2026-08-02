// Design: docs/architecture/system-architecture.md -- ze unified entry point
//
// Single main() for all ze binaries. Build tags control which commands
// register: ze_core (ze, ze-appliance), ze_setup, ze_test, ze_chaos,
// ze_perf, ze_analyze. The binary name does not affect dispatch.
//
// EAP-TLS against a TLS 1.2 peer with no Extended Master Secret: ze ships
// FAIL-CLOSED, and the operator opts in.
//
// RFC 5216 Section 2.3 DEFINES the EAP-TLS MSK as a crypto/tls
// ExportKeyingMaterial result. Go refuses that export on a TLS 1.2 session that
// did not negotiate RFC 7627, so ze cannot authenticate such a peer.
// strongSwan 5.9.14 is one: it caps at TLS 1.2 and does not negotiate 7627.
//
// A `//go:debug tlsunsafeekm=1` line HERE would fix that, and it was written and
// then removed (Thomas, 2026-08-01). It sets the default for every ze binary, so
// it would weaken the export rule for every user to suit one peer version. The
// GODEBUG name says "unsafe" for a reason: RFC 7627 exists to stop the triple
// handshake attack, and without it exported keying material can collide across
// sessions.
//
// An operator who must talk to such a peer sets `GODEBUG=tlsunsafeekm=1` in the
// environment, which is a decision they take knowingly and can audit. The IPsec
// interop lab sets it for scenario 04 for exactly that reason.
//
// TLS 1.3 needs none of this: the export is always available and RFC 9190
// supersedes RFC 5216. Ze implements and prefers that path
// (internal/component/ike/eap, exportEAPTLSMSK selects by negotiated version).

package main

import "os"

var (
	version   = "dev"
	buildDate = "unknown"
)

func main() {
	code := dispatchMain(os.Args[1:])
	flushCrashlog()
	os.Exit(code)
}
