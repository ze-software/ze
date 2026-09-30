// Design: docs/guide/bfd.md -- Enabling BFD on a BGP peer
// Related: peer_bfd.go -- verifyPeerBFDProfiles, the commit check that warns
// Related: rfc5882_bfd_auth_test.go -- the authenticated profile reaching the session
//
// RFC 5882 Section 10.2: "BFD authentication SHOULD be used and is strongly
// encouraged." for a BFD session advising an EBGP session. The SHOULD permits
// an unauthenticated profile, so the commit check accepts it and warns. Goal
// and method: verifyPeerBFDProfiles runs over the operator's bgp tree against
// a candidate bfd section, through the real bfd profile check, with the peer
// logger swapped for a Warn-level text sink whose lines are counted.
package reactor

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// rfc5882PlainSection is a bfd section whose "plain" profile has no auth block.
const rfc5882PlainSection = `{"bfd":{"profile":{"plain":{"detect-multiplier":"3"}}}}`

// rfc5882WarnSink swaps the peer logger for a Warn-level text handler writing
// to the returned buffer, restored when the test ends. Not safe for parallel
// tests: peerLogger is package state.
func rfc5882WarnSink(t *testing.T) *bytes.Buffer {
	t.Helper()
	var sink bytes.Buffer
	lg := slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelWarn}))
	prev := peerLogger
	peerLogger = func() *slog.Logger { return lg }
	t.Cleanup(func() { peerLogger = prev })
	return &sink
}

// rfc5882WarnLines returns the non-empty lines the sink holds.
func rfc5882WarnLines(sink *bytes.Buffer) []string {
	var lines []string
	for line := range strings.SplitSeq(sink.String(), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// rfc5882IBGPTree is bfdStrictTree with the peer's remote AS equal to the
// local AS 65000, so the session is IBGP.
func rfc5882IBGPTree(t *testing.T, bfd map[string]any) map[string]any {
	t.Helper()
	tree := bfdStrictTree(bfd)
	peers, ok := tree["peer"].(map[string]any)
	if !ok {
		t.Fatal("bfdStrictTree has no peer map")
	}
	peer, ok := peers["peer1"].(map[string]any)
	if !ok {
		t.Fatal("bfdStrictTree has no peer1")
	}
	peer["session"] = map[string]any{"asn": map[string]any{"remote": "65000"}}
	return tree
}

// RFC requirement: RFC5882-10.2-1 negative -- "BFD authentication SHOULD be
// used and is strongly encouraged.", and an EBGP peer (local AS 65000, remote
// AS 65001) names the bfd profile "plain", which has no auth block. The commit
// check accepts it, as the SHOULD permits, and logs exactly one Warn, which
// names the peer address 192.0.2.1 and the profile "plain".
func TestRFC5882EBGPUnauthenticatedProfileWarns(t *testing.T) {
	sink := rfc5882WarnSink(t)
	tree := bfdStrictTree(map[string]any{"enabled": "true", "profile": "plain"})

	if err := verifyPeerBFDProfiles(tree, rfc5882PlainSection); err != nil {
		t.Fatalf("commit refused an unauthenticated profile the SHOULD permits: %v", err)
	}

	lines := rfc5882WarnLines(sink)
	if len(lines) != 1 {
		t.Fatalf("commit logged %d Warn lines, want 1: %q", len(lines), lines)
	}
	for _, want := range []string{"level=WARN", "peer=192.0.2.1", "profile=plain", "RFC 5882 Section 10.2"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("Warn line %q does not carry %q", lines[0], want)
		}
	}
}

// RFC requirement: RFC5882-10.2-1 positive -- the warning is scoped to what
// Section 10.2 recommends: an EBGP peer naming the profile "secure", whose
// auth block configures Keyed SHA1, passes the commit check with no Warn, and
// an IBGP peer (remote AS 65000) naming the unauthenticated profile "plain"
// passes it with no Warn either.
func TestRFC5882AuthenticatedOrIBGPProfileDoesNotWarn(t *testing.T) {
	cases := []struct {
		name    string
		tree    map[string]any
		section string
	}{
		{"EBGP peer, authenticated profile", bfdStrictTree(map[string]any{"enabled": "true", "profile": "secure"}), rfc5882AuthSection},
		{"IBGP peer, unauthenticated profile", rfc5882IBGPTree(t, map[string]any{"enabled": "true", "profile": "plain"}), rfc5882PlainSection},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sink := rfc5882WarnSink(t)
			if err := verifyPeerBFDProfiles(tc.tree, tc.section); err != nil {
				t.Fatalf("commit refused the profile: %v", err)
			}
			if lines := rfc5882WarnLines(sink); len(lines) != 0 {
				t.Errorf("commit logged %d Warn lines, want none: %q", len(lines), lines)
			}
		})
	}
}

// RFC requirement: RFC5882-10.2-1 negative -- "BFD authentication SHOULD be
// used and is strongly encouraged.", and an EBGP peer (local AS 65000, remote
// AS 65001) enables BFD and names no profile, so its session carries no auth
// block. The commit check accepts it, as the SHOULD permits, and logs exactly
// one Warn, which names the peer address 192.0.2.1 and no profile.
func TestRFC5882EBGPPeerWithoutProfileWarns(t *testing.T) {
	sink := rfc5882WarnSink(t)
	tree := bfdStrictTree(map[string]any{"enabled": "true"})

	if err := verifyPeerBFDProfiles(tree, rfc5882PlainSection); err != nil {
		t.Fatalf("commit refused a peer with no bfd profile: %v", err)
	}

	lines := rfc5882WarnLines(sink)
	if len(lines) != 1 {
		t.Fatalf("commit logged %d Warn lines, want 1: %q", len(lines), lines)
	}
	for _, want := range []string{"level=WARN", "peer=192.0.2.1", "names no profile", "RFC 5882 Section 10.2"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("Warn line %q does not carry %q", lines[0], want)
		}
	}
	if strings.Contains(lines[0], "profile=") {
		t.Errorf("Warn line %q names a profile the peer did not name", lines[0])
	}
}

// RFC requirement: RFC5882-10.2-1 positive -- the no-profile warning is scoped
// to what Section 10.2 recommends: an IBGP peer (remote AS 65000) that enables
// BFD and names no profile passes the commit check with no Warn.
func TestRFC5882IBGPPeerWithoutProfileDoesNotWarn(t *testing.T) {
	sink := rfc5882WarnSink(t)
	tree := rfc5882IBGPTree(t, map[string]any{"enabled": "true"})

	if err := verifyPeerBFDProfiles(tree, rfc5882PlainSection); err != nil {
		t.Fatalf("commit refused a peer with no bfd profile: %v", err)
	}
	if lines := rfc5882WarnLines(sink); len(lines) != 0 {
		t.Errorf("commit logged %d Warn lines, want none: %q", len(lines), lines)
	}
}
