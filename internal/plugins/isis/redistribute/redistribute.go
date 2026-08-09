// Design: docs/architecture/isis/isis-11-redistribution.md -- IS-IS redistribution (both directions).
// Related: internal/plugins/isis/lsdb -- PrefixInfo (TLV 135) origination input.
// Related: internal/plugins/isis/redistribute/events -- redistevents producer wiring.
//
// Package isisredistribute wires IS-IS into Ze's protocol-agnostic redistribution
// framework in BOTH directions (umbrella AC-7 / AC-8):
//
//   - Producer (source.go): IS-IS registers the SINGLE config source "isis"
//     (RegisterSource) and, via the events sub-package, the redistevents producer
//     (RegisterProducer). SPF route changes are EMITTED as redistevents
//     RouteChangeBatch to the redistribute-orchestrator, which dispatches them to
//     the BGP consumer (export IS-IS -> BGP). It NEVER installs to the FIB (that is
//     spec-isis-9's Loc-RIB path).
//   - Consumer (consumer.go): IS-IS implements RedistConsumer ("isis"), turning
//     connected/static/BGP RouteEntry imports into TLV 135 Extended IP
//     Reachability entries in the local LSP set, then re-originating.
//
// All IS-IS redistribution code lives under this package (plugin-self-containment):
// no "isis" spelling appears in the generic config/redistribute package.

package isisredistribute

import (
	"net/netip"

	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
)

// DefaultRedistMetric is the FIXED default 32-bit prefix metric (RFC 5305 sec 4,
// TLV 135) applied to every redistributed route. The generic RouteEntry carries
// no metric (umbrella A-3), so v1 uses this single code constant rather than a
// config leaf; a configurable/per-route metric is future work. The value is the
// classical "external" default reachability cost; it is well below
// MAX_PATH_METRIC (0xFE000000) so SPF never treats a redistributed prefix as
// unreachable.
const DefaultRedistMetric uint32 = 100

// LSPInjector is the engine-facing seam the consumer (and connected-prefix
// advertisement) use to write redistributed/connected reachability into the
// node's own LSPs. The IS-IS engine implements it (register.go wires the live
// engine; tests use a fake). It is deliberately narrow: the redistribution code
// owns the prefix bookkeeping and only asks the engine to (a) tell it which levels
// to advertise into, (b) store/remove a redistributed TLV 135 prefix for a level,
// and (c) re-originate.
//
// SetRedistPrefix / RemoveRedistPrefix mutate the engine's redistributed-prefix
// set (distinct from connected prefixes); Originate triggers a full re-origination
// of the own LSP set (ISO/IEC 10589 clause 7.3.12), returning an error so the
// consumer can log a failed re-origination instead of swallowing it (R-3).
type LSPInjector interface {
	// OriginationLevels returns the LSDB levels the node originates own LSPs for
	// (L1, L2, or both). Redistributed routes are advertised into all of them in
	// v1 (no per-level redistribution selector).
	OriginationLevels() []lsdb.Level
	// SetRedistPrefix stores (adds or replaces) a redistributed TLV 135 prefix for
	// a level.
	SetRedistPrefix(level lsdb.Level, info lsdb.PrefixInfo)
	// RemoveRedistPrefix removes a redistributed prefix for a level, reporting
	// whether it existed.
	RemoveRedistPrefix(level lsdb.Level, prefix netip.Prefix) bool
	// SetRedistPrefixV6 stores (adds or replaces) a redistributed TLV 236 IPv6
	// prefix for a level (isis-12).
	SetRedistPrefixV6(level lsdb.Level, info lsdb.PrefixInfoV6)
	// RemoveRedistPrefixV6 removes a redistributed IPv6 prefix for a level,
	// reporting whether it existed (isis-12).
	RemoveRedistPrefixV6(level lsdb.Level, prefix netip.Prefix) bool
	// Originate re-originates the node's own LSP set across all configured levels.
	// It returns an error if origination fails so the caller can log it.
	Originate() error
}
