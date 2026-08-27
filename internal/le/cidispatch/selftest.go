// Design: docs/architecture/cli/command-namespacing.md -- the call-site gate, proved against fixtures
//
// selftest.go exercises the resolver and the emitter recogniser on fixtures, so
// a regression in either shows up as a failure rather than as silence.
//
// Three things are asserted and each one can break on its own: the command
// surface loaded at all, a command the migration removed still does not
// resolve, and the recogniser tells a dead literal, a dead static prefix, a
// prefix-less computed command and a pass-through variable apart.
//
// The file-selection rule is asserted in BOTH directions. The scanned half is
// what stops the unit-test exemption from quietly widening into "any file with
// test in the name"; the skipped half is what stops it from being deleted by
// someone who reads it as dead code.
//
// The table is declared ONCE and read twice: `le ci-dispatch selftest` runs it,
// and the package test runs the same rows so a failure names the case rather
// than a count.

package cidispatch

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
)

// surfaceFloor is the least commands the surface must carry before the selftest
// believes it loaded. A run that registered a handful would call almost every
// emitter dead.
const surfaceFloor = 100

// liveCommands must all resolve. Each is a spelling the verb-first tree
// declares today.
var liveCommands = []string{
	"show bgp", "show bgp peer list", "show bgp peer 10.0.0.1 detail",
	"request peer 10.0.0.1 flush", "request reload", "show status", "request halt",
	"monitor bgp", "show bgp health", "show runtime memory",
	"peer * update text nlri ipv4/unicast add 10.0.0.0/24",
}

// deadCommands must all fail to resolve. Each is a spelling the verb-first
// migration removed, and each was live before it.
var deadCommands = []string{
	"summary", "peer 10.0.0.1 detail", "peer 10.0.0.1 flush", "peer * list",
	"daemon reload", "daemon status", "daemon quit", "reload",
	"bgp health", "runtime memory", "bgp monitor", "bgp summary",
	"rpki status", "adj-rib-in show",
}

// fixtureSource holds one of every shape the recogniser must tell apart.
var fixtureSource = strings.Join([]string{
	// A removed spelling, verbatim: the defect this gate exists for.
	"    resp = dispatch(api, 'bgp health')",
	// A live command: must not be flagged.
	"    resp = dispatch(api, 'show bgp')",
	// A pass-through variable: no fixed command, counted not failed.
	"    resp = dispatch(api, cmd_from_a_variable)",
	// Computed but with a checkable static prefix that is live.
	"    resp = dispatch(api, 'show bgp peer ' + addr + ' detail')",
	// Computed with a static prefix that is DEAD: must be caught.
	"    resp = dispatch(api, 'bgp health for ' + addr)",
	// Computed with no static prefix at all: unverifiable, fails loudly.
	"    resp = dispatch(api, build_command(kind))",
	// The documented escape hatch, which must actually exempt.
	"    # ze-dispatch-check: dynamic -- table driven, each entry checked below",
	"    resp = dispatch(api, build_command(other))",
}, "\n")

// scannedPaths must all be scanned: each drives a daemon.
var scannedPaths = []string{
	"test/plugin/cursor-replay.ci",
	"test/scripts/ze_api.py",
	"test/perf/run.py",
	"internal/component/ssh/ssh.go",
	"scripts/dev/ci_observer_recover_check.py",
}

// skippedPaths must all be skipped: each is a unit test of the tooling, or not
// source at all.
var skippedPaths = []string{
	"test/scripts/ze_api_test.py",
	"scripts/dev/ci_observer_recover_check_test.py",
	"scripts/checks/ci_dispatch_commands_test.go",
	"docs/performance.md",
}

// selftestCase is one property the gate must have. check answers the empty
// string when it holds, and what the failure means otherwise.
type selftestCase struct {
	name  string
	check func(surface Surface, fixture fixtureResult) string
}

// fixtureResult is what one scan of fixtureSource produced.
type fixtureResult struct {
	findings     []Finding
	passthroughs int
}

// selftestCases is the whole selftest.
var selftestCases = []selftestCase{
	{
		name: "surface-loaded",
		check: func(surface Surface, _ fixtureResult) string {
			if len(surface.keys) >= surfaceFloor {
				return ""
			}
			var tb textbuf.Buffer
			return tb.Str("only ").Int(int64(len(surface.keys))).
				Str(" commands registered; the surface did not load").String()
		},
	},
	{
		name: "live-commands-resolve",
		check: func(surface Surface, _ fixtureResult) string {
			for _, live := range liveCommands {
				if surface.Resolves(live) {
					continue
				}
				var tb textbuf.Buffer
				return tb.Str("live command ").Quoted(live).Str(" did not resolve").String()
			}
			return ""
		},
	},
	{
		name: "removed-commands-do-not-resolve",
		check: func(surface Surface, _ fixtureResult) string {
			for _, dead := range deadCommands {
				if !surface.Resolves(dead) {
					continue
				}
				var tb textbuf.Buffer
				return tb.Str("removed command ").Quoted(dead).Str(" still resolved").String()
			}
			return ""
		},
	},
	{
		name: "fixture-finding-count",
		check: func(_ Surface, fixture fixtureResult) string {
			if len(fixture.findings) == 3 {
				return ""
			}
			var tb textbuf.Buffer
			return tb.Str("fixture: expected 3 findings, got ").Int(int64(len(fixture.findings))).String()
		},
	},
	{
		name: "fixture-dead-literal",
		check: func(_ Surface, fixture fixtureResult) string {
			if len(fixture.findings) > 0 && fixture.findings[0].Kind == KindDead && fixture.findings[0].Command == "bgp health" {
				return ""
			}
			return "fixture: expected a dead literal 'bgp health' as the first finding"
		},
	},
	{
		name: "fixture-dead-prefix",
		check: func(_ Surface, fixture fixtureResult) string {
			if len(fixture.findings) > 1 && fixture.findings[1].Kind == KindDead {
				return ""
			}
			return "fixture: expected a dead static prefix to be caught"
		},
	},
	{
		name: "fixture-unverifiable",
		check: func(_ Surface, fixture fixtureResult) string {
			if len(fixture.findings) > 2 && fixture.findings[2].Kind == KindUnverifiable {
				return ""
			}
			return "fixture: expected a prefix-less computed command to be unverifiable"
		},
	},
	{
		name: "fixture-passthrough",
		check: func(_ Surface, fixture fixtureResult) string {
			if fixture.passthroughs == 1 {
				return ""
			}
			var tb textbuf.Buffer
			return tb.Str("fixture: expected 1 pass-through variable, got ").Int(int64(fixture.passthroughs)).String()
		},
	},
	{
		name: "files-scanned",
		check: func(_ Surface, _ fixtureResult) string {
			for _, path := range scannedPaths {
				if _, _, skip := EmittersFor(path); !skip {
					continue
				}
				var tb textbuf.Buffer
				return tb.Str("file selection: ").Quoted(path).Str(" must be scanned but is skipped").String()
			}
			return ""
		},
	},
	{
		name: "files-skipped",
		check: func(_ Surface, _ fixtureResult) string {
			for _, path := range skippedPaths {
				if _, _, skip := EmittersFor(path); skip {
					continue
				}
				var tb textbuf.Buffer
				return tb.Str("file selection: ").Quoted(path).Str(" must be skipped but is scanned").String()
			}
			return ""
		},
	},
}

// Selftest builds the surface over tree, scans the fixture, and answers one row
// per case.
//
// The error is a surface that could not be built, which is a different fact
// from a recogniser that stopped working, so it is answered apart from the rows
// rather than as one more failing case.
func Selftest(tree string) (leroot.SelftestReport, error) {
	surface, err := NewSurface(tree)
	if err != nil {
		return leroot.SelftestReport{}, err
	}
	return SelftestWith(surface), nil
}

// SelftestWith is Selftest against a surface the caller already built, which is
// what the package test uses rather than relinking the product.
func SelftestWith(surface Surface) leroot.SelftestReport {
	findings, _, passthroughs := ScanFile(surface, "fixture.ci", fixtureSource, pyEmitters)
	fixture := fixtureResult{findings: findings, passthroughs: passthroughs}

	results := make([]leroot.SelftestResult, 0, len(selftestCases))
	for _, testCase := range selftestCases {
		if detail := testCase.check(surface, fixture); detail != "" {
			results = append(results, leroot.Fail(testCase.name, detail))
			continue
		}
		results = append(results, leroot.Pass(testCase.name))
	}

	return leroot.NewSelftestReport(
		"ci-dispatch-check selftest: OK",
		"ci-dispatch-check selftest FAILED:",
		results...,
	)
}

// runSelftest is the `le ci-dispatch selftest` action.
func runSelftest() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, err := Selftest(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return report, report.Code(1)
}
