package cmd

import (
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestAppendTracerouteRoundRendersEachHop proves the round block every
// terminal route of `monitor traceroute` writes: the round header, then one
// line per hop with its RTT when the hop answered. Method: render two hops,
// one answered and one silent, and compare the text.
func TestAppendTracerouteRoundRendersEachHop(t *testing.T) {
	hops := []map[string]any{
		{fieldTTL: 1, "addr": "192.0.2.1", "rtt-ms": 0.5},
		{fieldTTL: 2, "addr": "*"},
	}
	var tb textbuf.Buffer
	got := string(appendTracerouteRound(&tb, 3, hops))
	want := "--- round 3 ---\n  1  192.0.2.1  0.500 ms\n  2  *\n"
	if got != want {
		t.Errorf("round text:\n%q\nwant\n%q", got, want)
	}
}
