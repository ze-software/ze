// Design: docs/architecture/config/yang-config-design.md — editor test infrastructure

package testing

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/test/trace"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

var (
	errNoConfigFileSpecifiedUseOption = errors.New("no config file specified (use option=file:path=...)")
	errDaemonNotReachable             = errors.New("daemon not reachable")
)

// TestResult represents the outcome of running an .et test.
type TestResult struct {
	Passed   bool   // Whether all expectations passed
	Error    string // Error message if failed
	TempDir  string // Temp directory used (empty if cleaned up)
	Duration time.Duration
	Steps    []trace.StepResult
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
	result.Steps = runResult.Steps
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
			result.Error = "path traversal in tmpfs: " + tf.Path
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
	lifecycleMode := ""      // "wired" = mock shutdown/restart callbacks
	useHistoryStore := false // option=history:store -- persist history to zefs
	editorMode := "config"   // option=mode:value=operational -- operational-only mode
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
		case "lifecycle":
			if mode, ok := opt.Values["mode"]; ok {
				lifecycleMode = mode
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
		if editorMode == "operational" || editorMode == "command" {
			return NewHeadlessCommandModel(), nil
		}
		if configPath == "" {
			return nil, errNoConfigFileSpecifiedUseOption
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
		hm.SetReloadNotifier(func() error { return errDaemonNotReachable })
	}

	// Configure mock lifecycle callbacks (shutdown/restart) if requested.
	// Assign directly to model field via Model() pointer.
	if lifecycleMode == "wired" {
		m := hm.Model()
		m.SetShutdownFunc(func() {})
		m.SetRestartFunc(func() {})
	}

	// Set window size if specified
	_ = width
	_ = height
	_ = timeout

	// Process steps in order (inputs, expectations, waits, sessions interleaved)
	for stepIdx, step := range tc.Steps {
		stepNum := stepIdx + 1
		switch step.Type {
		case StepSession:
			sa := tc.Sessions[step.SessionIndex]
			if sa.User != "" {
				newHM, sessionErr := NewHeadlessModelWithSession(configPath, sa.User, sa.Origin)
				result.Steps = append(result.Steps, trace.StepResult{
					Step: stepNum, Kind: "session", Assert: sa.Name,
					Passed: sessionErr == nil, Detail: trace.ErrString(sessionErr),
				})
				if sessionErr != nil {
					result.Error = "step " + strconv.Itoa(stepNum) + " (session " + sa.Name + "): " + sessionErr.Error()
					return result
				}
				newHM.SetTmpDir(tmpDir)
				sessions[sa.Name] = newHM
				hm = newHM
			} else {
				existing, ok := sessions[sa.Name]
				if !ok {
					result.Steps = append(result.Steps, trace.StepResult{
						Step: stepNum, Kind: "session", Assert: sa.Name,
						Passed: false, Detail: "unknown session",
					})
					result.Error = "step " + strconv.Itoa(stepNum) + ": unknown session " + sa.Name
					return result
				}
				result.Steps = append(result.Steps, trace.StepResult{
					Step: stepNum, Kind: "session", Assert: sa.Name, Passed: true,
				})
				hm = existing
			}

		case StepRestart:
			// Drain pending commands on the old model before replacing it,
			// so timer goroutines don't outlive the model.
			hm.SettleWait()
			newHM, restartErr := createModel()
			result.Steps = append(result.Steps, trace.StepResult{
				Step: stepNum, Kind: "restart",
				Passed: restartErr == nil, Detail: trace.ErrString(restartErr),
			})
			if restartErr != nil {
				result.Error = "step " + strconv.Itoa(stepNum) + " (restart): " + restartErr.Error()
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
				result.Steps = append(result.Steps, trace.StepResult{
					Step: stepNum, Kind: "input", Assert: inp.Action,
					Passed: false, Detail: err.Error(),
				})
				result.Error = "step " + strconv.Itoa(stepNum) + " (input): " + err.Error()
				return result
			}
			var sendErr error
			for _, msg := range msgs {
				if sendErr = hm.SendMsg(msg); sendErr != nil {
					break
				}
			}
			result.Steps = append(result.Steps, trace.StepResult{
				Step: stepNum, Kind: "input", Assert: inp.Action,
				Passed: sendErr == nil, Detail: trace.ErrString(sendErr),
			})
			if sendErr != nil {
				result.Error = "step " + strconv.Itoa(stepNum) + " (input): sending: " + sendErr.Error()
				return result
			}

		case StepExpect:
			// Block until pending commands complete (file I/O that
			// exceeded the 900ms processCmdWithDepth timeout). Under
			// concurrent test load with race detector, this wait is
			// essential -- non-blocking Settle alone is insufficient.
			//
			// Multiple retries handle cascading pending items: draining
			// one snapshot may spawn new commands (e.g., command result
			// triggers status update) that need a subsequent SettleWait.
			exp := tc.Expects[step.ExpectIndex]
			var lastErr error
			for attempt := range 5 {
				hm.SettleWait()
				lastErr = CheckExpectation(exp, hm)
				if lastErr == nil {
					break
				}
				_ = attempt
			}
			result.Steps = append(result.Steps, trace.StepResult{
				Step: stepNum, Kind: "expect", Assert: exp.Type,
				Passed: lastErr == nil, Detail: trace.ErrString(lastErr),
			})
			if lastErr != nil {
				result.Error = "step " + strconv.Itoa(stepNum) + " (expect " + exp.Type + "): " + lastErr.Error()
				return result
			}

		case StepWait:
			// Handle wait actions (currently just skip - timer handling needs real time)
			_ = tc.Waits[step.WaitIndex]
			result.Steps = append(result.Steps, trace.StepResult{
				Step: stepNum, Kind: "wait", Passed: true,
			})
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
