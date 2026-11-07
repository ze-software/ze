//go:build integration && linux

// Design: docs/architecture/iface/logical-name-resolution.md -- consumers resolve through iface
// Related: scripted_ac_integration_linux_test.go -- the same scripted AC, one epoch
// This test needs the host's /dev/ppp grant from ./le setup install.
//
// Goal: prove a real Dial whose source-interface is a logical name bound by an
// os-name selector reaches a session on the kernel device that name resolves
// to. The session socket's connect(AF_PPPOX) takes a device name, so a dialer
// that handed it the configured name fails with no such device after PADS.
// Method: a veth pair zec0/zea0 in a user namespace; the resolver maps logical
// "wan" to kernel device "zec0" (what `ethernet wan { os-name zec0 }`
// publishes), no device named "wan" exists, and a scripted AC on zea0 runs
// discovery, LCP, CHAP and IPCP.

package pppoeclient

import (
	"context"
	"encoding/binary"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/iface"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
	"github.com/ze-software/ze/internal/test/pppoeac"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestDialOSNameSourceInterface dials through a logical source-interface whose
// os-name selector names a differently named kernel device.
func TestDialOSNameSourceInterface(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	if err := iface.LoadBackend("netlink"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := iface.CloseBackend(); err != nil {
			t.Error(err)
		}
	})
	peer := pppoeac.OpenVeth(t, "zec0", "zea0", 45)
	t.Cleanup(iface.SetResolverMappingForConsumerTest(map[string]string{"wan": "zec0"}))
	// The proof depends on the logical name naming no kernel device.
	if _, err := netlink.LinkByName("wan"); err == nil {
		t.Fatal("a kernel device named wan exists; the logical name must differ from the device")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	done := make(chan error, 1)
	go scriptedACOneEpoch(ctx, peer, done)
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil && ctx.Err() == nil {
			t.Error(err)
		}
	})

	sess, err := (&Dialer{}).Dial(iface.PPPoEClientConfig{
		SourceInterface: "wan", MTU: 1492, Timing: testDialTiming, Username: "subscriber", AuthSecret: "secret",
	}, ctx.Done(), slog.Default())
	if err != nil {
		t.Fatalf("Dial through logical source-interface wan (os-name zec0): %v", err)
	}
	t.Cleanup(sess.Cleanup)
	if sess.SessionID != 45 {
		t.Fatalf("session ID = %d, want 45", sess.SessionID)
	}
	if sess.LocalIP != netip.MustParseAddr("192.0.2.2") {
		t.Fatalf("local address = %v, want 192.0.2.2", sess.LocalIP)
	}
}

// scriptedACOneEpoch owns the peer reads until cancellation or completion.
func scriptedACOneEpoch(ctx context.Context, peer *pppoeac.Peer, done chan<- error) {
	done <- runScriptedACOneEpoch(ctx, peer)
}

func runScriptedACOneEpoch(ctx context.Context, peer *pppoeac.Peer) error {
	// RFC 2516 Section 5.
	session, err := peer.Accept(ctx)
	if err != nil {
		return err
	}
	options := []byte{3, 5, 0xc2, 0x23, 5, 5, 6, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(options[7:], 0x10203040)
	// RFC 1661 Sections 4.1 and 5.2.
	if err := session.LCP(ctx, 70, options, false); err != nil {
		return err
	}
	// RFC 1994 Sections 4.1 and 4.2.
	if err := session.CHAP(ctx, 80, []byte{1, 3, 5, 7}, "subscriber", "secret"); err != nil {
		return err
	}
	// RFC 1332 Sections 2 and 3.3.
	return session.IPCP(ctx, 90, netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("192.0.2.2"))
}
