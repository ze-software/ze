package engine

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/pkg/ze"
)

// addrBus is an in-process event bus holding only what the address worker uses: it keeps
// the handlers Subscribe was given and calls them on Emit. Every other method of
// ze.EventBus panics through the nil embedded interface, which no caller here reaches.
type addrBus struct {
	ze.EventBus
	mu       sync.Mutex
	handlers map[string][]func(any)
}

func (b *addrBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.handlers == nil {
		b.handlers = map[string][]func(any){}
	}
	b.handlers[namespace+"/"+eventType] = append(b.handlers[namespace+"/"+eventType], handler)
	return func() {}
}

func (b *addrBus) Emit(namespace, eventType string, payload any) (int, error) {
	b.mu.Lock()
	handlers := b.handlers[namespace+"/"+eventType]
	b.mu.Unlock()
	for _, h := range handlers {
		h(payload)
	}
	return len(handlers), nil
}

// fakeInterface is the address resolveInterfaceAddrFn answers for the configured
// interface; the test moves it as the iface monitor would see the kernel move it.
type fakeInterface struct {
	mu   sync.Mutex
	addr string
}

func (f *fakeInterface) set(addr string) {
	f.mu.Lock()
	f.addr = addr
	f.mu.Unlock()
}

// install swaps resolveInterfaceAddrFn for the test's life.
func (f *fakeInterface) install(t *testing.T) {
	t.Helper()
	previous := resolveInterfaceAddrFn
	resolveInterfaceAddrFn = func(string) (string, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.addr, nil
	}
	t.Cleanup(func() { resolveInterfaceAddrFn = previous })
}

// runningPeer returns the session the engine runs for name, or nil.
func runningPeer(state *ikeEngineState, name string) *PeerSession {
	peersMu.RLock()
	defer peersMu.RUnlock()
	return state.peers[name]
}

// waitForPeer waits for the worker to start a session for name, a goroutine the test
// does not drive, and fails the test after five seconds.
func waitForPeer(t *testing.T, state *ikeEngineState, name string) *PeerSession {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ps := runningPeer(state, name); ps != nil {
			return ps
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("peer %s was never started", name)
	return nil
}

// canBind reports whether this host lets a test bind host, which macOS refuses for every
// loopback address but 127.0.0.1.
func canBind(t *testing.T, host string) bool {
	t.Helper()
	var listener net.ListenConfig
	probe, err := listener.ListenPacket(t.Context(), "udp4", host+":0")
	if err != nil {
		return false
	}
	if err := probe.Close(); err != nil {
		t.Fatalf("release probe: %v", err)
	}
	return true
}

// interfaceConfig is a configuration whose peers take their local address from wan0.
func interfaceConfig(peers ...ipsec.SiteToSitePeer) *ipsec.IPsecConfig {
	cfg := testIPsecConfig(peers...)
	cfg.Interface = "wan0"
	return cfg
}

// startDeferred is what OnAllPluginsReady does after a startup apply.
func startDeferred(state *ikeEngineState) {
	state.applyMu.Lock()
	defer state.applyMu.Unlock()
	if state.startupCfg != nil {
		state.reconcile(state.startupCfg)
		state.startupCfg = nil
	}
}

// VALIDATES: AC-1 and AC-2, through the real entry point: a daemon that starts while
// `vpn ipsec interface` has no IPv4 address starts no peer and binds no IKE socket, and
// the `interface/addr-added` event that announces the address starts the peer on a socket
// bound there, with that address as its local address.
//
// Method: a fake bus carries the event to the worker startAddressWorker subscribed, and
// the interface resolver answers what the test sets.
//
// RED before the fix: nothing subscribed to the event and nothing re-read the interface,
// so the peer stayed down, or started with an empty local address, until a restart.
func TestInterfaceAddressArrivalStartsPeers(t *testing.T) {
	state := applyTestState(t)
	wan := &fakeInterface{}
	wan.install(t)
	bus := &addrBus{}
	state.worker = startAddressWorker(state, bus)
	t.Cleanup(func() {
		state.worker.Stop()
		state.worker = nil
	})

	if err := state.applyConfig(interfaceConfig(applyTestPeer("")), applyStartup); err != nil {
		t.Fatalf("startup apply with no interface address: %v", err)
	}
	startDeferred(state)
	if ps := runningPeer(state, "test-peer"); ps != nil {
		t.Fatal("a peer started with no address to send from (AC-1)")
	}
	if state.listeners.ike != nil {
		t.Fatal("an IKE socket is bound while the interface has no address")
	}
	if wait := interfaceWaitPtr.Load(); wait == nil || len(wait.peers) != 1 || wait.peers[0] != "test-peer" {
		t.Fatalf("the doctor is not told the peer waits: %+v", wait)
	}

	wan.set("127.0.0.1")
	if _, err := bus.Emit(ifaceevents.Namespace, ifaceevents.EventAddrAdded, `{"name":"wan0"}`); err != nil {
		t.Fatalf("emit: %v", err)
	}

	ps := waitForPeer(t, state, "test-peer")
	if ps.peerCfg.LocalAddress != "127.0.0.1" {
		t.Fatalf("the peer runs with local-address %q, want the interface address", ps.peerCfg.LocalAddress)
	}
	state.applyMu.Lock()
	defer state.applyMu.Unlock()
	if state.listeners.host != "127.0.0.1" || state.listeners.ike == nil {
		t.Fatalf("listener at %q (ike=%v), want 127.0.0.1", state.listeners.host, state.listeners.ike)
	}
	if ps.ike != state.listeners.ike {
		t.Fatal("the peer does not hold the socket bound at the interface address")
	}
}

// VALIDATES: AC-3: an interface address that changes restarts the peers that took it, on
// a socket at the new address, and closes the old socket.
func TestInterfaceAddressChangeRehomesPeers(t *testing.T) {
	if !canBind(t, "127.0.0.2") {
		t.Skip("this host binds only 127.0.0.1 on loopback (macOS); the move needs a second address")
	}
	state := applyTestState(t)
	wan := &fakeInterface{addr: "127.0.0.1"}
	wan.install(t)

	if err := state.applyConfig(interfaceConfig(applyTestPeer("")), applyReload); err != nil {
		t.Fatalf("apply: %v", err)
	}
	old := runningPeer(state, "test-peer")
	if old == nil {
		t.Fatal("setup: the peer did not start")
	}
	oldIKE := state.listeners.ike

	wan.set("127.0.0.2")
	state.followAddress()

	if !old.stopped() {
		t.Fatal("the session on the old address is still running")
	}
	now := runningPeer(state, "test-peer")
	if now == nil || now == old {
		t.Fatal("no new session was started on the new address")
	}
	if now.peerCfg.LocalAddress != "127.0.0.2" || now.ike != state.listeners.ike || state.listeners.host != "127.0.0.2" {
		t.Fatalf("new session local %q on pair %q", now.peerCfg.LocalAddress, state.listeners.host)
	}
	if err := sendOn(oldIKE); !errors.Is(err, transport.ErrClosed) {
		t.Fatalf("the socket at the old address is still open: %v", err)
	}

	// R-1: a second event with the same address changes nothing.
	state.followAddress()
	if runningPeer(state, "test-peer") != now {
		t.Fatal("an event that moved nothing restarted the peer")
	}
}

// VALIDATES: AC-4: an address that leaves stops the peers that took it and starts none
// until one returns; the socket closes and the doctor names the waiting peer.
func TestInterfaceAddressRemovalStopsPeers(t *testing.T) {
	state := applyTestState(t)
	wan := &fakeInterface{addr: "127.0.0.1"}
	wan.install(t)

	if err := state.applyConfig(interfaceConfig(applyTestPeer("")), applyReload); err != nil {
		t.Fatalf("apply: %v", err)
	}
	old := runningPeer(state, "test-peer")
	if old == nil {
		t.Fatal("setup: the peer did not start")
	}
	oldIKE := state.listeners.ike

	wan.set("")
	state.followAddress()

	if !old.stopped() || runningPeer(state, "test-peer") != nil {
		t.Fatal("the peer still runs after its address left")
	}
	if err := sendOn(oldIKE); !errors.Is(err, transport.ErrClosed) {
		t.Fatalf("the socket at the vanished address is still open: %v", err)
	}
	if wait := interfaceWaitPtr.Load(); wait == nil || len(wait.peers) != 1 {
		t.Fatalf("the doctor is not told the peer waits: %+v", wait)
	}

	wan.set("127.0.0.1")
	state.followAddress()
	if runningPeer(state, "test-peer") == nil {
		t.Fatal("the peer did not start when the address returned")
	}
}

// VALIDATES: AC-5, the listener set: a peer with its own literal local-address, beside
// peers that take the interface address, keeps its session and its socket while the
// interface address leaves and returns.
//
// RED before the listener set: one socket pair served every peer, so the rebind stopped
// every session (stopAllPeers) and closed the literal peer's socket with the others.
func TestLiteralPeerKeepsItsSocketAcrossInterfaceRebind(t *testing.T) {
	if !canBind(t, "127.0.0.2") {
		t.Skip("this host binds only 127.0.0.1 on loopback (macOS); the literal needs a second address")
	}
	state := applyTestState(t)
	wan := &fakeInterface{addr: "127.0.0.1"}
	wan.install(t)

	literal := applyTestPeer("127.0.0.2")
	literal.Name = "literal-peer"
	if err := state.applyConfig(interfaceConfig(applyTestPeer(""), literal), applyReload); err != nil {
		t.Fatalf("apply: %v", err)
	}
	kept := runningPeer(state, "literal-peer")
	if kept == nil || runningPeer(state, "test-peer") == nil {
		t.Fatal("setup: both peers should run")
	}
	keptIKE := kept.ike
	if keptIKE == nil || keptIKE == state.listeners.ike {
		t.Fatal("the literal peer does not hold a socket of its own")
	}

	wan.set("")
	state.followAddress()
	wan.set("127.0.0.1")
	state.followAddress()

	if runningPeer(state, "literal-peer") != kept || kept.stopped() {
		t.Fatal("an interface address event restarted the peer with its own local-address")
	}
	if err := sendOn(keptIKE); err != nil {
		t.Fatalf("the literal peer's socket was closed by the interface rebind: %v", err)
	}
	if runningPeer(state, "test-peer") == nil {
		t.Fatal("the interface peer did not come back with the address")
	}
}

// VALIDATES: AC-8: a route-chosen session whose source left the host, with no MOBIKE, is
// restarted, while one whose SA negotiated MOBIKE is left to the MOBIKE path (AC-7).
func TestRouteChosenAddressChangeWithoutMobikeRestarts(t *testing.T) {
	// The seams are swapped BEFORE the state is built, so they are restored after its
	// cleanup has stopped the session that reads them.
	previousHolds := hostHoldsAddrFn
	hostHoldsAddrFn = func(net.IP) (bool, error) { return false, nil }
	t.Cleanup(func() { hostHoldsAddrFn = previousHolds })
	// The session's own cycles must not replace the SA the test plants, so the route
	// lookup fails and every cycle ends before it builds one.
	//
	// The lookup also holds the session's first cycle until release closes. In production
	// only the session goroutine writes an initiator's ps.sa, and run reads it without the
	// lock (reconnectDelay) once a cycle fails. The test plants the SA from its own
	// goroutine instead, so the session MUST NOT reach that read until the planting is
	// done: closing release orders the two. The lookup cannot see stopCh, so release
	// closes before anything stops the session (Stop waits for run to return).
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseSession := func() { releaseOnce.Do(func() { close(release) }) }
	previousRoute := routeSourceFn
	routeSourceFn = func(net.IP) (net.IP, error) {
		<-release
		return nil, errors.New("no route in this test")
	}
	t.Cleanup(func() { routeSourceFn = previousRoute })
	state := applyTestState(t)
	// Registered after the state, so it runs first, and a test that fails before the
	// release below still lets the state's cleanup stop the session.
	t.Cleanup(releaseSession)

	if err := state.applyConfig(testIPsecConfig(applyTestPeer("")), applyReload); err != nil {
		t.Fatalf("apply: %v", err)
	}
	old := runningPeer(state, "test-peer")
	if old == nil {
		t.Fatal("setup: the peer did not start")
	}
	gone := &SA{PeerName: "test-peer", cycleLocal: net.ParseIP("203.0.113.1"), mobikeUp: 1}
	old.setSA(gone)
	state.applyMu.Lock()
	state.restartVanishedRouteSources()
	state.applyMu.Unlock()
	if runningPeer(state, "test-peer") != old {
		t.Fatal("a MOBIKE SA was restarted; the MOBIKE path owns its move")
	}

	atomic.StoreInt32(&gone.mobikeUp, 0)
	// The stop below waits for the session's run to return, and run is held in the
	// route lookup until release closes.
	releaseSession()
	state.applyMu.Lock()
	state.restartVanishedRouteSources()
	state.applyMu.Unlock()
	if !old.stopped() {
		t.Fatal("the session whose source left the host was not stopped")
	}
	if now := runningPeer(state, "test-peer"); now == nil || now == old {
		t.Fatal("the peer was not started again to pick its new source")
	}
}
