// Design: docs/architecture/testing/runner-architecture.md -- inherited operation deadlines.
package webtesting

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestBrowserCommandsInheritCancellation checks both command paths before spawn.
// MUTATION: derive either command context from Background instead of b.ctx.
func TestBrowserCommandsInheritCancellation(t *testing.T) {
	log := installFakeAgentBrowser(t)
	for _, output := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		browser := newBrowser("")
		browser.ctx = ctx
		var err error
		if output {
			_, err = browser.runAgentOutput("eval", "true")
		} else {
			err = browser.runAgent("eval", "true")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("output=%t command lost cancellation: %v", output, err)
		}
	}
	if data, err := os.ReadFile(log); err == nil && len(data) != 0 {
		t.Fatalf("canceled commands started: %s", data)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

// TestWBCaseDeadlineBoundaryOutcomes checks refusal before the first step and
// after an empty case, including pre-navigation options and standalone identity.
func TestWBCaseDeadlineBoundaryOutcomes(t *testing.T) {
	installFakeAgentBrowser(t)
	if result := RunWBCase(context.Background(), &WBTestCase{}, "", ""); result.Passed || !strings.Contains(result.Error, "requires a deadline") {
		t.Fatalf("missing deadline accepted: %+v", result)
	}
	for _, body := range []string{"", "action=wait:ms=1\n", "option=viewport:width=800:height=600\n", "option=locale:lang=en-US\n"} {
		tc, err := ParseWBFile(body)
		if err != nil {
			t.Fatal(err)
		}
		before := time.Now()
		ctx, cancel, err := CaseContext(context.Background(), 0, 1)
		if err != nil {
			t.Fatal(err)
		}
		deadline, _ := ctx.Deadline()
		if deadline.Before(before.Add(30*time.Second)) || deadline.After(time.Now().Add(30*time.Second)) {
			t.Fatalf("unspecified budget is not 30s: %v", deadline)
		}
		cancel()
		result := RunWBCase(ctx, tc, "", "")
		if result.Passed || !strings.Contains(result.Error, "context canceled") || len(result.Steps) != 0 {
			t.Fatalf("canceled case %q: %+v", body, result)
		}
	}
}

// TestBrowserRetriesInheritCancellation checks pre-call refusal, cancellation
// during a successful call, and preservation of the last failed call's cause.
// MUTATION: bounded derives its context from Background rather than its parent.
func TestBrowserRetriesInheritCancellation(t *testing.T) {
	for _, fetch := range []bool{false, true} {
		for _, phase := range []string{"before", "success", "failure"} {
			ctx, cancel := context.WithCancel(context.Background())
			browser := newBrowser("")
			browser.ctx = ctx
			if phase == "before" {
				cancel()
			}
			calls := 0
			cause := errors.New("owned query failed")
			run := func() error {
				calls++
				cancel()
				if phase == "failure" {
					return cause
				}
				return nil
			}
			var err error
			if fetch {
				_, err = browser.retryFetch(func() (string, error) { return "owned", run() })
			} else {
				err = browser.retryCommand(run)
			}
			cancel()
			if !errors.Is(err, context.Canceled) {
				t.Errorf("fetch=%t phase=%s lost cancellation: %v", fetch, phase, err)
			}
			want := 1
			if phase == "before" {
				want = 0
			}
			if calls != want {
				t.Errorf("fetch=%t phase=%s calls=%d want=%d", fetch, phase, calls, want)
			}
			if phase == "failure" && !errors.Is(err, cause) {
				t.Fatalf("lost query failure: %v", err)
			}
			if browser.ctx != ctx {
				t.Fatal("retry did not restore parent context")
			}
		}
	}
}

// TestBrowserWaitLoadCommandFailure checks that an already canceled command is
// refused by waitLoad rather than interpreted as a settled page.
func TestBrowserWaitLoadCommandFailure(t *testing.T) {
	installFakeAgentBrowser(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	browser := newBrowser("")
	browser.ctx = ctx
	if err := browser.waitLoad(); !errors.Is(err, context.Canceled) {
		t.Fatalf("waitLoad hid canceled command: %v", err)
	}
	if !filepath.IsAbs(ensureInitScript()) {
		t.Fatal("waitLoad did not exercise the installed-script route")
	}
}
