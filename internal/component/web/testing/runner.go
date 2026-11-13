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
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/trace"
)

var (
	errPressActionRequiresKeyParameter  = errors.New("press action requires key= parameter")
	errWaitUntilRequiresPathAndContains = errors.New("wait-until action requires path= and contains= parameters")
	errWaitHTMLRequiresOneCondition     = errors.New("wait-html action takes exactly one of contains=<text> or not-contains=<text>, " +
		"for example action=wait-html:not-contains=commit-bar visible")
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
	ctx           context.Context
	baseURL       string
	session       string
	daemonStarted bool
	observer      *renderObserver
}

var browserSequence atomic.Uint64

// newBrowser creates a browser instance with its own process-local identity.
func newBrowser(baseURL string) *Browser {
	var tb textbuf.Buffer
	session := tb.Str("ze-browser-").Int(int64(os.Getpid())).Byte('-').Uint(browserSequence.Add(1)).String()
	return newBrowserWithSession(baseURL, session)
}

// newBrowserWithSession creates a browser bound to an isolated agent-browser session.
func newBrowserWithSession(baseURL, session string) *Browser {
	return &Browser{ctx: context.Background(), baseURL: baseURL, session: session}
}

// Open navigates to baseURL + path.
func (b *Browser) Open(path string) error {
	var tb textbuf.Buffer
	url := tb.Str(b.baseURL).Str(path).String()
	if err := b.renderObserverScript(inflightInitJS); err != nil {
		return fmt.Errorf("install request observation before open %s: %w", url, err)
	}
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

// waitLoad waits for in-flight fetch/XHR requests and HTMX request work to
// drain. The finder UI
// keeps a persistent EventSource(/events) open, so `wait --load networkidle`
// never settles and burns a fixed ~1.44s on every call. Instead we poll an
// in-flight counter registered through the native SDK before navigation, with
// `eval` polling. Missing observation and a request that never settles fail
// rather than looking idle.
//
// The wait is bounded by the case deadline (b.ctx, which runWBCase refuses to
// run without), never by a fixed wall-clock limit of its own. A click's request
// is legitimate work for as long as the case may run: a commit's durable store
// write took 5.1s on a loaded host, and a fixed 5s bound failed that correct
// click as "did not settle". When the case deadline passes, the error names
// every request still pending (describePending), so a request that never
// answers is reported by method and URL rather than as a bare "false".
func (b *Browser) waitLoad() error {
	var lastErr error
	for {
		out, err := b.runAgentOutput("eval", inflightIdleExpr)
		if b.ctx.Err() != nil {
			return errors.Join(lastErr, err, b.describePending(), context.Cause(b.ctx))
		}
		if err == nil && strings.TrimSpace(out) == "true" {
			return context.Cause(b.ctx)
		}
		if err == nil {
			err = fmt.Errorf("browser requests did not settle: %q", strings.TrimSpace(out))
		}
		lastErr = err
		if waitErr := waitContext(b.ctx, waitLoadPoll); waitErr != nil {
			return errors.Join(err, b.describePending(), waitErr)
		}
	}
}

const (
	waitLoadPoll        = 40 * time.Millisecond
	pendingReadDeadline = 2 * time.Second
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
		if err := waitContext(b.ctx, slice); err != nil {
			return err
		}

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
// The retry deadline and every command inside it inherit the case deadline.
func (b *Browser) waitUntil(path, want string) error {
	if path == "" || want == "" {
		return errWaitUntilRequiresPathAndContains
	}
	return b.retryCommand(func() error {
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

// waitHTML polls the page the browser ALREADY holds until its body contains,
// or no longer contains, one text. It is the in-page counterpart of waitUntil:
// it never opens, reloads or navigates, so it judges what an htmx response did
// to the current DOM (an out-of-band swap, a class that goes away), which a
// re-opened page cannot show because the fresh render answers from the server.
//
// A negative expectation cannot stand in for it. `expect=html:not-contains=`
// states an absence (expect.go: negatives deliberately do not poll), and the
// only retry it gets is checkExpectationRetry's expectRetryDeadline, shorter
// than the budget a slow commit answer needs in a loaded suite. This action
// names the state the step waits for and polls it under expectDeadline, the
// budget waitUntil uses.
//
// It takes exactly one condition, and refuses any other key: a misspelled key
// would otherwise leave a step that waits for nothing and passes.
//
// The retry deadline and every command inside it inherit the case deadline.
func (b *Browser) waitHTML(values map[string]string) error {
	if len(values) != 1 {
		return errWaitHTMLRequiresOneCondition
	}
	if text, ok := values["contains"]; ok {
		return b.waitHTMLState(text, true)
	}
	if text, ok := values["not-contains"]; ok {
		return b.waitHTMLState(text, false)
	}
	return errWaitHTMLRequiresOneCondition
}

// waitHTMLState re-reads the current page body until text's presence equals
// present. At the deadline it answers the last page it judged, with the page
// excerpt, even when the deadline killed the read in progress: that read's
// error says nothing about the page, and alone it would hide the verdict.
func (b *Browser) waitHTMLState(text string, present bool) error {
	if text == "" {
		return errWaitHTMLRequiresOneCondition
	}
	var judged error
	err := b.retryCommand(func() error {
		html, htmlErr := b.getHTML()
		if htmlErr != nil {
			return fmt.Errorf("html: %w", htmlErr)
		}
		if strings.Contains(html, text) == present {
			return nil
		}
		if present {
			judged = fmt.Errorf("current page still does not contain %q%s", text, htmlEvidence(html))
			return judged
		}
		judged = fmt.Errorf("current page still contains %q%s", text, htmlEvidence(html))
		return judged
	})
	if err == nil {
		return nil
	}
	if judged == nil {
		return err
	}
	if errors.Is(err, judged) {
		return err
	}
	return errors.Join(judged, err)
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

// Login drives the real login form once and waits for the authenticated page.
// The native form POST is not counted by waitLoad's fetch/XHR instrumentation;
// returning on network idleness lets the next Open cancel authentication.
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
	if err := b.runAgent("click", `form[action="/login"] button[type="submit"]`); err != nil {
		return fmt.Errorf("submit login: %w", err)
	}
	return b.waitLogin()
}

// loginReadyExpr requires the shared authenticated-page chrome in both finder
// and workbench modes. An empty document, an error page, and a login overlay
// must not acknowledge authentication.
const loginReadyExpr = `document.readyState === 'complete' && document.getElementById('notification-bar') !== null && document.querySelector('form[action="/login"]') === null`

// waitLogin observes the submitted navigation without re-opening or submitting
// anything. The normal positive-expectation deadline bounds a rejected login.
func (b *Browser) waitLogin() error {
	err := b.retryPositive(func() error {
		out, err := b.runAgentOutput("eval", loginReadyExpr)
		if err != nil {
			return fmt.Errorf("read login completion: %w", err)
		}
		if strings.TrimSpace(out) != "true" {
			return errors.New("login did not reach an authenticated page")
		}
		return nil
	})
	if err != nil {
		snapshot, snapshotErr := b.fullSnapshot()
		if snapshotErr != nil {
			return fmt.Errorf("%w; snapshot: %w", err, snapshotErr)
		}
		return fmt.Errorf("%w; snapshot:\n%s", err, snapshot)
	}
	return nil
}

// Snapshot returns the interactive accessibility snapshot.
func (b *Browser) Snapshot() (string, error) {
	return b.runAgentOutput("snapshot", "-i")
}

// fullSnapshot returns the full accessibility snapshot, including static text.
func (b *Browser) fullSnapshot() (string, error) {
	return b.runAgentOutput("snapshot")
}

// scopedSnapshot returns the full accessibility snapshot of one element.
//
// Snapshot answers about the whole page and drops static text, so an assertion
// about a region that is mostly text cannot use it. A breadcrumb is that
// region: its trailing segment is a span, and the ancestors above it are links
// only while the page is deeper than one level.
func (b *Browser) scopedSnapshot(selector string) (string, error) {
	return b.runAgentOutput("snapshot", "-s", selector)
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
	if err := b.retryCommand(func() error {
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
	//
	// It also waits for the element to stop moving. agent-browser measures the
	// element's box and then dispatches a press and a release at that point, with
	// no check that the element is still there. A control that a CSS transition
	// is sliding (the error drawer's tab, for 0.2s after the drawer closes) moves
	// between the measure and the press, the click lands on whatever is now
	// under the point, and agent-browser reports success. On a fast runner the
	// next step's click arrives inside that window: test/web/error-panel-toggle.wb
	// line 65 clicked the tab, the drawer never opened, and line 66 failed.
	stable := clickStableExpr(id)
	if err := b.retryCommand(func() error {
		state, evalErr := b.runAgentOutput("eval", stable)
		if evalErr != nil {
			return fmt.Errorf("eval before click: %w", evalErr)
		}
		if strings.TrimSpace(state) == clickStateMoving {
			return errClickTargetMoving
		}
		return b.runAgent("click", sel)
	}); err != nil {
		return fmt.Errorf("click #%s: %w", id, err)
	}
	return b.waitLoad()
}

// errClickTargetMoving is the refusal a click returns when its element is
// still animated at the deadline. A transition that never ends is a page
// defect, and a click into it would land on nothing and report success.
var errClickTargetMoving = errors.New("the element is still moving: a CSS transition or animation " +
	"runs on it or on an element that contains it, so a click would land where it no longer is; " +
	"check the page's style.css for a transition that does not finish")

// clickStateMoving is what clickStableExpr answers while a finite animation
// runs on the element or an ancestor. Every other answer lets the click go:
// a missing element is reported by agent-browser's own click error, which
// retryCommand already waits on.
const clickStateMoving = "1"

// clickStableExpr answers clickStateMoving while document.getAnimations()
// holds a running, finite animation whose target is the element with this id
// or contains it. CSS transitions are animations there, and asking forces the
// style flush that starts a transition a class change just requested. An
// infinite animation (a spinner) never settles, so it does not hold the click.
func clickStableExpr(id string) string {
	var tb textbuf.Buffer

	return tb.Str(`(function(){var el=document.getElementById(`).Quoted(id).
		Str(`);if(!el)return 0;var moving=document.getAnimations().some(function(a){`).
		Str(`if(a.playState!=='running')return false;var t=a.effect&&a.effect.target;`).
		Str(`if(!t||!(t===el||t.contains(el)))return false;`).
		Str(`return a.effect.getComputedTiming().endTime!==Infinity;});return moving?1:2;})()`).
		String()
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

	return b.fill(ref, value)
}

// fillID fills an input by its HTML id attribute using a CSS selector.
func (b *Browser) fillID(id, value string) error {
	var tb textbuf.Buffer
	return b.fill(tb.Byte('#').Str(id).String(), value)
}

// fill types value into the element selector names, and refuses a target that
// did not take it.
//
// agent-browser fills a div, a span and a label the same way it fills an input:
// it reports success and stores the text nowhere. Nothing then posts, so the
// page the next step reads is the page before the fill, and the run fails
// wherever the missing value is finally asserted -- which reads as the product
// losing the edit. test/web/interface-mac-override.wb spent months red that
// way, on `fill:id=field-address`: the leaf editor's wrapper DIV carries
// field-<leaf> and its input carries input-<path> (view.go, fieldInputID), so
// the id named a real element that cannot hold a value.
//
// Reading the value back is what separates the two. A div answers "" to
// `get value`, with exit 0, which is the silent zero the assertion below turns
// into a refusal that names the selector.
func (b *Browser) fill(selector, value string) error {
	if err := b.runAgent("fill", selector, value); err != nil {
		return err
	}

	stored, err := b.runAgentOutput("get", "value", selector)
	if err != nil {
		return fmt.Errorf("read back %s after fill: %w", selector, err)
	}
	if strings.TrimRight(stored, "\n") != value {
		return fmt.Errorf("fill %s did not take: the element holds %q, not %q"+
			" -- name the input itself, not a wrapper that cannot hold a value",
			selector, strings.TrimRight(stored, "\n"), value)
	}
	return nil
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

// Screenshot saves the rendered page only after its actual work settles.
func (b *Browser) Screenshot(path string) error {
	if err := b.settleCapture(); err != nil {
		return err
	}
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
// (internal/le/web/assets/webassets.go), so what a head does NOT carry is as much a
// property of the page as what it does.
func (b *Browser) getHeadHTML() (string, error) {
	return b.runAgentOutput("get", "html", "head")
}

// Close closes the owned browser session. A browser without an owned session
// returns an error.
//
// Close goes through runAgentEnsureDaemon like every other command. A close
// sent with --ignore-https-errors to the daemon this browser started printed
// "--ignore-https-errors ignored: daemon already running" once per test. That
// warning is about this browser's own daemon, and it read as a shared daemon
// that had lost the option.
// Close MUST be called before releasing the case's resources. Cleanup retains
// the scheduler's command budget policy and gets a fresh command bound, separate
// from execution cancellation and its deadline. The caller MUST report an error
// as incomplete cleanup, never as a passing case.
func (b *Browser) Close() error {
	if b.session == "" {
		return errors.New("browser cleanup incomplete: no owned session")
	}
	parent := b.ctx
	b.ctx = context.WithoutCancel(parent)
	defer func() { b.ctx = parent }()
	if err := b.runAgentEnsureDaemon("close"); err != nil {
		return fmt.Errorf("browser session %q cleanup incomplete: %w", b.session, err)
	}
	b.daemonStarted = false
	b.observer = nil
	return nil
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

// agentPipeWaitDelay is a cleanup bound, separate from the execution deadline.
// It closes inherited output pipes after exit/cancellation; it does not prove
// that a detached browser daemon or Chrome process has exited.
const agentPipeWaitDelay = 2 * time.Second

func (b *Browser) runAgentCore(globalArgs []string, args ...string) error {
	ctx, cancel := context.WithTimeout(b.ctx, Bound(b.ctx, agentTimeout))
	defer cancel()

	if len(globalArgs) > 0 {
		args = append(append([]string{}, globalArgs...), args...)
	}
	cmd := exec.CommandContext(ctx, agentBrowserBin, args...) //nolint:gosec // args are test-controlled, not user input
	cmd.WaitDelay = agentPipeWaitDelay
	cmd.Env = b.agentEnv()
	cmd.Stderr = os.Stderr
	return errors.Join(cmd.Run(), context.Cause(ctx))
}

func (b *Browser) runAgentOutput(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(b.ctx, Bound(b.ctx, agentTimeout))
	defer cancel()

	cmd := exec.CommandContext(ctx, agentBrowserBin, args...) //nolint:gosec // args are test-controlled, not user input
	cmd.WaitDelay = agentPipeWaitDelay
	cmd.Env = b.agentEnv()
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	err = errors.Join(err, context.Cause(ctx))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (b *Browser) agentEnv() []string {
	inherited := os.Environ()
	if b.session != "" {
		inherited = env.Without(inherited, "agent.browser.session")
	}
	env := make([]string, 0, len(inherited)+2)
	hasIdle, hasSession := false, false
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
		if strings.HasPrefix(e, "AGENT_BROWSER_SESSION=") {
			hasSession = true
		}
	}
	var tb textbuf.Buffer
	if !hasIdle {
		env = append(env, tb.Str("AGENT_BROWSER_IDLE_TIMEOUT_MS=").Int(browserIdleWindow.Milliseconds()).String())
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
	ctx, cancel, err := CaseContext(context.Background(), tc.Timeout, 1)
	if err != nil {
		return &WBTestResult{Error: err.Error()}
	}
	defer cancel()
	return RunWBCase(ctx, tc, baseURL, session)
}

// RunWBCase executes a parsed case under the deadline the scheduler started
// before server startup. The caller MUST supply that context, without resetting it.
func RunWBCase(ctx context.Context, tc *WBTestCase, baseURL, session string) *WBTestResult {
	return runWBCase(ctx, tc, baseURL, session, nil)
}

// RunWBCaseWithCapture runs the same parsed journey as RunWBCase, then captures
// its settled final page before owned cleanup. The caller MUST supply the case
// deadline started before server startup; capture MUST obey that same context.
func RunWBCaseWithCapture(ctx context.Context, tc *WBTestCase, baseURL, session string, capture WBCapture) *WBTestResult {
	return runWBCase(ctx, tc, baseURL, session, capture)
}

func runWBCase(ctx context.Context, tc *WBTestCase, baseURL, session string, capture WBCapture) (result *WBTestResult) {
	if _, ok := ctx.Deadline(); !ok {
		return &WBTestResult{Error: "browser case requires a deadline before server startup"}
	}
	var browser *Browser
	if session == "" {
		browser = newBrowser(baseURL)
	} else {
		browser = newBrowserWithSession(baseURL, session)
	}
	browser.ctx = ctx
	// Free this test's browser session when it finishes. Sessions are keyed per
	// test. Without this close, more than 80 live pages accumulate and starve the
	// later tests. A daemon whose run died is reaped by its idle timeout
	// (browserIdleWindow), never by a sweep: agent-browser sessions are shared
	// by every run on the host, so closing all of them kills other runs' pages.
	//
	// There is no leading Close here. A close immediately followed by Open makes
	// agent-browser stop and start the same session back-to-back. The navigation
	// can then stay on about:blank. The caller names a session unique to its
	// run and test (zeTestBrowserSession in internal/test/cli), so its first
	// Open already starts with a clean context.
	defer func() {
		if err := browser.Close(); err != nil {
			if result != nil {
				result.Passed = false
				if result.Error != "" {
					result.Error += "; "
				}
				result.Error += err.Error()
			}
		}
	}()

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

	for i, step := range tc.Steps {
		if err := context.Cause(ctx); err != nil {
			return &WBTestResult{
				Error: err.Error(),
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
		default:
			panic("BUG: unknown web browser test step type")
		}
	}
	if capture != nil {
		if err := browser.capture(capture); err != nil {
			return &WBTestResult{Error: "capture: " + err.Error(), Steps: steps}
		}
	}

	if err := context.Cause(ctx); err != nil {
		return &WBTestResult{Error: err.Error(), Steps: steps}
	}
	return &WBTestResult{Passed: true, Steps: steps}
}

const (
	// defaultWBTimeout is the wall-clock budget a .wb file gets when it declares
	// none. ParseWBFile seeds WBTestCase.Timeout with it and
	// `option=timeout:value=` replaces it.
	defaultWBTimeout = 30 * time.Second
	// This window limits starting another attempt, not the work inside one.
	expectRetryDeadline = 5 * time.Second
	// Poll gently because each retry starts an agent-browser subprocess.
	expectRetryPoll = 250 * time.Millisecond
)

// checkExpectationRetry starts further attempts only within its polling window.
// Each attempt keeps the case context, so sequential nested positive and fetch
// retries receive their own allowances without extending the absolute deadline.
// Checks without an internal retry still poll for asynchronous page updates.
func checkExpectationRetry(b *Browser, e *WBExpectation) error {
	deadline := time.Now().Add(Bound(b.ctx, expectRetryDeadline))
	var lastErr error
	for {
		if err := context.Cause(b.ctx); err != nil {
			return errors.Join(lastErr, err)
		}
		if lastErr != nil {
			if !time.Now().Before(deadline) {
				return lastErr
			}
		}
		lastErr = checkExpectation(b, e)
		if lastErr == nil {
			return context.Cause(b.ctx)
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return errors.Join(lastErr, context.Cause(b.ctx))
		}
		if waitErr := waitContext(b.ctx, min(expectRetryPoll, remaining)); waitErr != nil {
			return errors.Join(lastErr, waitErr)
		}
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
	case "wait-html":
		return b.waitHTML(a.Values)
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
