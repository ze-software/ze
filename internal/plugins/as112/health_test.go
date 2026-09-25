package as112

import (
	"context"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/dnsserver"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// freePortLow and freePortHigh bound the ports freePort draws from. The span
// sits below the kernel's ephemeral range (Linux 32768-60999, BSD and macOS
// 49152-65535), so no client socket's automatic source port can land on it.
const (
	freePortLow   = 10000
	freePortHigh  = 32768
	freePortTries = 64
)

// freePort returns a loopback port that is free for BOTH UDP and TCP, because
// dnsserver.Manager binds every endpoint on both.
//
// Asking the kernel for "127.0.0.1:0" is not enough: that port comes from the
// ephemeral range and is released before dnsserver.Manager binds it, so any UDP or TCP
// client socket on the machine, this package's own dns.Client exchanges
// included, can be given the same number in between. Under a loaded gate run
// that made Apply fail with "no listeners bound".
func freePort(t *testing.T) uint16 {
	t.Helper()
	var lc net.ListenConfig
	ctx := context.Background()
	var lastErr error
	for range freePortTries {
		port := uint16(freePortLow + rand.IntN(freePortHigh-freePortLow))
		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))
		pc, err := lc.ListenPacket(ctx, "udp", addr)
		if err != nil {
			lastErr = err
			continue
		}
		ln, err := lc.Listen(ctx, "tcp", addr)
		if cerr := pc.Close(); cerr != nil {
			t.Fatalf("close udp probe: %v", cerr)
		}
		if err != nil {
			lastErr = err
			continue
		}
		if cerr := ln.Close(); cerr != nil {
			t.Fatalf("close tcp probe: %v", cerr)
		}
		return port
	}
	t.Fatalf("no port in [%d, %d) free for udp and tcp after %d tries: %v", freePortLow, freePortHigh, freePortTries, lastErr)
	return 0
}

// startTestServer binds a real dnsserver.Manager instance (through
// newServerManager) on 127.0.0.1 with a free port and returns the address.
func startTestServer(t *testing.T, cfg as112Config) string {
	t.Helper()
	resetAS112State(t)
	storeState(buildState(cfg, 1))

	mgr := newServerManager(testLogger(), nil)
	port := freePort(t)
	if err := mgr.apply(true, []dnsserver.Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	t.Cleanup(mgr.stopAll)
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))
}

// VALIDATES: AC-12 / finding M4 -- exit 0 when a real authoritative query to
// an anycast service address returns the expected AS112 answer; non-zero
// otherwise.
func TestHealthCommand_ExitCodes(t *testing.T) {
	addr := startTestServer(t, as112Config{Enabled: true})

	if code := runHealthQuery(context.Background(), addr, 2*time.Second); code != 0 {
		t.Fatalf("runHealthQuery against a live, enabled server = %d, want 0", code)
	}

	// Query a closed port: the probe must fail (non-zero), not hang or panic.
	closedAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(freePort(t))))
	if code := runHealthQuery(context.Background(), closedAddr, 500*time.Millisecond); code == 0 {
		t.Fatal("runHealthQuery against a closed port = 0, want non-zero")
	}
}

// VALIDATES: the on-box health probe's default target respects the
// configured address-family. An ipv6-only node never binds 127.0.0.1 (see
// serverEndpoints, register.go): defaulting to it would report a healthy
// ipv6-only node as unreachable. Conversely both/ipv4-only nodes always bind
// 127.0.0.1, so it remains the default there.
func TestDefaultHealthTarget_RespectsAddressFamily(t *testing.T) {
	t.Cleanup(func() { resetAS112State(t) })

	cases := []struct {
		family string
		want   string
	}{
		{addressFamilyBoth, "127.0.0.1:53"},
		{addressFamilyIPv4Only, "127.0.0.1:53"},
		{addressFamilyIPv6Only, "[::1]:53"},
	}
	for _, tc := range cases {
		t.Run(tc.family, func(t *testing.T) {
			resetAS112State(t)
			storeState(buildState(as112Config{Enabled: true, AddressFamily: tc.family}, 1))
			if got := defaultHealthTarget(); got != tc.want {
				t.Fatalf("defaultHealthTarget() with address-family %q = %q, want %q", tc.family, got, tc.want)
			}
		})
	}
}

// VALIDATES: before any config is committed (no published state), the
// default target still falls back to 127.0.0.1 rather than panicking on a
// nil state.
func TestDefaultHealthTarget_NoStateFallsBackToIPv4Loopback(t *testing.T) {
	resetAS112State(t)
	if got := defaultHealthTarget(); got != "127.0.0.1:53" {
		t.Fatalf("defaultHealthTarget() with no published state = %q, want \"127.0.0.1:53\"", got)
	}
}

// VALIDATES: the CLI dispatcher passes command args through with any
// keyword token still attached (e.g. "request as112 healthcheck target 1.2.3.4" reaches
// the handler as args=["target","1.2.3.4"], not args=["1.2.3.4"] -- see
// internal/component/plugin/server/command_test.go's
// TestDispatcherKeywordExtraction and internal/plugins/diag/cmd/tcp_check.go's
// parseTCPCheckArgs for the established, verified convention every
// keyword-arg command handler must follow). The YANG usage string
// (yang/ze-as112-cmd.yang) documents exactly this "target <ip>" form.
func TestParseHealthArgs(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		want    string
		wantErr bool
	}{
		{"no args uses default", nil, "", false},
		{"documented keyword form", []string{"target", "192.175.48.1"}, "192.175.48.1", false},
		{"keyword missing value", []string{"target"}, "", true},
		{"unrecognized keyword", []string{"bogus", "192.175.48.1"}, "", true},
		{"undocumented bare positional rejected", []string{"192.175.48.1"}, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseHealthArgs(tc.args)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseHealthArgs(%v) error = %v, wantErr %v", tc.args, err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("parseHealthArgs(%v) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}
