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
	goBuildAll       = regexp.MustCompile(`go\s+build\s+(-[A-Za-z0-9_]+\s+)*\./\.\.\.`)
	systemTmp        = regexp.MustCompile(`(^|[\s='"$(` + "`" + `:,])/tmp(/|\s|$)`)
	scratchWrite     = regexp.MustCompile(`(?:>>?\s*|\btee\s+(?:-a\s+)?)(["']?)([A-Za-z0-9_./-]*tmp/[^\s;|&)'\"]+)`)
	waitKeyword      = regexp.MustCompile(`\b(while|until)\b`)
	sleepCall        = regexp.MustCompile(`(?:^|[;&|(\s./])sleep\s*[0-9$"'(]`)
	loopPGrep        = regexp.MustCompile(`\b(while|until)\s+(?:!\s*)?(?:\[\[?\s*)?pgrep\b`)
	timeoutBound     = regexp.MustCompile(`(^|[^-A-Za-z0-9_])timeout\s+(?:-\S+\s+)*[0-9]+(?:\.[0-9]+)?[smhd]?\b`)
	rootTestPath     = regexp.MustCompile(`[^\s'\"]*(?:test/|_test\.go)[^\s'\"]*`)
	governedPath     = regexp.MustCompile(`(?:plan/|ai/rules/)`)
	governedRedirect = regexp.MustCompile(`>>?[ \t]*["']?(?:plan/|ai/rules/)`)
	governedSed      = regexp.MustCompile(`(?:^|[\s;&|(` + "`" + `])(?:sed|perl)[ \t]+[^|;&\n]*(?:[ \t])-i\b[^|;&\n]*(?:plan/|ai/rules/)`)
	governedTee      = regexp.MustCompile(`(?:^|[\s;&|(` + "`" + `])tee[ \t]+(?:-a[ \t]+)?["']?(?:plan/|ai/rules/)`)
	governedCopy     = regexp.MustCompile(`(?:^|[\s;&|(` + "`" + `])(?:mv|cp)[ \t]+[^|;&\n]*[ \t]["']?(?:plan/|ai/rules/)`)
	governedRuntime  = regexp.MustCompile(`\b(?:perl|ruby)\b`)
	governedWrite    = regexp.MustCompile(`(?:open\([^)]*["'][wa]|write_text\(|\.write\(|writelines\(|truncate\()`)
)

// ze point: none -- the worktree prohibition lives in ai/INSTRUCTIONS.md, outside the rule corpus
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

// ze point: git-safety/before-destructive-actions/never-run-a-destructive-git-verb
func bashDestructiveGit(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	if strings.Contains(command, "git restore --staged") {
		return nil
	}
	for _, pattern := range []string{
		"git commit", "git push", "git reset", "git checkout --", "git checkout -f",
		"git checkout HEAD", "git restore", "git revert", "git stash drop",
		"git stash clear", "git clean", "git push --force", "git push -f", "git merge",
	} {
		if strings.Contains(command, pattern) {
			return &verdict{2, "❌ Blocked: " + pattern + " (run manually)"}
		}
	}
	return nil
}

// ze point: none -- build hygiene; no rule states where a Go binary must be written
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
			for len(tokens) != 0 && (strings.HasPrefix(tokens[0], "-") || regexp.MustCompile(`^-?[0-9]+(?:\.[0-9]+)?[smhd]?$`).MatchString(tokens[0])) {
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
	heavyArea := func(area string) bool {
		return oneOf(area, "verify", "functional", "integration", "qemu", "test-unit", "verify-deps", "verify-lint")
	}
	if base == "le" {
		return len(tokens) > 1 && heavyArea(tokens[1])
	}
	if strings.HasPrefix(base, "ze") && len(tokens) > 2 && tokens[1] == "le" {
		return heavyArea(tokens[2])
	}
	return false
}

// ze point: commands/no-pipes-on-expensive-commands/never-pipe-an-expensive-command-read-the-log
// ze point: commands/directives/run-commands-through-native-actions-and-never-poll
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
			for len(tokens) != 0 && (strings.HasPrefix(tokens[0], "-") || regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?[smhd]?$`).MatchString(tokens[0])) {
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
			if !strings.HasPrefix(token, "-") && !regexp.MustCompile(`^[0-9]+$`).MatchString(token) {
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
func bashSystemTmp(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	if command == "" || !systemTmp.MatchString(command) {
		return nil
	}
	return &verdict{2, "❌ Blocked: /tmp access is forbidden\nUse this session's own scratch dir instead:\n  dir=$(./le session scratch ensure)   # <session-dir>/scratch/\n  <command> > \"$dir/<name>\""}
}

// ze point: commands/write-ad-hoc-scratch-under-your-per-session-dir/write-ad-hoc-scratch-under-this-session-s-private-directory
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
func bashTestDeletion(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	errors := make([]string, 0, 2)
	if regexp.MustCompile(`(^|[\s]|&&|\|)(rm|git rm)[\s]`).MatchString(command) {
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

func governedReason(command string) bool {
	marker := "ZE_ADMIT_GOVERNED_WRITE="
	index := strings.Index(command, marker)
	if index < 0 {
		return false
	}
	value := strings.Fields(command[index+len(marker):])
	return len(value) != 0 && strings.Trim(value[0], "'\"") != ""
}

// ze point: commands/directives/bash-must-not-edit-a-governed-document
func bashGovernedWrite(ctx context) *verdict {
	command := stringInput(ctx.input, "command")
	scriptWrite := governedRuntime.MatchString(command) && governedPath.MatchString(command) && governedWrite.MatchString(command)
	if !(governedRedirect.MatchString(command) || governedSed.MatchString(command) || governedTee.MatchString(command) || governedCopy.MatchString(command) || scriptWrite) || governedReason(command) {
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
