package engine

import (
	"errors"
	"log/slog"
	"net"
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
)

// applyTestState builds an engine state with its own SA table and peer map, and points
// the IKE port override at a free high port so the sockets bind unprivileged. It also
// makes ikeListenHost honor a peer's local-address, which is what lets a test move the
// listen address without touching an interface.
//
// Every socket and session the test leaves behind is released on cleanup, in the order
// a rebind uses: the peers first, then the sockets they hold.
func applyTestState(t *testing.T) *ikeEngineState {
	t.Helper()

	var listener net.ListenConfig
	probe, err := listener.ListenPacket(t.Context(), "udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve a port: %v", err)
	}
	bound, ok := probe.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("a udp listener reported a %T local address", probe.LocalAddr())
	}
	port := bound.Port
	if err := probe.Close(); err != nil {
		t.Fatalf("release the reserved port: %v", err)
	}

	previous := ikeTestPortFn
	ikeTestPortFn = func() string { return strconv.Itoa(port) }
	t.Cleanup(func() { ikeTestPortFn = previous })

	state := &ikeEngineState{
		table:        NewSATable(),
		peers:        make(map[string]*PeerSession),
		log:          slog.Default(),
		installedSPD: map[string]ipsec.SPDPolicy{},
	}
	t.Cleanup(func() {
		state.stopAllPeers()
		state.listeners.closeSockets(state.log)
	})
	return state
}

// applyTestPeer is testPeer with the local address the test wants and a remote address on
// the loopback network, so a session that starts sends its IKE_SA_INIT nowhere off-box.
func applyTestPeer(localAddress string) ipsec.SiteToSitePeer {
	peer := testPeer()
	peer.LocalAddress = localAddress
	peer.RemoteAddress = "127.0.0.9"
	return peer
}

// sendOn reports the error a raw send on tr draws, which is transport.ErrClosed once the
// socket is closed. It is how a test tells a socket that was really closed from one that
// was merely dropped from the pair.
func sendOn(tr *transport.UDPTransport) error {
	return tr.Send([]byte{0}, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 9), Port: 1})
}

// VALIDATES: an operator who moves the address ze listens on gets sockets at the new
// address on the commit that says so, rather than on the next daemon restart.
//
// Reload is driven through ikeEngineState.applyConfig, which is what OnConfigApply calls
// (staging.commit, register.go), so the test exercises the decision the operator's commit
// reaches rather than a helper under it.
//
// RED before the fix: both sockets were created only while the pointer was nil, so the
// second apply left the pair bound to 0.0.0.0 and the peers restarted onto it.
func TestReloadRebindsTheListenSocketsWhenTheAddressChanges(t *testing.T) {
	state := applyTestState(t)

	if err := state.applyConfig(testIPsecConfig(applyTestPeer("")), applyReload); err != nil {
		t.Fatalf("setup apply: %v", err)
	}
	if state.listeners.host != "0.0.0.0" {
		t.Fatalf("setup: bound at %q, want the wildcard", state.listeners.host)
	}
	oldIKE, oldNATT := state.listeners.ike, state.listeners.natt
	if oldIKE == nil || oldNATT == nil {
		t.Fatalf("setup: sockets not bound (ike=%v natt=%v)", oldIKE, oldNATT)
	}

	if err := state.applyConfig(testIPsecConfig(applyTestPeer("127.0.0.1")), applyReload); err != nil {
		t.Fatalf("reload apply: %v", err)
	}

	if state.listeners.host != "127.0.0.1" {
		t.Fatalf("after the reload the pair is bound at %q, want 127.0.0.1", state.listeners.host)
	}
	if state.listeners.ike == nil || state.listeners.ike == oldIKE {
		t.Fatal("the IKE socket was not rebound: ze still receives at the address the operator replaced")
	}
	if state.listeners.natt == nil || state.listeners.natt == oldNATT {
		t.Fatal("the NAT-T socket was not rebound")
	}
	if err := sendOn(oldIKE); !errors.Is(err, transport.ErrClosed) {
		t.Fatalf("the old IKE socket is still open: send returned %v", err)
	}
	if err := sendOn(oldNATT); !errors.Is(err, transport.ErrClosed) {
		t.Fatalf("the old NAT-T socket is still open: send returned %v", err)
	}
}

// VALIDATES: the ORDER of a rebind. A peer whose OWN configuration did not change is
// still stopped and restarted when the socket under it moves, so no running session is
// left holding a closed file descriptor.
//
// This is the case the order exists for, and `peerConfigChanged` cannot reach it: the
// operator edited `vpn ipsec interface`, every peer carries its own local-address, and
// the peer half of the configuration compares equal. PeerSession.ike is immutable after
// startPeerSession, so a session that survives the rebind can never send again.
//
// The interface lookup is stubbed because the box running this test has one address to
// bind. The two values it answers are both bindable, and the peer takes neither: its own
// local-address is what it binds to.
//
// RED before the fix: no rebind happened at all, so the pair stayed at the wildcard. With
// a rebind that closes the sockets and leaves the peers running, the surviving session
// holds the closed socket and the two pointer checks below fail.
func TestReloadRestartsAnUneditedPeerWhoseSocketMoved(t *testing.T) {
	state := applyTestState(t)

	ifAddr := "0.0.0.0"
	previous := resolveInterfaceAddrFn
	resolveInterfaceAddrFn = func(string) (string, error) { return ifAddr, nil }
	t.Cleanup(func() { resolveInterfaceAddrFn = previous })

	running := testIPsecConfig(applyTestPeer("127.0.0.1"))
	running.Interface = "ze-test-if0"
	if err := state.applyConfig(running, applyReload); err != nil {
		t.Fatalf("setup apply: %v", err)
	}
	oldIKE := state.listeners.ike
	first := state.peers["test-peer"]
	if first == nil {
		t.Fatal("setup: the peer did not start")
	}
	if first.ike != oldIKE {
		t.Fatal("setup: the session does not hold the socket the apply bound")
	}

	// Only the interface moved. The peer block is byte-for-byte the one already running.
	ifAddr = "127.0.0.1"
	moved := testIPsecConfig(applyTestPeer("127.0.0.1"))
	moved.Interface = "ze-test-if0"
	if peerConfigChanged(first, moved.Peers["test-peer"], moved.IKEGroups["test-ike"], moved.ESPGroups["test-esp"]) {
		t.Fatal("setup: the peer block changed, so this is not the unedited-peer case")
	}
	if err := state.applyConfig(moved, applyReload); err != nil {
		t.Fatalf("reload apply: %v", err)
	}

	second := state.peers["test-peer"]
	if second == nil {
		t.Fatal("the peer did not restart after the rebind")
	}
	if second == first {
		t.Fatal("the peer survived the rebind, so it holds a closed socket")
	}
	if second.ike != state.listeners.ike {
		t.Fatal("the restarted peer does not hold the socket the rebind opened")
	}
	if second.natt != state.listeners.natt {
		t.Fatal("the restarted peer does not hold the NAT-T socket the rebind opened")
	}
	select {
	case <-first.done:
	default:
		t.Fatal("the old session is still running on the closed socket")
	}
}

// VALIDATES: a reload that does not move the listen address does not bounce a tunnel. The
// rebind is keyed on the address and on nothing else, so an unrelated edit leaves both
// sockets and every unedited session in place.
func TestReloadKeepsTheSocketsWhenTheAddressIsUnchanged(t *testing.T) {
	state := applyTestState(t)

	cfg := testIPsecConfig(applyTestPeer("127.0.0.1"))
	if err := state.applyConfig(cfg, applyReload); err != nil {
		t.Fatalf("setup apply: %v", err)
	}
	ike, natt := state.listeners.ike, state.listeners.natt
	session := state.peers["test-peer"]

	if err := state.applyConfig(testIPsecConfig(applyTestPeer("127.0.0.1")), applyReload); err != nil {
		t.Fatalf("second apply: %v", err)
	}

	if state.listeners.ike != ike || state.listeners.natt != natt {
		t.Fatal("an edit that did not move the listen address rebound the sockets")
	}
	if state.peers["test-peer"] != session {
		t.Fatal("an edit that changed nothing restarted the peer")
	}
}

// VALIDATES: an operator who edits the remote-access pool gets a pool built from what
// they wrote. The comparison is over the whole ipsec.VirtualIPPool, so an edit to the DNS
// servers alone rebuilds it too.
//
// RED before the fix: the pool was built only while the pointer was nil, so the second
// apply kept the range the daemon started with and allocated 10.10.0.1 forever.
func TestReloadRebuildsTheVirtualIPPoolWhenTheOperatorEditsIt(t *testing.T) {
	state := applyTestState(t)

	cfg := testIPsecConfig()
	cfg.RemoteAccess = &ipsec.RemoteAccessConfig{
		Pool: ipsec.VirtualIPPool{Name: "clients", Range: "10.10.0.0/24", DNS: []string{"10.10.0.53"}},
	}
	if err := state.applyConfig(cfg, applyReload); err != nil {
		t.Fatalf("setup apply: %v", err)
	}
	if state.pool == nil {
		t.Fatal("setup: no pool was built")
	}

	edited := testIPsecConfig()
	edited.RemoteAccess = &ipsec.RemoteAccessConfig{
		Pool: ipsec.VirtualIPPool{Name: "clients", Range: "10.20.0.0/24", DNS: []string{"10.10.0.53"}},
	}
	if err := state.applyConfig(edited, applyReload); err != nil {
		t.Fatalf("reload apply: %v", err)
	}

	allocated, err := state.pool.Allocate()
	if err != nil {
		t.Fatalf("allocate from the reloaded pool: %v", err)
	}
	_, want, err := net.ParseCIDR("10.20.0.0/24")
	if err != nil {
		t.Fatalf("parse the edited range: %v", err)
	}
	if !want.Contains(allocated.IPv4) {
		t.Fatalf("the pool allocated %s, which is outside the edited range %s", allocated.IPv4, want)
	}

	// The DNS edit alone must rebuild it too: a client is pushed the resolver, so a pool
	// compared on its range alone would keep offering the one the operator replaced.
	resolverEdited := testIPsecConfig()
	resolverEdited.RemoteAccess = &ipsec.RemoteAccessConfig{
		Pool: ipsec.VirtualIPPool{Name: "clients", Range: "10.20.0.0/24", DNS: []string{"10.20.0.53"}},
	}
	rebuilt := state.pool
	if err := state.applyConfig(resolverEdited, applyReload); err != nil {
		t.Fatalf("resolver reload apply: %v", err)
	}
	if state.pool == rebuilt {
		t.Fatal("an edit to the pool's DNS servers alone did not rebuild the pool")
	}
}

// VALIDATES: a reload the engine refuses changes nothing about the running engine. The
// transaction rolls back, so a mutation made before the refusal would survive as a
// configuration the operator was told did not commit.
//
// RED before the fix: setCookieThreshold and installSPDPolicies ran above the refusal, so
// the rejected configuration's cookie threshold was in force when applyConfig returned
// the error.
func TestReloadRefusalLeavesTheRunningEngineUntouched(t *testing.T) {
	state := applyTestState(t)

	running := testIPsecConfig(applyTestPeer("127.0.0.1"))
	running.CookieThreshold = 7
	if err := state.applyConfig(running, applyReload); err != nil {
		t.Fatalf("setup apply: %v", err)
	}
	session := state.peers["test-peer"]
	sockets := state.listeners

	// A peer with no local-address of its own, over an interface no box has: the lookup
	// fails, so every such peer is unbindable and a reload MUST refuse the whole
	// configuration (unbindablePeers).
	refused := testIPsecConfig(applyTestPeer(""))
	refused.Interface = "ze-no-such-interface0"
	refused.CookieThreshold = 99
	refused.Policies = map[string]ipsec.SPDPolicy{"drop-rfc1918": {Name: "drop-rfc1918"}}

	if err := state.applyConfig(refused, applyReload); err == nil {
		t.Fatal("a reload whose peers cannot bind was accepted")
	}

	if got := cookieThreshold.Load(); got != 7 {
		t.Fatalf("the refused configuration's cookie threshold is in force: %d, want 7", got)
	}
	if len(state.installedSPD) != 0 {
		t.Fatalf("the refused configuration's SPD entries were installed: %v", state.installedSPD)
	}
	if state.peers["test-peer"] != session {
		t.Fatal("a refused reload restarted the running peer")
	}
	if state.listeners != sockets {
		t.Fatal("a refused reload moved the sockets")
	}
}
