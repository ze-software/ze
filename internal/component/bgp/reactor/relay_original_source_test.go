// Design: docs/architecture/api/process-protocol.md -- original source replay boundary.
// Related: relay_original_source.go -- receipt facts are Session-wide, never per-route grants.
package reactor

import (
	"encoding/hex"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type originalRelayFixture struct {
	api     *reactorAPIAdapter
	source  *Peer
	target  *Peer
	session *Session
	route   rpc.StoredRoute
	items   *[]fwdItem
	mu      *sync.Mutex
	done    chan struct{}
}

// newOriginalRelayFixture records the original using the actual Session issuer,
// then reconstructs through the same source/cache producer the public rail uses.
func newOriginalRelayFixture(t *testing.T) *originalRelayFixture {
	t.Helper()
	api, _, items, mu, done := relayFixture(t)
	source := api.r.peers[netip.MustParseAddrPort("10.0.0.1:179")]
	target := api.r.peers[netip.MustParseAddrPort("10.0.0.2:179")]
	require.NotNil(t, source)
	require.NotNil(t, target)
	session := sourceSessionFixture(t, source)
	require.NoError(t, session.captureSourceReceipt(source, &sourceReceipts))
	t.Cleanup(session.closeSourceReceipt)
	route := storedIPv4Route(source.addrString)
	route.SourceOwner = source.sourceOwner
	route.SourceConnectionID = session.connectionID
	route.SourceReceipt = session.sourceReceiptID()
	route.SourcePolicyEpoch = source.sourcePolicyEpoch.Load()
	route.MsgID = 19
	route.NLRIFraming = rpc.NLRIFramingPrefixOnly
	return &originalRelayFixture{api: api, source: source, target: target, session: session, route: route, items: items, mu: mu, done: done}
}

func (f *originalRelayFixture) cached(t *testing.T) (*ReceivedUpdate, uint64) {
	t.Helper()
	source, err := f.api.originalRelaySource(f.source.settings.Address, &f.route, plugin.OperatorSender())
	require.NoError(t, err)
	var spans []relayAttrSpan
	update, id, consumed, err := f.api.buildRelayUpdate([]rpc.StoredRoute{f.route}, &source, &spans)
	require.NoError(t, err)
	require.Equal(t, 1, consumed)
	t.Cleanup(func() { f.api.r.recentUpdates.Release(id) })
	return update, id
}

// TestOriginalRelayReceiptSeparatesSameAddressPeers checks the actual stored
// source evaluator and reconstruction: a sibling cannot replace ASN, router or
// encoding facts, even though the operator selector still displays one address.
func TestOriginalRelayReceiptSeparatesSameAddressPeers(t *testing.T) {
	f := newOriginalRelayFixture(t)
	sibling := NewPeer(&PeerSettings{Address: f.source.settings.Address, Port: 2179, Name: "sibling", LocalAS: 65000, PeerAS: 65199})
	sourceSessionFixture(t, sibling)
	f.api.r.peers[sibling.settings.PeerKey()] = sibling
	source, err := f.api.originalRelaySource(f.source.settings.Address, &f.route, plugin.OperatorSender())
	require.NoError(t, err)
	require.Same(t, f.source, source.info.peer)
	require.Equal(t, f.source.settings.PeerAS, source.info.filterInfo.PeerAS)
	require.Equal(t, f.session.connectionID, source.info.filterInfo.ConnectionID)
	update, _ := f.cached(t)
	info, err := originalCachedSource(update)
	require.NoError(t, err)
	require.Same(t, f.source, info.peer)
	require.Equal(t, f.session.connectionID, info.filterInfo.ConnectionID)
}

// TestPublicCachedForwardUsesOriginalReceipt drives both public cached entries
// to the actual forward pool. Original cached ownership survives a same-address
// sibling, and each dispatched item retains the original source generation.
func TestPublicCachedForwardUsesOriginalReceipt(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "batch"}[batch], func(t *testing.T) {
			f := newOriginalRelayFixture(t)
			_, id := f.cached(t)
			sibling := NewPeer(&PeerSettings{Address: f.source.settings.Address, Port: 2179, Name: "sibling", LocalAS: 65000, PeerAS: 65199})
			sourceSessionFixture(t, sibling)
			f.api.r.peers[sibling.settings.PeerKey()] = sibling
			var err error
			if batch {
				err = f.api.ForwardUpdatesDirect([]uint64{id}, []netip.AddrPort{f.target.settings.PeerKey()}, "", plugin.OperatorSender())
			} else {
				err = f.api.ForwardUpdate(selector.Addr(f.target.settings.Address), id, "", plugin.OperatorSender())
			}
			require.NoError(t, err)
			select {
			case <-f.done:
			case <-time.After(5 * time.Second):
				t.Fatal("original cached forward did not complete")
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			require.NotEmpty(t, *f.items)
			for _, item := range *f.items {
				require.Same(t, f.source, item.receivedPeer)
				require.Equal(t, f.source.forwardGeneration.Load(), item.receivedGeneration)
			}
		})
	}
}

// TestOriginalRelayRefusesPartialAndSuccessorFacts keeps an old receipt alive
// through a genuine selecting lease, then replaces the source physical Session.
// Receipt knowledge cannot borrow fresh generation, policy or sibling authority.
func TestOriginalRelayRefusesPartialAndSuccessorFacts(t *testing.T) {
	f := newOriginalRelayFixture(t)
	update, _ := f.cached(t)
	for _, mutate := range []func(*rpc.StoredRoute){
		func(r *rpc.StoredRoute) { r.SourceOwner++ },
		func(r *rpc.StoredRoute) { r.SourceReceipt = 0 },
		func(r *rpc.StoredRoute) { r.SourceConnectionID++ },
		func(r *rpc.StoredRoute) { r.SourcePolicyEpoch++ },
	} {
		bad := f.route
		mutate(&bad)
		_, err := f.api.originalRelaySource(f.source.settings.Address, &bad, plugin.OperatorSender())
		require.Error(t, err)
	}
	owner, _ := sourceReceiptOwner(t)
	lease, err := sourceReceipts.acquire(f.route.SourceReceipt, owner)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, lease.revoke(t.Context())) })
	successor := sourceSessionFixture(t, f.source)
	require.NoError(t, successor.captureSourceReceipt(f.source, &sourceReceipts))
	t.Cleanup(successor.closeSourceReceipt)
	_, err = originalCachedSource(update)
	require.ErrorIs(t, err, errForwardNoSource)
	_, err = f.api.originalRelaySource(f.source.settings.Address, &f.route, plugin.OperatorSender())
	require.ErrorIs(t, err, errRelayNoSource)
	fresh := f.route
	fresh.SourceReceipt = successor.sourceReceiptID()
	fresh.SourceConnectionID = successor.connectionID
	_, err = f.api.originalRelaySource(f.source.settings.Address, &fresh, plugin.OperatorSender())
	require.NoError(t, err)
}

// TestOriginalRelayManualVariantDoesNotAbsorbOwnedInput preserves useful manual
// reconstruction while refusing source receipts or an unprivileged caller.
func TestOriginalRelayManualVariantDoesNotAbsorbOwnedInput(t *testing.T) {
	f := newOriginalRelayFixture(t)
	manual := storedIPv4Route(f.source.addrString)
	source, err := f.api.originalRelaySource(f.source.settings.Address, &manual, plugin.OperatorSender())
	require.NoError(t, err)
	require.True(t, source.ok)
	body, decodeErr := hex.DecodeString(manual.NLRIHex)
	require.NoError(t, decodeErr)
	require.NotEmpty(t, body)
	_, err = f.api.originalRelaySource(f.source.settings.Address, &manual, plugin.ProcessSender("unattached"))
	require.Error(t, err)
	manual.SourceOwner = f.source.sourceOwner
	_, err = f.api.originalRelaySource(f.source.settings.Address, &manual, plugin.OperatorSender())
	require.Error(t, err)
}

// TestRelayRunDoesNotMixOriginalPhysicalSources keeps coalescing at Session
// identity. Legal ADD-PATH identifier zero remains a route-level value, not an
// absent original receipt, and does not split otherwise identical rows.
func TestRelayRunDoesNotMixOriginalPhysicalSources(t *testing.T) {
	f := newOriginalRelayFixture(t)
	next := f.route
	next.PathID = 0
	require.Equal(t, 2, relayRunLen([]rpc.StoredRoute{f.route, next}))
	for _, mutate := range []func(*rpc.StoredRoute){func(r *rpc.StoredRoute) { r.SourceOwner++ }, func(r *rpc.StoredRoute) { r.SourceReceipt++ }, func(r *rpc.StoredRoute) { r.SourceConnectionID++ }} {
		next = f.route
		mutate(&next)
		require.Equal(t, 1, relayRunLen([]rpc.StoredRoute{f.route, next}))
	}
}
