// Design: plan/learned/932-isis-6-lsdb.md -- engine <-> LSDB wiring (origination trigger, aging loop).
// Related: server.go -- the engine struct + lifecycle this extends
// Related: circuits.go -- the adjacency circuits whose Up neighbors feed origination
// Related: events.go -- the LSPChange event emitted on an LSDB change
//
// RFC: rfc/short/rfc1195.md -- TLV 129/132 origination from live state
// RFC: rfc/short/rfc5305.md -- TLV 22 (Extended IS Reachability) wide metric; the broadcast-circuit star entry (isis-8)
//
// This file is the root-package glue between the per-circuit adjacency runtime
// (isis-5) and the LSDB subsystem (isis-6, internal/plugins/isis/lsdb). It owns
// the engine's LSDB instance and originator, the origination trigger fired on an
// adjacency Up/Down (the Wiring Test: adjacency Up -> origination -> store -> SRM
// on eligible circuits), the per-second aging loop, the LSP-change event
// emission, and the `show isis database` snapshot. The flooding of the SRM-armed
// LSPs and the wire transmission are isis-7; this spec sets the flags and stores.

package isis

import (
	"net/netip"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/circuit"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/spf"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// agingInterval is the LSP aging cadence: once per second every LSDB entry's
// Remaining Lifetime is decremented (ISO/IEC 10589 clause 7.3.16.4).
const agingInterval = 1 * time.Second

// initLSDB constructs the engine's LSDB and originator. Called from newEngine so
// the store exists before any circuit opens or any LSP arrives.
func (e *engine) initLSDB() {
	e.lsdb = lsdb.New(time.Now)
	e.originator = lsdb.NewOriginator(e.lsdb, time.Now)
}

// circuitIDFor returns a stable lsdb.CircuitID for an interface name, assigning a
// fresh one on first use. The LSDB indexes SRM/SSN flags by this small ID (spec
// A-3) rather than the sparse kernel ifindex. The engine holds the mapping so a
// reopened interface keeps its ID and a closed one's flags can be cleared.
func (e *engine) circuitIDFor(name string) lsdb.CircuitID {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.circuitIDs == nil {
		e.circuitIDs = make(map[string]lsdb.CircuitID)
	}
	if id, ok := e.circuitIDs[name]; ok {
		return id
	}
	e.nextCircuitID++
	id := e.nextCircuitID
	e.circuitIDs[name] = id
	return id
}

// startAgingLoop launches the per-second LSP aging goroutine (ISO/IEC 10589
// clause 7.3.16.4). Each tick decrements every entry's Remaining Lifetime,
// purges entries that reach 0 (re-flood + grace, isis-7 sends the purge), and
// garbage-collects entries past the grace period. Newly purged own/foreign LSPs
// get SRM re-armed on all eligible circuits so the purge floods, and an LSP
// change event is emitted. The goroutine stops on ctx cancellation (shutdown).
func (e *engine) startAgingLoop() {
	e.wg.Go(func() {
		tick := time.NewTicker(agingInterval)
		defer tick.Stop()
		for {
			select {
			case <-e.ctx.Done():
				return
			case <-tick.C:
				e.ageOnce()
			}
		}
	})
}

// ageOnce runs one aging tick and reacts to its result: arm SRM to flood freshly
// purged LSPs (isis-7 transmits) and emit LSP-change events for purges and
// deletions. Extracted from the loop so a test can drive a single tick.
//
// A purge event is either a LOCAL expiry (an LSP that aged to 0 here) or a
// RECEIVED purge that the tick surfaces once for re-flood within its grace window
// (ISO/IEC 10589 clause 7.3.16: a received purge is re-flooded and retained,
// distinct from a local expiry). Both arm SRM so the purge floods; the
// distinction is read from the event (p.ReceivedPurge) so the two paths stay
// observably separate (spec AC-9, R-4) and a future divergence in behavior has a
// single hook.
func (e *engine) ageOnce() {
	res := e.lsdb.Tick()
	for _, p := range res.PurgedL1 {
		e.refloodPurge(lsdb.Level1, p)
	}
	for _, p := range res.PurgedL2 {
		e.refloodPurge(lsdb.Level2, p)
	}
	for _, del := range res.DeletedL1 {
		e.emitLSPChange("l1", del.LSPID, 0, "purge")
	}
	for _, del := range res.DeletedL2 {
		e.emitLSPChange("l2", del.LSPID, 0, "purge")
	}
	// Periodic own-LSP and pseudo-node refresh (ISO/IEC 10589 clause 7.3.16.1,
	// spec-isis-6 AC-3). Folded into the aging tick rather than a separate goroutine
	// because this loop is already lifecycle-managed (stopped on shutdown) and runs
	// at the right 1s cadence: in a quiescent network nothing else calls originate()
	// or originatePseudonode(), so without this the node's own LSPs (and a DIS's
	// pseudo-node LSP) age to MaxAge and purge -- the node disappears from peers'
	// LSDBs. Both calls are cheap when no refresh is due (a timestamp compare, no
	// levelState rebuild).
	e.refreshOwnLSPs()
	e.refreshPseudonodes()
}

// refloodPurge arms SRM on every eligible circuit so the purge floods (isis-7
// transmits the stored zero-lifetime LSP) and emits the LSP-change event. It
// handles both a local expiry and a received purge: a received purge (clause
// 7.3.16) is re-flooded distinctly from a local expiry, but both re-arm SRM here
// so the zero-lifetime LSP propagates. The p.ReceivedPurge distinction is read so
// the received-purge re-flood is a deliberate, traced path rather than an
// accident of the local-expiry loop (spec AC-9, R-4).
func (e *engine) refloodPurge(level lsdb.Level, p lsdb.PurgeEvent) {
	e.armFloodByString(level, p.LSPID)
	e.emitLSPChange(level.String(), p.LSPID, 0, "purge")
}

// origInput is the per-level INPUT to own-LSP origination: the node identity and
// the live level state. origination (lsdb.Originator.Originate) is a pure function
// of this pair -- identical input deterministically yields identical own-LSP
// bytes (the sequence number aside, which the originator always bumps). So the
// engine can compare the freshly-built input against the last-originated input and
// skip the regenerate/store/flood when nothing changed, collapsing a burst of
// redundant re-origination requests (an adjacency flap fires originate() from
// several goroutines for the SAME resulting state) to ONE re-flood / ONE sequence
// bump (the flooding-amplification fix; ISO/IEC 10589 clause 7.3.12 still requires
// full regeneration on a REAL change, which a differing input triggers).
type origInput struct {
	node  lsdb.NodeInfo
	state lsdb.LevelState
}

// originate regenerates the node's own LSP set for every configured level from
// the live adjacency + prefix state and stores it in the LSDB (the Wiring Test:
// adjacency Up -> origination). It is called on any adjacency transition and on
// circuit open/close. For each (re)originated or purged fragment it arms SRM on
// all eligible circuits (so isis-7 floods it) and emits an LSP-change event.
// Origination is full regeneration (not incremental), matching ISO/IEC 10589
// clause 7.3.12. A node with no NET (idle engine) originates nothing.
//
// The whole reaction runs under e.origMu so the per-level "did the input change?"
// compare-and-originate is atomic across the many goroutines that call originate()
// (transition hooks, the DIS re-elect tick, circuit close, redistribution, the SPF
// leak callback). Serializing a control-plane event is cheap and removes both the
// flooding amplification AND the build-then-store reorder window.
func (e *engine) originate() {
	e.origMu.Lock()
	defer e.origMu.Unlock()

	e.mu.Lock()
	cfg := e.cfg
	e.mu.Unlock()
	if !cfg.Present() || e.originator == nil {
		return
	}

	node := e.nodeInfo(cfg)
	for _, level := range originationLevels(cfg.Level) {
		state := e.levelState(level)
		// Skip a redundant re-origination: the input is byte-equal to the last
		// origination's input AND no refresh is yet due. A topology change (a
		// different input) or an elapsed refresh interval both fall through and
		// re-originate, so a real change always floods and an own LSP never ages out
		// (clause 7.3.16.1).
		if e.originationUnchanged(level, node, state, cfg) {
			continue
		}
		e.setLastOrigInput(level, node, state)

		res := e.originator.Originate(level, node, state)
		levelTok := level.String()
		for _, id := range res.Originated {
			e.armFlood(level, id)
			e.emitLSPChange(levelTok, id.String(), uint32(e.sequenceOf(level, id)), "add")
		}
		for _, id := range res.Purged {
			e.armFlood(level, id)
			e.emitLSPChange(levelTok, id.String(), uint32(e.sequenceOf(level, id)), "purge")
		}
	}
}

// originationUnchanged reports whether re-originating level right now would be a
// pure no-op: the origination input matches the last one AND no refresh is yet due.
// The caller holds e.origMu. A first origination (no recorded input), a changed
// input, or an elapsed refresh interval all return false so origination proceeds.
//
// Freshness is measured as elapsed time since the last origination, compared to
// lsp-refresh-interval (ISO/IEC 10589 clause 7.3.16.1: an own LSP is refreshed at
// lsp-refresh-interval, well before MaxAge). Once that long has passed an unchanged
// input is still re-originated to reset the lifetime and bump the sequence so the
// LSP never expires in the network. Elapsed time is used (not the stored entry's
// Remaining Lifetime) because the LSDB aging goroutine mutates that lifetime under
// the LSDB lock and originate() holds no LSDB lock here; reading it would race.
//
// The periodic refresh in a fully quiescent network (no adjacency event for a whole
// refresh interval) is driven by the aging loop: refreshOwnLSPs (ageOnce) calls
// originate() once a refresh is due on any level, and this guard then re-originates
// the due levels (clause 7.3.16.1, spec-isis-6 AC-3). So this guard both collapses a
// burst of identical re-origination requests AND, paired with the aging-loop driver,
// guarantees an own LSP is re-stamped before MaxAge and never ages out.
func (e *engine) originationUnchanged(level lsdb.Level, node lsdb.NodeInfo, state lsdb.LevelState, cfg Config) bool {
	e.mu.Lock()
	prev, ok := e.lastOrigInput[level]
	at, hadAt := e.lastOrigAt[level]
	e.mu.Unlock()
	if !ok || !hadAt || !origInputEqual(prev, origInput{node: node, state: state}) {
		return false
	}
	// Input unchanged: re-originate only once a refresh is due (the refresh interval
	// has elapsed since the last origination).
	return time.Since(at) < refreshInterval(cfg)
}

// refreshInterval resolves the own-LSP refresh period (ISO/IEC 10589 clause
// 7.3.16.1: an own LSP is refreshed at lsp-refresh-interval, well before MaxAge).
// It is the lsp-refresh-interval leaf, with two guards so a refresh ALWAYS lands
// before the lifetime: an unset (0) refresh, or one >= the lifetime (which would
// let the LSP age out before a refresh), falls back to half the lifetime. The same
// resolution drives both the coalescing skip (originationUnchanged) and the
// periodic refresh-due probe (refreshDueLevels), so the two never disagree on when
// a refresh is owed.
func refreshInterval(cfg Config) time.Duration {
	refresh := cfg.LSPRefreshInterval
	lifetime := cfg.LSPLifetime
	if lifetime == 0 {
		lifetime = DefaultLSPLifetime
	}
	if refresh == 0 || refresh >= lifetime {
		refresh = lifetime / 2
	}
	return time.Duration(refresh) * time.Second
}

// refreshOwnLSPs re-originates the node's own LSP set when a periodic refresh is
// due on any configured level (ISO/IEC 10589 clause 7.3.16.1, spec-isis-6 AC-3).
// It is called once per second by the aging loop (ageOnce): in a fully QUIESCENT
// network no adjacency/topology event fires originate(), so without this driver an
// own LSP would never be re-stamped and would age to MaxAge and purge -- the node
// would vanish from every peer's LSDB. The check is cheap: refreshDueLevels only
// compares the recorded last-origination timestamps against the refresh interval
// (it does NOT rebuild levelState every second). When a refresh is due on at least
// one level it calls originate(), whose per-level coalescing (originationUnchanged)
// re-originates EXACTLY the due levels -- bumping the sequence and resetting the
// Remaining Lifetime to MaxAge -- and is a no-op on the others, so a quiescent node
// refreshes without spurious re-floods.
func (e *engine) refreshOwnLSPs() {
	if len(e.refreshDueLevels(time.Now())) == 0 {
		return
	}
	e.originate()
}

// refreshDueLevels returns the configured levels whose own LSP is due a periodic
// refresh as of now: the level has a recorded last-origination timestamp and at
// least refreshInterval has elapsed since it. A level never yet originated (no
// timestamp) is NOT reported -- origination is driven by openCircuits/transitions,
// not by this probe. Exposed (rather than inlined in refreshOwnLSPs) so a test can
// drive the refresh-due decision deterministically by planting lastOrigAt in the
// past, without sleeping for a real refresh interval. The decision reads only the
// timestamps under e.mu, so it is cheap enough to run every aging tick.
func (e *engine) refreshDueLevels(now time.Time) []lsdb.Level {
	e.mu.Lock()
	cfg := e.cfg
	e.mu.Unlock()
	if !cfg.Present() {
		return nil
	}
	interval := refreshInterval(cfg)
	var due []lsdb.Level
	e.mu.Lock()
	for _, level := range originationLevels(cfg.Level) {
		at, ok := e.lastOrigAt[level]
		if ok && now.Sub(at) >= interval {
			due = append(due, level)
		}
	}
	e.mu.Unlock()
	return due
}

// setLastOrigInput records the origination input and timestamp for level so the
// next originate() can detect an unchanged input and skip the redundant re-flood
// (and force a refresh once the interval elapses). The caller holds e.origMu. The
// slices in state are freshly built by levelState each call and not mutated
// afterward, so retaining them is safe.
func (e *engine) setLastOrigInput(level lsdb.Level, node lsdb.NodeInfo, state lsdb.LevelState) {
	e.mu.Lock()
	if e.lastOrigInput == nil {
		e.lastOrigInput = make(map[lsdb.Level]origInput)
	}
	if e.lastOrigAt == nil {
		e.lastOrigAt = make(map[lsdb.Level]time.Time)
	}
	e.lastOrigInput[level] = origInput{node: node, state: state}
	e.lastOrigAt[level] = time.Now()
	e.mu.Unlock()
}

// origInputEqual reports whether two origination inputs are equal: same node
// identity/attributes and the same live level state. Equality means origination
// would produce the identical own LSP. The slices are compared element-wise
// (AreaID holds a byte slice, so it needs AreaID.Equal, not ==; every other field
// is a comparable value type). Order matters and is stable: levelState builds the
// neighbor/prefix/address lists deterministically (de-duped in iteration order,
// the IPv6 prefixes sorted), so a positional compare is a correct change detector.
func origInputEqual(a, b origInput) bool {
	return nodeInfoEqual(a.node, b.node) && levelStateEqual(a.state, b.state)
}

// nodeInfoEqual reports whether two NodeInfo values are equal for origination.
func nodeInfoEqual(a, b lsdb.NodeInfo) bool {
	if a.SystemID != b.SystemID ||
		a.Hostname != b.Hostname ||
		a.AdvertiseIPv4 != b.AdvertiseIPv4 ||
		a.AdvertiseIPv6 != b.AdvertiseIPv6 ||
		a.Overload != b.Overload ||
		a.MaxLifetime != b.MaxLifetime ||
		a.MaxLSPSize != b.MaxLSPSize {
		return false
	}
	if len(a.Areas) != len(b.Areas) {
		return false
	}
	for i := range a.Areas {
		if !a.Areas[i].Equal(b.Areas[i]) {
			return false
		}
	}
	return true
}

// levelStateEqual reports whether two LevelState values are equal for origination.
func levelStateEqual(a, b lsdb.LevelState) bool {
	if len(a.Neighbors) != len(b.Neighbors) ||
		len(a.Prefixes) != len(b.Prefixes) ||
		len(a.InterfaceAddrs) != len(b.InterfaceAddrs) ||
		len(a.PrefixesV6) != len(b.PrefixesV6) ||
		len(a.InterfaceAddrsV6) != len(b.InterfaceAddrsV6) {
		return false
	}
	for i := range a.Neighbors {
		if a.Neighbors[i].Neighbor != b.Neighbors[i].Neighbor ||
			a.Neighbors[i].Metric.Value() != b.Neighbors[i].Metric.Value() {
			return false
		}
	}
	for i := range a.Prefixes {
		if !prefixInfoEqual(a.Prefixes[i], b.Prefixes[i]) {
			return false
		}
	}
	for i := range a.InterfaceAddrs {
		if a.InterfaceAddrs[i] != b.InterfaceAddrs[i] {
			return false
		}
	}
	for i := range a.PrefixesV6 {
		if !prefixInfoV6Equal(a.PrefixesV6[i], b.PrefixesV6[i]) {
			return false
		}
	}
	for i := range a.InterfaceAddrsV6 {
		if a.InterfaceAddrsV6[i] != b.InterfaceAddrsV6[i] {
			return false
		}
	}
	return true
}

// prefixInfoEqual reports whether two TLV 135 entries are identical (prefix,
// metric, up/down bit).
func prefixInfoEqual(a, b lsdb.PrefixInfo) bool {
	return a.Prefix == b.Prefix && a.Metric.Value() == b.Metric.Value() && a.UpDown == b.UpDown
}

// prefixInfoV6Equal reports whether two TLV 236 entries are identical (prefix,
// metric, up/down and external bits).
func prefixInfoV6Equal(a, b lsdb.PrefixInfoV6) bool {
	return a.Prefix == b.Prefix && a.Metric.Value() == b.Metric.Value() && a.UpDown == b.UpDown && a.External == b.External
}

// nodeInfo builds the lsdb.NodeInfo from the resolved config: the System ID, the
// area addresses (TLV 1), the hostname (TLV 137), the advertised protocols (TLV
// 129), the overload bit (RFC 3787), and the timers/MTU for lifetime and
// fragmentation.
func (e *engine) nodeInfo(cfg Config) lsdb.NodeInfo {
	return lsdb.NodeInfo{
		SystemID:      cfg.SystemID,
		Areas:         areaAddresses(cfg.NETs),
		Hostname:      cfg.Hostname,
		AdvertiseIPv4: true,
		AdvertiseIPv6: e.anyIPv6Circuit(),
		Overload:      cfg.Overload,
		MaxLifetime:   cfg.LSPLifetime,
		MaxLSPSize:    e.minCircuitMTU(),
	}
}

// levelState builds the lsdb.LevelState for one level: the Up adjacencies across
// all circuits forming that level (TLV 22 neighbors, each at the originating
// circuit's metric), the node's own IPv4 interface addresses (TLV 132), and the
// connected/redistributed prefixes (TLV 135, fed by isis-11 via SetPrefixes).
func (e *engine) levelState(level lsdb.Level) lsdb.LevelState {
	adjLevel := adjacency.Level1
	if level == lsdb.Level2 {
		adjLevel = adjacency.Level2
	}

	e.circuitsMu.RLock()
	circuits := make([]*circuit.Circuit, 0, len(e.circuitByName))
	for _, c := range e.circuitByName {
		circuits = append(circuits, c)
	}
	e.circuitsMu.RUnlock()

	var state lsdb.LevelState
	addrSeen := map[netip.Addr]struct{}{}
	addrSeenV6 := map[netip.Addr]struct{}{}

	for _, c := range circuits {
		ic, ok := e.interfaceConfig(c.Name())
		if !ok {
			continue
		}
		metric := levelMetric(ic, level)
		// The node's own interface address (TLV 132) is the SPF next-hop source.
		if a := interfaceIPv4(ic); a.IsValid() {
			if _, dup := addrSeen[a]; !dup {
				addrSeen[a] = struct{}{}
				state.InterfaceAddrs = append(state.InterfaceAddrs, a)
			}
		}
		// The node's own NON-LINK-LOCAL IPv6 interface addresses (TLV 232 in the
		// LSP, RFC 5308 sec 3). Only on circuits that advertise IPv6. Link-local
		// addresses go in the IIH TLV 232 (the circuit layer), never the LSP.
		if advertisesIPv6(ic) {
			for _, a := range interfaceIPv6NonLinkLocal(ic.Name) {
				if _, dup := addrSeenV6[a]; !dup {
					addrSeenV6[a] = struct{}{}
					state.InterfaceAddrsV6 = append(state.InterfaceAddrsV6, a)
				}
			}
		}
		m, err := types.NewMetric(metric)
		if err != nil {
			// Metric out of the 24-bit wide-metric range: clamp to the max.
			m, _ = types.NewMetric(types.MaxMetric)
		}
		// ISO/IEC 10589 clause 8.4.5 (the star, AC-7): on a BROADCAST circuit where
		// a DIS is elected (a pseudo-node exists), the own LSP advertises the LAN as
		// a SINGLE Extended IS Reachability (TLV 22) entry pointing at the
		// pseudo-node (metric = circuit metric), NOT one entry per peer -- only if at
		// least one adjacency is Up on the circuit at this level (a DIS with no Up
		// neighbor still points at its own pseudo-node so the LAN appears). A
		// point-to-point circuit always lists the neighbor directly (no DIS, no
		// pseudo-node). The pseudo-node identity comes from isis-8 (dis_wiring.go).
		if c.IsBroadcast() {
			if pn, ok := e.lookupElectedPseudonode(c.Name(), level); ok && e.circuitHasUpAt(c, adjLevel) {
				state.Neighbors = append(state.Neighbors, lsdb.AdjacencyInfo{
					Neighbor: pn,
					Metric:   m,
				})
				continue // do NOT also add per-peer entries for this circuit (R-3)
			}
			// No DIS elected yet on this broadcast circuit: fall through to per-peer
			// (transient, before the first election commits).
		}
		// Up adjacencies on this circuit at this level become TLV 22 neighbors
		// (point-to-point, or a broadcast circuit before a DIS is elected).
		for _, row := range c.Table().Snapshot() {
			if row.State != adjacency.StateUp.String() || row.Level != adjLevel.String() {
				continue
			}
			sys, err := types.ParseSystemID(row.SystemID)
			if err != nil {
				continue
			}
			state.Neighbors = append(state.Neighbors, lsdb.AdjacencyInfo{
				Neighbor: types.NewSourceID(sys, 0),
				Metric:   m,
			})
		}
	}

	// Connected prefixes (own enabled/passive interfaces, isis-11 connected
	// advertisement), redistributed prefixes (connected/static/BGP imported via
	// the redistribution consumer, isis-11), and INTER-LEVEL LEAKED prefixes (the
	// other level's reachability, RFC 2966, isis-9 AC-4/AC-5) all become TLV 135
	// entries. The leaked entries carry the up/down bit (set for an L2->L1 down
	// leak); the connected/redistributed ones carry it clear. A receiver arbitrates
	// duplicates by the RFC 5308 sec 5 order (route.go preferenceRank), so a leaked
	// copy of a connected prefix never wins over the connected one.
	e.mu.Lock()
	state.Prefixes = append(state.Prefixes, e.prefixes[level]...)
	for _, info := range e.redistPrefixes[level] {
		state.Prefixes = append(state.Prefixes, info)
	}
	state.Prefixes = append(state.Prefixes, e.leakedPrefixes[level]...)
	// IPv6 (isis-12): connected + redistributed + leaked IPv6 prefixes become TLV
	// 236 entries. The link-local exclusion (RFC 5308 sec 2) is enforced at the
	// source (interfaceIPv6Prefixes / the consumer), but filter again here as
	// defense in depth so an unexpected link-local entry never reaches TLV 236.
	v6 := make([]lsdb.PrefixInfoV6, 0, len(e.prefixesV6[level])+len(e.redistPrefixesV6[level])+len(e.leakedPrefixesV6[level]))
	v6 = append(v6, e.prefixesV6[level]...)
	for _, info := range e.redistPrefixesV6[level] {
		v6 = append(v6, info)
	}
	v6 = append(v6, e.leakedPrefixesV6[level]...)
	e.mu.Unlock()
	state.PrefixesV6 = lsdb.NonLinkLocalV6Prefixes(v6)

	return state
}

// setPrefixes replaces the connected/redistributed IPv4 prefixes the node
// advertises at level (TLV 135). isis-11 (redistribution) will call this and
// then trigger a re-origination. Stored on the engine so origination picks them
// up; the LSDB itself is prefix-agnostic. Held under the engine mutex.
func (e *engine) setPrefixes(level lsdb.Level, prefixes []lsdb.PrefixInfo) {
	e.mu.Lock()
	if e.prefixes == nil {
		e.prefixes = make(map[lsdb.Level][]lsdb.PrefixInfo)
	}
	e.prefixes[level] = slices.Clone(prefixes)
	e.mu.Unlock()
}

// setPrefixesV6 replaces the connected IPv6 prefixes the node advertises at level
// (TLV 236, isis-12). The IPv6 twin of setPrefixes; kept separate so connected
// IPv6 advertisement does not clobber the IPv4 connected set.
func (e *engine) setPrefixesV6(level lsdb.Level, prefixes []lsdb.PrefixInfoV6) {
	e.mu.Lock()
	if e.prefixesV6 == nil {
		e.prefixesV6 = make(map[lsdb.Level][]lsdb.PrefixInfoV6)
	}
	e.prefixesV6[level] = slices.Clone(prefixes)
	e.mu.Unlock()
}

// applyLeak stores the RFC 2966 inter-level leak set computed by the SPF Computer
// (SetOnLeak) and re-originates ONLY when the stored set actually changed. It is
// the origination side of L1<->L2 leaking (spec AC-4/AC-5): the L2-reachable
// prefixes are re-originated into L1 with the up/down bit set (down leak), and the
// L1-reachable prefixes into L2 up. spf.LeakPrefixes already enforced loop
// prevention (a prefix carrying the up/down bit is never leaked), which makes this
// a fixpoint: the leaked-down entries this run adds carry the up/down bit, so the
// re-origination's SPF run recomputes the SAME leak set, applyLeak sees no change,
// and the feedback loop terminates without churn.
//
// On a single-level node the leak is empty; if a previously non-empty set becomes
// empty (e.g. the source level lost every prefix) the stored set is cleared and a
// re-origination withdraws the leaked entries.
func (e *engine) applyLeak(leak spf.LeakResult) {
	v4 := map[lsdb.Level][]lsdb.PrefixInfo{
		lsdb.Level1: leakedToPrefixInfos(leak.IntoL1),
		lsdb.Level2: leakedToPrefixInfos(leak.IntoL2),
	}
	v6 := map[lsdb.Level][]lsdb.PrefixInfoV6{
		lsdb.Level1: leakedToPrefixInfosV6(leak.IntoL1V6),
		lsdb.Level2: leakedToPrefixInfosV6(leak.IntoL2V6),
	}

	e.mu.Lock()
	changed := false
	if e.leakedPrefixes == nil {
		e.leakedPrefixes = make(map[lsdb.Level][]lsdb.PrefixInfo)
	}
	if e.leakedPrefixesV6 == nil {
		e.leakedPrefixesV6 = make(map[lsdb.Level][]lsdb.PrefixInfoV6)
	}
	for _, level := range []lsdb.Level{lsdb.Level1, lsdb.Level2} {
		if !prefixInfosEqual(e.leakedPrefixes[level], v4[level]) {
			e.leakedPrefixes[level] = v4[level]
			changed = true
		}
		if !prefixInfosEqualV6(e.leakedPrefixesV6[level], v6[level]) {
			e.leakedPrefixesV6[level] = v6[level]
			changed = true
		}
	}
	e.mu.Unlock()

	// Only re-originate on a real change so the leak fixpoint does not re-flood an
	// identical own LSP every SPF run (the steady state is a no-op).
	if changed {
		e.originate()
	}
}

// leakedToPrefixInfos converts the SPF leak set into TLV 135 PrefixInfo entries
// for origination, carrying the RFC 2966 up/down bit each leaked prefix was
// stamped with (set for an L2->L1 down leak, clear for an L1->L2 up leak).
func leakedToPrefixInfos(leaked []spf.LeakedPrefix) []lsdb.PrefixInfo {
	if len(leaked) == 0 {
		return nil
	}
	out := make([]lsdb.PrefixInfo, 0, len(leaked))
	for _, lp := range leaked {
		out = append(out, lsdb.PrefixInfo{
			Prefix: lp.Prefix,
			Metric: types.NewPrefixMetric(lp.Metric),
			UpDown: lp.UpDown,
		})
	}
	return out
}

// leakedToPrefixInfosV6 is the TLV 236 (IPv6) twin of leakedToPrefixInfos
// (RFC 5308 sec 5). External is false: a leaked prefix is an IS-IS-internal
// reachability re-advertised across a level boundary, not a redistributed route.
func leakedToPrefixInfosV6(leaked []spf.LeakedPrefix) []lsdb.PrefixInfoV6 {
	if len(leaked) == 0 {
		return nil
	}
	out := make([]lsdb.PrefixInfoV6, 0, len(leaked))
	for _, lp := range leaked {
		out = append(out, lsdb.PrefixInfoV6{
			Prefix:   lp.Prefix,
			Metric:   types.NewPrefixMetric(lp.Metric),
			UpDown:   lp.UpDown,
			External: false,
		})
	}
	return out
}

// prefixInfosEqual reports whether two TLV 135 leak sets are identical (same
// prefixes, metrics, and up/down bits, in order). spf.LeakPrefixes returns a
// prefix-sorted slice, so a positional compare is a stable change detector that
// avoids a re-origination of an identical own LSP.
func prefixInfosEqual(a, b []lsdb.PrefixInfo) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Prefix != b[i].Prefix || a[i].Metric.Value() != b[i].Metric.Value() || a[i].UpDown != b[i].UpDown {
			return false
		}
	}
	return true
}

// prefixInfosEqualV6 is the TLV 236 twin of prefixInfosEqual.
func prefixInfosEqualV6(a, b []lsdb.PrefixInfoV6) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Prefix != b[i].Prefix || a[i].Metric.Value() != b[i].Metric.Value() || a[i].UpDown != b[i].UpDown {
			return false
		}
	}
	return true
}

// armFlood sets the SRM flag for (level, id) on every circuit forming that level
// so isis-7 floods the LSP. This spec only SETS the flags (the data model);
// isis-7 reads and clears them and performs the transmission. A circuit's
// configured level (from the running config) gates whether it is eligible for
// the level.
func (e *engine) armFlood(level lsdb.Level, id types.LSPID) {
	e.circuitsMu.RLock()
	names := make([]string, 0, len(e.circuitByName))
	for name := range e.circuitByName {
		names = append(names, name)
	}
	e.circuitsMu.RUnlock()
	for _, name := range names {
		ic, ok := e.interfaceConfig(name)
		if !ok || !configFormsLevel(ic.Level, level) {
			continue
		}
		e.lsdb.SetSRM(level, id, e.circuitIDFor(name))
	}
}

// armFloodByString arms SRM for an LSP ID given in its canonical string form
// (the aging tick reports IDs as strings). A parse failure is ignored (the ID
// came from the LSDB and is always well-formed).
func (e *engine) armFloodByString(level lsdb.Level, idStr string) {
	id, err := types.ParseLSPID(idStr)
	if err != nil {
		return
	}
	e.armFlood(level, id)
}

// clearCircuitFlags drops a closed circuit's SRM/SSN flags from every LSP so a
// stale circuit ID is never left flagged. Called on circuit close.
func (e *engine) clearCircuitFlags(name string) {
	e.mu.Lock()
	id, ok := e.circuitIDs[name]
	e.mu.Unlock()
	if !ok || e.lsdb == nil {
		return
	}
	e.lsdb.ClearCircuit(id)
}

// sequenceOf returns the stored sequence number for (level, id), or 0 when the
// entry is absent (e.g. a purge already garbage-collected). Used to populate the
// LSP-change event payload.
func (e *engine) sequenceOf(level lsdb.Level, id types.LSPID) types.SequenceNumber {
	if entry := e.lsdb.Lookup(level, id); entry != nil {
		return entry.Sequence()
	}
	return 0
}

// emitLSPChange emits an LSDB change on the IS-IS event bus (events.go) so the
// web UI / looking glass react without polling, AND (re-)triggers SPF. action is
// "add" | "refresh" | "purge". A nil bus makes the EMIT a no-op, but the SPF
// trigger still fires.
//
// This is the chokepoint for every LSDB change that affects the topology
// (origination, aging/purge, a NEWER received LSP), so it is also where SPF is
// (re-)triggered (isis-9): a debounced run recomputes the route set whenever the
// topology changes (spec Data Flow step 1, AC-9). The debounce coalesces a burst
// into one run per level. A change that does NOT alter the topology (an Equal
// received duplicate that only refreshed a held lifetime) must use
// publishLSPChange instead so it notifies consumers WITHOUT thrashing SPF.
func (e *engine) emitLSPChange(level, lspID string, sequence uint32, action string) {
	// Trigger SPF on every topology-changing LSDB change, independent of whether
	// an event bus is wired (route install must happen even with no web/event
	// consumers).
	e.triggerSPF()
	e.publishLSPChange(level, lspID, sequence, action)
}

// publishLSPChange emits the LSDB change on the event bus WITHOUT triggering SPF.
// It is used for a change that notifies consumers but does not alter the routing
// topology -- specifically an Equal received LSP that only refreshed the held
// Remaining Lifetime (ISO/IEC 10589 clause 7.3.16): the web UI still wants the
// "refresh", but SPF would recompute an identical route set, so a stream of
// duplicates must not thrash it. A nil bus makes this a no-op.
func (e *engine) publishLSPChange(level, lspID string, sequence uint32, action string) {
	bus := getEventBus()
	if bus == nil {
		return
	}
	if _, err := LSPChange.Emit(bus, &LSPChangeEvent{
		Level:    level,
		LSPID:    lspID,
		Sequence: sequence,
		Action:   action,
	}); err != nil {
		e.log.Debug("isis: lsp-change emit", "lsp-id", lspID, "err", err)
	}
}

// databaseSnapshot returns the `show isis database` view across both levels (the
// snapshot API consumed by isis-13). Each row carries the LSPID, sequence,
// lifetime, checksum, and overload flag (spec AC-10). L1 rows precede L2 rows;
// within a level the LSDB sorts by LSP ID.
func (e *engine) databaseSnapshot() []any {
	out := make([]any, 0)
	if e.lsdb == nil {
		return out
	}
	type lspRow struct {
		Level    string `json:"level"`
		LSPID    string `json:"lsp-id"`
		Sequence uint32 `json:"sequence"`
		Lifetime uint16 `json:"lifetime"`
		Checksum uint16 `json:"checksum"`
		Overload bool   `json:"overload"`
		Purged   bool   `json:"purged,omitempty"`
	}
	for _, lvl := range []lsdb.Level{lsdb.Level1, lsdb.Level2} {
		for _, row := range e.lsdb.Snapshot(lvl) {
			out = append(out, lspRow{
				Level:    lvl.String(),
				LSPID:    row.LSPID,
				Sequence: row.Sequence,
				Lifetime: row.Lifetime,
				Checksum: row.Checksum,
				Overload: row.Overload,
				Purged:   row.Purged,
			})
		}
	}
	return out
}

// databaseDetailSnapshot returns the `show isis database detail` view: each LSP
// summary row from databaseSnapshot, expanded with the decoded TLV list (type,
// length, lowercase-hex value). It is the detail counterpart of databaseSnapshot
// (spec-isis-13 AC-2): the summary shows freshness metadata, the detail expands
// the TLV contents so the operator can read what each LSP advertises. A
// malformed stored LSP keeps its summary row but carries no TLVs (one bad LSP
// does not blank the whole table; security review: error isolation).
func (e *engine) databaseDetailSnapshot() []any {
	out := make([]any, 0)
	if e.lsdb == nil {
		return out
	}
	type tlvRow struct {
		Type  uint8  `json:"type"`
		Len   int    `json:"len"`
		Value string `json:"value"`
	}
	type lspDetail struct {
		Level    string   `json:"level"`
		LSPID    string   `json:"lsp-id"`
		Sequence uint32   `json:"sequence"`
		Lifetime uint16   `json:"lifetime"`
		Checksum uint16   `json:"checksum"`
		Overload bool     `json:"overload"`
		Purged   bool     `json:"purged,omitempty"`
		TLVs     []tlvRow `json:"tlvs"`
	}
	var hexBuf textbuf.Buffer
	for _, lvl := range []lsdb.Level{lsdb.Level1, lsdb.Level2} {
		for _, row := range e.lsdb.Snapshot(lvl) {
			d := lspDetail{
				Level:    lvl.String(),
				LSPID:    row.LSPID,
				Sequence: row.Sequence,
				Lifetime: row.Lifetime,
				Checksum: row.Checksum,
				Overload: row.Overload,
				Purged:   row.Purged,
				TLVs:     []tlvRow{},
			}
			id, err := types.ParseLSPID(row.LSPID)
			if err == nil {
				if entry := e.lsdb.Lookup(lvl, id); entry != nil {
					if lsp, derr := entry.Decode(); derr == nil {
						for i := range lsp.TLVs {
							t := &lsp.TLVs[i]
							d.TLVs = append(d.TLVs, tlvRow{
								Type:  t.Type,
								Len:   len(t.Value),
								Value: hexBuf.Reset().Hex(t.Value).String(),
							})
						}
						packet.ReleaseTLVs(lsp.TLVs)
					}
				}
			}
			out = append(out, d)
		}
	}
	return out
}

// ---- small helpers ----

// originationLevels maps the node's configured level to the LSDB levels it
// originates own LSPs for.
func originationLevels(l Level) []lsdb.Level {
	switch l {
	case LevelL1:
		return []lsdb.Level{lsdb.Level1}
	case LevelL2:
		return []lsdb.Level{lsdb.Level2}
	default:
		return []lsdb.Level{lsdb.Level1, lsdb.Level2}
	}
}

// interfaceConfig returns the running InterfaceConfig for a circuit name.
func (e *engine) interfaceConfig(name string) (InterfaceConfig, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	ic, ok := e.running[name]
	return ic, ok
}

// levelMetric resolves the wide IS-reachability metric a circuit advertises at a
// level: the per-level override when set, else the circuit-wide metric (RFC 5305
// 24-bit; the YANG bounds it).
func levelMetric(ic InterfaceConfig, level lsdb.Level) uint32 {
	if level == lsdb.Level2 && ic.Level2.Metric > 0 {
		return ic.Level2.Metric
	}
	if level == lsdb.Level1 && ic.Level1.Metric > 0 {
		return ic.Level1.Metric
	}
	return ic.Metric
}

// configFormsLevel reports whether an interface configured at the given Level
// participates in an LSDB level (an l1-l2 circuit participates in both).
func configFormsLevel(cfgLevel Level, level lsdb.Level) bool {
	switch level {
	case lsdb.Level1:
		return cfgLevel.HasL1()
	case lsdb.Level2:
		return cfgLevel.HasL2()
	default:
		return false
	}
}

// anyIPv6Circuit reports whether any running circuit enables the IPv6 family, so
// TLV 129 advertises NLPID 0x8E (the data plane is isis-12).
func (e *engine) anyIPv6Circuit() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, ic := range e.running {
		if advertisesIPv6(ic) {
			return true
		}
	}
	return false
}

// minCircuitMTU returns the smallest MTU across running circuits (the safe LSP
// fragmentation bound, spec A-5), or 0 when none is known (the originator then
// uses its default).
func (e *engine) minCircuitMTU() int {
	e.circuitsMu.RLock()
	defer e.circuitsMu.RUnlock()
	min := 0
	for name := range e.circuitByName {
		if mtu, ok := e.transport.InterfaceMTU(name); ok && mtu > 0 {
			if min == 0 || mtu < min {
				min = mtu
			}
		}
	}
	return min
}
