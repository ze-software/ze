//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- installed PPP state.
// Related: pppoe_config_integration_linux_test.go -- registered config delivery.
// This test needs the host's /dev/ppp grant from ./le setup install.

package iface_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/vishvananda/netlink"

	_ "github.com/ze-software/ze/internal/component/l2tp/pppoeclient"
	"github.com/ze-software/ze/internal/core/rtproto"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
	"github.com/ze-software/ze/internal/test/pppoeac"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestPPPoEConfigRenegotiatesKernelState starts from operator configuration
// delivered through the registered plugin server and checks real kernel dumps
// after two authenticated epochs on the same PPP device.
func TestPPPoEConfigRenegotiatesKernelState(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	peer := pppoeac.OpenVeth(t, "zec0", "zea0", 44)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	restart := make(chan struct{})
	done := make(chan error, 1)
	joined := false
	go kernelStateAC(ctx, peer, restart, done)
	t.Cleanup(func() {
		cancel()
		if !joined {
			if err := <-done; err != nil {
				t.Errorf("AC script: %v", err)
			}
		}
	})
	startPPPoEConfig(t, pppoeOperatorConfig)
	before := waitPPPoEKernelState(t, ctx, netip.MustParseAddr("192.0.2.2"), netip.MustParseAddr("192.0.2.1"), 1450)
	close(restart)
	after := waitPPPoEKernelState(t, ctx, netip.MustParseAddr("198.51.100.2"), netip.MustParseAddr("198.51.100.1"), 1400)
	if after.Attrs().Index != before.Attrs().Index {
		t.Fatal("renegotiation replaced the PPP ifindex")
	}
	if after.Attrs().Name != before.Attrs().Name {
		t.Fatal("renegotiation changed the PPP device name")
	}
	select {
	case err := <-done:
		joined = true
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("AC did not finish both epochs")
	}
	t.Logf("same SID 44 and %s ifindex %d; sole local/peer pair, default gateway, metric 231, protocol %d and MTU replaced", after.Attrs().Name, after.Attrs().Index, rtproto.Iface)
}

// kernelStateAC owns the peer reader for both epochs and reports its final result.
// The caller MUST cancel and join it before OpenVeth cleanup closes the socket.
func kernelStateAC(ctx context.Context, peer *pppoeac.Peer, restart <-chan struct{}, done chan<- error) {
	done <- runKernelStateAC(ctx, peer, restart)
}

func runKernelStateAC(ctx context.Context, peer *pppoeac.Peer, restart <-chan struct{}) error {
	// RFC 2516 Section 5.
	session, err := peer.Accept(ctx)
	if err != nil {
		return err
	}
	for epoch := range 2 {
		if epoch == 1 {
			select {
			case <-restart:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		options := []byte{1, 4, 5, 170, 3, 5, 0xc2, 0x23, 5, 5, 6, 0, 0, 0, 0}
		binary.BigEndian.PutUint32(options[11:], uint32(0x10203040+epoch))
		if epoch == 1 {
			binary.BigEndian.PutUint16(options[2:4], 1400)
		}
		// RFC 1661 Sections 4.1 and 5.2.
		if err := session.LCP(ctx, uint8(70+epoch), options, epoch == 1); err != nil {
			return err
		}
		// RFC 1994 Sections 4.1 and 4.2.
		if err := session.CHAP(ctx, uint8(80+epoch), []byte{1, 3, 5, byte(7 + epoch)}, "subscriber", "secret"); err != nil {
			return err
		}
		local, remote := netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("192.0.2.2")
		if epoch == 1 {
			local, remote = netip.MustParseAddr("198.51.100.1"), netip.MustParseAddr("198.51.100.2")
		}
		// RFC 1332 Sections 2 and 3.3.
		if err := session.IPCP(ctx, uint8(90+epoch), local, remote); err != nil {
			return err
		}
	}
	return nil
}

// waitPPPoEKernelState polls installed state rather than assuming publication
// means runSession has applied it. Dump errors remain errors in the retry result.
func waitPPPoEKernelState(t *testing.T, ctx context.Context, local, peer netip.Addr, mtu int) netlink.Link {
	t.Helper()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	var last error
	for {
		links, err := netlink.LinkList()
		if err != nil {
			t.Fatal(err)
		}
		last = fmt.Errorf("no PPP device")
		for _, link := range links {
			if link.Type() != "ppp" {
				continue
			}
			last = pppoeKernelState(link, local, peer, mtu)
			if last == nil {
				return link
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("PPP installed state: %v: %v", last, ctx.Err())
		case <-tick.C:
		}
	}
}

// pppoeKernelState reads both endpoint attributes and the sole default route.
// Exact counts also prove that the old address and old gateway are absent.
func pppoeKernelState(link netlink.Link, local, peer netip.Addr, mtu int) error {
	if link.Attrs().MTU != mtu {
		return fmt.Errorf("MTU = %d, want %d", link.Attrs().MTU, mtu)
	}
	addresses, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return err
	}
	if len(addresses) != 1 {
		return fmt.Errorf("IPv4 address count = %d, want one", len(addresses))
	}
	address := addresses[0]
	if address.IPNet == nil {
		return fmt.Errorf("missing local prefix")
	}
	if address.IPNet.String() != netip.PrefixFrom(local, 32).String() {
		return fmt.Errorf("local = %v, want %v/32", address.IPNet, local)
	}
	if address.Peer == nil {
		return fmt.Errorf("missing peer prefix")
	}
	if address.Peer.String() != netip.PrefixFrom(peer, 32).String() {
		return fmt.Errorf("peer = %v, want %v/32", address.Peer, peer)
	}
	routes, err := netlink.RouteList(link, netlink.FAMILY_V4)
	if err != nil {
		return err
	}
	defaults := 0
	for i := range routes {
		route := &routes[i]
		if route.Dst == nil {
			return fmt.Errorf("IPv4 route dump has no destination: %+v", route)
		}
		if route.Dst.String() != "0.0.0.0/0" {
			continue
		}
		defaults++
		if route.Gw.String() != peer.String() {
			return fmt.Errorf("default gateway = %v, want %v", route.Gw, peer)
		}
		if route.Priority != 231 {
			return fmt.Errorf("default metric = %d, want 231", route.Priority)
		}
		if int(route.Protocol) != int(rtproto.Iface) {
			return fmt.Errorf("default protocol = %d, want %d", route.Protocol, rtproto.Iface)
		}
	}
	if defaults != 1 {
		return fmt.Errorf("default route count = %d, want one; dump: %+v", defaults, routes)
	}
	return nil
}
