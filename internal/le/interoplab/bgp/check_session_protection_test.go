package bgp

import (
	"strings"
	"testing"
)

// TestSessionProtectionProbePredicates drives both polarities of the probe
// reading checkSessionProtection makes, with no container: each outcome the
// probe script prints is accepted only where it is the wanted one, and any
// other line, the script's own error included, is refused rather than read as
// a drop.
func TestSessionProtectionProbePredicates(t *testing.T) {
	cases := []struct {
		output string
		want   sessionProbeOutcome
	}{
		{"connected\n", sessionProbeConnected},
		{"refused\n", sessionProbeRefused},
		{"timeout\n", sessionProbeTimedOut},
	}
	for _, tc := range cases {
		got, err := parseSessionProbe(tc.output)
		if err != nil {
			t.Fatalf("parseSessionProbe(%q): %v", tc.output, err)
		}
		if got != tc.want {
			t.Fatalf("parseSessionProbe(%q) = %s, want %s", tc.output, got, tc.want)
		}
		for _, other := range []sessionProbeOutcome{sessionProbeConnected, sessionProbeRefused, sessionProbeTimedOut} {
			err := requireSessionProbe(tc.output, "probe", other)
			if other == tc.want {
				if err != nil {
					t.Fatalf("%q refused when %s is wanted: %v", tc.output, other, err)
				}
				continue
			}
			if err == nil {
				t.Fatalf("%q accepted when %s is wanted", tc.output, other)
			}
		}
	}

	// A reset is an answer, so it must never pass for a drop.
	if requireSessionProbe("refused\n", "probe", sessionProbeTimedOut) == nil {
		t.Fatal("a reset passed as a dropped SYN")
	}
	for _, output := range []string{"", "error [Errno 99] Cannot assign requested address\n", "timeout\ntimeout\n", "Timeout\n"} {
		if _, err := parseSessionProbe(output); err == nil {
			t.Fatalf("parseSessionProbe(%q) accepted a line the script does not print for an outcome", output)
		}
		if requireSessionProbe(output, "probe", sessionProbeTimedOut) == nil {
			t.Fatalf("%q passed as a dropped SYN", output)
		}
	}
}

// TestSessionProtectionCoppCounter drives the reading of ze_copp's
// unknown-source-drop counter: the counted drop of the input chain is read,
// a counted drop in another chain is not, and a table without the input chain
// or without its counted drop (a CoPP that lost its final term) is refused.
// The rise check refuses an unchanged and a lower reading.
func TestSessionProtectionCoppCounter(t *testing.T) {
	const table = `table inet ze_copp {
	set bgp-neighbor-ipv4 {
		type ipv4_addr
		flags interval
		elements = { 172.30.0.3 }
	}

	chain input {
		type filter hook input priority filter; policy accept;
		ct state established,related accept
		tcp dport 179 ip saddr @bgp-neighbor-ipv4 jump bgp-neighbor
		tcp dport 179 counter packets 3 bytes 180 drop
	}

	chain bgp-neighbor {
		accept
	}
}
`
	got, err := coppUnknownSourceDrops(table)
	if err != nil {
		t.Fatalf("the rendered table was refused: %v", err)
	}
	if got != 3 {
		t.Fatalf("unknown-source-drop = %d, want 3", got)
	}

	noDrop := strings.Replace(table, "\t\ttcp dport 179 counter packets 3 bytes 180 drop\n", "", 1)
	if _, err := coppUnknownSourceDrops(noDrop); err == nil {
		t.Fatal("an input chain without its counted drop was accepted")
	}
	elsewhere := strings.Replace(noDrop, "\t\taccept\n", "\t\tcounter packets 9 bytes 540 drop\n", 1)
	if _, err := coppUnknownSourceDrops(elsewhere); err == nil {
		t.Fatal("a counted drop outside the input chain was read as unknown-source-drop")
	}
	noInput := strings.Replace(table, "chain input {", "chain forward {", 1)
	if _, err := coppUnknownSourceDrops(noInput); err == nil {
		t.Fatal("a table with no input chain was accepted")
	}

	if err := requireCoppDropsRose(3, 4); err != nil {
		t.Fatalf("a rising counter was refused: %v", err)
	}
	if requireCoppDropsRose(3, 3) == nil {
		t.Fatal("an unchanged counter passed")
	}
	if requireCoppDropsRose(3, 0) == nil {
		t.Fatal("a lower counter passed")
	}
}
