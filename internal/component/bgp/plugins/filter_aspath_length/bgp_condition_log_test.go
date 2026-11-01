package filter_aspath_length

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

// TestBGPMatchLogsASPathLengthReject proves a match naming an
// as-path-length instance writes the line its own filter wrote
// (handleFilterUpdate): one Info line for a reject naming the instance as
// filter=, the peer, the hop count and the bounds, none for an accept, none
// in a display-only trace. Method: capture the logger and run a path within
// and one beyond max 3, then the long one as a trace.
func TestBGPMatchLogsASPathLengthReject(t *testing.T) {
	chain := matchChain(t, "short")
	long := bgpUpdate(t, pathOf(65001, 65002, 65003, 65004))
	wantLine(t, runLogged(t, chain, long, policyview.Session{}, "import", false),
		"level=INFO", `msg="as-path-length reject"`, "filter=short", "peer=192.0.2.9", "length=4", "max=3", "min=-1")
	if lines := runLogged(t, chain, bgpUpdate(t, pathOf(65001)), policyview.Session{}, "import", false); lines != "" {
		t.Fatalf("an accept wrote %q, want nothing", lines)
	}
	if lines := runLogged(t, chain, long, policyview.Session{}, "import", true); lines != "" {
		t.Fatalf("a trace wrote %q, want nothing", lines)
	}
}
