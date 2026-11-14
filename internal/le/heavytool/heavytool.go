// Design: docs/contributing/running-commands.md -- heavy work starts in the main thread, under job admission

// Package heavytool holds the one definition of a heavy tool: an external
// program that runs a build, a test, an analysis or a rewrite over the
// packages its line names, so its run is work the shared machine schedules.
// The tools are the Go toolchain's executing verbs, the linters and the
// formatters. They are external programs and cannot register, so they are
// named here, each once.
//
// The package also holds the one reading of a command's words down to its
// program (Split, Program, Started): the assignments and launchers in front
// of it (launcherValueOptions) and the Go toolchain's flag in front of its
// verb. Three callers read the definition through it. The Bash hook hands
// back a subagent's line whose words Started names (internal/le/hookruntime,
// heavyTool). The main thread's raw-job guard refuses a narrower set on
// purpose, because it advises a short form, and reads the words in front of
// the program (Split) and the linter's cheap subcommands from here
// (internal/le/hookruntime, rawHeavyWords, over LintRunsNoAnalysis). `le exec`
// takes a job slot only when the words it carries are ones Started names
// (internal/le/exec, holdHeavy), and its line is heavy for the hook by the
// same answer (internal/le/exec, carriesHeavy), so the hook and `le exec`
// never read `env go test` two ways.
//
// The definition is a deny list. A program it does not name is not heavy,
// whatever it runs: that gap is accepted by the owner.
//
// The package reads words only. Reading a shell line into words (the parse
// tree, a string handed to a shell, which ShellCommand only finds) is the
// caller's job, and so is judging an `le` line, which the registry answers:
// LeWords only says which words are one. It imports the standard library
// only, so every caller can import it without a cycle.
package heavytool

import (
	"path/filepath"
	"slices"
	"strings"
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
// nothing. The verb is the word after `go`, so a caller holding a command's
// words as typed asks Started, which first reads through what stands in
// front of the program and of the verb.
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

// LeWords answers the words typed after le when program, one command's words
// from its program on, runs le: as `le`, `./le` or a path ending in `/le`, or
// as the `ze le` personality of a binary whose name starts with `ze`. It
// answers false for any other program, and for empty words. It recognizes the
// program and judges nothing: whether the le line is heavy is the registry's
// answer (leroot.HeavyLine), which this package cannot import. The Bash hook
// reads a shell line's le program through it, and `le exec` the le program it
// carries, so the two cannot disagree about what runs le.
func LeWords(program []string) ([]string, bool) {
	if len(program) == 0 {
		return nil, false
	}
	base := filepath.Base(program[0])
	if base == "le" {
		return program[1:], true
	}
	if strings.HasPrefix(base, "ze") && len(program) > 2 && program[1] == "le" {
		return program[2:], true
	}
	return nil, false
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
