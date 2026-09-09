package completion

import (
	"strings"
	"testing"
)

// TestPeerCompletionsReadAnAsdotList proves peer completion still works when
// the daemon wrote its peer list under bgp/as-notation asdot. The entry an
// operator completes on carries the spelling they read. The method formats one
// list of each spelling.
//
// VALIDATES: peerEntry.RemoteAS is an asn.Number, so a quoted AS number
// decodes and the "as<N>" entry follows the notation.
// PREVENTS: completion answering nothing at all under asdot. A uint32 field
// failed the whole decode, and this function reports that as zero entries.
func TestPeerCompletionsReadAnAsdotList(t *testing.T) {
	// This process configures NO notation, and that is the contract under
	// test. asn.Number carries the spelling the daemon wrote, so completion
	// answers it, reads no configuration of the daemon, and cannot disagree
	// with it. An earlier version of this test called asn.Configure here, which
	// manufactured the precondition it claimed to state.

	const dotted = `{"peers":{"192.0.2.1":{"name":"transit-a","remote-as":"1.10","state":"established"}}}`

	var out strings.Builder
	if got := formatPeerCompletions(&out, dotted); got != 0 {
		t.Fatalf("formatPeerCompletions = %d, want 0 (success)", got)
	}
	text := out.String()
	if !strings.Contains(text, "as1.10") {
		t.Errorf("completions = %q, want an as1.10 entry", text)
	}
	if !strings.Contains(text, "transit-a") || !strings.Contains(text, "192.0.2.1") {
		t.Errorf("completions = %q, want the name and the address beside it", text)
	}
}

// TestPeerCompletionsReadAnAsplainList proves the asplain list every release
// before the leaf existed wrote still completes, decimal spelling and all.
//
// VALIDATES: the tolerant decode accepts a JSON number.
// PREVENTS: the fix for asdot breaking the form that already worked.
func TestPeerCompletionsReadAnAsplainList(t *testing.T) {
	const plain = `{"peers":{"192.0.2.1":{"name":"transit-a","remote-as":65546,"state":"established"}}}`

	var out strings.Builder
	if got := formatPeerCompletions(&out, plain); got != 0 {
		t.Fatalf("formatPeerCompletions = %d, want 0 (success)", got)
	}
	if text := out.String(); !strings.Contains(text, "as65546") {
		t.Errorf("completions = %q, want an as65546 entry", text)
	}
}
