package cmd

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/traffic"
)

func TestFormatQoSMapEmpty(t *testing.T) {
	got := formatQoSMap(nil)
	if got != "No traffic control configured." {
		t.Errorf("got %q, want empty message", got)
	}
}

func TestFormatQoS(t *testing.T) {
	qos := traffic.InterfaceQoS{
		Interface: "eth0",
		Qdisc: traffic.Qdisc{
			Type:         traffic.QdiscHTB,
			DefaultClass: "bulk",
			Classes: []traffic.TrafficClass{
				{
					Name:     "voip",
					Rate:     10_000_000,
					Ceil:     100_000_000,
					Priority: 0,
					Filters: []traffic.TrafficFilter{
						{Type: traffic.FilterMark, Value: 0x10},
					},
				},
				{
					Name:     "bulk",
					Rate:     85_000_000,
					Ceil:     100_000_000,
					Priority: 2,
				},
			},
		},
	}
	got := FormatQoS(qos)
	if !strings.Contains(got, "interface eth0") {
		t.Errorf("missing interface header:\n%s", got)
	}
	if !strings.Contains(got, "qdisc htb default bulk") {
		t.Errorf("missing qdisc line:\n%s", got)
	}
	if !strings.Contains(got, "class voip") {
		t.Errorf("missing class voip:\n%s", got)
	}
	if !strings.Contains(got, "rate 10mbit") {
		t.Errorf("missing rate:\n%s", got)
	}
	if !strings.Contains(got, "match mark 0x10") {
		t.Errorf("missing filter:\n%s", got)
	}
}

func TestFormatRate(t *testing.T) {
	tests := []struct {
		bps  uint64
		want string
	}{
		{1_000_000_000, "1gbit"},
		{100_000_000, "100mbit"},
		{10_000_000, "10mbit"},
		{1_000_000, "1mbit"},
		{100_000, "100kbit"},
		{500, "500bit"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatRate(tt.bps)
			if got != tt.want {
				t.Errorf("formatRate(%d) = %q, want %q", tt.bps, got, tt.want)
			}
		})
	}
}

func TestFormatQoSRootTBFRate(t *testing.T) {
	qos := traffic.InterfaceQoS{
		Interface: "ppp0",
		Qdisc:     traffic.Qdisc{Type: traffic.QdiscTBF, RateBps: 10_000_000},
	}
	want := "interface ppp0 {\n  qdisc tbf {\n    rate 10mbit;\n  }\n}\n"
	if got := FormatQoS(qos); got != want {
		t.Fatalf("root TBF output = %q, want %q", got, want)
	}
}

// TestFormatQoSWithStats verifies FormatQoS appends readable stats lines when
// Qdisc.Stats or TrafficClass.Stats is non-nil, preserves raw kernel qdisc
// kinds, and keeps output unchanged when Stats is nil.
//
// VALIDATES: AC-8 (nil-stats byte-for-byte preservation and non-nil stats
// rendering on root and classes).
// PREVENTS: broken offline CLI formatting or silent omission of live stats.
func TestFormatQoSWithStats(t *testing.T) {
	nilQoS := traffic.InterfaceQoS{
		Interface: "eth0",
		Qdisc:     traffic.Qdisc{Type: traffic.QdiscHTB},
	}
	wantNil := "interface eth0 {\n  qdisc htb {\n  }\n}\n"
	if got := FormatQoS(nilQoS); got != wantNil {
		t.Fatalf("nil-stats output = %q, want %q", got, wantNil)
	}

	withStats := traffic.InterfaceQoS{
		Interface: "eth0",
		Qdisc: traffic.Qdisc{
			Type:         traffic.QdiscHTB,
			Kind:         "htb",
			DefaultClass: "default",
			Stats: &traffic.QdiscStats{
				Bytes:      4096,
				Packets:    32,
				Qlen:       2,
				Backlog:    256,
				Drops:      1,
				Overlimits: 3,
				Requeues:   4,
			},
			Classes: []traffic.TrafficClass{
				{
					Name:     "default",
					Rate:     10_000_000,
					Ceil:     100_000_000,
					Priority: 1,
					Stats: &traffic.QdiscStats{
						Bytes:      2048,
						Packets:    16,
						Qlen:       1,
						Backlog:    128,
						Drops:      1,
						Overlimits: 2,
						Requeues:   0,
					},
				},
			},
		},
	}
	got := FormatQoS(withStats)
	wantRootStats := "    stats bytes 4096 packets 32 qlen 2 backlog 256 drops 1 overlimits 3 requeues 4;\n"
	wantClassStats := "      stats bytes 2048 packets 16 qlen 1 backlog 128 drops 1 overlimits 2 requeues 0;\n"
	if !strings.Contains(got, wantRootStats) {
		t.Fatalf("missing root stats line:\ngot:\n%s\nwant substring:\n%s", got, wantRootStats)
	}
	if !strings.Contains(got, wantClassStats) {
		t.Fatalf("missing class stats line:\ngot:\n%s\nwant substring:\n%s", got, wantClassStats)
	}

	unshaped := traffic.InterfaceQoS{
		Interface: "lo",
		Qdisc: traffic.Qdisc{
			Kind:  "noqueue",
			Stats: &traffic.QdiscStats{Bytes: 64, Packets: 1},
		},
	}
	wantUnshaped := "interface lo {\n  qdisc noqueue {\n    stats bytes 64 packets 1 qlen 0 backlog 0 drops 0 overlimits 0 requeues 0;\n  }\n}\n"
	if got := FormatQoS(unshaped); got != wantUnshaped {
		t.Fatalf("unshaped output = %q, want %q", got, wantUnshaped)
	}
}
