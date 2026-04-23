package cmd

import (
	"net/netip"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
)

func TestFormatTablesEmpty(t *testing.T) {
	got := FormatTables(nil)
	if got != "No firewall tables configured." {
		t.Errorf("got %q, want empty message", got)
	}
}

func TestFormatTablesWithChain(t *testing.T) {
	tables := []firewall.Table{{
		Name:   "ze_wan",
		Family: firewall.FamilyInet,
		Chains: []firewall.Chain{{
			Name:     "input",
			IsBase:   true,
			Type:     firewall.ChainFilter,
			Hook:     firewall.HookInput,
			Priority: 0,
			Policy:   firewall.PolicyDrop,
			Terms: []firewall.Term{{
				Name:    "allow-ssh",
				Matches: []firewall.Match{firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 22, Hi: 22}}}},
				Actions: []firewall.Action{firewall.Accept{}},
			}},
		}},
	}}
	got := FormatTables(tables)
	if !strings.Contains(got, "table inet wan") {
		t.Errorf("missing table header in output:\n%s", got)
	}
	if !strings.Contains(got, "chain input") {
		t.Errorf("missing chain in output:\n%s", got)
	}
	if !strings.Contains(got, "destination port 22") {
		t.Errorf("missing match in output:\n%s", got)
	}
	if !strings.Contains(got, "accept") {
		t.Errorf("missing action in output:\n%s", got)
	}
	if !strings.Contains(got, "policy drop") {
		t.Errorf("missing policy in output:\n%s", got)
	}
}

func TestFormatMatchTypes(t *testing.T) {
	tests := []struct {
		name string
		m    firewall.Match
		want string
	}{
		{"source addr", firewall.MatchSourceAddress{Prefix: netip.MustParsePrefix("10.0.0.0/8")}, "source address 10.0.0.0/8"},
		{"dest port", firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 443, Hi: 443}}}, "destination port 443"},
		{"port range", firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 80, Hi: 90}}}, "destination port 80-90"},
		{"port list", firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 5060, Hi: 5061}, {Lo: 16384, Hi: 32767}}}, "destination port 5060-5061,16384-32767"},
		{"protocol", firewall.MatchProtocol{Protocol: "tcp"}, "protocol tcp"},
		{"connstate", firewall.MatchConnState{States: firewall.ConnStateEstablished | firewall.ConnStateRelated}, "connection state established,related"},
		{"mark", firewall.MatchMark{Value: 0x10, Mask: 0xFF}, "mark 0x10/0xff"},
		{"set ref src addr", firewall.MatchInSet{SetName: "blocked", MatchField: firewall.SetFieldSourceAddr}, "source address @blocked"},
		{"set ref dst addr", firewall.MatchInSet{SetName: "peers", MatchField: firewall.SetFieldDestAddr}, "destination address @peers"},
		{"set ref src port", firewall.MatchInSet{SetName: "voip", MatchField: firewall.SetFieldSourcePort}, "source port @voip"},
		{"set ref dst port", firewall.MatchInSet{SetName: "web", MatchField: firewall.SetFieldDestPort}, "destination port @web"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatMatch(tt.m)
			if got != tt.want {
				t.Errorf("formatMatch = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatActionTypes(t *testing.T) {
	tests := []struct {
		name string
		a    firewall.Action
		want string
	}{
		{"accept", firewall.Accept{}, "accept"},
		{"drop", firewall.Drop{}, "drop"},
		{"reject", firewall.Reject{Type: "icmp"}, "reject with icmp"},
		{"jump", firewall.Jump{Target: "helper"}, "jump helper"},
		{"counter", firewall.Counter{Name: "my-ctr"}, "counter my-ctr"},
		{"limit packets", firewall.Limit{Rate: 10, Unit: "second", Burst: 5, Dimension: firewall.RateDimensionPackets}, "limit rate 10/second burst 5"},
		{"limit bytes 1M", firewall.Limit{Rate: 1024 * 1024, Unit: "second", Burst: 5, Dimension: firewall.RateDimensionBytes}, "limit rate 1mbytes/second burst 5"},
		{"limit bytes 500K", firewall.Limit{Rate: 500 * 1024, Unit: "minute", Burst: 0, Dimension: firewall.RateDimensionBytes}, "limit rate 500kbytes/minute burst 0"},
		{"limit bytes bare", firewall.Limit{Rate: 12345, Unit: "second", Burst: 0, Dimension: firewall.RateDimensionBytes}, "limit rate 12345bytes/second burst 0"},
		{"mark set", firewall.SetMark{Value: 0x10}, "mark set 0x10"},
		{"connmark set", firewall.SetConnMark{Value: 0x20, Mask: 0xFF}, "connection-mark set 0x20"},
		{"dscp set", firewall.SetDSCP{Value: 46}, "dscp set 46"},
		{"tcp-mss set", firewall.SetTCPMSS{Size: 1400}, "tcp-mss set 1400"},
		{"redirect port", firewall.Redirect{Port: 8080}, "redirect to 8080"},
		{"redirect bare", firewall.Redirect{}, "redirect"},
		{"masquerade", firewall.Masquerade{}, "masquerade"},
		{"snat single", firewall.SNAT{Address: netip.MustParseAddr("10.0.0.1")}, "snat to 10.0.0.1"},
		{"snat single + port", firewall.SNAT{Address: netip.MustParseAddr("10.0.0.1"), Port: 80}, "snat to 10.0.0.1:80"},
		{"snat range", firewall.SNAT{Address: netip.MustParseAddr("10.0.0.1"), AddressEnd: netip.MustParseAddr("10.0.0.10")}, "snat to 10.0.0.1-10.0.0.10"},
		{"snat range + port", firewall.SNAT{Address: netip.MustParseAddr("10.0.0.1"), AddressEnd: netip.MustParseAddr("10.0.0.10"), Port: 80}, "snat to 10.0.0.1-10.0.0.10:80"},
		{"dnat range + port range", firewall.DNAT{Address: netip.MustParseAddr("10.0.0.1"), AddressEnd: netip.MustParseAddr("10.0.0.10"), Port: 1024, PortEnd: 2048}, "dnat to 10.0.0.1-10.0.0.10:1024-2048"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAction(tt.a)
			if got != tt.want {
				t.Errorf("formatAction = %q, want %q", got, tt.want)
			}
		})
	}
}

// VALIDATES: gap-6 -- formatSet renders per-element entries with their
// optional timeout. Elements with Timeout=0 render without the braces;
// elements with Timeout>0 render as `element <value> { timeout N; }`
// so `show firewall` round-trips the shape the operator wrote.
// PREVENTS: a non-zero timeout silently disappearing from `show` output.
func TestFormatSetElementsTimeout(t *testing.T) {
	set := &firewall.Set{
		Name: "blocked",
		Type: firewall.SetTypeIPv4,
		Elements: []firewall.SetElement{
			{Value: "10.0.0.1", Timeout: 3600},
			{Value: "10.0.0.2"},
		},
	}
	var b strings.Builder
	formatSet(&b, set)
	out := b.String()
	if !strings.Contains(out, "element 10.0.0.1 { timeout 3600; }") {
		t.Errorf("missing 10.0.0.1 with timeout in:\n%s", out)
	}
	if !strings.Contains(out, "element 10.0.0.2;") {
		t.Errorf("missing 10.0.0.2 bare in:\n%s", out)
	}
}

func TestFormatCountersEmpty(t *testing.T) {
	got := FormatCounters(nil)
	if got != "No counters." {
		t.Errorf("got %q, want empty message", got)
	}
}

func TestFormatCounters(t *testing.T) {
	counters := []firewall.ChainCounters{{
		Chain: "input",
		Terms: []firewall.TermCounter{
			{Name: "allow-ssh", Packets: 42, Bytes: 1234},
		},
	}}
	got := FormatCounters(counters)
	if !strings.Contains(got, "chain input") {
		t.Errorf("missing chain header:\n%s", got)
	}
	if !strings.Contains(got, "allow-ssh") {
		t.Errorf("missing term name:\n%s", got)
	}
	if !strings.Contains(got, "packets 42") {
		t.Errorf("missing packet count:\n%s", got)
	}
}

func TestStripPrefix(t *testing.T) {
	if got := StripPrefix("ze_wan"); got != "wan" {
		t.Errorf("StripPrefix = %q, want %q", got, "wan")
	}
	if got := StripPrefix("other"); got != "other" {
		t.Errorf("StripPrefix = %q, want %q", got, "other")
	}
}
