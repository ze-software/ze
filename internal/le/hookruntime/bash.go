// Design: docs/architecture/core-design.md -- native Bash hook policy
package hookruntime

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var (
	goBuildCall      = regexp.MustCompile(`(^|[^A-Za-z0-9_])go\s+build([^A-Za-z0-9_]|$)`)
	goBuildBin       = regexp.MustCompile(`-o\s+bin/`)
	goBuildSession   = regexp.MustCompile(`-o\s+tmp/session/[0-9]{4}-[0-9]{2}-[0-9]{2}-[A-Za-z0-9._-]+/bin/`)
	goBuildAll       = regexp.MustCompile(`go\s+build\s+(-[A-Za-z0-9_]+\s+)*\./\.{3}`)
	systemTmp        = regexp.MustCompile(`(^|[\s='"$(` + "`" + `:,])/tmp(/|\s|$)`)
	scratchWrite     = regexp.MustCompile(`(?:>>?\s*|\btee\s+(?:-a\s+)?)(["']?)([A-Za-z0-9_./-]*tmp/[^\s;|&)'\"]+)`)
	waitKeyword      = regexp.MustCompile(`\b(while|until)\b`)
	sleepCall        = regexp.MustCompile(`(?:^|[;&|(\s./])sleep\s*[0-9$"'(]`)
	loopPGrep        = regexp.MustCompile(`\b(while|until)\s+(?:!\s*)?(?:\[\[?\s*)?pgrep\b`)
	timeoutBound     = regexp.MustCompile(`(^|[^-A-Za-z0-9_])timeout\s+(?:-\S+\s+)*[0-9]+(?:\.[0-9]+)?[smhd]?\b`)
	rootTestPath     = regexp.MustCompile(`[^\s'\"]*(?:test/|_test\.go)[^\s'\"]*`)
	governedPath     = regexp.MustCompile(`(?:plan/|ai/rules/)`)
	governedRedirect = regexp.MustCompile(`>>?[ \t]*["']?(?:plan/|ai/rules/)`)
	// The in-place flag is matched with an OPTIONAL argument prefix and inside a
	// CLUSTER of short flags, because both halves were holes. It can be the first
	// argument, `sed -i 's/a/b/' plan/x`, which a mandatory intervening argument
	// made unreachable. It can also be bundled with its neighbors: `perl -0pi`,
	// `perl -pi`, `sed -Ei`, `sed -i.bak` and `sed --in-place` all edit in place,
	// and a standalone `-i` saw none of them.
	governedSed     = regexp.MustCompile(`(?:^|[\s;&|(` + "`" + `])(?:sed|perl|ruby)[ \t]+(?:[^|;&\n]*[ \t])?-[-A-Za-z0-9]*i[^|;&\n]*(?:plan/|ai/rules/)`)
	governedTee     = regexp.MustCompile(`(?:^|[\s;&|(` + "`" + `])tee[ \t]+(?:-a[ \t]+)?["']?(?:plan/|ai/rules/)`)
	governedCopy    = regexp.MustCompile(`(?:^|[\s;&|(` + "`" + `])(?:mv|cp)[ \t]+[^|;&\n]*[ \t]["']?(?:plan/|ai/rules/)`)
	governedRuntime = regexp.MustCompile(`\b(?:perl|ruby)\b`)
	governedWrite   = regexp.MustCompile(`(?:open\([^)]*["'][wa]|write_text\(|\.write\(|writelines\(|truncate\()`)
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
				"script that stages, commits and checks the index for another session's paths.\n" +
				"To delete a tracked file, use plain `rm` and pass the path to `remove`."}
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
// bashRootBuild refuses a Go compile that names no output path. Without -o the
// binary lands in the working directory under the package name, where the next
// tracked-file check reports it as an untracked artifact.
func bashRootBuild(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	if !goBuildCall.MatchString(command) || goBuildBin.MatchString(command) ||
		goBuildSession.MatchString(command) || strings.Contains(command, "go build ./...") ||
		goBuildAll.MatchString(command) {
		return nil
	}
	return &verdict{2, red + bold + "✘ BLOCKED: go build without -o bin/" + reset + "\n\n" +
		"  " + red + "→" + reset + " Use: go build -o bin/<name> ./cmd/<name>"}
}

func shellWords(text string) []string {
	words := make([]string, 0, 16)
	var word strings.Builder
	quote := rune(0)
	escaped := false
	flush := func() {
		if word.Len() != 0 {
			words = append(words, word.String())
			word.Reset()
		}
	}
	for _, character := range text {
		if escaped {
			word.WriteRune(character)
			escaped = false
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

func commandExpensive(segment string) bool {
	tokens := shellWords(segment)
	launchers := map[string]bool{"bash": true, "sh": true, "perl": true, "ruby": true, "sudo": true, "time": true, "nice": true, "env": true, "timeout": true}
	for len(tokens) != 0 {
		head := tokens[0]
		if strings.Contains(strings.Split(head, "/")[0], "=") && !strings.HasPrefix(head, "-") {
			tokens = tokens[1:]
			continue
		}
		if !launchers[head] {
			break
		}
		tokens = tokens[1:]
		if head == "timeout" || head == "nice" {
			for len(tokens) != 0 && (strings.HasPrefix(tokens[0], "-") || regexp.MustCompile(`^-?\d+(?:\.\d+)?[smhd]?$`).MatchString(tokens[0])) {
				tokens = tokens[1:]
			}
		}
	}
	for len(tokens) != 0 && strings.HasPrefix(tokens[0], "-") {
		tokens = tokens[1:]
	}
	if len(tokens) == 0 {
		return false
	}
	word := tokens[0]
	base := filepath.Base(word)
	if base == "go" {
		return len(tokens) > 1 && oneOf(tokens[1], "test", "build", "vet", "run")
	}
	if base == "golangci-lint" || base == "ze-test" || strings.HasPrefix(base, "ze-test-") {
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

// beforeRedirection answers the words up to the first redirection. The
// tokenizer keeps `2>&1` and `>` as words of their own. A caller counting what
// follows an area name would therefore read `le functional 2>&1` as a command
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
// The suites stopped running on their bare name, so `le functional` and
// `le test-unit` print a listing and the run needs `gating` or `all`. `le
// verify` still runs the gate on its bare name, which is why the exemption
// names the two areas rather than the shape.
func heavyArea(words []string) bool {
	words = beforeRedirection(words)
	if len(words) == 0 {
		return false
	}
	if len(words) == 1 && oneOf(words[0], "functional", "test-unit") {
		return false
	}
	if len(words) > 1 && words[0] == "verify" && oneOf(words[1], "status", "summary") {
		return false
	}
	return oneOf(words[0], "verify", "functional", "integration", "qemu", "test-unit")
}

// ze point: commands/no-pipes-on-expensive-commands/never-pipe-an-expensive-command-read-the-log
// ze point: commands/directives/run-commands-through-native-actions-and-never-poll
// bashLossyPipe refuses piping an expensive command through a filter that drops
// output. The truncated text is what the caller would judge the run by, and the
// part a filter removes is usually the part that says why the run failed.
func bashLossyPipe(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	flat := strings.ReplaceAll(command, "\\\n", " ")
	flat = regexp.MustCompile(`\|[ \t]*\n`).ReplaceAllString(flat, "| ")
	for _, statement := range regexp.MustCompile(`&&|\|\||;|\n`).Split(flat, -1) {
		segments := regexp.MustCompile(`\|&?`).Split(statement, -1)
		if len(segments) < 2 {
			continue
		}
		expensive := false
		for _, segment := range segments[:len(segments)-1] {
			expensive = expensive || commandExpensive(segment)
		}
		if !expensive {
			continue
		}
		for _, segment := range segments[1:] {
			fields := strings.Fields(segment)
			if len(fields) == 0 || !oneOf(fields[0], "head", "tail", "grep", "egrep", "fgrep", "awk", "sed", "cat", "less", "more") {
				continue
			}
			return &verdict{2, "❌ Blocked: piping an expensive command's output through a lossy filter (" + fields[0] + ")\n" +
				"  -- The truncated output is what you would judge the run by.\n" +
				"  -- Use: dir=$(./le session scratch ensure); ./le verify worktree > \"$dir/verify.log\" 2>&1\n" +
				"  -- Or:  dir=$(./le session scratch ensure); <command> 2>&1 | tee \"$dir/out.log\"\n" +
				"  -- Then: Read the log with offset/limit"}
		}
	}
	return nil
}

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

func rawHeavy(segment string) (string, string) {
	tokens := shellWords(segment)
	if len(tokens) == 0 {
		return "", ""
	}
	for len(tokens) != 0 {
		head := tokens[0]
		if strings.HasPrefix(head, "ZE_ADMIT_RAW=") && strings.Trim(strings.TrimPrefix(head, "ZE_ADMIT_RAW="), "'\"") != "" {
			return "", ""
		}
		if strings.Contains(strings.Split(head, "/")[0], "=") {
			tokens = tokens[1:]
			continue
		}
		if oneOf(head, "bash", "sh", "perl", "ruby", "sudo", "time", "nice", "env", "timeout") {
			tokens = tokens[1:]
			for len(tokens) != 0 && (strings.HasPrefix(tokens[0], "-") || regexp.MustCompile(`^\d+(?:\.\d+)?[smhd]?$`).MatchString(tokens[0])) {
				tokens = tokens[1:]
			}
			continue
		}
		break
	}
	if len(tokens) == 0 {
		return "", ""
	}
	base := filepath.Base(tokens[0])
	if base == "go" && len(tokens) > 1 && tokens[1] == "test" {
		return "`go test`", admittedCommand("unit-pkg", tokens)
	}
	if base == "golangci-lint" {
		sub := ""
		for _, token := range tokens[1:] {
			if !strings.HasPrefix(token, "-") {
				sub = token
				break
			}
		}
		if oneOf(sub, "config", "version", "help", "linters", "cache", "completion") {
			return "", ""
		}
		return "`golangci-lint`", admittedCommand("lint", tokens)
	}
	if base == "ze-test" || strings.HasPrefix(base, "ze-test-") {
		suite := "<suite>"
		positional := make([]string, 0, 2)
		for _, token := range tokens[1:] {
			if !strings.HasPrefix(token, "-") && !regexp.MustCompile(`^\d+$`).MatchString(token) {
				positional = append(positional, token)
			}
		}
		if len(positional) != 0 {
			suite = positional[0]
			if suite == "bgp" && len(positional) > 1 {
				suite = positional[1]
			}
		}
		return "the functional runner `" + tokens[0] + "`", admittedCommand("functional-"+suite, tokens)
	}
	return "", ""
}

func admittedCommand(label string, tokens []string) string {
	return "./le job run label " + label + " command " + strings.Join(tokens, " ")
}

// ze point: commands/directives/heavy-jobs-are-admitted-by-native-actions-never-typed-raw
// bashRawHeavy refuses a heavy job typed raw, outside job admission. One machine
// carries several sessions, and an unadmitted job oversubscribes it.
func bashRawHeavy(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	for _, segment := range commandSegments(command) {
		what, replacement := rawHeavy(segment)
		if what == "" {
			continue
		}
		return &verdict{2, "❌ Blocked: " + what + " run raw, outside job admission (ai/rules/commands.md)\n" +
			"  -- One machine, several sessions: registered `./le` actions admit\n" +
			"     their own heavy work. A raw tool reaches the box unadmitted.\n" +
			"  -- Use: " + replacement + "\n" +
			"  -- Generic form:\n     ./le job run label <label> command <argv...>\n" +
			"  -- A one-off that must not queue states its reason:\n" +
			"     ZE_ADMIT_RAW=\"bisecting one 2s case\" <command>"}
	}
	return nil
}

// ze point: commands/directives/run-commands-through-native-actions-and-never-poll
// bashPollLoop refuses an unbounded wait loop. A loop with no timeout holds the
// session until something outside it intervenes.
func bashPollLoop(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	locations := waitKeyword.FindAllStringIndex(command, -1)
	searches := map[string]bool{"grep": true, "egrep": true, "fgrep": true, "rg": true, "ag": true, "git": true, "sed": true, "awk": true, "cat": true, "echo": true, "printf": true}
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
			"     ./le verify current mode full for the same cores."}
	}
	return nil
}

// ze point: testing/temporary-files/use-project-tmp-for-scratch-files
// bashSystemTmp refuses the system temporary directory. Session scratch belongs
// under the per-session directory, which is isolated and reaped with the session.
func bashSystemTmp(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	if command == "" || !systemTmp.MatchString(command) {
		return nil
	}
	return &verdict{2, "❌ Blocked: /tmp access is forbidden\nUse this session's own scratch dir instead:\n  dir=$(./le session scratch ensure)   # <session-dir>/scratch/\n  <command> > \"$dir/<name>\""}
}

// ze point: commands/write-ad-hoc-scratch-under-your-per-session-dir/write-ad-hoc-scratch-under-this-session-s-private-directory
// bashScratch refuses ad-hoc scratch written at the tmp/ root. Sessions share
// that tree, so an unqualified name collides with whatever another session wrote.
func bashScratch(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
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
		"  -- Use: dir=$(./le session scratch ensure); <command> > \"$dir/out.log\"\n" +
		"  -- A subdirectory passes, and so do the root names that are shared by design: ze-verify*, commit-*, delete-*, mutation*, test-timings*\n" +
		"  -- ai/rules/commands.md, 'Write Ad-Hoc Scratch Under Your Per-Session Dir'"}
}

func draftOnly(command string) bool {
	targets := rootTestPath.FindAllString(command, -1)
	if len(targets) == 0 {
		return false
	}
	for _, target := range targets {
		clean := filepath.ToSlash(filepath.Clean(target))
		if clean != "test/draft" && !strings.HasPrefix(clean, "test/draft/") {
			return false
		}
	}
	return true
}

// ze point: testing/directives/write-the-test-first-and-never-weaken-it
// bashTestDeletion refuses deleting a test without approval. A deleted test is
// indistinguishable from a test that never existed, and its coverage goes with it.
func bashTestDeletion(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	errors := make([]string, 0, 2)
	if regexp.MustCompile(`(^|\s|&&|\|)(rm|git rm)\s`).MatchString(command) {
		if strings.Contains(command, "_test.go") || strings.Contains(command, ".ci") {
			errors = append(errors, "Attempting to delete test file via: "+command)
		}
		if regexp.MustCompile(`rm.*-r.*test/|rm.*-r.*internal/.*test`).MatchString(command) {
			errors = append(errors, "Attempting recursive deletion in test directory: "+command)
		}
	}
	if regexp.MustCompile(`git checkout.*(_test\.go|\.ci)`).MatchString(command) && regexp.MustCompile(`git checkout (--|[.])`).MatchString(command) {
		errors = append(errors, "Attempting to discard test file changes: "+command)
	}
	if len(errors) == 0 || draftOnly(command) {
		return nil
	}
	lines := []string{yellow + bold + "❓ Test deletion - user approval required" + reset, ""}
	for _, problem := range errors {
		lines = append(lines, "  → "+problem)
	}
	lines = append(lines, "", "  "+bold+"Allow this test deletion?"+reset)
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
// any of the five routes: a redirect, an in-place editor, tee, a copy or move,
// or an interpreter script that opens a file for writing.
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
