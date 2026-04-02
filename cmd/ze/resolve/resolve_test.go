// Design: docs/architecture/resolve.md -- end-to-end tests for resolve CLI
package resolve

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	mdns "github.com/miekg/dns"
)

// captureRun calls Run and captures stdout + stderr.
func captureRun(args ...string) (code int, stdout, stderr string) {
	origOut, origErr := os.Stdout, os.Stderr

	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	code = Run(args)

	if err := wOut.Close(); err != nil {
		return -1, "", fmt.Sprintf("close stdout pipe: %v", err)
	}
	if err := wErr.Close(); err != nil {
		return -1, "", fmt.Sprintf("close stderr pipe: %v", err)
	}

	var bufOut, bufErr bytes.Buffer
	if _, err := io.Copy(&bufOut, rOut); err != nil {
		return -1, "", fmt.Sprintf("read stdout: %v", err)
	}
	if _, err := io.Copy(&bufErr, rErr); err != nil {
		return -1, "", fmt.Sprintf("read stderr: %v", err)
	}
	if err := rOut.Close(); err != nil {
		return -1, "", fmt.Sprintf("close stdout reader: %v", err)
	}
	if err := rErr.Close(); err != nil {
		return -1, "", fmt.Sprintf("close stderr reader: %v", err)
	}

	os.Stdout = origOut
	os.Stderr = origErr

	return code, bufOut.String(), bufErr.String()
}

// VALIDATES: AC-1 -- dns a record lookup returns addresses via CLI.
// PREVENTS: CLI not wired to DNS resolver.
func TestCmdDNSA(t *testing.T) {
	addr := startFakeDNS(t)

	code, stdout, _ := captureRun("dns", "--server", addr, "a", "example.com")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "192.0.2.1") {
		t.Errorf("stdout missing 192.0.2.1: %s", stdout)
	}
}

// VALIDATES: AC-7 -- peeringdb max-prefix returns prefix counts via CLI.
// PREVENTS: CLI not wired to PeeringDB resolver.
func TestCmdPeeringDBMaxPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(fakePeeringDBHandler))
	defer srv.Close()

	code, stdout, _ := captureRun("peeringdb", "--url", srv.URL, "max-prefix", "65001")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "ipv4: 65001") {
		t.Errorf("stdout missing 'ipv4: 65001': %s", stdout)
	}
	if !strings.Contains(stdout, "ipv6: 13000") {
		t.Errorf("stdout missing 'ipv6: 13000': %s", stdout)
	}
}

// VALIDATES: AC-9 -- peeringdb as-set returns AS-SET names via CLI.
// PREVENTS: CLI not wired to PeeringDB AS-SET lookup.
func TestCmdPeeringDBASSet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(fakePeeringDBHandler))
	defer srv.Close()

	code, stdout, _ := captureRun("peeringdb", "--url", srv.URL, "as-set", "65001")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "AS-TEST") {
		t.Errorf("stdout missing 'AS-TEST': %s", stdout)
	}
}

// VALIDATES: AC-5 -- cymru asn-name returns org name via CLI with fake DNS.
// PREVENTS: CLI not wired to Cymru resolver.
func TestCmdCymruASNName(t *testing.T) {
	addr := startFakeCymruDNS(t)

	code, stdout, _ := captureRun("cymru", "--dns-server", addr, "asn-name", "13335")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "Cloudflare, Inc.") {
		t.Errorf("stdout missing 'Cloudflare, Inc.': %s", stdout)
	}
}

// VALIDATES: AC-11 -- irr as-set expansion returns ASNs via CLI.
// PREVENTS: CLI not wired to IRR resolver.
func TestCmdIRRASSet(t *testing.T) {
	addr := startFakeIRR(t)

	code, stdout, _ := captureRun("irr", "--server", addr, "as-set", "AS-TEST")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	for _, asn := range []string{"AS65001", "AS65002", "AS65003"} {
		if !strings.Contains(stdout, asn) {
			t.Errorf("stdout missing %s: %s", asn, stdout)
		}
	}
}

// VALIDATES: AC-13 -- irr prefix lookup returns prefixes via CLI.
// PREVENTS: CLI not wired to IRR prefix lookup.
func TestCmdIRRPrefix(t *testing.T) {
	addr := startFakeIRR(t)

	code, stdout, _ := captureRun("irr", "--server", addr, "prefix", "AS-TEST")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "10.0.0.0/24") {
		t.Errorf("stdout missing 10.0.0.0/24: %s", stdout)
	}
	if !strings.Contains(stdout, "2001:db8::/32") {
		t.Errorf("stdout missing 2001:db8::/32: %s", stdout)
	}
}

// VALIDATES: AC-20 -- peeringdb not found returns error.
// PREVENTS: zero exit code on not-found ASN.
func TestCmdPeeringDBNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(fakePeeringDBHandler))
	defer srv.Close()

	code, _, stderr := captureRun("peeringdb", "--url", srv.URL, "max-prefix", "0")
	if code != exitError {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "not found") {
		t.Errorf("stderr missing 'not found': %s", stderr)
	}
}

// VALIDATES: AC-12 -- irr empty AS-SET prints message.
// PREVENTS: silent empty output for non-existent AS-SET.
func TestCmdIRRASSetEmpty(t *testing.T) {
	addr := startFakeIRR(t)

	code, _, stderr := captureRun("irr", "--server", addr, "as-set", "AS-NONEXISTENT")
	if code != exitError {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "no members found") {
		t.Errorf("stderr missing 'no members found': %s", stderr)
	}
}

// VALIDATES: AC-15 -- help prints structured help, exits 0.
// PREVENTS: help flag not handled.
func TestCmdHelp(t *testing.T) {
	code, _, stderr := captureRun("--help")
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stderr, "Services:") {
		t.Errorf("help missing 'Services:': %s", stderr)
	}
}

// VALIDATES: AC-16 -- dns subcommand help exits 0.
// PREVENTS: --help treated as unknown operation.
func TestCmdDNSHelp(t *testing.T) {
	code, _, stderr := captureRun("dns", "--help")
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stderr, "Operations:") {
		t.Errorf("help missing 'Operations:': %s", stderr)
	}
}

// VALIDATES: AC-21 -- unknown subcommand exits 1.
// PREVENTS: panic on invalid subcommand.
func TestCmdUnknown(t *testing.T) {
	code, _, stderr := captureRun("foo")
	if code != exitError {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "unknown resolve subcommand") {
		t.Errorf("stderr missing error: %s", stderr)
	}
}

// VALIDATES: AC-19 -- invalid ASN exits 1.
// PREVENTS: panic on non-numeric ASN.
func TestCmdCymruInvalidASN(t *testing.T) {
	code, _, stderr := captureRun("cymru", "asn-name", "abc")
	if code != exitError {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "invalid ASN") {
		t.Errorf("stderr missing 'invalid ASN': %s", stderr)
	}
}

// VALIDATES: AC-10 -- peeringdb as-set with no AS-SET registered exits 1.
// PREVENTS: silent empty output for ASN with no AS-SET.
func TestCmdPeeringDBASSetEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(fakePeeringDBHandler))
	defer srv.Close()

	// Fake: odd ASN -> "AS-TEST", even ASN -> empty irr_as_set.
	code, _, stderr := captureRun("peeringdb", "--url", srv.URL, "as-set", "2")
	if code != exitError {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "no AS-SET registered") {
		t.Errorf("stderr missing 'no AS-SET registered': %s", stderr)
	}
}

// fakePeeringDBHandler serves deterministic PeeringDB responses.
func fakePeeringDBHandler(w http.ResponseWriter, r *http.Request) {
	asnStr := r.URL.Query().Get("asn")
	if asnStr == "" {
		http.Error(w, `{"data":[]}`, http.StatusBadRequest)
		return
	}

	var asn uint64
	for _, c := range asnStr {
		if c < '0' || c > '9' {
			http.Error(w, `{"data":[]}`, http.StatusBadRequest)
			return
		}
		asn = asn*10 + uint64(c-'0')
	}

	w.Header().Set("Content-Type", "application/json")

	if asn == 0 {
		if _, wErr := io.WriteString(w, `{"data":[]}`); wErr != nil {
			return
		}
		return
	}

	ipv4 := asn
	ipv6 := asn / 5

	var irrASSet string
	if asn%2 == 1 {
		irrASSet = "AS-TEST"
	}

	if _, wErr := fmt.Fprintf(w,
		`{"data":[{"asn":%d,"info_prefixes4":%d,"info_prefixes6":%d,"irr_as_set":"%s"}]}`,
		asn, ipv4, ipv6, irrASSet); wErr != nil {
		return
	}
}

// startFakeDNS starts a fake UDP DNS server returning deterministic A records.
func startFakeDNS(t *testing.T) string {
	t.Helper()

	lc := &net.ListenConfig{}
	pc, err := lc.ListenPacket(context.Background(), "udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	srv := &mdns.Server{
		PacketConn: pc,
		Handler: mdns.HandlerFunc(func(w mdns.ResponseWriter, r *mdns.Msg) {
			m := new(mdns.Msg)
			m.SetReply(r)
			if len(r.Question) > 0 && r.Question[0].Qtype == mdns.TypeA {
				m.Answer = append(m.Answer, &mdns.A{
					Hdr: mdns.RR_Header{
						Name:   r.Question[0].Name,
						Rrtype: mdns.TypeA,
						Class:  mdns.ClassINET,
						Ttl:    300,
					},
					A: net.ParseIP("192.0.2.1"),
				})
			}
			_ = w.WriteMsg(m)
		}),
	}

	go func() { _ = srv.ActivateAndServe() }()
	t.Cleanup(func() { _ = srv.Shutdown() })

	return pc.LocalAddr().String()
}

// startFakeCymruDNS starts a fake UDP DNS server returning Cymru-formatted TXT records.
func startFakeCymruDNS(t *testing.T) string {
	t.Helper()

	lc := &net.ListenConfig{}
	pc, err := lc.ListenPacket(context.Background(), "udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	srv := &mdns.Server{
		PacketConn: pc,
		Handler: mdns.HandlerFunc(func(w mdns.ResponseWriter, r *mdns.Msg) {
			m := new(mdns.Msg)
			m.SetReply(r)
			if len(r.Question) > 0 && r.Question[0].Qtype == mdns.TypeTXT {
				m.Answer = append(m.Answer, &mdns.TXT{
					Hdr: mdns.RR_Header{
						Name:   r.Question[0].Name,
						Rrtype: mdns.TypeTXT,
						Class:  mdns.ClassINET,
						Ttl:    300,
					},
					Txt: []string{"13335 | US | arin | 2010-07-14 | CLOUDFLARENET - Cloudflare, Inc., US"},
				})
			}
			_ = w.WriteMsg(m)
		}),
	}

	go func() { _ = srv.ActivateAndServe() }()
	t.Cleanup(func() { _ = srv.Shutdown() })

	return pc.LocalAddr().String()
}

// startFakeIRR starts a fake TCP whois server for IRR queries.
func startFakeIRR(t *testing.T) string {
	t.Helper()

	lc := &net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	responses := map[string]string{
		"!iAS-TEST":  "A3\nAS65001 AS65002 AS65003\nC\n",
		"!a4AS-TEST": "A3\n10.0.0.0/24 10.0.1.0/24 172.16.0.0/16\nC\n",
		"!a6AS-TEST": "A1\n2001:db8::/32\nC\n",
	}

	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				buf := make([]byte, 4096)
				n, readErr := c.Read(buf)
				if readErr != nil {
					return
				}
				query := strings.TrimSpace(string(buf[:n]))
				resp, ok := responses[query]
				if !ok {
					resp = "D\n"
				}
				if _, writeErr := fmt.Fprint(c, resp); writeErr != nil {
					return
				}
			}(conn)
		}
	}()

	t.Cleanup(func() { _ = ln.Close() })

	return ln.Addr().String()
}
