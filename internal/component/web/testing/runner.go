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
	"github.com/ze-software/ze/internal/test/runner"
	"github.com/ze-software/ze/internal/test/trace"
)

var (
	errPressActionRequiresKeyParameter  = errors.New("press action requires key= parameter")
	errWaitUntilRequiresPathAndContains = errors.New("wait-until action requires path= and contains= parameters")
)

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

// newBrowser creates a browser instance targeting the given base URL.
func newBrowser(baseURL string) *Browser {
	return &Browser{baseURL: baseURL}
}

// newBrowserWithSession creates a browser bound to an isolated agent-browser session.
func newBrowserWithSession(baseURL, session string) *Browser {
	return &Browser{baseURL: baseURL, session: session}
}

// Open navigates to baseURL + path.
func (b *Browser) Open(path string) error {
	var tb textbuf.Buffer
	url := tb.Str(b.baseURL).Str(path).String()
	if err := b.runAgentEnsureDaemon("open", url); err != nil {
		return fmt.Errorf("open %s: %w", url, err)
	}
	return b.waitLoad()
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

// waitLoad waits for in-flight fetch/XHR requests to drain. The finder UI
// keeps a persistent EventSource(/events) open, so `wait --load networkidle`
// never settles and burns a fixed ~1.44s on every call. Instead we poll an
// in-flight counter installed via AGENT_BROWSER_INIT_SCRIPTS (inflightInitJS)
// with `eval`, which returns instantly, under a hard wall-clock deadline.
// Polling from here (never a blocking `wait --fn`) means a request that never
// settles degrades to "proceed after the deadline" instead of hanging until
// the process is killed mid-command, which wedges the agent-browser daemon.
// Falls back to networkidle when the init script could not be written.
func (b *Browser) waitLoad() error {
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

// browserIdleWindow is how long the agent-browser daemon lives with no command.
// agentEnv sets it, and waitMs works around it: the two must agree, so the
// value lives here once.
const browserIdleWindow = 60 * time.Second

// waitMs sleeps for the requested time WITHOUT letting the browser reap itself.
//
// The daemon shuts down after browserIdleWindow with no command, and it takes
// the page with it. A wait was pure sleep, so `action=wait:ms=70000` returned
// to a fresh browser holding nothing and every assertion after it read an empty
// page. Measured with the window at 6s: one command at 4s kept the page, and no
// command lost it. The slices are half the window, so a slow machine still
// touches the daemon inside it.
func (b *Browser) waitMs(ms string) error {
	var tb textbuf.Buffer

	d, err := time.ParseDuration(tb.Str(ms).Str("ms").Slice())
	if err != nil {
		return fmt.Errorf("parse wait duration %q: %w", ms, err)
	}

	for remaining := d; remaining > 0; {
		slice := min(remaining, browserIdleWindow/2)
		time.Sleep(slice)

		remaining -= slice
		if remaining <= 0 {
			break
		}

		if keepErr := b.runAgent("get", "url"); keepErr != nil {
			return fmt.Errorf("keep the browser alive across a %s wait: %w", d, keepErr)
		}
	}

	return nil
}

// waitUntil re-opens path until the page it serves contains want. It is the
// state-based counterpart of waitMs: the test waits for the server to REPORT the
// condition instead of guessing how long the server needs to reach it.
//
// Neither existing wait covers this. waitLoad settles on the browser's own
// in-flight counter, whose predicate (inflightIdleExpr) is true both before a
// request begins and after it ends, so it cannot prove a click's mutation reached
// the server. A retried expectation cannot either: it re-reads the DOM the page
// already holds, and a readback the browser never fetched again can only ever
// report the state that was true before the click. Re-opening the page each round
// is what makes the answer current.
//
// IT LEAVES THE BROWSER ON path. Every expectation after it therefore asserts
// against the readback, not against the page the test was driving. That is
// deliberate and it is load-bearing in commit-flow.wb, whose following
// `not-contains` must judge the readback: returning to the previous page would
// put that absence back on a page where it is true either way, which is the
// vacuity the wait exists to remove. It is still a trap for the next author, so
// either assert on the readback here or `action=open` back to where you were.
//
// It stops polling at the same expectDeadline every other browser retry uses
// (expect.go), and fails naming the path and the wanted text. That deadline is
// NOT a wall-clock cap on the step: retryCommand checks it between rounds, never
// inside one, so the round in flight when it expires still runs to completion. A
// round issues an `open`, one or more `eval` polls (waitLoad) and a `get html`,
// each capped only by agentTimeout, so a stalled round overshoots. The file's own
// option=timeout is what bounds the test above this, checked between steps by
// runWBTestCase.
func (b *Browser) waitUntil(path, want string) error {
	if path == "" || want == "" {
		return errWaitUntilRequiresPathAndContains
	}
	return retryCommand(func() error {
		if err := b.Open(path); err != nil {
			return err
		}
		html, err := b.getHTML()
		if err != nil {
			return fmt.Errorf("html: %w", err)
		}
		if !strings.Contains(html, want) {
			return fmt.Errorf("%s does not contain %q", path, want)
		}
		return nil
	})
}

// setViewport resizes the browser viewport (agent-browser set viewport <w> <h>)
// so mobile-layout assertions run at the requested size. Applied before the
// first navigation.
func (b *Browser) setViewport(width, height int) error {
	var tb textbuf.Buffer
	w := tb.Int(int64(width)).String()
	h := tb.Reset().Int(int64(height)).String()
	return b.runAgentEnsureDaemon("set", "viewport", w, h)
}

// setLocale sets the browser Accept-Language header (agent-browser set headers
// <json>) so the UI renders under the given locale for the rest of the session.
func (b *Browser) setLocale(lang string) error {
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
	if err := b.fillID("username", user); err != nil {
		return err
	}
	if err := b.fillID("password", password); err != nil {
		return err
	}
	if err := b.Press("Enter"); err != nil {
		return err
	}
	return b.waitLoad()
}

// Snapshot returns the interactive accessibility snapshot.
func (b *Browser) Snapshot() (string, error) {
	return b.runAgentOutput("snapshot", "-i")
}

// fullSnapshot returns the full accessibility snapshot, including static text.
func (b *Browser) fullSnapshot() (string, error) {
	return b.runAgentOutput("snapshot")
}

// Press sends a key press (e.g., "Enter", "Tab", "Escape").
func (b *Browser) Press(key string) error {
	if err := b.runAgent("press", key); err != nil {
		return fmt.Errorf("press %s: %w", key, err)
	}
	return b.waitLoad()
}

// pressOn finds an element by visible text, focuses it, and presses a key.
func (b *Browser) pressOn(text, key string) error {
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
	return b.waitLoad()
}

// pressOnID focuses an element by HTML id and presses a key.
func (b *Browser) pressOnID(id, key string) error {
	var tb textbuf.Buffer
	sel := tb.Byte('#').Str(id).String()
	if err := b.runAgent("focus", sel); err != nil {
		return fmt.Errorf("focus #%s: %w", id, err)
	}
	if err := b.runAgent("press", key); err != nil {
		return fmt.Errorf("press %s on #%s: %w", key, id, err)
	}
	return b.waitLoad()
}

// Click finds an element by visible text in the snapshot, then clicks its @ref.
func (b *Browser) Click(text string) error {
	// Retried as a whole: the element may not be in the snapshot yet, which is
	// the same wait an expectation needs. See retryCommand (expect.go).
	if err := retryCommand(func() error {
		snap, snapErr := b.Snapshot()
		if snapErr != nil {
			return fmt.Errorf("snapshot before click: %w", snapErr)
		}

		ref := findRefByText(snap, text)
		if ref == "" {
			return fmt.Errorf("no element with text containing %q in snapshot:\n%s", text, snap)
		}

		if clickErr := b.runAgent("click", ref); clickErr != nil {
			return fmt.Errorf("click %s (text=%q): %w", ref, text, clickErr)
		}
		return nil
	}); err != nil {
		return err
	}
	return b.waitLoad()
}

// clickID clicks an element by its HTML id attribute using a CSS selector.
func (b *Browser) clickID(id string) error {
	var tb textbuf.Buffer
	sel := tb.Byte('#').Str(id).String()
	// agent-browser reports a missing element as a non-zero exit, so a control
	// the previous step produced asynchronously (an htmx out-of-band swap, say)
	// is a race unless the click waits for it. See retryCommand (expect.go).
	if err := retryCommand(func() error { return b.runAgent("click", sel) }); err != nil {
		return fmt.Errorf("click #%s: %w", id, err)
	}
	return b.waitLoad()
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

// fillID fills an input by its HTML id attribute using a CSS selector.
func (b *Browser) fillID(id, value string) error {
	var tb textbuf.Buffer
	sel := tb.Byte('#').Str(id).String()
	return b.runAgent("fill", sel, value)
}

// hover finds an element by text and hovers.
func (b *Browser) hover(text string) error {
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

// hoverID hovers an element by its HTML id attribute using a CSS selector.
func (b *Browser) hoverID(id string) error {
	var tb textbuf.Buffer
	sel := tb.Byte('#').Str(id).String()
	return b.runAgent("hover", sel)
}

// Screenshot saves a screenshot to the given path.
func (b *Browser) Screenshot(path string) error {
	return b.runAgent("screenshot", path)
}

// getHTML returns the page BODY's HTML. The head is fetched on its own, by
// getHeadHTML: `get html <selector>` returns one element's inner HTML, so no
// single call answers for both.
func (b *Browser) getHTML() (string, error) {
	return b.runAgentOutput("get", "html", "body")
}

// getURL returns the address bar. Every other read goes through the DOM, and
// the DOM cannot answer a history question: a pushed URL changes the address
// and leaves the markup alone.
func (b *Browser) getURL() (string, error) {
	return b.runAgentOutput("get", "url")
}

// Back is the browser's own back button, and the only way to prove what a
// pushed URL does when the operator returns to it. htmx 2 restores from its own
// history cache; htmx 4 keeps none, so the browser navigates for real and the
// answer to that navigation has to be a whole page.
func (b *Browser) Back() error {
	return b.runAgent("back")
}

// Forward is the browser's own forward button, and it answers a question back
// cannot: htmx 4 drives history through the Navigation API, which holds entries
// on both sides of the current one. A back that renders a whole page proves the
// entry BEHIND was navigable; the entry AHEAD is a second entry, restored by a
// second traversal, and only forward reaches it.
func (b *Browser) Forward() error {
	return b.runAgent("forward")
}

// getHeadHTML returns the page HEAD's HTML, which is where a page states what
// it loads. Each page loads the assets its own markup needs
// (internal/le/webassets/webassets.go), so what a head does NOT carry is as much a
// property of the page as what it does.
func (b *Browser) getHeadHTML() (string, error) {
	return b.runAgentOutput("get", "html", "head")
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

// inflightIdleExpr is the predicate waitLoad polls: no in-flight fetch/XHR
// and quiet for a short debounce. The debounce bridges chained requests --
// cli.js issues fetch(/cli) -> htmx refresh -> fetch(/config/changes) in
// sequence, so the counter dips to zero for a microtask between them.
const inflightIdleExpr = `(window.__zeInflight||0)===0 && (performance.now()-(window.__zeLastChange||0))>=120`

var (
	initScriptOnce sync.Once
	initScriptPath string
)

// ensureInitScript writes the in-flight instrumentation to a temp file once
// and returns its path, or "" if it could not be written (waitLoad then
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
	inherited := os.Environ()
	env := make([]string, 0, len(inherited)+3)
	hasIdle, hasInit, hasSession := false, false, false
	for _, e := range inherited {
		// Chrome launches crash before DevTools starts when TMPDIR points at the
		// functional suite's noexec scratch filesystem. Let Chrome use the
		// platform temp directory. Ze's own test processes keep the scratch path.
		if strings.HasPrefix(e, "TMPDIR=") {
			continue
		}
		env = append(env, e)
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
		env = append(env, tb.Str("AGENT_BROWSER_IDLE_TIMEOUT_MS=").Int(browserIdleWindow.Milliseconds()).String())
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
		browser = newBrowserWithSession(baseURL, session)
	} else {
		browser = newBrowser(baseURL)
	}
	// Free this test's browser session when it finishes. Sessions are keyed per
	// test. Without this close, more than 80 live pages accumulate and starve the
	// later tests. The suite-end sweep is only a backstop.
	//
	// There is no leading Close here. A close immediately followed by Open makes
	// agent-browser stop and start the same session back-to-back. The navigation
	// can then stay on about:blank. Each test has a unique session, so its first
	// Open already starts with a clean context.
	defer browser.Close()

	var (
		steps []trace.StepResult
		tb    textbuf.Buffer
	)

	// Apply session-level options before the first navigation: a mobile
	// viewport and/or an Accept-Language locale carry through every open.
	if tc.Viewport.Width > 0 && tc.Viewport.Height > 0 {
		if err := browser.setViewport(tc.Viewport.Width, tc.Viewport.Height); err != nil {
			return &WBTestResult{Error: tb.Str("set viewport: ").Err(err).String()}
		}
	}
	if tc.Locale != "" {
		if err := browser.setLocale(tc.Locale); err != nil {
			return &WBTestResult{Error: tb.Reset().Str("set locale: ").Err(err).String()}
		}
	}

	// The file's declared budget. Checked BETWEEN steps, which bounds the test at
	// its timeout plus one step. Every step is itself bounded. A browser command is
	// bounded by agentTimeout, a retrying step by expectDeadline or
	// expectRetryDeadline, and `action=wait` by the millisecond count the file
	// asked for. Checking inside a step would mean threading the deadline through
	// every retry loop for a bound the caller already has.
	//
	// Nothing read the Timeout field until 2026-08-23, so every .wb that declared a
	// timeout declared nothing. `pr.Run` (internal/test/runner/parallel.go) gives
	// each test the suite's context and no per-test deadline. A web test that
	// stopped making progress therefore hung the suite instead of failing it.
	//
	// A zero Timeout means UNSPECIFIED, never "no time at all". ParseWBFile always
	// seeds the field and refuses a non-positive declared value. Zero therefore
	// reaches here only from a WBTestCase built in code, where the caller declared
	// nothing. Reading it literally would expire the deadline before step one and
	// fail every such run. That is a zero value that looks like a valid answer
	// (docs/contributing/ze-go-style.md, "Types that cannot lie").
	// The declared value uses the same verification headroom as the other suite
	// budgets. The full verification run can overlap this suite with the race
	// unit stage. CPU starvation can make a 2s standalone step take much longer.
	// The files declared their timeouts before this field was enforced. Headroom
	// keeps those existing values usable, while a standalone run keeps the
	// declared value and reports a real hang quickly.
	budget := tc.Timeout
	if budget <= 0 {
		budget = defaultWBTimeout
	}
	if runner.VerifyModeEnabled() {
		budget *= runner.ParallelTimeoutHeadroom
	}
	deadline := time.Now().Add(budget)

	for i, step := range tc.Steps {
		if !time.Now().Before(deadline) {
			return &WBTestResult{
				Error: tb.Reset().Str("timed out after ").Str(budget.String()).
					Str(" (option=timeout) with ").Int(int64(len(tc.Steps) - i)).
					Str(" step(s) left; raise the declared timeout or find what stopped making progress").String(),
				Steps: steps,
			}
		}
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
	// defaultWBTimeout is the wall-clock budget a .wb file gets when it declares
	// none. ParseWBFile seeds WBTestCase.Timeout with it and
	// `option=timeout:value=` replaces it.
	defaultWBTimeout = 30 * time.Second
	// Generous: under CPU load an HTMX/JS render can land seconds after waitLoad
	// reports network idle. The assertion must out-wait the render without
	// out-waiting the test timeout.
	expectRetryDeadline = 5 * time.Second
	// Poll gently because each retry starts an agent-browser subprocess.
	expectRetryPoll = 250 * time.Millisecond
)

// checkExpectationRetry polls an expectation until a short deadline expires.
// A browser snapshot is a point-in-time read. Under CPU load, an HTMX or
// JavaScript render can land after waitLoad reports network idle. Retrying turns
// that race into a bounded wait. A wrong page still fails after the deadline.
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
			return b.clickID(id)
		}
		return b.Click(a.Values["text"])
	case "fill":
		if id, ok := a.Values["id"]; ok {
			return b.fillID(id, a.Values["value"])
		}
		return b.Fill(a.Values["text"], a.Values["value"])
	case "hover":
		if id, ok := a.Values["id"]; ok {
			return b.hoverID(id)
		}
		return b.hover(a.Values["text"])
	case "wait":
		if ms, ok := a.Values["ms"]; ok {
			return b.waitMs(ms)
		}
		return b.waitLoad()
	case "wait-until":
		return b.waitUntil(a.Values["path"], a.Values["contains"])
	case "back":
		return b.Back()
	case "forward":
		return b.Forward()
	case "press":
		key := a.Values["key"]
		if key == "" {
			return errPressActionRequiresKeyParameter
		}
		if id, ok := a.Values["id"]; ok {
			return b.pressOnID(id, key)
		}
		if text, ok := a.Values["text"]; ok {
			return b.pressOn(text, key)
		}
		return b.Press(key)
	case "screenshot":
		return b.Screenshot(a.Values["file"])
	case "login":
		return b.Login(a.Values["user"], a.Values["password"])
	}
	return fmt.Errorf("unknown action kind %q", a.Kind)
}
