// VALIDATES: an ExaBGP API flow route no packet can match (an ICMP type or code
// with a port) is refused by the translator, in both the braced and the flat
// family form, answered `error` as ExaBGP 6.0 answers it, logged with the route,
// and never handed to ze.
// PREVENTS: the bridge sending ze a route RFC 8955 Section 4.2 says SHOULD NOT
// be propagated, and the script reading a refusal that names no route.

package bridge

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/nlri/flowspec"
)

// unmatchableFlowLines are API lines whose route matches an ICMP type or code
// and a port. ExaBGP 6.0 (pin 685f772e4e) parses each through
// configuration/grammar/tree/flow.py FlowLine.parse, whose propagated() drops
// the route with flow.unmatchable, so api_flow returns no route and
// announce_flow and withdraw_flow answer answer_error.
var unmatchableFlowLines = []struct {
	name  string
	line  string
	route string
}{
	{
		name:  "braced announce, icmp-type with destination-port",
		line:  "neighbor 10.0.0.1 announce flow route { match { destination 10.0.0.1/32; icmp-type =8; destination-port =80; } then { discard; } }",
		route: "icmp-type =8; destination-port =80;",
	},
	{
		name:  "braced withdraw, icmp-code with port",
		line:  "neighbor 10.0.0.1 withdraw flow route { match { source 10.0.0.1/32; icmp-code =0; port =443; } }",
		route: "icmp-code =0; port =443;",
	},
	{
		name:  "flat family announce, icmp-type with source-port",
		line:  "neighbor 10.0.0.1 announce ipv4 flow source-ipv4 10.0.1.0/24 icmp-type =8 source-port =53 extended-community [rate-limit:0]",
		route: "icmp-type =8 source-port =53",
	},
}

// TestFlowRouteUnmatchableRefused checks the translator refuses each
// unmatchable route with flowspec.ErrFlowSpecUnmatchable and names the route,
// and still translates the two halves of the combination on their own.
// Method: Translator.Line over each line, then over a matchable control.
func TestFlowRouteUnmatchableRefused(t *testing.T) {
	for _, tc := range unmatchableFlowLines {
		t.Run(tc.name, func(t *testing.T) {
			translation, err := Translator{}.Line(tc.line)
			if !errors.Is(err, flowspec.ErrFlowSpecUnmatchable) {
				t.Fatalf("Line(%q) = %+v, %v; want ErrFlowSpecUnmatchable", tc.line, translation.Commands, err)
			}
			if !strings.Contains(err.Error(), tc.route) {
				t.Fatalf("refusal %q does not name the route %q", err, tc.route)
			}
		})
	}
	for _, line := range []string{
		"neighbor 10.0.0.1 announce flow route { match { destination 10.0.0.1/32; protocol icmp; icmp-type =8; } then { discard; } }",
		"neighbor 10.0.0.1 announce flow route { match { destination 10.0.0.1/32; protocol tcp; destination-port =80; } then { discard; } }",
		"neighbor 10.0.0.1 announce ipv4 flow source-ipv4 10.0.1.0/24 icmp-code =0 extended-community [rate-limit:0]",
	} {
		if _, err := (Translator{}).Line(line); err != nil {
			t.Fatalf("matchable route refused: Line(%q): %v", line, err)
		}
	}
}

// TestFlowRouteUnmatchableAnswersError checks what the script reads back and
// what the operator's log says. ExaBGP 6.0 answers the line `error`, and Ze
// writes its reason line before the marker, as for every line the translator
// refuses (AnswerBatch). Method: feed each line to ScriptLines.Batch over a
// recording runner and a captured log.
func TestFlowRouteUnmatchableAnswersError(t *testing.T) {
	for _, tc := range unmatchableFlowLines {
		t.Run(tc.name, func(t *testing.T) {
			g := newGroupScript()
			var logged bytes.Buffer
			g.lines.Log = slog.New(slog.NewTextHandler(&logged, nil))

			got := g.send(t, tc.line)
			if !strings.HasPrefix(got, "error: ") {
				t.Fatalf("script read %q, want the reason line first", got)
			}
			if !strings.HasSuffix(got, "\nerror\n") {
				t.Fatalf("script read %q, want the error marker last", got)
			}
			if strings.Contains(got, "done") {
				t.Fatalf("script read %q, ExaBGP answers error, never done", got)
			}
			if len(g.runner.dispatched) != 0 {
				t.Fatalf("an unmatchable route reached ze: %q", g.runner.dispatched)
			}
			if !strings.Contains(logged.String(), "exabgp-bridge line refused") {
				t.Fatalf("no refusal logged: %q", logged.String())
			}
			if !strings.Contains(logged.String(), tc.route) {
				t.Fatalf("log %q does not name the route %q", logged.String(), tc.route)
			}
		})
	}
}
