// Design: docs/contributing/running-commands.md -- heavy work starts in the main thread, under job admission

// Package heavytool holds the one definition of a heavy tool: an external
// program that runs a build, a test, an analysis or a rewrite over the
// packages its line names, so its run is work the shared machine schedules.
// The tools are the Go toolchain's executing verbs, the linters and the
// formatters. They are external programs and cannot register, so they are
// named here, each once.
//
// Three callers read the definition. The Bash hook hands back a subagent's
// line whose program Name names (internal/le/hookruntime, heavyTool). The
// main thread's raw-job guard refuses a narrower set on purpose, because it
// advises a short form, and reads the linter's cheap subcommands from here
// (internal/le/hookruntime, rawHeavyWords, over LintRunsNoAnalysis). `le exec`
// will take a job slot only when the program it carries is one Name names.
//
// The definition is a deny list. A program it does not name is not heavy,
// whatever it runs: that gap is accepted by the owner.
//
// The package reads words only. Reading a shell line down to a program's
// words (assignments, launchers, the parse tree) is the caller's job, and so
// is judging an `le` line, which the registry answers. It imports the
// standard library only, so every caller can import it without a cycle.
package heavytool

import (
	"path/filepath"
	"slices"
)

// goProgram is the Go toolchain's program name.
const goProgram = "go"

// Linter is the name of the linter whose subcommands decide whether it is
// heavy (LintRunsNoAnalysis).
const Linter = "golangci-lint"

// goVerbs holds the Go toolchain's verbs that execute over the packages a
// line names. version, env, list, doc and help read and start no build of
// the tree, so they are absent. fmt and fix are the toolchain's own formatter
// and rewriter: each runs over every package the line names, as gofmt and the
// linter do.
var goVerbs = []string{"test", "build", "vet", "run", "generate", "install", "fmt", "fix"}

// tools holds the linters and formatters that are heavy whatever their
// arguments are.
var tools = []string{"staticcheck", "govulncheck", "gofmt", "goimports", "gofumpt"}

// lintSubcommandsWithoutAnalysis holds the subcommands of Linter that run no
// analysis.
var lintSubcommandsWithoutAnalysis = []string{"config", "version", "help", "linters", "cache", "completion"}

// Name answers the name of the heavy tool that program, one command's words
// from its program on, starts: `go` and its verb (`go test`), or the tool's
// base name (`gofmt`, `golangci-lint`), the way a refusal names it. The second
// result says whether the words start a heavy tool at all, so a caller
// branches on it and never on an empty name. The program is judged by its
// base name, so `/usr/local/go/bin/go test` is `go test`. Empty words start
// nothing.
func Name(program []string) (string, bool) {
	if len(program) == 0 {
		return "", false
	}
	base := filepath.Base(program[0])
	if base == goProgram {
		if len(program) > 1 && slices.Contains(goVerbs, program[1]) {
			return goProgram + " " + program[1], true
		}
		return "", false
	}
	if base == Linter {
		if LintRunsNoAnalysis(program) {
			return "", false
		}
		return Linter, true
	}
	if slices.Contains(tools, base) {
		return base, true
	}
	return "", false
}

// LintRunsNoAnalysis reports whether program, a Linter command from its
// program on, names one of the linter's subcommands that run no analysis: the
// first word after the program that is not an option is the subcommand. A
// line that names no subcommand answers false, because the linter may then
// analyze. It does not judge the program's name: the caller has.
func LintRunsNoAnalysis(program []string) bool {
	for index := 1; index < len(program); index++ {
		if program[index] != "" && program[index][0] == '-' {
			continue
		}
		return slices.Contains(lintSubcommandsWithoutAnalysis, program[index])
	}
	return false
}
