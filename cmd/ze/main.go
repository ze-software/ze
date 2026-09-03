// Design: docs/architecture/system-architecture.md -- ze unified entry point
// RFC: rfc/short/rfc5216.md -- EAP-TLS MSK export, which the note below is about
// RFC: rfc/short/rfc9190.md -- EAP-TLS 1.3, the path that needs no opt-in
// RFC: rfc/short/rfc7627.md -- extended master secret, why the export is refused
//
// Single main() for all ze binaries. Build tags control which commands
// register: ze_core (ze, ze-appliance), ze_setup, ze_test, ze_chaos,
// ze_perf, ze_analyze. The binary name does not affect dispatch.
//
// EAP-TLS against a TLS 1.2 peer with no Extended Master Secret: ze is
// FAIL-CLOSED, and the opt-in no longer exists.
//
// RFC 5216 Section 2.3 DEFINES the EAP-TLS MSK as a crypto/tls
// ExportKeyingMaterial result. Go refuses that export on a TLS 1.2 session that
// did not negotiate RFC 7627, so ze cannot authenticate such a peer.
// strongSwan 5.9.14 lands there by DEFAULT, not by limitation: charon ships
// `version_max = 1.2` in /etc/strongswan.d/charon.conf, and its own comment says
// "default to TLS 1.2 until 1.3 is stable for use in EAP". Setting
// `charon.tls.version_max = 1.3` on that same 5.9.14 image reaches an
// established SA, which test/interop-ipsec/scenarios/eap-tls13 proves.
//
// A `go:debug tlsunsafeekm=1` line HERE would once have lifted the refusal, and
// it was written and then removed (Thomas, 2026-08-01). It sets the default for
// every ze binary, so it would weaken the export rule for every user to suit one
// peer version. The GODEBUG name says "unsafe" for a reason: RFC 7627
// Section 6.1 states that an attacker who forces the same master secret on two
// sessions compromises every property that relies on its uniqueness, and names
// the TLS exporter as one of them: it "no longer provides a unique key bound to
// the current session". That exporter is the EAP-TLS MSK.
//
// Go 1.27 then REMOVED the setting (internal/godebugs, table.go, `Removed: 27`),
// which took the environment variable with it. The runtime raises a fatal error
// before main() when a removed key carries its old value, so an operator who now
// sets `tlsunsafeekm` back to its old value stops the daemon rather than reaching
// the peer. Ze therefore tells nobody to set it, here or anywhere else, and it
// does not write the assignment out even to warn about it: a reader can paste a
// string that is on the page (cmd/ze/godebug_guidance_test.go).
//
// An operator who meets such a peer has three answers, and ze names all three in
// the error it logs (internal/core/eap, eapTLS12ExportRefused): move the
// peer to TLS 1.3, add RFC 7627 to its TLS 1.2 stack, or configure another EAP
// method.
//
// TLS 1.3 needs none of this: the export is always available and RFC 9190
// supersedes RFC 5216. Ze implements and prefers that path
// (internal/core/eap, exportEAPTLSMSK selects by negotiated version).

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
