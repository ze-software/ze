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
				Matches: []firewall.Match{firewall.MatchDestinationPort{Port: 22}},
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
		{"dest port", firewall.MatchDestinationPort{Port: 443}, "destination port 443"},
		{"port range", firewall.MatchDestinationPort{Port: 80, PortEnd: 90}, "destination port 80-90"},
		{"protocol", firewall.MatchProtocol{Protocol: "tcp"}, "protocol tcp"},
		{"connstate", firewall.MatchConnState{States: firewall.ConnStateEstablished | firewall.ConnStateRelated}, "connection state established,related"},
		{"mark", firewall.MatchMark{Value: 0x10, Mask: 0xFF}, "mark 0x10/0xff"},
		{"set ref", firewall.MatchInSet{SetName: "blocked"}, "@blocked"},
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
		{"limit", firewall.Limit{Rate: 10, Unit: "second", Burst: 5}, "limit rate 10/second burst 5"},
		{"mark set", firewall.SetMark{Value: 0x10}, "mark set 0x10"},
		{"masquerade", firewall.Masquerade{}, "masquerade"},
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
