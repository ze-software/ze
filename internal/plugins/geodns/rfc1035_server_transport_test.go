// Design: docs/architecture/dns/server-harness.md -- the datagram size bound
// lives at the harness's single write. It classifies the transport from the
// remote address miekg/dns reports. This file drives it over real sockets,
// which is the only way to see what that address is.
// RFC: rfc/short/rfc1035.md -- UDP message size and the TC bit

package geodns

import (
	"net"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// bigZoneConfig returns a geodns config serving one host with 40 A records,
// which packs to about 1300 octets uncompressed and about 650 compressed, so the
// reply exceeds the 512-octet datagram bound either way.
func bigZoneConfig(t *testing.T, port uint16) geodnsConfig {
	t.Helper()
	addrs := make([]string, 0, 40)
	for i := 1; i <= 40; i++ {
		addrs = append(addrs, `"10.0.0.`+strconv.Itoa(i)+`"`)
	}
	cfg, err := parseConfig(`{"service":{"geodns":{"enabled":"true","zone":["test.example."],` +
		`"host-set":{"big":{"host":{"big.test.example.":{"address":[` + strings.Join(addrs, ",") + `]}}}},` +
		`"source":{"0.0.0.0/0":{"host-set":"big"}}}}}`)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	cfg.Listeners = []listenerEndpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}}
	return cfg
}

// VALIDATES: over a real listener, one query draws a datagram of at most 512
// octets with TC set, and the same query over TCP draws every record with TC
// clear. The datagram half measures the octets that left the socket, so it holds
// RFC 1035 section 4.2.1's bound itself rather than a belief about it.
// PREVENTS: the transport rule being proven only against a hand-written
// ResponseWriter. The harness classifies a transport from the remote address
// miekg/dns reports, and only a real listener shows what that address is on each
// transport.
func TestRFC1035_UDPTruncatedTCPWholeOverRealSockets(t *testing.T) {
	port := freePort(t)
	cfg := bigZoneConfig(t, port)
	storeApplied(cfg, 1)
	mgr := newServerManager(testLogger())
	if err := mgr.apply(cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	t.Cleanup(mgr.stopAll)

	q := new(dns.Msg)
	q.SetQuestion("big.test.example.", dns.TypeA)
	packed, err := q.Pack()
	if err != nil {
		t.Fatalf("pack query: %v", err)
	}

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("client socket: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	server := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(port)}
	if _, err := conn.WriteToUDP(packed, server); err != nil {
		t.Fatalf("send query: %v", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	buf := make([]byte, 4096)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("read reply: %v", err)
	}
	// RFC requirement: RFC1035-4.2.1-1 positive -- the datagram that left the
	// socket is at most 512 octets, "not counting the IP or UDP headers".
	if n > dns.MinMsgSize {
		t.Errorf("UDP datagram is %d octets, want at most %d", n, dns.MinMsgSize)
	}
	udpReply := new(dns.Msg)
	if err := udpReply.Unpack(buf[:n]); err != nil {
		t.Fatalf("unpack UDP reply: %v", err)
	}
	// RFC requirement: RFC1035-4.2.1-2 positive -- the shortened message carries
	// the TC bit, which is what tells the client to retry over a stream.
	if !udpReply.Truncated {
		t.Error("UDP reply has TC clear though it was shortened")
	}
	if len(udpReply.Answer) == 0 {
		t.Error("UDP reply carries no answer at all; only the records that do not fit may be dropped")
	}

	client := &dns.Client{Net: "tcp", Timeout: 5 * time.Second}
	tcpReply, _, err := client.Exchange(q, server.String())
	if err != nil {
		t.Fatalf("tcp exchange: %v", err)
	}
	// RFC requirement: RFC1035-4.2.1-2 negative -- the same query over a stream
	// transport draws every record, with TC clear.
	if tcpReply.Truncated {
		t.Error("TCP reply has TC set, want the whole answer")
	}
	if len(tcpReply.Answer) != 40 {
		t.Errorf("TCP reply carries %d answers, want all 40", len(tcpReply.Answer))
	}
	if len(tcpReply.Answer) <= len(udpReply.Answer) {
		t.Errorf("TCP reply carries %d answers and UDP %d; the datagram bound dropped nothing, so this test proved nothing",
			len(tcpReply.Answer), len(udpReply.Answer))
	}
}
