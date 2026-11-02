package cmd

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/probe"
)

// TestSameQuestionPairsOneProducer proves AC-14 for show traceroute: the
// daemon RPC (handleTraceroute) and the no-daemon route (tracerouteAnswer,
// which showTracerouteLocal prints) answer through one producer.
//
// Method: each refused request must carry the same reason on both routes,
// the local line being "show traceroute: <reason>". A traced path is not
// compared: it needs raw sockets and live hops.
func TestSameQuestionPairsOneProducer(t *testing.T) {
	for _, words := range [][]string{
		{"192.0.2.1", probe.DFKeyword},
		{"192.0.2.1", "max-hops", "0"},
		{"not-an-address"},
	} {
		resp, err := handleTraceroute(nil, commandtest.Args(words...))
		if err != nil {
			t.Fatalf("handleTraceroute %q: %v", words, err)
		}
		local := tracerouteAnswer(commandtest.Args(words...))
		if resp.Status != plugin.StatusError || local.Code != 1 {
			t.Fatalf("%q: daemon status %q, local code %d, want both refused", words, resp.Status, local.Code)
		}
		if want := "show traceroute: " + resp.Error + "\n"; local.Diagnostic != want {
			t.Errorf("%q: local refusal %q, want the daemon reason %q", words, local.Diagnostic, want)
		}
	}
}
