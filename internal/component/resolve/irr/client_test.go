package irr

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeIRRServer starts a TCP server that responds to RPSL whois queries
// with deterministic data. Returns the server address and a cleanup function.
func fakeIRRServer(t *testing.T, handler func(conn net.Conn)) (string, func()) {
	t.Helper()
	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return // listener closed
			}
			go handler(conn)
		}
	}()

	return ln.Addr().String(), func() { _ = ln.Close() }
}

// handleASSetQuery responds to "!i" and "!a" queries with test data.
func handleASSetQuery(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	buf := make([]byte, 4096)
	n, readErr := conn.Read(buf)
	if readErr != nil {
		return
	}
	query := string(buf[:n])

	var response string
	switch query {
	case "!iAS-TEST\n":
		response = "A3\nAS65001 AS65002 AS65003\nC\n"
	case "!iAS-NESTED\n":
		response = "A2\nAS65001 AS-CHILD\nC\n"
	case "!iAS-CHILD\n":
		response = "A1\nAS65002\nC\n"
	case "!iAS-CYCLE\n":
		response = "A1\nAS65001 AS-CYCLE\nC\n"
	case "!iAS-EMPTY\n":
		response = "D\n"
	case "!a4AS-TEST\n":
		response = "A5\n10.0.0.0/24 10.0.1.0/24 10.0.2.0/24 172.16.0.0/16 172.16.1.0/24\nC\n"
	case "!a6AS-TEST\n":
		response = "A2\n2001:db8::/32 2001:db8:1::/48\nC\n"
	case "!a4AS-EMPTY\n", "!a6AS-EMPTY\n":
		response = "D\n"
	default:
		response = "D\n"
	}

	if _, err := fmt.Fprint(conn, response); err != nil {
		return
	}
}

// VALIDATES: AS-SET expansion returns all member ASNs.
// PREVENTS: missing ASNs from flat AS-SET.
func TestResolveASSet(t *testing.T) {
	addr, cleanup := fakeIRRServer(t, handleASSetQuery)
	defer cleanup()

	c := NewIRR(addr)
	asns, err := c.ResolveASSet(context.Background(), "AS-TEST")
	if err != nil {
		t.Fatalf("ResolveASSet: %v", err)
	}

	want := []uint32{65001, 65002, 65003}
	if len(asns) != len(want) {
		t.Fatalf("got %d ASNs, want %d: %v", len(asns), len(want), asns)
	}
	for i, asn := range asns {
		if asn != want[i] {
			t.Errorf("asns[%d] = %d, want %d", i, asn, want[i])
		}
	}
}

// VALIDATES: recursive AS-SET expansion resolves nested sets.
// PREVENTS: nested AS-SETs silently ignored.
func TestResolveASSetNested(t *testing.T) {
	addr, cleanup := fakeIRRServer(t, handleASSetQuery)
	defer cleanup()

	c := NewIRR(addr)
	asns, err := c.ResolveASSet(context.Background(), "AS-NESTED")
	if err != nil {
		t.Fatalf("ResolveASSet: %v", err)
	}

	want := []uint32{65001, 65002}
	if len(asns) != len(want) {
		t.Fatalf("got %d ASNs, want %d: %v", len(asns), len(want), asns)
	}
	for i, asn := range asns {
		if asn != want[i] {
			t.Errorf("asns[%d] = %d, want %d", i, asn, want[i])
		}
	}
}

// VALIDATES: cyclic AS-SET references terminate without infinite loop.
// PREVENTS: stack overflow from circular AS-SET references.
func TestResolveASSetCycle(t *testing.T) {
	addr, cleanup := fakeIRRServer(t, handleASSetQuery)
	defer cleanup()

	c := NewIRR(addr)
	asns, err := c.ResolveASSet(context.Background(), "AS-CYCLE")
	if err != nil {
		t.Fatalf("ResolveASSet: %v", err)
	}

	if len(asns) != 1 || asns[0] != 65001 {
		t.Errorf("got %v, want [65001]", asns)
	}
}

// VALIDATES: empty/invalid AS-SET returns no error and no ASNs.
// PREVENTS: error on non-existent AS-SET in IRR.
func TestResolveASSetEmpty(t *testing.T) {
	addr, cleanup := fakeIRRServer(t, handleASSetQuery)
	defer cleanup()

	c := NewIRR(addr)
	asns, err := c.ResolveASSet(context.Background(), "AS-EMPTY")
	if err != nil {
		t.Fatalf("ResolveASSet: %v", err)
	}

	if len(asns) != 0 {
		t.Errorf("got %v, want empty", asns)
	}
}

// VALIDATES: prefix lookup returns aggregated IPv4 and IPv6 prefixes.
// PREVENTS: missing prefixes or broken aggregation.
func TestLookupPrefixes(t *testing.T) {
	addr, cleanup := fakeIRRServer(t, handleASSetQuery)
	defer cleanup()

	c := NewIRR(addr)
	pl, err := c.LookupPrefixes(context.Background(), "AS-TEST")
	if err != nil {
		t.Fatalf("LookupPrefixes: %v", err)
	}

	// 172.16.1.0/24 is covered by 172.16.0.0/16, so aggregated away.
	// 2001:db8:1::/48 is covered by 2001:db8::/32, so aggregated away.
	if len(pl.IPv4) != 4 {
		t.Errorf("got %d IPv4 prefixes, want 4: %v", len(pl.IPv4), pl.IPv4)
	}
	if len(pl.IPv6) != 1 {
		t.Errorf("got %d IPv6 prefixes, want 1: %v", len(pl.IPv6), pl.IPv6)
	}

	// Check the aggregated IPv6 result.
	if len(pl.IPv6) > 0 {
		want := netip.MustParsePrefix("2001:db8::/32")
		if pl.IPv6[0] != want {
			t.Errorf("IPv6[0] = %s, want %s", pl.IPv6[0], want)
		}
	}
}

// VALIDATES: empty AS-SET returns empty prefix list, not error.
// PREVENTS: error on AS-SET with no announced prefixes.
func TestLookupPrefixesEmpty(t *testing.T) {
	addr, cleanup := fakeIRRServer(t, handleASSetQuery)
	defer cleanup()

	c := NewIRR(addr)
	pl, err := c.LookupPrefixes(context.Background(), "AS-EMPTY")
	if err != nil {
		t.Fatalf("LookupPrefixes: %v", err)
	}

	if !pl.Empty() {
		t.Errorf("expected empty, got IPv4=%d IPv6=%d", len(pl.IPv4), len(pl.IPv6))
	}
}

// VALIDATES: unreachable server returns error.
// PREVENTS: silent failure on network error.
func TestLookupPrefixesUnreachable(t *testing.T) {
	c := NewIRR("127.0.0.1:1") // port 1 should refuse connections
	_, err := c.LookupPrefixes(context.Background(), "AS-TEST")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

// VALIDATES: context cancellation stops the query.
// PREVENTS: hanging queries ignoring context.
func TestLookupPrefixesContextCancel(t *testing.T) {
	addr, cleanup := fakeIRRServer(t, func(conn net.Conn) {
		// Never respond, simulating slow server.
		buf := make([]byte, 4096)
		if _, err := conn.Read(buf); err != nil {
			return
		}
		// Hold connection open until test closes it.
	})
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	c := NewIRR(addr)
	_, err := c.LookupPrefixes(ctx, "AS-TEST")
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestParseASN(t *testing.T) {
	tests := []struct {
		input string
		want  uint32
		ok    bool
	}{
		{"AS65001", 65001, true},
		{"65001", 65001, true},
		{"as65001", 65001, true},
		{"AS4294967295", 4294967295, true}, // max uint32
		{"AS1", 1, true},
		{"AS0", 0, false},            // zero ASN invalid
		{"", 0, false},               // empty string
		{"AS", 0, false},             // prefix only
		{"ASFOO", 0, false},          // non-numeric after AS
		{"AS-SET", 0, false},         // AS-SET name, not ASN
		{"AS4294967296", 0, false},   // overflow: max uint32 + 1
		{"AS99999999999", 0, false},  // way too large
		{"  AS65001  ", 65001, true}, // whitespace trimmed
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := parseASN(tt.input)
			if ok != tt.ok || got != tt.want {
				t.Errorf("parseASN(%q) = (%d, %v), want (%d, %v)", tt.input, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestAggregateAndSort(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "no overlap",
			input: []string{"10.0.1.0/24", "10.0.0.0/24"},
			want:  []string{"10.0.0.0/24", "10.0.1.0/24"},
		},
		{
			name:  "covered by broader",
			input: []string{"172.16.0.0/16", "172.16.1.0/24"},
			want:  []string{"172.16.0.0/16"},
		},
		{
			name:  "duplicates removed",
			input: []string{"10.0.0.0/24", "10.0.0.0/24"},
			want:  []string{"10.0.0.0/24"},
		},
		{
			name:  "empty input",
			input: nil,
			want:  nil,
		},
		{
			name:  "ipv6 aggregation",
			input: []string{"2001:db8::/32", "2001:db8:1::/48", "2001:db8:2::/48"},
			want:  []string{"2001:db8::/32"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var input []netip.Prefix
			for _, s := range tt.input {
				input = append(input, netip.MustParsePrefix(s))
			}

			result := aggregateAndSort(input)

			if len(result) != len(tt.want) {
				t.Fatalf("got %d prefixes, want %d: %v", len(result), len(tt.want), result)
			}
			for i, p := range result {
				want := netip.MustParsePrefix(tt.want[i])
				if p != want {
					t.Errorf("result[%d] = %s, want %s", i, p, want)
				}
			}
		})
	}
}

func TestNewIRRDefaultServer(t *testing.T) {
	c := NewIRR("")
	if c.server != "whois.radb.net:43" {
		t.Errorf("default server = %q, want %q", c.server, "whois.radb.net:43")
	}
}

func TestNewIRRCustomPort(t *testing.T) {
	c := NewIRR("rr.ntt.net:4343")
	if c.server != "rr.ntt.net:4343" {
		t.Errorf("server = %q, want %q", c.server, "rr.ntt.net:4343")
	}
}

func TestNewIRRAutoPort(t *testing.T) {
	c := NewIRR("rr.ntt.net")
	if c.server != "rr.ntt.net:43" {
		t.Errorf("server = %q, want %q", c.server, "rr.ntt.net:43")
	}
}

// VALIDATES: isAnswerMarker correctly identifies RPSL answer codes.
// PREVENTS: answer markers parsed as ASNs or prefixes.
func TestIsAnswerMarker(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"A3", true},
		{"A125", true},
		{"A0", true},
		{"A", false},       // too short
		{"", false},        // empty
		{"B3", false},      // wrong prefix
		{"AS65001", false}, // AS prefix, not answer marker
		{"A3X", false},     // non-digit after initial digits
		{"ABC", false},     // all letters
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isAnswerMarker(tt.input)
			if got != tt.want {
				t.Errorf("isAnswerMarker(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// VALIDATES: validateASSetName rejects control characters (whois injection prevention).
// PREVENTS: RPSL command injection via newlines in AS-SET names.
func TestValidateASSetName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "AS-EXAMPLE", false},
		{"valid with source", "RIPE::AS-EXAMPLE", false},
		{"valid with dots", "AS-EXAMPLE.NET", false},
		{"valid with underscore", "AS_EXAMPLE", false},
		{"empty", "", true},
		{"newline injection", "AS-TEST\n!dAS-VICTIM", true},
		{"carriage return", "AS-TEST\r\n!d", true},
		{"null byte", "AS-TEST\x00", true},
		{"space", "AS TEST", true},
		{"semicolon", "AS-TEST;DROP", true},
		{"tab", "AS-TEST\t", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateASSetName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateASSetName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// VALIDATES: ResolveASSet rejects names with control characters.
// PREVENTS: whois command injection via user-supplied AS-SET names.
func TestResolveASSetInjection(t *testing.T) {
	addr, cleanup := fakeIRRServer(t, handleASSetQuery)
	defer cleanup()

	c := NewIRR(addr)
	_, err := c.ResolveASSet(context.Background(), "AS-TEST\n!dVICTIM")
	if err == nil {
		t.Fatal("expected error for injected AS-SET name")
	}
}

// VALIDATES: LookupPrefixes rejects names with control characters.
// PREVENTS: whois command injection via prefix lookup.
func TestLookupPrefixesInjection(t *testing.T) {
	addr, cleanup := fakeIRRServer(t, handleASSetQuery)
	defer cleanup()

	c := NewIRR(addr)
	_, err := c.LookupPrefixes(context.Background(), "AS-TEST\n!a4VICTIM")
	if err == nil {
		t.Fatal("expected error for injected AS-SET name")
	}
}

// VALIDATES: ResolveASSet returns error when recursion depth exceeds limit.
// PREVENTS: resource exhaustion from deeply nested AS-SET chains.
func TestResolveASSetDepthLimit(t *testing.T) {
	// Fake server returns a unique nested AS-SET for every query,
	// creating an unbounded chain: AS-DEEP-0 -> AS-DEEP-1 -> AS-DEEP-2 -> ...
	depthHandler := func(conn net.Conn) {
		defer func() { _ = conn.Close() }()

		buf := make([]byte, 4096)
		n, readErr := conn.Read(buf)
		if readErr != nil {
			return
		}
		query := string(buf[:n])

		// Extract the number from "!iAS-DEEP-N\n" and return "AS-DEEP-(N+1)".
		query = strings.TrimPrefix(query, "!iAS-DEEP-")
		query = strings.TrimSuffix(query, "\n")
		num, parseErr := strconv.Atoi(query)
		if parseErr != nil {
			if _, err := fmt.Fprint(conn, "D\n"); err != nil {
				return
			}
			return
		}

		next := fmt.Sprintf("AS-DEEP-%d", num+1)
		if _, err := fmt.Fprintf(conn, "A1\n%s\nC\n", next); err != nil {
			return
		}
	}

	addr, cleanup := fakeIRRServer(t, depthHandler)
	defer cleanup()

	c := NewIRR(addr)
	_, err := c.ResolveASSet(context.Background(), "AS-DEEP-0")
	if err == nil {
		t.Fatal("expected error for depth limit exceeded")
	}
	if !strings.Contains(err.Error(), "recursion depth exceeded") {
		t.Errorf("error should mention recursion depth, got: %v", err)
	}
}

// countingIRRServer wraps a handler and counts TCP connections.
func countingIRRServer(t *testing.T, counter *atomic.Int32, handler func(conn net.Conn)) (string, func()) {
	t.Helper()
	return fakeIRRServer(t, func(conn net.Conn) {
		counter.Add(1)
		handler(conn)
	})
}

// VALIDATES: AC-8 -- second ResolveASSet call returns cached result, no whois query.
// PREVENTS: redundant whois queries for recently resolved AS-SETs.
func TestResolveASSetCache(t *testing.T) {
	var hits atomic.Int32
	addr, cleanup := countingIRRServer(t, &hits, handleASSetQuery)
	defer cleanup()

	c := NewIRR(addr)

	first, err := c.ResolveASSet(context.Background(), "AS-TEST")
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("expected 1 TCP connection, got %d", hits.Load())
	}

	second, err := c.ResolveASSet(context.Background(), "AS-TEST")
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if hits.Load() != 1 {
		t.Errorf("expected cache hit (still 1 TCP connection), got %d", hits.Load())
	}
	if len(first) != len(second) {
		t.Errorf("cached result differs: first=%v, second=%v", first, second)
	}
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("cached result[%d]: got %d, want %d", i, second[i], first[i])
		}
	}
}

// VALIDATES: AC-8 -- second LookupPrefixes call returns cached result, no whois query.
// PREVENTS: redundant whois queries for recently resolved prefix lists.
func TestLookupPrefixesCache(t *testing.T) {
	var hits atomic.Int32
	addr, cleanup := countingIRRServer(t, &hits, handleASSetQuery)
	defer cleanup()

	c := NewIRR(addr)

	first, err := c.LookupPrefixes(context.Background(), "AS-TEST")
	if err != nil {
		t.Fatalf("first lookup: %v", err)
	}
	// LookupPrefixes makes 2 TCP connections (one for !a4, one for !a6).
	initialHits := hits.Load()
	if initialHits < 1 {
		t.Fatalf("expected at least 1 TCP connection, got %d", initialHits)
	}

	second, err := c.LookupPrefixes(context.Background(), "AS-TEST")
	if err != nil {
		t.Fatalf("second lookup: %v", err)
	}
	if hits.Load() != initialHits {
		t.Errorf("expected cache hit (still %d TCP connections), got %d", initialHits, hits.Load())
	}
	if len(first.IPv4) != len(second.IPv4) || len(first.IPv6) != len(second.IPv6) {
		t.Errorf("cached result differs: first=%+v, second=%+v", first, second)
	}
}
