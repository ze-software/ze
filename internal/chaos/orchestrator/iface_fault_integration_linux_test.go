//go:build linux

// Design: docs/architecture/chaos-web-dashboard.md -- end-to-end netns interface fault test

package orchestrator

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/chaos/engine"
	"github.com/ze-software/ze/internal/chaos/guard"
	"github.com/ze-software/ze/internal/chaos/peer"
	"github.com/ze-software/ze/internal/chaos/web"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestIfaceFaultLiveBGPSession drives iface-addr-remove and iface-link-flap
// through runScheduler, newControlDispatcher, and runPeerLoop over a real
// Linux veth pair inside an isolated userns network namespace.
//
// VALIDATES: CLI default ActionParams (--fault-iface / --fault-addr) and
// PeerLocalAddr bind a simulated BGP peer to a veth interface, cycle its
// address and link state via netlink, tear down the live BGP session on
// link-flap, and recover the session automatically.
// PREVENTS: Interface faults silently no-oping when invoked through the
// orchestrator scheduler or leaving the veth down / BGP session unrecovered.
func TestIfaceFaultLiveBGPSession(t *testing.T) {
	if !userns.Enter(t) {
		return
	}

	const (
		serverIface = "ze_cf0"
		peerIface   = "ze_cf1"
		serverCIDR  = "10.242.0.1/24"
		peerCIDR    = "10.242.0.2/24"
		serverIP    = "10.242.0.1"
		peerIP      = "10.242.0.2"
	)

	setupVethPair(t, serverIface, serverCIDR, peerIface, peerCIDR)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", net.JoinHostPort(serverIP, "0"))
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	var acceptedFromPeerIP atomic.Int32
	var srvWG sync.WaitGroup
	srvWG.Go(func() {
		serveBGPPeer(ctx, ln, peerIP, &acceptedFromPeerIP)
	})
	defer func() {
		_ = ln.Close()
		srvWG.Wait()
	}()

	es := newEstablishedState(1)
	g := guard.New(1)
	chaosCh := make(chan engine.ChaosAction, 1)
	channels := []chan engine.ChaosAction{chaosCh}
	controlCh := make(chan web.ControlCommand, 8)
	events := make(chan peer.Event, 256)

	actionParams := buildIfaceFaultParams(peerIface, peerCIDR, 1, 50*time.Millisecond)
	schedDone := make(chan struct{})
	var doneOnce sync.Once
	finishSched := func() { doneOnce.Do(func() { close(schedDone) }) }

	go runScheduler(ctx, ChaosConfig{
		Rate:           0,
		Interval:       50 * time.Millisecond,
		Warmup:         time.Hour,
		EnabledActions: []engine.ActionType{engine.ActionIfaceLinkFlap, engine.ActionIfaceAddrRemove},
		ActionParams:   actionParams,
	}, 42, 1, es, g, channels, controlCh, time.Time{}, true, finishSched)

	simCfg := peer.SimulatorConfig{
		Profile: peer.SimProfile{
			Index:      0,
			ASN:        65001,
			RouterID:   netip.MustParseAddr("10.242.0.2"),
			HoldTime:   90,
			RouteCount: 5,
			TotalPeers: 1,
			Families:   []string{"ipv4/unicast"},
		},
		Seed:      42,
		Addr:      ln.Addr().String(),
		LocalAddr: peerIP,
		Events:    events,
		Chaos:     chaosCh,
		Quiet:     true,
	}

	var peerWG sync.WaitGroup
	peerWG.Go(func() {
		runPeerLoop(ctx, simCfg, 0, events)
	})

	establishedCh := make(chan struct{}, 8)
	disconnectedCh := make(chan struct{}, 8)
	chaosExecCh := make(chan string, 8)
	errCh := make(chan error, 8)

	var evWG sync.WaitGroup
	evWG.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-events:
				switch ev.Type {
				case peer.EventEstablished:
					es.Set(ev.PeerIndex, true)
					g.OnEstablished(ev.PeerIndex)
					select {
					case establishedCh <- struct{}{}:
					default:
					}
				case peer.EventDisconnected:
					es.Set(ev.PeerIndex, false)
					g.OnDisconnected(ev.PeerIndex)
					select {
					case disconnectedCh <- struct{}{}:
					default:
					}
				case peer.EventChaosExecuted:
					select {
					case chaosExecCh <- ev.ChaosAction:
					default:
					}
				case peer.EventError:
					select {
					case errCh <- ev.Err:
					default:
					}
				case peer.EventRouteSent, peer.EventRouteReceived, peer.EventRouteWithdrawn,
					peer.EventEORSent, peer.EventReconnecting, peer.EventWithdrawalSent,
					peer.EventDroppedEvents, peer.EventRouteAction:
				}
			}
		}
	})

	defer func() {
		cancel()
		<-schedDone
		peerWG.Wait()
		evWG.Wait()
	}()

	// Wait for the initial BGP session to establish over the veth pair.
	waitSignal(t, ctx, establishedCh, errCh, "initial BGP establishment over veth")
	assert.GreaterOrEqual(t, acceptedFromPeerIP.Load(), int32(1), "peer must bind source IP %s", peerIP)

	dispatch := newControlDispatcher(ctx, controlCh, schedDone, nil)

	// Trigger iface-addr-remove without explicit params: inherits CLI defaults.
	addrRes, err := dispatch(web.ControlCommand{
		Type: ctrlTypeTrigger,
		Trigger: &web.ManualTrigger{
			ActionType: engine.NameIfaceAddrRemove,
			Peers:      []int{0},
		},
	})
	require.NoError(t, err)
	require.NoError(t, addrRes.Err)
	assert.Equal(t, []int{0}, addrRes.Dispatched)

	execAction := waitAction(t, ctx, chaosExecCh, errCh, "iface-addr-remove execution")
	assert.Equal(t, engine.NameIfaceAddrRemove, execAction)
	assert.True(t, vethHasAddr(t, peerIface, peerCIDR), "address must be restored after iface-addr-remove")

	// Trigger iface-link-flap without explicit params: tears down and recovers the live session.
	flapRes, err := dispatch(web.ControlCommand{
		Type: ctrlTypeTrigger,
		Trigger: &web.ManualTrigger{
			ActionType: engine.NameIfaceLinkFlap,
			Peers:      []int{0},
		},
	})
	require.NoError(t, err)
	require.NoError(t, flapRes.Err)
	assert.Equal(t, []int{0}, flapRes.Dispatched)

	execAction = waitAction(t, ctx, chaosExecCh, errCh, "iface-link-flap execution")
	assert.Equal(t, engine.NameIfaceLinkFlap, execAction)

	waitSignal(t, ctx, disconnectedCh, errCh, "BGP disconnect after iface-link-flap")
	waitSignal(t, ctx, establishedCh, errCh, "BGP reconnect and re-establishment after iface-link-flap")

	assert.True(t, vethIsUp(t, peerIface), "interface %s must be back UP after link flap", peerIface)
	assert.GreaterOrEqual(t, acceptedFromPeerIP.Load(), int32(2), "expected peer reconnect from %s after flap", peerIP)
}

func setupVethPair(t *testing.T, nameA, cidrA, nameB, cidrB string) {
	t.Helper()
	require.NoError(t, netlink.LinkAdd(&netlink.Veth{
		Name:     nameA,
		PeerName: nameB,
	}))
	for _, spec := range []struct{ name, cidr string }{
		{nameA, cidrA},
		{nameB, cidrB},
	} {
		link, err := netlink.LinkByName(spec.name)
		require.NoError(t, err)
		addr, err := netlink.ParseAddr(spec.cidr)
		require.NoError(t, err)
		require.NoError(t, netlink.AddrAdd(link, addr))
		require.NoError(t, netlink.LinkSetUp(link))
	}
}

func vethIsUp(t *testing.T, name string) bool {
	t.Helper()
	link, err := netlink.LinkByName(name)
	require.NoError(t, err)
	return link.Attrs().Flags&net.FlagUp != 0
}

func vethHasAddr(t *testing.T, name, cidr string) bool {
	t.Helper()
	link, err := netlink.LinkByName(name)
	require.NoError(t, err)
	want, err := netlink.ParseAddr(cidr)
	require.NoError(t, err)
	addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	require.NoError(t, err)
	for _, a := range addrs {
		if a.IPNet != nil && a.IPNet.IP.Equal(want.IPNet.IP) {
			return true
		}
	}
	return false
}

func serveBGPPeer(ctx context.Context, ln net.Listener, wantPeerIP string, accepted *atomic.Int32) {
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		if host, _, splitErr := net.SplitHostPort(conn.RemoteAddr().String()); splitErr == nil && host == wantPeerIP {
			accepted.Add(1)
		}
		c := conn
		wg.Go(func() {
			defer func() { _ = c.Close() }()
			stopClose := context.AfterFunc(ctx, func() { _ = c.Close() })
			defer stopClose()

			if err := readBGPFrame(c); err != nil {
				return
			}
			open, err := peer.BuildOpen(peer.SessionConfig{
				ASN:      65000,
				RouterID: netip.MustParseAddr("10.242.0.1"),
				HoldTime: 90,
				Families: []string{"ipv4/unicast"},
			})
			if err != nil {
				return
			}
			if !writeBGPMessage(c, open) {
				return
			}
			if err := readBGPFrame(c); err != nil {
				return
			}
			if !writeBGPMessage(c, message.NewKeepalive()) {
				return
			}
			for {
				if err := readBGPFrame(c); err != nil {
					return
				}
			}
		})
	}
}

func writeBGPMessage(conn net.Conn, msg message.Message) bool {
	buf := make([]byte, msg.Len(nil))
	n := msg.WriteTo(buf, 0, nil)
	if n <= 0 {
		return false
	}
	_, err := conn.Write(buf[:n])
	return err == nil
}

func readBGPFrame(conn net.Conn) error {
	var hdr [message.HeaderLen]byte
	if _, err := io.ReadFull(conn, hdr[:]); err != nil {
		return err
	}
	msgLen := int(binary.BigEndian.Uint16(hdr[16:18]))
	if msgLen < message.HeaderLen {
		return io.ErrUnexpectedEOF
	}
	if msgLen > message.HeaderLen {
		if _, err := io.CopyN(io.Discard, conn, int64(msgLen-message.HeaderLen)); err != nil {
			return err
		}
	}
	return nil
}

func waitSignal(t *testing.T, ctx context.Context, ch <-chan struct{}, errCh <-chan error, label string) {
	t.Helper()
	select {
	case <-ch:
	case err := <-errCh:
		t.Fatalf("unexpected peer error while waiting for %s: %v", label, err)
	case <-ctx.Done():
		t.Fatalf("timed out waiting for %s: %v", label, ctx.Err())
	}
}

func waitAction(t *testing.T, ctx context.Context, ch <-chan string, errCh <-chan error, label string) string {
	t.Helper()
	select {
	case got := <-ch:
		return got
	case err := <-errCh:
		t.Fatalf("unexpected peer error while waiting for %s: %v", label, err)
	case <-ctx.Done():
		t.Fatalf("timed out waiting for %s: %v", label, ctx.Err())
	}
	return ""
}
