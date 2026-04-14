package cmd

import (
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
)

func TestFormatQoSMapEmpty(t *testing.T) {
	got := FormatQoSMap(nil)
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
