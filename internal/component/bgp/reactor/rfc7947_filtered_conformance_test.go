// Design: docs/architecture/plugin/rib-storage-design.md -- RFC 7947 Section 2.1 pre-policy Adj-RIB-In retention
// RFC: rfc/short/rfc7947.md -- Section 2.1, route server Adj-RIB-In accepts all client UPDATEs
// Related: reactor_notify.go -- enqueueFilteredIngress and RSClient admission gate
// Related: session_prefix.go -- unique retained prefix bound for rs-client peers
package reactor

import (
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/config"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC7947ClientDeniedUpdateRetainedInAdjRIBIn proves that a route server
// accepts UPDATE messages received from an rs-client peer into its Adj-RIB-In
// even when ingress loop detection or import policy denies the route, marking
// the stored entry import-filtered and recording the denying step name.
//
// RFC requirement: RFC7947-2.1-1 positive -- a route server MUST accept all
// UPDATE messages received from each of its clients for inclusion in its
// Adj-RIB-In, even when omitted from Loc-RIB due to routing policy.
// Producer: internal/component/bgp/reactor/reactor_notify.go::enqueueFilteredIngress
func TestRFC7947ClientDeniedUpdateRetainedInAdjRIBIn(t *testing.T) {
	t.Run("loop-denial", func(t *testing.T) {
		f := newFilteredReceivedLive(t, true, mustPolicy(t, nil, routepolicy.FinalAccept))
		prefix := netip.MustParsePrefix("203.0.114.0/24")
		attrs := firstASAttrs(4, 65002, 65000)
		f.send(t, f.source, &message.Update{
			PathAttributes: attrs,
			NLRI:           ownershipLiveNLRI([]netip.Prefix{prefix}),
		})

		var answer struct {
			Peers map[string][]struct {
				Family     string `json:"family"`
				AttrHex    string `json:"attr-hex"`
				NLRIHex    string `json:"nlri-hex"`
				Filtered   *bool  `json:"filtered"`
				FilterStep string `json:"filter-step"`
			} `json:"adj-rib-in"`
		}
		require.NoError(t, json.Unmarshal(f.command(t, "show bgp adj-rib-in"), &answer))
		rows := answer.Peers[f.source.peer.Settings().Address.String()]
		require.Len(t, rows, 1)
		assert.Equal(t, "ipv4/unicast", rows[0].Family)
		assert.Equal(t, hex.EncodeToString(attrs), rows[0].AttrHex)
		assert.Equal(t, hex.EncodeToString(ownershipLiveNLRI([]netip.Prefix{prefix})), rows[0].NLRIHex)
		require.NotNil(t, rows[0].Filtered)
		assert.True(t, *rows[0].Filtered)
		assert.NotEmpty(t, rows[0].FilterStep)

		var status struct {
			TotalRoutes    int `json:"total-routes"`
			FilteredRoutes int `json:"filtered-routes"`
		}
		require.NoError(t, json.Unmarshal(f.command(t, "show bgp adj-rib-in status"), &status))
		assert.Equal(t, 2, status.TotalRoutes, "total-routes includes accepted control and filtered route")
		assert.Equal(t, 1, status.FilteredRoutes, "filtered-routes counts only import-filtered entries")

		f.requireWithheld(t, prefix)
	})

	t.Run("import-policy-denial", func(t *testing.T) {
		f := newFilteredReceivedLive(t, true, mustPolicy(t, nil, routepolicy.FinalReject))
		prefix := netip.MustParsePrefix("203.0.115.0/24")
		attrs := firstASAttrs(4, 65002)
		f.send(t, f.source, &message.Update{
			PathAttributes: attrs,
			NLRI:           ownershipLiveNLRI([]netip.Prefix{prefix}),
		})

		var answer struct {
			Peers map[string][]struct {
				AttrHex    string `json:"attr-hex"`
				NLRIHex    string `json:"nlri-hex"`
				Filtered   *bool  `json:"filtered"`
				FilterStep string `json:"filter-step"`
			} `json:"adj-rib-in"`
		}
		require.NoError(t, json.Unmarshal(f.command(t, "show bgp adj-rib-in"), &answer))
		rows := answer.Peers[f.source.peer.Settings().Address.String()]
		require.Len(t, rows, 1)
		assert.Equal(t, hex.EncodeToString(attrs), rows[0].AttrHex)
		require.NotNil(t, rows[0].Filtered)
		assert.True(t, *rows[0].Filtered)
		assert.Equal(t, policyChainStepName, rows[0].FilterStep)

		f.requireWithheld(t, prefix)
	})
}

// TestRFC7947OrdinaryPeerDeniedUpdateAbsentFromAdjRIBIn proves that pre-policy
// filtered retention is confined to route-server client peers (rs-client true):
// an ordinary peer's ingress-denied UPDATE stays absent from Adj-RIB-In.
//
// RFC requirement: RFC7947-2.1-1 negative -- an ordinary (non-rs-client) peer
// whose UPDATE is denied on ingress is not retained on the filtered-update rail.
// Producer: internal/component/bgp/reactor/reactor_notify.go::retainDeniedIngress
func TestRFC7947OrdinaryPeerDeniedUpdateAbsentFromAdjRIBIn(t *testing.T) {
	f := newFilteredReceivedLive(t, false, mustPolicy(t, nil, routepolicy.FinalAccept))
	prefix := netip.MustParsePrefix("203.0.114.0/24")
	attrs := firstASAttrs(4, 65002, 65000)
	f.send(t, f.source, &message.Update{
		PathAttributes: attrs,
		NLRI:           ownershipLiveNLRI([]netip.Prefix{prefix}),
	})
	f.requireAbsent(t, f.source, prefix)
	f.requireWithheld(t, prefix)
}

// TestReceiveGrantAcceptsUpdateFilteredToken verifies that the registered
// update-filtered event name reaches both parseOneReceiveFlag and the YANG
// ReceiveEventValidator + completion list, while misspellings are rejected.
//
// VALIDATES: AC-10 and AC-11 of spec-rfc7947-adj-rib-in-accepts-filtered-updates.
func TestReceiveGrantAcceptsUpdateFilteredToken(t *testing.T) {
	b := &ProcessBinding{}
	require.NoError(t, parseOneReceiveFlag(bgpevents.EventUpdateFiltered, b))
	assert.Equal(t, events.DirBoth, b.Receive[bgpevents.EventUpdateFiltered])

	bRecv := &ProcessBinding{}
	require.NoError(t, parseOneReceiveFlag(bgpevents.EventUpdateFiltered+"-received", bRecv))
	assert.Equal(t, events.DirReceived, bRecv.Receive[bgpevents.EventUpdateFiltered])

	err := parseOneReceiveFlag("update-filterd", &ProcessBinding{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update-filterd")
	assert.Contains(t, err.Error(), bgpevents.EventUpdateFiltered)

	v := config.ReceiveEventValidator()
	require.NoError(t, v.ValidateFn("receive", bgpevents.EventUpdateFiltered))
	require.NoError(t, v.ValidateFn("receive", bgpevents.EventUpdateFiltered+"-received"))
	err = v.ValidateFn("receive", "update-filterd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update-filterd")

	completions := v.CompleteFn()
	assert.True(t, slices.Contains(completions, bgpevents.EventUpdateFiltered),
		"completion list must offer %s (got %v)", bgpevents.EventUpdateFiltered, completions)
}

// TestRFC7947FilteredRetainedPopulationBound runs the AC-15 counterexample
// sequence: maximum=1 in offered mode on an rs-client peer, denied A retained,
// unmatched withdrawal B decrementing the offered tally to 0, then denied C.
// C MUST NOT leave two retained entries; replacing A does not consume a second
// slot; withdrawing A releases its slot; an accepted control D occupies the
// released slot.
//
// VALIDATES: AC-15 and A-2 of spec-rfc7947-adj-rib-in-accepts-filtered-updates.
func TestRFC7947FilteredRetainedPopulationBound(t *testing.T) {
	t.Run("unit-offered-mode-retains-unique-bound", func(t *testing.T) {
		ps := newTestPeerSettingsWithPrefix(1, 0)
		ps.RSClient = true
		ps.PrefixTeardown = map[string]bool{"ipv4/unicast": false}
		s := NewSession(ps)

		// Announce A (10.0.0.0/24): offered=1, retained={A}.
		checkOK(t, s, announceBody(t, 0, 1))
		assert.Equal(t, int64(1), s.prefixCounts.counts[ipv4UKey])
		assert.Len(t, s.prefixCounts.sets[ipv4UKey], 1)

		// Unmatched withdrawal B (10.0.1.0/24): offered drops to 0, retained still {A}.
		checkOK(t, s, withdrawBody(t, 1, 1))
		assert.Equal(t, int64(0), s.prefixCounts.counts[ipv4UKey], "offered mode still decrements on withdrawal")
		assert.Len(t, s.prefixCounts.sets[ipv4UKey], 1, "retained set keeps A because B was not held")

		// Announce C (10.0.2.0/24): offered would move 0->1, but retained would exceed max=1 -> dropped!
		notif, drop := s.checkPrefixLimits(testWireUpdate(announceBody(t, 2, 1)))
		assert.Nil(t, notif)
		assert.True(t, drop, "denied C must be refused when unique retained population would exceed maximum=1")
		assert.Len(t, s.prefixCounts.sets[ipv4UKey], 1, "rollback must leave only A in retained set")

		// Replace A (10.0.0.0/24) while offered=0: does not consume a second slot.
		checkOK(t, s, announceBody(t, 0, 1))
		assert.Len(t, s.prefixCounts.sets[ipv4UKey], 1, "replacing A must not consume a second slot")

		// Withdraw A: releases its slot.
		checkOK(t, s, withdrawBody(t, 0, 1))
		assert.Empty(t, s.prefixCounts.sets[ipv4UKey], "withdrawing held key A must release its slot")

		// Announce D (10.0.3.0/24): occupies the released slot cleanly.
		checkOK(t, s, announceBody(t, 3, 1))
		assert.Len(t, s.prefixCounts.sets[ipv4UKey], 1)
	})

	t.Run("live-rs-client-sequence", func(t *testing.T) {
		source := acceptingLiveSettings(t, "192.0.2.1", 65000, 65002)
		source.RSClient = true
		source.ImportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
		source.PrefixMaximum = map[string]uint32{"ipv4/unicast": 1}
		source.PrefixTeardown = map[string]bool{"ipv4/unicast": false}
		control := acceptingLiveSettings(t, "192.0.2.2", 65000, 65003)
		control.RSClient = true
		for _, settings := range []*PeerSettings{source, control} {
			require.NoError(t, EnsureProcessBinding(settings, "bgp-adj-rib-in", "*", "update"))
		}
		peers := lowLiveRouter(t, source, control)
		f := &filteredReceivedLive{source: peers[0], control: peers[1]}

		pfxA := netip.MustParsePrefix("203.0.114.0/24")
		pfxB := netip.MustParsePrefix("203.0.115.0/24")
		pfxC := netip.MustParsePrefix("203.0.116.0/24")
		pfxD := netip.MustParsePrefix("203.0.117.0/24")
		loopAttrs := firstASAttrs(4, 65002, 65000)
		loopAttrs2 := firstASAttrs(4, 65002, 65000, 65099)
		okAttrs := firstASAttrs(4, 65002)

		// 1. Denied A is retained.
		f.send(t, f.source, &message.Update{PathAttributes: loopAttrs, NLRI: ownershipLiveNLRI([]netip.Prefix{pfxA})})
		f.requireRoute(t, f.source, pfxA, loopAttrs)

		// 2. Unmatched withdrawal B leaves A retained.
		f.send(t, f.source, &message.Update{WithdrawnRoutes: ownershipLiveNLRI([]netip.Prefix{pfxB})})
		f.requireRoute(t, f.source, pfxA, loopAttrs)

		// 3. Denied C cannot leave two retained entries.
		f.send(t, f.source, &message.Update{PathAttributes: loopAttrs, NLRI: ownershipLiveNLRI([]netip.Prefix{pfxC})})
		f.requireRoute(t, f.source, pfxA, loopAttrs)
		f.requireAbsent(t, f.source, pfxC)
		assert.Len(t, f.rows(t, f.source), 1, "unique retained entries must stay bounded at maximum=1")

		// 4. Replacing A updates its bytes without consuming a second slot.
		f.send(t, f.source, &message.Update{PathAttributes: loopAttrs2, NLRI: ownershipLiveNLRI([]netip.Prefix{pfxA})})
		f.requireRoute(t, f.source, pfxA, loopAttrs2)
		assert.Len(t, f.rows(t, f.source), 1)

		// 5. Withdrawing A releases its slot.
		f.send(t, f.source, &message.Update{WithdrawnRoutes: ownershipLiveNLRI([]netip.Prefix{pfxA})})
		assert.Empty(t, f.rows(t, f.source))

		// 6. Accepted control D occupies the released slot.
		f.send(t, f.source, &message.Update{PathAttributes: okAttrs, NLRI: ownershipLiveNLRI([]netip.Prefix{pfxD})})
		f.requireRoute(t, f.source, pfxD, okAttrs)
		assert.Len(t, f.rows(t, f.source), 1)
	})
}

// TestNotifyEmitsNoFilteredUpdateOnTeardown verifies that an ingress step that
// requests session teardown returns before queueing an update-filtered event.
//
// VALIDATES: Behavior to preserve / R-4 of spec-rfc7947-adj-rib-in-accepts-filtered-updates.
func TestNotifyEmitsNoFilteredUpdateOnTeardown(t *testing.T) {
	transport := &answerTransport{
		reads: []string{"med"},
		sets:  []string{"med"},
		answer: func(rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
			return rpc.FilterUpdateOutput{Action: rpc.FilterAccept, Teardown: true, NotifyCode: 6, NotifySubcode: 2}
		},
	}
	r := &Reactor{
		attrModHandlers:     attrModHandlersWithDefaults(),
		filterTransportSeam: transport,
		orderedIngressSteps: []orderedIngressStep{{name: policyChainStepName, policyChain: true}},
		peers:               make(map[netip.AddrPort]*Peer),
	}
	addr := netip.MustParseAddr("192.0.2.1")
	peer := NewPeer(&PeerSettings{
		Address:      addr,
		LocalAS:      65000,
		PeerAS:       65002,
		RSClient:     true,
		ImportPolicy: boundaryPolicy(t, routepolicy.FinalAccept, boundaryRef{name: "teardown"}),
	})
	r.peers[netip.AddrPortFrom(addr, 179)] = peer

	body := message.PackTo(&message.Update{
		PathAttributes: firstASAttrs(4, 65002),
		NLRI:           ownershipLiveNLRI([]netip.Prefix{netip.MustParsePrefix("203.0.118.0/24")}),
	}, nil)[message.HeaderLen:]
	wu := wireu.NewWireUpdate(body, 0)
	var follow []ingressPolicyContinuation
	accepted := r.notifyOneMessage(peer, nil, addr, msgtype.TypeUPDATE, body, wu, 0,
		rpc.DirectionReceived, BufHandle{}, nil, "", 0, ingressPolicyUndecided, 0, false, "", &follow)
	assert.False(t, accepted, "teardown verdict must not accept the UPDATE")
	assert.Empty(t, follow, "teardown verdict must queue neither withdrawals nor update-filtered")
}
