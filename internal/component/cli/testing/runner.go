// Design: docs/architecture/config/yang-config-design.md — editor test infrastructure

package testing

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
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

		// Guard against path traversal (e.g., "../../../etc/cron.d/malicious").
		if !strings.HasPrefix(filepath.Clean(filePath), filepath.Clean(tmpDir)+string(os.PathSeparator)) {
			result.Error = fmt.Sprintf("path traversal in tmpfs: %s", tf.Path)
			return result
		}

		// Create parent directories if needed
		if dir := filepath.Dir(filePath); dir != tmpDir {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				result.Error = fmt.Sprintf("creating dir for %s: %v", tf.Path, err)
				return result
			}
		}

		// Determine file mode
		mode := os.FileMode(0o600)
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
	reloadMode := ""         // "success", "fail", or "" (standalone)
	useHistoryStore := false // option=history:store -- persist history to zefs
	editorMode := "edit"     // option=mode:value=command -- command-only mode
	sessionUser := ""
	sessionOrigin := ""

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
		case "history":
			if _, ok := opt.Values["store"]; ok {
				useHistoryStore = true
			}
		case "mode":
			if val, ok := opt.Values["value"]; ok {
				editorMode = val
			}
		case "session":
			if user, ok := opt.Values["user"]; ok {
				sessionUser = user
			}
			if origin, ok := opt.Values["origin"]; ok {
				sessionOrigin = origin
			}
		}
	}

	// Create blob store for history persistence (if requested).
	// The store lives in tmpDir and persists across restart= steps.
	var historyStore *zefs.BlobStore
	if useHistoryStore {
		storePath := filepath.Join(tmpDir, "history.zefs")
		var storeErr error
		historyStore, storeErr = zefs.Create(storePath)
		if storeErr != nil {
			result.Error = fmt.Sprintf("creating history store: %v", storeErr)
			return result
		}
		defer historyStore.Close() //nolint:errcheck // test cleanup
	}

	// createModel builds a HeadlessModel based on the current mode.
	createModel := func() (*HeadlessModel, error) {
		if editorMode == "command" {
			return NewHeadlessCommandModel(), nil
		}
		if configPath == "" {
			return nil, fmt.Errorf("no config file specified (use option=file:path=...)")
		}
		if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
			return nil, fmt.Errorf("config file not found: %s", configPath)
		}
		if sessionUser != "" {
			return NewHeadlessModelWithSession(configPath, sessionUser, sessionOrigin)
		}
		return NewHeadlessModel(configPath)
	}

	// wireHistory sets up history persistence on the model.
	wireHistory := func(hm *HeadlessModel) {
		if historyStore != nil {
			hm.Model().SetHistory(cli.NewHistory(historyStore, "testuser"))
		}
	}

	hm, hmErr := createModel()
	if hmErr != nil {
		result.Error = fmt.Sprintf("creating editor: %v", hmErr)
		return result
	}
	hm.SetTmpDir(tmpDir)
	wireHistory(hm)

	// Multi-session map: session name -> headless model.
	// SEQUENTIAL: test steps run serially; no concurrent map access.
	sessions := map[string]*HeadlessModel{}

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

	// Process steps in order (inputs, expectations, waits, sessions interleaved)
	for stepIdx, step := range tc.Steps {
		switch step.Type {
		case StepSession:
			sa := tc.Sessions[step.SessionIndex]
			if sa.User != "" {
				// Create new session model.
				newHM, sessionErr := NewHeadlessModelWithSession(configPath, sa.User, sa.Origin)
				if sessionErr != nil {
					result.Error = fmt.Sprintf("step %d (session %s): %v", stepIdx+1, sa.Name, sessionErr)
					return result
				}
				newHM.SetTmpDir(tmpDir)
				sessions[sa.Name] = newHM
				hm = newHM
			} else {
				// Switch to existing session.
				existing, ok := sessions[sa.Name]
				if !ok {
					result.Error = fmt.Sprintf("step %d: unknown session %q", stepIdx+1, sa.Name)
					return result
				}
				hm = existing
			}

		case StepRestart:
			// Drain pending commands on the old model before replacing it,
			// so timer goroutines don't outlive the model.
			hm.SettleWait()
			// Simulate exit + relaunch: create a fresh headless model
			// from the same config file. The blob store persists, so
			// history is reloaded from zefs on the new model.
			newHM, restartErr := createModel()
			if restartErr != nil {
				result.Error = fmt.Sprintf("step %d (restart): %v", stepIdx+1, restartErr)
				return result
			}
			newHM.SetTmpDir(tmpDir)
			wireHistory(newHM)
			hm = newHM

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
			// Block until pending commands complete (file I/O that
			// exceeded the 15ms processCmdWithDepth timeout). Under
			// concurrent test load with race detector, this wait is
			// essential -- non-blocking Settle alone is insufficient.
			hm.SettleWait()
			exp := tc.Expects[step.ExpectIndex]
			if err := CheckExpectation(exp, hm); err != nil {
				// Command may still be in-flight under extreme load.
				// One more blocking settle as safety net.
				hm.SettleWait()
				if retryErr := CheckExpectation(exp, hm); retryErr != nil {
					result.Error = fmt.Sprintf("step %d (expect %s): %v", stepIdx+1, exp.Type, retryErr)
					return result
				}
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
