// Design: docs/architecture/testing/ci-format.md -- web browser expectation checking
// Related: parser.go -- .wb file parsing

package webtesting

import (
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// expectDeadline bounds how long a POSITIVE expectation waits for the thing
	// it is looking for. Sized against the HARNESS, not against how long a page
	// "should" take: the suite drives four tests at once through a single shared
	// agent-browser daemon, where one `snapshot` or `eval` round trip costs
	// seconds and a 10-step test that runs in 3s alone takes 20-45s. At 5s this
	// budget bought one or two samples and `commit-flow.wb` still lost the race.
	// It stays well inside the per-test `option=timeout` (30-60s), so a genuinely
	// missing element still fails the step rather than the whole test.
	expectDeadline = 15 * time.Second
	expectPoll     = 100 * time.Millisecond
)

// retryPositive re-evaluates check until it passes or the deadline expires, and
// returns the LAST failure so the error still carries a current snapshot.
//
// Positive expectations must poll because `action=wait` cannot prove the update
// they are waiting for has started. WaitLoad's predicate (runner.go
// inflightIdleExpr) is `no in-flight request AND quiet for 120ms`, which is true
// both AFTER a request finishes and BEFORE one begins: a click that dispatches
// its htmx request asynchronously leaves a window in which the page is idle, the
// wait returns immediately, and a single-sample assertion reads the pre-request
// DOM. That is what failed `commit-flow.wb` line 15 and
// `scenario-interface-setup.wb` under the suite's 4-way concurrency while both
// passed alone.
//
// NEGATIVE expectations deliberately do NOT poll. "Not present" is satisfied by
// the first sample, so retrying could only ever convert a real failure into a
// pass by sampling earlier; the preceding wait is what makes the absence
// meaningful.
func retryPositive(check func() error) error {
	return retryCommand(check)
}

// retryCommand re-runs anything that reports failure as an error until it
// succeeds or expectDeadline expires, returning the LAST error. retryPositive is
// its assertion use; Browser.Click and Browser.ClickID are its command use.
//
// A click needs it for the same reason a positive expectation does, and the
// suite proved it: `click #commit-review-btn: exit status 1` on commit-flow.wb
// line 17, one full-suite run in five, while line 15 had just found that same
// button's text. agent-browser reports a missing element as a non-zero exit, so
// clicking a control the previous step produced asynchronously is a race unless
// the click waits for it.
//
// Retrying a click cannot double-submit the failure mode this exists for: an
// element that is not there was not clicked. A command that acted and THEN
// reported an error would be re-run, which is the accepted residual.
func retryCommand(run func() error) error {
	deadline := time.Now().Add(expectDeadline)
	for {
		err := run()
		if err == nil {
			return nil
		}
		if !time.Now().Before(deadline) {
			return err
		}
		time.Sleep(expectPoll)
	}
}

// retryFetch re-runs a browser QUERY until it succeeds, and is not an assertion
// retry: it loops on the command erroring, never on what the command returned.
//
// `agent-browser html` exits non-zero from time to time under the suite's
// four-way concurrency against one shared daemon, and a negative expectation
// then fails on `html: exit status 1` -- a tool failure wearing an assertion's
// clothes, and one that says nothing about the page. Negatives cannot use
// retryPositive (retrying an absence would only ever turn a real failure into a
// pass), but they can insist on getting a usable answer before judging it.
func retryFetch(fetch func() (string, error)) (string, error) {
	deadline := time.Now().Add(expectDeadline)
	for {
		out, err := fetch()
		if err == nil {
			return out, nil
		}
		if !time.Now().Before(deadline) {
			return "", err
		}
		time.Sleep(expectPoll)
	}
}

// checkExpectation validates a single expectation against the current browser state.
func checkExpectation(b *Browser, e *WBExpectation) error {
	switch e.Kind {
	case "element":
		return checkElement(b, e)
	case "breadcrumb":
		return checkBreadcrumb(b, e)
	case "html":
		return checkHTML(b, e)
	case "url":
		return checkURL(b, e)
	case "title":
		return checkTitle(b, e)
	}
	return fmt.Errorf("unknown expectation kind %q", e.Kind)
}

func checkElement(b *Browser, e *WBExpectation) error {
	snap, err := retryFetch(b.Snapshot)
	if err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}

	if id, ok := e.Values["id"]; ok {
		var tb textbuf.Buffer
		attrDouble := tb.Str("id=\"").Str(id).Byte('"').String()
		attrSingle := tb.Reset().Str("id='").Str(id).Byte('\'').String()
		if err := retryPositive(func() error {
			html, htmlErr := b.getHTML()
			if htmlErr != nil {
				return fmt.Errorf("html: %w", htmlErr)
			}
			if !strings.Contains(html, attrDouble) && !strings.Contains(html, attrSingle) {
				current, snapErr := b.Snapshot()
				if snapErr != nil {
					current = snap
				}
				return fmt.Errorf("expected element with id %q not found in DOM; snapshot:\n%s", id, current)
			}
			return nil
		}); err != nil {
			return err
		}
	}

	if id, ok := e.Values["not-id"]; ok {
		html, htmlErr := retryFetch(b.getHTML)
		if htmlErr != nil {
			return fmt.Errorf("html: %w", htmlErr)
		}
		if strings.Contains(html, "id=\""+id+"\"") || strings.Contains(html, "id='"+id+"'") {
			return fmt.Errorf("unexpected element with id %q found", id)
		}
	}

	if text, ok := e.Values["text"]; ok {
		want := strings.ToLower(text)
		if err := retryPositive(func() error {
			fullSnap, textErr := b.fullSnapshot()
			if textErr != nil {
				return fmt.Errorf("full snapshot: %w", textErr)
			}
			if !strings.Contains(strings.ToLower(fullSnap), want) {
				return fmt.Errorf("expected element with text %q not found in snapshot:\n%s", text, fullSnap)
			}
			return nil
		}); err != nil {
			return err
		}
	}

	if text, ok := e.Values["not-text"]; ok {
		fullSnap, textErr := retryFetch(b.fullSnapshot)
		if textErr != nil {
			return fmt.Errorf("full snapshot: %w", textErr)
		}
		if strings.Contains(strings.ToLower(fullSnap), strings.ToLower(text)) {
			return fmt.Errorf("unexpected element with text %q found in snapshot:\n%s", text, fullSnap)
		}
	}

	return nil
}

func checkHTML(b *Browser, e *WBExpectation) error {
	html, err := retryFetch(b.getHTML)
	if err != nil {
		return fmt.Errorf("html: %w", err)
	}
	if sub, ok := e.Values["contains"]; ok {
		if err := retryPositive(func() error {
			current, htmlErr := b.getHTML()
			if htmlErr != nil {
				return fmt.Errorf("html: %w", htmlErr)
			}
			if !strings.Contains(current, sub) {
				return fmt.Errorf("HTML does not contain %q", sub)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	if sub, ok := e.Values["not-contains"]; ok {
		if strings.Contains(html, sub) {
			return fmt.Errorf("HTML unexpectedly contains %q", sub)
		}
	}
	return nil
}

func checkBreadcrumb(b *Browser, e *WBExpectation) error {
	snap, err := b.Snapshot()
	if err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}

	snapLower := strings.ToLower(snap)

	if csv, ok := e.Values["contains"]; ok {
		for seg := range strings.SplitSeq(csv, ",") {
			seg = strings.TrimSpace(seg)
			if !strings.Contains(snapLower, strings.ToLower(seg)) {
				return fmt.Errorf("breadcrumb missing segment %q in snapshot:\n%s", seg, snap)
			}
		}
	}

	if csv, ok := e.Values["not-contains"]; ok {
		for seg := range strings.SplitSeq(csv, ",") {
			seg = strings.TrimSpace(seg)
			if strings.Contains(snapLower, "\""+strings.ToLower(seg)+"\"") ||
				strings.Contains(snapLower, " "+strings.ToLower(seg)+" ") {
				return fmt.Errorf("breadcrumb has unexpected segment %q", seg)
			}
		}
	}

	return nil
}

func checkURL(b *Browser, e *WBExpectation) error {
	snap, err := b.Snapshot()
	if err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}

	if sub, ok := e.Values["contains"]; ok {
		if !strings.Contains(snap, sub) {
			return fmt.Errorf("URL does not contain %q", sub)
		}
	}

	return nil
}

func checkTitle(b *Browser, e *WBExpectation) error {
	text, err := b.fullSnapshot()
	if err != nil {
		return fmt.Errorf("full snapshot: %w", err)
	}

	if sub, ok := e.Values["contains"]; ok {
		if !strings.Contains(strings.ToLower(text), strings.ToLower(sub)) {
			return fmt.Errorf("page text does not contain %q", sub)
		}
	}

	return nil
}
