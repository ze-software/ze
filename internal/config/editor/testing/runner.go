package testing

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// TestResult represents the outcome of running an .et test.
type TestResult struct {
	Passed   bool   // Whether all expectations passed
	Error    string // Error message if failed
	TempDir  string // Temp directory used (empty if cleaned up)
	Duration time.Duration
}

// RunETTest parses and executes an .et test from content string.
// Returns a TestResult with pass/fail status and any error message.
func RunETTest(content string) *TestResult {
	start := time.Now()
	result := &TestResult{}

	// Parse the .et content
	tc, err := ParseETFile(content)
	if err != nil {
		result.Error = fmt.Sprintf("parse error: %v", err)
		return result
	}

	// Run the test case
	runResult := runTestCase(tc)
	result.Passed = runResult.Passed
	result.Error = runResult.Error
	result.Duration = time.Since(start)

	return result
}

// RunETFile loads and executes an .et test from a file path.
func RunETFile(path string) *TestResult {
	content, err := os.ReadFile(path) //nolint:gosec // Test file path
	if err != nil {
		return &TestResult{Error: fmt.Sprintf("reading file: %v", err)}
	}
	return RunETTest(string(content))
}

// runTestCase executes a parsed test case.
func runTestCase(tc *TestCase) *TestResult {
	result := &TestResult{}

	// Create temp directory for test files
	tmpDir, err := os.MkdirTemp("", "ze-editor-test-*")
	if err != nil {
		result.Error = fmt.Sprintf("creating temp dir: %v", err)
		return result
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Write tmpfs files to temp directory
	for _, tf := range tc.Tmpfs {
		filePath := filepath.Join(tmpDir, tf.Path)

		// Create parent directories if needed
		if dir := filepath.Dir(filePath); dir != tmpDir {
			if err := os.MkdirAll(dir, 0750); err != nil {
				result.Error = fmt.Sprintf("creating dir for %s: %v", tf.Path, err)
				return result
			}
		}

		// Determine file mode
		mode := os.FileMode(0600)
		if tf.Mode != "" {
			if m, err := strconv.ParseUint(tf.Mode, 8, 32); err == nil {
				mode = os.FileMode(m)
			}
		}

		if err := os.WriteFile(filePath, []byte(tf.Content), mode); err != nil {
			result.Error = fmt.Sprintf("writing %s: %v", tf.Path, err)
			return result
		}
	}

	// Get config file path from options
	configPath := ""
	timeout := 30 * time.Second
	width := 80
	height := 24
	reloadMode := "" // "success", "fail", or "" (standalone)

	for _, opt := range tc.Options {
		switch opt.Type {
		case "file":
			if path, ok := opt.Values["path"]; ok {
				configPath = filepath.Join(tmpDir, path)
			}
		case "timeout":
			if val, ok := opt.Values["value"]; ok {
				if d, err := time.ParseDuration(val); err == nil {
					timeout = d
				}
			}
		case "width":
			if val, ok := opt.Values["value"]; ok {
				if w, err := strconv.Atoi(val); err == nil {
					width = w
				}
			}
		case "height":
			if val, ok := opt.Values["value"]; ok {
				if h, err := strconv.Atoi(val); err == nil {
					height = h
				}
			}
		case "reload":
			if mode, ok := opt.Values["mode"]; ok {
				reloadMode = mode
			}
		}
	}

	if configPath == "" {
		result.Error = "no config file specified (use option=file:path=...)"
		return result
	}

	// Check config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		result.Error = fmt.Sprintf("config file not found: %s", configPath)
		return result
	}

	// Create headless model
	hm, err := NewHeadlessModel(configPath)
	if err != nil {
		result.Error = fmt.Sprintf("creating editor: %v", err)
		return result
	}

	// Configure mock reload notifier if requested
	switch reloadMode {
	case "success":
		hm.SetReloadNotifier(func() error { return nil })
	case "fail":
		hm.SetReloadNotifier(func() error { return fmt.Errorf("daemon not reachable") })
	}

	// Set window size if specified
	_ = width
	_ = height
	_ = timeout

	// Process steps in order (inputs, expectations, waits interleaved)
	for stepIdx, step := range tc.Steps {
		switch step.Type {
		case StepInput:
			inp := tc.Inputs[step.InputIndex]
			input := inp.ToInput()
			msgs, err := input.ToMessages()
			if err != nil {
				result.Error = fmt.Sprintf("step %d (input): %v", stepIdx+1, err)
				return result
			}
			for _, msg := range msgs {
				if err := hm.SendMsg(msg); err != nil {
					result.Error = fmt.Sprintf("step %d (input): sending: %v", stepIdx+1, err)
					return result
				}
			}

		case StepExpect:
			exp := tc.Expects[step.ExpectIndex]
			if err := CheckExpectation(exp, hm); err != nil {
				result.Error = fmt.Sprintf("step %d (expect %s): %v", stepIdx+1, exp.Type, err)
				return result
			}

		case StepWait:
			// Handle wait actions (currently just skip - timer handling needs real time)
			_ = tc.Waits[step.WaitIndex]
		}
	}

	// All expectations passed
	result.Passed = true
	return result
}

// RunMultipleETFiles runs multiple .et test files and returns all results.
func RunMultipleETFiles(paths []string) []*TestResult {
	results := make([]*TestResult, len(paths))
	for i, path := range paths {
		results[i] = RunETFile(path)
	}
	return results
}

// RunETDirectory finds and runs all .et files in a directory.
func RunETDirectory(dir string) ([]*TestResult, error) {
	var paths []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".et" {
			paths = append(paths, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	return RunMultipleETFiles(paths), nil
}
