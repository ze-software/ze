// Design: docs/architecture/testing/runner-architecture.md -- owned browser cleanup.
// Related: deadline.go -- CaseContext and Bound retain the scheduler's policy.
package webtesting

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/test/runner"
)

// closeBudgetProbe observes the existing budget-value boundary while Close
// invokes its real command runner. It forwards every value to the case context.
type closeBudgetProbe struct {
	context.Context
	onBudget func(any)
}

func (p *closeBudgetProbe) Value(key any) any {
	value := p.Context.Value(key)
	if _, budget := key.(caseBudgetKey); budget {
		p.onBudget(value)
	}
	return value
}

// TestBrowserClosePreservesParallelBudget proves a canceled parallel case can
// close its owned session using the scheduler's existing command policy. The
// real Close must consult that policy after removing execution cancellation and
// its deadline, execute only its owned command, and restore the parent context.
func TestBrowserClosePreservesParallelBudget(t *testing.T) {
	argsPath, sessionPath := installCloseBudgetAgent(t, false)
	browser, parent, lookups := canceledParallelClose(t)
	if err := browser.Close(); err != nil {
		t.Fatalf("canceled parallel case prevented owned cleanup: %v", err)
	}
	if *lookups == 0 {
		t.Error("owned close did not consult the parallel case's command policy")
	}
	if browser.ctx != parent {
		t.Error("owned close replaced the execution context")
	}
	if browser.daemonStarted || browser.observer != nil {
		t.Error("successful owned close retained its live browser state")
	}
	assertCloseBudgetCommand(t, argsPath, sessionPath, browser.session)
}

// TestBrowserCloseParallelHangStaysBounded keeps the same canceled parallel
// policy while the owned executable never returns. An independent deadline
// still fails cleanup and must not clear the state needed to report that owner.
func TestBrowserCloseParallelHangStaysBounded(t *testing.T) {
	argsPath, sessionPath := installCloseBudgetAgent(t, true)
	previous := agentTimeout
	agentTimeout = time.Second
	t.Cleanup(func() { agentTimeout = previous })
	browser, parent, lookups := canceledParallelClose(t)
	observer := browser.observer
	err := browser.Close()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("hung parallel close lost its independent deadline: %v", err)
	}
	if !strings.Contains(err.Error(), "cleanup incomplete") || !strings.Contains(err.Error(), browser.session) {
		t.Errorf("hung close lost its owned cleanup failure: %v", err)
	}
	if *lookups == 0 {
		t.Error("hung owned close did not consult the parallel case's command policy")
	}
	if browser.ctx != parent {
		t.Error("hung close replaced the execution context")
	}
	if !browser.daemonStarted || browser.observer != observer {
		t.Error("failed owned close discarded its live browser state")
	}
	assertCloseBudgetCommand(t, argsPath, sessionPath, browser.session)
}

// canceledParallelClose retains an actual CaseContext so the probe observes
// the same private value that Bound reads in production, rather than inventing
// a substitute factor or measuring a scheduling-dependent elapsed interval.
func canceledParallelClose(t *testing.T) (*Browser, context.Context, *int) {
	t.Helper()
	caseContext, cancel, err := CaseContext(context.Background(), time.Minute, 4)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if !errors.Is(caseContext.Err(), context.Canceled) {
		t.Fatal("fixture case did not cancel")
	}
	if _, exists := caseContext.Deadline(); !exists {
		t.Fatal("fixture case has no execution deadline to remove")
	}
	lookups := new(int)
	browser := newBrowserWithSession("", "owned-parallel-budget")
	parent := &closeBudgetProbe{Context: caseContext}
	parent.onBudget = func(value any) {
		(*lookups)++
		factor, ok := value.(time.Duration)
		if want := time.Duration(runner.ParallelTimeoutHeadroom); !ok || factor != want {
			t.Errorf("owned close received budget policy %v, want the parallel factor %v", value, want)
		}
		if browser.ctx.Err() != nil || browser.ctx.Done() != nil || context.Cause(browser.ctx) != nil {
			t.Error("owned cleanup inherited execution cancellation")
		}
		if _, exists := browser.ctx.Deadline(); exists {
			t.Error("owned cleanup inherited the execution deadline")
		}
	}
	browser.ctx = parent
	browser.daemonStarted = true
	browser.observer = &renderObserver{network: "unix", address: "owned-observer-before-close"}
	return browser, parent, lookups
}

func installCloseBudgetAgent(t *testing.T, hang bool) (string, string) {
	t.Helper()
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "close-args")
	sessionPath := filepath.Join(dir, "close-session")
	body := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" > \"$CLOSE_BUDGET_ARGS\"\n" +
		"printf '%s\\n' \"$AGENT_BROWSER_SESSION\" > \"$CLOSE_BUDGET_SESSION\"\n"
	if hang {
		body += "exec sleep 60\n"
	} else {
		body += "exit 0\n"
	}
	if err := os.WriteFile(filepath.Join(dir, agentBrowserBin), []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CLOSE_BUDGET_ARGS", argsPath)
	t.Setenv("CLOSE_BUDGET_SESSION", sessionPath)
	t.Setenv("AGENT_BROWSER_SESSION", "foreign-session-must-not-close")
	return argsPath, sessionPath
}

func assertCloseBudgetCommand(t *testing.T, argsPath, sessionPath, session string) {
	t.Helper()
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read actual owned close command: %v", err)
	}
	if got := strings.TrimSpace(string(args)); got != "close" {
		t.Errorf("actual close arguments %q, want only close", got)
	}
	owned, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("read actual owned close session: %v", err)
	}
	if got := strings.TrimSpace(string(owned)); got != session {
		t.Errorf("actual close session %q, want %q", got, session)
	}
}
