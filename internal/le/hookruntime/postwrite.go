// Design: docs/architecture/core-design.md -- native post-edit hook policy
package hookruntime

import (
	stdcontext "context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/journal"
)

func existingGo(ctx context, skipTest bool) bool {
	if !oneOf(ctx.tool, "Write", "Edit") || !strings.HasSuffix(ctx.path, ".go") || skipTest && strings.HasSuffix(ctx.path, "_test.go") {
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

// ze point: quality/linting/fix-lint-issues-never-disable-a-linter
func postFormatGo(ctx context) *verdict {
	if !existingGo(ctx, false) {
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
		command := exec.CommandContext(timeout, binary, "-w", absolutePath(ctx))
		_, _ = command.CombinedOutput()
		cancel()
	}
	if binary, err := exec.LookPath("goimports"); err == nil {
		timeout, cancel := stdcontext.WithTimeout(stdcontext.Background(), 30*time.Second)
		command := exec.CommandContext(timeout, binary, "-local", "github.com/ze-software/ze", "-format-only", "-w", absolutePath(ctx))
		_, _ = command.CombinedOutput()
		cancel()
	}
	if binary, err := exec.LookPath("golangci-lint"); err == nil {
		relative, _ := filepath.Rel(module, absolutePath(ctx))
		timeout, cancel := stdcontext.WithTimeout(stdcontext.Background(), 60*time.Second)
		command := exec.CommandContext(timeout, binary, "run", "--new-from-rev=HEAD", "--timeout=30s", "./"+filepath.ToSlash(filepath.Dir(relative))+"/...")
		command.Dir = module
		output, _ := command.CombinedOutput()
		cancel()
		text := string(output)
		if text != "" && !strings.Contains(text, "no issues") && !strings.HasPrefix(text, "0 issues") {
			lines := make([]string, 0, 3)
			issues := 0
			for _, line := range strings.Split(text, "\n") {
				if strings.Contains(line, ":") {
					issues++
				}
				if line != "" && len(lines) < 3 {
					lines = append(lines, "  "+dim+line+reset)
				}
			}
			if issues != 0 {
				return &verdict{2, fmt.Sprintf("%s⚠ lint: %d issues%s\n%s", yellow, issues, reset, strings.Join(lines, "\n"))}
			}
		}
	}
	return nil
}

// ze point: none -- the 1,000-line advisory is Go style guidance outside the rule corpus
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
func postDeferral(ctx context) *verdict {
	path := filepath.ToSlash(ctx.path)
	if !strings.HasSuffix(path, ".md") || strings.Contains(path, "plan/deferrals") || strings.Contains(path, ".claude/memory/") || strings.Contains(path, ".claude/plan/") || strings.Contains(path, "tmp/session/") || strings.Contains(path, "plan/learned/") {
		return nil
	}
	content := stringInput(ctx.input, "new_string")
	if ctx.tool == "Write" {
		content = stringInput(ctx.input, "content")
	}
	for _, phrase := range []string{"deferred to", "deferred for", "defer to", "out of scope", "future work", "future spec", "handle later", "address later", "skip for now", "skipping for now", "postpone", "not yet implemented", "not yet wired"} {
		if strings.Contains(strings.ToLower(content), phrase) {
			return &verdict{1, yellow + bold + "  Deferral language detected in " + filepath.Base(path) + reset + "\n  " + yellow + "Pattern: '" + phrase + "'" + reset + "\n  " + yellow + "Record in the source's plan/deferrals/<source>.md shard if this is deferred work." + reset}
		}
	}
	return nil
}

// ze point: none -- journal row validation is defined by the journal format, not a rule point
func postJournal(ctx context) *verdict {
	path := filepath.ToSlash(ctx.path)
	if !regexp.MustCompile(`(^|/)plan/journal/.+\.md$`).MatchString(path) || strings.HasSuffix(path, "/README.md") {
		return nil
	}
	report, err := journal.ValidateFile(ctx.root, ctx.path)
	if err != nil {
		return &verdict{1, yellow + bold + "⚠ journal: native validation could not run, so " + filepath.Base(path) + " was NOT checked" + reset + "\n  " + yellow + err.Error() + reset}
	}
	if report.ExitCode() == 0 {
		return nil
	}
	return &verdict{1, yellow + bold + "⚠ journal: " + filepath.Base(path) + " is not commit-readable" + reset + "\n" + report.Text() + "  " + yellow + "./le commit create blocks the same file." + reset}
}

// ze point: none -- RFC header placement is Go style guidance outside the rule corpus
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
	if strings.Contains(head, "// RFC:") || len(regexp.MustCompile(`RFC [0-9]{4}|rfc[0-9]{4}`).FindAllString(text, -1)) < 2 {
		return nil
	}
	return &verdict{0, yellow + "⚠ " + base + " references RFCs but has no // RFC: rfc/short/rfcNNNN.md header" + reset}
}

// ze point: none -- test-file documentation is Go style guidance outside the rule corpus
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
func postFuzz(ctx context) *verdict {
	if !existingGo(ctx, true) {
		return nil
	}
	path := filepath.ToSlash(ctx.path)
	if !(strings.Contains(path, "/message/") || strings.Contains(path, "/nlri/") || strings.Contains(path, "/attribute/") || strings.Contains(path, "/capability/")) {
		return nil
	}
	text := readEdited(ctx)
	if !regexp.MustCompile(`(?m)^func (?:\([^)]+\) )?Parse[A-Z]`).MatchString(text) {
		return nil
	}
	entries, _ := filepath.Glob(filepath.Join(filepath.Dir(absolutePath(ctx)), "*_test.go"))
	for _, entry := range entries {
		body, _ := os.ReadFile(entry)
		if regexp.MustCompile(`(?m)^func Fuzz[A-Z]`).Match(body) {
			return nil
		}
	}
	return &verdict{0, yellow + "⚠️  Wire format parsing without fuzz tests: " + filepath.Base(ctx.path) + reset}
}

// ze point: none -- vague-name detection is Go style guidance outside the rule corpus
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
func postBoundary(ctx context) *verdict {
	if !existingGo(ctx, true) {
		return nil
	}
	if !regexp.MustCompile(`if .* (>|<|>=|<=) (?:[0-9]|0x)|return .*?(Invalid.*Range|OutOfBounds|Exceeds)`).MatchString(readEdited(ctx)) {
		return nil
	}
	testPath := strings.TrimSuffix(absolutePath(ctx), ".go") + "_test.go"
	body, err := os.ReadFile(testPath)
	if err != nil {
		return &verdict{0, yellow + "⚠️  Numeric validation but no test file: " + filepath.Base(testPath) + reset}
	}
	if !regexp.MustCompile(`(?i)boundary|invalid.*above|invalid.*below|max.*valid|min.*valid`).Match(body) {
		return &verdict{0, yellow + "⚠️  Numeric validation but no boundary tests in " + filepath.Base(testPath) + reset}
	}
	return nil
}
