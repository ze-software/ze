// Design: docs/architecture/core-design.md -- administrative distance seam
// Related: internal/component/sysrib/sysrib.go -- parseAdminDistanceConfig, the producer
// Related: internal/core/rib/locrib/entry.go -- selectBest, which ranks the stamped result

// Package distance carries every protocol's administrative distance from the
// one place it is declared to the producers that stamp it on a route.
//
// The declaration is `rib { distance { } }`
// (internal/component/sysrib/yang/ze-rib-conf.yang) and sysrib resolves it,
// schema defaults included, in parseAdminDistanceConfig. The consumers are the
// protocols that insert into the shared Loc-RIB: the BGP RIB plugin, IS-IS SPF
// and OSPF SPF.
//
// WHY A SEAM RATHER THAN A DIRECT CALL. locrib.selectBest ranks paths on the
// value the producer stamped, and it runs BEFORE sysrib sees the route: sysrib
// consumes one already-arbitrated best per prefix. So a distance the producer
// did not know about cannot change cross-protocol selection, however carefully
// sysrib resolves it afterwards. The value has to reach the producer, and the
// producers sit in internal/plugins and internal/component while the config
// lives above them. A leaf package neither side owns is the seam that lets the
// declaration reach them without inverting the dependency, which is the shape
// igpcost (internal/core/rib/igpcost) already uses for the IGP next-hop cost.
//
// AN UNSET SEAM DOES NOT ANSWER ZERO. igpcost can report 0 for an unset seam
// because 0 means "no interior cost known" and makes its tiebreak a no-op. A
// distance of 0 is the opposite: it is the BEST possible distance, the one
// `connected` holds, so a route stamped 0 by accident beats every other
// protocol for that prefix. Of therefore reports whether the declaration
// answered, and a caller with no answer uses its own bootstrap default rather
// than a zero nobody chose (ai/rules/principles.md).
package distance

import "sync/atomic"

// Func returns the declared administrative distance for a protocol name, and
// whether the declaration names that protocol. The names are the ones the
// config leaves use: "connected", "static", "ebgp", "ospf", "isis", "ibgp".
type Func func(protocol string) (uint8, bool)

// fnPtr holds the registered lookup. Read on the route-install path and written
// once per configure, so an atomic pointer beats a mutex here.
var fnPtr atomic.Pointer[Func]

// Set registers the distance lookup. sysrib calls it once its configuration is
// parsed, and again on every reload. A nil fn clears the seam, after which Of
// reports that nothing answered.
func Set(fn Func) {
	if fn == nil {
		fnPtr.Store(nil)
		return
	}
	fnPtr.Store(&fn)
}

// Of returns the declared distance for protocol and true, or false when no
// resolver is registered yet or the declaration does not name that protocol.
//
// A false is NOT distance zero. The caller supplies its own bootstrap default,
// which is reachable only before the first configure has run.
func Of(protocol string) (uint8, bool) {
	p := fnPtr.Load()
	if p == nil {
		return 0, false
	}
	return (*p)(protocol)
}

// OrDefault returns the declared distance for protocol, or fallback when the
// declaration has not answered. It is the form every producer wants: stamp the
// operator's value once configuration exists, and the protocol's own classical
// default before that.
func OrDefault(protocol string, fallback uint8) uint8 {
	if d, ok := Of(protocol); ok {
		return d
	}
	return fallback
}
