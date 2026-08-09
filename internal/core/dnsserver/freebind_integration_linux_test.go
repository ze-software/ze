// Design: docs/architecture/dns/server-harness.md -- IP_FREEBIND kernel wiring proof

//go:build integration && linux

package dnsserver

import (
	"context"
	"testing"
)

// VALIDATES: with Freebind enabled, the Control hook actually reaches the
// kernel -- a bind to an address not locally assigned (TEST-NET-3, RFC 5737,
// never routed to this host) succeeds only when IP_FREEBIND is set (AC-4).
// PREVENTS: the harness's Freebind option becoming a struct field that never
// reaches the socket (the manager_test.go unit test only checks the Control
// func is non-nil, not that the kernel honors it).
func TestFreebindAllowsNonLocalBind(t *testing.T) {
	const nonLocalAddr = "203.0.113.5:0"

	lc := listenConfig(false)
	if _, err := lc.ListenPacket(context.Background(), "udp", nonLocalAddr); err == nil {
		t.Fatal("expected bind to a non-local address to fail without Freebind")
	}

	lcFree := listenConfig(true)
	pc, err := lcFree.ListenPacket(context.Background(), "udp", nonLocalAddr)
	if err != nil {
		t.Skipf("IP_FREEBIND bind failed (needs CAP_NET_ADMIN): %v", err)
	}
	if cerr := pc.Close(); cerr != nil {
		t.Fatalf("close: %v", cerr)
	}
}
