package cmd

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/probe"
)

// TestSameQuestionPairsOneProducer proves AC-14 for show ping: the daemon RPC
// (handleShowPing) and the no-daemon route (pingAnswer, which showPingLocal
// prints) answer through one producer.
//
// Method: a refused request must carry the same reason on both routes (the
// local line is "show ping: <reason>"); an answered request over the fake ICMP
// connection must carry the same keys and the same status on both routes. The
// RTT figures differ between two runs, so values that are measured are not
// compared.
func TestSameQuestionPairsOneProducer(t *testing.T) {
	for _, words := range [][]string{
		{"192.0.2.1", probe.DFKeyword},
		{"192.0.2.1", "count", "0"},
		{"not-an-address"},
	} {
		resp, err := handleShowPing(nil, commandtest.Args(words...))
		if err != nil {
			t.Fatalf("handleShowPing %q: %v", words, err)
		}
		local := pingAnswer(commandtest.Args(words...))
		if resp.Status != plugin.StatusError || local.Code != 1 {
			t.Fatalf("%q: daemon status %q, local code %d, want both refused", words, resp.Status, local.Code)
		}
		if want := "show ping: " + resp.Error + "\n"; local.Diagnostic != want {
			t.Errorf("%q: local refusal %q, want the daemon reason %q", words, local.Diagnostic, want)
		}
	}

	// answered installs a fake ICMP connection that answers the one probe.
	answered := func() {
		fc := newFakePingConn(clock.RealClock{})
		recordingOpener(t, fc)
		go func() {
			w := <-fc.wrote
			fc.injectReply(testPID(), w.seq)
		}()
	}
	answered()
	resp, err := handleShowPing(nil, commandtest.Args("192.0.2.1", "count", "1", "timeout", "1s"))
	if err != nil || resp.Status != plugin.StatusDone {
		t.Fatalf("daemon route: %v %q %s", err, resp.Status, resp.Error)
	}
	daemon, _ := resp.Data.(plugin.Map)
	answered()
	local := pingAnswer(commandtest.Args("192.0.2.1", "count", "1", "timeout", "1s"))
	if local.Code != 0 {
		t.Fatalf("local route refused: %s", local.Diagnostic)
	}
	localMap, _ := local.Payload.(plugin.Map)
	if !slices.Equal(sortedKeys(daemon), sortedKeys(localMap)) {
		t.Errorf("keys differ: daemon %v, local %v", sortedKeys(daemon), sortedKeys(localMap))
	}
	if daemon["received"] != localMap["received"] {
		t.Errorf("received differs: daemon %v, local %v", daemon["received"], localMap["received"])
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
