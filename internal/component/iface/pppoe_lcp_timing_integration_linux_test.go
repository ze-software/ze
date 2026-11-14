//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- operator-configured LCP retransmission.
// Related: pppoe_config_integration_linux_test.go -- registered configuration delivery.

package iface_test

import (
	"context"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/test/pppoeac"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestPPPoEClientLCPConfiguredInitialBudgetEndToEnd observes configured
// retransmission and budget exhaustion from initial operator-configured Dial.
func TestPPPoEClientLCPConfiguredInitialBudgetEndToEnd(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	checkConfiguredLCPBudget(t, false)
}

// TestPPPoEClientLCPConfiguredRestartBudgetEndToEnd observes the same timer
// and request budget when the peer restarts LCP on the established session.
func TestPPPoEClientLCPConfiguredRestartBudgetEndToEnd(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	checkConfiguredLCPBudget(t, true)
}

// The caller of checkConfiguredLCPBudget MUST enter its own namespace first.
// It observes the actual Dial timer after discovery, then the exact
// request budget and PADT. Initial and restart callers use fresh namespaces.
func checkConfiguredLCPBudget(t *testing.T, restart bool) {
	t.Helper()
	peer := pppoeac.OpenVeth(t, "zec0", "zea0", 44)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	startPPPoEConfig(t, keepaliveConfig(`
        lcp { restart-timer 500; max-configure 3; }`))
	var session *pppoeac.Session
	if restart {
		session = upKeepaliveSession(t, ctx, peer, 0)
		if err := session.SendControl(ppp.ProtoLCP, ppp.LCPConfigureRequest, 93,
			[]byte{ppp.LCPOptMRU, 4, 0x05, 0xd4}); err != nil {
			t.Fatal(err)
		}
	} else {
		var err error
		session, err = peer.Accept(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}
	nextConfiguredLCPRequest(t, ctx, session)
	// The bound starts only after discovery and the first request. It
	// permits 1.5 seconds of scheduling delay beyond the 500 ms timer.
	secondCtx, secondCancel := context.WithTimeout(ctx, 2*time.Second)
	nextConfiguredLCPRequest(t, secondCtx, session)
	secondCancel()
	remaining, err := session.UnansweredConfigureRequests(ctx)
	requests := 2 + remaining
	if err != nil {
		t.Fatalf("no PADT after %d unanswered Configure-Requests: %v", requests, err)
	}
	if requests != 3 {
		t.Fatalf("Configure-Requests before PADT = %d, want configured budget 3", requests)
	}
}

// nextConfiguredLCPRequest observes a request without replying. A restart's
// Configure-Ack may follow its first request and precede the retransmission.
// The context bounds that read; any other control packet is unexpected here.
func nextConfiguredLCPRequest(t *testing.T, ctx context.Context, session *pppoeac.Session) {
	t.Helper()
	for {
		proto, packet, err := session.ReadControl(ctx)
		if err != nil {
			t.Fatalf("wait for the next configured LCP Configure-Request: %v", err)
		}
		if proto != ppp.ProtoLCP {
			t.Fatalf("protocol = %#x, want LCP during negotiation", proto)
		}
		if packet.Code == ppp.LCPConfigureRequest {
			return
		}
		if packet.Code != ppp.LCPConfigureAck {
			t.Fatalf("LCP code = %d, want Configure-Request or the restart's Configure-Ack", packet.Code)
		}
	}
}

// TestPPPoEClientLCPConfiguredBudgetAllowsSuccess contrasts an answering peer
// with the silent one: Max-Configure limits unanswered requests and must not
// prevent LCP, authentication and IPCP from opening when the peer responds.
func TestPPPoEClientLCPConfiguredBudgetAllowsSuccess(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	peer := pppoeac.OpenVeth(t, "zec0", "zea0", 44)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	startPPPoEConfig(t, keepaliveConfig(`
        lcp { restart-timer 1000; max-configure 1; }`))
	upKeepaliveSession(t, ctx, peer, 0)
}
