// Design: docs/architecture/core-design.md — peer event handlers for route server
// Overview: server.go — route server plugin orchestration

package bgp_rs

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

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
	rs.peers[peerAddr].Up = (state == "up")
	rs.mu.Unlock()

	switch state {
	case "down":
		rs.handleStateDown(peerAddr)
	case "up":
		rs.handleStateUp(peerAddr)
	}
}

// handleStateDown processes peer session teardown.
// Sends withdrawals asynchronously — per-lifecycle goroutine (not hot path).
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

	go func() {
		for _, info := range entries {
			rs.updateRoute("!"+peerAddr, fmt.Sprintf("update text nlri %s del %s", info.Family, info.Prefix))
		}
	}()
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
	// Mark peer as replaying — excluded from selectForwardTargets until complete.
	// Increment generation so stale goroutines from a previous session (rapid
	// reconnect) don't prematurely clear Replaying for the new session.
	rs.mu.Lock()
	var gen uint64
	if rs.peers[peerAddr] != nil {
		rs.peers[peerAddr].Replaying = true
		rs.peers[peerAddr].ReplayGen++
		gen = rs.peers[peerAddr].ReplayGen
	}
	rs.mu.Unlock()

	// Spawn per-peer lifecycle goroutine for replay (not blocking event loop).
	go rs.replayForPeer(peerAddr, gen)
}

// replayForPeer runs the full+delta replay sequence for a newly-connected peer.
// Runs in a per-peer lifecycle goroutine — not blocking the event loop.
// The gen parameter is the replay generation at the time handleStateUp was called.
// If the peer's ReplayGen has changed (rapid reconnect), this goroutine is stale
// and must not clear Replaying — the newer goroutine owns that transition.
func (rs *RouteServer) replayForPeer(peerAddr string, gen uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Full replay from index 0.
	cmd := fmt.Sprintf("adj-rib-in replay %s 0", peerAddr)
	status, data, err := rs.dispatchCommand(ctx, cmd)
	if err != nil || status != statusDone {
		logger().Error("replay failed", "peer", peerAddr, "command", cmd, "status", status, "error", err)
		// Still add to forward targets so peer gets new routes going forward.
		// Only if this goroutine's generation is still current.
		rs.mu.Lock()
		if p := rs.peers[peerAddr]; p != nil && p.ReplayGen == gen {
			p.Replaying = false
		}
		rs.mu.Unlock()
		return
	}

	// Parse last-index from replay response.
	lastIndex, _ := parseReplayResponse(data)

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

	// Convergent delta replay: catch routes that adj-rib-in received after
	// the full replay snapshot. For internal plugins, events arrive via
	// DirectBridge on the engine's delivery goroutine while replay commands
	// arrive on Socket B — these are concurrent, so adj-rib-in may not have
	// stored recently-delivered routes when the full replay ran. Repeat until
	// no new routes appear (replayed==0), with a brief pause between attempts
	// to let adj-rib-in's event handler process pending deliveries.
	for i := range replayConvergenceMax {
		if lastIndex == 0 {
			break
		}
		if i > 0 {
			time.Sleep(replayConvergenceDelay)
		}
		deltaCmd := fmt.Sprintf("adj-rib-in replay %s %d", peerAddr, lastIndex)
		_, deltaData, deltaErr := rs.dispatchCommand(ctx, deltaCmd)
		if deltaErr != nil {
			logger().Warn("delta replay failed", "peer", peerAddr, "attempt", i, "error", deltaErr)
			break
		}
		newLast, replayed := parseReplayResponse(deltaData)
		if replayed == 0 {
			break
		}
		logger().Debug("delta replay caught new routes", "peer", peerAddr, "attempt", i, "replayed", replayed)
		lastIndex = newLast
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
		families = append(families, f)
	}
	rs.mu.RUnlock()

	// Sort for deterministic ordering in tests and logs.
	sort.Strings(families)

	for _, family := range families {
		rs.updateRoute(peerAddr, fmt.Sprintf("update text nlri %s eor", family))
	}
	logger().Info("sent EOR", "peer", peerAddr, "families", families)
}

// parseReplayResponse extracts last-index and replayed count from a replay response.
// Expected format: {"last-index":N,"replayed":M}.
func parseReplayResponse(data string) (lastIndex uint64, replayed int) {
	var resp struct {
		LastIndex uint64 `json:"last-index"`
		Replayed  int    `json:"replayed"`
	}
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		return 0, 0
	}
	return resp.LastIndex, resp.Replayed
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
		peer.Families = make(map[string]bool)

		// RFC 4760 Section 1: ipv4/unicast is the implicit default only when
		// the peer sends no Multiprotocol capability. If the peer advertises
		// MP but omits ipv4/unicast, it explicitly declines it.
		hasMP := capabilityPresent(event.Open.Capabilities, "multiprotocol")

		for _, cap := range event.Open.Capabilities {
			peer.Capabilities[cap.Name] = true

			if cap.Name == "multiprotocol" && cap.Value != "" {
				peer.Families[cap.Value] = true
			}
		}

		if !hasMP {
			peer.Families["ipv4/unicast"] = true
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
	family := event.AFI + "/" + event.SAFI

	if peerAddr == "" {
		return
	}

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
		if peer.Families != nil && !peer.SupportsFamily(family) {
			continue
		}
		targets = append(targets, addr)
	}
	rs.mu.RUnlock()

	// Send refreshes asynchronously — per-lifecycle goroutine (not hot path).
	go func() {
		for _, addr := range targets {
			rs.updateRoute(addr, "refresh "+family)
		}
	}()
}

// handleCommand processes command requests via SDK execute-command callback.
// Returns (status, data, error) for the SDK to send back to the engine.
func (rs *RouteServer) handleCommand(command string) (string, string, error) {
	switch command {
	case "rs status":
		return statusDone, `{"running":true}`, nil
	case "rs peers":
		return statusDone, rs.peersJSON(), nil
	default: // fail on unknown command
		return statusError, "", fmt.Errorf("unknown command: %s", command)
	}
}

// peersJSON returns peer state as JSON.
func (rs *RouteServer) peersJSON() string {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	peers := make([]map[string]any, 0, len(rs.peers))
	for _, p := range rs.peers {
		peers = append(peers, map[string]any{
			"address": p.Address,
			"asn":     p.ASN,
			"up":      p.Up,
		})
	}

	data, _ := json.Marshal(map[string]any{"peers": peers})
	return string(data)
}
