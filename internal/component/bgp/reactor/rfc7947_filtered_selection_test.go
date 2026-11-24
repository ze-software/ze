// Design: docs/architecture/plugin/rib-storage-design.md -- received retention and selection eligibility.
// Related: rfc7947_filtered_received_test.go -- command observations and applied receive barriers.
// Related: rfc7947_rs_announcement_election_test.go -- registered RIB/route-server session fixture.
// Related: rfc4271_selected_boundaries_test.go -- shared recipient TCP capture fence.
// RFC naming: untagged -- red RFC 7947 retention and policy-boundary probes have no conformance proof yet.
package reactor

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// filteredSelectedLive adds the selecting RIB and recipient observations to
// the received-store command fixture. Every BGP route enters a real session;
// only the terminal interior route used to resolve NEXT_HOP is test-installed.
type filteredSelectedLive struct {
	*filteredReceivedLive
	client    *Peer
	recipient *lowLivePeer
	peers     []*lowLivePeer
	markers   byte
}

func newFilteredSelectedLive(t *testing.T, policy filterapi.Policy, fastPath, connectClient bool) *filteredSelectedLive {
	t.Helper()
	settings := []*PeerSettings{
		acceptingLiveSettings(t, "192.0.2.201", 65000, 65002),
		acceptingLiveSettings(t, "192.0.2.202", 65000, 65003),
		acceptingLiveSettings(t, "192.0.2.203", 65000, 65004),
	}
	settings[0].ImportPolicy = policy
	for _, setting := range settings {
		setting.RSClient = true
		setting.RSFastPath = fastPath
		// The existing wildcard admits a future distinct filtered event without
		// naming or fabricating an event the product has not registered yet.
		addTestPeerBusGrant(setting, testPeerEndpointGrant{Name: "bgp-adj-rib-in", ReceiveAll: true, Send: []string{"update"}})
	}
	peers := lowLiveStart(t, lowLiveOptions{}, settings...)
	loc := locrib.Default()
	require.NotNil(t, loc, "the registered RIB plugin must use the daemon Loc-RIB")
	nextHop := netip.MustParsePrefix("192.0.2.254/32")
	protocol := redistevents.RegisterProtocol("rfc7947-filtered-selection-test")
	loc.Insert(family.IPv4Unicast, nextHop, locrib.Path{Source: protocol, Metric: 7})
	t.Cleanup(func() { loc.Remove(family.IPv4Unicast, nextHop, protocol, 0) })
	require.True(t, igpcost.Resolve(loc, nextHop.Addr()).Resolved,
		"an unresolved test next hop must not stand in for the import-policy decision")
	f := &filteredSelectedLive{
		filteredReceivedLive: &filteredReceivedLive{
			source:  lowLiveConnect(t, peers[0], 0x0a00e201),
			control: lowLiveConnect(t, peers[1], 0x0a00e202),
		},
		client: peers[2],
	}
	f.peers = append(f.peers, f.source, f.control)
	if connectClient {
		f.connectClient(t)
	}
	f.send(t, f.control, &message.Update{
		PathAttributes: firstASAttrs(4, 65003),
		NLRI:           ownershipLiveNLRI([]netip.Prefix{filteredReceivedControl()}),
	})
	f.requireRoute(t, f.control, filteredReceivedControl(), firstASAttrs(4, 65003))
	f.requireSelected(t, filteredReceivedControl())
	if connectClient {
		ownershipLiveThrough(t, f.recipient, filteredReceivedControl(), firstASAttrs(4, 65003))
	}
	return f
}

func (f *filteredSelectedLive) connectClient(t *testing.T) {
	t.Helper()
	f.recipient = lowLiveConnect(t, f.client, 0x0a00e203)
	f.peers = append(f.peers, f.recipient)
}

// send first joins receipt and the receive store, then the selecting RIB and
// forwarding plugin. Thus a later control marker is queued after these input
// handlers; a KEEPALIVE counter alone cannot establish that ordering.
func (f *filteredSelectedLive) send(t *testing.T, source *lowLivePeer, update *message.Update) {
	t.Helper()
	f.filteredReceivedLive.send(t, source, update)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	for _, name := range []string{"bgp-rib", "bgp-rs"} {
		proc := source.peer.reactor.api.ProcessManager().GetProcess(name)
		require.NotNil(t, proc)
		require.NoError(t, proc.DrainEventsApplied(ctx))
	}
}

func (f *filteredSelectedLive) requireSelected(t *testing.T, prefix netip.Prefix) {
	t.Helper()
	nextHop := netip.MustParseAddr("192.0.2.254")
	require.True(t, igpcost.Resolve(locrib.Default(), nextHop).Resolved)
	lowEventually(t, func() bool {
		path, present := locrib.Default().Best(family.IPv4Unicast, prefix)
		if !present {
			return false
		}
		return path.NextHop == nextHop
	}, "accepted route selected in the daemon Loc-RIB: "+prefix.String())
}

func (f *filteredSelectedLive) assertUnselected(t *testing.T, prefix netip.Prefix) {
	t.Helper()
	path, present := locrib.Default().Best(family.IPv4Unicast, prefix)
	assert.False(t, present, "policy-denied %s must not remain selected: %+v", prefix, path)
}

// assertFiltered is deliberately nonfatal: an absent or stale received entry
// must not prevent the test from observing selection, replay and recipient
// leakage as independent downstream failures. Its result gates the later
// removal check, which needs an actually retained, marked precondition.
func (f *filteredSelectedLive) assertFiltered(t *testing.T, prefix netip.Prefix, attrs []byte) bool {
	t.Helper()
	var answer struct {
		Peers map[string][]struct {
			filteredReceivedRow
			Filtered *bool `json:"filtered"`
		} `json:"adj-rib-in"`
	}
	require.NoError(t, json.Unmarshal(f.command(t, "show bgp adj-rib-in"), &answer))
	wantNLRI := hex.EncodeToString(ownershipLiveNLRI([]netip.Prefix{prefix}))
	for _, row := range answer.Peers[f.source.peer.Settings().Address.String()] {
		if row.NLRIHex != wantNLRI {
			continue
		}
		attributesMatch := assert.Equal(t, hex.EncodeToString(attrs), row.AttrHex,
			"the retained entry must describe the denied replacement, never an older accepted version")
		assert.Equal(t, "ipv4/unicast", row.Family)
		if !assert.NotNil(t, row.Filtered, "the receive-store answer must expose its filtered mark") {
			return false
		}
		marked := assert.True(t, *row.Filtered, "an import-denied entry must be marked filtered")
		if !attributesMatch {
			return false
		}
		return marked
	}
	assert.Fail(t, "the denied UPDATE is missing from the pre-policy receive store", prefix.String())
	return false
}

// settle uses two accepted control UPDATEs and the destination's empty owed
// set to bound its complete TCP history, including a queued recovery decision.
// The loop is bounded at two markers, and each wait has lowEventually's bound.
func (f *filteredSelectedLive) settle(t *testing.T) ownershipLiveProjection {
	t.Helper()
	for range 2 {
		f.markers++
		marker := netip.PrefixFrom(netip.AddrFrom4([4]byte{203, 0, 120 + f.markers, 0}), 24)
		f.send(t, f.control, &message.Update{
			PathAttributes: firstASAttrs(4, 65003),
			NLRI:           ownershipLiveNLRI([]netip.Prefix{marker}),
		})
		f.requireSelected(t, marker)
		ownershipLiveThrough(t, f.recipient, marker, firstASAttrs(4, 65003))
		lowEventually(t, func() bool { return adjOutOwedCount(f.recipient.peer) == 0 }, "recipient owes no policy decision")
	}
	// Recovery output may follow the last marker. Join its writer and then
	// the socket capture before rebuilding the final route history.
	forwardSocketBarrier(t, f.source.peer.reactor)
	ordinaryBoundaryCapture(t, f.recipient)
	view := ownershipLiveRead(t, f.recipient)
	ownershipLiveRoute(t, view, filteredReceivedControl(), firstASAttrs(4, 65003))
	return view
}

// assertReplayWithheld asks both real receive-store replay handlers for the
// actual recipient. The accepted route must remain an announcement on both
// rails, so an empty or refusing replay implementation cannot pass.
func (f *filteredSelectedLive) assertReplayWithheld(t *testing.T, prefix netip.Prefix) {
	t.Helper()
	destination := f.client.Settings().Address.String()
	var page ribevents.ReplayPage
	require.NoError(t, json.Unmarshal(f.command(t, ribevents.ReplayRoutesCommand, destination, "0"), &page))
	control := hex.EncodeToString(ownershipLiveNLRI([]netip.Prefix{filteredReceivedControl()}))
	denied := hex.EncodeToString(ownershipLiveNLRI([]netip.Prefix{prefix}))
	controlFound := false
	for _, route := range page.Routes {
		assert.NotEqual(t, denied, route.NLRIHex, "the policy-denied entry escaped the peer-up replay walk")
		if route.NLRIHex == control {
			controlFound = true
			require.False(t, route.Withdraw)
			require.Equal(t, hex.EncodeToString(firstASAttrs(4, 65003)), route.AttrHex)
		}
	}
	require.True(t, controlFound, "the accepted control must remain available to the peer-up replay walk")
	sourceOwner := strconv.FormatUint(f.source.peer.sourceOwner, 10)
	sourceConn := strconv.FormatUint(f.source.peer.currentSession().connectionID, 10)
	var path ribevents.ReplayPage
	require.NoError(t, json.Unmarshal(f.command(t, ribevents.ReplayPathCommand, destination,
		f.source.peer.Settings().Address.String(), "ipv4/unicast", prefix.String(), "0", sourceOwner, sourceConn), &path))
	require.Len(t, path.Routes, 1)
	assert.True(t, path.Routes[0].Withdraw, "single-path recovery must withhold the policy-denied entry")
	controlOwner := strconv.FormatUint(f.control.peer.sourceOwner, 10)
	controlConn := strconv.FormatUint(f.control.peer.currentSession().connectionID, 10)
	var accepted ribevents.ReplayPage
	require.NoError(t, json.Unmarshal(f.command(t, ribevents.ReplayPathCommand, destination,
		f.control.peer.Settings().Address.String(), "ipv4/unicast", filteredReceivedControl().String(), "0", controlOwner, controlConn), &accepted))
	require.Len(t, accepted.Routes, 1)
	require.False(t, accepted.Routes[0].Withdraw)
	require.Equal(t, hex.EncodeToString(firstASAttrs(4, 65003)), accepted.Routes[0].AttrHex)
}

// TestRFC7947FilteredReceivedExcludedFromSelection observes the denied UPDATE
// at the received command, actual selected Loc-RIB and recipient socket. The
// accepted control proves receipt, reachability, selection and useful export.
// RFC 7947 Section 2.1: "A route server MUST accept all UPDATE messages received from each of its clients for inclusion in its Adj-RIB-In."
// VALIDATES: retaining a denied client UPDATE does not grant selection, live advertisement or replay permission.
func TestRFC7947FilteredReceivedExcludedFromSelection(t *testing.T) {
	for _, fastPath := range []bool{false, true} {
		rail := "cached-rail"
		if fastPath {
			rail = "fast-rail"
		}
		for _, denial := range []string{"loop", "explicit-import-reject", "absent-import-policy"} {
			t.Run(rail+"/"+denial, func(t *testing.T) {
				var policy filterapi.Policy
				attrs := firstASAttrs(4, 65002)
				if denial == "loop" {
					policy = mustPolicy(t, nil, routepolicy.FinalAccept)
					attrs = firstASAttrs(4, 65002, 65000)
				}
				if denial == "explicit-import-reject" {
					policy = mustPolicy(t, nil, routepolicy.FinalReject)
				}
				f := newFilteredSelectedLive(t, policy, fastPath, true)
				prefix := netip.MustParsePrefix("198.19.221.0/24")
				f.send(t, f.source, &message.Update{PathAttributes: attrs,
					NLRI: ownershipLiveNLRI([]netip.Prefix{prefix})})
				f.assertFiltered(t, prefix, attrs)
				f.assertUnselected(t, prefix)
				view := f.settle(t)
				assert.Empty(t, view.announcements[prefix], "no denied announcement may reach the live recipient")
				assert.NotContains(t, view.routes, prefix)
				f.assertReplayWithheld(t, prefix)
				ownershipLiveEstablished(t, f.peers)
			})
		}
	}
}

// TestRFC7947FilteredReplacementClearsSelectedRoute starts with a selected
// and advertised path, then receives its loop-denied replacement. The old
// accepted bytes may survive in neither the selected table nor an export rail.
// RFC 7947 Section 2.1: "These UPDATE messages MAY be omitted from the route server's Loc-RIB or Loc-RIBs, due to filters configured for the purpose of implementing routing policy."
// VALIDATES: policy denial replaces received truth without keeping an older selected, advertised or replayable version.
func TestRFC7947FilteredReplacementClearsSelectedRoute(t *testing.T) {
	for _, fastPath := range []bool{false, true} {
		rail := "cached-rail"
		if fastPath {
			rail = "fast-rail"
		}
		t.Run(rail, func(t *testing.T) {
			f := newFilteredSelectedLive(t, mustPolicy(t, nil, routepolicy.FinalAccept), fastPath, true)
			prefix := netip.MustParsePrefix("198.19.222.0/24")
			accepted := firstASAttrs(4, 65002)
			f.send(t, f.source, &message.Update{PathAttributes: accepted,
				NLRI: ownershipLiveNLRI([]netip.Prefix{prefix})})
			f.requireRoute(t, f.source, prefix, accepted)
			f.requireSelected(t, prefix)
			ownershipLiveThrough(t, f.recipient, prefix, accepted)
			denied := firstASAttrs(4, 65002, 65000)
			f.send(t, f.source, &message.Update{PathAttributes: denied,
				NLRI: ownershipLiveNLRI([]netip.Prefix{prefix})})
			retained := f.assertFiltered(t, prefix, denied)
			f.assertUnselected(t, prefix)
			view := f.settle(t)
			assert.NotContains(t, view.routes, prefix, "the earlier accepted path must be withdrawn after its denied replacement")
			assert.Len(t, view.announcements[prefix], 1, "only the original accepted announcement may leave Ze")
			assert.Equal(t, 1, view.withdrawals[prefix], "policy denial must withdraw the previously advertised path")
			f.assertReplayWithheld(t, prefix)
			require.True(t, retained, "removal is not proven until the denied replacement is actually retained and marked")
			f.send(t, f.source, &message.Update{WithdrawnRoutes: ownershipLiveNLRI([]netip.Prefix{prefix})})
			f.requireAbsent(t, f.source, prefix)
			f.assertUnselected(t, prefix)
			view = f.settle(t)
			assert.NotContains(t, view.routes, prefix)
			ownershipLiveEstablished(t, f.peers)
		})
	}
}

// TestRFC7947FilteredReceivedWithheldOnClientJoin establishes the recipient
// after the denied UPDATE. It requires real accepted replay before judging
// denied-route absence, then tests removal only from a proven filtered entry.
// RFC 7947 Section 2.1: "A route server MUST accept all UPDATE messages received from each of its clients for inclusion in its Adj-RIB-In."
// VALIDATES: peer-up replay keeps policy-denied received entries private, while accepted selected routes remain usable.
func TestRFC7947FilteredReceivedWithheldOnClientJoin(t *testing.T) {
	for _, fastPath := range []bool{false, true} {
		rail := "cached-rail"
		if fastPath {
			rail = "fast-rail"
		}
		t.Run(rail, func(t *testing.T) {
			f := newFilteredSelectedLive(t, mustPolicy(t, nil, routepolicy.FinalReject), fastPath, false)
			prefix := netip.MustParsePrefix("198.19.223.0/24")
			attrs := firstASAttrs(4, 65002)
			f.send(t, f.source, &message.Update{PathAttributes: attrs,
				NLRI: ownershipLiveNLRI([]netip.Prefix{prefix})})
			retained := f.assertFiltered(t, prefix, attrs)
			f.assertUnselected(t, prefix)
			f.connectClient(t)
			view := ownershipLiveThrough(t, f.recipient, filteredReceivedControl(), firstASAttrs(4, 65003))
			assert.Empty(t, view.announcements[prefix], "a new client must never receive a denied replay announcement")
			assert.NotContains(t, view.routes, prefix)
			view = f.settle(t)
			assert.Empty(t, view.announcements[prefix])
			f.assertReplayWithheld(t, prefix)
			require.True(t, retained, "removal is not proven until the denied UPDATE is actually retained and marked")
			f.send(t, f.source, &message.Update{WithdrawnRoutes: ownershipLiveNLRI([]netip.Prefix{prefix})})
			f.requireAbsent(t, f.source, prefix)
			f.requireSelected(t, filteredReceivedControl())
			view = f.settle(t)
			assert.Empty(t, view.announcements[prefix])
			assert.NotContains(t, view.routes, prefix)
			ownershipLiveEstablished(t, f.peers)
		})
	}
}
