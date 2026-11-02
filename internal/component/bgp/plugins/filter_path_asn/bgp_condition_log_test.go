package filter_path_asn

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

// runLogged runs chain over update as a live import from 192.0.2.9,
// or as a display-only trace when trace is set, and answers the
// lines the logger wrote.
func runLogged(t *testing.T, chain *policychain.Chain, update *wireu.WireUpdate, session policyview.Session, trace bool) string {
	t.Helper()
	buf := captureLogger(t, slog.LevelDebug)
	var view policyview.View
	view.Reset(update, session)
	defer view.Clear()
	sets := new(policychain.RunSets)
	sets.SetRoute(policychain.ExternalRoute{Direction: "import", Peer: "192.0.2.9"})
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

// TestBGPMatchLogsRejectASNDecision proves a match naming a reject-asn list
// writes the lines the list's own filter wrote (the removed filter-update handler,
// logListedReject, logPatternReject): Debug for an accept, Info for a
// reject naming the ASN and its position or the pattern, each with the list
// as filter=, the direction, the peer and the AS path, and none in a
// display-only trace. Method: capture the logger at Debug and run an import
// from AS 65001 with a clean path, one carrying 3356, one the pattern hits.
func TestBGPMatchLogsRejectASNDecision(t *testing.T) {
	chain := rejectASNChain(t)
	from := policyview.Session{EBGP: true, Sender: 65001, SenderKnown: true}
	clean := pathUpdate(t, 65001, 65002)
	wantLine(t, runLogged(t, chain, clean, from, false),
		"level=DEBUG", `msg="reject-asn accept"`, "filter=guard", "direction=import", "peer=192.0.2.9", `as-path="65001 65002"`)
	wantLine(t, runLogged(t, chain, pathUpdate(t, 65001, 3356, 65002), from, false),
		"level=INFO", `msg="reject-asn reject"`, "filter=guard", "direction=import", "asn=3356", "position=transit", `as-path="65001 3356 65002"`)
	wantLine(t, runLogged(t, chain, pathUpdate(t, 65001, 64512), from, false),
		"level=INFO", `msg="reject-asn reject"`, "filter=guard", `pattern="^65001 64512$"`, `as-path="65001 64512"`)
	if lines := runLogged(t, chain, clean, from, true); lines != "" {
		t.Fatalf("a trace wrote %q, want nothing", lines)
	}
}
