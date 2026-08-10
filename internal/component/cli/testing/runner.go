// Design: docs/architecture/config/yang-config-design.md — editor test infrastructure
// Related: fake_monitor.go -- option=monitor:ping=fake deterministic monitor fakes

package testing

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/trace"
	"github.com/ze-software/ze/pkg/zefs"
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

// runETTest parses and executes an .et test from content string.
// Returns a TestResult with pass/fail status and any error message.
func runETTest(content string) *TestResult {
	start := time.Now()
	result := &TestResult{}

	// Parse the .et content
	tc, err := parseETFile(content)
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
	return runETTest(string(content))
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
			var tb textbuf.Buffer
			result.Error = tb.Str("path traversal in tmpfs: ").Str(tf.Path).String()
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
	monitorPing := ""        // option=monitor:ping=fake -- deterministic ping factory + resolvers
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
		case "monitor":
			if ping, ok := opt.Values["ping"]; ok {
				monitorPing = ping
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
	createModel := func() (*headlessModel, error) {
		if editorMode == "operational" || editorMode == "command" {
			return newHeadlessCommandModel(), nil
		}
		if configPath == "" {
			return nil, errNoConfigFileSpecifiedUseOption
		}
		if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
			return nil, fmt.Errorf("config file not found: %s", configPath)
		}
		if sessionUser != "" {
			return newHeadlessModelWithSession(configPath, sessionUser, sessionOrigin)
		}
		return newHeadlessModel(configPath)
	}

	// wireHistory sets up history persistence on the model.
	wireHistory := func(hm *headlessModel) {
		if historyStore != nil {
			hm.Model().SetHistory(cli.NewHistory(historyStore, "testuser"))
		}
	}

	hm, hmErr := createModel()
	if hmErr != nil {
		result.Error = fmt.Sprintf("creating editor: %v", hmErr)
		return result
	}
	hm.setTmpDir(tmpDir)
	wireHistory(hm)

	// Multi-session map: session name -> headless model.
	// SEQUENTIAL: test steps run serially; no concurrent map access.
	sessions := map[string]*headlessModel{}

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

	// Configure deterministic monitor ping factory + resolvers if requested.
	if monitorPing == "fake" {
		wireFakePingMonitor(hm)
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
				newHM, sessionErr := newHeadlessModelWithSession(configPath, sa.User, sa.Origin)
				result.Steps = append(result.Steps, trace.StepResult{
					Step: stepNum, Kind: "session", Assert: sa.Name,
					Passed: sessionErr == nil, Detail: trace.ErrString(sessionErr),
				})
				if sessionErr != nil {
					var tb textbuf.Buffer
					result.Error = tb.Str("step ").Int(int64(stepNum)).Str(" (session ").Str(sa.Name).Str("): ").Err(sessionErr).String()
					return result
				}
				newHM.setTmpDir(tmpDir)
				sessions[sa.Name] = newHM
				hm = newHM
			} else {
				existing, ok := sessions[sa.Name]
				if !ok {
					result.Steps = append(result.Steps, trace.StepResult{
						Step: stepNum, Kind: "session", Assert: sa.Name,
						Passed: false, Detail: "unknown session",
					})
					var tb textbuf.Buffer
					result.Error = tb.Str("step ").Int(int64(stepNum)).Str(": unknown session ").Str(sa.Name).String()
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
			hm.settleWait()
			newHM, restartErr := createModel()
			result.Steps = append(result.Steps, trace.StepResult{
				Step: stepNum, Kind: "restart",
				Passed: restartErr == nil, Detail: trace.ErrString(restartErr),
			})
			if restartErr != nil {
				var tb textbuf.Buffer
				result.Error = tb.Str("step ").Int(int64(stepNum)).Str(" (restart): ").Err(restartErr).String()
				return result
			}
			newHM.setTmpDir(tmpDir)
			wireHistory(newHM)
			hm = newHM

		case StepInput:
			inp := tc.Inputs[step.InputIndex]
			input := inp.toInput()
			msgs, err := input.toMessages()
			if err != nil {
				result.Steps = append(result.Steps, trace.StepResult{
					Step: stepNum, Kind: "input", Assert: inp.Action,
					Passed: false, Detail: err.Error(),
				})
				var tb textbuf.Buffer
				result.Error = tb.Str("step ").Int(int64(stepNum)).Str(" (input): ").Err(err).String()
				return result
			}
			var sendErr error
			for _, msg := range msgs {
				if sendErr = hm.sendMsg(msg); sendErr != nil {
					break
				}
			}
			result.Steps = append(result.Steps, trace.StepResult{
				Step: stepNum, Kind: "input", Assert: inp.Action,
				Passed: sendErr == nil, Detail: trace.ErrString(sendErr),
			})
			if sendErr != nil {
				var tb textbuf.Buffer
				result.Error = tb.Str("step ").Int(int64(stepNum)).Str(" (input): sending: ").Err(sendErr).String()
				return result
			}

		case StepExpect:
			// SettleWait is the barrier: it blocks until every command in
			// flight (file I/O that exceeded the 900ms processCmdWithDepth
			// deadline and was handed to the pending set) has completed and
			// been applied, so the assertion below cannot race the edit it is
			// checking.
			//
			// Draining one generation can schedule the next (a command
			// result that triggers a status update), so re-drain while the
			// assertion fails and work remains in flight. The bound is on
			// generations of cascaded commands, never on wall-clock time,
			// so a slow machine changes how long each drain takes but not
			// whether the assertion sees settled state. Once nothing is
			// pending, no further drain can change the answer -- stop and
			// report the failure instead of spinning.
			exp := tc.Expects[step.ExpectIndex]
			var lastErr error
			for range 5 {
				hm.settleWait()
				lastErr = checkExpectation(exp, hm)
				if lastErr == nil || !hm.hasPending() {
					break
				}
			}
			result.Steps = append(result.Steps, trace.StepResult{
				Step: stepNum, Kind: "expect", Assert: exp.Type,
				Passed: lastErr == nil, Detail: trace.ErrString(lastErr),
			})
			if lastErr != nil {
				var tb textbuf.Buffer
				result.Error = tb.Str("step ").Int(int64(stepNum)).Str(" (expect ").Str(exp.Type).Str("): ").Err(lastErr).String()
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
