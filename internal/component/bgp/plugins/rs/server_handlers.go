// Design: docs/architecture/core-design.md — peer event handlers for route server
// RFC: rfc/short/rfc4724.md — End-of-RIB marker, sent per negotiated family once peer-up replay terminates (sendEOR)
// RFC: rfc/short/rfc4760.md — Section 1: ipv4/unicast is implicit only when the peer advertises no MP capability (handleOpen)
// Overview: server.go — route server plugin orchestration

package rs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// errUnknownCommandMarker is the engine's ErrUnknownCommand text ("unknown
// command"), surfaced here as a string sentinel because errors cross the
// plugin IPC boundary as serialized strings and lose their wrap chain --
// errors.Is cannot reach through to the sentinel declared in
// internal/component/plugin/server/command.go. If that text ever changes, the
// soft-dep fallback silently reverts to ERROR logging; the
// TestRSSoftDepSkipsReplay unit test uses the same marker and would start
// failing, which is the detection signal.
const errUnknownCommandMarker = "unknown command"

// cmdAdjRIBInReplay is the dispatch-command that asks bgp-adj-rib-in to replay a
// peer's stored Adj-RIB-In. Args carry the peer address and cursor index.
const cmdAdjRIBInReplay = "request bgp adj-rib-in replay"

// ClaimPeerUpReplay is the exclusive-role token this plugin declares in its
// registration (registry.Registration.Claims, see register.go). The engine
// unions the claims of every plugin in the startup set and delivers the result
// on the Stage-2 configure callback, so bgp-adj-rib-in learns that peer-up
// replay is owned BEFORE it sends Stage 5 ready -- i.e. before the engine starts
// peers, with no timing window against session establishment.
//
// This is the ordering that matters, because the claim does NOT gate the live
// forward. A peer marked Replaying IS still a selectForwardTargets destination
// (server_forward.go selects on peer.Up alone; TestReplayingPeerIncludedInForwardTargets
// pins that, and plan/learned/630-rs-fastpath-3-passthrough.md records the
// decision -- BGP UPDATE duplicates are idempotent and excluding replaying peers
// loses routes). So the claim landing before a peer establishes is the ONLY
// thing standing between that peer and a doubled replay.
//
// bgp-adj-rib-in reads this same spelling; the two plugins agree on the token,
// the engine treats it as opaque.
const ClaimPeerUpReplay = "bgp-peer-up-replay"

// cmdAdjRIBInClaimReplay takes ownership of peer-up replay from bgp-adj-rib-in
// for a bgp-adj-rib-in that joined AFTER this plugin was configured -- a
// mid-life auto-load or respawn, where the Stage-2 declaration above could not
// have reached it. It is a late-join corrective, no longer the startup path.
//
// It must never be relied on for startup ordering: it is dispatched from
// OnAllPluginsReady, which plugin/server/startup.go sendPostStartupToAll fans
// out on detached goroutines immediately before starting peers. Measured margin
// on an idle darwin host when it WAS the startup path: 1-2 ms, 6/6 runs of
// test/plugin/rfc7606-relay-one-field.
const cmdAdjRIBInClaimReplay = "request bgp adj-rib-in claim-replay"

// now reports the current time from this plugin's injected clock.
//
// A nil clk means real time, so a RouteServer built as a struct literal (the
// test helpers do) is usable without wiring one. Kept as a helper rather than a
// clock.Clock accessor so the default path costs no interface dispatch.
func (rs *RouteServer) now() time.Time {
	if rs.clk == nil {
		return time.Now()
	}
	return rs.clk.Now()
}

// sleep pauses using this plugin's injected clock. See now for the nil case.
//
// Under a fake clock this returns immediately and time advances only where the
// test says it does, which is what makes replayForPeer's catch-up budget
// testable without a real wait.
func (rs *RouteServer) sleep(d time.Duration) {
	if rs.clk == nil {
		time.Sleep(d)
		return
	}
	rs.clk.Sleep(d)
}

// isDispatchUnknownCommand reports whether err from rs.dispatchCommand comes
// from the engine's ErrUnknownCommand -- i.e. no plugin handled the command.
// Used as the soft-dep "adj-rib-in is not loaded" signal in replayForPeer.
// Caller is responsible for knowing that the only command rs dispatches is
// "request bgp adj-rib-in replay ..."; this helper does not re-verify the command text.
func isDispatchUnknownCommand(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), errUnknownCommandMarker)
}

// handleState processes peer state changes.
// ze-bgp JSON: {"type":"bgp","bgp":{"message":{"type":"state"},"peer":{...},"state":"up"}}.
func (rs *RouteServer) handleState(event *Event) {
	peerAddr := event.PeerAddr
	state := event.State

	if peerAddr == "" {
		return
	}

	rs.mu.Lock()
	if rs.peers[peerAddr] == nil {
		rs.peers[peerAddr] = &PeerState{Address: peerAddr}
	}
	up := state == "up"
	rs.peers[peerAddr].Up = up
	// From here on `!Up` means DOWN for this peer, never "not yet" -- see
	// PeerState.StateSeen and processForward's guard.
	rs.peers[peerAddr].StateSeen = true
	cut := rs.seenMsgID
	if up {
		// THE CUT, captured in the same critical section that makes this peer a
		// live forward target. Both facts are written under one rs.mu.Lock, so no
		// forward-target selection can ever observe Up without the matching
		// ForwardFrom, and none can observe a stale ForwardFrom from a previous
		// session. selectForwardTargets reads both under rs.mu.RLock.
		rs.peers[peerAddr].ForwardFrom = rs.seenMsgID
	}
	rs.mu.Unlock()

	// Logged outside the critical section: this is the peer-up path, and the same
	// lock gates every forward-target selection.
	logger().Debug("peer state applied", "peer", peerAddr, "state", state, "cut", cut)

	switch state {
	case "down":
		rs.handleStateDown(peerAddr)
	case "up":
		rs.handleStateUp(peerAddr)
	}
}

// withdrawalBatchSize is the number of prefixes per batched withdrawal RPC.
// Batching reduces per-RPC overhead (tokenize, JSON, command registry) from
// 100k individual calls to ~200 batched calls for a typical 100k-route teardown.
// Sized to keep each text command under ~50KB while providing 500x fewer RPCs.
const withdrawalBatchSize = 500

// handleStateDown processes peer session teardown.
// Sends withdrawals asynchronously -- per-lifecycle goroutine (not hot path).
// Batches withdrawal RPCs by family to reduce GC pressure from text-RPC overhead.
func (rs *RouteServer) handleStateDown(peerAddr string) {
	// Drain workers first: in-flight forwards may update the withdrawal map.
	// PeerDown waits for all workers to finish, so after this call no more
	// updates for this peer can occur.
	rs.workers.PeerDown(peerAddr)

	// Extract and clear withdrawal entries for this peer.
	rs.withdrawalMu.Lock()
	entries := rs.withdrawals[peerAddr]
	delete(rs.withdrawals, peerAddr)
	rs.withdrawalMu.Unlock()

	go rs.sendBatchedWithdrawals(peerAddr, entries)
}

// sendBatchedWithdrawals groups withdrawal entries by family and sends them
// in batched text RPCs. Each batch packs up to withdrawalBatchSize prefixes
// into a single "update text nlri <family> del <prefix1> del <prefix2> ..."
// command, reducing the number of RPC roundtrips by ~500x compared to
// one-prefix-per-RPC.
func (rs *RouteServer) sendBatchedWithdrawals(peerAddr string, entries map[withdrawalKey]struct{}) {
	if len(entries) == 0 {
		return
	}

	// Group by family for batched commands. String conversion on cold path only.
	byFamily := make(map[string][]string)
	for wk := range entries {
		famStr := wk.fam.String()
		if wk.nlriStr != "" {
			byFamily[famStr] = append(byFamily[famStr], wk.nlriStr)
		} else {
			byFamily[famStr] = append(byFamily[famStr], "prefix "+wk.prefix.String())
		}
	}

	addr, err := netip.ParseAddr(peerAddr)
	if err != nil {
		logger().Error("invalid peer address in withdrawal", "peer", peerAddr, "error", err)
		return
	}
	excludeSel := selector.ExcludeAddr(addr)
	var buf strings.Builder

	for fam, prefixes := range byFamily {
		for i := 0; i < len(prefixes); i += withdrawalBatchSize {
			end := min(i+withdrawalBatchSize, len(prefixes))
			batch := prefixes[i:end]

			buf.Reset()
			buf.WriteString("update text nlri ")
			buf.WriteString(fam)
			for _, p := range batch {
				buf.WriteString(" del ")
				buf.WriteString(p)
			}
			rs.updateRouteSel(excludeSel, buf.String())
		}
	}
}

// claimReplayOwnership re-affirms to bgp-adj-rib-in that this plugin drives
// peer-up replay, for a bgp-adj-rib-in that was not present when this plugin's
// claim was declared.
//
// It is a BACKSTOP, not the ordering mechanism. Startup ordering comes from the
// declaration in register.go (Registration.Claims = ClaimPeerUpReplay): the
// engine delivers that on bgp-adj-rib-in's Stage-2 configure callback, which
// completes before it sends Stage 5 ready and therefore before peers start.
// This dispatch covers only the mid-life case the declaration cannot reach --
// a bgp-adj-rib-in auto-loaded by a config reload, or respawned, after this
// plugin was already configured.
//
// Called from OnAllPluginsReady, which guarantees the dispatcher's command
// registry is frozen so the dispatch resolves. That callback is NOT ordered
// against session establishment -- sendPostStartupToAll fans out on detached
// goroutines and signalStartupComplete then starts peers
// (internal/component/plugin/server/startup.go), and waiting there deadlocks.
// So nothing that must hold for the first peer may depend on this call; the
// declaration is what covers that peer. Standing down is idempotent, so the two
// paths overlapping is harmless.
//
// A missing adj-rib-in is not an error -- it is an OptionalDependency, and with
// it absent there is no self-replay to stand down.
func (rs *RouteServer) claimReplayOwnership() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	status, _, err := rs.dispatchCommand(ctx, cmdAdjRIBInClaimReplay)
	switch {
	case isDispatchUnknownCommand(err):
		// adj-rib-in not loaded; nothing replays but this plugin anyway.
	case err != nil || status != statusDone:
		// Not fatal, but the duplicate-announce this claim prevents is now
		// possible, so say so rather than fail silently.
		logger().Warn("could not claim adj-rib-in replay ownership; peer-up routes may be announced twice",
			"status", status, "error", err)
	}
}

// handleStateUp processes peer session establishment.
//
// Replays existing routes to the newly-connected peer via DispatchCommand
// to bgp-adj-rib-in, replacing the previous ROUTE-REFRESH approach which
// caused a thundering herd (N peers × M families). The replay runs in a
// per-peer lifecycle goroutine (not blocking the event loop).
// A convergent delta replay loop then covers routes that adj-rib-in may not
// have stored yet at full-replay time (race between event delivery and replay).
func (rs *RouteServer) handleStateUp(peerAddr string) {
	// Mark peer as replaying. This does NOT withhold it from selectForwardTargets
	// (see cmdAdjRIBInClaimReplay above) -- the flag exists only so the replay
	// goroutine's generation bookkeeping can tell sessions apart. Increment the
	// generation so stale goroutines from a previous session (rapid reconnect)
	// don't prematurely clear Replaying for the new session.
	rs.mu.Lock()
	var gen, cut uint64
	if rs.peers[peerAddr] != nil {
		rs.peers[peerAddr].Replaying = true
		rs.peers[peerAddr].ReplayGen++
		gen = rs.peers[peerAddr].ReplayGen
		// Read back the cut handleState committed for THIS session rather than
		// re-reading rs.seenMsgID. Today the two are equal -- handleState calls
		// this on the same event-loop goroutine, so no UPDATE can be taken
		// delivery of in between -- but the replay must be bounded by the value
		// the forward rail is actually filtering on. Re-deriving it would couple
		// the two to that call-site detail, and if this ever ran off the event
		// loop the replay would stop short of where the live rail starts and drop
		// whatever fell in the gap.
		cut = rs.peers[peerAddr].ForwardFrom
	}
	rs.mu.Unlock()

	// Spawn per-peer lifecycle goroutine for replay (not blocking event loop).
	go rs.replayForPeer(peerAddr, gen, cut)
}

// replayForPeer runs the full+delta replay sequence for a newly-connected peer.
// Runs in a per-peer lifecycle goroutine — not blocking the event loop.
// The gen parameter is the replay generation at the time handleStateUp was called.
// If the peer's ReplayGen has changed (rapid reconnect), this goroutine is stale
// and must not clear Replaying — the newer goroutine owns that transition.
func (rs *RouteServer) replayForPeer(peerAddr string, gen, cut uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Full replay from index 0.
	replayCommand := cmdAdjRIBInReplay
	// "<peer> <from-index> <max-msg-id>": resume from the start, stop at the cut.
	cutArg := textbuf.StringUint(cut)
	replayArgs := []string{peerAddr, "0", cutArg}
	status, data, err := rs.dispatchCommand(ctx, replayCommand, replayArgs...)
	if err != nil || status != statusDone {
		// Graceful soft-dep fallback: when bgp-adj-rib-in is an
		// OptionalDependencies entry that was not registered at build time,
		// DispatchCommand returns ErrUnknownCommand ("unknown command ..."). Log
		// one WARN per RouteServer instance (sync.Once) and skip the replay
		// loop instead of spamming ERROR for every peer-up. Other replay
		// failures (IPC timeout, engine error) get an ERROR log so they stay
		// visible.
		if isDispatchUnknownCommand(err) {
			rs.adjRibInMissingOnce.Do(func() {
				logger().Warn("bgp-adj-rib-in not loaded; replay-on-peer-up disabled (optional dependency)")
			})
		} else {
			logger().Error("replay failed", "peer", peerAddr, "command", replayCommand, "args", replayArgs, "status", status, "error", err)
		}
		// Clear Replaying so the peer is included in forward targets for any
		// UPDATE arriving from now on. Only if this goroutine's generation is
		// still current (a rapid reconnect spawns a newer goroutine that owns
		// the transition).
		rs.mu.Lock()
		if p := rs.peers[peerAddr]; p != nil && p.ReplayGen == gen {
			p.Replaying = false
		}
		rs.mu.Unlock()
		// Always send EOR when replay terminates -- the peer needs per-family
		// end-of-RIB markers to finish its initial sync regardless of whether
		// the replay itself succeeded, failed transiently, or was skipped
		// because adj-rib-in is not loaded. Without this, any replay-failure
		// path left the peer waiting for EOR that would never come.
		rs.sendEOR(peerAddr, gen)
		return
	}

	// Parse last-index and adj-rib-in's ingest position from the replay response.
	progress := parseReplayProgress(data)

	// Add peer to forward targets (new UPDATEs now flow to this peer).
	// Only if this goroutine's generation is still current.
	rs.mu.Lock()
	stale := rs.peers[peerAddr] == nil || rs.peers[peerAddr].ReplayGen != gen
	if !stale {
		rs.peers[peerAddr].Replaying = false
	}
	rs.mu.Unlock()

	if stale {
		return
	}

	// Two convergence phases, because there are two different things to wait for
	// and only one of them used to be waited on at all. Each plugin has its OWN
	// delivery goroutine and event queue (plugin/process/delivery.go: one
	// eventChan per Process), so adj-rib-in routinely trails this plugin by
	// several UPDATEs, and replay commands arrive on a different path again
	// (MuxConn).
	//
	// PHASE 1, catch up to the cut. The cut suppressed every UPDATE at or below
	// it on the live rail (server_forward.go selectForwardTargets) on the promise
	// that this replay carries them, and adj-rib-in can only carry them once it
	// has consumed the stream that far. Nothing here can be inferred from "did
	// that call return routes": an empty answer means either "level with the cut,
	// genuinely nothing left" or "not there yet", and the old loop read both as
	// done -- in fact it skipped the loop ENTIRELY when the full replay came back
	// empty (last-index 0), which is exactly the state "adj-rib-in has ingested
	// nothing yet". Then neither rail delivered those routes and they were lost.
	// test/plugin/forward-overflow-two-tier.ci catches it as an UPDATE dropped
	// with why="below-cut(N)" and the destination peer missing 10.0.0.0/24.
	//
	// PHASE 2, drain. Once adj-rib-in is level with the cut, an empty answer is
	// unambiguous, so the original "stop when a call adds nothing" is sound and
	// unchanged.
	deadline := rs.now().Add(replayCatchUpBudget)
	for !progress.coversCut(cut) {
		if rs.now().After(deadline) {
			// Say it rather than let the peer look complete: past this point the
			// cut is knowingly suppressing UPDATEs the replay could not cover
			// (ai/rules/evidence.md).
			logger().Warn("peer-up replay gave up waiting for adj-rib-in to reach the cut; routes at or below it may be missing",
				"peer", peerAddr, "cut", cut, "ingested", progress.ingestedString(), "budget", replayCatchUpBudget)
			break
		}
		rs.sleep(replayConvergenceDelay)
		next, deltaErr := rs.deltaReplay(ctx, peerAddr, cutArg, progress)
		if deltaErr != nil {
			logger().Warn("catch-up replay failed", "peer", peerAddr, "error", deltaErr)
			break
		}
		progress = next
	}

	for i := range replayConvergenceMax {
		if progress.replayed == 0 {
			break
		}
		if i > 0 {
			rs.sleep(replayConvergenceDelay)
		}
		next, deltaErr := rs.deltaReplay(ctx, peerAddr, cutArg, progress)
		if deltaErr != nil {
			logger().Warn("delta replay failed", "peer", peerAddr, "attempt", i, "error", deltaErr)
			break
		}
		progress = next
		if progress.replayed > 0 {
			logger().Debug("delta replay caught new routes", "peer", peerAddr, "attempt", i, "replayed", progress.replayed)
		}
	}

	// Send End-of-RIB per negotiated family (RFC 4271).
	// Re-check generation: peer may have reconnected during the delta loop.
	rs.sendEOR(peerAddr, gen)
}

// sendEOR sends End-of-RIB markers for each of the peer's negotiated families.
// Checks generation to avoid sending EOR from a stale replay goroutine.
func (rs *RouteServer) sendEOR(peerAddr string, gen uint64) {
	rs.mu.RLock()
	p := rs.peers[peerAddr]
	if p == nil || p.ReplayGen != gen || len(p.Families) == 0 {
		rs.mu.RUnlock()
		return
	}
	families := make([]string, 0, len(p.Families))
	for f := range p.Families {
		families = append(families, f.String())
	}
	rs.mu.RUnlock()

	// Sort for deterministic ordering in tests and logs.
	sort.Strings(families)

	for _, fam := range families {
		rs.updateRoute(peerAddr, "update text nlri "+fam+" eor")
	}
	logger().Info("sent EOR", "peer", peerAddr, "families", families)
}

// replayProgress is what one replay call reports back about the replay's state.
type replayProgress struct {
	// cursor is the highest sequence index DELIVERED so far ("last-index"), the
	// resume point for the next call.
	cursor uint64
	// replayed is how many routes the most recent call delivered.
	replayed int
	// ingested is adj-rib-in's ingest position ("ingested-msg-id"), or nil when
	// it tracks none.
	ingested *uint64
}

// coversCut reports whether adj-rib-in has consumed the UPDATE stream far enough
// for a replay bounded at cut to be ABLE to carry every route the live rail
// suppressed on the promise that this replay would.
//
// No signal (ingested == nil) reads as covered. That is not a fail-open: it means
// adj-rib-in stores no MessageIDs, so replayCut.excludes (adj_rib_in/replay_cut.go)
// never excludes anything and its replay is already unbounded. There is nothing to
// wait for, and waiting would tax every peer-up for it.
func (p replayProgress) coversCut(cut uint64) bool {
	return p.ingested == nil || *p.ingested >= cut
}

// ingestedString renders the ingest position for a log line, printing the VALUE
// rather than the pointer and naming the absent case explicitly. Cold path (the
// give-up branch only), so the allocation is not on any hot path.
func (p replayProgress) ingestedString() string {
	if p.ingested == nil {
		return "none"
	}
	return textbuf.StringUint(*p.ingested)
}

// parseReplayProgress decodes one replay call's answer, including the
// responder's ingest position.
//
// Format: {"last-index":N,"replayed":M[,"ingested-msg-id":P]}. The third field is
// a POINTER so its absence is distinguishable from a value of 0 -- adj-rib-in
// omits it when it tracks no position (a forked plugin on the text path, whose
// stored routes carry no MessageID and are therefore never cut-excluded), while 0
// is the real position of a plugin that has ingested nothing yet. Conflating the
// two is what this whole signal exists to avoid.
func parseReplayProgress(data json.RawMessage) replayProgress {
	var resp struct {
		LastIndex uint64  `json:"last-index"`
		Replayed  int     `json:"replayed"`
		Ingested  *uint64 `json:"ingested-msg-id"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return replayProgress{}
	}
	return replayProgress{cursor: resp.LastIndex, replayed: resp.Replayed, ingested: resp.Ingested}
}

// deltaReplay asks adj-rib-in for whatever it has stored since prev.cursor,
// still bounded by the same cut, and folds the answer into a new progress.
//
// The cursor advances only on a call that delivered something: "last-index" is
// the highest sequence DELIVERED, so an empty call reports 0, and adopting that
// as the cursor would rewind the next call to the start of the store.
func (rs *RouteServer) deltaReplay(ctx context.Context, peerAddr, cutArg string, prev replayProgress) (replayProgress, error) {
	args := []string{peerAddr, textbuf.StringUint(prev.cursor), cutArg}
	_, data, err := rs.dispatchCommand(ctx, cmdAdjRIBInReplay, args...)
	if err != nil {
		return prev, err
	}
	next := parseReplayProgress(data)
	if next.replayed == 0 {
		next.cursor = prev.cursor
	}
	return next, nil
}

// handleOpen processes OPEN events to capture peer capabilities.
// Text format capabilities: "cap <code> <name> [<value>]" tokens parsed by parseTextOpen.
func (rs *RouteServer) handleOpen(event *Event) {
	peerAddr := event.PeerAddr
	if peerAddr == "" {
		return
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.peers[peerAddr] == nil {
		rs.peers[peerAddr] = &PeerState{Address: peerAddr}
	}
	peer := rs.peers[peerAddr]

	peer.ASN = event.PeerASN

	if event.Open != nil {
		peer.Capabilities = make(map[string]bool)
		peer.Families = make(map[family.Family]bool)

		// RFC 4760 Section 1: ipv4/unicast is the implicit default only when
		// the peer sends no Multiprotocol capability. If the peer advertises
		// MP but omits ipv4/unicast, it explicitly declines it.
		hasMP := capabilityPresent(event.Open.Capabilities, 1) // RFC 4760: multiprotocol

		for _, cap := range event.Open.Capabilities {
			peer.Capabilities[cap.Name] = true

			if cap.Name == "multiprotocol" && cap.Value != "" {
				if fam, ok := family.LookupFamily(cap.Value); ok {
					peer.Families[fam] = true
				}
			}
		}

		if !hasMP {
			peer.Families[family.IPv4Unicast] = true
		}
	}
}

// handleRefresh processes route refresh requests.
// Text format: "peer <addr> <dir> refresh <id> family <afi/safi>" parsed by parseTextRefresh.
//
// Collects eligible peers under the lock, then sends refresh commands after
// releasing — updateRoute does an SDK RPC with a 10 s timeout, so holding
// the lock during network I/O would block all state updates.
func (rs *RouteServer) handleRefresh(event *Event) {
	peerAddr := event.PeerAddr
	if peerAddr == "" {
		return
	}
	fam := family.Family{AFI: event.AFI, SAFI: event.SAFI}

	rs.mu.RLock()
	var targets []string
	for addr, peer := range rs.peers {
		if addr == peerAddr {
			continue
		}
		if !peer.Up {
			continue
		}
		if !peer.HasCapability("route-refresh") {
			continue
		}
		if peer.Families != nil && !peer.SupportsFamily(fam) {
			continue
		}
		targets = append(targets, addr)
	}
	rs.mu.RUnlock()

	// Send refreshes asynchronously — per-lifecycle goroutine (not hot path).
	famStr := fam.String()
	go func() {
		for _, addr := range targets {
			rs.peerAction(addr, "refresh "+famStr)
		}
	}()
}

// handleCommand processes command requests via SDK execute-command callback.
// Returns (status, data, error) for the SDK to send back to the engine.
func (rs *RouteServer) handleCommand(command string) (string, any, error) {
	switch command {
	case "show bgp rs status":
		return statusDone, map[string]any{"running": true}, nil
	case "show bgp rs peers":
		return statusDone, rs.peerStatus(), nil
	default:
		return statusError, "", fmt.Errorf("unknown command: %s", command)
	}
}

// peerStatus returns peer state.
func (rs *RouteServer) peerStatus() any {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	peers := make([]map[string]any, 0, len(rs.peers))
	for _, p := range rs.peers {
		peers = append(peers, map[string]any{
			"address": p.Address,
			"remote":  map[string]any{"as": p.ASN},
			"up":      p.Up,
		})
	}

	return map[string]any{"peers": peers}
}
