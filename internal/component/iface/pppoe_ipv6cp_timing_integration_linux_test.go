//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- configured IPv6CP on a real PPP unit.
// Related: pppoe_config_integration_linux_test.go -- registered configuration entry point.

package iface_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/test/pppoeac"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestPPPoEClientIPv6CPConfiguredTimerEndToEnd reads actual IPv6CP retries
// after real configuration delivery and Dial. IPCP is deliberately unanswered
// so its separate three-second timer cannot produce the observed retry.
// MUTATION: force newClientIPv6CP's Dial argument to 100*time.Millisecond
// or 3*time.Second.
func TestPPPoEClientIPv6CPConfiguredTimerEndToEnd(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	peer := pppoeac.OpenVeth(t, "zec0", "zea0", 44)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	startPPPoEConfig(t, keepaliveConfig(`
        ipv6 { enabled true; }
        ipv6cp { restart-timer 500; }`))
	session, err := peer.Accept(ctx)
	if err != nil {
		t.Fatal(err)
	}
	options := []byte{3, 5, 0xc2, 0x23, 5, 5, 6, 0x10, 0x20, 0x30, 0x40}
	if err := session.LCP(ctx, 70, options, false); err != nil {
		t.Fatal(err)
	}
	if err := session.CHAP(ctx, 80, []byte{1, 3, 5, 7}, "subscriber", "secret"); err != nil {
		t.Fatal(err)
	}
	first := nextConfiguredIPv6CPRequest(t, ctx, session)
	firstReceived := time.Now()
	// Measure receipt spacing after discovery. Allow 150 ms for the first
	// request to have queued before its read and 400 ms of late scheduling,
	// while excluding both a fixed 100 ms timer and the three-second default.
	secondCtx, secondCancel := context.WithTimeout(ctx, 2*time.Second)
	defer secondCancel()
	second := nextConfiguredIPv6CPRequest(t, secondCtx, session)
	gap := time.Since(firstReceived)
	if !bytes.Equal(second.Data, first.Data) {
		t.Fatalf("retry options %x differ from outstanding request %x", second.Data, first.Data)
	}
	if gap < 350*time.Millisecond {
		t.Fatalf("IPv6CP retry receipt gap %s, want at least 350 ms for configured 500 ms timer", gap)
	}
	if gap > 900*time.Millisecond {
		t.Fatalf("IPv6CP retry receipt gap %s, want at most 900 ms for configured 500 ms timer", gap)
	}
}

// nextConfiguredIPv6CPRequest ignores only the independently negotiating IPCP
// request. Echoes are answered by the scripted peer's normal reader.
func nextConfiguredIPv6CPRequest(t *testing.T, ctx context.Context, session *pppoeac.Session) ppp.LCPPacket {
	t.Helper()
	for {
		proto, packet, err := session.ReadExpectedControl(ctx)
		if err != nil {
			t.Fatalf("wait for configured IPv6CP request: %v", err)
		}
		if packet.Code != ppp.LCPConfigureRequest {
			t.Fatalf("protocol %#x code %d, want a Configure-Request", proto, packet.Code)
		}
		if proto == ppp.ProtoIPv6CP {
			return packet
		}
		if proto != ppp.ProtoIPCP {
			t.Fatalf("protocol %#x, want IPv6CP or its independent IPCP", proto)
		}
	}
}
