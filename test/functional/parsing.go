package functional

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"
)

// ParsingTest holds a single parsing test case.
type ParsingTest struct {
	Name string
	Nick string
	File string

	// Results
	Active   bool
	State    State
	Output   string
	Error    error
	Duration time.Duration
}

// ParsingTests manages parsing test discovery and execution.
type ParsingTests struct {
	tests   []*ParsingTest
	byNick  map[string]*ParsingTest
	baseDir string
}

// NewParsingTests creates a new parsing test manager.
func NewParsingTests(baseDir string) *ParsingTests {
	return &ParsingTests{
		byNick:  make(map[string]*ParsingTest),
		baseDir: baseDir,
	}
}

// Discover finds all .conf files in the directory.
func (pt *ParsingTests) Discover(dir string) error {
	pattern := filepath.Join(dir, "*.conf")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	sort.Strings(files)
	ResetNickCounter()

	for _, confFile := range files {
		name := filepath.Base(confFile)
		nick := generateNick(name)

		test := &ParsingTest{
			Name: name,
			Nick: nick,
			File: confFile,
		}
		pt.tests = append(pt.tests, test)
		pt.byNick[nick] = test
	}

	return nil
}

// Registered returns all tests in order.
func (pt *ParsingTests) Registered() []*ParsingTest {
	return pt.tests
}

// Selected returns active tests.
func (pt *ParsingTests) Selected() []*ParsingTest {
	var result []*ParsingTest
	for _, t := range pt.tests {
		if t.Active {
			result = append(result, t)
		}
	}
	return result
}

// Count returns the number of tests.
func (pt *ParsingTests) Count() int {
	return len(pt.tests)
}

// EnableAll activates all tests.
func (pt *ParsingTests) EnableAll() {
	for _, t := range pt.tests {
		t.Active = true
	}
}

// EnableByNick activates a test by nick.
func (pt *ParsingTests) EnableByNick(nick string) bool {
	if t, ok := pt.byNick[nick]; ok {
		t.Active = true
		return true
	}
	return false
}

// List prints available tests.
func (pt *ParsingTests) List() {
	fmt.Println("\nAvailable parsing tests:")
	fmt.Println()
	for _, t := range pt.tests {
		fmt.Printf("  %s  %s\n", t.Nick, t.Name)
	}
	fmt.Println()
}

// ParsingRunner executes parsing tests.
type ParsingRunner struct {
	tests     *ParsingTests
	baseDir   string
	zebgpPath string
	colors    *Colors
}

// NewParsingRunner creates a parsing test runner.
func NewParsingRunner(tests *ParsingTests, baseDir, zebgpPath string) *ParsingRunner {
	return &ParsingRunner{
		tests:     tests,
		baseDir:   baseDir,
		zebgpPath: zebgpPath,
		colors:    NewColors(),
	}
}

// Run executes selected tests.
func (r *ParsingRunner) Run(ctx context.Context, verbose, quiet bool) bool {
	selected := r.tests.Selected()
	if len(selected) == 0 {
		fmt.Println("No tests selected")
		return true
	}

	passed, failed := 0, 0

	for _, test := range selected {
		test.State = StateRunning
		start := time.Now()

		success := r.runTest(ctx, test)
		test.Duration = time.Since(start)

		if success {
			test.State = StateSuccess
			passed++
			if !quiet {
				fmt.Printf("%s %s (%s)\n", r.colors.Green("✓"), test.Name, test.Duration.Truncate(time.Millisecond))
			}
		} else {
			test.State = StateFail
			failed++
			if !quiet {
				fmt.Printf("%s %s: %v\n", r.colors.Red("✗"), test.Name, test.Error)
				if verbose && test.Output != "" {
					fmt.Printf("  Output: %s\n", test.Output)
				}
			}
		}
	}

	// Summary
	if !quiet {
		fmt.Printf("\nParsing tests: %d passed, %d failed\n", passed, failed)
	}

	return failed == 0
}

// runTest executes a single parsing test.
func (r *ParsingRunner) runTest(ctx context.Context, test *ParsingTest) bool {
	// Run zebgp validate with quiet mode
	cmd := exec.CommandContext(ctx, r.zebgpPath, "validate", "-q", test.File) //nolint:gosec // Test runner
	output, err := cmd.CombinedOutput()
	test.Output = string(output)

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

// Summary returns counts by state.
func (pt *ParsingTests) Summary() (passed, failed int) {
	for _, t := range pt.tests {
		switch t.State { //nolint:exhaustive // only count terminal states
		case StateSuccess:
			passed++
		case StateFail:
			failed++
		}
	}
	return
}

// Build compiles zebgp for parsing tests.
func (r *ParsingRunner) Build(ctx context.Context) error {
	// Use the provided zebgpPath - assume it's already built
	if _, err := os.Stat(r.zebgpPath); err != nil {
		return fmt.Errorf("zebgp binary not found at %s: %w", r.zebgpPath, err)
	}
	return nil
}
