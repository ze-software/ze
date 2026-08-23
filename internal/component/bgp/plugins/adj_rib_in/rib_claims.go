// Design: docs/architecture/api/process-protocol.md — Stage 2 configure, exclusive-role claims
// Overview: rib.go — AdjRIBInManager, replayOwned, and the peer-up self-replay this stands down
// Related: rib_commands.go — claimReplayCommand, the late-join corrective for the same flag

package adj_rib_in

import "slices"

// claimPeerUpReplay is the exclusive-role token bgp-rs declares to take peer-up
// replay over from this plugin (rs/server_handlers.go ClaimPeerUpReplay declares
// the same spelling; rs/register.go puts it in Registration.Claims). The engine
// treats the token as opaque and delivers it on the Stage-2 configure callback.
//
// Spelled out here rather than imported from the rs package on purpose: this
// plugin must build and run with bgp-rs deleted from the tree
// (ai/rules/plugins.md). An absent bgp-rs simply never claims
// the token, and self-replay stays on.
const claimPeerUpReplay = "bgp-peer-up-replay"

// applyStartupClaims stands peer-up self-replay down when another plugin has
// claimed that role, using the claim set the engine delivers on the Stage-2
// configure callback.
//
// Ordering is the entire point. Stage 2 is part of the sequential startup
// handshake: it returns before this plugin sends Stage 5 ready, which is before
// its startup phase completes, which is before the engine calls
// SignalPluginStartupComplete -> StartPeers. So the decision is in place before
// any session can establish and before any state event can arrive here. The
// previous design took it from bgp-rs's OnAllPluginsReady dispatch, which the
// engine fans out on detached goroutines immediately before starting peers
// (internal/component/plugin/server/startup.go, sendPostStartupToAll) -- a
// 1-2 ms window on an idle host that inverted under suite load and produced a
// duplicate announce to the first peer.
//
// Never latches OFF: the claim is monotonic within a process lifetime, and the
// late-join corrective (claimReplayCommand) may have set it already.
//
// claimActive is sdk.Plugin.ClaimActive, taken as a function value so the
// stand-down decision is drivable from a unit test without a live handshake. A
// nil claimActive means the caller has no claim source at all, which resolves to
// "not claimed" -- the fail-closed direction, because self-replay at worst
// duplicates an idempotent UPDATE while standing down for an absent owner loses
// routes (ai/rules/evidence.md).
func (r *AdjRIBInManager) applyStartupClaims(claimActive func(role string) bool) {
	if claimActive == nil || !claimActive(claimPeerUpReplay) {
		return
	}
	if !r.replayOwned.Swap(true) {
		logger().Info("peer-up replay ownership declared by another plugin at startup; self-replay disabled",
			"role", claimPeerUpReplay)
	}
}

// replayDrivenElsewhere reports whether another plugin drives peer-up replay for
// the peer whose state event carries unheldRoles.
//
// Two facts decide it and the engine states both, because neither is visible
// from inside this plugin.
//
// The CLAIM says a role has an owner in this daemon. It arrives at Stage 2,
// before any session can establish, which is what makes the first peer-up safe
// (applyStartupClaims above).
//
// unheldRoles says that owner takes no delivery of THIS peer's events, so the
// claim does not reach it (pluginserver.Server.UnheldRoles). Delivery is
// per-peer and a claim is not. Take a peer whose `attach process` block gives
// `state` to this plugin and not to bgp-rs. bgp-rs cannot replay it, and it
// never becomes a forward target either: selectForwardTargets keys on peer.Up
// (rs/server_forward.go), which only handleState sets. Standing down there
// serves that peer to nobody.
//
// An absent statement means the claim stands. That is the direction the engine
// speaks in: it retracts a promise it made, and says nothing when the promise
// holds.
func (r *AdjRIBInManager) replayDrivenElsewhere(unheldRoles []string) bool {
	if !r.replayOwned.Load() {
		return false
	}
	return !slices.Contains(unheldRoles, claimPeerUpReplay)
}
