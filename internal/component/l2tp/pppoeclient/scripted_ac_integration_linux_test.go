//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- real Dial with a scripted AC.
// Related: internal/test/pppoeac -- shared wire fixture.
// This test needs the host's /dev/ppp grant from ./le setup install.

package pppoeclient

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
	"github.com/ze-software/ze/internal/test/pppoeac"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestScriptedACRenegotiatesDial checks two authenticated epochs over the same
// real PPP unit. Exact control replies are checked by the script, while the
// consumer checks the published addresses and retained kernel identity.
// This is not the runSession address/route or IP DROP proof.
func TestScriptedACRenegotiatesDial(t *testing.T) {
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
	peer := pppoeac.OpenVeth(t, "zec0", "zea0", 44)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	restart := make(chan struct{})
	done := make(chan error, 1)
	joined := false
	go scriptedACRenegotiation(ctx, peer, restart, done)
	t.Cleanup(func() {
		cancel()
		if !joined {
			if err := <-done; err != nil {
				t.Error(err)
			}
		}
	})
	sess, err := (&Dialer{}).Dial(iface.PPPoEClientConfig{
		SourceInterface: "zec0", MTU: 1492, Timing: testDialTiming, Username: "subscriber", AuthSecret: "secret",
	}, ctx.Done(), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sess.Cleanup)
	if sess.SessionID != 44 {
		t.Fatalf("session ID = %d, want 44", sess.SessionID)
	}
	name := fmt.Sprintf("ppp%d", sess.UnitNum)
	before, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatal(err)
	}
	if sess.LocalIP != netip.MustParseAddr("192.0.2.2") {
		t.Fatalf("initial local address: %v", sess.LocalIP)
	}
	close(restart)
	select {
	case change := <-sess.Changes:
		if change.LocalIP != netip.MustParseAddr("198.51.100.2") {
			t.Fatalf("new local address: %v", change.LocalIP)
		}
		if change.PeerIP != netip.MustParseAddr("198.51.100.1") {
			t.Fatalf("new peer address: %v", change.PeerIP)
		}
	case <-sess.Done:
		t.Fatal("renegotiation ended the session")
	case <-ctx.Done():
		t.Fatal("renegotiation did not publish addresses")
	}
	after, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatal(err)
	}
	if after.Attrs().Index != before.Attrs().Index {
		t.Fatal("renegotiation replaced the PPP interface")
	}
	select {
	case err := <-done:
		joined = true
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("wire script did not finish")
	}
}

// scriptedACRenegotiation owns the peer reads until cancellation or completion.
func scriptedACRenegotiation(ctx context.Context, peer *pppoeac.Peer, restart <-chan struct{}, done chan<- error) {
	done <- runScriptedACRenegotiation(ctx, peer, restart)
}

func runScriptedACRenegotiation(ctx context.Context, peer *pppoeac.Peer, restart <-chan struct{}) error {
	// RFC 2516 Section 5.
	session, err := peer.Accept(ctx)
	if err != nil {
		return err
	}
	var previousMagic uint32
	for epoch := range 2 {
		if epoch == 1 {
			select {
			case <-restart:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		magic := uint32(0x10203040 + epoch)
		options := []byte{3, 5, 0xc2, 0x23, 5, 5, 6, 0, 0, 0, 0}
		binary.BigEndian.PutUint32(options[7:], magic)
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
		var data [4]byte
		binary.BigEndian.PutUint32(data[:], magic)
		// RFC 1661 Section 5.8.
		if err := session.SendControl(ppp.ProtoLCP, ppp.LCPEchoRequest, uint8(100+epoch), data[:]); err != nil {
			return err
		}
		proto, reply, err := session.ReadControl(ctx)
		if err != nil {
			return err
		}
		if proto != ppp.ProtoLCP {
			return fmt.Errorf("unexpected echo protocol %x", proto)
		}
		if reply.Code != ppp.LCPEchoReply {
			return fmt.Errorf("unexpected echo code %d", reply.Code)
		}
		if reply.Identifier != uint8(100+epoch) {
			return fmt.Errorf("unexpected echo identifier %d", reply.Identifier)
		}
		if len(reply.Data) != 4 {
			return fmt.Errorf("unexpected echo data %x", reply.Data)
		}
		currentMagic := binary.BigEndian.Uint32(reply.Data)
		if currentMagic == 0 {
			return fmt.Errorf("echo reply has zero client magic")
		}
		if currentMagic == previousMagic {
			return fmt.Errorf("echo reply retained the previous client magic")
		}
		previousMagic = currentMagic
	}
	return nil
}
