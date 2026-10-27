package bgp

import (
	"encoding/json"
	"strings"
	"testing"
)

// kdigReply is kdig 3.6's default output for one reply, in its own shape.
const kdigReply = `;; ->>HEADER<<- opcode: QUERY; status: NOERROR; id: 4242
;; Flags: qr rd ra; QUERY: 1; ANSWER: 2; AUTHORITY: 0; ADDITIONAL: 1

;; EDNS PSEUDOSECTION:
;; Version: 0; flags: ; UDP size: 1232 B; ext-rcode: NOERROR

;; QUESTION SECTION:
;; www.lab.example.    		IN	A

;; ANSWER SECTION:
www.lab.example.    	300	IN	A	192.0.2.11
www.lab.example.    	299	IN	A	192.0.2.10

;; Received 76 B
;; Time 2026-10-27 20:00:00 UTC
;; From 172.30.0.2@53(UDP) in 0.4 ms
`

// TestParseKdigReadsStatusFlagsAndRRset checks the kdig reader every DNS
// assertion rests on: the status, the header flags, and the answer RRset
// sorted with the TTL left out, so a cached answer whose TTL counted down
// still equals the first.
func TestParseKdigReadsStatusFlagsAndRRset(t *testing.T) {
	got, err := parseKdig(kdigReply)
	if err != nil {
		t.Fatal(err)
	}
	if err := dnsAnswerVerdict(got, "NOERROR", dnsWWWRecords); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.flags, " ") != "qr rd ra" {
		t.Fatalf("flags %v, want qr rd ra", got.flags)
	}
	if err := truncatedVerdict(got); err == nil {
		t.Fatal("a reply without tc passed the truncation verdict")
	}
	tc, err := parseKdig(strings.Replace(kdigReply, "qr rd ra", "qr tc rd ra", 1))
	if err != nil {
		t.Fatal(err)
	}
	if err := truncatedVerdict(tc); err != nil {
		t.Fatal(err)
	}
}

// TestParseKdigRefusesNoReply checks that kdig's output for a server that
// never answered is an error, not an empty NOERROR-less answer.
func TestParseKdigRefusesNoReply(t *testing.T) {
	if _, err := parseKdig(";; WARNING: failed to query server 172.30.0.2@53(UDP)\n"); err == nil {
		t.Fatal("output with no reply header parsed as a reply")
	}
}

// TestDNSAnswerVerdicts drives both polarities of the RCODE and RRset
// comparisons: a wrong status, a missing record, an extra record, and ze
// differing from Unbound each fail; equal answers pass.
func TestDNSAnswerVerdicts(t *testing.T) {
	got, err := parseKdig(kdigReply)
	if err != nil {
		t.Fatal(err)
	}
	if dnsAnswerVerdict(got, "SERVFAIL", dnsWWWRecords) == nil {
		t.Fatal("wrong status passed")
	}
	if dnsAnswerVerdict(got, "NOERROR", dnsWWWRecords[:1]) == nil {
		t.Fatal("an extra record passed")
	}
	if dnsAnswerVerdict(got, "NOERROR", append([]string{"A 192.0.2.12"}, dnsWWWRecords...)) == nil {
		t.Fatal("a missing record passed")
	}
	if dnsAnswerVerdict(got, "NOERROR", nil) == nil {
		t.Fatal("a non-empty answer passed as empty")
	}
	if err := sameAnswerVerdict(got, got); err != nil {
		t.Fatal(err)
	}
	refused := kdigAnswer{status: "REFUSED"}
	if sameAnswerVerdict(got, refused) == nil {
		t.Fatal("different status passed as the same answer")
	}
	if err := dnsAnswerVerdict(refused, "REFUSED", nil); err != nil {
		t.Fatal(err)
	}
	fewer := kdigAnswer{status: "NOERROR", rrs: got.rrs[:1]}
	if sameAnswerVerdict(got, fewer) == nil {
		t.Fatal("different RRset passed as the same answer")
	}
}

// TestUnboundQueryCount checks that only Unbound's query lines from the
// named source for the exact name and type count.
func TestUnboundQueryCount(t *testing.T) {
	log := strings.Join([]string{
		"[1761595200] unbound[1:0] info: 172.30.0.2 www.lab.example. A IN",
		"[1761595200] unbound[1:0] info: 172.30.0.42 www.lab.example. A IN",
		"[1761595200] unbound[1:0] info: 172.30.0.2 www.lab.example. AAAA IN",
		"[1761595200] unbound[1:0] info: 172.30.0.2 www.lab.example. A IN NOERROR 0.000 1 76",
		"[1761595201] unbound[1:0] info: 172.30.0.2 www.lab.example. A IN",
	}, "\n")
	if n := unboundQueryCount(log, "172.30.0.2", dnsNameWWW, "A"); n != 2 {
		t.Fatalf("counted %d, want 2", n)
	}
}

// TestUpstreamTruncatedOf checks that the truncated count is read from the
// upstream-queries object at any depth, and that a missing object or a
// missing count is an error rather than zero.
func TestUpstreamTruncatedOf(t *testing.T) {
	cases := []struct {
		name, raw string
		want      uint64
		fails     bool
	}{
		{"nested", `{"dns-resolver":{"upstream-queries":{"answered":4,"truncated":1}}}`, 1, false},
		{"zero", `{"upstream-queries":{"truncated":0}}`, 0, false},
		{"no object", `{"queries":3}`, 0, true},
		{"no count", `{"upstream-queries":{"answered":4}}`, 0, true},
	}
	for _, tc := range cases {
		var decoded any
		if err := json.Unmarshal([]byte(tc.raw), &decoded); err != nil {
			t.Fatal(err)
		}
		got, err := upstreamTruncatedOf(decoded, tc.raw)
		if tc.fails {
			if err == nil {
				t.Fatalf("%s: answered %d, want an error", tc.name, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Fatalf("%s: got %d, %v; want %d", tc.name, got, err, tc.want)
		}
	}
}

// TestDhcpcdLeaseVerdict reads dhcpcd's lease dump and holds it to the
// pool address and exactly ze as the DNS server: a second server, a wrong
// server, a wrong address and a dump with no lease each fail.
func TestDhcpcdLeaseVerdict(t *testing.T) {
	dump := "broadcast_address='172.30.0.255'\ndhcp_lease_time='3600'\n" +
		"domain_name_servers='172.30.0.2'\nip_address='172.30.0.60'\nsubnet_mask='255.255.255.0'\n"
	lease, err := parseDhcpcdLease(dump)
	if err != nil {
		t.Fatal(err)
	}
	if err := dhcpLeaseVerdict(lease, "172.30.0.60", "172.30.0.2"); err != nil {
		t.Fatal(err)
	}
	if dhcpLeaseVerdict(lease, "172.30.0.61", "172.30.0.2") == nil {
		t.Fatal("wrong address passed")
	}
	if dhcpLeaseVerdict(lease, "172.30.0.60", "172.30.0.3") == nil {
		t.Fatal("wrong server passed")
	}
	two, err := parseDhcpcdLease(strings.Replace(dump, "'172.30.0.2'", "'172.30.0.2 172.30.0.3'", 1))
	if err != nil {
		t.Fatal(err)
	}
	if dhcpLeaseVerdict(two, "172.30.0.60", "172.30.0.2") == nil {
		t.Fatal("a second server passed")
	}
	if _, err := parseDhcpcdLease(""); err == nil {
		t.Fatal("an empty dump parsed as a lease")
	}
}
