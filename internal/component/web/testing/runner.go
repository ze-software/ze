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

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/trace"
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
	session       string
	daemonStarted bool
}

// NewBrowser creates a browser instance targeting the given base URL.
func NewBrowser(baseURL string) *Browser {
	return &Browser{baseURL: baseURL}
}

// NewBrowserWithSession creates a browser bound to an isolated agent-browser session.
func NewBrowserWithSession(baseURL, session string) *Browser {
	return &Browser{baseURL: baseURL, session: session}
}

// Open navigates to baseURL + path.
func (b *Browser) Open(path string) error {
	var tb textbuf.Buffer
	url := tb.Str(b.baseURL).Str(path).String()
	if err := b.runAgentEnsureDaemon("open", url); err != nil {
		return fmt.Errorf("open %s: %w", url, err)
	}
	return b.WaitLoad()
}

// runAgentEnsureDaemon runs an agent-browser command, guaranteeing the daemon is
// started with --ignore-https-errors. That flag is honored only at daemon
// launch, so it must ride whichever command spawns the daemon -- open, set
// viewport, or set headers, whichever the test issues first. Once the daemon is
// running the flag is a no-op and is dropped. The latch flips only after a
// successful start, so a failed launch still retries with the flag. Routing
// every pre-navigation command through here is what stops a preceding
// option=viewport / option=locale from starting the daemon WITHOUT cert-ignore
// (which surfaced as ERR_CERT_AUTHORITY_INVALID on the following open).
func (b *Browser) runAgentEnsureDaemon(args ...string) error {
	if b.daemonStarted {
		return b.runAgent(args...)
	}
	if err := b.runAgentWithHTTPSIgnore(args...); err != nil {
		return err
	}
	b.daemonStarted = true
	return nil
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
		return b.runAgent("wait", "--load", "networkidle")
	}
	deadline := time.Now().Add(waitLoadDeadline)
	for {
		out, err := b.runAgentOutput("eval", inflightIdleExpr)
		if err == nil && strings.TrimSpace(out) == "true" {
			return nil
		}
		if !time.Now().Before(deadline) {
			return nil
		}
		time.Sleep(waitLoadPoll)
	}
}

const (
	waitLoadDeadline = 5 * time.Second
	waitLoadPoll     = 40 * time.Millisecond
)

// WaitMs waits for a duration in milliseconds.
func (b *Browser) WaitMs(ms string) error {
	var tb textbuf.Buffer
	d, err := time.ParseDuration(tb.Str(ms).Str("ms").Slice())
	if err != nil {
		return fmt.Errorf("parse wait duration %q: %w", ms, err)
	}
	time.Sleep(d)
	return nil
}

// SetViewport resizes the browser viewport (agent-browser set viewport <w> <h>)
// so mobile-layout assertions run at the requested size. Applied before the
// first navigation.
func (b *Browser) SetViewport(width, height int) error {
	var tb textbuf.Buffer
	w := tb.Int(int64(width)).String()
	h := tb.Reset().Int(int64(height)).String()
	return b.runAgentEnsureDaemon("set", "viewport", w, h)
}

// SetLocale sets the browser Accept-Language header (agent-browser set headers
// <json>) so the UI renders under the given locale for the rest of the session.
func (b *Browser) SetLocale(lang string) error {
	var tb textbuf.Buffer
	hdr := tb.Str(`{"Accept-Language":"`).Str(lang).Str(`"}`).String()
	return b.runAgentEnsureDaemon("set", "headers", hdr)
}

// Login drives the login form: it navigates to the root (which renders the
// login form when the session is unauthenticated), fills the credentials, and
// submits. Used by the `action=login:user=..:password=..` directive to exercise
// role-gated pages.
func (b *Browser) Login(user, password string) error {
	if err := b.Open("/"); err != nil {
		return err
	}
	if err := b.FillID("username", user); err != nil {
		return err
	}
	if err := b.FillID("password", password); err != nil {
		return err
	}
	if err := b.Press("Enter"); err != nil {
		return err
	}
	return b.WaitLoad()
}

// Snapshot returns the interactive accessibility snapshot.
func (b *Browser) Snapshot() (string, error) {
	return b.runAgentOutput("snapshot", "-i")
}

// FullSnapshot returns the full accessibility snapshot, including static text.
func (b *Browser) FullSnapshot() (string, error) {
	return b.runAgentOutput("snapshot")
}

// Press sends a key press (e.g., "Enter", "Tab", "Escape").
func (b *Browser) Press(key string) error {
	if err := b.runAgent("press", key); err != nil {
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

	if err := b.runAgent("focus", ref); err != nil {
		return fmt.Errorf("focus %s (text=%q): %w", ref, text, err)
	}

	if err := b.runAgent("press", key); err != nil {
		return fmt.Errorf("press %s on %s (text=%q): %w", key, ref, text, err)
	}
	return b.WaitLoad()
}

// PressOnID focuses an element by HTML id and presses a key.
func (b *Browser) PressOnID(id, key string) error {
	var tb textbuf.Buffer
	sel := tb.Byte('#').Str(id).String()
	if err := b.runAgent("focus", sel); err != nil {
		return fmt.Errorf("focus #%s: %w", id, err)
	}
	if err := b.runAgent("press", key); err != nil {
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

	if err := b.runAgent("click", ref); err != nil {
		return fmt.Errorf("click %s (text=%q): %w", ref, text, err)
	}
	return b.WaitLoad()
}

// ClickID clicks an element by its HTML id attribute using a CSS selector.
func (b *Browser) ClickID(id string) error {
	var tb textbuf.Buffer
	sel := tb.Byte('#').Str(id).String()
	if err := b.runAgent("click", sel); err != nil {
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

	return b.runAgent("fill", ref, value)
}

// FillID fills an input by its HTML id attribute using a CSS selector.
func (b *Browser) FillID(id, value string) error {
	var tb textbuf.Buffer
	sel := tb.Byte('#').Str(id).String()
	return b.runAgent("fill", sel, value)
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

	return b.runAgent("hover", ref)
}

// HoverID hovers an element by its HTML id attribute using a CSS selector.
func (b *Browser) HoverID(id string) error {
	var tb textbuf.Buffer
	sel := tb.Byte('#').Str(id).String()
	return b.runAgent("hover", sel)
}

// Screenshot saves a screenshot to the given path.
func (b *Browser) Screenshot(path string) error {
	return b.runAgent("screenshot", path)
}

// GetText returns the full page text.
func (b *Browser) GetText() (string, error) {
	return b.runAgentOutput("get", "text", "body")
}

// GetHTML returns the full page HTML.
func (b *Browser) GetHTML() (string, error) {
	return b.runAgentOutput("get", "html", "body")
}

// Close closes the browser session. When the browser is bound to a session,
// only that session is closed. Without a session, all sessions are closed.
func (b *Browser) Close() {
	if b.session != "" {
		_ = b.runAgentWithHTTPSIgnore("close")
	} else {
		_ = b.runAgentWithHTTPSIgnore("close", "--all")
	}
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
				var tb textbuf.Buffer
				return tb.Byte('@').Str(strings.TrimSpace(after[:end])).String()
			}
		}
	}
	return ""
}

const agentBrowserBin = "agent-browser"

// agentTimeout is the default timeout for agent-browser commands.
var agentTimeout = 30 * time.Second

func (b *Browser) runAgent(args ...string) error {
	return b.runAgentCore(nil, args...)
}

func (b *Browser) runAgentWithHTTPSIgnore(args ...string) error {
	return b.runAgentCore([]string{"--ignore-https-errors"}, args...)
}

func (b *Browser) runAgentCore(globalArgs []string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), agentTimeout)
	defer cancel()

	if len(globalArgs) > 0 {
		args = append(append([]string{}, globalArgs...), args...)
	}
	cmd := exec.CommandContext(ctx, agentBrowserBin, args...) //nolint:gosec // args are test-controlled, not user input
	cmd.Env = b.agentEnv()
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (b *Browser) runAgentOutput(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), agentTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, agentBrowserBin, args...) //nolint:gosec // args are test-controlled, not user input
	cmd.Env = b.agentEnv()
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

func (b *Browser) agentEnv() []string {
	env := os.Environ()
	hasIdle, hasInit, hasSession := false, false, false
	for _, e := range env {
		if strings.HasPrefix(e, "AGENT_BROWSER_IDLE_TIMEOUT_MS=") {
			hasIdle = true
		}
		if strings.HasPrefix(e, "AGENT_BROWSER_INIT_SCRIPTS=") {
			hasInit = true
		}
		if strings.HasPrefix(e, "AGENT_BROWSER_SESSION=") {
			hasSession = true
		}
	}
	var tb textbuf.Buffer
	if !hasIdle {
		env = append(env, "AGENT_BROWSER_IDLE_TIMEOUT_MS=60000")
	}
	if !hasInit {
		if p := ensureInitScript(); p != "" {
			env = append(env, tb.Str("AGENT_BROWSER_INIT_SCRIPTS=").Str(p).String())
		}
	}
	if b.session != "" && !hasSession {
		env = append(env, tb.Reset().Str("AGENT_BROWSER_SESSION=").Str(b.session).String())
	}
	return env
}

// RunWBFileWithSession parses and executes a .wb test file within
// an isolated agent-browser session.
func RunWBFileWithSession(path, baseURL, session string) *WBTestResult {
	content, err := os.ReadFile(path) //nolint:gosec // test file path from controlled test discovery
	if err != nil {
		var tb textbuf.Buffer
		return &WBTestResult{Error: tb.Str("read ").Str(path).Str(": ").Err(err).String()}
	}

	tc, err := ParseWBFile(string(content))
	if err != nil {
		var tb textbuf.Buffer
		return &WBTestResult{Error: tb.Str("parse ").Str(path).Str(": ").Err(err).String()}
	}

	if tc.SkipReason != "" {
		return &WBTestResult{Passed: true, Skipped: true, SkipReason: tc.SkipReason}
	}

	return runWBTestCase(tc, baseURL, session)
}

func runWBTestCase(tc *WBTestCase, baseURL, session string) *WBTestResult {
	var browser *Browser
	if session != "" {
		browser = NewBrowserWithSession(baseURL, session)
	} else {
		browser = NewBrowser(baseURL)
	}
	// Free this test's browser session when it finishes. Sessions are keyed per
	// test (unique nick), so without a per-test close they accumulate in the
	// shared agent-browser daemon for the whole suite -- 80+ live pages that
	// starve later tests of resources and surface as flaky empty snapshots. The
	// suite-end sweep is a backstop, not a substitute.
	//
	// There is deliberately NO leading Close() here. A close immediately followed
	// by the first Open() on the same session makes agent-browser tear down and
	// relaunch that session's daemon back-to-back; under concurrency the relaunch
	// races the navigation and the page ends up stuck on about:blank ("(empty
	// page)" in snapshots). The session nick is unique per test, so the context is
	// already pristine -- the first Open() navigates a clean page with no prior
	// close needed. (Verified: with the leading close, ~10/12 concurrent opens
	// landed on about:blank; without it, 12/12 navigated correctly.)
	defer browser.Close()

	var (
		steps []trace.StepResult
		tb    textbuf.Buffer
	)

	// Apply session-level options before the first navigation: a mobile
	// viewport and/or an Accept-Language locale carry through every open.
	if tc.Viewport.Width > 0 && tc.Viewport.Height > 0 {
		if err := browser.SetViewport(tc.Viewport.Width, tc.Viewport.Height); err != nil {
			return &WBTestResult{Error: tb.Str("set viewport: ").Err(err).String()}
		}
	}
	if tc.Locale != "" {
		if err := browser.SetLocale(tc.Locale); err != nil {
			return &WBTestResult{Error: tb.Reset().Str("set locale: ").Err(err).String()}
		}
	}

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
					Error: tb.Reset().Str("line ").Str(strconv.Itoa(a.Line)).Str(": action ").Str(a.Kind).Str(": ").Str(err.Error()).String(),
					Steps: steps,
				}
			}
		case WBStepExpect:
			e := tc.Expects[step.ExpectIndex]
			err := checkExpectationRetry(browser, &e)
			steps = append(steps, trace.StepResult{
				Step: i + 1, Line: e.Line,
				Kind: "expect", Assert: e.Kind,
				Passed: err == nil, Detail: trace.ErrString(err),
			})
			if err != nil {
				return &WBTestResult{
					Error: tb.Reset().Str("line ").Str(strconv.Itoa(e.Line)).Str(": expect ").Str(e.Kind).Str(": ").Str(err.Error()).String(),
					Steps: steps,
				}
			}
		}
	}

	return &WBTestResult{Passed: true, Steps: steps}
}

const (
	// Generous: under parallel CPU load an HTMX/JS render can land seconds after
	// WaitLoad reports the network idle, so the assertion must out-wait the slow
	// render without out-waiting the test's own option=timeout.
	expectRetryDeadline = 5 * time.Second
	// Poll gently: each retry re-snapshots via agent-browser subprocesses, so a
	// tight interval would itself load the shared daemon under parallel runs.
	expectRetryPoll = 250 * time.Millisecond
)

// checkExpectationRetry polls an expectation until it passes or a short deadline
// elapses. A browser snapshot is a point-in-time read: under parallel CPU load a
// page's HTMX/JS render can land a few hundred milliseconds after WaitLoad reports
// the network idle, so a single snapshot occasionally catches a half-rendered
// ("empty") page. Retrying converts that race into a bounded wait -- the standard
// auto-waiting-assertion pattern -- so a correct page is never failed for being
// briefly late, while a genuinely wrong page still fails after the deadline.
func checkExpectationRetry(b *Browser, e *WBExpectation) error {
	deadline := time.Now().Add(expectRetryDeadline)
	for {
		err := checkExpectation(b, e)
		if err == nil {
			return nil
		}
		if !time.Now().Before(deadline) {
			return err
		}
		time.Sleep(expectRetryPoll)
	}
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
	case "login":
		return b.Login(a.Values["user"], a.Values["password"])
	}
	return fmt.Errorf("unknown action kind %q", a.Kind)
}
