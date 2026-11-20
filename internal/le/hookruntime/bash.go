// Design: docs/architecture/core-design.md -- native Bash hook policy
package hookruntime

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/derived"
	"github.com/ze-software/ze/internal/le/heavytool"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// catCommand is the command word `cat`, which several checks name: a lossy
// filter, a search command, a whole-document read and a reader command.
const catCommand = "cat"

// gitCommand is the command word `git`, which several checks name: a search
// command, a reader command and a program that applies a patch.
const gitCommand = "git"

var (
	goBuildCall      = regexp.MustCompile(`(^|[^A-Za-z0-9_])go\s+build([^A-Za-z0-9_]|$)`)
	goBuildBin       = regexp.MustCompile(`-o\s+bin/`)
	goBuildSession   = regexp.MustCompile(`-o\s+tmp/session/[0-9]{4}-[0-9]{2}-[0-9]{2}-[A-Za-z0-9._-]+/bin/`)
	systemTmp        = regexp.MustCompile(`(^|[\s='"$(` + "`" + `:,<>}])/tmp(/|\s|$)`)
	scratchWrite     = regexp.MustCompile(`(?:>>?\s*|\btee\s+(?:-a\s+)?)(["']?)([A-Za-z0-9_./-]*tmp/[^\s;|&)'\"]+)`)
	waitKeyword      = regexp.MustCompile(`\b(while|until)\b`)
	sleepCall        = regexp.MustCompile(`(?:^|[;&|(\s./])sleep\s*[0-9$"'(]`)
	loopPGrep        = regexp.MustCompile(`\b(while|until)\s+(?:!\s*)?(?:\[\[?\s*)?pgrep\b`)
	timeoutBound     = regexp.MustCompile(`(^|[^-A-Za-z0-9_])timeout\s+(?:-\S+\s+)*[0-9]+(?:\.[0-9]+)?[smhd]?\b`)
	rmWord           = regexp.MustCompile(`(^|[\s;&|(/` + "`" + `])rm\s`)
	governedPath     = regexp.MustCompile(`(?:plan/|ai/rules/)`)
	governedRedirect = regexp.MustCompile(`>>?[ \t]*["']?(?:plan/|ai/rules/)`)
	// The in-place flag is matched with an OPTIONAL argument prefix and inside a
	// CLUSTER of short flags, because both halves were holes. It can be the first
	// argument, `sed -i 's/a/b/' plan/x`, which a mandatory intervening argument
	// made unreachable. It can also be bundled with its neighbors: `perl -0pi`,
	// `perl -pi`, `sed -Ei`, `sed -i.bak` and `sed --in-place` all edit in place,
	// and a standalone `-i` saw none of them.
	//
	// governedArgs is one segment's arguments with a quoted word taken whole,
	// because a sed script such as 's/a/b/;s/c/d/' carries a semicolon that
	// ended the segment before the path when the class stopped at every `;`.
	// governedQuoteOpen lets the path sit inside a quoted word: "./plan/x".
	governedArgs      = `(?:'[^']*'|"[^"]*"|[^|;&\n'"])*`
	governedQuoteOpen = `(?:["'][^"'\n]*)?`
	governedSed       = regexp.MustCompile(`(?:^|[\s;&|(` + "`" + `])(?:sed|perl|ruby)[ \t]+(?:` + governedArgs + `[ \t])?-[-A-Za-z0-9]*i` + governedArgs + governedQuoteOpen + `(?:plan/|ai/rules/)`)
	governedTee       = regexp.MustCompile(`(?:^|[\s;&|(` + "`" + `])tee[ \t]+(?:-a[ \t]+)?["']?(?:plan/|ai/rules/)`)
	governedCopy      = regexp.MustCompile(`(?:^|[\s;&|(` + "`" + `])(?:mv|cp)[ \t]+` + governedArgs + `[ \t]["']?(?:plan/|ai/rules/)`)
	// The interpreter tier names every script runtime whose write calls
	// governedWrite can recognize, because a runtime missing here writes freely:
	// `python3 -c "open('plan/x','w').write(...)"` passed while the list held perl
	// and ruby alone. governedWrite holds each runtime's write calls: an open mode
	// that writes (Python and Ruby `'w'`, `'a'`, `'x'`, `'r+'`; Perl `'>'`, `'>>'`,
	// `'+<'`), the write methods, Node and Deno file writes, and a rename, replace,
	// copy or move. A mode is a whole quoted token, so the path `"ai/rules/x"` is
	// not read as append mode, and a script that only reads stays free.
	governedRuntime = regexp.MustCompile(`\b(?:perl|ruby|python[0-9.]*|node(?:js)?|deno|bun)\b`)
	governedWrite   = regexp.MustCompile(`(?:\bopen\([^)]*["'](?:[wax]|r\+)[bt+]*["']|\bopen\b[^;\n]*["'](?:\+<|\+?>)|write_(?:text|bytes)\(|\.write\(|writelines\(|truncate\(|(?:write(?:Text)?File|appendFile|copyFile|rename)(?:Sync)?\(|createWriteStream\(|os\.replace\(|shutil\.(?:copy\w*|move)\(|\.replace\([^)]*(?:plan/|ai/rules/))`)
)

// ze point: none -- the ban on copying a worktree's files into main lives in ai/INSTRUCTIONS.md, "Worktrees", outside the rule corpus
// bashWorktreeCopy refuses a file copy out of a worktree into the main tree.
// A worktree agent lands its work by committing, and a direct copy overwrites
// whatever another session left uncommitted at the destination.
//
// Scope, the owner's ruling: the guard stops an honest agent's mistake,
// never a deliberate evasion, and it must not refuse normal work. It reads
// the line as text, so a form it does not follow passes, and a form it
// cannot place is refused only where the line also reads a worktree file.
//
// The main checkout is the session's root, or, for a session started inside
// one of its worktrees, the checkout that holds .claude/worktrees
// (worktreeMainCheckout). The guard reads a line that starts under that
// checkout's .claude/ (the session's cwd) or a word of which may name a path
// under .claude/worktrees (worktreeLineConcerns), and judges the commands the
// line runs from where it starts (worktreeCopyLine): a copy verb counts only
// where it runs as a command, and a copy, a redirection or a program that
// writes what it reads only when its source is under a worktree, or may be,
// and its destination is in the main tree, or may be. A write is into the
// main tree only when its destination resolves inside the main checkout and
// outside .claude/worktrees and its gitignored tmp/; a destination the hook
// cannot place counts as the main tree. `~`, `$HOME` and `$PWD` are the
// directories they name unless the line assigns HOME or PWD, a variable the
// line assigns once by a bare assignment outside a loop or a function is its
// value, also at the head of a word that expands more, and a destination
// that starts with a variable bound only to worktree paths stays under them
// when the write runs from the directory a relative value was bound in. The
// program an le carrier runs (`./le job run ... command <argv>`, `./le exec
// <argv>`), found where the registry declares it (leroot.CarriedArgv), runs
// on this host and is judged as a command of the line, from the directory
// the line runs in. `le exec` runs it there; `le job run` runs it at the
// checkout root, so that reading is right when the line runs from a
// checkout root, which is how agents run ./le. A line another host runs
// (`ssh`, or a shell a later word of a remote runner such as `rex -S host
// run -- sh -c` is handed) writes there: its absolute paths and its `~` and
// `$HOME` paths are outside the checkout, a `cd` whose target names
// .claude/worktrees/<name> places the relative paths after it in that
// worktree while they stay inside it, a relative path that climbs from
// that worktree into the remote checkout's tmp/ is outside its main tree,
// and any other relative path stays unplaced. A line is judged by its text
// only when the parser cannot read it, when it is handed to a shell in a word whose value the hook cannot
// build, or when an expansion of the line that hands it names its program. The worktree work ai/owner/machines.md requires
// of cross-machine work (`git worktree add` and `remove`, a test run inside
// the worktree with its log there, `scp` or `rsync` of a bundle to another
// host, a page whose prose names a worktree, a remote run whose output lands
// outside the checkout, a log under `~/tmp`) copies nothing into the main
// tree, and passes.
//
// Known limits, forms a text-reading guard does not follow: eval, source or
// `.` changing the directory; a bare `exec` redirection; PWD, HOME, CDPATH or
// a bound variable assigned by read, printf -v or mapfile; a value assigned
// in a branch that may not run (an `if`, a side of `&&` or `||`, a subshell,
// a pipeline stage), which is taken as holding; a link created on
// the same line, or a /proc path; a two-step copy through tmp/ or outside the
// checkout; a copy verb's option values after its operands; interpreters and
// writers it does not enumerate (awk, sed's w, python, busybox, su -c,
// script); a worktree reached by a `cd` the hook cannot name; and an le
// carrier typed from a checkout subdirectory, judged from that subdirectory
// although `le job run` runs its argv at the checkout root.
func bashWorktreeCopy(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	session := filepath.Clean(ctx.root)
	root := worktreeMainCheckout(session)
	cwd := worktreeLineStart(session, ctx.payload.Cwd)
	parse := ctx.commandParse()
	if !worktreeLineConcerns(command, parse, root, cwd) {
		return nil
	}
	switch finding := worktreeCopyLine(command, parse, root, cwd).(type) {
	case worktreeCopyNone:
		return nil
	case worktreeCopyFound:
		return &verdict{2, worktreeCopyRefusal(worktreeCopyFoundText(finding))}
	case worktreeCopyUnread:
		return &verdict{2, worktreeCopyRefusal("The hook could not read this line, and its text may copy a file out of a worktree.\n")}
	}
	panic("BUG: worktreeCopyLine answered a finding of no known kind")
}

// worktreeUnplacedText is the line a refusal adds when its destination counts
// as the main tree only because the hook could not place it.
const worktreeUnplacedText = "A destination the hook cannot place, such as one that starts with a `$VAR` the hook cannot read, counts as the main tree.\n"

// worktreeCopyFoundText says what a refused copy writes and from where, and,
// only when the destination counts as the main tree because the hook could
// not place it, says so.
func worktreeCopyFoundText(found worktreeCopyFound) string {
	text := "It writes " + found.target + " from " + found.source + ".\n"
	if !found.unplaced {
		return text
	}
	return text + worktreeUnplacedText
}

// worktreeCopyRefusal is the refusal of a copy out of a worktree: what the
// line writes and from where (copied), why that is refused, where a run's
// output may go, and the commit route, last.
func worktreeCopyRefusal(copied string) string {
	return "❌ Blocked: copying files out of a worktree into the main tree\n" +
		copied +
		"A direct copy overwrites the uncommitted work other sessions left in the main checkout.\n" +
		"A worktree agent commits on its own branch, and the user lands that branch on main with `git rebase <branch>`.\n" +
		"To keep a run's output, write it inside the worktree, under the checkout's tmp/, or outside the checkout.\n" +
		"Commit your work on the worktree branch with:\n" +
		"./le commit create subject \"<subject>\" file <path>"
}

// shellInvoker matches a command that hands a quoted string to another shell to
// RUN. Quotes mean prose everywhere else, and here they mean the opposite, so
// the prose exemption below is withdrawn for the whole command when this
// matches.
var shellInvoker = regexp.MustCompile(`(?:^|[\s;&|(])(?:(?:ba|z|k|da)?sh)[ \t]+(?:-[A-Za-z]*c\b|--command\b)|(?:^|[\s;&|(])eval\b`)

// insideQuotes reports whether offset sits inside a single- or double-quoted
// string. A backslash escapes the next byte, which is how a shell-quoted
// alternation such as `a\|b` reads.
func insideQuotes(command string, offset int) bool {
	var quote byte
	for i := 0; i < offset && i < len(command); i++ {
		switch c := command[i]; {
		case c == '\\':
			i++
		case quote == 0 && (c == '\'' || c == '"'):
			quote = c
		case quote == c:
			quote = 0
		}
	}
	return quote != 0
}

// gitVerbRun reports whether command RUNS the given git verb, rather than
// merely naming it.
//
// A substring test cannot tell the two apart, and the difference is not
// academic: a commit message explaining why a verb is banned, a grep for one,
// and a comment quoting one were all refused, so a session could not write
// about the rule it was obeying. The verb has to sit where a command starts:
// at the beginning, or after a separator, a pipe, an opening subshell, or a
// newline. Quoting it, as `git add`, is then how prose names it safely.
//
// The end of the match is judged by the pattern's shape. A pattern that ends
// in the verb (`git merge`, the word `git` and one more) matches only where the
// verb ends too: at the end of the command, or before a byte that cannot
// continue a git command name (gitNameByte). Without that test every such
// pattern was a prefix, and `git merge-base`, which only reads history, was
// refused as `git merge`. Any other byte ends the verb, so a quote, an
// expansion, a redirect or a separator glued to it (`git merge;ls`) is still
// the verb. A pattern that continues past the verb with an argument
// (`git checkout --`, `git branch -d`) is a prefix on purpose: its own space
// already ends the verb, and its tail covers a family of forms (`--ours`,
// `HEAD~1`, the combined flag `-dr`).
func gitVerbRun(command, verb string) bool {
	prose := !shellInvoker.MatchString(command)
	verbEnding := strings.Count(verb, " ") == 1
	for offset := 0; ; {
		at := strings.Index(command[offset:], verb)
		if at < 0 {
			return false
		}
		at += offset
		offset = at + len(verb)
		// A longer command name that begins with the verb is another command,
		// so this occurrence is not a run of the verb. The scan goes on, so a
		// real run later in the line (`git merge-base a b && git merge x`) is
		// still found.
		if verbEnding && offset < len(command) && gitNameByte(command[offset]) {
			continue
		}
		// A verb inside quotes is being NAMED. The separator test alone cannot
		// see that, and the pipe is where it showed: `grep "a\|git me`+`rge" docs/`
		// puts a separator directly before the verb, so a search for the rule
		// was refused as an attempt to break it. Quoting is what the comment
		// above offers as the safe way to write about a verb, and until now it
		// worked only when no separator happened to precede it.
		if prose && insideQuotes(command, at) {
			continue
		}
		if at == 0 {
			return true
		}
		switch command[at-1] {
		case '\'', '"':
			// An opening quote is a command position only where another shell
			// is being handed the string to RUN. Everywhere else it opens
			// prose, which is the case the exemption above already took.
			if !prose {
				return true
			}
		case ' ', '\t':
			// A leading space is only a command position when what precedes
			// it is a separator rather than another word: `&& git add` runs
			// it, `about git add` names it.
			head := strings.TrimRight(command[:at], " \t")
			if head == "" || strings.HasSuffix(head, "&&") || strings.HasSuffix(head, "||") ||
				strings.HasSuffix(head, ";") || strings.HasSuffix(head, "|") ||
				strings.HasSuffix(head, "(") || strings.HasSuffix(head, "\n") {
				return true
			}
		case ';', '&', '|', '(', '\n':
			return true
		}
	}
}

// gitNameByte reports whether b can continue a git command name. Git's command
// names are ASCII letters, digits and hyphens (`merge-base`, `mergetool`), so
// every other byte ends the verb.
func gitNameByte(b byte) bool {
	return b == '-' || ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') || ('0' <= b && b <= '9')
}

// gitGlobalOptions matches the options git accepts BEFORE its verb. Every
// pattern in the two guards below is the literal text `git <verb>`, and an
// option between the two hides the verb from all of them: `git -C /path commit`
// contains no such text, so the shared index was reachable through a directory
// flag. `-c commit.gpgsign=false`, which CLAUDE.md bans outright, was reachable
// the same way.
var gitGlobalOptions = regexp.MustCompile(`(^|[\s;&|(` + "`" + `])git((?:[ \t]+(?:-C[ \t]+\S+|-c[ \t]+\S+|--git-dir(?:=\S+|[ \t]+\S+)|--work-tree(?:=\S+|[ \t]+\S+)|--namespace(?:=\S+|[ \t]+\S+)|--exec-path(?:=\S+)?|--no-pager|--paginate|--bare|--no-replace-objects|--literal-pathspecs|--glob-pathspecs|--noglob-pathspecs|--icase-pathspecs))+)`)

// gitInvocation puts the verb back beside the word `git` by removing the
// options git reads before it. A guard that compares against `git <verb>` calls
// this first, so the option is not a way around the guard.
func gitInvocation(command string) string {
	return gitGlobalOptions.ReplaceAllString(command, "${1}git")
}

// ze point: git-safety/before-destructive-actions/never-run-a-destructive-git-verb
// bashDestructiveGit refuses every git verb that discards work or publishes it.
// Committing and pushing are allowed, through the prepared script alone, because
// sessions share one index and a loose verb carries another session's changes.
func bashDestructiveGit(ctx context) *verdict {
	command := gitInvocation(stringInput(ctx.input, "command"))
	for _, pattern := range []string{
		"git commit", "git push", "git reset", "git checkout --", "git checkout -f",
		"git checkout HEAD", "git restore", "git revert", "git stash",
		"git clean", "git push --force", "git push -f",
		// The staging verbs. Several sessions share one index, so a loose
		// stage puts another session's path into your commit, or yours into
		// theirs. The generated commit script stages inside itself, and it
		// reaches the tool as `bash <script>`, so this never blocks it.
		"git add", "git rm", "git mv",
	} {
		if gitVerbRun(command, pattern) {
			return &verdict{2, "❌ Blocked: " + pattern + " (run manually)\n" +
				"Staging and committing go through ./le commit create, which writes one\n" +
				"script that commits your named paths from an index of its own.\n" +
				"To delete a tracked file, pass its path to `remove` and leave the file in place:\n" +
				"the script deletes the working-tree copy once the commit holds its content."}
		}
	}
	return nil
}

// ze point: git-safety/directives/stay-on-your-branch-and-integrate-by-rebase
// bashBranchMove refuses the verbs that create, switch, rename, delete or
// integrate a branch. Which branch a session works on is the user's choice, and
// a session that moves it lands the work where the user is not looking.
//
// `git merge` was the only one of these the guard above carried, under a
// message about staging that does not describe it. Integration is what it does,
// so it moved here beside the rest of the family.
func bashBranchMove(ctx context) *verdict {
	command := gitInvocation(stringInput(ctx.input, "command"))
	for _, pattern := range []string{
		"git merge", "git rebase", "git switch",
		// Each of these performs a merge into the index and working tree under
		// a longer name. The verb is matched as a whole word, so `git merge`
		// no longer covers them, and each is named here. Their relatives
		// `git merge-base`, `git merge-tree` and `git merge-file` write
		// neither the index nor a branch, so they pass.
		"git merge-recursive", "git merge-octopus", "git merge-resolve",
		"git merge-ours", "git merge-subtree", "git merge-index",
		"git merge-one-file", "git mergetool",
		"git checkout -b", "git checkout -B",
		"git branch -d", "git branch -D", "git branch --delete",
		"git branch -m", "git branch -M", "git branch --move",
	} {
		if gitVerbRun(command, pattern) {
			return &verdict{2, "❌ Blocked: " + pattern + " (the branch is the user's to move)\n" +
				"Stay on the branch this session started on, and ask the user to create,\n" +
				"switch, rename, delete or integrate one. A worktree branch lands on main\n" +
				"with `git rebase <branch>` run by the user, never `git merge`."}
		}
	}
	return nil
}

// ze point: none -- build hygiene; no rule states where a Go binary must be written
// bashRootBuild refuses a Go compile that writes a binary outside bin/, and a
// compile of the whole tree. Without -o the binary of a main package lands in
// the working directory under the package name, where the next tracked-file
// check reports it as an untracked artifact. A compile of packages that
// produce no binary passes, and the advice names the caller's own package
// (goBuildLine, gobuild.go).
//
// A subagent's line that the heavy-work check hands back (handsBackHeavy) is
// judged the same and refused the same, without text
// (withoutHandedBackAdvice): a subagent runs no build wherever its binary
// lands, so advice naming an output path would describe a command it is
// refused as well, and the hand-back carries the next step. A subagent's line
// that check did not hand back, one whose text names `go build` in an
// argument, is refused with a text that names no output path either. The
// main thread's verdict and text are unchanged.
func bashRootBuild(ctx context) *verdict {
	refusal := rootBuildRefusal(ctx)
	if refusal == nil {
		return nil
	}
	// A subagent's line the heavy-work check did not hand back holds no
	// build command that check reads: the text judgment met the words
	// `go build` in an argument (goBuildText). No output path is a route for
	// a caller that runs no build, so the text names none.
	unhanded := red + bold + "✘ BLOCKED: a line whose text names `go build`" + reset + "\n\n" +
		"  -- A subagent runs no build, whatever its output path.\n" +
		"  -- If the words stand in a path or a pattern and the line builds nothing, reword it so the two do not\n" +
		"     stand together, or use the Read and Edit tools on the file.\n" +
		"  -- If the line does build:\n" + owedCommandAdvice(ctx)
	return withoutHandedBackAdvice(ctx, refusal, unhanded)
}

// rootBuildRefusal is the build guard's verdict with its advice, for every
// caller: bashRootBuild decides whether the caller reads the text.
func rootBuildRefusal(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	outcome, advice := goBuildLine(ctx.root, command, ctx.commandParse(), commandIdentityPrefix(ctx))
	switch outcome {
	case goBuildAllowed:
		return nil
	case goBuildBinary:
		return &verdict{2, red + bold + "✘ BLOCKED: go build without -o bin/" + reset + "\n\n" + advice}
	case goBuildWholeTree:
		// The outcome also holds a text-judged `...` pattern under one
		// directory (goBuildText), so the header names a tree of packages.
		return &verdict{2, red + bold + "✘ BLOCKED: a go build of a package tree (a `...` pattern)" + reset + "\n\n" + advice}
	case goBuildToolchain:
		return &verdict{2, red + bold + "✘ BLOCKED: a go build flag that hands the toolchain a program or a file" + reset + "\n\n" + advice}
	case goBuildUnspecified:
	}
	return &verdict{2, red + bold + "✘ BLOCKED: BUG: the go build judgment answered no outcome" + reset}
}

// shellWords splits one command's text at whitespace into words with their
// quotes removed, keeping a redirection such as `2>&1` one word. A backslash
// outside single quotes escapes the next character, except inside double
// quotes, where bash keeps a backslash before any character but `$`, a
// backtick, `"`, `\` and a newline.
func shellWords(text string) []string {
	words := make([]string, 0, 16)
	var word textbuf.Buffer
	word.Reset()
	quote := rune(0)
	escaped := false
	flush := func() {
		if word.Len() != 0 {
			words = append(words, word.String())
		}
	}
	for index, character := range text {
		if escaped {
			word.WriteRune(character)
			escaped = false
			continue
		}
		if character == '\\' && quote == '"' && strings.IndexByte("$`\"\\\n", byteAt(text, index+1)) < 0 {
			// Inside double quotes bash keeps a backslash that precedes any
			// other character, as readWord does, so `"\/dev/null"` is not
			// /dev/null and `-o "\bin/ze"` does not name bin/.
			word.WriteRune(character)
			continue
		}
		if character == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if character == quote {
				quote = 0
			} else {
				word.WriteRune(character)
			}
			continue
		}
		if character == '\'' || character == '"' {
			quote = character
			continue
		}
		if unicode.IsSpace(character) {
			flush()
			continue
		}
		word.WriteRune(character)
	}
	flush()
	return words
}

// byteAt answers the byte at index, and 0 past the end of the text.
func byteAt(text string, index int) byte {
	if index < len(text) {
		return text[index]
	}
	return 0
}

// commandExpensive reports whether a text segment runs an expensive command:
// expensiveWords over its program words as the text judgment reads them.
func commandExpensive(segment string) bool {
	return expensiveWords(programWords(segment))
}

// expensiveWords is the cost judgment of program words, the one the text
// judgment applies to a segment and the tree judgment to a parsed command
// (treeLossyPipe): a go test, build, vet or run, the linter, and an le
// command, typed as `le` or as the `ze le` personality, whose line starts
// heavy work and whose own output is the verdict of that work.
//
// An le line is expensive when leroot.HeavyLine answers true and
// leroot.CarrierLine answers false. HeavyLine is read from what each area
// declared where its verb registers: an admitted area (RegisterAdmitted), an
// expensive action (RegisterExpensive), a heavy area or selection
// (RegisterHeavy) and a heavy action row (leaction.Action.Heavy). The
// heavy-work check asks that predicate alone of a subagent's line
// (bashSubagentHeavy), because it asks another question: does the line start
// heavy work. This guard asks whether a filter would truncate the verdict the
// command prints, and a carrier (an action whose row declares an argv
// parameter, as `job run` does, or an area registered with RegisterCarrier,
// as `exec` is) prints the output of the command it was given: that command
// is an argument here, and an argument is no command of the pipeline.
//
// This file therefore names no area and no action: a listing, a help page
// and an action that only reads are not expensive, and an action joins by
// declaring itself. The hook runs inside le, so every registration is linked.
func expensiveWords(tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	word := tokens[0]
	base := filepath.Base(word)
	if base == "go" {
		return len(tokens) > 1 && oneOf(tokens[1], "test", "build", "vet", "run")
	}
	if base == lintCommand {
		return true
	}
	words, typed := heavytool.LeWords(tokens)
	if !typed {
		return false
	}
	line := beforeRedirection(words)
	if leroot.CarrierLine(line) {
		// The command the line carries is its argument, never a command of
		// the pipeline, however heavy the carrier is.
		return false
	}
	return leroot.HeavyLine(line)
}

// programWords answers a segment's words from its program on, the way
// commandExpensive reads a segment: the assignments and launchers in front of
// the program and the options before it are removed. The piped-command
// refusal shows these words, so the command it advises is the one the guard
// judged.
func programWords(segment string) []string {
	return heavytool.Program(shellWords(segment))
}

// beforeRedirection answers the words up to the first redirection. The
// tokenizer keeps `2>&1` and `>` as words of their own. A caller counting what
// follows an area name would therefore read `le test functional 2>&1` as a command
// carrying a verb, and leroot.HeavyLine MUST be given command words only.
func beforeRedirection(words []string) []string {
	for i, word := range words {
		if strings.ContainsAny(word, "<>") {
			return words[:i]
		}
	}
	return words
}

// ze point: commands/no-pipes-on-expensive-commands/never-pipe-an-expensive-command-read-the-log
// ze point: commands/directives/run-commands-through-native-actions-and-never-poll
// bashLossyPipe refuses piping an expensive command through a filter that drops
// output. The truncated text is what the caller would judge the run by, and the
// part a filter removes is usually the part that says why the run failed. An
// le command is expensive when its line starts heavy work and is no carrier
// of another command (expensiveWords): `./le job run ... command go test ./x
// | tail` passes, because the piped command is the admitted form this refusal
// advises and its `go test` is an argument. A
// reader line (ctxReaderLine) passes: no reader is an expensive command, so
// `grep -n 'ok | go test ./x | head'` pipes nothing. On every other line the
// verdict is the text judgment (textLossyPipe), unchanged, OR the tree
// judgment (treeLossyPipe) on a line that parses, which reads each pipeline
// at any depth: the tree only adds refusals.
//
// A subagent's line that the heavy-work check hands back (handsBackHeavy) is
// judged the same and refused the same, without text
// (withoutHandedBackAdvice): the admitted form this refusal advises is one a
// subagent is refused too, and the hand-back carries the next step. A
// subagent's piped command that check does not refuse starts no heavy work
// for that caller (a linter subcommand that runs no analysis), so its text is
// the subagent's own (lossyPipeSubagentAdvice): the command with no filter,
// behind the caller's identity line, and never the admitted form.
func bashLossyPipe(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	if _, outside := ctxReaderLine(ctx); outside == "" {
		return nil
	}
	// The tree goes first so that, where both refuse, the advice is the
	// parsed command and not a text segment cut at a quoted bar (AC-15).
	identity := commandIdentityPrefix(ctx)
	if words, written, filter, found := treeLossyPipe(command, ctx.commandParse()); found {
		return withoutHandedBackAdvice(ctx, lossyPipeRefusal(identity, filter, words, written),
			lossyPipeSubagentAdvice(identity, filter, written))
	}
	filter, segment, found := textLossyPipe(command)
	if !found {
		return nil
	}
	words := beforeRedirection(programWords(segment))
	written := quotedWords(words)
	return withoutHandedBackAdvice(ctx, lossyPipeRefusal(identity, filter, words, written),
		lossyPipeSubagentAdvice(identity, filter, written))
}

// textLossyPipe is the piped-command guard's text judgment, its verdict at
// HEAD for a line that is not a reader line: in each statement of a split at
// `&&`, `||`, `;` and a newline, quoted ones included, the first expensive
// segment before a `|` or `|&`, when a later segment's first field is a lossy
// filter. It answers that filter and that segment.
func textLossyPipe(command string) (filter, segment string, found bool) {
	flat := strings.ReplaceAll(command, "\\\n", " ")
	flat = regexp.MustCompile(`\|[ \t]*\n`).ReplaceAllString(flat, "| ")
	for _, statement := range regexp.MustCompile(`&&|\|\||;|\n`).Split(flat, -1) {
		segments := regexp.MustCompile(`\|&?`).Split(statement, -1)
		if len(segments) < 2 {
			continue
		}
		expensive := ""
		for _, segment := range segments[:len(segments)-1] {
			if expensive == "" && commandExpensive(segment) {
				expensive = segment
			}
		}
		if expensive == "" {
			continue
		}
		for _, later := range segments[1:] {
			fields := strings.Fields(later)
			if len(fields) == 0 || !lossyFilter(fields[0]) {
				continue
			}
			return fields[0], expensive, true
		}
	}
	return "", "", false
}

// commandSegments splits a line into command texts at every separator, inside
// quotes included, which is the text judgment a guard keeps for any line
// that is not a reader line (ctxReaderLine).
func commandSegments(command string) []string {
	parts := segmentSeparator.Split(command, -1)
	out := parts[:0]
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			out = append(out, strings.TrimSpace(part))
		}
	}
	return out
}

// segmentSeparator matches the operators the text judgments cut a line at:
// commandSegments, quote-blind, and unquotedSegments, outside the quoted
// parts of a parsed line. It is compiled once, at package load, because both
// run on every Bash call.
var segmentSeparator = regexp.MustCompile(`&&|\|\||;|\n|\|&?|&`)

// admittedCommand writes the job-admission form of a command a refusal
// advises: `./le job run label <label> quiet command <argv>`. Every refusal
// advises the quiet form: quiet sends the child's output to this session's
// scratch log rather than the transcript (ai/rules/commands.md). argv holds
// the command's words already written back as shell words, one each
// (parsedCommand.written, quotedWords): a value the caller quoted
// (`-run 'TestA|TestB'`) is advised as the same argument, and a word the
// shell expands (`./internal/le/$p`) as typed, so the shell expands it again
// when the advice runs where the command stood.
func admittedCommand(label string, argv []string) string {
	var advice strings.Builder
	advice.WriteString("./le job run label " + label + " quiet")
	advice.WriteString(" command")
	for _, word := range argv {
		advice.WriteByte(' ')
		advice.WriteString(word)
	}
	return advice.String()
}

// quotedWord writes value as one shell word whatever it holds: as it is, or
// single-quoted with each single quote closed, escaped and reopened.
func quotedWord(value string) string {
	if word, written := shellWord(value); written {
		return word
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// ze point: commands/directives/heavy-jobs-are-admitted-by-native-actions-never-typed-raw
// bashRawHeavy refuses a heavy job typed raw, outside job admission. One machine
// carries several sessions, and an unadmitted job oversubscribes it. A reader
// line (ctxReaderLine) runs no heavy job, so a grep pattern naming one
// passes. On every other line the verdict is the text judgment
// (textRawHeavy), unchanged, OR the tree judgment (treeRawHeavy) on a line
// that parses, which reads every command at any depth: the tree only adds
// refusals.
//
// The advice is the main thread's, the caller who schedules heavy work. A
// subagent's line that the heavy-work check hands back (handsBackHeavy) is
// judged the same and refused the same, without text
// (withoutHandedBackAdvice): the admitted form and the stated-reason escape
// this guard advises are both refused to a subagent, and the hand-back
// carries the next step. No text this guard shows a subagent names either.
func bashRawHeavy(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	if _, outside := ctxReaderLine(ctx); outside == "" {
		return nil
	}
	// Where both refuse, the advice is the tree's: the parsed command the
	// shell runs, never a text segment cut at a quoted bar (AC-15). A line
	// that does not parse keeps the text refusal, there being nothing else to
	// read.
	refusal := treeRawHeavy(command, ctx.commandParse(), ctx.root)
	if refusal.what == "" {
		refusal = textRawHeavy(command, ctx.root)
	}
	if refusal.what == "" {
		return nil
	}
	short := ""
	if refusal.short != "" && refusal.vet {
		short = "  -- The short form vets a package with its feature tags,\n" +
			"     and takes no other flag: " + refusal.short + "\n"
	}
	if refusal.short != "" && !refusal.vet {
		short = "  -- The short form tests a package with its feature tags and the race detector,\n" +
			"     and takes no other flag: " + refusal.short + "\n"
	}
	moved := ""
	if refusal.uncarried != "" {
		moved = "  -- Your line changes directory first (" + refusal.uncarried + "), which this form cannot carry:\n" +
			"     run it from the directory that change reaches, or name that directory as\n" +
			"     `env -C <dir>` in front of the program.\n"
	}
	// The Use line is read by the main thread alone, whose identity prefix
	// is empty: a subagent never reads this text (withoutHandedBackAdvice,
	// below). Its line is handed back, or, were the heavy-work check to miss
	// a tool this guard names (heavyTool reads each one, so no such line is
	// known), it reads unhanded, which names neither the admitted form nor
	// the stated-reason escape: a subagent has neither.
	header := "❌ Blocked: " + refusal.what + " run raw, outside job admission (ai/rules/commands.md)\n"
	unhanded := header +
		"  -- A subagent starts no heavy job, admitted or raw: the main thread schedules each one.\n" +
		owedCommandAdvice(ctx)
	return withoutHandedBackAdvice(ctx, &verdict{2, header +
		"  -- One machine, several sessions: registered `./le` actions admit\n" +
		"     their own heavy work. A raw tool reaches the box unadmitted.\n" +
		"  -- Use: " + commandIdentityPrefix(ctx) + refusal.use + "\n" + moved + short +
		"  -- Generic form:\n     ./le job run label <label> quiet command <argv...>\n" +
		"  -- A one-off that must not queue states its reason:\n" +
		"     ZE_ADMIT_RAW=\"bisecting one 2s case\" <command>"}, unhanded)
}

// ze point: commands/directives/run-commands-through-native-actions-and-never-poll
// bashPollLoop refuses an unbounded wait loop. A loop with no timeout holds the
// session until something outside it intervenes.
func bashPollLoop(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	locations := waitKeyword.FindAllStringIndex(command, -1)
	searches := map[string]bool{"grep": true, "egrep": true, "fgrep": true, "rg": true, "ag": true, gitCommand: true, "sed": true, "awk": true, catCommand: true, "echo": true, "printf": true}
	for _, location := range locations {
		head := command[:location[0]]
		cut := 0
		for _, separator := range regexp.MustCompile(`&&|\|\||;|\n`).FindAllStringIndex(head, -1) {
			cut = separator[1]
		}
		prefix := head[cut:]
		fields := strings.Fields(prefix)
		if len(fields) != 0 && searches[filepath.Base(fields[0])] {
			continue
		}
		tail := command[location[0]:]
		body := tail
		if end := strings.Index(body, "done"); end >= 0 {
			body = body[:end]
		}
		if !sleepCall.MatchString(body) && !loopPGrep.MatchString(tail) {
			continue
		}
		if timeoutBound.MatchString(prefix) {
			continue
		}
		return &verdict{2, "❌ Blocked: unbounded wait loop (ai/rules/commands.md)\n" +
			"  -- A command you started with run_in_background notifies you when it\n" +
			"     exits. Waiting for one you launched needs no loop: delete it.\n" +
			"  -- A poll that is the only signal must die on its own. Put the bound\n" +
			"     immediately in front of the loop, in the same statement:\n" +
			"     timeout 300 bash -c 'until [ -f <path> ]; do sleep 30; done'\n" +
			"  -- A repeated event belongs to the Monitor tool. Leave persistent\n" +
			"     false, or its timeout_ms deadline does not apply.\n" +
			"  -- One watcher at a time. Each wake competes with QEMU, Docker and\n" +
			"     every verify run for the same cores."}
	}
	return nil
}

// ze point: testing/temporary-files/use-project-tmp-for-scratch-files
// bashSystemTmp refuses the system temporary directory. Session scratch belongs
// under the per-session directory, which is isolated and reaped with the session.
// Its text judgment (systemTmp) is its verdict, and only a reader line lifts
// it, judged by its words instead (namesSystemTmp), so a grep pattern
// holding a /tmp path passes there.
func bashSystemTmp(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	if command == "" {
		return nil
	}
	if !namesSystemTmp(ctx) {
		return nil
	}
	return &verdict{2, "❌ Blocked: /tmp access is forbidden\nUse this session's own scratch dir instead:\n  " +
		commandIdentityPrefix(ctx) + "dir=$(./le session scratch ensure)   # <session-dir>/scratch/\n  <command> > \"$dir/<name>\""}
}

// ze point: commands/write-ad-hoc-scratch-under-your-per-session-dir/write-ad-hoc-scratch-under-this-session-s-private-directory
// bashScratch refuses ad-hoc scratch written at the tmp/ root. Sessions share
// that tree, so an unqualified name collides with whatever another session wrote.
//
// Its text judgment (scratchWrite) refuses only a write: a `>` or `>>`
// redirection, or a `tee`, into a tmp/ root name. A reader line
// (ctxReaderLine) writes no file, because the predicate admits output only
// onto /dev/null or another descriptor and `tee` is no reader, so such a
// match there is text inside a quoted argument (`grep -rn '> tmp/out.log'
// ai/rules`) and the guard lifts. On every other line a here-document body
// that is only data to the readers of the line (readerData) is blanked out
// of the text first; the line's redirection targets stay, so `cat >
// tmp/out.log <<'EOF'` is still refused.
func bashScratch(ctx context) *verdict {
	if _, outside := ctxReaderLine(ctx); outside == "" {
		return nil
	}
	command := withoutReaderData(stringInput(ctx.input, "command"), ctx.commandParse(), commandIdentityPrefix(ctx))
	seen := map[string]bool{}
	offenders := make([]string, 0, 2)
	for _, match := range scratchWrite.FindAllStringSubmatch(command, -1) {
		path := match[2]
		if !seen[path] && isAdHocScratch(path, ctx.root) {
			seen[path] = true
			offenders = append(offenders, path)
		}
	}
	if len(offenders) == 0 {
		return nil
	}
	return &verdict{2, red + bold + "❌ Refused: the command writes ad-hoc scratch at the tmp/ root: " + strings.Join(offenders, ", ") + reset + "\n" +
		"  -- tmp/ is keyed per CHECKOUT, so that name is one file for every session in this tree, and nothing removes it.\n" +
		"  -- Use: " + commandIdentityPrefix(ctx) + "dir=$(./le session scratch ensure); <command> > \"$dir/out.log\"\n" +
		"  -- A subdirectory passes, and so do the root names that are shared by design: ze-verify*, commit-*, delete-*, mutation*, test-timings*\n" +
		"  -- ai/rules/commands.md, 'Write Ad-Hoc Scratch Under Your Per-Session Dir'"}
}

// testElement is the path element, or the part of one, that marks a test tree.
const testElement = "test"

// ze point: testing/directives/write-the-test-first-and-never-weaken-it
// bashTestDeletion refuses a command that deletes a test. A deleted test is
// indistinguishable from a test that never existed, and its coverage goes with it.
// It fails closed: it refuses when the line runs a delete ANYWHERE and names a
// test path ANYWHERE, without deciding which command deletes which path, because
// a shell has more ways to reach an rm than a hook can parse. The one line it
// reads closer is a line of plain rm commands and readers (plainRmArguments),
// where only the rm arguments are judged, so `rm -f <scratch> && grep x
// internal/a_test.go` passes. Only the path test is exact, so a test path is a
// path the rule guards and not any path spelling "test". The words come from
// the call's one parse (deletionWords); a line that does not parse, and that
// runs rm, is refused.
func bashTestDeletion(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	errors := make([]string, 0, 2)
	// deleted holds each test path an rm reaches, and discarded reports a
	// git checkout that discards test changes: the fix differs for the two.
	deleted := make([]string, 0, 2)
	discarded := false
	words, parsed := deletionWords(command, ctx.commandParse())
	if !parsed && rmWord.MatchString(command) {
		errors = append(errors, "The command does not parse as bash, so what its rm deletes is unknown: "+command)
	}
	if runsDeletion(words) {
		judged := words
		if arguments, plain := plainRmArguments(command, ctx.commandParse()); plain {
			judged = arguments
		}
		for _, word := range judged {
			if testPath(word) {
				errors = append(errors, "Attempting to delete test path "+word+" via: "+command)
				deleted = append(deleted, word)
			}
		}
	}
	if regexp.MustCompile(`git checkout.*(_test\.go|\.ci)`).MatchString(command) && regexp.MustCompile(`git checkout (--|[.])`).MatchString(command) {
		errors = append(errors, "Attempting to discard test file changes: "+command)
		discarded = true
	}
	if len(errors) == 0 {
		return nil
	}
	// The header and the body say the same thing: the agent cannot approve
	// it, and the route is the commit or the main thread (review N1).
	lines := []string{red + bold + "❌ Blocked: Test deletion - the agent cannot approve it" + reset, ""}
	for _, problem := range errors {
		lines = append(lines, "  → "+problem)
	}
	lines = append(lines, "",
		"  -- A deleted test takes its coverage with it, and the agent that deletes it cannot approve that (ai/rules/testing.md).")
	if len(deleted) != 0 || !parsed {
		lines = append(lines,
			"  -- A tracked test that the agreed work removes is deleted by the commit that lands the work. Leave the file in place and name it:")
		if len(deleted) == 0 {
			lines = append(lines, "     ./le commit create ... remove <path>")
		}
		for _, path := range deleted {
			lines = append(lines, "     ./le commit create ... remove "+quotedWord(path))
		}
	}
	if discarded {
		lines = append(lines,
			"  -- Discarding a test's changes reverts work, and git checkout -- is forbidden (AGENTS.md): leave the changes in place.")
	}
	lines = append(lines, "  -- Otherwise do not delete or discard it: report the path and the reason to the main thread.")
	return &verdict{2, strings.Join(lines, "\n")}
}

// governedReason reports whether the command states a reason for reaching a
// governed tree from the shell. The reason must be non-empty after quotes are
// stripped, because an empty one admits every command while recording nothing.
func governedReason(command string) bool {
	_, stated, found := strings.Cut(command, "ZE_ADMIT_GOVERNED_WRITE=")
	if !found {
		return false
	}
	value := strings.Fields(stated)
	return len(value) != 0 && strings.Trim(value[0], "'\"") != ""
}

// governedShellWrite reports whether the command writes into a governed tree by
// any of the six routes: a redirect, an in-place editor, tee, a copy or move,
// a Perl, Ruby, Python, Node, Deno or Bun script that writes a file, or a write
// through a variable, pipe or cd that holds a governed path
// (bash_governed_taint.go).
//
// Each route is its own named test rather than one compound condition, so a
// reader can see which route fired and a new route is a new line.
func governedShellWrite(command string) bool {
	if governedRedirect.MatchString(command) {
		return true
	}
	if governedSed.MatchString(command) {
		return true
	}
	if governedTee.MatchString(command) {
		return true
	}
	if governedCopy.MatchString(command) {
		return true
	}
	if governedTaintWrite(command) {
		return true
	}
	return governedRuntime.MatchString(command) &&
		governedPath.MatchString(command) &&
		governedWrite.MatchString(command)
}

// ze point: commands/directives/bash-must-not-edit-a-governed-document
//
// bashGovernedWrite refuses a shell write into plan/ or ai/rules/. Those trees
// are guarded by the Write/Edit hook, and a shell write runs none of its checks.
func bashGovernedWrite(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	if !governedShellWrite(withoutNestedTreeNames(rootRelative(ctx.root, command))) {
		return nil
	}
	if governedReason(command) {
		return nil
	}
	return &verdict{2, strings.Join([]string{
		red + bold + "❌ Blocked: shell write to plan/ or ai/rules/" + reset,
		"", "  The native Write/Edit hook guards these trees, and",
		"  settings.json wires it to Write|Edit|MultiEdit|NotebookEdit only.",
		"  A shell write runs none of its checks.", "",
		"  → Edit the file with the Edit tool. Prefer Edit over Write:",
		"    a Write over an existing rule point is refused separately.",
		"  → Reading is untouched: grep, cat and `sed -n` stay free.",
		"  → A payload that only READS these trees and writes elsewhere",
		"    is refused too; that is the cost of catching a path built",
		"    from a variable. State the reason and it lands:",
		"      ZE_ADMIT_GOVERNED_WRITE=\"reads plan/, writes scratch\" <command>",
	}, "\n")}
}

func scratchMessage(path string) string {
	return fmt.Sprintf("%s%s❌ Refused: ad-hoc scratch at the tmp/ root: %s%s", red, bold, path, reset)
}

// ze point: principles/directives/a-wrong-value-must-not-look-like-a-right-one
// preMaterializeDerived rebuilds every registered artifact the command names
// and the tree does not hold.
//
// This is the read half of the derived-artifact loop. postInvalidateDerived
// removes an artifact whenever one of its inputs is written, so an artifact is
// absent for most of a session, and a grep would otherwise read nothing. The
// rebuild runs BEFORE the command, so what the command reads describes the tree
// as it now stands rather than as it stood at the last commit.
//
// A WHOLE artifact is left exactly as it is. A read that rewrote it would
// mutate the tree on every grep, and its content is already correct: the write
// hook removed it the moment it stopped being correct. Whole is not the same
// question as present for a directory artifact, which answers it itself
// (derived.Artifact.Whole, derived.Artifact.Complete).
//
// The build runs through derived.Artifact.Ensure, under the checkout's
// derived-artifact lock: another session's invalidation waits for it rather
// than removing a directory the build is writing into, and a reader that waited
// for a peer's build of the same artifact reuses it. The wait for the lock is
// bounded by materializeLockWait, inside this hook's harness timeout: a lock
// still held at that bound refuses the command and tells the reader to run it
// again, rather than letting the harness kill the hook in silence.
//
// A rebuild that fails BLOCKS the command. The alternative is a grep reading a
// file nobody built, which answers "no match" for a tree the author cannot see,
// and that answer is indistinguishable from a real absence. A lock that cannot
// be taken blocks it for the same reason, with its own fix.
func preMaterializeDerived(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	if command == "" {
		return nil
	}
	return materializeNamed(ctx.root, command, derived.All(), materializeLockWait)
}

// The harness kills a hook at the timeout `.claude/settings.json` gives its
// command: 30 s for `le ai hooks pretool-bash` and 90 s for `le ai hooks
// posttool-writeedit`. These two constants are the only copy here, and
// TestDerivedLockWaitsStayInsideTheHookTimeouts reads the settings file and
// refuses a drift between them.
const (
	pretoolBashTimeout       = 30 * time.Second
	posttoolWriteEditTimeout = 90 * time.Second
)

// Each hook waits for the derived-artifact lock for half its own timeout, so
// its other checks and the build or the removal keep the other half. A hook
// the harness kills while it waits reports nothing, so the wait MUST end
// first and say so: a reader is told to run the command again, and an
// invalidation reports the artifact stale and not removed.
const (
	materializeLockWait = pretoolBashTimeout / 2
	invalidateLockWait  = posttoolWriteEditTimeout / 2
)

// materializeNamed is preMaterializeDerived over a given set of artifacts and
// lock wait, so a test can drive the decision over an artifact whose rebuild
// it controls and over a lock it holds.
func materializeNamed(root, command string, artifacts []derived.Artifact, wait time.Duration) *verdict {
	for _, artifact := range artifacts {
		if !commandNamesArtifact(command, artifact.Path) {
			continue
		}
		// A tree that does not hold the artifact's DIRECTORY does not hold the
		// artifact, so there is no index here to be stale and nothing to build.
		// Without this, every command naming `ai/` in a scratch checkout a test
		// or a fixture created was refused for a tree the artifact never lived
		// in. The grep itself reports a missing path loudly, so the silent
		// wrong answer this check exists to prevent cannot arise.
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(pathDirectory(artifact.Path)))); err != nil {
			continue
		}
		whole, err := artifact.Whole(root)
		if err != nil {
			return &verdict{2, "❌ Blocked: " + artifact.Path + " cannot be read: " + err.Error() +
				"\nIt is derived, so the command would judge a tree nobody rendered."}
		}
		if whole {
			continue
		}
		if err := artifact.Ensure(root, time.Now().Add(wait)); err != nil {
			return materializeRefusal(artifact.Path, err)
		}
	}
	return nil
}

// materializeRefusal is the verdict for an artifact Ensure could not build:
// the lock's own refusal when the lock was the cause, the build's otherwise.
func materializeRefusal(path string, err error) *verdict {
	if lockErr, ok := errors.AsType[*derived.LockError](err); ok {
		if lockErr.Cause == derived.LockCauseBusy {
			return &verdict{2, "❌ Blocked: " + path + " is derived, and another session is rebuilding it now." +
				"\nThe command would read it before that rebuild finished. Run the command again."}
		}
		// lockErr.Error() carries the fix on its last line, so it comes last
		// and nothing is printed after it.
		return &verdict{2, "❌ Blocked: " + path + " is derived and was not built." +
			"\nWithout the lock, another session's edit can remove the artifact while it is built." +
			"\nCause: " + lockErr.Error()}
	}
	return &verdict{2, "❌ Blocked: " + path + " is derived and could not be built: " + err.Error() +
		"\nRunning the command now would read an absent file as an empty answer."}
}

// ze point: go-standards/directives/read-the-ze-style-guide-before-the-first-go-edit
// ze point: architecture/directives/read-the-design-doors-before-the-first-code-edit
// bashDocumentRead records a Bash print of a whole gated document (the Go style
// guide, or a page under ai/doors), which writeStyleGuideRead,
// writeDesignDoorsRead and writeSpecDoorsRead ask for before an edit. It never
// refuses. It credits only a plain read line (plainDocumentReads), whose every
// command the hook can place without a shell parser, and records nothing for
// any other line, absolute operands included: a quote, an expansion, a comment,
// a heredoc or a redirection can hide a cd or turn a print into text that
// never runs, and a list of such shapes is never complete. Each printed file is
// resolved as the kernel opens it, and the record names the checkout that copy
// lies in (readDocument), so a cat of a worktree's copy counts for that
// worktree's edits alone. writeStyleGuideRead and doorRefusal describe the
// same line to the reader, so a change here changes them.
func bashDocumentRead(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	if !strings.Contains(command, styleGuidePath) && !strings.Contains(command, doorsDir+"/") {
		return nil
	}
	paths, plain := plainDocumentReads(ctx, command)
	if !plain {
		return nil
	}
	for _, path := range paths {
		if checkout, document, ok := readDocument(ctx, path); ok {
			writeMarkerFile(documentReadMarker(ctx, checkout, document), "")
		}
	}
	return nil
}

// plainDocumentReads answers the files a plain read line prints whole, each
// resolved by plainReadPhysical, and whether command is such a line. A plain
// read line holds only plainReadCharacter characters and no `&` outside `&&`,
// and is a sequence of commands joined by `&&` or `;`: an optional `cd <path>`
// first, then only `cat [-n] <path>...` or `nl <path>...`. The cd moves the
// line where cdTarget can tell, and a cd it cannot place makes the line not
// plain. A relative operand is joined to the physical directory (the cd's, or
// the session's cwd) and walked whole, so a link is followed before the `..`
// after it, as the kernel does; an absolute operand is walked the same way.
//
// Every operand MUST be a regular file the hook can open (plainReadFile), or
// the whole line is not plain. One that fails makes its cat fail, and `&&`
// then skips every later command, so crediting the later operands would record
// a read that never ran: `cat /nonexistent && cat ai/doors/system.md` never
// prints system.md. The same holds for a directory and for a file the hook
// cannot read. A start directory, a cd target or an operand the walk refuses
// (a path under /proc or /dev among them) makes the line not plain too.
//
// A line whose prints together exceed plainReadPrintMax bytes is not plain
// either: the harness cuts a Bash output it judges too long, so the reader saw
// a cut copy and no file of the line was read whole.
func plainDocumentReads(ctx context, command string) ([]string, bool) {
	if !plainReadText(command) {
		return nil, false
	}
	logical := worktreeLineStart(ctx.root, ctx.payload.Cwd)
	physical, walked := plainReadPhysical(logical)
	if !walked {
		return nil, false
	}
	var paths []string
	printedSize := 0
	for index, segment := range strings.Split(strings.ReplaceAll(command, "&&", ";"), ";") {
		words := strings.Fields(segment)
		if len(words) == 0 {
			return nil, false
		}
		if index == 0 && words[0] == "cd" {
			if len(words) != 2 {
				return nil, false
			}
			target, _ := cdTarget(logical, words[1:], []bool{false})
			if target == "" {
				return nil, false
			}
			physical, walked = plainReadPhysical(target)
			if !walked {
				return nil, false
			}
			continue
		}
		operands, printed := plainPrintOperands(words)
		if !printed {
			return nil, false
		}
		numbered := plainPrintNumbers(words)
		for _, operand := range operands {
			path := operand
			if !filepath.IsAbs(path) {
				path = physical + string(filepath.Separator) + operand
			}
			resolved, size, readable := plainReadFile(path, numbered)
			if !readable {
				return nil, false
			}
			printedSize += size
			if printedSize > plainReadPrintMax {
				return nil, false
			}
			paths = append(paths, resolved)
		}
	}
	return paths, true
}

// plainReadPrintMax bounds the bytes one plain read line may print and still
// be credited. The harness cuts a Bash output past 30000 characters
// (BASH_MAX_OUTPUT_LENGTH, by default), and a character is at least one byte,
// so a print of at most this many bytes reaches the reader whole, with room
// to spare.
const plainReadPrintMax = 25000

// plainReadNumberWidth is the bytes `cat -n` and `nl` put before each line:
// a number right-aligned in six columns, then a tab (`%6d\t`). nl puts the
// same width of blanks before a line it does not number. A file within
// plainReadPrintMax has fewer than a million lines, so the number never
// widens the prefix.
const plainReadNumberWidth = 7

// plainReadFile answers the physical path of the absolute path name
// (plainReadPhysical), the bytes a print of it writes, each line prefixed by
// plainReadNumberWidth bytes when numbered, and whether it is a regular file
// the hook can open, which is what a cat of it needs to print it. A file
// larger than plainReadPrintMax answers false without being read, since no
// line that prints it can be credited.
func plainReadFile(name string, numbered bool) (string, int, bool) {
	resolved, walked := plainReadPhysical(name)
	if !walked {
		return "", 0, false
	}
	info, err := os.Lstat(resolved)
	if err != nil {
		return "", 0, false
	}
	if !info.Mode().IsRegular() {
		return "", 0, false
	}
	if info.Size() > plainReadPrintMax {
		return "", 0, false
	}
	file, err := os.Open(resolved) // #nosec G304 -- opening the operand the shell would print is the check
	if err != nil {
		return "", 0, false
	}
	body, readErr := io.ReadAll(io.LimitReader(file, plainReadPrintMax+1))
	if err := file.Close(); err != nil {
		return "", 0, false
	}
	if readErr != nil {
		return "", 0, false
	}
	size := len(body)
	if numbered {
		size += plainReadNumberWidth * textLines(body)
	}
	return resolved, size, true
}

// textLines counts the lines of body as cat and nl print them: one per
// newline, and one more for a last line with no newline.
func textLines(body []byte) int {
	lines := bytes.Count(body, []byte{'\n'})
	if len(body) != 0 && body[len(body)-1] != '\n' {
		lines++
	}
	return lines
}

// plainPrintNumbers reports whether a command plainPrintOperands accepted
// prefixes each line with its number: `cat -n` and `nl` do.
func plainPrintNumbers(words []string) bool {
	if words[0] == "nl" {
		return true
	}
	return len(words) > 1 && words[1] == "-n"
}

// plainReadLinksMax bounds the links one walk of plainReadPhysical follows,
// as Linux bounds a path lookup at 40, so a loop of links ends the walk.
const plainReadLinksMax = 40

// plainReadPhysical answers the physical path of the absolute path name,
// walked one component at a time as the kernel opens it: a link is read with
// os.Readlink and its target walked in its place, so a `..` after a link
// climbs from the link's target. It answers false when a component cannot be
// read, when a component that is not a directory has more after it, when the
// walk follows more than plainReadLinksMax links, and when any path the walk
// reaches is in a process-relative tree (processRelativeTree), whether the
// name or a link target on the way leads there. filepath.EvalSymlinks is not
// used because it follows such links in the hook's own process. The loop is
// bounded by the components of name and of at most plainReadLinksMax targets.
func plainReadPhysical(name string) (string, bool) {
	if !filepath.IsAbs(name) {
		return "", false
	}
	resolved := string(filepath.Separator)
	pending := strings.Split(name, "/")
	links := 0
	for len(pending) != 0 {
		part := pending[0]
		pending = pending[1:]
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			resolved = filepath.Dir(resolved)
			continue
		}
		next := filepath.Join(resolved, part)
		if processRelativeTree(next) {
			return "", false
		}
		info, err := os.Lstat(next)
		if err != nil {
			return "", false
		}
		if info.Mode()&os.ModeSymlink == 0 {
			if !info.IsDir() && len(pending) != 0 {
				return "", false
			}
			resolved = next
			continue
		}
		links++
		if links > plainReadLinksMax {
			return "", false
		}
		target, err := os.Readlink(next)
		if err != nil {
			return "", false
		}
		if filepath.IsAbs(target) {
			resolved = string(filepath.Separator)
		}
		pending = append(strings.Split(target, "/"), pending...)
	}
	return resolved, true
}

// processRelativeTree reports whether path is /proc or /dev or lies under one.
// The kernel answers their entries for the process that looks, so
// /proc/self/cwd and /dev/fd reach one place for the hook and another for the
// shell that runs the line, and the hook cannot say which file a print reads.
func processRelativeTree(path string) bool {
	for _, tree := range []string{"/proc", "/dev"} {
		if path == tree {
			return true
		}
		if strings.HasPrefix(path, tree+"/") {
			return true
		}
	}
	return false
}

// plainReadText reports whether every character of command is a
// plainReadCharacter, and every `&` is half of `&&`.
func plainReadText(command string) bool {
	for _, character := range command {
		if !plainReadCharacter(character) {
			return false
		}
	}
	return !strings.Contains(strings.ReplaceAll(command, "&&", ""), "&")
}

// plainReadCharacter reports whether character may appear on a plain read
// line: a letter or digit, a character of a plain path (`/ . - _ + , @ % :`),
// a space or a tab, and the separators `;` and `&`. Every character the shell
// gives a meaning to beyond that (`$`, a quote, a backslash, a backtick, `#`,
// `<`, `>`, `|`, `(`, `)`, `{`, `}`, `*`, `?`, `[`, `~`, `=`, `!`, a newline)
// makes the line one the hook does not place.
func plainReadCharacter(character rune) bool {
	if 'a' <= character && character <= 'z' {
		return true
	}
	if 'A' <= character && character <= 'Z' {
		return true
	}
	if '0' <= character && character <= '9' {
		return true
	}
	return strings.ContainsRune("/.-_+,@%: \t;&", character)
}

// plainPrintOperands answers the operands of one command of a plain read line
// that prints every line of each, and whether the command is one: `cat`,
// optionally with `-n`, or `nl`, followed by at least one operand and no other
// option.
func plainPrintOperands(words []string) ([]string, bool) {
	operands := words[1:]
	switch words[0] {
	case catCommand:
		if len(operands) != 0 && operands[0] == "-n" {
			operands = operands[1:]
		}
	case "nl":
	default:
		return nil, false
	}
	if len(operands) == 0 {
		return nil, false
	}
	for _, operand := range operands {
		if strings.HasPrefix(operand, "-") {
			return nil, false
		}
	}
	return operands, true
}

// commandNamesArtifact reports whether the command text reaches the artifact,
// by its own path or by a directory that holds it with a trailing slash.
//
// The directory half is what a recursive search needs. `grep -rn X ai/` names
// every artifact under ai/ without spelling one, so matching the full path
// alone let that command walk a directory the artifacts were missing from and
// answer "no match" for a tree the author could not see.
//
// WHAT THIS DOES NOT CATCH, stated because a reader will meet it. A search that
// names neither the path nor a slashed directory holding it is not matched, and
// after an edit removed the artifact such a search reads a tree without it:
//
//	rg ResolveBGPTree                 -- names no path at all
//	grep -rn foo .                    -- names the checkout, not the directory
//	grep -rn X ai                     -- the same files, without the slash
//	./le <action>                     -- reads an artifact inside its own process
//
// Widening it to catch them has no bound: every search over the checkout would
// pay a rebuild, and the bare segment `ai` is two letters that appear inside
// ordinary words. The trailing slash is how a directory is written when it is
// MEANT as a path, and that is where the line is drawn.
// docs/contributing/navigating-the-code.md tells the reader to name the path
// when the answer matters.
func commandNamesArtifact(command, path string) bool {
	if strings.Contains(command, path) {
		return true
	}
	for directory := pathDirectory(path); directory != ""; directory = pathDirectory(directory) {
		if strings.Contains(command, directory+"/") {
			return true
		}
	}
	return false
}

// pathDirectory answers the parent of a slash-separated repository path, and ""
// when the path has no parent left. filepath.Dir is not used because it answers
// "." at the top, which is a directory this search must not match.
func pathDirectory(path string) string {
	cut := strings.LastIndex(path, "/")
	if cut <= 0 {
		return ""
	}
	return path[:cut]
}
