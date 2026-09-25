// Design: docs/architecture/testing/ci-format.md -- every harness command is `le test <name>`
// Related: ../../../test/cli/ci_runner.go -- the suite runner a suite command calls
//
// Package harnesstool adapts one handler of the test harness to an le command
// under the `test` namespace (plan/spec-le-subject-first-command-tree.md, D-8).
//
// It registers nothing itself and holds no register.go. Each harness command
// has its own package at internal/le/test/<name>, whose register.go calls
// leroot.Register, leroot.RegisterShape and leroot.RegisterForwarding with the
// answer and help built here, so a new harness command adds one package and
// edits no list.
package harnesstool

import (
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/test/cli"
	"github.com/ze-software/ze/internal/test/runner"
)

// Answer adapts a harness handler to an le answer. Every word after
// `test <name>` reaches run unchanged, and run's exit code is le's exit code.
// The answer carries no payload, because the harness writes its own output.
//
// The command MUST be registered as forwarding (leroot.RegisterForwarding), so
// a trailing help word reaches run and the harness prints its own help.
func Answer(run func([]string) int) leroot.Answer {
	return func(args []string) (any, int) {
		return nil, run(args)
	}
}

// Meta answers the help metadata of a harness command.
func Meta(shortHelp string) registry.Meta {
	return registry.Meta{
		ShortHelp: shortHelp,
		Mode:      "offline",
		Section:   registry.SectionTest,
	}
}

// SuiteAnswer adapts one functional suite to an le answer that runs the .ci
// files under test/<suite.TestSubdir>.
//
// suite.DefaultParallel is the DEFAULT for -p, not a ceiling. Zero means "no
// opinion, use the host-derived default" (runner.DefaultSuiteConcurrency), NOT
// "all at once": unset and all-at-once used to be the same value, which is how
// `ospf --all` came to launch 97 ze daemons simultaneously and kill CI's runner
// agent. An operator who genuinely wants that behavior still asks for it with
// `-p 0`. The default is resolved when the command runs, so registration reads
// no host state.
func SuiteAnswer(suite cli.CIRunnerConfig) leroot.Answer {
	return Answer(func(args []string) int {
		resolved := suite
		if resolved.DefaultParallel == 0 {
			resolved.DefaultParallel = runner.DefaultSuiteConcurrency()
		}
		return cli.RunCISubcommand(resolved, args)
	})
}

// SuiteMeta answers the help metadata of a suite command.
func SuiteMeta(suite cli.CIRunnerConfig) registry.Meta {
	var tb textbuf.Buffer
	return Meta(tb.Str("Run ").Str(suite.Description).Str(" functional tests").String())
}
