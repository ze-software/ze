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

// goTestVerb is the go subcommand that runs tests.
const goTestVerb = "test"

// verifyArea is the le area that runs the verification gate, apart from its
// two read-only actions (heavyArea).
const verifyArea = "verify"

// lossyPipeRefusal is the piped-command refusal for the expensive segment.
// It shows the caller's own command in the quiet admitted form, which keeps
// the whole output in this session's scratch log and answers the exit code,
// the log's path and the failure lines. It never advises a redirect or a
// tee into a scratch path, which ai/rules/commands.md forbids. A
// verification command gets the scoped package run instead, because no agent
// runs one (ai/META-DECISIONS.md), so the refusal never names it.
func lossyPipeRefusal(filter, segment string) *verdict {
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
			"     ./le test unit pkg path <package>"}
	}
	advice := head +
		"  -- Run it with no filter, redirect or tee, in the quiet admitted form (ai/rules/commands.md). Its output goes to this session's scratch log, and the answer carries the exit code, the log's path and the failure lines:\n" +
		"     " + admittedCommand(lossyPipeLabel(words), true, words) + "\n"
	if goTest(words) {
		advice += "  -- For one package with its feature tags, the short form: ./le test unit pkg path <package>\n"
	}
	return &verdict{2, advice + "  -- Then read the log it names with the Read tool, offset and limit."}
}

// lossyPipeLabel names the job the piped-command refusal advises. A go test
// and a lint take the labels the raw-job refusal gives them (rawHeavy), and
// any other program takes its name and its first argument. MUST be called
// with at least one word.
func lossyPipeLabel(words []string) string {
	base := filepath.Base(words[0])
	if goTest(words) {
		return "unit-pkg"
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
