// VALIDATES: the bridge refuses an ExaBGP flow route whose component types no
// packet can satisfy together beyond ICMP and a port (ICMP beside TCP flags),
// and passes a route whose contradiction is in the protocol value to ze, which
// judges it again with the values.
// PREVENTS: the bridge sending ze an ICMP and TCP-flags route RFC 8955 Section
// 4.2 says SHOULD NOT be propagated, and the bridge refusing what it cannot
// judge without the values.

package bridge

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/nlri/flowspec"
)

// TestFlowRouteUnmatchableTypesRefused checks the translator refuses ICMP
// beside TCP flags, in IPv4 and IPv6, names the route, and translates a route
// whose protocol value contradicts its TCP flags into the update text command
// that ze's encoder refuses (TestFlowSpecUnmatchableByValueRefused in the
// flowspec NLRI plugin covers that text). Method: Translator.Line over each.
func TestFlowRouteUnmatchableTypesRefused(t *testing.T) {
	for _, tc := range []struct{ line, route string }{
		{
			line:  "neighbor 10.0.0.1 announce flow route { match { destination 10.0.0.1/32; icmp-type =8; tcp-flags syn; } then { discard; } }",
			route: "icmp-type =8; tcp-flags syn;",
		},
		{
			line:  "neighbor 10.0.0.1 announce flow route { match { destination 2001:db8::1/128; icmp-code =0; tcp-flags syn; } then { discard; } }",
			route: "icmp-code =0; tcp-flags syn;",
		},
	} {
		translation, err := Translator{}.Line(tc.line)
		if !errors.Is(err, flowspec.ErrFlowSpecUnmatchable) {
			t.Fatalf("Line(%q) = %+v, %v; want ErrFlowSpecUnmatchable", tc.line, translation.Commands, err)
		}
		if !strings.Contains(err.Error(), tc.route) {
			t.Fatalf("refusal %q does not name the route %q", err, tc.route)
		}
	}

	line := "neighbor 10.0.0.1 announce flow route { match { destination 10.0.0.1/32; protocol udp; tcp-flags syn; } then { discard; } }"
	translation, err := Translator{}.Line(line)
	if err != nil {
		t.Fatalf("Line(%q) refused by the bridge: %v; the protocol value is ze's to judge", line, err)
	}
	if len(translation.Commands) != 1 {
		t.Fatalf("Line(%q) = %+v, want one command", line, translation.Commands)
	}
	if got := translation.Commands[0].Text; !strings.Contains(got, "protocol") || !strings.Contains(got, "tcp-flags") {
		t.Fatalf("command %q does not carry the protocol and TCP flags ze refuses", got)
	}
}
