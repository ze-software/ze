// Design: docs/architecture/testing/ci-format.md — test runner framework

package runner

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ParsingTest holds a single parsing test case.
type ParsingTest struct {
	BaseTest // Embeds Name, Nick, Active, Error
	File     string

	// For .ci files: inline config content (nil for .conf files)
	InlineConfig []byte

	// Negative test support: if set, expect validation to fail with this error
	// Can be a plain substring or a regex pattern (prefixed with "regex:")
	ExpectError  string
	ExpectRegex  *regexp.Regexp // Compiled regex if ExpectError starts with "regex:"
	IsRegexMatch bool           // True if using regex matching

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
				File:        confFile,
				ExpectError: expectError,
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
// Format:
//
//	stdin=config:terminator=<TERM>
//	<config content>
//	<TERM>
//	cmd=foreground:seq=1:exec=ze config validate -:stdin=config
//	expect=exit:code=<N>
//	expect=stderr:contains=<error>  (optional, for negative tests)
func (pt *ParsingTests) parseCIFile(filePath string) (*ParsingTest, error) {
	content, err := os.ReadFile(filePath) //nolint:gosec // Test runner
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

	// Parse the file
	scanner := bufio.NewScanner(bytes.NewReader(content))
	var configContent []byte
	var inStdinBlock bool
	var terminator string

	for scanner.Scan() {
		line := scanner.Text()

		// Inside stdin block - collect content
		if inStdinBlock {
			if line == terminator {
				inStdinBlock = false
				continue
			}
			configContent = append(configContent, line...)
			configContent = append(configContent, '\n')
			continue
		}

		// Skip comments and empty lines
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Parse stdin= line
		if after, ok := strings.CutPrefix(trimmed, "stdin="); ok {
			rest := after
			parts := strings.SplitSeq(rest, ":")
			for part := range parts {
				if after, ok := strings.CutPrefix(part, "terminator="); ok {
					terminator = after
					inStdinBlock = true
					break
				}
			}
			continue
		}

		// Parse expect=exit:code=N
		if strings.HasPrefix(trimmed, "expect=exit:code=") {
			// We don't need to store this - positive tests expect 0, negative expect non-0
			continue
		}

		// Parse expect=stderr:contains=<error>
		if after, ok := strings.CutPrefix(trimmed, "expect=stderr:contains="); ok {
			test.ExpectError = after
			continue
		}

		// Skip other lines (cmd:, etc.)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(configContent) == 0 {
		return nil, fmt.Errorf("no config content found in stdin block")
	}

	test.InlineConfig = configContent
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
		case t.ExpectError != "":
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

	// Add tests to runner
	for _, test := range selected {
		runner.AddTest(test.Name, test, func(runCtx context.Context, t *ParsingTest) (bool, error) {
			success := r.runTest(runCtx, t)
			if !success {
				return false, t.Error
			}
			return true, nil
		})
	}

	// Set failure callback for verbose output with reproduction command
	runner.SetOnFail(func(test *ParsingTest, _ error) {
		fmt.Fprintf(os.Stdout, "%s %s: %v\n", r.colors.Red("✗"), test.Name, test.Error) //nolint:errcheck // user output
		if test.Output != "" {
			fmt.Fprintf(os.Stdout, "  Output: %s\n", test.Output) //nolint:errcheck // user output
		}
		// Show reproduction command for file-based configs
		if test.File != "" && test.InlineConfig == nil {
			fmt.Fprintf(os.Stdout, "  %s %s validate %s\n", r.colors.Gray("Reproduce:"), r.zePath, test.File) //nolint:errcheck // user output
		}
	})

	return runner.Run(ctx)
}

// runTest executes a single parsing test.
// For positive tests (ExpectError empty): expect success (exit 0).
// For negative tests (ExpectError set): expect failure with matching error message.
func (r *ParsingRunner) runTest(ctx context.Context, test *ParsingTest) bool {
	// Determine config file path
	configPath := test.File

	// For inline config (.ci files), write to temp file
	if test.InlineConfig != nil {
		tmpFile, err := os.CreateTemp("", "ze-parse-test-*.conf")
		if err != nil {
			test.Error = fmt.Errorf("create temp file: %w", err)
			return false
		}
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		if _, err := tmpFile.Write(test.InlineConfig); err != nil {
			_ = tmpFile.Close()
			test.Error = fmt.Errorf("write temp file: %w", err)
			return false
		}
		if err := tmpFile.Close(); err != nil {
			test.Error = fmt.Errorf("close temp file: %w", err)
			return false
		}
		configPath = tmpFile.Name()
	}

	// Run ze config validate
	// Use quiet mode for positive tests (faster), normal mode for negative tests (need error output)
	var cmd *exec.Cmd
	if test.ExpectError != "" {
		cmd = exec.CommandContext(ctx, r.zePath, "config", "validate", configPath) //nolint:gosec // Test runner
	} else {
		cmd = exec.CommandContext(ctx, r.zePath, "config", "validate", "-q", configPath) //nolint:gosec // Test runner
	}
	output, err := cmd.CombinedOutput()
	test.Output = string(output)

	// Negative test: expect failure with specific error
	if test.ExpectError != "" {
		if err == nil {
			test.Error = fmt.Errorf("expected failure but validation succeeded")
			return false
		}
		// Check that output matches expected error (regex or substring)
		if test.IsRegexMatch {
			if !test.ExpectRegex.MatchString(test.Output) {
				test.Error = fmt.Errorf("expected error matching regex %q, got: %s", test.ExpectRegex.String(), test.Output)
				return false
			}
		} else {
			if !strings.Contains(test.Output, test.ExpectError) {
				test.Error = fmt.Errorf("expected error containing %q, got: %s", test.ExpectError, test.Output)
				return false
			}
		}
		return true
	}

	// Positive test: expect success
	if err != nil {
		// Check if it's an exit code error
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			test.Error = fmt.Errorf("validation failed with exit code %d", exitErr.ExitCode())
		} else {
			test.Error = fmt.Errorf("command failed: %w", err)
		}
		return false
	}

	return true
}

// Build compiles ze for parsing tests.
func (r *ParsingRunner) Build(ctx context.Context) error {
	// Use the provided zePath - assume it's already built
	if _, err := os.Stat(r.zePath); err != nil {
		return fmt.Errorf("ze binary not found at %s: %w", r.zePath, err)
	}
	return nil
}
