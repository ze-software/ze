// Design: docs/architecture/core-design.md -- administrative distance seam
// Related: internal/component/sysrib/sysrib.go -- parseAdminDistanceConfig, the producer
// Related: internal/core/rib/locrib/distance.go -- resolvedDistance, the one consumer

// Package distance carries every protocol's administrative distance from the
// one place it is declared to the Loc-RIB, which ranks on it.
//
// The declaration is `rib { distance { } }`
// (internal/component/sysrib/yang/ze-rib-conf.yang) and sysrib resolves it,
// schema defaults included, in parseAdminDistanceConfig. The consumer is the
// Loc-RIB (internal/core/rib/locrib): it looks the distance up by protocol each
// time it ranks a path, and sysrib asks it to re-run selection whenever the
// declaration changes. Protocols hand their routes over without a distance, so
// a reload that changes one re-ranks the routes already installed.
//
// WHY A SEAM RATHER THAN A DIRECT CALL. locrib sits in internal/core while the
// config lives in a component above it. A leaf package neither side owns lets
// the declaration reach the Loc-RIB without inverting the dependency, which is
// the shape igpcost (internal/core/rib/igpcost) already uses for the IGP
// next-hop cost.
//
// AN UNSET SEAM DOES NOT ANSWER ZERO. igpcost can report 0 for an unset seam
// because 0 means "no interior cost known" and makes its tiebreak a no-op. A
// distance of 0 is the opposite: it is the BEST possible distance, the one
// `connected` holds, so a route ranked 0 by accident beats every other protocol
// for that prefix. Of therefore reports whether the declaration answered, and
// Resolve falls back to the bootstrap table below rather than to a zero nobody
// chose.
package distance

import (
	"maps"
	"sync/atomic"
)

// Func returns the declared administrative distance for a protocol name, and
// whether the declaration names that protocol. The names are the ones the
// config leaves use: "connected", "static", "ebgp", "ospf", "isis", "ibgp".
type Func func(protocol string) (uint8, bool)

// fnPtr holds the registered lookup. Read on every ranking and written once per
// configure, so an atomic pointer beats a mutex here.
var fnPtr atomic.Pointer[Func]

// Set registers the distance lookup. sysrib calls it at plugin start with the
// schema defaults, and again on every configure and rollback, each time before
// it asks the Loc-RIB to re-rank (locrib.RIB.Reselect). A nil fn clears the
// seam, after which Of reports that nothing answered.
//
// The seam is process-global, and so is the Loc-RIB that reads it.
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
// A false is NOT distance zero.
func Of(protocol string) (uint8, bool) {
	p := fnPtr.Load()
	if p == nil {
		return 0, false
	}
	return (*p)(protocol)
}

// bootstrap is the distance each declared protocol ranks at before sysrib has
// published the declaration: the window between process start and sysrib's
// first Set. Each value is the YANG default of the leaf it stands in for, and
// internal/component/sysrib/distance_bootstrap_test.go compares the two, so
// this one copy cannot drift from the declaration in silence.
var bootstrap = map[string]uint8{
	"connected": 0,
	"static":    10,
	"ebgp":      20,
	"ospf":      110,
	"isis":      115,
	"ibgp":      200,
}

// Bootstrap returns a copy of the bootstrap table, for the check that pins it
// to the YANG declaration.
func Bootstrap() map[string]uint8 {
	return maps.Clone(bootstrap)
}

// Resolve returns the distance protocol ranks at: the declared value once the
// declaration has answered, the bootstrap value before that, and false for a
// protocol neither names.
func Resolve(protocol string) (uint8, bool) {
	if d, ok := Of(protocol); ok {
		return d, true
	}
	d, ok := bootstrap[protocol]
	return d, ok
}
