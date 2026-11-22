// Design: docs/architecture/behavior/peer-lifecycle.md -- what a refused BGP listener tells the operator
// Related: reactor_startup_test.go -- startupFailingListenerFactory
// Related: reactor_listener_start_window_test.go -- openStartWindow, reactorHasPeer

package reactor

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/privilege"
)

// refusedBind is what net.Listen returns when the kernel refuses the bind
// with errno: the chain the listener factory hands the reactor.
func refusedBind(errno syscall.Errno) error {
	return &net.OpError{Op: "listen", Net: "tcp", Err: os.NewSyscallError("bind", errno)}
}

// privilegedPeer is a passive peer named upstream1 whose listener is
// 127.0.0.1 on the BGP port, which the kernel's default unprivileged port
// start (1024) makes privileged.
func privilegedPeer() *PeerSettings {
	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.2"), 65000, 65001, 0x01020304)
	settings.Name = "upstream1"
	settings.Connection = ConnectionPassive
	settings.LocalAddress = netip.MustParseAddr("127.0.0.1")
	settings.LocalPort = DefaultBGPPort
	return settings
}

// assertCapabilityRefusal checks the refusal of privilegedPeer's listener
// refused with EACCES: the address, port and owner, the errno, and the
// capability with its remedy wherever the kernel's start makes port 179
// privileged. The expectation follows the host's own start, read with the
// producer's reader, so the test holds on a host that lowered it (a
// container runtime often sets 0) as on one that did not.
func assertCapabilityRefusal(t *testing.T, err error) {
	t.Helper()
	var refusal *opererr.Error
	require.ErrorAs(t, err, &refusal, "the listener refusal MUST be an operator error, got %v", err)
	assert.Contains(t, refusal.Problem(), "127.0.0.1:179", "the refusal MUST name the configured address and port")
	assert.Contains(t, refusal.Problem(), "bgp peer upstream1", "the refusal MUST name the peer that configured the listener")
	assert.ErrorIs(t, err, syscall.EACCES, "the kernel's answer MUST stay reachable as the evidence")

	start, readErr := network.UnprivilegedPortStart()
	if readErr == nil && start <= DefaultBGPPort {
		assert.NotContains(t, refusal.Why(), "only a process that holds CAP_NET_BIND_SERVICE",
			"port 179 is not privileged on this host, so the capability is not the cause")
		return
	}
	assert.Contains(t, refusal.Why(), "CAP_NET_BIND_SERVICE", "a privileged port refused MUST name the capability")
	assert.Equal(t, privilege.CapabilityRemedy("CAP_NET_BIND_SERVICE"), refusal.Fix(),
		"the fix MUST be the capability remedy")
}

// TestStartListenerRefusalNamesAddressPortOwnerAndCapability checks what the
// BGP start answers when the kernel refuses a configured listener.
//
// Method: a passive peer on 127.0.0.1:179 is added before the start, and the
// listener factory answers EACCES as an unprivileged process gets for port
// 179. StartWithContext MUST answer the operator error naming the address,
// port, peer, CAP_NET_BIND_SERVICE and the remedy. No socket is bound, so the
// test holds when it runs as root.
//
// VALIDATES: the start route names what refused and how to fix it.
// PREVENTS: "bgp: start reactor: listen on 127.0.0.1:179: listen tcp
// 127.0.0.1:179: bind: permission denied" with no capability and no fix,
// now that the generic start-up capability warning is gone.
func TestStartListenerRefusalNamesAddressPortOwnerAndCapability(t *testing.T) {
	r := New(&Config{Standalone: true})
	settings := privilegedPeer()
	require.NoError(t, r.AddPeer(settings))
	require.Equal(t, int(DefaultBGPPort), r.peerListenPort(settings), "prerequisite: the peer listens on the BGP port")
	r.SetListenerFactory(startupFailingListenerFactory{err: refusedBind(syscall.EACCES)})

	err := r.StartWithContext(context.Background())
	require.Error(t, err, "a start whose listener the kernel refused MUST fail")
	assertCapabilityRefusal(t, err)
	assert.False(t, r.Running(), "a start whose listener failed MUST NOT mark the reactor running")
}

// TestAddPeerListenerRefusalNamesAddressPortOwnerAndCapability checks the
// reload route: AddPeer on a started reactor, as a reload, a config operation
// or an API call reaches it.
//
// Method: the reactor starts with no peer, held in its start window where
// AddPeer opens the peer's listener at once. The factory answers EACCES.
// AddPeer MUST answer the same operator error the start answers, and leave no
// peer behind.
//
// VALIDATES: every route reports one refusal (startListenerForAddressPort).
// PREVENTS: a reload that refuses a peer with the bare errno.
func TestAddPeerListenerRefusalNamesAddressPortOwnerAndCapability(t *testing.T) {
	r := New(&Config{Standalone: true})
	r.SetListenerFactory(startupFailingListenerFactory{err: refusedBind(syscall.EACCES)})
	w := openStartWindow(t, r)

	settings := privilegedPeer()
	addErr := r.AddPeer(settings)
	require.Error(t, addErr, "AddPeer MUST refuse a peer whose listener the kernel refused")
	assertCapabilityRefusal(t, addErr)
	assert.False(t, reactorHasPeer(r, settings), "the refused peer MUST NOT stay in the peer set")

	w.release(t)
}

// TestAddPeerListenerAddressInUseKeepsItsOwnFix checks that a refusal other
// than permission does not name the capability.
//
// Method: a socket holds a free port on 127.0.0.1; inside the start window
// AddPeer adds a passive peer on it. The refusal MUST name the address and
// the command that finds the holder, and MUST NOT name CAP_NET_BIND_SERVICE.
//
// VALIDATES: EADDRINUSE keeps its own cause and fix on the BGP route.
// PREVENTS: a capability remedy for a port another program holds.
func TestAddPeerListenerAddressInUseKeepsItsOwnFix(t *testing.T) {
	r := New(&Config{Standalone: true})
	w := openStartWindow(t, r)

	listenPort := freePort(t)
	endpoint := startWindowEndpoint(listenPort)
	var lc net.ListenConfig
	occupier, err := lc.Listen(context.Background(), "tcp", endpoint)
	require.NoError(t, err)
	t.Cleanup(func() { _ = occupier.Close() })

	addErr := r.AddPeer(startWindowPeer("127.0.0.2", listenPort))
	var refusal *opererr.Error
	require.ErrorAs(t, addErr, &refusal)
	assert.Contains(t, refusal.Problem(), endpoint)
	assert.Contains(t, refusal.Fix(), "ss -ltnp", "an address in use MUST name the command that finds its holder")
	assert.NotContains(t, refusal.Error(), "CAP_NET_BIND_SERVICE")
	assert.True(t, errors.Is(addErr, syscall.EADDRINUSE))

	w.release(t)
}

// TestListenOwnerNamesTheClaimants checks the owner a refusal names when
// several claimants share one listener, and when none does.
//
// Method: two passive peers and a dynamic group claim 127.0.0.1:1179, and an
// active peer on the same address does not. listenOwner MUST name the first
// claimant in sort order and count the others, ignore the active peer, and
// answer "bgp" for an endpoint nobody claims.
//
// VALIDATES: the owner is deterministic and names configuration items.
// PREVENTS: a refusal naming a different peer each time (map order), or a
// blank owner.
func TestListenOwnerNamesTheClaimants(t *testing.T) {
	r := New(&Config{Standalone: true})
	local := netip.MustParseAddr("127.0.0.1")
	const port = 1179
	claimant := func(peer, name string, mode ConnectionMode) *PeerSettings {
		s := NewPeerSettings(netip.MustParseAddr(peer), 65000, 65001, 0x01020304)
		s.Name = name
		s.Connection = mode
		s.LocalAddress = local
		s.LocalPort = port
		return s
	}
	require.NoError(t, r.AddPeer(claimant("127.0.0.3", "b-peer", ConnectionPassive)))
	require.NoError(t, r.AddPeer(claimant("127.0.0.4", "a-peer", ConnectionPassive)))
	require.NoError(t, r.AddPeer(claimant("127.0.0.5", "dialer", ConnectionActive)))

	r.mu.RLock()
	assert.Equal(t, "bgp peer a-peer and 1 more", r.listenOwner(local, port))
	assert.Equal(t, listenOwnerUnknown, r.listenOwner(local, port+1), "an endpoint nobody claims MUST still name the subsystem")
	r.mu.RUnlock()

	r.mu.Lock()
	r.dynamicGroups = []*DynamicGroupConfig{{GroupName: "rs-clients", Settings: claimant("127.0.0.6", "", ConnectionPassive)}}
	got := r.listenOwner(local, port)
	r.mu.Unlock()
	assert.Equal(t, "bgp group rs-clients and 2 more", got)
}
