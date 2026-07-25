// Design: plan/learned/934-isis-9-spf-rib.md -- engine <-> SPF wiring (LSDB read, next-hop, install).
// Related: server.go -- the engine struct + lifecycle this extends
// Related: lsdb_wiring.go -- the LSDB the SPF Source reads and the change points that Trigger SPF
// Related: internal/plugins/isis/spf -- the graph/Dijkstra/route/install package
//
// RFC: rfc/short/rfc1195.md -- TLV 132 neighbor interface address is the SPF next-hop source
// RFC: rfc/short/rfc5305.md -- TLV 22 edges / TLV 135 prefixes the SPF graph reads
//
// This file is the root-package glue between the LSDB + adjacency runtime and the
// spf package: it builds the engine's SPF Computer, adapts the LSDB to the SPF
// Source interface (decoding each held LSP once per run), adapts the per-circuit
// adjacency tables to the SPF NextHopResolver (a first-hop System ID -> the
// neighbor's learned IPv4 address + its circuit), triggers a debounced SPF run on
// every LSDB change, and serves the `show isis route` snapshot. FIB install is
// the spf package's job (Loc-RIB insertion -> sysrib -> fibkernel); this layer
// owns no second FIB path.

package isis

import (
	"context"
	"net/netip"

	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routeinstall"
	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/circuit"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/spf"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// initSPF constructs the engine's SPF Computer, wiring the LSDB as the link-state
// Source, the engine itself as the NextHopResolver, and the shared process-wide
// Loc-RIB as the install target (locrib.Default(); nil in a forked subprocess, in
// which case install no-ops exactly like the BGP RIB). Called from newEngine
// after initLSDB so the LSDB exists. The configured levels are set in setConfig
// once the level is known; here both levels are wired and SPF computes the ones
// present in the graph.
func (e *engine) initSPF() {
	loc := locrib.Default()
	inst := spf.NewInstaller(loc)
	instV6 := spf.NewInstallerV6(loc)
	if loc == nil {
		// Forked subprocess: no local Loc-RIB (default.go returns nil under
		// ze.plugin.hub.token). Ship SPF routes to the engine over RPC via the
		// route-install client set at engine start; one sink serves both families.
		// (spec-forked-route-install)
		if client := routeInstallClient(); client != nil {
			sink := routeinstall.New(context.Background(), client)
			inst.SetRemoteSink(sink)
			instV6.SetRemoteSink(sink)
		}
	}
	e.spf = spf.NewComputer(spf.Config{
		Source:   (*lsdbSPFSource)(e),
		Resolver: (*engineNextHopResolver)(e),
		Levels:   []spf.Level{spf.Level1, spf.Level2},
		// IPv4 install path (isis-9).
		Installer: inst,
		// IPv6 install path (isis-12): the SAME shared SPF tree, the IPv6 next-hop
		// resolver (neighbor link-local from TLV 232), and the IPv6-family Loc-RIB
		// Installer. The IPv6 pass is always wired; it produces no routes on an
		// IPv4-only topology (no TLV 236 leaves), so wiring it unconditionally is
		// harmless and keeps the engine dual-stack-ready.
		ResolverV6:  (*engineNextHopResolverV6)(e),
		InstallerV6: instV6,
	})
	// RFC 2966 inter-level leak (AC-4/AC-5): after each SPF run the Computer hands
	// the engine the other level's reachable prefixes to re-originate into this
	// level's own LSP (L2->L1 with the up/down bit, L1->L2 up). applyLeak stores
	// the set and re-originates only on a change; because the leak skips down-bit
	// prefixes, the re-origination's SPF run recomputes the same set and the loop
	// terminates. On a single-level node the leak is always empty (no-op).
	e.spf.SetOnLeak(e.applyLeak)
}

// triggerSPF arms a debounced SPF run (spec AC-9: a burst of LSDB changes
// collapses to one run per level). Called from every LSDB change point
// (origination, aging/purge, received-LSP store) so routes track the topology. A
// nil Computer (engine without SPF, e.g. an early test) is a no-op.
func (e *engine) triggerSPF() {
	if e.spf == nil {
		return
	}
	// Tag the next recorded SPF run as LSDB-driven for `show isis spf-log`
	// (spec-isis-13 AC-6). A direct Run() with no tag reports "manual".
	e.spf.SetSPFLogTrigger("lsdb-change")
	e.spf.Trigger()
}

// spfLogSnapshot returns the `show isis spf-log` view (spec-isis-13 AC-6): the
// recent SPF runs newest-first (timestamp, level, trigger, duration, node
// count). Empty when SPF has not run. A nil Computer is a no-op.
func (e *engine) spfLogSnapshot() []spf.SPFLogEntry {
	if e.spf == nil {
		return nil
	}
	return e.spf.SPFLog()
}

// routeSnapshot returns the `show isis route` view (rendered by isis-13) of the
// routes SPF currently has installed in the Loc-RIB. Empty when SPF has not run
// or no remote prefix is reachable.
func (e *engine) routeSnapshot() []spf.RouteSnapshotEntry {
	if e.spf == nil {
		return nil
	}
	return e.spf.Snapshot()
}

// routeSnapshotV6 returns the `show isis route ipv6` view (isis-12) of the IPv6
// routes SPF currently has installed in the Loc-RIB. Empty when SPF has not run
// or no remote IPv6 prefix is reachable.
func (e *engine) routeSnapshotV6() []spf.RouteSnapshotEntry {
	if e.spf == nil {
		return nil
	}
	return e.spf.SnapshotV6()
}

// ---- LSDB -> spf.Source adapter ----
//
// lsdbSPFSource adapts the engine's LSDB to the spf.Source interface. It is the
// engine viewed through that single read method, so SPF reads the database once
// per run (spec A-9) without the spf package importing lsdb.

type lsdbSPFSource engine

// spfLevel maps an spf.Level to the LSDB level.
func spfToLSDBLevel(l spf.Level) lsdb.Level {
	if l == spf.Level2 {
		return lsdb.Level2
	}
	return lsdb.Level1
}

// Records returns every non-purged LSP held at the level, decoded into the SPF
// record shape (originator Source ID, overload bit, decoded TLVs). A purged LSP
// (zero Remaining Lifetime) is excluded from SPF (it advertises nothing). A
// malformed stored LSP is skipped (one bad node, not the whole run; security
// review: error isolation).
func (s *lsdbSPFSource) Records(level spf.Level) []spf.LSPRecord {
	e := (*engine)(s)
	if e.lsdb == nil {
		return nil
	}
	ll := spfToLSDBLevel(level)
	ids := e.lsdb.LSPIDs(ll)
	out := make([]spf.LSPRecord, 0, len(ids))
	for _, id := range ids {
		entry := e.lsdb.Lookup(ll, id)
		if entry == nil || entry.IsPurged() {
			continue
		}
		lsp, err := entry.Decode()
		if err != nil {
			continue
		}
		out = append(out, spf.LSPRecord{
			Source:   id.SourceID(),
			Overload: entry.IsOverloaded(),
			LSP:      lsp,
		})
	}
	return out
}

// ---- adjacency tables -> spf.NextHopResolver adapter ----
//
// engineNextHopResolver adapts the engine's per-circuit adjacency tables to the
// spf.NextHopResolver interface: given a first-hop neighbor System ID and a
// level, it returns the neighbor's learned IPv4 interface address (TLV 132,
// stored on the adjacency) and the circuit it is adjacent on (the outgoing
// interface). This is the Shared Contracts "Next-hop derivation for SPF".

type engineNextHopResolver engine

// ResolveNextHop finds an Up adjacency to neighbor at level across all circuits
// and returns its learned IPv4 next-hop and outgoing interface. Returns ok=false
// when no Up adjacency with a valid IPv4 address exists (the next-hop is then
// dropped by SPF). The first matching circuit (lowest-named, since Snapshot
// sorts) wins for a multi-homed neighbor, deterministically.
//
// It reads adjacency state via the table's locked Snapshot (value copies), NOT
// the live *Adjacency pointer: the circuit goroutine is the single writer and
// mutates records under the table lock, so reading the live pointer's fields off
// that lock would race (the FSM writes State/IPv4 on every Hello). Snapshot
// copies under the read lock, so the SPF run (on its own goroutine) is safe.
func (r *engineNextHopResolver) ResolveNextHop(level spf.Level, neighbor types.SystemID) (spf.NextHop, bool) {
	e := (*engine)(r)
	adjLevelTok := adjacency.Level1.String()
	if level == spf.Level2 {
		adjLevelTok = adjacency.Level2.String()
	}
	neighborTok := neighbor.String()

	e.circuitsMu.RLock()
	circuits := make([]*circuit.Circuit, 0, len(e.circuitByName))
	for _, c := range e.circuitByName {
		circuits = append(circuits, c)
	}
	e.circuitsMu.RUnlock()

	for _, c := range circuits {
		for _, row := range c.Table().Snapshot() {
			if row.SystemID != neighborTok || row.Level != adjLevelTok {
				continue
			}
			if row.State != adjacency.StateUp.String() || row.IPv4 == "" {
				continue
			}
			addr, err := netip.ParseAddr(row.IPv4)
			if err != nil || !addr.IsValid() {
				continue
			}
			return spf.NextHop{Addr: addr, Interface: c.Name()}, true
		}
	}
	return spf.NextHop{}, false
}

// ---- adjacency tables -> spf.NextHopResolverV6 adapter (isis-12) ----
//
// engineNextHopResolverV6 adapts the engine's per-circuit adjacency tables to the
// spf.NextHopResolverV6 interface: given a first-hop neighbor System ID and a
// level, it returns the neighbor's learned IPv6 LINK-LOCAL address (TLV 232 from
// its IIH, stored on the adjacency as row.IPv6) and the circuit it is adjacent on
// (Shared Contracts "Next-hop derivation for SPF"). A link-local next-hop is only
// usable with its interface, so the circuit name is always carried (R-2).

type engineNextHopResolverV6 engine

// ResolveNextHopV6 finds an Up adjacency to neighbor at level across all circuits
// and returns its learned IPv6 next-hop (the neighbor link-local) and outgoing
// interface. Returns ok=false when no Up adjacency with a valid IPv6 address
// exists (the next-hop is then dropped by SPF, never installed pointing nowhere).
// Reads adjacency state via the table's locked Snapshot (value copies), NOT the
// live *Adjacency pointer, for the same race-safety reason as the IPv4 resolver.
func (r *engineNextHopResolverV6) ResolveNextHopV6(level spf.Level, neighbor types.SystemID) (spf.NextHop, bool) {
	e := (*engine)(r)
	adjLevelTok := adjacency.Level1.String()
	if level == spf.Level2 {
		adjLevelTok = adjacency.Level2.String()
	}
	neighborTok := neighbor.String()

	e.circuitsMu.RLock()
	circuits := make([]*circuit.Circuit, 0, len(e.circuitByName))
	for _, c := range e.circuitByName {
		circuits = append(circuits, c)
	}
	e.circuitsMu.RUnlock()

	for _, c := range circuits {
		for _, row := range c.Table().Snapshot() {
			if row.SystemID != neighborTok || row.Level != adjLevelTok {
				continue
			}
			if row.State != adjacency.StateUp.String() || row.IPv6 == "" {
				continue
			}
			addr, err := netip.ParseAddr(row.IPv6)
			if err != nil || !addr.IsValid() || !addr.Is6() {
				continue
			}
			// The next-hop is typically link-local; carry the circuit so the kernel
			// can resolve the egress for the fe80:: address (RFC 5308 sec 3, R-2).
			return spf.NextHop{Addr: addr.WithZone(""), Interface: c.Name()}, true
		}
	}
	return spf.NextHop{}, false
}
