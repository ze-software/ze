//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- configured LCP Nak budget at the wire.
// Related: pppoe_config_integration_linux_test.go -- registered configuration delivery.

package iface_test

import (
	"bytes"
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/test/pppoeac"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestPPPoEClientLCPConfiguredMaxFailureEndToEnd reads actual replies from
// operator-configured Dial, then proves an answering peer can still open PPP.
// MUTATION: replace Dial's configured max-failure snapshot with five.
func TestPPPoEClientLCPConfiguredMaxFailureEndToEnd(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	peer := pppoeac.OpenVeth(t, "zec0", "zea0", 44)
	ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
	defer cancel()
	startPPPoEConfig(t, keepaliveConfig(`lcp { max-failure 2; }`))
	session, err := peer.Accept(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// PADS precedes Dial's kernel socket setup. Its first LCP request proves
	// the PPPoX receiver exists before the peer sends the refusal sequence.
	proto, initial, err := session.ReadExpectedControl(ctx)
	if err != nil {
		t.Fatalf("wait for initial client LCP request: %v", err)
	}
	if proto != ppp.ProtoLCP {
		t.Fatalf("initial protocol = %#x, want LCP", proto)
	}
	if initial.Code != ppp.LCPConfigureRequest {
		t.Fatalf("initial LCP code = %d, want Configure-Request", initial.Code)
	}
	bad := []byte{ppp.LCPOptMRU, 4, 0x07, 0xd0}
	for index := range 3 {
		id := uint8(40 + index)
		if err := session.SendControl(ppp.ProtoLCP, ppp.LCPConfigureRequest, id, bad); err != nil {
			t.Fatal(err)
		}
		reply := nextMaxFailureReply(t, ctx, session, id)
		want := ppp.LCPConfigureNak
		if index == 2 {
			want = ppp.LCPConfigureReject
			if !bytes.Equal(reply.Data, bad) {
				t.Fatalf("Reject options = %x, want original MRU %x", reply.Data, bad)
			}
		}
		if reply.Code != want {
			t.Fatalf("reply %d Code = %d, want %d with max-failure 2", index+1, reply.Code, want)
		}
	}
	// The peer now proposes supported authentication. LCP, CHAP and IPCP must
	// complete; a client refusing every request cannot satisfy the control.
	options := []byte{3, 5, 0xc2, 0x23, 5, 5, 6, 0x10, 0x20, 0x30, 0x40}
	// RFC 1661 Section 5.2: "The acknowledged Configuration Options MUST NOT
	// be reordered or modified in any way." Ack the retained client request
	// only after the refusal sequence, then open the other Configure exchange.
	if err := session.SendControl(ppp.ProtoLCP, ppp.LCPConfigureAck, initial.Identifier, initial.Data); err != nil {
		t.Fatal(err)
	}
	// RFC 1661 Section 5.1.
	if err := session.SendControl(ppp.ProtoLCP, ppp.LCPConfigureRequest, 70, options); err != nil {
		t.Fatal(err)
	}
	accepted := nextMaxFailureReply(t, ctx, session, 70)
	if accepted.Code != ppp.LCPConfigureAck {
		t.Fatalf("supported options received Code %d, want Configure-Ack", accepted.Code)
	}
	if !bytes.Equal(accepted.Data, options) {
		t.Fatalf("supported options Ack = %x, want unchanged %x", accepted.Data, options)
	}
	if err := session.CHAP(ctx, 80, []byte{1, 3, 5, 7}, "subscriber", "secret"); err != nil {
		t.Fatal(err)
	}
	if err := session.IPCP(ctx, 90, netip.MustParseAddr("192.0.2.44"), netip.MustParseAddr("192.0.2.1")); err != nil {
		t.Fatal(err)
	}
}

// nextMaxFailureReply leaves the independently retransmitting client request
// unanswered until the refusal sequence finishes. Its context bounds the loop.
func nextMaxFailureReply(t *testing.T, ctx context.Context, session *pppoeac.Session, id uint8) ppp.LCPPacket {
	t.Helper()
	for {
		proto, reply, err := session.ReadExpectedControl(ctx)
		if err != nil {
			t.Fatalf("wait for LCP reply Identifier %d: %v", id, err)
		}
		if proto != ppp.ProtoLCP {
			t.Fatalf("protocol = %#x, want LCP", proto)
		}
		if reply.Code == ppp.LCPConfigureRequest {
			continue
		}
		if reply.Identifier != id {
			t.Fatalf("reply Identifier = %d, want %d", reply.Identifier, id)
		}
		return reply
	}
}
