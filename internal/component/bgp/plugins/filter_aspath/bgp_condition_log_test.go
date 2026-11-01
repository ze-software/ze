package filter_aspath

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"

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

func pathOf(asns ...uint32) []byte {
	return asPathAttr(&attribute.ASPath{Segments: []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: asns}}})
}

// TestBGPMatchLogsASPathListDecision proves a match naming an as-path-list
// writes the line the list's own filter wrote (handleFilterUpdate): one Info
// line naming the verdict, the list as filter=, the peer and the AS path,
// and none in a display-only trace. Method: capture the logger and run an
// accepted and a rejected path, then the accepted one as a trace.
func TestBGPMatchLogsASPathListDecision(t *testing.T) {
	chain := matchChain(t, "transit")
	accepted := bgpUpdate(t, pathOf(65001, 65002))
	wantLine(t, runLogged(t, chain, accepted, policyview.Session{}, "import", false),
		"level=INFO", `msg="as-path-list accept"`, "filter=transit", "peer=192.0.2.9", `as-path="65001 65002"`)
	wantLine(t, runLogged(t, chain, bgpUpdate(t, pathOf(65001, 64512)), policyview.Session{}, "import", false),
		"level=INFO", `msg="as-path-list reject"`, "filter=transit", `as-path="65001 64512"`)
	if lines := runLogged(t, chain, accepted, policyview.Session{}, "import", true); lines != "" {
		t.Fatalf("a trace wrote %q, want nothing", lines)
	}
}
