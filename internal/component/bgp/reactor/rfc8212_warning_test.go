// Design: docs/guide/bgp-policy.md -- Policy model, the RFC 8212 default for an absent direction
// Related: rfc8212_warning.go -- warnRFC8212AbsentPolicy, the producer under test
// RFC naming: untagged -- tests the establishment warning for an absent RFC 8212 direction, an operator aid the RFC does not oblige, not the Section 3 verdict itself.
//
// Goal: the Established warning names the peer, the direction, RFC 8212 and
// the exact editor command, once per absent direction per session, and never
// for IBGP or a configured direction. Method: build a Peer from settings, swap
// the peer logger for a Warn-level text sink, call the producer the
// Established transition calls, and read the lines it wrote.
package reactor

import (
	"strings"
	"testing"

	routepolicy "github.com/ze-software/ze/internal/component/policy"
)

// rfc8212WarningPeer builds a peer at 192.0.2.1 with local AS 65000 and the
// given remote AS, named name.
func rfc8212WarningPeer(peerAS uint32, name string) *Peer {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, peerAS, 0x01020304)
	settings.Name = name
	return NewPeer(settings)
}

// rfc8212RequireLine fails unless exactly one line holds every fragment.
func rfc8212RequireLine(t *testing.T, lines []string, fragments ...string) {
	t.Helper()
	found := 0
	for _, line := range lines {
		holds := true
		for _, fragment := range fragments {
			if !strings.Contains(line, fragment) {
				holds = false
				break
			}
		}
		if holds {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("want exactly one line holding %q, found %d in %q", fragments, found, lines)
	}
}

// An EBGP peer with neither direction configured logs two warnings: import
// and export, each with its own repair command and the commit that applies it.
func TestRFC8212WarningEBGPBothAbsent(t *testing.T) {
	sink := rfc5882WarnSink(t)
	rfc8212WarningPeer(65001, "transit-a").warnRFC8212AbsentPolicy()

	lines := rfc5882WarnLines(sink)
	if len(lines) != 2 {
		t.Fatalf("logged %d warnings, want 2: %q", len(lines), lines)
	}
	rfc8212RequireLine(t, lines, "RFC 8212", "no import policy", "peer=transit-a", "address=192.0.2.1",
		"direction=import", `fix="set bgp peer transit-a filter import final accept"`, `then="commit now"`)
	rfc8212RequireLine(t, lines, "RFC 8212", "no export policy", "peer=transit-a",
		"direction=export", `fix="set bgp peer transit-a filter export final accept"`, `then="commit now"`)
}

// An IBGP peer keeps its accepting default, so nothing is closed and nothing
// is logged.
func TestRFC8212WarningIBGPSilent(t *testing.T) {
	sink := rfc5882WarnSink(t)
	rfc8212WarningPeer(65000, "core").warnRFC8212AbsentPolicy()

	if lines := rfc5882WarnLines(sink); len(lines) != 0 {
		t.Fatalf("IBGP logged %d warnings, want 0: %q", len(lines), lines)
	}
}

// A configured direction never warns, whatever its final: a configured reject
// is the operator's decision, not RFC 8212's. The absent direction still does.
func TestRFC8212WarningConfiguredDirectionSilent(t *testing.T) {
	sink := rfc5882WarnSink(t)
	peer := rfc8212WarningPeer(65001, "transit-a")
	peer.settings.ImportPolicy = mustPolicy(t, nil, routepolicy.FinalReject)
	peer.warnRFC8212AbsentPolicy()

	lines := rfc5882WarnLines(sink)
	if len(lines) != 1 {
		t.Fatalf("logged %d warnings, want 1 (export only): %q", len(lines), lines)
	}
	rfc8212RequireLine(t, lines, "direction=export", "set bgp peer transit-a filter export final accept")

	sink.Reset()
	peer.settings.ExportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
	peer.warnRFC8212AbsentPolicy()
	if lines := rfc5882WarnLines(sink); len(lines) != 0 {
		t.Fatalf("both directions configured, logged %d warnings, want 0: %q", len(lines), lines)
	}
}

// A static member of a group is repaired at its own path under the group.
func TestRFC8212WarningGroupMemberPath(t *testing.T) {
	sink := rfc5882WarnSink(t)
	peer := rfc8212WarningPeer(65001, "transit-a")
	peer.settings.GroupName = "upstreams"
	peer.settings.ImportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
	peer.warnRFC8212AbsentPolicy()

	rfc8212RequireLine(t, rfc5882WarnLines(sink), "direction=export",
		`fix="set bgp group upstreams peer transit-a filter export final accept"`)
}

// A dynamic peer exists only through its group, so the command names the
// group and never the generated dyn- name. A reconnection warns again, and it
// is judged on the AS the new OPEN carried: here the second session comes
// from the local AS, so it is IBGP and silent.
func TestRFC8212WarningDynamicNamesGroupAndReconnect(t *testing.T) {
	sink := rfc5882WarnSink(t)
	peer := rfc8212WarningPeer(65001, "dyn-192.0.2.1")
	peer.settings.IsDynamic = true
	peer.settings.GroupName = "ixp"

	peer.warnRFC8212AbsentPolicy()
	lines := rfc5882WarnLines(sink)
	if len(lines) != 2 {
		t.Fatalf("first session logged %d warnings, want 2: %q", len(lines), lines)
	}
	rfc8212RequireLine(t, lines, "peer=dyn-192.0.2.1", "direction=import",
		`fix="set bgp group ixp filter import final accept"`)
	rfc8212RequireLine(t, lines, "direction=export", `fix="set bgp group ixp filter export final accept"`)

	// The next EBGP session warns again: the producer keeps no once-ever state.
	sink.Reset()
	peer.warnRFC8212AbsentPolicy()
	if lines := rfc5882WarnLines(sink); len(lines) != 2 {
		t.Fatalf("reconnected EBGP session logged %d warnings, want 2: %q", len(lines), lines)
	}

	// resolveDynamicPeerSettings publishes the new OPEN's AS under p.mu.
	sink.Reset()
	peer.mu.Lock()
	peer.settings.PeerAS = 65000
	peer.mu.Unlock()
	peer.warnRFC8212AbsentPolicy()
	if lines := rfc5882WarnLines(sink); len(lines) != 0 {
		t.Fatalf("reconnected as IBGP, logged %d warnings, want 0: %q", len(lines), lines)
	}
}
