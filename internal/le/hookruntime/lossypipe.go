// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: bash.go -- bashLossyPipe, the guard whose refusal this writes
package hookruntime

import (
	"path/filepath"
	"strings"
)

// lintCommand is the command word of the Go linter, which the cost judgment,
// the raw-job refusal and the piped-command refusal all name.
const lintCommand = "golangci-lint"

// goTestVerb is the go subcommand that runs tests, and goVetVerb the one
// that vets.
const (
	goTestVerb = "test"
	goVetVerb  = "vet"
)

// shortPackageForm is the short package action both refusals name for a
// package test: `./le test unit pkg`, which adds the feature tags and the
// race detector and admits the run.
const shortPackageForm = pkgActionCommand + " " + pkgPathKeyword + " <package>"

// verifyArea is the le area that runs the verification gate, apart from its
// two read-only actions (heavyArea).
const verifyArea = "verify"

// lossyPipeRefusal is the piped-command refusal for the expensive segment.
// It shows the caller's own command in the quiet admitted form, which keeps
// the whole output in this session's scratch log and answers the exit code,
// the log's path and the failure lines. It never advises a redirect or a
// tee into a scratch path, which ai/rules/commands.md forbids. A
// verification command gets the scoped package run instead, because no agent
// runs one (ai/META-DECISIONS.md), so the refusal never names it. identity is
// the caller's identity line (commandIdentityPrefix), which the advised
// command starts with so that following it is not refused again.
func lossyPipeRefusal(identity, filter, segment string) *verdict {
	words := beforeRedirection(programWords(segment))
	head := "❌ Blocked: piping an expensive command's output through a lossy filter (" + filter + ")\n" +
		"  -- The filter drops the lines that say why a run failed, and what it keeps is what you would judge the run by.\n"
	if len(words) == 0 {
		return &verdict{2, head +
			"  -- Run the command with no filter, redirect or tee, in the quiet admitted form (ai/rules/commands.md):\n" +
			"     ./le job run label <label> quiet command <argv...>\n" +
			"  -- Then read the log it names with the Read tool, offset and limit."}
	}
	if verifyCommand(words) {
		return &verdict{2, head +
			"  -- No agent runs a verification command: GitHub CI verifies a pushed commit (ai/META-DECISIONS.md).\n" +
			"  -- The evidence you owe is the scoped run of each package your change reaches, which runs quiet:\n" +
			"     " + shortPackageForm}
	}
	advice := head +
		"  -- Run it with no filter, redirect or tee, in the quiet admitted form (ai/rules/commands.md). Its output goes to this session's scratch log, and the answer carries the exit code, the log's path and the failure lines:\n" +
		"     " + identity + admittedCommand(lossyPipeLabel(words), words) + "\n"
	if goTest(words) {
		advice += "  -- For one package with its feature tags, the short form: " + shortPackageForm + "\n"
	}
	return &verdict{2, advice + "  -- Then read the log it names with the Read tool, offset and limit."}
}

// advisedSegment answers the command text the piped-command refusal shows.
// The guard's verdict reads a quote-blind split, which keeps HEAD's text
// verdict, but that split cuts `go test -run 'TestA|TestB' ./x | tail` at
// the quoted bar and would advise a different command. On a line the hook's
// splitter models, the advice is the first expensive command, quotes intact,
// of the first pipeline where a lossy filter follows it, so an expensive
// command of another statement is never advised in its place (review I2);
// any other line keeps the text segment.
func advisedSegment(command, textSegment string) string {
	if !modeledSyntax(command) {
		return textSegment
	}
	line, parsed := lineCommands(command)
	if !parsed {
		return textSegment
	}
	start := 0
	for end := range line.commands {
		if oneOf(line.separators[end], "|", "|&") {
			continue
		}
		if segment, found := pipelineExpensive(line, start, end); found {
			return segment
		}
		start = end + 1
	}
	return textSegment
}

// pipelineExpensive answers the first expensive command of the pipeline
// line.commands[start..end] that a lossy filter follows in the same pipeline.
func pipelineExpensive(line shellLine, start, end int) (string, bool) {
	for index := start; index < end; index++ {
		if !commandExpensive(line.segments[index]) {
			continue
		}
		for filter := index + 1; filter <= end; filter++ {
			if lossyFilter(line.commands[filter][0]) {
				return line.segments[index], true
			}
		}
	}
	return "", false
}

// lossyFilter reports a program that drops lines of what it reads: the
// filters the piped-command refusal judges.
func lossyFilter(program string) bool {
	return oneOf(program, "head", "tail", "grep", "egrep", "fgrep", "awk", "sed", catCommand, "less", "more")
}

// lossyPipeLabel names the job the piped-command refusal advises. A go test
// and a lint take the labels the raw-job refusal gives them (rawHeavy), and
// any other program takes its name and its first argument. MUST be called
// with at least one word.
func lossyPipeLabel(words []string) string {
	base := filepath.Base(words[0])
	if goTest(words) {
		return testLabel
	}
	if base == lintCommand {
		return "lint"
	}
	if len(words) > 1 {
		return base + "-" + filepath.Base(words[1])
	}
	return base
}

// goTest reports whether program words run `go test`.
func goTest(words []string) bool {
	if len(words) < 2 {
		return false
	}
	return filepath.Base(words[0]) == "go" && words[1] == goTestVerb
}

// goVet reports whether program words run `go vet`.
func goVet(words []string) bool {
	if len(words) < 2 {
		return false
	}
	return filepath.Base(words[0]) == "go" && words[1] == goVetVerb
}

// verifyCommand reports whether program words run an le verification area,
// `le verify ...` or `ze le verify ...`. commandExpensive already judged the
// words heavy, so the areas that run nothing (`verify status`, `verify
// summary`) never reach it. MUST be called with at least one word.
func verifyCommand(words []string) bool {
	base := filepath.Base(words[0])
	if base == "le" {
		return len(words) > 1 && words[1] == verifyArea
	}
	if strings.HasPrefix(base, "ze") {
		return len(words) > 2 && words[1] == "le" && words[2] == verifyArea
	}
	return false
}
