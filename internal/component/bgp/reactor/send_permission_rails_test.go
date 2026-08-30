// Design: docs/architecture/api/architecture.md -- the send half of an attach block
// Related: send_permission.go -- filterPermittedPeers, the one guard every rail shares
// Related: send_permission_test.go -- the same guard over the six rails that
//   resolve a peer selector through getMatchingPeersSel
//
// The four rails here name their peers WITHOUT that resolver: `cache forward`
// matches a selector of its own inside ForwardUpdate, and forward-cached,
// relay-stored-route and `peer <addr> raw` address peers directly. They were the
// four with no permission at all until round 1 of the Review Gate, so each one
// gets a REFUSAL driven from the entry point a process reaches, beside an
// acceptance that differs only in the process name.

package reactor

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// The cache plugin's command grammar, which the daemon gets from the
	// composition root (plugin/all/all_ze_bgp.go). Without it the dispatcher has
	// the ze-bgp:cache-forward wire method and no `request cache forward` alias,
	// so the entry point this file drives would not exist in the test binary.
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/cache/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	// railGranted is attached to the destination peer with `send [ update ]`, so
	// every rail must serve it.
	railGranted = "relay"
	// railRefused is attached to nothing, so every rail must refuse it. It is a
	// real process name rather than the zero Sender: an unset sender is refused
	// one branch earlier, before any peer is read, so a test built on it would
	// never reach the attach block at all.
	railRefused = "stranger"
	// railOther is attached with a send type the rail under test does not ask
	// for, so it separates "attached" from "permitted". The raw rail needs it:
	// raw used to be gated on attachment alone, so an attached-but-unpermitted
	// process is exactly the case that changed.
	railOther = "bystander"
	// railDest is the destination peer every rail below addresses.
	railDest = "10.0.0.2"
)

// railEnv is the forward-rail fixture: an established source, an established
// destination that attaches railGranted, and a capturing forward pool. The pool
// is the destination's wire on these three rails, so "nothing was sent" is
// "nothing was dispatched".
type railEnv struct {
	api   *reactorAPIAdapter
	cache *RecentUpdateCache

	mu         sync.Mutex
	dispatched []fwdItem
	saw        chan struct{}
}

// items returns a snapshot of everything the pool has dispatched.
func (e *railEnv) items() []fwdItem {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]fwdItem(nil), e.dispatched...)
}

// awaitDispatch waits for one pool dispatch, so an acceptance is judged on the
// item that arrived rather than on the absence of an error.
func (e *railEnv) awaitDispatch(t *testing.T) {
	t.Helper()
	select {
	case <-e.saw:
	case <-time.After(2 * time.Second):
		t.Fatal("the permitted process never reached the forward pool")
	}
}

// railFixture builds the two-peer topology the forward rails need. EBGP on both
// sides, so RFC 4456 reflection rules never suppress the destination and the
// only thing that can stop a forward is the permission.
func railFixture(t *testing.T) *railEnv {
	t.Helper()

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	src := makeRSPeer(t, forwardSourceAddr, 65001, ctx, ctxID)
	src.recvCtxID = ctxID
	dst := makeRSPeer(t, railDest, 65002, ctx, ctxID)
	dst.settings.ProcessBindings = []ProcessBinding{sendUpdateOnly(railGranted)}

	env := &railEnv{saw: make(chan struct{}, 8)}
	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		env.mu.Lock()
		env.dispatched = append(env.dispatched, items...)
		env.mu.Unlock()
		for i := range items {
			if items[i].done != nil {
				items[i].done()
			}
			env.saw <- struct{}{}
		}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	env.cache = newRecentUpdateCache(100)
	t.Cleanup(env.cache.Stop)

	r := &Reactor{
		config:          &Config{LocalAS: 65000},
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   env.cache,
		clock:           clock.RealClock{},
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			dst.Settings().PeerKey(): dst,
		},
		fwdPool: pool,
	}
	env.api = &reactorAPIAdapter{r: r}
	return env
}

// railServer wires a real plugin server over one reactor adapter, which is how
// the daemon builds it (reactor.go, StartWithContext), and returns a command
// context carrying the given sender.
func railServer(t *testing.T, api *reactorAPIAdapter, sender plugin.Sender) *pluginserver.CommandContext {
	t.Helper()
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, api)
	require.NoError(t, err)
	return &pluginserver.CommandContext{Server: srv, Sender: sender}
}

// TestCacheForwardEntryPointRefusesAnUnattachedProcess drives `request cache
// forward` through the registered command, not through ForwardUpdate.
//
// The entry point is what makes this evidence. Round 1 found four rails
// unguarded because their tests called the reactor method while the traffic
// arrived through a registered command, and a test that skips the dispatch chain
// cannot see a handler that drops ctx.Sender on the way.
//
// VALIDATES: AC-9 and AC-10 on the `ze-bgp:cache-forward` rail -- a process the
// destination does not attach is refused, and the UPDATE reaches no wire.
// PREVENTS: any connected process forwarding a cached UPDATE into a peer that
// never attached it, which is route injection under another command's name.
func TestCacheForwardEntryPointRefusesAnUnattachedProcess(t *testing.T) {
	resetSendPermissionMetrics(t)
	env := railFixture(t)

	const refusedID, grantedID = uint64(41), uint64(42)
	cacheReflectableUpdate(t, env.cache, refusedID)
	cacheReflectableUpdate(t, env.cache, grantedID)

	refused := railServer(t, env.api, plugin.ProcessSender(railRefused))
	resp, err := refused.Server.Dispatcher().Dispatch(refused, "request cache forward 41 "+railDest)
	require.ErrorIs(t, err, errSendNotPermitted, "the destination attaches no such process")
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, railRefused, "the refusal must name the process")
	assert.Empty(t, env.items(), "a refused forward must put nothing on the destination's wire")

	// The same command, the same destination, the same cached UPDATE: only the
	// process name differs, so what is measured is the attach block and nothing
	// else.
	granted := railServer(t, env.api, plugin.ProcessSender(railGranted))
	resp, err = granted.Server.Dispatcher().Dispatch(granted, "request cache forward 42 "+railDest)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	env.awaitDispatch(t)
	require.Len(t, env.items(), 1, "the attached process must be served")
}

// TestPeerRawEntryPointNeedsTheRawSendWord drives `peer <addr> raw` through the
// registered command.
//
// Raw is gated on `send [ raw ]`, the word the owner added to the send
// vocabulary on 2026-08-30. Until then it was gated on ATTACHMENT alone, and
// this test pinned that: a process attached with `send [ update ]` reached the
// socket. The ruling overturned the rule, so the case it used to accept is the
// sharp negative here.
//
// VALIDATES: the raw rail refuses a process the peer does not attach AND a
// process attached with another send type, and the bytes reach no socket in
// either case.
// PREVENTS: a connected process writing a BGP message of its own choosing, a
// forged NOTIFICATION included, into a session that granted it routes only.
func TestPeerRawEntryPointNeedsTheRawSendWord(t *testing.T) {
	resetSendPermissionMetrics(t)

	peer, conn := newAttachedPeer(t, railDest, sendUpdateOnly(railOther), sendRawOnly(railGranted))
	api := newSendPermissionReactor(peer)

	refused := railServer(t, api, plugin.ProcessSender(railRefused))
	resp, err := refused.Server.Dispatcher().Dispatch(refused, "peer "+railDest+" raw update hex DEADBEEF")
	require.ErrorIs(t, err, errSendNotPermitted)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Empty(t, conn.written(), "a refused raw injection must put nothing on the peer's socket")

	updateOnly := railServer(t, api, plugin.ProcessSender(railOther))
	resp, err = updateOnly.Server.Dispatcher().Dispatch(updateOnly, "peer "+railDest+" raw update hex DEADBEEF")
	require.ErrorIs(t, err, errSendNotPermitted,
		"`send [ update ]` permits routes ze builds, never a message the process builds itself")
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Empty(t, conn.written(), "an attached process without `send [ raw ]` must reach no socket")

	granted := railServer(t, api, plugin.ProcessSender(railGranted))
	resp, err = granted.Server.Dispatcher().Dispatch(granted, "peer "+railDest+" raw update hex DEADBEEF")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.NotEmpty(t, conn.written(), "the process the peer grants `send [ raw ]` must reach the socket")
}

// TestForwardCachedRailRefusesAnUnattachedProcess is the forward-cached rail,
// the route server's per-flush fast path.
//
// The plugin server's own entry point for it is Server.forwardCached
// (plugin/server/dispatch_cached.go), which cannot be called from this package:
// the plugin server imports the reactor, so the dependency runs one way only.
// What that entry point owes is the process NAME, and
// TestCachedRailsNameTheProcessAsTheSender pins it there. What this test owes is
// the answer the guard gives that name, and it is driven with the ProcessSender
// the entry point builds.
//
// VALIDATES: a destination that does not attach the process is refused, the
// whole batch fails, and nothing is dispatched.
// PREVENTS: a cache consumer flushing UPDATEs into any peer address it can name.
func TestForwardCachedRailRefusesAnUnattachedProcess(t *testing.T) {
	resetSendPermissionMetrics(t)
	env := railFixture(t)

	const refusedID, grantedID = uint64(51), uint64(52)
	cacheReflectableUpdate(t, env.cache, refusedID)
	cacheReflectableUpdate(t, env.cache, grantedID)
	dst := []netip.AddrPort{netip.AddrPortFrom(netip.MustParseAddr(railDest), 0)}

	err := env.api.ForwardUpdatesDirect([]uint64{refusedID}, dst, railRefused, plugin.ProcessSender(railRefused))
	require.ErrorIs(t, err, errSendNotPermitted)
	assert.Contains(t, err.Error(), railRefused, "the refusal must name the process")
	assert.Empty(t, env.items(), "a refused batch must put nothing on the destination's wire")

	require.NoError(t, env.api.ForwardUpdatesDirect([]uint64{grantedID}, dst, railGranted, plugin.ProcessSender(railGranted)))
	env.awaitDispatch(t)
	require.Len(t, env.items(), 1, "the attached process must be served")
}

// TestRelayStoredRouteRailRefusesAnUnattachedProcess is the relay rail, which a
// plugin uses to replay its stored routes into a peer that has just come up.
//
// Its plugin-server entry point is Server.opRelayStoredRoute, unreachable from
// here for the reason TestForwardCachedRailRefusesAnUnattachedProcess states,
// and pinned there by TestCachedRailsNameTheProcessAsTheSender.
//
// VALIDATES: a destination that does not attach the process is refused and no
// stored route reaches the forward pool.
// PREVENTS: a plugin replaying routes it supplies into a peer that never
// attached it -- the destination and the route bytes both come from the caller
// on this rail.
func TestRelayStoredRouteRailRefusesAnUnattachedProcess(t *testing.T) {
	resetSendPermissionMetrics(t)
	env := railFixture(t)

	dst := netip.MustParseAddr(railDest)
	routes := []rpc.StoredRoute{storedIPv4Route(forwardSourceAddr)}

	err := env.api.RelayStoredRoute(dst, routes, plugin.ProcessSender(railRefused))
	require.ErrorIs(t, err, errSendNotPermitted)
	assert.Contains(t, err.Error(), railRefused, "the refusal must name the process")
	assert.Empty(t, env.items(), "a refused relay must put nothing on the destination's wire")

	require.NoError(t, env.api.RelayStoredRoute(dst, routes, plugin.ProcessSender(railGranted)))
	env.awaitDispatch(t)
	require.Len(t, env.items(), 1, "the attached process must be served")
}

// TestRailsRefuseACommandWithNoSender pins the branch these four rails take
// BEFORE any peer is read, which no test above can reach: every case there names
// a real process, because an unset sender is refused one branch earlier
// (railRefused).
//
// It is driven at the reactor rather than at a registered command, because the
// state it describes is a dispatch path that failed to name its sender: every
// entry point in the tree names one, so the command grammar cannot produce it.
// What is pinned is the answer the rails owe when a future one forgets.
//
// VALIDATES: cache forward, forward-cached, relay-stored-route and raw each
// refuse the zero plugin.Sender with errSendNoSender, report it under the
// bounded "unset" process label, and dispatch nothing.
// PREVENTS: the branch being dropped and the rails falling through to the attach
// filter, which answers errSendNotPermitted -- a defect in the dispatch path
// reported to an operator as their config mistake, on the rail where raw bytes
// reach a socket. cmd/raw's mock carries its own copy of this rule
// (plugins/cmd/raw/mock_reactor_test.go, SendRawMessage), and a mirror whose
// original is gone goes on passing alone.
func TestRailsRefuseACommandWithNoSender(t *testing.T) {
	resetSendPermissionMetrics(t)
	reg := &announceFakeRegistry{}
	setSendPermissionMetricsRegistry(reg)
	env := railFixture(t)

	const cachedID = uint64(61)
	cacheReflectableUpdate(t, env.cache, cachedID)
	addr := netip.MustParseAddr(railDest)

	var nobody plugin.Sender // the zero value: nobody said who is sending

	err := env.api.ForwardUpdate(selector.All(), cachedID, "", nobody)
	require.ErrorIs(t, err, errSendNoSender, "a cache forward from nobody must be refused")
	assert.Contains(t, err.Error(), "CommandContext.Sender",
		"the refusal must name the field the dispatch path failed to set")

	require.ErrorIs(t,
		env.api.ForwardUpdatesDirect([]uint64{cachedID}, []netip.AddrPort{netip.AddrPortFrom(addr, 0)}, "", nobody),
		errSendNoSender, "a forward-cached batch from nobody must be refused")
	require.ErrorIs(t,
		env.api.RelayStoredRoute(addr, []rpc.StoredRoute{storedIPv4Route(forwardSourceAddr)}, nobody),
		errSendNoSender, "a stored-route relay from nobody must be refused")
	require.ErrorIs(t,
		env.api.SendRawMessage(addr, 2, []byte{0x00, 0x00, 0x00, 0x00}, nobody),
		errSendNoSender, "a raw injection from nobody must be refused")

	assert.Empty(t, env.items(), "a command nobody claimed must reach no destination")

	// Reported, not only refused. The series is keyed on the sender's name, and
	// Sender.String gives an unset sender the bounded value "unset".
	require.NotNil(t, reg.vec, "the refusal must reach the counter")
	assert.Equal(t, 3, reg.vec.counters["unset|update"].n,
		"the three UPDATE rails must move the unset series once each")
	assert.Equal(t, 1, reg.vec.counters["unset|raw"].n,
		"raw is refused under its own send type, which no send list can grant")
}
