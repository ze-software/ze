// Design: docs/architecture/testing/ci-format.md -- web browser test runner
// Related: parser.go -- .wb file parsing
// Related: expect.go -- expectation checking

package webtesting

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/test/trace"
)

var errPressActionRequiresKeyParameter = errors.New("press action requires key= parameter")

// WBTestResult holds the outcome of a single .wb test.
type WBTestResult struct {
	Passed     bool
	Error      string
	Skipped    bool
	SkipReason string
	Steps      []trace.StepResult
}

// Browser wraps agent-browser CLI commands.
type Browser struct {
	baseURL       string
	daemonStarted bool
}

// NewBrowser creates a browser instance targeting the given base URL.
func NewBrowser(baseURL string) *Browser {
	return &Browser{baseURL: baseURL}
}

// Open navigates to baseURL + path.
func (b *Browser) Open(path string) error {
	url := b.baseURL + path
	if b.daemonStarted {
		if err := runAgent("open", url); err != nil {
			return fmt.Errorf("open %s: %w", url, err)
		}
	} else {
		if err := runAgentWithHTTPSIgnore("open", url); err != nil {
			return fmt.Errorf("open %s: %w", url, err)
		}
		b.daemonStarted = true
	}
	return b.WaitLoad()
}

// WaitLoad waits for in-flight fetch/XHR requests to drain. The finder UI
// keeps a persistent EventSource(/events) open, so `wait --load networkidle`
// never settles and burns a fixed ~1.44s on every call. Instead we poll an
// in-flight counter installed via AGENT_BROWSER_INIT_SCRIPTS (inflightInitJS)
// with `eval`, which returns instantly, under a hard wall-clock deadline.
// Polling from here (never a blocking `wait --fn`) means a request that never
// settles degrades to "proceed after the deadline" instead of hanging until
// the process is killed mid-command, which wedges the agent-browser daemon.
// Falls back to networkidle when the init script could not be written.
func (b *Browser) WaitLoad() error {
	if ensureInitScript() == "" {
		return runAgent("wait", "--load", "networkidle")
	}
	deadline := time.Now().Add(waitLoadDeadline)
	for {
		out, err := runAgentOutput("eval", inflightIdleExpr)
		if err == nil && strings.TrimSpace(out) == "true" {
			return nil
		}
		if !time.Now().Before(deadline) {
			// Proceed anyway: the explicit action=wait sleeps and the
			// subsequent expectation provide the real assertion.
			return nil
		}
		time.Sleep(waitLoadPoll)
	}
}

const (
	// waitLoadDeadline bounds WaitLoad so a request that never settles can
	// never hang the runner or wedge the daemon.
	waitLoadDeadline = 5 * time.Second
	// waitLoadPoll is the gap between eval polls of the in-flight predicate.
	waitLoadPoll = 40 * time.Millisecond
)

// WaitMs waits for a duration in milliseconds.
func (b *Browser) WaitMs(ms string) error {
	d, err := time.ParseDuration(ms + "ms")
	if err != nil {
		return fmt.Errorf("parse wait duration %q: %w", ms, err)
	}
	time.Sleep(d)
	return nil
}

// Snapshot returns the interactive accessibility snapshot.
func (b *Browser) Snapshot() (string, error) {
	return runAgentOutput("snapshot", "-i")
}

// FullSnapshot returns the full accessibility snapshot, including static text.
func (b *Browser) FullSnapshot() (string, error) {
	return runAgentOutput("snapshot")
}

// Press sends a key press (e.g., "Enter", "Tab", "Escape").
func (b *Browser) Press(key string) error {
	if err := runAgent("press", key); err != nil {
		return fmt.Errorf("press %s: %w", key, err)
	}
	return b.WaitLoad()
}

// PressOn finds an element by visible text, focuses it, and presses a key.
func (b *Browser) PressOn(text, key string) error {
	snap, err := b.Snapshot()
	if err != nil {
		return fmt.Errorf("snapshot before press: %w", err)
	}

	ref := findRefByText(snap, text)
	if ref == "" {
		return fmt.Errorf("no element with text containing %q for press", text)
	}

	if err := runAgent("focus", ref); err != nil {
		return fmt.Errorf("focus %s (text=%q): %w", ref, text, err)
	}

	if err := runAgent("press", key); err != nil {
		return fmt.Errorf("press %s on %s (text=%q): %w", key, ref, text, err)
	}
	return b.WaitLoad()
}

// PressOnID focuses an element by HTML id and presses a key.
func (b *Browser) PressOnID(id, key string) error {
	if err := runAgent("focus", "#"+id); err != nil {
		return fmt.Errorf("focus #%s: %w", id, err)
	}
	if err := runAgent("press", key); err != nil {
		return fmt.Errorf("press %s on #%s: %w", key, id, err)
	}
	return b.WaitLoad()
}

// Click finds an element by visible text in the snapshot, then clicks its @ref.
func (b *Browser) Click(text string) error {
	snap, err := b.Snapshot()
	if err != nil {
		return fmt.Errorf("snapshot before click: %w", err)
	}

	ref := findRefByText(snap, text)
	if ref == "" {
		return fmt.Errorf("no element with text containing %q in snapshot:\n%s", text, snap)
	}

	if err := runAgent("click", ref); err != nil {
		return fmt.Errorf("click %s (text=%q): %w", ref, text, err)
	}
	return b.WaitLoad()
}

// ClickID clicks an element by its HTML id attribute using a CSS selector.
func (b *Browser) ClickID(id string) error {
	if err := runAgent("click", "#"+id); err != nil {
		return fmt.Errorf("click #%s: %w", id, err)
	}
	return b.WaitLoad()
}

// Fill finds an input by placeholder/label text and fills it.
func (b *Browser) Fill(text, value string) error {
	snap, err := b.Snapshot()
	if err != nil {
		return fmt.Errorf("snapshot before fill: %w", err)
	}

	ref := findRefByText(snap, text)
	if ref == "" {
		return fmt.Errorf("no input with text containing %q in snapshot:\n%s", text, snap)
	}

	return runAgent("fill", ref, value)
}

// FillID fills an input by its HTML id attribute using a CSS selector.
func (b *Browser) FillID(id, value string) error {
	return runAgent("fill", "#"+id, value)
}

// Hover finds an element by text and hovers.
func (b *Browser) Hover(text string) error {
	snap, err := b.Snapshot()
	if err != nil {
		return fmt.Errorf("snapshot before hover: %w", err)
	}

	ref := findRefByText(snap, text)
	if ref == "" {
		return fmt.Errorf("no element with text containing %q", text)
	}

	return runAgent("hover", ref)
}

// HoverID hovers an element by its HTML id attribute using a CSS selector.
func (b *Browser) HoverID(id string) error {
	return runAgent("hover", "#"+id)
}

// Screenshot saves a screenshot to the given path.
func (b *Browser) Screenshot(path string) error {
	return runAgent("screenshot", path)
}

// GetText returns the full page text.
func (b *Browser) GetText() (string, error) {
	return runAgentOutput("get", "text", "body")
}

// GetHTML returns the full page HTML.
func (b *Browser) GetHTML() (string, error) {
	return runAgentOutput("get", "html", "body")
}

// Close closes the browser.
func (b *Browser) Close() {
	_ = runAgentWithHTTPSIgnore("close", "--all")
	b.daemonStarted = false
}

// findRefByText searches the snapshot output for a line containing the text
// (case-insensitive) and extracts the ref=eN value.
func findRefByText(snapshot, text string) string {
	textLower := strings.ToLower(text)
	for line := range strings.SplitSeq(snapshot, "\n") {
		if strings.Contains(strings.ToLower(line), textLower) {
			if _, after, ok := strings.Cut(line, "ref="); ok {
				end := strings.IndexAny(after, "],")
				if end < 0 {
					end = len(after)
				}
				return "@" + strings.TrimSpace(after[:end])
			}
		}
	}
	return ""
}

const agentBrowserBin = "agent-browser"

// agentTimeout is the default timeout for agent-browser commands.
var agentTimeout = 30 * time.Second

// runAgent executes agent-browser with the given arguments.
func runAgent(args ...string) error {
	return runAgentWithEnv(nil, args...)
}

// runAgentWithHTTPSIgnore executes agent-browser with the HTTPS-ignore global
// flag. Use it only for session cleanup/startup; agent-browser warns if the
// flag is repeated once the daemon is already running.
func runAgentWithHTTPSIgnore(args ...string) error {
	return runAgentWithEnv([]string{"--ignore-https-errors"}, args...)
}

func runAgentWithEnv(globalArgs []string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), agentTimeout)
	defer cancel()

	if len(globalArgs) > 0 {
		args = append(append([]string{}, globalArgs...), args...)
	}
	cmd := exec.CommandContext(ctx, agentBrowserBin, args...) //nolint:gosec // args are test-controlled, not user input
	cmd.Env = agentEnv()
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runAgentOutput executes agent-browser and returns stdout.
func runAgentOutput(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), agentTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, agentBrowserBin, args...) //nolint:gosec // args are test-controlled, not user input
	cmd.Env = agentEnv()
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// inflightInitJS instruments fetch and XHR with an in-flight counter,
// registered as an agent-browser init script so it re-runs on every
// navigation. It deliberately leaves EventSource alone: the finder UI's
// persistent /events connection is what stops networkidle from ever
// settling, and it must not count toward "requests in flight".
const inflightInitJS = `(function () {
  if (window.__zeWaitInstalled) return;
  window.__zeWaitInstalled = true;
  window.__zeInflight = 0;
  window.__zeLastChange = performance.now();
  function mark() { window.__zeLastChange = performance.now(); }
  var origFetch = window.fetch;
  if (origFetch) {
    window.fetch = function () {
      window.__zeInflight++; mark();
      return origFetch.apply(this, arguments).finally(function () { window.__zeInflight--; mark(); });
    };
  }
  var origSend = XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.send = function () {
    window.__zeInflight++; mark();
    this.addEventListener('loadend', function () { window.__zeInflight--; mark(); });
    return origSend.apply(this, arguments);
  };
})();
`

// inflightIdleExpr is the predicate WaitLoad polls: no in-flight fetch/XHR
// and quiet for a short debounce. The debounce bridges chained requests --
// cli.js issues fetch(/cli) -> htmx refresh -> fetch(/config/changes) in
// sequence, so the counter dips to zero for a microtask between them.
const inflightIdleExpr = `(window.__zeInflight||0)===0 && (performance.now()-(window.__zeLastChange||0))>=120`

var (
	initScriptOnce sync.Once
	initScriptPath string
)

// ensureInitScript writes the in-flight instrumentation to a temp file once
// and returns its path, or "" if it could not be written (WaitLoad then
// falls back to networkidle). agent-browser loads it via the
// AGENT_BROWSER_INIT_SCRIPTS env set in agentEnv.
func ensureInitScript() string {
	initScriptOnce.Do(func() {
		f, err := os.CreateTemp("", "ze-inflight-*.js")
		if err != nil {
			return
		}
		_, werr := f.WriteString(inflightInitJS)
		cerr := f.Close()
		if werr != nil || cerr != nil {
			return
		}
		initScriptPath = f.Name()
	})
	return initScriptPath
}

func agentEnv() []string {
	env := os.Environ()
	hasIdle, hasInit := false, false
	for _, e := range env {
		if strings.HasPrefix(e, "AGENT_BROWSER_IDLE_TIMEOUT_MS=") {
			hasIdle = true
		}
		if strings.HasPrefix(e, "AGENT_BROWSER_INIT_SCRIPTS=") {
			hasInit = true
		}
	}
	if !hasIdle {
		env = append(env, "AGENT_BROWSER_IDLE_TIMEOUT_MS=60000")
	}
	if !hasInit {
		if p := ensureInitScript(); p != "" {
			env = append(env, "AGENT_BROWSER_INIT_SCRIPTS="+p)
		}
	}
	return env
}

// RunWBFile parses and executes a .wb test file.
func RunWBFile(path, baseURL string) *WBTestResult {
	content, err := os.ReadFile(path) //nolint:gosec // test file path from controlled test discovery
	if err != nil {
		return &WBTestResult{Error: fmt.Sprintf("read %s: %v", path, err)}
	}

	tc, err := ParseWBFile(string(content))
	if err != nil {
		return &WBTestResult{Error: fmt.Sprintf("parse %s: %v", path, err)}
	}

	if tc.SkipReason != "" {
		return &WBTestResult{Passed: true, Skipped: true, SkipReason: tc.SkipReason}
	}

	return runWBTestCase(tc, baseURL)
}

func runWBTestCase(tc *WBTestCase, baseURL string) *WBTestResult {
	// Each test gets a fresh browser session.
	_ = runAgentWithHTTPSIgnore("close", "--all")

	browser := NewBrowser(baseURL)
	var steps []trace.StepResult

	for i, step := range tc.Steps {
		switch step.Type {
		case WBStepAction:
			a := tc.Actions[step.ActionIndex]
			err := executeAction(browser, &a)
			steps = append(steps, trace.StepResult{
				Step: i + 1, Line: a.Line,
				Kind: "action", Assert: a.Kind,
				Passed: err == nil, Detail: trace.ErrString(err),
			})
			if err != nil {
				return &WBTestResult{
					Error: "line " + strconv.Itoa(a.Line) + ": action " + a.Kind + ": " + err.Error(),
					Steps: steps,
				}
			}
		case WBStepExpect:
			e := tc.Expects[step.ExpectIndex]
			err := checkExpectation(browser, &e)
			steps = append(steps, trace.StepResult{
				Step: i + 1, Line: e.Line,
				Kind: "expect", Assert: e.Kind,
				Passed: err == nil, Detail: trace.ErrString(err),
			})
			if err != nil {
				return &WBTestResult{
					Error: "line " + strconv.Itoa(e.Line) + ": expect " + e.Kind + ": " + err.Error(),
					Steps: steps,
				}
			}
		}
	}

	return &WBTestResult{Passed: true, Steps: steps}
}

func executeAction(b *Browser, a *WBAction) error {
	switch a.Kind {
	case "open":
		return b.Open(a.Values["path"])
	case "click":
		if id, ok := a.Values["id"]; ok {
			return b.ClickID(id)
		}
		return b.Click(a.Values["text"])
	case "fill":
		if id, ok := a.Values["id"]; ok {
			return b.FillID(id, a.Values["value"])
		}
		return b.Fill(a.Values["text"], a.Values["value"])
	case "hover":
		if id, ok := a.Values["id"]; ok {
			return b.HoverID(id)
		}
		return b.Hover(a.Values["text"])
	case "wait":
		if ms, ok := a.Values["ms"]; ok {
			return b.WaitMs(ms)
		}
		return b.WaitLoad()
	case "press":
		key := a.Values["key"]
		if key == "" {
			return errPressActionRequiresKeyParameter
		}
		if id, ok := a.Values["id"]; ok {
			return b.PressOnID(id, key)
		}
		if text, ok := a.Values["text"]; ok {
			return b.PressOn(text, key)
		}
		return b.Press(key)
	case "screenshot":
		return b.Screenshot(a.Values["file"])
	}
	return fmt.Errorf("unknown action kind %q", a.Kind)
}
