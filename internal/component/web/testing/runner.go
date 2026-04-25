// Design: docs/architecture/testing/ci-format.md -- web browser test runner
// Related: parser.go -- .wb file parsing
// Related: expect.go -- expectation checking

package webtesting

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// WBTestResult holds the outcome of a single .wb test.
type WBTestResult struct {
	Passed     bool
	Error      string
	Skipped    bool
	SkipReason string
}

// Browser wraps agent-browser CLI commands.
type Browser struct {
	baseURL string
}

// NewBrowser creates a browser instance targeting the given base URL.
func NewBrowser(baseURL string) *Browser {
	return &Browser{baseURL: baseURL}
}

// Open navigates to baseURL + path.
func (b *Browser) Open(path string) error {
	url := b.baseURL + path
	if err := runAgent("open", url, "--ignore-https-errors"); err != nil {
		return fmt.Errorf("open %s: %w", url, err)
	}
	return b.WaitLoad()
}

// WaitLoad waits for network idle.
func (b *Browser) WaitLoad() error {
	return runAgent("wait", "--load", "networkidle")
}

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

// Fill finds an input by placeholder/label text and fills it.
func (b *Browser) Fill(text, value string) error {
	snap, err := b.Snapshot()
	if err != nil {
		return fmt.Errorf("snapshot before fill: %w", err)
	}

	ref := findRefByText(snap, text)
	if ref == "" {
		return fmt.Errorf("no input with text containing %q", text)
	}

	return runAgent("fill", ref, value)
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

// Screenshot saves a screenshot to the given path.
func (b *Browser) Screenshot(path string) error {
	return runAgent("screenshot", path)
}

// GetText returns the full page text.
func (b *Browser) GetText() (string, error) {
	return runAgentOutput("get", "text")
}

// Close closes the browser.
func (b *Browser) Close() {
	_ = runAgent("close")
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
	ctx, cancel := context.WithTimeout(context.Background(), agentTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, agentBrowserBin, args...) //nolint:gosec // args are test-controlled, not user input
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runAgentOutput executes agent-browser and returns stdout.
func runAgentOutput(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), agentTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, agentBrowserBin, args...) //nolint:gosec // args are test-controlled, not user input
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
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
	_ = runAgent("close")

	browser := NewBrowser(baseURL)

	for _, step := range tc.Steps {
		switch step.Type {
		case WBStepAction:
			a := tc.Actions[step.ActionIndex]
			if err := executeAction(browser, &a); err != nil {
				return &WBTestResult{Error: fmt.Sprintf("line %d: action %s: %v", a.Line, a.Kind, err)}
			}
		case WBStepExpect:
			e := tc.Expects[step.ExpectIndex]
			if err := checkExpectation(browser, &e); err != nil {
				return &WBTestResult{Error: fmt.Sprintf("line %d: expect %s: %v", e.Line, e.Kind, err)}
			}
		}
	}

	return &WBTestResult{Passed: true}
}

func executeAction(b *Browser, a *WBAction) error {
	switch a.Kind {
	case "open":
		return b.Open(a.Values["path"])
	case "click":
		return b.Click(a.Values["text"])
	case "fill":
		return b.Fill(a.Values["text"], a.Values["value"])
	case "hover":
		return b.Hover(a.Values["text"])
	case "wait":
		if ms, ok := a.Values["ms"]; ok {
			return b.WaitMs(ms)
		}
		return b.WaitLoad()
	case "press":
		key := a.Values["key"]
		if key == "" {
			return fmt.Errorf("press action requires key= parameter")
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
