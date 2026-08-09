// Design: docs/architecture/ospf/ospf-ext-14-debug-introspection.md -- the shared debug-injection gate.
// RFC: rfc/short/rfc5250.md (IPv4 opaque inject), rfc/short/rfc5340.md (IPv6 native inject).
//
// The debug LSA-injection surface is DOUBLE-GATED (AC-16/AC-17, R-1): the read-only authz
// profile denies every `debug` command (internal/component/authz/authz.go), AND the engine
// itself refuses to originate a crafted LSA until an operator explicitly enables injection
// with `debug ospf inject enable`. Both gates are required and fail closed. The enablement
// is a single process-global runtime flag shared by BOTH address families (one gate, one
// doctor Warning), NOT config: it is deliberately not persisted, so a reboot returns the
// router to the safe default (injection off).

package ospf

import "sync/atomic"

// debugInjectEnabled is the shared (both address families) debug-injection enablement gate.
// OFF by default; a fresh router cannot inject even for an authorized operator until
// `debug ospf inject enable` turns it on. Second, independent gate behind authz `deny debug`.
var debugInjectEnabled atomic.Bool

// setDebugInjectEnabled sets the shared debug-injection enablement (both address families).
func setDebugInjectEnabled(on bool) { debugInjectEnabled.Store(on) }

// debugInjectIsEnabled reports whether debug LSA injection is currently enabled.
func debugInjectIsEnabled() bool { return debugInjectEnabled.Load() }

// codeOSPFDebugEnabled is the ext-14 doctor code raised (Warning) when debug LSA injection
// is left enabled (AC-25). It is DISTINCT from the two config-sanity codes and registered
// through its own check so those remain untouched.
const codeOSPFDebugEnabled = "doctor-ospf-debug-enabled"
