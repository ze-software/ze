package reactor

import (
	"context"
	"encoding/json"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/configop"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRFC8212ImportSwapFencesEstablishment proves establishment joins the
// admission boundary of a swap that removes the last accepting import policy
// of a peer that is not Established. Method: the swap's seam runs once the
// swap found the peer not Established and before it publishes; there a
// second goroutine establishes the session (enterEstablished, the call the
// FSM's Established transition makes) and then reads the import decision the
// first UPDATE would be admitted under. The seam waits for that goroutine up
// to a bound, so a missing fence is observed every run.
// VALIDATES: the session established inside the window admits under the
// rejecting policy only, and the epoch did not move (nothing was admitted).
// PREVENTS: a session that establishes between the decision and the publish
// admitting UPDATEs under the removed accepting policy, which no epoch then
// invalidates (RFC 8212 Section 3).
func TestRFC8212ImportSwapFencesEstablishment(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0", LocalAS: 65001})
	accept, err := filterapi.NewPolicy(nil, routepolicy.FinalAccept)
	require.NoError(t, err)
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.ImportPolicy = accept
	peer := NewPeer(settings)
	peer.reactor = r
	r.peers[settings.PeerKey()] = peer
	epoch := peer.sourcePolicyEpoch.Load()

	seen := make(chan bool, 1)
	importSwapGateHook = func(p *Peer) {
		done := make(chan struct{})
		go func() {
			defer close(done)
			p.enterEstablished()
			s := p.settingsSnapshot()
			seen <- s.ImportPolicy.AcceptsAtEnd(s.IsEBGP())
		}()
		select {
		case <-done:
		case <-time.After(200 * time.Millisecond):
		}
	}
	t.Cleanup(func() { importSwapGateHook = nil })

	next := peer.settingsSnapshot()
	next.ImportPolicy = filterapi.Policy{}
	copyImport := func(dst, src *PeerSettings) { dst.ImportPolicy = src.ImportPolicy }
	j := &rfc8212TxJournal{Journal: sdk.NewJournal(), txID: "tx-gate", deadline: time.Now().Add(30 * time.Second)}
	require.NoError(t, (&reactorAPIAdapter{r: r}).swapPeerSettingsJournaled([]peerSettingsSwap{{
		key: settings.PeerKey(), next: next, apply: copyImport}}, j))
	select {
	case accepts := <-seen:
		require.False(t, accepts, "the session established during the swap admits under the removed accepting policy")
	case <-time.After(5 * time.Second):
		t.Fatal("establishment never completed")
	}
	require.Equal(t, PeerStateEstablished, peer.State())
	require.Equal(t, epoch, peer.sourcePolicyEpoch.Load(), "epoch moved although nothing was admitted")
}

// TestRFC8212ImportPrepareRefusesReplacedSession proves prepare names only the
// session that holds its pause. Method: the prepare seam runs once the
// receive frontier settled and replaces the peer's session, as a reconnect
// between the pause and the identity would.
// VALIDATES: the change is refused naming the replacement, the paused
// session's receipt is released, no owner saw prepare, the policy is
// unchanged and the peer's claim is free for the retry.
// PREVENTS: a token naming the new session's generation while the old session
// holds the pause and the new one admits under the old policy.
func TestRFC8212ImportPrepareRefusesReplacedSession(t *testing.T) {
	fx := newRFC8212ImportFixture(t, false)
	successor := newAdmissionSession(t).s
	prepareSettledHook = func(p *Peer) {
		p.mu.Lock()
		p.session = successor
		p.mu.Unlock()
		p.forwardGeneration.Add(1)
	}
	t.Cleanup(func() { prepareSettledHook = nil })
	j := &rfc8212TxJournal{Journal: sdk.NewJournal(), txID: "tx-replaced", deadline: time.Now().Add(30 * time.Second)}
	require.ErrorContains(t, fx.swap(j), "session replaced while its input was paused")
	require.False(t, fx.f.s.IsPaused(), "the replaced session stays paused")
	fx.f.s.pauseMu.Lock()
	claims := fx.f.s.admissionClaims
	fx.f.s.pauseMu.Unlock()
	require.Zero(t, claims, "admission receipt not released")
	ops, _, eligible, _ := fx.owner.seen()
	require.Empty(t, ops, "an owner was prepared with the successor's identity")
	require.True(t, eligible)
	s := fx.peer.settingsSnapshot()
	require.True(t, s.ImportPolicy.AcceptsAtEnd(s.IsEBGP()), "policy published although prepare refused")
	require.NoError(t, fx.r.sourcePolicy.claim(fx.peer, "tx-retry"), "the refused transaction kept the peer's claim")
}

// TestRFC8212ImportFinalizeOutlivesApplyDeadline proves finalize is bounded by
// the completion receipt and the session's HoldTimer, not by the operation's
// apply deadline. Method: the operation's apply deadline is 1.5 seconds; the
// reload's acceptance arrives after it has passed, as a slow but healthy
// reload's does (commit, config write, listener migration, publication),
// with a config-complete deadline ten seconds out.
// VALIDATES: the owner finalizes, the receipt reports no error, and input
// resumes under the accepted rejecting policy.
// PREVENTS: "candidate is active but config finalization failed" after every
// reload slower than its apply budget, because owners refuse a past deadline.
func TestRFC8212ImportFinalizeOutlivesApplyDeadline(t *testing.T) {
	fx := newRFC8212ImportFixture(t, false)
	applyDeadline := time.Now().Add(1500 * time.Millisecond)
	j := &rfc8212TxJournal{Journal: sdk.NewJournal(), txID: "tx-slow", deadline: applyDeadline}
	require.NoError(t, fx.swap(j))
	require.NoError(t, fx.r.CheckSourcePolicyTx(j.txID))
	time.Sleep(time.Until(applyDeadline) + 100*time.Millisecond)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	require.NoError(t, fx.r.CompleteSourcePolicyTx(ctx, j.txID, true))
	j.Discard()
	ops, _, eligible, _ := fx.owner.seen()
	require.Equal(t, []rpc.SourcePolicyOperation{rpc.SourcePolicyPrepare, rpc.SourcePolicyApply, rpc.SourcePolicyFinalize}, ops)
	require.False(t, eligible)
	fx.settled(t, false)
}

// TestRFC8212ImportUnresolvedCandidateRefused proves a candidate that could
// only be built from an operation's config leaves is never read as removing
// the peer's accepting import policy. Method: a reactor with no config path
// and no reload function resolves a modify-peer carrying the peer's own body,
// as an import-layer operation does: the leaves carry no filter chain. That
// candidate is then swapped in place with the marker the resolution reported.
// VALIDATES: the resolution reports the policy unresolved; the swap is
// refused naming the unknown effective policy, and the running accepting
// policy stays.
// PREVENTS: a false D-2 that pauses the session and publishes an empty
// (rejecting) policy the configuration never asked for.
func TestRFC8212ImportUnresolvedCandidateRefused(t *testing.T) {
	r := New(&Config{})
	accept, err := filterapi.NewPolicy(nil, routepolicy.FinalAccept)
	require.NoError(t, err)
	settings := NewPeerSettings(mustParseAddr("203.0.113.1"), 65000, 65001, 0)
	settings.Name = "edge"
	settings.LocalAddress = mustParseAddr("192.0.2.1")
	settings.ImportPolicy = accept
	require.NoError(t, r.AddPeer(settings))
	body := json.RawMessage(`{"connection":{"remote":{"ip":"203.0.113.1"},"local":{"ip":"192.0.2.1"}},"session":{"asn":{"local":"65000","remote":"65001"}}}`)
	op := rpc.ConfigOperation{ID: "bgp-modify-peer-edge", Root: "bgp", Owner: "bgp", Type: configop.ModifyPeer,
		Target: rpc.ResourceRef{Kind: rpc.ResourcePeer, Peer: "edge"},
		Params: rpc.ConfigOperationParams{Peer: "edge", Address: "192.0.2.1", OldConfig: body, Config: body}}
	adapter := &reactorAPIAdapter{r: r}
	next, resolved, err := adapter.resolveOperationCandidate("", &op)
	require.NoError(t, err)
	require.False(t, resolved, "a candidate built from config leaves reported as carrying the effective policy")

	copyImport := func(dst, src *PeerSettings) { dst.ImportPolicy = src.ImportPolicy }
	j := &rfc8212TxJournal{Journal: sdk.NewJournal(), txID: "tx-unresolved", deadline: time.Now().Add(30 * time.Second)}
	err = adapter.swapPeerSettingsJournaled([]peerSettingsSwap{{key: settings.PeerKey(), next: next, apply: copyImport,
		policyUnresolved: !resolved}}, j)
	require.ErrorContains(t, err, "effective import policy is unknown")
	s := r.Peers()[0].settingsSnapshot()
	require.True(t, s.ImportPolicy.AcceptsAtEnd(s.IsEBGP()), "accepting import policy replaced by a guess")
}
