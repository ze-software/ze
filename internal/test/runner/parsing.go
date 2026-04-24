// Design: docs/architecture/testing/ci-format.md — test runner framework

package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/test/tmpfs"
)

// ciCommand holds a parsed cmd= line and its associated expectations.
type ciCommand struct {
	Seq       int
	Exec      string
	StdinName string

	ExpectExitCode  int
	HasExitCode     bool
	ExpectStdout    []string
	ExpectStdoutNot []string
	ExpectStdoutRe  []*regexp.Regexp
	ExpectStderr    []string
	RejectStdout    []string
	RejectStdoutRe  []*regexp.Regexp
	RejectStderrRe  []*regexp.Regexp
}

// ParsingTest holds a single parsing test case.
type ParsingTest struct {
	BaseTest // Embeds Name, Nick, Active, Error
	File     string

	// For .ci files: inline config content (nil for .conf files)
	InlineConfig []byte

	// Parsed commands from cmd= lines (nil for legacy .conf files)
	Commands []*ciCommand

	// Tmpfs files to materialize in working directory
	TmpfsFiles map[string][]byte

	// Stdin blocks for piping into commands
	StdinBlocks map[string][]byte

	// Negative test support: if non-empty, expect validation to fail.
	// Each entry is a substring that must appear in stderr.
	// .expect files produce a single entry (optionally regex-prefixed).
	ExpectErrors []string
	ExpectRegex  *regexp.Regexp // Compiled regex when single ExpectErrors entry starts with "regex:"
	IsRegexMatch bool           // True if using regex matching (single-entry .expect files only)

	// Environment variables to set when running commands.
	EnvVars []string

	// SkipReason: when non-empty, the runner reports SKIP without
	// running the test. Set by option=skip-os:value=<list>
	// when the current GOOS is in the list.
	SkipReason string

	// Results
	Output string
}

// ParsingTests manages parsing test discovery and execution.
type ParsingTests struct {
	*TestSet[*ParsingTest]
	baseDir string
}

// NewParsingTests creates a new parsing test manager.
func NewParsingTests(baseDir string) *ParsingTests {
	return &ParsingTests{
		TestSet: NewTestSet[*ParsingTest](),
		baseDir: baseDir,
	}
}

// Discover finds parsing tests in the directory.
// Supports two formats:
//   - Legacy: valid/*.conf (positive) and invalid/*.conf + .expect (negative)
//   - Unified: *.ci files with stdin=, cmd:, expect: lines
func (pt *ParsingTests) Discover(dir string) error {
	ResetNickCounter()

	// First, try to discover .ci files (unified format)
	ciPattern := filepath.Join(dir, "*.ci")
	ciFiles, _ := filepath.Glob(ciPattern)
	sort.Strings(ciFiles)

	for _, ciFile := range ciFiles {
		test, err := pt.parseCIFile(ciFile)
		if err != nil {
			return fmt.Errorf("parse %s: %w", ciFile, err)
		}
		pt.Add(test)
	}

	// If .ci files found, we're done
	if pt.Count() > 0 {
		return nil
	}

	// Fall back to legacy format: valid/*.conf and invalid/*.conf

	// Discover positive tests (expect success) in valid/ subdirectory
	validDir := filepath.Join(dir, "valid")
	if _, err := os.Stat(validDir); err == nil {
		pattern := filepath.Join(validDir, "*.conf")
		files, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}

		sort.Strings(files)

		for _, confFile := range files {
			name := filepath.Base(confFile)
			nick := GenerateNick(name)

			test := &ParsingTest{
				BaseTest: BaseTest{
					Name: "valid/" + name,
					Nick: nick,
				},
				File: confFile,
			}
			pt.Add(test)
		}
	}

	// Discover negative tests (expect failure) in invalid/ subdirectory
	invalidDir := filepath.Join(dir, "invalid")
	if _, err := os.Stat(invalidDir); err == nil {
		invalidPattern := filepath.Join(invalidDir, "*.conf")
		invalidFiles, err := filepath.Glob(invalidPattern)
		if err != nil {
			return err
		}

		sort.Strings(invalidFiles)

		for _, confFile := range invalidFiles {
			name := filepath.Base(confFile)
			expectFile := confFile[:len(confFile)-5] + ".expect" // .conf -> .expect

			// Read expected error from .expect file
			expectBytes, err := os.ReadFile(expectFile) //nolint:gosec // Test runner, path from glob
			if err != nil {
				return fmt.Errorf("negative test %s requires .expect file: %w", name, err)
			}
			expectError := strings.TrimSpace(string(expectBytes))
			if expectError == "" {
				return fmt.Errorf("negative test %s has empty .expect file", name)
			}

			nick := GenerateNick(name)

			test := &ParsingTest{
				BaseTest: BaseTest{
					Name: "invalid/" + name,
					Nick: nick,
				},
				File:         confFile,
				ExpectErrors: []string{expectError},
			}

			// Check for regex prefix
			const regexPrefix = "regex:"
			if strings.HasPrefix(expectError, regexPrefix) {
				pattern := strings.TrimSpace(expectError[len(regexPrefix):])
				re, err := regexp.Compile(pattern)
				if err != nil {
					return fmt.Errorf("negative test %s has invalid regex pattern: %w", name, err)
				}
				test.ExpectRegex = re
				test.IsRegexMatch = true
			}

			pt.Add(test)
		}
	}

	// Error if no tests found
	if pt.Count() == 0 {
		return fmt.Errorf("no parsing tests found in %s (expected *.ci or valid/*.conf)", dir)
	}

	return nil
}

// parseCIFile parses a .ci file for parsing tests.
// Uses tmpfs.ReadFrom to handle stdin= and tmpfs= blocks, then parses
// cmd=, expect=, reject=, and option= directives from remaining lines.
func (pt *ParsingTests) parseCIFile(filePath string) (*ParsingTest, error) {
	v, err := tmpfs.ReadFrom(filePath)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSuffix(filepath.Base(filePath), ".ci")
	nick := GenerateNick(name)

	test := &ParsingTest{
		BaseTest: BaseTest{
			Name: name,
			Nick: nick,
		},
		File: filePath,
	}

	if len(v.Files) > 0 {
		test.TmpfsFiles = make(map[string][]byte, len(v.Files))
		for _, f := range v.Files {
			test.TmpfsFiles[f.Path] = f.Content
		}
	}

	if len(v.StdinBlocks) > 0 {
		test.StdinBlocks = v.StdinBlocks
		if cfg, ok := v.StdinBlocks["config"]; ok {
			test.InlineConfig = cfg
		}
	}

	var cur *ciCommand
	for _, line := range v.OtherLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if after, ok := strings.CutPrefix(trimmed, "cmd="); ok {
			rc, parseErr := parseCmdExec("foreground", after)
			if parseErr != nil {
				return nil, fmt.Errorf("%s: %w", filepath.Base(filePath), parseErr)
			}
			cur = &ciCommand{Seq: rc.Seq, Exec: rc.Exec, StdinName: rc.Stdin}
			test.Commands = append(test.Commands, cur)
			continue
		}

		if after, ok := strings.CutPrefix(trimmed, "expect=exit:code="); ok {
			if cur == nil {
				continue
			}
			code, parseErr := strconv.Atoi(after)
			if parseErr != nil {
				return nil, fmt.Errorf("%s: invalid exit code %q", filepath.Base(filePath), after)
			}
			cur.ExpectExitCode = code
			cur.HasExitCode = true
			continue
		}

		if after, ok := strings.CutPrefix(trimmed, "expect=stderr:contains="); ok {
			if cur != nil {
				cur.ExpectStderr = append(cur.ExpectStderr, after)
			}
			test.ExpectErrors = append(test.ExpectErrors, after)
			continue
		}

		if after, ok := strings.CutPrefix(trimmed, "expect=stdout:contains="); ok {
			if cur != nil {
				cur.ExpectStdout = append(cur.ExpectStdout, after)
			}
			continue
		}

		if after, ok := strings.CutPrefix(trimmed, "expect=stdout:not:contains="); ok {
			if cur != nil {
				cur.ExpectStdoutNot = append(cur.ExpectStdoutNot, after)
			}
			continue
		}

		if after, ok := strings.CutPrefix(trimmed, "expect=stdout:regex="); ok {
			re, compErr := regexp.Compile(after)
			if compErr != nil {
				return nil, fmt.Errorf("%s: invalid stdout regex %q: %w", filepath.Base(filePath), after, compErr)
			}
			if cur != nil {
				cur.ExpectStdoutRe = append(cur.ExpectStdoutRe, re)
			}
			continue
		}

		if after, ok := strings.CutPrefix(trimmed, "reject=stdout:contains="); ok {
			if cur != nil {
				cur.RejectStdout = append(cur.RejectStdout, after)
			}
			continue
		}

		if after, ok := strings.CutPrefix(trimmed, "reject=stdout:pattern="); ok {
			re, compErr := regexp.Compile(after)
			if compErr != nil {
				return nil, fmt.Errorf("%s: invalid reject stdout pattern %q: %w", filepath.Base(filePath), after, compErr)
			}
			if cur != nil {
				cur.RejectStdoutRe = append(cur.RejectStdoutRe, re)
			}
			continue
		}

		if after, ok := strings.CutPrefix(trimmed, "reject=stderr:pattern="); ok {
			re, compErr := regexp.Compile(after)
			if compErr != nil {
				return nil, fmt.Errorf("%s: invalid reject stderr pattern %q: %w", filepath.Base(filePath), after, compErr)
			}
			if cur != nil {
				cur.RejectStderrRe = append(cur.RejectStderrRe, re)
			}
			continue
		}

		if after, ok := strings.CutPrefix(trimmed, "option=skip-os:value="); ok {
			for skipOS := range strings.SplitSeq(after, ",") {
				if strings.TrimSpace(skipOS) == runtime.GOOS {
					test.SkipReason = fmt.Sprintf("skip-os=%s (current GOOS=%s)", after, runtime.GOOS)
					break
				}
			}
			continue
		}

		if after, ok := strings.CutPrefix(trimmed, "option=env:"); ok {
			var envVar, envVal string
			for field := range strings.SplitSeq(after, ":") {
				if v, ok := strings.CutPrefix(field, "var="); ok {
					envVar = v
				}
				if v, ok := strings.CutPrefix(field, "value="); ok {
					envVal = v
				}
			}
			if envVar != "" {
				test.EnvVars = append(test.EnvVars, envVar+"="+envVal)
			}
			continue
		}
	}

	if test.InlineConfig == nil && len(test.TmpfsFiles) == 0 && len(test.Commands) == 0 {
		return nil, fmt.Errorf("no config content or commands found")
	}

	return test, nil
}

// List prints available tests with type-specific formatting.
func (pt *ParsingTests) List() {
	fmt.Fprintln(os.Stdout, "\nAvailable parsing tests:") //nolint:errcheck // user output
	fmt.Fprintln(os.Stdout)                               //nolint:errcheck // user output
	for _, t := range pt.Registered() {
		switch {
		case t.IsRegexMatch:
			fmt.Fprintf(os.Stdout, "  %s  %s (expect failure, regex)\n", t.Nick, t.Name) //nolint:errcheck // user output
		case len(t.ExpectErrors) > 0:
			fmt.Fprintf(os.Stdout, "  %s  %s (expect failure)\n", t.Nick, t.Name) //nolint:errcheck // user output
		default:
			fmt.Fprintf(os.Stdout, "  %s  %s\n", t.Nick, t.Name) //nolint:errcheck // user output
		}
	}
	fmt.Fprintln(os.Stdout) //nolint:errcheck // user output
}

// ParsingRunner executes parsing tests.
type ParsingRunner struct {
	tests   *ParsingTests
	baseDir string
	zePath  string
	colors  *Colors
}

// NewParsingRunner creates a parsing test runner.
func NewParsingRunner(tests *ParsingTests, baseDir, zePath string) *ParsingRunner {
	return &ParsingRunner{
		tests:   tests,
		baseDir: baseDir,
		zePath:  zePath,
		colors:  NewColors(),
	}
}

// Run executes selected tests in parallel with real-time progress display.
func (r *ParsingRunner) Run(ctx context.Context, verbose, quiet bool) bool {
	selected := r.tests.Selected()
	if len(selected) == 0 {
		fmt.Fprintln(os.Stdout, "No tests selected") //nolint:errcheck // user output
		return true
	}

	// Create parallel runner with generic type for direct test access
	runner := NewParallelRunner[*ParsingTest](r.colors)
	runner.SetQuiet(quiet)
	runner.SetVerbose(verbose)
	runner.SetLabel("parse")
	runner.SetNoHeader(true) // header managed by caller
	runner.SetBaseDir(r.baseDir)

	// Add tests to runner
	for _, test := range selected {
		rec := runner.AddTest(test.Name, test, func(runCtx context.Context, t *ParsingTest) (bool, error) {
			success := r.runTest(runCtx, t)
			if !success {
				return false, t.Error
			}
			return true, nil
		})
		// Propagate per-test SkipReason (from option=skip-os) onto the
		// Record so ParallelRunner.Run honors it without running the test.
		rec.SkipReason = test.SkipReason
	}

	runner.SetOnFail(func(test *ParsingTest, _ error) {
		fmt.Fprintf(os.Stdout, "\n%s %s: %v\n", r.colors.Red("✗"), test.Name, test.Error) //nolint:errcheck // user output
		if test.File != "" {
			fmt.Fprintf(os.Stdout, "  %s %s\n", r.colors.Gray("File:"), test.File) //nolint:errcheck // user output
		}
	})

	return runner.Run(ctx)
}

// runTest executes a single parsing test.
// For .ci files with cmd= lines: executes each command in sequence with full
// expectation checking. For legacy .conf files: runs ze config validate.
func (r *ParsingRunner) runTest(ctx context.Context, test *ParsingTest) bool {
	if len(test.Commands) > 0 {
		return r.runCITest(ctx, test)
	}
	return r.runLegacyTest(ctx, test)
}

// runCITest executes a .ci test with full cmd=, tmpfs, stdin, expect, reject support.
func (r *ParsingRunner) runCITest(ctx context.Context, test *ParsingTest) bool {
	workDir, setupErr := r.setupWorkDir(test)
	if setupErr != nil {
		test.Error = setupErr
		return false
	}
	defer os.RemoveAll(workDir) //nolint:errcheck // test cleanup

	sort.Slice(test.Commands, func(i, j int) bool {
		return test.Commands[i].Seq < test.Commands[j].Seq
	})

	var allOutput strings.Builder
	for _, ci := range test.Commands {
		if !r.runOneCommand(ctx, test, ci, workDir, &allOutput) {
			return false
		}
	}

	test.Output = allOutput.String()
	return true
}

func (r *ParsingRunner) setupWorkDir(test *ParsingTest) (string, error) {
	workDir, mkErr := os.MkdirTemp("", "ze-parse-ci-*")
	if mkErr != nil {
		return "", fmt.Errorf("create work dir: %w", mkErr)
	}

	for path, content := range test.TmpfsFiles {
		full := filepath.Join(workDir, path)
		dir := filepath.Dir(full)
		if dirErr := os.MkdirAll(dir, 0o750); dirErr != nil {
			os.RemoveAll(workDir) //nolint:errcheck // cleanup on error
			return "", fmt.Errorf("mkdir %s: %w", dir, dirErr)
		}
		if wErr := os.WriteFile(full, content, 0o644); wErr != nil { //nolint:gosec // Test runner
			os.RemoveAll(workDir) //nolint:errcheck // cleanup on error
			return "", fmt.Errorf("write tmpfs %s: %w", path, wErr)
		}
	}

	for name, content := range test.StdinBlocks {
		stdinPath := filepath.Join(workDir, "stdin-"+name+".conf")
		if wErr := os.WriteFile(stdinPath, content, 0o644); wErr != nil { //nolint:gosec // Test runner
			os.RemoveAll(workDir) //nolint:errcheck // cleanup on error
			return "", fmt.Errorf("write stdin %s: %w", name, wErr)
		}
	}

	return workDir, nil
}

// runOneCommand executes a single cmd= and checks its expectations.
func (r *ParsingRunner) runOneCommand(ctx context.Context, test *ParsingTest, ci *ciCommand, workDir string, allOutput *strings.Builder) bool {
	cmdLine := ci.Exec
	if strings.HasPrefix(cmdLine, "ze ") {
		cmdLine = r.zePath + cmdLine[2:]
	} else if cmdLine == "ze" {
		cmdLine = r.zePath
	}

	parts, splitErr := splitCommand(cmdLine)
	if splitErr != nil {
		test.Error = fmt.Errorf("seq %d: parse command %q: %w", ci.Seq, ci.Exec, splitErr)
		return false
	}

	// Replace "-" with the stdin file path. After replacement, containsDash
	// returns false, so the fallback pipe-stdin branch below is skipped
	// (stdin was already provided as a file argument).
	for i, p := range parts {
		if p == "-" && ci.StdinName != "" {
			parts[i] = filepath.Join(workDir, "stdin-"+ci.StdinName+".conf")
		}
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...) //nolint:gosec // Test runner
	cmd.Dir = workDir
	if len(test.EnvVars) > 0 {
		cmd.Env = append(os.Environ(), test.EnvVars...)
	}

	if ci.StdinName != "" && !containsDash(parts) {
		if block, ok := test.StdinBlocks[ci.StdinName]; ok {
			cmd.Stdin = strings.NewReader(string(block))
		}
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	stdoutStr := stdout.String()
	stderrStr := stderr.String()
	allOutput.WriteString(stdoutStr + stderrStr)

	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			test.Error = fmt.Errorf("seq %d: command failed: %w", ci.Seq, runErr)
			return false
		}
	}

	if ci.HasExitCode && exitCode != ci.ExpectExitCode {
		test.Error = fmt.Errorf("seq %d: expected exit code %d, got %d\nstdout: %s\nstderr: %s",
			ci.Seq, ci.ExpectExitCode, exitCode, stdoutStr, stderrStr)
		return false
	}

	if msg := checkExpectations(ci, stdoutStr, stderrStr); msg != "" {
		test.Error = fmt.Errorf("seq %d: %s", ci.Seq, msg)
		return false
	}

	return true
}

func checkExpectations(ci *ciCommand, stdoutStr, stderrStr string) string {
	for _, expect := range ci.ExpectStdout {
		if !strings.Contains(stdoutStr, expect) {
			return fmt.Sprintf("stdout missing %q\nstdout: %s", expect, stdoutStr)
		}
	}
	for _, expect := range ci.ExpectStdoutNot {
		if strings.Contains(stdoutStr, expect) {
			return fmt.Sprintf("stdout must not contain %q\nstdout: %s", expect, stdoutStr)
		}
	}
	for _, re := range ci.ExpectStdoutRe {
		if !re.MatchString(stdoutStr) {
			return fmt.Sprintf("stdout does not match regex %q\nstdout: %s", re.String(), stdoutStr)
		}
	}
	for _, expect := range ci.ExpectStderr {
		if !strings.Contains(stderrStr, expect) {
			return fmt.Sprintf("stderr missing %q\nstderr: %s", expect, stderrStr)
		}
	}
	for _, reject := range ci.RejectStdout {
		if strings.Contains(stdoutStr, reject) {
			return fmt.Sprintf("stdout must not contain %q\nstdout: %s", reject, stdoutStr)
		}
	}
	for _, re := range ci.RejectStdoutRe {
		if re.MatchString(stdoutStr) {
			return fmt.Sprintf("stdout matches forbidden pattern %q\nstdout: %s", re.String(), stdoutStr)
		}
	}
	for _, re := range ci.RejectStderrRe {
		if re.MatchString(stderrStr) {
			return fmt.Sprintf("stderr matches forbidden pattern %q\nstderr: %s", re.String(), stderrStr)
		}
	}
	return ""
}

// runLegacyTest handles .conf files (valid/ and invalid/ directories).
func (r *ParsingRunner) runLegacyTest(ctx context.Context, test *ParsingTest) bool {
	configPath := test.File

	if test.InlineConfig != nil {
		tmpFile, writeErr := os.CreateTemp("", "ze-parse-test-*.conf")
		if writeErr != nil {
			test.Error = fmt.Errorf("create temp file: %w", writeErr)
			return false
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath) //nolint:errcheck // test cleanup

		if _, writeErr = tmpFile.Write(test.InlineConfig); writeErr != nil {
			tmpFile.Close() //nolint:errcheck // already failing
			test.Error = fmt.Errorf("write temp file: %w", writeErr)
			return false
		}
		if writeErr = tmpFile.Close(); writeErr != nil {
			test.Error = fmt.Errorf("close temp file: %w", writeErr)
			return false
		}
		configPath = tmpPath
	}

	isNegative := len(test.ExpectErrors) > 0
	var cmd *exec.Cmd
	if isNegative {
		cmd = exec.CommandContext(ctx, r.zePath, "config", "validate", configPath) //nolint:gosec // Test runner
	} else {
		cmd = exec.CommandContext(ctx, r.zePath, "config", "validate", "-q", configPath) //nolint:gosec // Test runner
	}
	output, runErr := cmd.CombinedOutput()
	test.Output = string(output)

	if isNegative {
		if runErr == nil {
			test.Error = fmt.Errorf("expected failure but validation succeeded")
			return false
		}
		if test.IsRegexMatch {
			if !test.ExpectRegex.MatchString(test.Output) {
				test.Error = fmt.Errorf("expected error matching regex %q, got: %s", test.ExpectRegex.String(), test.Output)
				return false
			}
		} else {
			var missing []string
			for _, expect := range test.ExpectErrors {
				if !strings.Contains(test.Output, expect) {
					missing = append(missing, expect)
				}
			}
			if len(missing) > 0 {
				test.Error = fmt.Errorf("expected error containing %v, got: %s", missing, test.Output)
				return false
			}
		}
		return true
	}

	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			test.Error = fmt.Errorf("validation failed with exit code %d", exitErr.ExitCode())
		} else {
			test.Error = fmt.Errorf("command failed: %w", runErr)
		}
		return false
	}

	return true
}

// splitCommand splits on whitespace, respecting single/double quotes.
// Does not handle backslash escapes; .ci exec= values do not use them.
func splitCommand(s string) ([]string, error) {
	var args []string
	var current strings.Builder
	var inQuote rune

	for _, r := range s {
		switch {
		case inQuote != 0:
			if r == inQuote {
				inQuote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '"' || r == '\'':
			inQuote = r
		case r == ' ' || r == '\t':
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if inQuote != 0 {
		return nil, fmt.Errorf("unclosed quote in %q", s)
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	return args, nil
}

func containsDash(parts []string) bool {
	return slices.Contains(parts, "-")
}

// Build compiles ze for parsing tests.
func (r *ParsingRunner) Build(ctx context.Context) error {
	// Use the provided zePath - assume it's already built
	if _, err := os.Stat(r.zePath); err != nil {
		return fmt.Errorf("ze binary not found at %s: %w", r.zePath, err)
	}
	return nil
}
