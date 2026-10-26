// Design: docs/architecture/core-design.md -- native post-edit hook policy
package hookruntime

import (
	"bytes"
	stdcontext "context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/derived"
	"github.com/ze-software/ze/internal/le/gitenv"
	specjournal "github.com/ze-software/ze/internal/le/spec/journal"
)

func existingGo(ctx context, skipTest bool) bool {
	if !oneOf(ctx.tool, toolWrite, "Edit") || !strings.HasSuffix(ctx.path, ".go") || skipTest && strings.HasSuffix(ctx.path, "_test.go") {
		return false
	}
	info, err := os.Stat(absolutePath(ctx))
	return err == nil && info.Mode().IsRegular()
}

func readEdited(ctx context) string {
	body, err := os.ReadFile(absolutePath(ctx))
	if err != nil {
		return ""
	}
	return string(body)
}

// generatedMarkerRE is the line Go defines for generated source. One line, at
// the start of a line, exactly as `go generate` specifies it.
var generatedMarkerRE = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)

// postLintCommand builds the golangci-lint run over the edited file's package.
//
// --allow-serial-runners waits for golangci-lint's machine-wide lock inside the
// caller's 60s budget. The default gives up after five seconds and prints
// "parallel golangci-lint is running", which the issue count then reported to
// the author as a lint failure in the file they just edited.
//
// --new-from-rev makes golangci-lint's revgrep run `git status` and `git diff`
// on lint's own context, and its 30s timeout SIGKILLs them. Killed while
// refreshing the index, they left an empty .git/index.lock that blocked every
// session's git writes (three times on 2026-10-23/24), so the environment tells
// git to skip that optional refresh.
func postLintCommand(timeout stdcontext.Context, binary, module, relative string) *exec.Cmd {
	command := exec.CommandContext(timeout, binary, "run", "--allow-serial-runners", "--new-from-rev=HEAD", "--timeout=30s", "./"+filepath.ToSlash(filepath.Dir(relative))+"/...") //nolint:gosec // golangci-lint from PATH, run on the package the developer just edited
	command.Dir = module
	command.Env = gitenv.ReadOnlyPatch(os.Environ())
	return command
}

// ze point: quality/linting/fix-lint-issues-never-disable-a-linter
// postFormatGo formats the edited Go file, then refuses it while the linter
// still reports an issue in its package. A generated file is left as its
// generator wrote it: formatting one here writes a shape the generator will
// never reproduce, and the output check that compares the two then reads the
// file as out of date with nothing in the tree to explain why.
func postFormatGo(ctx context) *verdict {
	if !existingGo(ctx, false) {
		return nil
	}
	if generatedMarkerRE.MatchString(readEdited(ctx)) {
		return nil
	}
	module := filepath.Dir(absolutePath(ctx))
	for module != filepath.Dir(module) {
		if _, err := os.Stat(filepath.Join(module, "go.mod")); err == nil {
			break
		}
		module = filepath.Dir(module)
	}
	if _, err := os.Stat(filepath.Join(module, "go.mod")); err != nil {
		return nil
	}
	if binary, err := exec.LookPath("gofmt"); err == nil {
		timeout, cancel := stdcontext.WithTimeout(stdcontext.Background(), 30*time.Second)
		command := exec.CommandContext(timeout, binary, "-w", absolutePath(ctx)) //nolint:gosec // gofmt from PATH, run on the file the developer just edited
		_, _ = command.CombinedOutput()
		cancel()
	}
	if binary, err := exec.LookPath("goimports"); err == nil {
		timeout, cancel := stdcontext.WithTimeout(stdcontext.Background(), 30*time.Second)
		command := exec.CommandContext(timeout, binary, "-local", "github.com/ze-software/ze", "-format-only", "-w", absolutePath(ctx)) //nolint:gosec // goimports from PATH, run on the file the developer just edited
		_, _ = command.CombinedOutput()
		cancel()
	}
	relative, _ := filepath.Rel(module, absolutePath(ctx))
	pkg := "./" + filepath.ToSlash(filepath.Dir(relative)) + "/..."
	binary, err := exec.LookPath("golangci-lint")
	if err != nil {
		return lintNotRun(pkg, "golangci-lint is not on PATH", "", "./le setup tools")
	}
	timeout, cancel := stdcontext.WithTimeout(stdcontext.Background(), postLintBudget)
	output, runErr := postLintCommand(timeout, binary, module, relative).CombinedOutput()
	expired := timeout.Err() != nil
	cancel()
	return postLintVerdict(pkg, filepath.ToSlash(relative), string(output), runErr, expired)
}

// postLintBudget is how long the post-edit lint may take, the wait for
// golangci-lint's machine-wide lock included. The hook's own timeout in
// .claude/settings.json is 90s, so the verdict is written before it fires.
const postLintBudget = 60 * time.Second

// lintIssuesFound is golangci-lint's exit code for a run that completed and
// found issues (pkg/exitcodes IssuesFound). Every other non-zero code is a run
// that did not complete: a failure, its own timeout, a load error.
const lintIssuesFound = 1

// postLintVerdict reads one post-edit lint run. Only a clean exit is a pass and
// only exit 1 is a list of issues; a run killed at the budget, stopped at its
// own timeout or failed before analysis is said as a run that did NOT happen.
// Before this distinction a killed run printed nothing and read as clean, so
// the author was told nothing about a package no linter had looked at.
//
// The run covers the whole package, but only the edited file's issues block.
// --new-from-rev=HEAD reports every uncommitted change in the package, and in
// this shared checkout most of those are another session's: blocking on them
// handed one agent another agent's work, in its context, on every edit. A
// package that does not build blocks whichever file broke it, because an edit
// here can break a sibling file, and a broken package build is everyone's.
func postLintVerdict(pkg, file, output string, runErr error, expired bool) *verdict {
	if expired {
		return lintNotRun(pkg, "it ran past "+postLintBudget.String()+", most often waiting behind another session's lint", output, "")
	}
	if runErr == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) {
		return lintNotRun(pkg, runErr.Error(), output, "")
	}
	if exitErr.ExitCode() != lintIssuesFound {
		return lintNotRun(pkg, "golangci-lint stopped with "+exitErr.String(), output, "")
	}
	if lintTypecheckRE.MatchString(output) {
		// A compile failure is one multi-line issue whose first line names
		// whichever file golangci-lint loaded first, so it cannot be judged
		// by file. The package does not build, which blocks every author.
		lines := make([]string, 0, 4)
		lines = append(lines, yellow+"⚠ lint: "+pkg+" does not build"+reset)
		for line := range strings.SplitSeq(output, "\n") {
			if lintIssueRE.MatchString(line) && len(lines) < 4 {
				lines = append(lines, "  "+dim+line+reset)
			}
		}
		return &verdict{2, strings.Join(lines, "\n")}
	}
	headers := lintIssueRE.FindAllStringSubmatch(output, -1)
	if len(headers) == 0 {
		// Exit 1 with no issue line this parser recognizes: the output format
		// moved under it. Blocking with the raw text is the answer that cannot
		// pass a package nobody judged.
		return lintNotRun(pkg, "its issues could not be read from the output", output, "")
	}
	owned := make([]string, 0, len(headers))
	foreign := 0
	for _, header := range headers {
		if header[1] == file {
			owned = append(owned, header[0])
			continue
		}
		foreign++
	}
	if len(owned) == 0 {
		// Code 0 reaches the person watching, never the agent's context.
		return &verdict{0, dim + "lint: " + file + " is clean; " + strconv.Itoa(foreign) + " issue(s) in other changed files of " + pkg + reset}
	}
	lines := make([]string, 0, 4)
	lines = append(lines, yellow+"⚠ lint: "+strconv.Itoa(len(owned))+" issue(s) in "+file+reset)
	for _, header := range owned[:min(3, len(owned))] {
		lines = append(lines, "  "+dim+header+reset)
	}
	return &verdict{2, strings.Join(lines, "\n")}
}

// lintIssueRE matches the first line of one golangci-lint text-format issue,
// `path.go:line:col: message (linter)`, capturing the path. The source excerpt
// and caret lines that follow it, and the closing per-linter summary, never
// match, so each issue is counted once.
var lintIssueRE = regexp.MustCompile(`(?m)^([^\s:]+\.go):\d+(?::\d+)?: .*$`)

// lintTypecheckRE matches the closing summary's typecheck row, `* typecheck: N`,
// the one place a compile failure names its linter on a line of its own.
var lintTypecheckRE = regexp.MustCompile(`(?m)^\* typecheck: \d+`)

// lintNotRun is the verdict for a lint run that did not complete. It blocks
// (code 2) because only a blocking post-tool verdict reaches the agent, and an
// agent that is not told keeps reading the silence as a pass. fix replaces the
// default command when the cure is not a rerun.
func lintNotRun(pkg, why, output, fix string) *verdict {
	if fix == "" {
		fix = "./le go lint run scope \"" + pkg + "\""
	}
	lines := make([]string, 0, 5)
	lines = append(lines, yellow+"⚠ lint did NOT run on "+pkg+": "+why+reset)
	for line := range strings.SplitSeq(output, "\n") {
		if line != "" && len(lines) < 4 {
			lines = append(lines, "  "+dim+line+reset)
		}
	}
	lines = append(lines, "  Run: "+fix)
	return &verdict{2, strings.Join(lines, "\n")}
}

// ze point: none -- the 1,000-line advisory is Go style guidance outside the rule corpus
// postFileSize reports a Go file past the 1,000-line advisory.
func postFileSize(ctx context) *verdict {
	if !existingGo(ctx, true) {
		return nil
	}
	text := readEdited(ctx)
	lines := strings.Count(text, "\n")
	if lines <= 1000 {
		return nil
	}
	return &verdict{1, fmt.Sprintf("%s%s⚠️  File too large: %s (%d lines > 1000)%s", red, bold, filepath.Base(ctx.path), lines, reset)}
}

// ze point: none -- the deferral-language advisory has no one rule point that states its heuristic
// postDeferral reports deferral language in a document that is not a deferral.
func postDeferral(ctx context) *verdict {
	path := filepath.ToSlash(ctx.path)
	if !strings.HasSuffix(path, ".md") || strings.Contains(path, ".claude/memory/") || strings.Contains(path, ".claude/plan/") || strings.Contains(path, "tmp/session/") || strings.Contains(path, "plan/learned/") {
		return nil
	}
	content := stringInput(ctx.input, "new_string")
	if ctx.tool == toolWrite {
		content = stringInput(ctx.input, "content")
	}
	for _, phrase := range []string{"deferred to", "deferred for", "defer to", "out of scope", "future work", "future spec", "handle later", "address later", "skip for now", "skipping for now", "postpone", "not yet implemented", "not yet wired"} {
		if strings.Contains(strings.ToLower(content), phrase) {
			return &verdict{1, yellow + bold + "  Deferral language detected in " + filepath.Base(path) + reset + "\n  " + yellow + "Pattern: '" + phrase + "'" + reset + "\n  " + yellow + "Write the remainder as its own spec, in the bucket that work belongs to, and name that spec here." + reset}
		}
	}
	return nil
}

// ze point: none -- journal row validation is defined by the journal format, not a rule point
// postJournal reports a journal file that ./le commit create would refuse.
func postJournal(ctx context) *verdict {
	path := filepath.ToSlash(ctx.path)
	if !regexp.MustCompile(`(^|/)plan/journal/.+\.md$`).MatchString(path) || strings.HasSuffix(path, "/README.md") {
		return nil
	}
	report, err := specjournal.ValidateFile(ctx.root, ctx.path)
	if err != nil {
		return &verdict{1, yellow + bold + "⚠ journal: native validation could not run, so " + filepath.Base(path) + " was NOT checked" + reset + "\n  " + yellow + err.Error() + reset}
	}
	if report.ExitCode() == 0 {
		return nil
	}
	return &verdict{1, yellow + bold + "⚠ journal: " + filepath.Base(path) + " is not commit-readable" + reset + "\n" + report.Text() + "  " + yellow + "./le commit create blocks the same file." + reset}
}

// ze point: none -- RFC header placement is Go style guidance outside the rule corpus
// postRFCHeader reports a file citing RFCs with no // RFC: header.
func postRFCHeader(ctx context) *verdict {
	if !existingGo(ctx, false) {
		return nil
	}
	base := filepath.Base(ctx.path)
	if strings.HasSuffix(base, "_test.go") || strings.HasSuffix(base, "_gen.go") || oneOf(base, "register.go", "embed.go", "doc.go") {
		return nil
	}
	text := readEdited(ctx)
	head := strings.Join(strings.Split(text, "\n")[:min(10, len(strings.Split(text, "\n")))], "\n")
	if strings.Contains(head, "// RFC:") || len(regexp.MustCompile(`RFC \d{4}|rfc\d{4}`).FindAllString(text, -1)) < 2 {
		return nil
	}
	return &verdict{0, yellow + "⚠ " + base + " references RFCs but has no // RFC: rfc/short/rfcNNNN.md header" + reset}
}

// ze point: none -- test-file documentation is Go style guidance outside the rule corpus
// postTestDocs reports a test file with no VALIDATES:/PREVENTS: comment.
func postTestDocs(ctx context) *verdict {
	if !strings.HasSuffix(ctx.path, "_test.go") || !existingGo(ctx, false) {
		return nil
	}
	text := readEdited(ctx)
	if regexp.MustCompile(`(?m)^func Test[A-Z]`).MatchString(text) && !strings.Contains(text, "VALIDATES:") && !strings.Contains(text, "PREVENTS:") {
		return &verdict{0, yellow + "⚠️  Test file without documentation: " + filepath.Base(ctx.path) + " (add VALIDATES:/PREVENTS: comments)" + reset}
	}
	return nil
}

// ze point: none -- fuzz-test discovery is an advisory with no bound rule point
// postFuzz reports a wire parser whose package carries no fuzz target.
func postFuzz(ctx context) *verdict {
	if !existingGo(ctx, true) {
		return nil
	}
	path := filepath.ToSlash(ctx.path)
	if !anyContains(path, "/message/", "/nlri/", "/attribute/", "/capability/") {
		return nil
	}
	text := readEdited(ctx)
	if !regexp.MustCompile(`(?m)^func (?:\([^)]+\) )?Parse[A-Z]`).MatchString(text) {
		return nil
	}
	entries, _ := filepath.Glob(filepath.Join(filepath.Dir(absolutePath(ctx)), "*_test.go"))
	for _, entry := range entries {
		body, _ := os.ReadFile(entry) //nolint:gosec // a *_test.go path this hook globbed beside the edited file
		if regexp.MustCompile(`(?m)^func Fuzz[A-Z]`).Match(body) {
			return nil
		}
	}
	return &verdict{0, yellow + "⚠️  Wire format parsing without fuzz tests: " + filepath.Base(ctx.path) + reset}
}

// ze point: none -- vague-name detection is Go style guidance outside the rule corpus
// postVague reports a vague variable name in the edited Go file.
func postVague(ctx context) *verdict {
	if !existingGo(ctx, true) {
		return nil
	}
	if regexp.MustCompile(`(^|[^A-Za-z0-9_])(Data|Info|Result|Item|Thing|Temp|Tmp|Val|Obj)[ \t]+[A-Za-z0-9_]+[ \t]*=`).MatchString(readEdited(ctx)) {
		return &verdict{0, yellow + "⚠️  Vague variable names detected in " + filepath.Base(ctx.path) + reset}
	}
	return nil
}

// ze point: none -- boundary-test discovery is an advisory with no bound rule point
// postBoundary reports numeric validation whose sibling test names no boundary.
func postBoundary(ctx context) *verdict {
	if !existingGo(ctx, true) {
		return nil
	}
	if !regexp.MustCompile(`if .* (>|<|>=|<=) (?:[0-9]|0x)|return .*?(Invalid.*Range|OutOfBounds|Exceeds)`).MatchString(readEdited(ctx)) {
		return nil
	}
	testPath := strings.TrimSuffix(absolutePath(ctx), ".go") + "_test.go"
	body, err := os.ReadFile(testPath) //nolint:gosec // the sibling test of the file the tool just edited
	if err != nil {
		return &verdict{0, yellow + "⚠️  Numeric validation but no test file: " + filepath.Base(testPath) + reset}
	}
	if !regexp.MustCompile(`(?i)boundary|invalid.*above|invalid.*below|max.*valid|min.*valid`).Match(body) {
		return &verdict{0, yellow + "⚠️  Numeric validation but no boundary tests in " + filepath.Base(testPath) + reset}
	}
	return nil
}

// tracked answers whether git holds path in its index, relative to root.
//
// The false arm AUTHORIZES A DELETION, so the two ways of not knowing are told
// apart rather than lumped together. A tree with no `.git` has no index, so
// nothing in it is tracked and the honest answer is FALSE: that is the ordinary
// case for a throwaway checkout, and answering true there would stop the hook
// working at all. Any OTHER failure means this process could not look, and a
// question it cannot answer must not read as permission to delete, so it
// answers TRUE and the caller removes nothing.
func tracked(root, path string) bool {
	// A tree that is not a git checkout holds no index, so nothing in it is
	// tracked and the question is answered without forking git. A worktree
	// carries .git as a FILE rather than a directory, so this stats both.
	//
	// ABSENT is the only error that means "no index". Any other one means this
	// process could not look, and the false arm of this function AUTHORIZES A
	// DELETION, so an unreadable checkout reads as tracked.
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return !errors.Is(err, os.ErrNotExist)
	}
	timeout, cancel := stdcontext.WithTimeout(stdcontext.Background(), gitTimeout)
	defer cancel()
	// `ls-files` exits zero whether or not it matched, and prints the path only
	// when the index holds it, so the OUTPUT is the answer and the exit code
	// reports whether git ran at all. `--error-unmatch` would conflate the two.
	command := exec.CommandContext(timeout, "git", "ls-files", "--", path) //nolint:gosec // path is a registered artifact path, and -- ends the option list
	command.Dir = root
	listed, err := command.Output()
	if err != nil {
		return true
	}
	return len(bytes.TrimSpace(listed)) != 0
}

// removeArtifact deletes one artifact, which is a file or a whole directory.
//
// A generator that writes a DIRECTORY of files registers the directory, because
// the set of files inside it is derived as well: rfc/requirements holds one
// shard per summary, and a summary that stops declaring requirements leaves a
// file the generator no longer owns. os.Remove refuses a directory that holds
// anything, so such an artifact would be reported unremovable on every edit and
// would sit there stale.
//
// Lstat rather than Stat, so a symlink is unlinked rather than followed into
// whatever it points at.
func removeArtifact(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}

// ze point: principles/directives/a-wrong-value-must-not-look-like-a-right-one
// postInvalidateDerived removes every derived artifact the written file feeds.
//
// A derived artifact is a pure function of the tree, so the moment one of its
// inputs moves, the file on disk answers about a tree that no longer exists and
// nothing says so. Removing it makes the staleness LOUD: the next reader either
// rebuilds it (preMaterializeDerived) or meets an absent file, and neither is a
// wrong answer wearing the shape of a right one.
//
// Which artifacts a path feeds is the registry's answer, not this function's.
// Nothing here names an artifact, so a fourth one is a derived.Register call in
// its own package and no edit to this file.
func postInvalidateDerived(ctx context) *verdict {
	if !oneOf(ctx.tool, toolWrite, "Edit") || ctx.path == "" {
		return nil
	}
	written := relativePath(ctx)
	if written == "" || strings.HasPrefix(written, "../") {
		return nil
	}

	// Every artifact is judged, and a refusal over one never ends the pass: the
	// artifacts are independent, so one left in place says nothing about the
	// ones behind it. Returning from inside this loop left every artifact after
	// the first refusal present and stale, which is the wrong answer this check
	// exists to prevent.
	artifacts := derived.All()
	removed := make([]string, 0, len(artifacts))
	refused := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		if !artifact.Feeds(ctx.root, written) {
			continue
		}
		// A TRACKED path is never removed here, whatever the registry says. Git
		// reads the removal as a deletion to stage, so another session's commit
		// script would carry it, and this checkout is shared. A registration
		// whose artifact is still tracked is a migration half done rather than a
		// license to delete somebody's file (ai/rules/never-destroy-work.md).
		if tracked(ctx.root, artifact.Path) {
			refused = append(refused, artifact.Path+
				" is registered as derived and still TRACKED: untrack it, or drop its derived.Register call")
			continue
		}
		err := removeArtifact(filepath.Join(ctx.root, filepath.FromSlash(artifact.Path)))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			// A removal that failed leaves a file describing the tree from
			// before this write, which a later grep reads as current. The
			// author is told here rather than by the wrong answer later.
			refused = append(refused, artifact.Path+" is stale and could not be removed: "+err.Error())
			continue
		}
		removed = append(removed, artifact.Path)
	}

	if len(refused) == 0 {
		if len(removed) == 0 {
			return nil
		}
		return &verdict{0, dim + "derived: removed " + strings.Join(removed, ", ") +
			" (rebuilt when a command names one)" + reset}
	}
	// One verdict carries both halves. An author told only about the refusal
	// cannot tell whether the rest of the pass ran.
	message := yellow + bold + "⚠ derived: " + strings.Join(refused, "\n  ") + reset
	if len(removed) != 0 {
		message += "\n  " + dim + "removed " + strings.Join(removed, ", ") + reset
	}
	return &verdict{1, message}
}
