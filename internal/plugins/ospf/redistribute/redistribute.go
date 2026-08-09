// Design: docs/architecture/ospf/ospf-10-as-external-asbr.md -- OSPF redistribution (both directions).
// Related: internal/plugins/ospf/lsdb -- Type 5 AS-External-LSA origination.
// Related: internal/plugins/ospf/redistribute/events -- redistevents producer wiring.
//
// Package ospfredistribute wires OSPF into Ze's protocol-agnostic redistribution
// framework in BOTH directions (umbrella AC-7 / AC-8):
//
//   - Producer (source.go): OSPF registers the SINGLE config source "ospf"
//     (RegisterSource) and, via the events sub-package, the redistevents producer
//     (RegisterProducer). SPF route changes are EMITTED as redistevents
//     RouteChangeBatch to the redistribute-orchestrator, which dispatches them to
//     the BGP consumer (export OSPF -> BGP). It NEVER installs to the FIB (that is
//     spec-ospf-8's Loc-RIB path).
//   - Consumer (consumer.go): OSPF implements RedistConsumer ("ospf"), turning
//     connected/static/BGP RouteEntry imports into Type 5 AS-External-LSAs in the
//     AS-wide store, making the node an ASBR, then re-flooding.
//
// All OSPF redistribution code lives under this package (plugin-self-containment):
// no "ospf" spelling appears in the generic config/redistribute package.

package ospfredistribute

import "net/netip"

// ExternalInjector is the engine-facing seam the consumer uses to originate and
// withdraw Type 5 AS-External-LSAs for redistributed routes. The OSPF engine
// implements it (register.go wires the live engine; tests use a fake). It is
// deliberately narrow: the redistribution code owns the per-prefix source
// bookkeeping and only asks the engine to (a) originate a Type 5 for a prefix
// learned from source, and (b) withdraw it. The engine owns the per-source
// metric / metric-type / route-tag lookup (from the `ospf` container's
// `redistribute` config), the ASBR E-bit, and the AS-wide flooding.
type ExternalInjector interface {
	// InjectExternal originates (or replaces) a Type 5 AS-External-LSA for prefix,
	// learned from source (connected/static/bgp). It returns an error so the
	// consumer can log a failed origination instead of swallowing it (R-3).
	InjectExternal(prefix netip.Prefix, source string) error
	// WithdrawExternal MaxAge-purges the Type 5 for prefix, reporting whether one
	// existed. Returning false (never injected) is not an error.
	WithdrawExternal(prefix netip.Prefix) (bool, error)
}

// OptionalInjector is an ExternalInjector whose backing engine may be absent (e.g. an RFC
// 5838 IPv4-over-OSPFv3 AF that is not configured). injectorFor treats an inactive optional
// injector as "not present" and falls back to the base injector, so IPv4 redistribution
// still originates an OSPFv2 Type 5 rather than being silently swallowed (ext-15 review).
type OptionalInjector interface {
	ExternalInjector
	// Active reports whether the injector currently has a backing engine to inject into.
	Active() bool
}
