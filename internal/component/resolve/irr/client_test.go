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

// TestIRRSourceAddress verifies SetSourceAddress records the local IP and that
// query applies it as the dialer's local source (so a non-local address fails
// the bind rather than being silently ignored).
//
// VALIDATES: AC-11/AC-12 -- IRR whois queries bind the configured source-address.
// PREVENTS: source-address being stored but never used by the dialer.
func TestIRRSourceAddress(t *testing.T) {
	c := NewIRR("192.0.2.2:43")
	if c.sourceAddress != "" {
		t.Fatalf("new client sourceAddress = %q, want empty", c.sourceAddress)
	}

	c.SetSourceAddress("198.51.100.1")
	if c.sourceAddress != "198.51.100.1" {
		t.Fatalf("sourceAddress = %q, want %q", c.sourceAddress, "198.51.100.1")
	}

	// A source-address not assigned to any local interface must fail the TCP
	// bind, proving query() applies it as LocalAddr. 192.0.2.9 is RFC 5737
	// TEST-NET-1 (not a local address); the remote is a literal IP:port so no
	// DNS is needed and the bind is what fails.
	c.SetSourceAddress("192.0.2.9")
	if _, err := c.query(context.Background(), "!gAS-TEST\n"); err == nil {
		t.Fatal("expected connect error for non-local source-address, got nil")
	} else if !strings.Contains(err.Error(), "connect") {
		t.Errorf("error = %v, want a connect/bind failure", err)
	}
}

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

// VALIDATES: AC-1 -- the four RPSL reply shapes are distinguishable. A server
// error and a truncated reply are errors; a key-not-found and a query that
// succeeded with no records are empty answers, not failures.
// PREVENTS: "I have nothing" and "the answer is nothing" collapsing into one
// state that empties a live prefix filter.
func TestLookupPrefixesDistinguishesEmptyFromData(t *testing.T) {
	tests := []struct {
		name      string
		reply     string
		wantErr   bool
		wantCount int
	}{
		{"records", "A1\n10.0.0.0/24\nC\n", false, 1},
		{"key-not-found", "D\n", false, 0},
		{"query-ok-no-records", "C\n", false, 0},
		{"multiple-copies-of-key", "E\n", true, 0},
		{"server-error", "F access denied\n", true, 0},
		{"truncated-no-status-line", "A1\n10.0.0.0/24\n", true, 0},
		{"truncated-last-record-starts-with-c", "A2\n10.0.0.0/24\nCUSTOMER-ROUTE\n", true, 0},
		{"nothing-at-all", "", true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, cleanup := fakeIRRServer(t, replyWith(tt.reply))
			defer cleanup()

			pl, err := NewIRR(addr).LookupPrefixes(context.Background(), "AS-SUBJECT")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("reply %q returned no error; a server that answered nothing usable must not read as an empty prefix list", tt.reply)
				}
				return
			}
			if err != nil {
				t.Fatalf("LookupPrefixes: %v", err)
			}
			if len(pl.IPv4) != tt.wantCount {
				t.Errorf("IPv4 count = %d, want %d", len(pl.IPv4), tt.wantCount)
			}
		})
	}
}

// VALIDATES: AC-2 -- an answer that carried no prefixes is not written into the
// one-hour cache, so the next lookup reaches the server again.
// PREVENTS: an operator forcing a refresh and getting the same empty answer
// with no network query.
func TestLookupPrefixesDoesNotCacheEmptyAnswer(t *testing.T) {
	var served atomic.Int32
	addr, cleanup := fakeIRRServer(t, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()
		buf := make([]byte, 4096)
		if _, err := conn.Read(buf); err != nil {
			return
		}
		// The first pair of family queries answers "key not found"; every
		// later query answers with a record.
		reply := "A1\n10.0.0.0/24\nC\n"
		if served.Add(1) <= 2 {
			reply = "D\n"
		}
		if _, err := fmt.Fprint(conn, reply); err != nil {
			return
		}
	})
	defer cleanup()

	c := NewIRR(addr)
	first, err := c.LookupPrefixes(context.Background(), "AS-SUBJECT")
	if err != nil {
		t.Fatalf("first LookupPrefixes: %v", err)
	}
	if !first.Empty() {
		t.Fatalf("first lookup should be empty, got %v", first.IPv4)
	}

	second, err := c.LookupPrefixes(context.Background(), "AS-SUBJECT")
	if err != nil {
		t.Fatalf("second LookupPrefixes: %v", err)
	}
	if second.Empty() {
		t.Fatal("the empty answer was cached: the second lookup never reached the server")
	}
}

// VALIDATES: AC-8 -- the read-only operator lookups keep the 1h cache, so a
// show command costs one query per hour and not one per invocation.
// PREVENTS: the refresh path losing its cache read and taking the show commands
// with it.
func TestLookupPrefixesServesReadOnlyCallersFromCache(t *testing.T) {
	var served atomic.Int32
	addr, cleanup := fakeIRRServer(t, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()
		buf := make([]byte, 4096)
		if _, err := conn.Read(buf); err != nil {
			return
		}
		// The first pair of family queries carries a record; a later query
		// answers "key not found", so a cache miss is visible in the result.
		reply := "D\n"
		if served.Add(1) <= 2 {
			reply = "A1\n10.0.0.0/24\nC\n"
		}
		if _, err := fmt.Fprint(conn, reply); err != nil {
			return
		}
	})
	defer cleanup()

	c := NewIRR(addr)
	if _, err := c.LookupPrefixes(context.Background(), "AS-SUBJECT"); err != nil {
		t.Fatalf("first LookupPrefixes: %v", err)
	}

	second, err := c.LookupPrefixes(context.Background(), "AS-SUBJECT")
	if err != nil {
		t.Fatalf("second LookupPrefixes: %v", err)
	}
	if len(second.IPv4) != 1 {
		t.Fatalf("second lookup v4=%d, want the cached 1", len(second.IPv4))
	}
	if got := served.Load(); got != 2 {
		t.Fatalf("server saw %d queries, want 2: the cached answer was not served", got)
	}
}

// VALIDATES: AC-8 -- RefreshPrefixes queries the server inside the cache TTL,
// and stores what it learned for the read-only lookups.
// PREVENTS: an operator refresh answered from memory, and a refresh that
// reaches the server while the show commands keep the answer it replaced.
func TestRefreshPrefixesAlwaysQueriesServer(t *testing.T) {
	var served atomic.Int32
	addr, cleanup := fakeIRRServer(t, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()
		buf := make([]byte, 4096)
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		reply := "D\n" // IPv6 holds no route objects in either round
		if strings.TrimSpace(string(buf[:n])) == "!a4AS-SUBJECT" {
			reply = "A1\n10.0.0.0/24\nC\n"
			if served.Add(1) > 1 {
				reply = "A1\n192.0.2.0/24\nC\n"
			}
		}
		if _, err := fmt.Fprint(conn, reply); err != nil {
			return
		}
	})
	defer cleanup()

	c := NewIRR(addr)
	if _, err := c.LookupPrefixes(context.Background(), "AS-SUBJECT"); err != nil {
		t.Fatalf("seeding LookupPrefixes: %v", err)
	}

	refreshed, err := c.RefreshPrefixes(context.Background(), "AS-SUBJECT")
	if err != nil {
		t.Fatalf("RefreshPrefixes: %v", err)
	}
	if len(refreshed.IPv4) != 1 || refreshed.IPv4[0].String() != "192.0.2.0/24" {
		t.Fatalf("RefreshPrefixes = %v, want the second answer 192.0.2.0/24: it read the cache", refreshed.IPv4)
	}

	cached, err := c.LookupPrefixes(context.Background(), "AS-SUBJECT")
	if err != nil {
		t.Fatalf("LookupPrefixes after refresh: %v", err)
	}
	if len(cached.IPv4) != 1 || cached.IPv4[0].String() != "192.0.2.0/24" {
		t.Fatalf("cached after refresh = %v, want 192.0.2.0/24: the refresh did not update the cache", cached.IPv4)
	}
}

// VALIDATES: the RPSL status line is read, not skipped as an unparseable word.
// PREVENTS: a server error, a not-found and a genuine empty answer collapsing
// into one indistinguishable state.
func TestParseReply(t *testing.T) {
	tests := []struct {
		reply       string
		wantStatus  replyStatus
		wantPayload string
	}{
		{"A1\n10.0.0.0/24\nC\n", replyOK, "10.0.0.0/24"},
		{"C\n", replyOK, ""},
		{"D\n", replyNotFound, ""},
		{"E\n", replyFailed, ""},
		{"F access denied\n", replyFailed, ""},
		{"A1\n10.0.0.0/24\n", replyFailed, ""},
		{"A2\n10.0.0.0/24\nCUSTOMER-ROUTE\n", replyFailed, ""},
		{"F\n", replyFailed, ""},
		{"", replyFailed, ""},
	}
	for _, tt := range tests {
		t.Run(tt.reply, func(t *testing.T) {
			payload, status, detail := parseReply(tt.reply)
			if status != tt.wantStatus {
				t.Errorf("status = %v, want %v", status, tt.wantStatus)
			}
			if strings.TrimSpace(payload) != tt.wantPayload {
				t.Errorf("payload = %q, want %q", payload, tt.wantPayload)
			}
			if status == replyFailed && detail == "" {
				t.Error("a failed reply must carry a reason for the operator")
			}
		})
	}
}

// VALIDATES: a server error during AS-SET expansion is an error, not an empty
// member list.
// PREVENTS: a filter built from a silently truncated AS-SET expansion.
func TestResolveASSetServerError(t *testing.T) {
	addr, cleanup := fakeIRRServer(t, replyWith("F database unavailable\n"))
	defer cleanup()

	if _, err := NewIRR(addr).ResolveASSet(context.Background(), "AS-SUBJECT"); err == nil {
		t.Fatal("a server error must not read as an AS-SET with no members")
	}
}

// replyWith returns a handler that answers every query with the same raw reply.
func replyWith(reply string) func(net.Conn) {
	return func(conn net.Conn) {
		defer func() { _ = conn.Close() }()
		buf := make([]byte, 4096)
		if _, err := conn.Read(buf); err != nil {
			return
		}
		if _, err := fmt.Fprint(conn, reply); err != nil {
			return
		}
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
		if _, err := fmt.Fprintf(conn, "A1\n%s\nC\n", next); err != nil { //nolint:errcheck // output
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
