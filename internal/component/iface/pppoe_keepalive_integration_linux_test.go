//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- configured keepalive and redial, end to end.
// Related: pppoe_config_integration_linux_test.go -- registered config delivery.
// Related: internal/test/pppoeac -- the scripted access concentrator.

package iface_test

import (
	"context"
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/test/pppoeac"
	"github.com/ze-software/ze/internal/test/userns"
)

// keepaliveConfig is the operator config of these tests with the given
// keepalive and reconnect blocks inside the pppoe-client entry.
func keepaliveConfig(blocks string) string {
	return `
interface {
    backend netlink;
    pppoe-client wan {
        source-interface zec0;
        authentication {
            username subscriber;
            password secret;
        }
` + blocks + `
    }
}
`
}

// upKeepaliveSession accepts discovery and completes LCP, CHAP and IPCP, so
// the client's keepalive loop runs on the returned session.
func upKeepaliveSession(t *testing.T, ctx context.Context, peer *pppoeac.Peer, epoch uint8) *pppoeac.Session {
	t.Helper()
	// RFC 2516 Section 5.
	session, err := peer.Accept(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// RFC 1661 Section 5.1, RFC 1994 Section 4.1.
	options := []byte{3, 5, 0xc2, 0x23, 5, 5, 6, 0x10, 0x20, 0x30, 0x40 + epoch}
	if err := session.LCP(ctx, 70+epoch, options, false); err != nil {
		t.Fatal(err)
	}
	if err := session.CHAP(ctx, 80+epoch, []byte{1, 3, 5, 7 + epoch}, "subscriber", "secret"); err != nil {
		t.Fatal(err)
	}
	// RFC 1332 Section 3.3.
	if err := session.IPCP(ctx, 90+epoch, netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("192.0.2.2")); err != nil {
		t.Fatal(err)
	}
	return session
}

// TestPPPoEClientKeepaliveConfigEndToEndDetection proves AC-2 from the config
// apply entry point through the production dialer and a real PPP unit: with
// `keepalive interval 1; failures 2` and an access concentrator that stops
// answering, the AC receives exactly two unanswered Echo-Requests and then
// the client's PADT. The defaults (10 s, 3) send a third request and miss
// the deadline, so the count cannot pass by accident.
func TestPPPoEClientKeepaliveConfigEndToEndDetection(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	peer := pppoeac.OpenVeth(t, "zec0", "zea0", 44)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	startPPPoEConfig(t, keepaliveConfig(`
        keepalive {
            interval 1;
            failures 2;
        }`))
	session := upKeepaliveSession(t, ctx, peer, 0)
	// RFC 1661 Section 5.8.
	echoes, err := session.UnansweredEchoes(ctx)
	if err != nil {
		t.Fatalf("no PADT after %d unanswered Echo-Requests: %v", echoes, err)
	}
	if echoes != 2 {
		t.Fatalf("client sent %d unanswered Echo-Requests before PADT, want 2", echoes)
	}
}

// TestPPPoEClientKeepaliveConfigEndToEndBackoff proves AC-3's lower bound end
// to end: with `reconnect delay-min 3; delay-max 4`, after the AC ends each
// session the next PADI comes at least 3 s, then at least 4 s, later. The
// defaults wait 1 s then 2 s, so both bounds are red without the leaves. Only
// lower bounds are asserted: an upper bound would measure the machine. The
// exact 5, 10, 20, 20 sequence is TestPPPoEClientReconnectDelayFromConfig.
func TestPPPoEClientKeepaliveConfigEndToEndBackoff(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	peer := pppoeac.OpenVeth(t, "zec0", "zea0", 44)
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
	defer cancel()
	startPPPoEConfig(t, keepaliveConfig(`
        reconnect {
            delay-min 3;
            delay-max 4;
        }`))
	wants := []time.Duration{3 * time.Second, 4 * time.Second}
	var endedAt time.Time
	for attempt := 0; attempt <= len(wants); attempt++ {
		// RFC 2516 Section 5.
		session, err := peer.Accept(ctx)
		if err != nil {
			t.Fatalf("attempt %d: %v", attempt, err)
		}
		if attempt > 0 {
			padiAt, err := session.DiscoveryStarted()
			if err != nil {
				t.Fatal(err)
			}
			if gap := padiAt.Sub(endedAt); gap < wants[attempt-1] {
				t.Fatalf("redial %d came %s after the session ended, want at least %s", attempt, gap, wants[attempt-1])
			}
		}
		// RFC 2516 Section 5.5: end the session before LCP opens.
		if err := session.SendPADT(); err != nil {
			t.Fatal(err)
		}
		endedAt = time.Now()
	}
}

// TestPPPoEClientKeepaliveConfigEndToEndReloadUptake proves AC-6 end to end:
// a reload that changes only the keepalive leaves keeps the session (the AC's
// Echo-Request on the same session id is still answered) and the running
// session keeps its old count (2), while the next session uses the new one (3).
func TestPPPoEClientKeepaliveConfigEndToEndReloadUptake(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	peer := pppoeac.OpenVeth(t, "zec0", "zea0", 44)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	srv := startPPPoEConfig(t, keepaliveConfig(`
        keepalive {
            interval 5;
            failures 2;
        }`))
	session := upKeepaliveSession(t, ctx, peer, 0)

	reloaded, err := config.LoadConfig(keepaliveConfig(`
        keepalive {
            interval 1;
            failures 3;
        }`), "", nil)
	if err != nil {
		t.Fatalf("load reloaded config: %v", err)
	}
	if err := srv.ReloadConfig(ctx, reloaded.Tree.ToPluginMap()); err != nil {
		t.Fatalf("reload: %v", err)
	}

	// The same session still answers: a restart would have torn it down.
	var data [4]byte
	binary.BigEndian.PutUint32(data[:], 0x10203040)
	// RFC 1661 Section 5.8.
	if err := session.SendControl(ppp.ProtoLCP, ppp.LCPEchoRequest, 200, data[:]); err != nil {
		t.Fatal(err)
	}
	aliveCtx, aliveCancel := context.WithTimeout(ctx, 5*time.Second)
	proto, reply, err := session.ReadExpectedControl(aliveCtx)
	aliveCancel()
	if err != nil {
		t.Fatalf("the session did not answer after a timer-only reload: %v", err)
	}
	if proto != ppp.ProtoLCP {
		t.Fatalf("answer protocol %04x, want LCP", proto)
	}
	if reply.Code != ppp.LCPEchoReply {
		t.Fatalf("answer code %d, want Echo-Reply", reply.Code)
	}
	if reply.Identifier != 200 {
		t.Fatalf("answer identifier %d, want 200", reply.Identifier)
	}

	// The running session keeps the failure count it was dialed with.
	echoes, err := session.UnansweredEchoes(ctx)
	if err != nil {
		t.Fatalf("old session: no PADT after %d unanswered Echo-Requests: %v", echoes, err)
	}
	if echoes != 2 {
		t.Fatalf("old session ended after %d unanswered Echo-Requests, want 2", echoes)
	}

	// The next session uses the reloaded count.
	next := upKeepaliveSession(t, ctx, peer, 1)
	echoes, err = next.UnansweredEchoes(ctx)
	if err != nil {
		t.Fatalf("new session: no PADT after %d unanswered Echo-Requests: %v", echoes, err)
	}
	if echoes != 3 {
		t.Fatalf("new session ended after %d unanswered Echo-Requests, want 3", echoes)
	}
}
