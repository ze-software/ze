// Design: docs/architecture/cli/command-namespacing.md -- call-site checks
//
// The selftest exercises the resolver and native Go emitter recognizer against
// fixtures. It distinguishes live and removed literals, static prefixes,
// computed commands, and pass-through variables. It also pins which source
// files are scanned.

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
	"request reload", "show status", "request halt",
	"monitor bgp", "show bgp health", "show runtime memory",
}

// deadCommands must all fail to resolve. Each is a spelling removed by the
// verb-first command tree.
var deadCommands = []string{
	"summary", "peer 10.0.0.1 detail", "peer 10.0.0.1 flush", "peer * list",
	"daemon reload", "daemon status", "daemon quit", "reload",
	"bgp health", "runtime memory", "bgp monitor", "bgp summary",
	"rpki status", "adj-rib-in show",
}

// fixtureSource holds one of every shape the recognizer must tell apart.
var fixtureSource = strings.Join([]string{
	"    response, err := client.SendCommand(\"bgp health\")",
	"    response, err := client.SendCommand(\"show bgp\")",
	"    response, err := client.SendCommand(command)",
	"    response, err := client.SendCommand(\"show bgp peer \" + address + \" detail\")",
	"    response, err := client.SendCommand(\"bgp health for \" + address)",
	"    response, err := client.SendCommand(buildCommand(kind))",
	"    // le-ci-dispatch: dynamic -- table driven, each entry checked below",
	"    response, err := client.SendCommand(buildCommand(other))",
}, "\n")

// scannedPaths must all be scanned because each can emit a daemon command.
var scannedPaths = []string{
	"internal/component/cli/client/main.go",
	"internal/component/config/cli/cmd_archive.go",
}

// skippedPaths must all be skipped because they are tests or prose.
var skippedPaths = []string{
	"internal/component/ssh/answer_test.go",
	"internal/le/cidispatch/cidispatch_test.go",
	"docs/performance.md",
}

// selftestCase is one property the check must have. check answers the empty
// string when it holds, and explains the failure otherwise.
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
				if _, skip := emittersFor(path); !skip {
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
				if _, skip := emittersFor(path); skip {
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
	surface, err := newSurface(tree)
	if err != nil {
		return leroot.SelftestReport{}, err
	}
	return selftestWith(surface), nil
}

// selftestWith is Selftest against a surface the caller already built, which is
// what the package test uses rather than relinking the product.
func selftestWith(surface Surface) leroot.SelftestReport {
	findings, _, passthroughs := ScanFile(surface, "fixture.go", fixtureSource, goEmitters)
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
		"ci-dispatch selftest: OK",
		"ci-dispatch selftest FAILED:",
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
