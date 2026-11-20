// Design: docs/contributing/running-commands.md -- owner authorization requires a terminal
// Related: bash.go -- the Bash guards this pair sits beside
// Related: readerline.go -- judgeReaderWalk and readerAdmit, the reader-line judgment the Bash guard asks
package hookruntime

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// ownerRecordsDir is the directory of Thomas's design approvals, his away
// record and the judge's verdicts, written only by `./le spec design`.
const ownerRecordsDir = "tmp/spec-design"

// ownerRecordPath matches a slash path that lies in the records directory of
// any checkout: the directory itself, or a path below it.
var ownerRecordPath = regexp.MustCompile(`(^|/)` + regexp.QuoteMeta(ownerRecordsDir) + `(/|$)`)

// ownerRecordWord matches a command that names the records directory: the
// directory name, starting a word or after a slash, and not followed by a
// character a longer name would carry. `./le spec design` names no path
// under it, and a spec named spec-design-* never carries tmp/ before it.
var ownerRecordWord = regexp.MustCompile(`(^|[^A-Za-z0-9_.-])` + regexp.QuoteMeta(ownerRecordsDir) + `($|[^A-Za-z0-9_.-])`)

// ownerRecordTools are the tools that write the file their path names.
var ownerRecordTools = []string{toolWrite, toolEdit, toolMultiEdit, "NotebookEdit"}

// ze point: none -- the rule that only Thomas's terminal records an owner authorization lives in ai/META-DECISIONS.md, "Test changes", outside the rule corpus
// writeOwnerRecord refuses a Write, Edit, MultiEdit or NotebookEdit whose path
// lies under tmp/spec-design/, where only `./le spec design` writes Thomas's
// approvals (plan/release/validation/spec-design-needs-owner-approval.md,
// AC-34). The path is judged in every checkout, a worktree's included,
// because each one's records admit its own claims.
func writeOwnerRecord(ctx context) *verdict {
	if !oneOf(ctx.tool, ownerRecordTools...) {
		return nil
	}
	if !ownerRecordPath.MatchString(filepath.ToSlash(absolutePath(ctx))) {
		return nil
	}
	return ownerRecordBlockVerdict("the " + ctx.tool + " of " + relativePath(ctx))
}

// ze point: none -- the rule that only Thomas's terminal records an owner authorization lives in ai/META-DECISIONS.md, "Test changes", outside the rule corpus
// bashOwnerRecord refuses a Bash line that names tmp/spec-design/ and is not
// made only of readers (the reader-line walk over the call's one parse, with
// ownerRecordAdmit), so `ls` and `cat` of the records pass, and a
// redirection, a copy or a removal does not (AC-34). A path built at run time
// hides from it: that residue is the wrapper ai/META-DECISIONS.md already
// bans.
func bashOwnerRecord(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	if !ownerRecordWord.MatchString(command) {
		return nil
	}
	walk := readerWalk{source: command, admit: ownerRecordAdmit}
	if _, outside := judgeReaderWalk(command, ctx.commandParse(), "", walk); outside == "" {
		return nil
	}
	return ownerRecordBlockVerdict("a shell line that writes there")
}

// leCommandNames are the spellings of the le launcher a line runs it by.
var leCommandNames = []string{"./le", "le", "bin/le", "./bin/le"}

// ownerRecordAdmit admits what readerAdmit admits, and a `./le commit create`
// call: it writes its message and script under tmp/commit-*, never here, so a
// subject or body that names the directory is data. Every other le action is
// judged as any command, since one (job run) runs a command it is handed.
func ownerRecordAdmit(command lineCommand) string {
	if leCommitCreate(command) {
		return ""
	}
	return readerAdmit(command)
}

// leCommitCreate reports whether command is `./le commit create`, its three
// words typed literally.
func leCommitCreate(command lineCommand) bool {
	if len(command.words) < 3 {
		return false
	}
	if slices.Contains(command.expands[:3], true) {
		return false
	}
	if !oneOf(command.words[0], leCommandNames...) {
		return false
	}
	return slices.Equal(command.words[1:3], []string{"commit", "create"})
}

// ownerRecordBlockVerdict is the refusal both guards print: what, why, and the
// command to hand Thomas.
func ownerRecordBlockVerdict(what string) *verdict {
	return &verdict{2, strings.Join([]string{
		red + bold + "BLOCKED: tmp/spec-design/ holds Thomas's approvals, written only by ./le spec design" + reset,
		"  refused: " + what,
		"  why: an approval record admits a spec's claim, so only Thomas's terminal or the judge action may write one",
		"  fix: hand Thomas the command to run from his own terminal:",
		"    ./le spec design approve spec <spec-path> words \"<what he said>\"",
		"  or, for a judge verdict, run ./le spec design judge from the judge agent",
	}, "\n")}
}
