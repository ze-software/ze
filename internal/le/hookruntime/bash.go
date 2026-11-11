// Design: docs/architecture/core-design.md -- native Bash hook policy
package hookruntime

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/derived"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// catCommand is the command word `cat`, which several checks name: a lossy
// filter, a search command, a whole-document read and a reader command.
const catCommand = "cat"

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

// ze point: none -- the worktree prohibition lives in ai/INSTRUCTIONS.md, outside the rule corpus
// bashWorktreeCopy refuses a file copy out of a worktree into the main tree.
// A worktree agent lands its work by committing, and a direct copy overwrites
// whatever another session left uncommitted at the destination.
func bashWorktreeCopy(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	if command == "" || !strings.Contains(command, ".claude/worktrees") {
		return nil
	}
	blocked := false
	for _, token := range []string{"cp ", "cp -", "mv ", "mv -", "rsync ", "install "} {
		blocked = blocked || strings.Contains(command, token)
	}
	blocked = blocked || regexp.MustCompile(`\s>`).MatchString(command)
	if !blocked {
		return nil
	}
	return &verdict{2, "❌ Blocked: copying files from worktree to main repo\nWorktree agents must commit their changes. Use git merge or cherry-pick.\nDirect file copying overwrites uncommitted work from other sessions."}
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
func gitVerbRun(command, verb string) bool {
	prose := !shellInvoker.MatchString(command)
	for offset := 0; ; {
		at := strings.Index(command[offset:], verb)
		if at < 0 {
			return false
		}
		at += offset
		offset = at + len(verb)
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
func bashRootBuild(ctx context) *verdict {
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
// command, typed as `le` or as the `ze le` personality, whose area or action
// runs something (heavyArea).
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
	if base == "le" {
		return heavyArea(tokens[1:])
	}
	if strings.HasPrefix(base, "ze") && len(tokens) > 2 && tokens[1] == "le" {
		return heavyArea(tokens[2:])
	}
	return false
}

// programWords answers a segment's words from its program on, the way
// commandExpensive reads a segment: the assignments and launchers in front of
// the program and the options before it are removed. The piped-command
// refusal shows these words, so the command it advises is the one the guard
// judged.
func programWords(segment string) []string {
	return programTokens(shellWords(segment))
}

// programTokens answers words from their program on: programWords over
// words already read, a text segment's or a parsed command's tokens
// (parsedCommand.tokens). It returns a subslice of tokens.
func programTokens(tokens []string) []string {
	for len(tokens) != 0 {
		head := tokens[0]
		if strings.Contains(strings.Split(head, "/")[0], "=") && !strings.HasPrefix(head, "-") {
			tokens = tokens[1:]
			continue
		}
		if _, launcher := launcherValueOptions[head]; !launcher {
			break
		}
		tokens = tokens[1+launcherArguments(head, tokens[1:]):]
	}
	for len(tokens) != 0 && strings.HasPrefix(tokens[0], "-") {
		tokens = tokens[1:]
	}
	return tokens
}

// beforeRedirection answers the words up to the first redirection. The
// tokenizer keeps `2>&1` and `>` as words of their own. A caller counting what
// follows an area name would therefore read `le test functional 2>&1` as a command
// carrying a verb. It would then refuse a listing as the 24-suite run.
func beforeRedirection(words []string) []string {
	for i, word := range words {
		if strings.ContainsAny(word, "<>") {
			return words[:i]
		}
	}
	return words
}

// heavyArea answers whether the le area the words name runs tests, a build, or
// the verification gate. An area name is two words since the command grouping,
// so the two-word name is read before the one-word name: `le verify status` and
// `le verify summary` read and write the verification certificate and run
// nothing, while every other area under `verify` runs the gate.
//
// The suites stopped running on their bare name, so `le test functional` and
// `le test unit` print a listing and the run needs `gating` or `all`. `le
// verify` still runs the gate on its bare name, which is why the exemption
// names the two areas rather than the shape.
//
// A suite area is a member of `test`, so the word after `test` names the
// area: `le test unit all` is the area `test unit`. No
// harness binary exists (spec-le-subject-first-command-tree, D-8), so
// no file name is read as the functional runner. A harness command under
// `test` is heavy when its registration says its run is admitted
// (leroot.Admits): `le test bgp` and `le test ospf` run the functional runner,
// while `le test peer` and `le mock rpki` are helper tools a test starts
// (AC-45). The hook runs inside le, so every registration is linked.
//
// `le go lint` runs the linter and is heavy. The retired top-level suite
// names answer `unknown command` since Phase 3 of that spec, so a suite is
// heavy only under `test`.
func heavyArea(words []string) bool {
	words = beforeRedirection(words)
	if len(words) == 0 {
		return false
	}
	if expensiveAction(words) {
		return true
	}
	area, rest := words[0], words[1:]
	if area == "go" {
		return len(rest) > 0 && rest[0] == "lint"
	}
	if area == verifyArea {
		if len(rest) > 0 && oneOf(rest[0], "status", "summary") {
			return false
		}
		return true
	}
	if area != "test" || len(rest) == 0 {
		return false
	}
	if leroot.Admits("test " + rest[0]) {
		return true
	}
	suite, verbs := rest[0], rest[1:]
	if len(verbs) == 0 && oneOf(suite, "functional", "unit") {
		return false
	}
	return oneOf(suite, "functional", "integration", "qemu", "unit")
}

// expensiveNameWordsMax bounds the words expensiveAction joins into a
// command name: an le name is its area's words and a verb, and no area is
// deeper than three words.
const expensiveNameWordsMax = 4

// expensiveAction reports whether words open with the whole name of an le
// action that registered its run as expensive (leroot.RegisterExpensive):
// `ai hooks unit` runs every hook golden row and behavioral fixture in
// process and takes no job slot, so no area rule reads it as heavy, while
// the lines a filter drops from it are the ones that say which row failed.
// The registration is the declaration, so an action joins by registering
// and this function names none.
func expensiveAction(words []string) bool {
	for count := 1; count <= min(len(words), expensiveNameWordsMax); count++ {
		if leroot.Expensive(strings.Join(words[:count], " ")) {
			return true
		}
	}
	return false
}

// ze point: commands/no-pipes-on-expensive-commands/never-pipe-an-expensive-command-read-the-log
// ze point: commands/directives/run-commands-through-native-actions-and-never-poll
// bashLossyPipe refuses piping an expensive command through a filter that drops
// output. The truncated text is what the caller would judge the run by, and the
// part a filter removes is usually the part that says why the run failed. A
// reader line (ctxReaderLine) passes: no reader is an expensive command, so
// `grep -n 'ok | go test ./x | head'` pipes nothing. On every other line the
// verdict is the text judgment (textLossyPipe), unchanged, OR the tree
// judgment (treeLossyPipe) on a line that parses, which reads each pipeline
// at any depth: the tree only adds refusals.
func bashLossyPipe(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	if _, outside := ctxReaderLine(ctx); outside == "" {
		return nil
	}
	// The tree goes first so that, where both refuse, the advice is the
	// parsed command and not a text segment cut at a quoted bar (AC-15).
	if words, filter, found := treeLossyPipe(command, ctx.commandParse()); found {
		return lossyPipeRefusal(commandIdentityPrefix(ctx), filter, words)
	}
	filter, segment, found := textLossyPipe(command)
	if !found {
		return nil
	}
	return lossyPipeRefusal(commandIdentityPrefix(ctx), filter, beforeRedirection(programWords(segment)))
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
	parts := regexp.MustCompile(`&&|\|\||;|\n|\|&?|&`).Split(command, -1)
	out := parts[:0]
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			out = append(out, strings.TrimSpace(part))
		}
	}
	return out
}

// admittedCommand writes the job-admission form of a command a refusal
// advises: `./le job run label <label> quiet command <argv>`. Every refusal
// advises the quiet form: quiet sends the child's output to this session's
// scratch log rather than the transcript (ai/rules/commands.md).
func admittedCommand(label string, tokens []string) string {
	var advice strings.Builder
	advice.WriteString("./le job run label " + label + " quiet")
	advice.WriteString(" command")
	// Each word is written back as one shell word, so a value the caller
	// quoted (`-run 'TestA|TestB'`) is advised as the same argument.
	for _, token := range tokens {
		advice.WriteByte(' ')
		advice.WriteString(quotedWord(token))
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
	// The Use line starts with the caller's identity line, so a subagent
	// following it is not refused by bashAgentIdentity (review N7).
	return &verdict{2, "❌ Blocked: " + refusal.what + " run raw, outside job admission (ai/rules/commands.md)\n" +
		"  -- One machine, several sessions: registered `./le` actions admit\n" +
		"     their own heavy work. A raw tool reaches the box unadmitted.\n" +
		"  -- Use: " + commandIdentityPrefix(ctx) + refusal.use + "\n" + moved + short +
		"  -- Generic form:\n     ./le job run label <label> quiet command <argv...>\n" +
		"  -- A one-off that must not queue states its reason:\n" +
		"     ZE_ADMIT_RAW=\"bisecting one 2s case\" <command>"}
}

// ze point: commands/directives/run-commands-through-native-actions-and-never-poll
// bashPollLoop refuses an unbounded wait loop. A loop with no timeout holds the
// session until something outside it intervenes.
func bashPollLoop(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	locations := waitKeyword.FindAllStringIndex(command, -1)
	searches := map[string]bool{"grep": true, "egrep": true, "fgrep": true, "rg": true, "ag": true, "git": true, "sed": true, "awk": true, catCommand: true, "echo": true, "printf": true}
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
	if !governedShellWrite(command) {
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
// A rebuild that fails BLOCKS the command. The alternative is a grep reading a
// file nobody built, which answers "no match" for a tree the author cannot see,
// and that answer is indistinguishable from a real absence.
func preMaterializeDerived(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	if command == "" {
		return nil
	}

	for _, artifact := range derived.All() {
		if !commandNamesArtifact(command, artifact.Path) {
			continue
		}
		// A tree that does not hold the artifact's DIRECTORY does not hold the
		// artifact, so there is no index here to be stale and nothing to build.
		// Without this, every command naming `ai/` in a scratch checkout a test
		// or a fixture created was refused for a tree the artifact never lived
		// in. The grep itself reports a missing path loudly, so the silent
		// wrong answer this check exists to prevent cannot arise.
		if _, err := os.Stat(filepath.Join(ctx.root, filepath.FromSlash(pathDirectory(artifact.Path)))); err != nil {
			continue
		}
		whole, err := artifact.Whole(ctx.root)
		if err != nil {
			return &verdict{2, "❌ Blocked: " + artifact.Path + " cannot be read: " + err.Error() +
				"\nIt is derived, so the command would judge a tree nobody rendered."}
		}
		if whole {
			continue
		}
		if err := artifact.Rebuild(ctx.root); err != nil {
			return &verdict{2, "❌ Blocked: " + artifact.Path + " is derived and could not be built: " + err.Error() +
				"\nRunning the command now would read an absent file as an empty answer."}
		}
	}
	return nil
}

// ze point: go-standards/directives/read-the-ze-style-guide-before-the-first-go-edit
// ze point: architecture/directives/read-the-design-doors-before-the-first-code-edit
// bashDocumentRead records a Bash print of a whole gated document (the Go style
// guide, or a page under ai/doors), which writeStyleGuideRead and
// writeDesignDoorsRead ask for before a code edit. It never refuses. A command
// with a pipe, a redirection or a substitution records nothing, because the
// output then goes somewhere other than the reader. It runs before the
// command, so a command that then fails still leaves the record.
func bashDocumentRead(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	if !strings.Contains(command, styleGuidePath) && !strings.Contains(command, doorsDir+"/") {
		return nil
	}
	for _, diverts := range []string{"|", ">", "<(", "$("} {
		if strings.Contains(command, diverts) {
			return nil
		}
	}
	for _, segment := range commandSegments(command) {
		for _, document := range printedDocuments(shellWords(segment)) {
			writeMarkerFile(documentReadMarker(ctx, document), "")
		}
	}
	return nil
}

// printedDocuments answers the gated documents one command prints whole: a
// program that prints every line of a file, with those documents among its
// operands. head, tail and sed print a range, so they are no read, and neither
// is a bat with a line range or a less or more told to start at a line with
// `+N`. A glob is not expanded here, so `cat ai/doors/*.md` names no document.
func printedDocuments(words []string) []string {
	if len(words) == 0 {
		return nil
	}
	program := filepath.Base(words[0])
	ranged := program == "bat"
	switch program {
	case catCommand, "bat", "nl", "less", "more":
	default:
		return nil
	}
	var documents []string
	for _, word := range words[1:] {
		if strings.HasPrefix(word, "+") {
			return nil
		}
		if ranged && strings.HasPrefix(word, "-r") {
			return nil
		}
		if ranged && strings.HasPrefix(word, "--line-range") {
			return nil
		}
		if document := gatedDocument(word); document != "" {
			documents = append(documents, document)
		}
	}
	return documents
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
