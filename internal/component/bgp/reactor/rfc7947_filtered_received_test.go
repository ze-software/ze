// Design: docs/architecture/plugin/rib-storage-design.md -- pre-policy received routes
// Related: reactor_notify.go -- ordered ingress and received-event publication
// Related: rfc7705_live_behavior_test.go -- live sessions and registered receive store
// RFC naming: untagged -- red probes of RFC 7947 Section 2.1 retention; the absent behavior has no conformance proof yet.
package reactor

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// filteredReceivedLive retains the real plugin command boundary. Its control
// source has an accepting policy, so dropping every UPDATE cannot pass a probe.
type filteredReceivedLive struct {
	source  *lowLivePeer
	control *lowLivePeer
}

func newFilteredReceivedLive(t *testing.T, client bool, policy filterapi.Policy) *filteredReceivedLive {
	t.Helper()
	source := acceptingLiveSettings(t, "192.0.2.1", 65000, 65002)
	source.RSClient = client
	source.ImportPolicy = policy
	control := acceptingLiveSettings(t, "192.0.2.2", 65000, 65003)
	control.RSClient = true
	for _, settings := range []*PeerSettings{source, control} {
		// A wildcard is an existing grant. It admits a separately registered
		// filtered event without naming a type that does not exist yet.
		require.NoError(t, EnsureProcessBinding(settings, "bgp-adj-rib-in", "*", "update"))
	}
	peers := lowLiveRouter(t, source, control)
	f := &filteredReceivedLive{source: peers[0], control: peers[1]}
	f.send(t, f.control, &message.Update{
		PathAttributes: firstASAttrs(4, 65003),
		NLRI:           ownershipLiveNLRI([]netip.Prefix{filteredReceivedControl()}),
	})
	f.requireRoute(t, f.control, filteredReceivedControl(), firstASAttrs(4, 65003))
	return f
}

func filteredReceivedControl() netip.Prefix {
	return netip.MustParsePrefix("203.0.113.0/24")
}

// send joins the source's read loop, delivery worker and receive-store handler.
// The KEEPALIVE follows the UPDATE on the same socket; its counter means the
// UPDATE's notify call returned. The publication cut then joins the peer's
// worker before the process's applied drain joins the store mutation.
func (f *filteredReceivedLive) send(t *testing.T, source *lowLivePeer, update *message.Update) {
	t.Helper()
	before := source.peer.Stats().KeepalivesReceived
	source.send(t, message.PackTo(update, nil))
	source.send(t, message.PackTo(message.NewKeepalive(), nil))
	lowEventually(t, func() bool {
		return source.peer.Stats().KeepalivesReceived > before
	}, "the source's UPDATE followed by its KEEPALIVE")
	proc := source.peer.reactor.api.ProcessManager().GetProcess("bgp-adj-rib-in")
	require.NotNil(t, proc)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, source.peer.receiveCut().wait(ctx))
	require.NoError(t, proc.DrainEventsApplied(ctx))
}

func (f *filteredReceivedLive) command(t *testing.T, command string, args ...string) json.RawMessage {
	t.Helper()
	proc := f.source.peer.reactor.api.ProcessManager().GetProcess("bgp-adj-rib-in")
	require.NotNil(t, proc)
	return llgrWriterCommand(t, proc, command, args...)
}

type filteredReceivedRow struct {
	Family  string `json:"family"`
	AttrHex string `json:"attr-hex"`
	NLRIHex string `json:"nlri-hex"`
}

func (f *filteredReceivedLive) rows(t *testing.T, source *lowLivePeer) []filteredReceivedRow {
	t.Helper()
	var answer struct {
		Peers map[string][]filteredReceivedRow `json:"adj-rib-in"`
	}
	require.NoError(t, json.Unmarshal(f.command(t, "show bgp adj-rib-in"), &answer))
	return answer.Peers[source.peer.Settings().Address.String()]
}

func (f *filteredReceivedLive) requireRoute(t *testing.T, source *lowLivePeer, prefix netip.Prefix, attrs []byte) {
	t.Helper()
	wantNLRI := hex.EncodeToString(ownershipLiveNLRI([]netip.Prefix{prefix}))
	for _, row := range f.rows(t, source) {
		if row.NLRIHex != wantNLRI {
			continue
		}
		require.Equal(t, "ipv4/unicast", row.Family)
		require.Equal(t, hex.EncodeToString(attrs), row.AttrHex, "the receive store must retain the original pre-policy attributes")
		return
	}
	t.Fatalf("Adj-RIB-In did not retain %s from %s", prefix, source.peer.Settings().Address)
}

func (f *filteredReceivedLive) requireAbsent(t *testing.T, source *lowLivePeer, prefix netip.Prefix) {
	t.Helper()
	wantNLRI := hex.EncodeToString(ownershipLiveNLRI([]netip.Prefix{prefix}))
	for _, row := range f.rows(t, source) {
		require.NotEqual(t, wantNLRI, row.NLRIHex, "the withdrawn or ordinary-peer denied route must be absent")
	}
}

// requireWithheld checks both replay consumers, with an accepted route as the
// positive control. A retained denial must not return an announcement on either
// the paged peer-up walk or the single-path validation recovery rail.
func (f *filteredReceivedLive) requireWithheld(t *testing.T, prefix netip.Prefix) {
	t.Helper()
	const destination = "192.0.2.254"
	var page ribevents.ReplayPage
	require.NoError(t, json.Unmarshal(f.command(t, ribevents.ReplayRoutesCommand, destination, "0"), &page))
	control := hex.EncodeToString(ownershipLiveNLRI([]netip.Prefix{filteredReceivedControl()}))
	denied := hex.EncodeToString(ownershipLiveNLRI([]netip.Prefix{prefix}))
	controlFound := false
	for _, route := range page.Routes {
		require.NotEqual(t, denied, route.NLRIHex, "a policy-denied route escaped the peer-up replay walk")
		if route.NLRIHex == control {
			controlFound = true
			require.False(t, route.Withdraw)
			require.Equal(t, hex.EncodeToString(firstASAttrs(4, 65003)), route.AttrHex)
		}
	}
	require.True(t, controlFound, "replay must still include the accepted control route")
	sourceOwner := strconv.FormatUint(f.source.peer.sourceOwner, 10)
	sourceConn := strconv.FormatUint(f.source.peer.currentSession().connectionID, 10)
	var path ribevents.ReplayPage
	require.NoError(t, json.Unmarshal(f.command(t, ribevents.ReplayPathCommand, destination,
		f.source.peer.Settings().Address.String(), "ipv4/unicast", prefix.String(), "0", sourceOwner, sourceConn), &path))
	require.Len(t, path.Routes, 1)
	require.True(t, path.Routes[0].Withdraw, "validation recovery must answer a policy-denied path as withdrawn")
	controlOwner := strconv.FormatUint(f.control.peer.sourceOwner, 10)
	controlConn := strconv.FormatUint(f.control.peer.currentSession().connectionID, 10)
	var accepted ribevents.ReplayPage
	require.NoError(t, json.Unmarshal(f.command(t, ribevents.ReplayPathCommand, destination,
		f.control.peer.Settings().Address.String(), "ipv4/unicast", filteredReceivedControl().String(), "0", controlOwner, controlConn), &accepted))
	require.Len(t, accepted.Routes, 1)
	require.False(t, accepted.Routes[0].Withdraw, "validation recovery must retain the accepted control")
}

// TestRFC7947ReceivedLoopDenialRetained exercises the registered loop guard
// through real ingress and reads the receive-store command answer. An ordinary
// peer is the role control; its denied route stays absent under the agreed scope.
// RFC 7947 Section 2.1: "A route server MUST accept all UPDATE messages received from each of its clients for inclusion in its Adj-RIB-In."
// VALIDATES: RFC 7947 Section 2.1 client UPDATE inclusion independently of a loop-policy denial.
func TestRFC7947ReceivedLoopDenialRetained(t *testing.T) {
	for _, client := range []bool{true, false} {
		name := "ordinary-peer"
		if client {
			name = "route-server-client"
		}
		t.Run(name, func(t *testing.T) {
			f := newFilteredReceivedLive(t, client, mustPolicy(t, nil, routepolicy.FinalAccept))
			prefix := netip.MustParsePrefix("203.0.114.0/24")
			attrs := firstASAttrs(4, 65002, 65000)
			f.send(t, f.source, &message.Update{PathAttributes: attrs,
				NLRI: ownershipLiveNLRI([]netip.Prefix{prefix})})
			if client {
				f.requireRoute(t, f.source, prefix, attrs)
			} else {
				f.requireAbsent(t, f.source, prefix)
			}
			f.requireWithheld(t, prefix)
		})
	}
}

// TestRFC7947ReceivedImportFinalDenialRetained observes the pre-policy store
// for explicit reject and absent EBGP import policy, with the same valid wire
// UPDATE and an accepted peer as the control.
// RFC 7947 Section 2.1: "A route server MUST accept all UPDATE messages received from each of its clients for inclusion in its Adj-RIB-In."
// VALIDATES: RFC 7947 Section 2.1 inclusion before explicit or default import-policy denial.
func TestRFC7947ReceivedImportFinalDenialRetained(t *testing.T) {
	for _, explicit := range []bool{true, false} {
		name := "absent-import-policy"
		var policy filterapi.Policy
		if explicit {
			name = "explicit-final-reject"
			policy = mustPolicy(t, nil, routepolicy.FinalReject)
		}
		t.Run(name, func(t *testing.T) {
			f := newFilteredReceivedLive(t, true, policy)
			prefix := netip.MustParsePrefix("203.0.114.0/24")
			attrs := firstASAttrs(4, 65002)
			f.send(t, f.source, &message.Update{PathAttributes: attrs,
				NLRI: ownershipLiveNLRI([]netip.Prefix{prefix})})
			f.requireRoute(t, f.source, prefix, attrs)
			f.requireWithheld(t, prefix)
		})
	}
}

// TestRFC7947ReceivedMixedDenialRetainsAnnouncement sends a mixed UPDATE
// whose ordinary policy result accepts a withdrawal-only projection. The
// receive store must remove the withdrawn key and keep the original denied
// announcement; its later withdrawal must remove that retained entry too.
// RFC 7947 Section 2.1: "A route server MUST accept all UPDATE messages received from each of its clients for inclusion in its Adj-RIB-In."
// VALIDATES: RFC 7947 Section 2.1 inclusion of the original announcement when the accepted consumer payload contains only withdrawals.
func TestRFC7947ReceivedMixedDenialRetainsAnnouncement(t *testing.T) {
	f := newFilteredReceivedLive(t, true, mustPolicy(t, nil, routepolicy.FinalReject))
	old := netip.MustParsePrefix("203.0.114.0/24")
	next := netip.MustParsePrefix("203.0.115.0/24")
	attrs := firstASAttrs(4, 65002)
	f.send(t, f.source, &message.Update{PathAttributes: attrs,
		NLRI: ownershipLiveNLRI([]netip.Prefix{old})})
	f.send(t, f.source, &message.Update{
		WithdrawnRoutes: ownershipLiveNLRI([]netip.Prefix{old}),
		PathAttributes:  attrs,
		NLRI:            ownershipLiveNLRI([]netip.Prefix{next}),
	})
	f.requireAbsent(t, f.source, old)
	f.requireRoute(t, f.source, next, attrs)
	require.Len(t, f.rows(t, f.source), 1, "withdrawals precede announcements in the original UPDATE")
	f.requireWithheld(t, next)
	f.send(t, f.source, &message.Update{WithdrawnRoutes: ownershipLiveNLRI([]netip.Prefix{next})})
	require.Empty(t, f.rows(t, f.source), "an explicit withdrawal must remove the filtered entry")
	f.requireRoute(t, f.control, filteredReceivedControl(), firstASAttrs(4, 65003))
}

// TestRFC7947ReceivedDeniedReplacementSupersedesOldBytes replaces a received
// accepted path with a loop-denied version of the same NLRI. Keeping the old
// attributes or replaying that earlier version would misrepresent the client.
// RFC 7947 Section 2.1: "A route server MUST accept all UPDATE messages received from each of its clients for inclusion in its Adj-RIB-In."
// VALIDATES: RFC 7947 Section 2.1 retention of the client's current UPDATE after a previously accepted version.
func TestRFC7947ReceivedDeniedReplacementSupersedesOldBytes(t *testing.T) {
	f := newFilteredReceivedLive(t, true, mustPolicy(t, nil, routepolicy.FinalAccept))
	prefix := netip.MustParsePrefix("203.0.114.0/24")
	accepted := firstASAttrs(4, 65002)
	f.send(t, f.source, &message.Update{PathAttributes: accepted,
		NLRI: ownershipLiveNLRI([]netip.Prefix{prefix})})
	f.requireRoute(t, f.source, prefix, accepted)
	denied := firstASAttrs(4, 65002, 65000)
	f.send(t, f.source, &message.Update{PathAttributes: denied,
		NLRI: ownershipLiveNLRI([]netip.Prefix{prefix})})
	f.requireRoute(t, f.source, prefix, denied)
	require.Len(t, f.rows(t, f.source), 1, "a replacement occupies the same received-route key")
	f.requireWithheld(t, prefix)
}

// TestRFC7947ReceivedValidationCannotAcceptPolicyDenial accepts a retained
// policy denial through the real validation command. Origin validation may
// record Valid, but it cannot grant the import policy's missing permission.
// RFC 7947 Section 2.1: "These UPDATE messages MAY be omitted from the route server's Loc-RIB or Loc-RIBs, due to filters configured for the purpose of implementing routing policy."
// VALIDATES: RFC 7947 Section 2.1 separation of received retention and local-policy advertisement after a validation decision.
func TestRFC7947ReceivedValidationCannotAcceptPolicyDenial(t *testing.T) {
	f := newFilteredReceivedLive(t, true, mustPolicy(t, nil, routepolicy.FinalAccept))
	f.command(t, "request bgp adj-rib-in enable-validation")
	prefix := netip.MustParsePrefix("203.0.114.0/24")
	attrs := firstASAttrs(4, 65002, 65000)
	f.send(t, f.source, &message.Update{PathAttributes: attrs,
		NLRI: ownershipLiveNLRI([]netip.Prefix{prefix})})
	f.requireRoute(t, f.source, prefix, attrs)
	for _, route := range []struct {
		peer   *lowLivePeer
		prefix netip.Prefix
	}{{f.source, prefix}, {f.control, filteredReceivedControl()}} {
		f.command(t, "request bgp adj-rib-in accept-routes", route.peer.peer.Settings().Address.String(),
			"ipv4/unicast", route.prefix.String(), "0", "1")
	}
	f.requireRoute(t, f.source, prefix, attrs)
	f.requireWithheld(t, prefix)
	require.False(t, ribevents.RouteEligible(ribevents.ValidationRoute{
		Peer:         f.source.peer.Settings().Address,
		PeerOwner:    f.source.peer.sourceOwner,
		ConnectionID: f.source.peer.currentSession().connectionID,
		Family:       family.IPv4Unicast,
		Prefix:       prefix,
	}, 0), "a Valid origin cannot make an import-denied route eligible")
	require.True(t, ribevents.RouteEligible(ribevents.ValidationRoute{
		Peer:         f.control.peer.Settings().Address,
		PeerOwner:    f.control.peer.sourceOwner,
		ConnectionID: f.control.peer.currentSession().connectionID,
		Family:       family.IPv4Unicast,
		Prefix:       filteredReceivedControl(),
	}, 0), "the validation gate must admit the accepted control route")
}
