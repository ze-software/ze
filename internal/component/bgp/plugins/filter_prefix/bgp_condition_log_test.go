package filter_prefix

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
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

// runFromPeer runs chain over update as a live import from 192.0.2.9, or as
// a display-only trace when trace is set.
func runFromPeer(t *testing.T, chain *policychain.Chain, nlri []byte, trace bool) {
	t.Helper()
	var view policyview.View
	view.Reset(bgpUpdate(t, nil, nil, nlri), policyview.Session{})
	defer view.Clear()
	sets := new(policychain.RunSets)
	sets.SetRoute(policychain.ExternalRoute{Direction: "import", Peer: "192.0.2.9"})
	if trace {
		chain.Trace(&view, sets, func(policychain.MemberTrace) {})
		return
	}
	chain.Run(&view, sets)
}

// TestBGPMatchLogsPrefixListDecision proves a match naming a prefix-list
// writes the line the list's own filter wrote for each decision
// (handleFilterUpdate): one Info line per UPDATE on the package's logger,
// naming the verdict, the list as filter=, the peer, the judged prefixes,
// and for a modify the accepted and rejected counts. Method: capture the
// logger at Info and run an accepted, a rejected and a mixed UPDATE.
func TestBGPMatchLogsPrefixListDecision(t *testing.T) {
	chain := matchChain(t)
	cases := []struct {
		name string
		nlri []byte
		want []string
	}{
		{"accept", []byte{24, 10, 1, 0}, []string{`msg="prefix-list accept"`, "nlri=10.1.0.0/24"}},
		{"reject", []byte{24, 192, 0, 2}, []string{`msg="prefix-list reject"`, "nlri=192.0.2.0/24"}},
		{"modify", []byte{24, 10, 1, 0, 24, 192, 0, 2},
			[]string{`msg="prefix-list modify"`, "accepted=1 rejected=1", `nlri="10.1.0.0/24 192.0.2.0/24"`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := captureLogger(t, slog.LevelInfo)
			runFromPeer(t, chain, tc.nlri, false)
			line := buf.String()
			if n := strings.Count(line, "\n"); n != 1 {
				t.Fatalf("wrote %d lines, want 1: %q", n, line)
			}
			for _, want := range append(tc.want, "level=INFO", "filter=protected", "peer=192.0.2.9") {
				if !strings.Contains(line, want) {
					t.Fatalf("line %q lacks %q", line, want)
				}
			}
		})
	}
}

// TestBGPMatchTraceLogsNothing proves a display-only run (show policy test)
// writes no decision line: no route is refused. Method: a mixed UPDATE
// through Chain.Trace with the logger captured at Debug.
func TestBGPMatchTraceLogsNothing(t *testing.T) {
	chain := matchChain(t)
	buf := captureLogger(t, slog.LevelDebug)
	runFromPeer(t, chain, []byte{24, 10, 1, 0, 24, 192, 0, 2}, true)
	if buf.Len() != 0 {
		t.Fatalf("a trace wrote %q, want nothing", buf.String())
	}
}

// TestBGPMatchLogDisabledAllocatesNothing proves the decision line costs no
// allocation when its level is off. Method: the logger at Warn, then
// testing.AllocsPerRun over a mixed UPDATE that drops a prefix.
func TestBGPMatchLogDisabledAllocatesNothing(t *testing.T) {
	chain := matchChain(t)
	buf := captureLogger(t, slog.LevelWarn)
	update := bgpUpdate(t, nil, nil, []byte{24, 10, 1, 0, 24, 192, 0, 2})
	var view policyview.View
	sets := new(policychain.RunSets)
	run := func() {
		view.Reset(update, policyview.Session{})
		sets.SetRoute(policychain.ExternalRoute{Direction: "import", Peer: "192.0.2.9"})
		if outcome := chain.Run(&view, sets); outcome.Action != policychain.OutcomeAccept {
			t.Fatalf("outcome = %+v, want accept with drops", outcome)
		}
		view.Clear()
	}
	run()
	if allocs := testing.AllocsPerRun(100, run); allocs != 0 {
		t.Fatalf("a run allocates %.1f times with the line off, want 0", allocs)
	}
	if buf.Len() != 0 {
		t.Fatalf("wrote %q at Warn, want nothing", buf.String())
	}
}
