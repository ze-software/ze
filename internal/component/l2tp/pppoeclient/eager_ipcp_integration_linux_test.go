//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- eager peer after authentication.
// Related: internal/test/pppoeac -- shared discovery and authentication script.

package pppoeclient

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/test/pppoeac"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestDialAcknowledgesFirstEagerIPCP sends Success then IPCP on the wire without
// waiting for the client's request. The first peer request must receive its Ack
// without a peer retransmission. The deterministic reader test controls scheduling.
func TestDialAcknowledgesFirstEagerIPCP(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	restart := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- runEagerIPCPPeer(ctx, peer, restart) }()
	joined := false
	t.Cleanup(func() {
		cancel()
		if !joined {
			if err := <-done; err != nil {
				t.Error(err)
			}
		}
	})
	session, err := (&Dialer{}).Dial(iface.PPPoEClientConfig{
		SourceInterface: "zec0", MTU: 1492, Timing: testDialTiming, Username: "subscriber", AuthSecret: "secret",
	}, ctx.Done(), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Cleanup)
	close(restart)
	err = <-done
	joined = true
	if err != nil {
		t.Fatal(err)
	}
}

// runEagerIPCPPeer owns the only AC read loop until return. The caller MUST
// cancel ctx and join this worker before closing the peer socket.
func runEagerIPCPPeer(ctx context.Context, peer *pppoeac.Peer, restart <-chan struct{}) error {
	// RFC 2516 Section 5.
	session, err := peer.Accept(ctx)
	if err != nil {
		return err
	}
	options := []byte{3, 5, 0xc2, 0x23, 5, 5, 6, 0x10, 0x20, 0x30, 0x40}
	// RFC 1661 Sections 4.1 and 5.2.
	if err := session.LCP(ctx, 70, options, false); err != nil {
		return err
	}
	// RFC 1994 Sections 4.1 and 4.2.
	if err := session.CHAP(ctx, 80, []byte{1, 3, 5, 7}, "subscriber", "secret"); err != nil {
		return err
	}
	// RFC 1332 Sections 2 and 3.3.
	if err := session.IPCP(ctx, 90, netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("192.0.2.2")); err != nil {
		return err
	}
	select {
	case <-restart:
	case <-ctx.Done():
		return ctx.Err()
	}
	options[10]++
	// RFC 1661 Sections 4.1 and 5.2.
	if err := session.LCP(ctx, 71, options, true); err != nil {
		return err
	}
	// RFC 1994 Sections 4.1 and 4.2.
	if err := session.CHAP(ctx, 81, []byte{2, 4, 6, 8}, "subscriber", "secret"); err != nil {
		return err
	}
	// Success was just sent. Do not wait for the client's IPCP before ours.
	address := []byte{3, 6, 198, 51, 100, 1}
	// RFC 1332 Section 2 and RFC 1661 Section 5.1.
	if err := session.SendControl(ppp.ProtoIPCP, ppp.LCPConfigureRequest, 94, address); err != nil {
		return err
	}
	// This deadline detects a missing first reply before the client's 3s retry.
	ackCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	for range 2 {
		// RFC 1332 Section 2 and RFC 1661 Section 5.2.
		proto, packet, err := session.ReadControl(ackCtx)
		if err != nil {
			return fmt.Errorf("first eager IPCP request was not acknowledged: %w", err)
		}
		if proto != ppp.ProtoIPCP {
			return fmt.Errorf("expected IPCP, received protocol %x", proto)
		}
		if packet.Code == ppp.LCPConfigureRequest {
			continue
		}
		if packet.Code != ppp.LCPConfigureAck {
			return fmt.Errorf("expected IPCP Ack, received code %d", packet.Code)
		}
		if packet.Identifier != 94 {
			return fmt.Errorf("IPCP Ack identifier %d, want 94", packet.Identifier)
		}
		if !bytes.Equal(packet.Data, address) {
			return fmt.Errorf("IPCP Ack options %x, want %x", packet.Data, address)
		}
		return nil
	}
	return fmt.Errorf("first eager IPCP request was not acknowledged before repeated client requests")
}
