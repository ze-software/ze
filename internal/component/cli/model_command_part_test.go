// Design: docs/architecture/api/commands.md -- the model reads a line's command part under the one grammar.

package cli

import "testing"

// TestModelReadsTheCommandPartUnderTheOneGrammar asks whether the model's
// readers that only need the command a line names (the ping and traceroute
// monitor detectors, and the subject an explanation is about) read it under
// the one grammar: a quoted pipe is part of a word and starts no chain, a bare
// one does.
//
// Method: each reader is asked about a line holding a quoted pipe and a line
// holding a real chain.
//
// MUTATION: put back `strings.ContainsRune(trimmed, '|')` in a detector, or
// `strings.Cut(input, "|")` in subjectOf; the quoted rows fail.
func TestModelReadsTheCommandPartUnderTheOneGrammar(t *testing.T) {
	const quotedPing = `monitor ping 192.0.2.1 "a|b"`
	if !isPingMonitorCommand(quotedPing) || isPipedPingMonitorCommand(quotedPing) {
		t.Errorf("%q was read as piped: a quoted pipe starts no chain", quotedPing)
	}
	if isPingMonitorCommand("monitor ping 192.0.2.1|log") || !isPipedPingMonitorCommand("monitor ping 192.0.2.1|log") {
		t.Error("a joined bare pipe was not read as the start of a chain")
	}

	const quotedTraceroute = `monitor traceroute 192.0.2.1 "a|b"`
	if !isTracerouteMonitorCommand(quotedTraceroute) || isPipedTracerouteMonitorCommand(quotedTraceroute) {
		t.Errorf("%q was read as piped: a quoted pipe starts no chain", quotedTraceroute)
	}
	if isTracerouteMonitorCommand("monitor traceroute 192.0.2.1 | log") ||
		!isPipedTracerouteMonitorCommand("monitor traceroute 192.0.2.1 | log") {
		t.Error("a bare pipe was not read as the start of a chain")
	}

	for input, want := range map[string]string{
		`show env get "a|b" | count`:  `show env get "a|b"`,
		"show  bgp   peer | match up": "show bgp peer",
		"show bgp peer":               "show bgp peer",
	} {
		if got := subjectOf(input); got != want {
			t.Errorf("subjectOf(%q) = %q, want %q", input, got, want)
		}
	}
}
