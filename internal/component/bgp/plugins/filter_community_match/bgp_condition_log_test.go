package filter_community_match

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// captureLogger points the package logger at a text handler of level and
// answers what it writes. It swaps a package variable, so a test using it
// MUST NOT run in parallel.
func captureLogger(t *testing.T, level slog.Level) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	captured := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: level}))
	old := logger
	logger = func() *slog.Logger { return captured }
	t.Cleanup(func() { logger = old })
	return &buf
}

// runLogged runs chain over update as a live run from 192.0.2.9 in
// direction, or as a display-only trace when trace is set, and answers the
// lines the logger wrote.
func runLogged(t *testing.T, chain *policychain.Chain, update *wireu.WireUpdate, session policyview.Session, direction string, trace bool) string {
	t.Helper()
	buf := captureLogger(t, slog.LevelDebug)
	var view policyview.View
	view.Reset(update, session)
	defer view.Clear()
	sets := new(policychain.RunSets)
	sets.SetRoute(policychain.ExternalRoute{Direction: direction, Peer: "192.0.2.9"})
	if trace {
		chain.Trace(&view, sets, func(policychain.MemberTrace) {})
	} else {
		chain.Run(&view, sets)
	}
	return buf.String()
}

// wantLine fails unless lines is exactly one line holding every want.
func wantLine(t *testing.T, lines string, want ...string) {
	t.Helper()
	if n := strings.Count(lines, "\n"); n != 1 {
		t.Fatalf("wrote %d lines, want 1: %q", n, lines)
	}
	for _, w := range want {
		if !strings.Contains(lines, w) {
			t.Fatalf("line %q lacks %q", lines, w)
		}
	}
}

// TestBGPMatchLogsCommunityDecision proves a match naming a community-match
// list writes the line the list's own filter wrote (handleFilterUpdate): one
// Info line naming the verdict, the list as filter= and the peer, and none
// in a display-only trace. Method: capture the logger and run a route
// carrying 65000:100 (accepted) and one carrying 65000:200 (rejected).
func TestBGPMatchLogsCommunityDecision(t *testing.T) {
	chain := communityChain(t)
	accepted := conditionUpdate(t, optionalTransitive(8, []byte{0xfd, 0xe8, 0, 100}))
	wantLine(t, runLogged(t, chain, accepted, policyview.Session{}, "import", false),
		"level=INFO", `msg="community-match accept"`, "filter=cl", "peer=192.0.2.9")
	wantLine(t, runLogged(t, chain, conditionUpdate(t, optionalTransitive(8, []byte{0xfd, 0xe8, 0, 200})), policyview.Session{}, "import", false),
		"level=INFO", `msg="community-match reject"`, "filter=cl", "peer=192.0.2.9")
	if lines := runLogged(t, chain, accepted, policyview.Session{}, "import", true); lines != "" {
		t.Fatalf("a trace wrote %q, want nothing", lines)
	}
}
