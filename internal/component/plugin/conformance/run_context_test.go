// Design: docs/plugin-development/testing.md -- an actively canceled query is not a plugin violation.
// Related: ../declarations.go -- QueryDeclaration owns the process-group stop and Wait.

package conformance

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestRunCancelsAndJoinsAStartedQuery requires a real started shell and its
// descendant to stop before Run answers caller cancellation. A declaration
// written before cancellation must not turn that interruption into a failed
// conformance report; the existing timeout violation remains a separate case.
func TestRunCancelsAndJoinsAStartedQuery(t *testing.T) {
	const queryBudget = 20 * time.Second
	const returnBudget = 2 * time.Second
	workDir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	type result struct {
		report Report
		err    error
	}
	answer := make(chan result, 1)
	run := "printf '%s\\n' \"$$\" > query-owner.pid; " +
		writes(string(declarationBytes(t, validDeclaration()))) +
		"; sh -c 'printf \"%s\\n\" \"$$\" > query-child.pid; exec sleep 30' & wait"
	go func() {
		report, err := Run(ctx, Target{Name: "configured-cancel-query", Run: run, WorkDir: workDir}, queryBudget)
		answer <- result{report: report, err: err}
	}()
	var owned *runQueryGroup
	joined := false
	defer func() {
		cancel()
		// Only verified group membership authorizes a cleanup signal.
		// Without it the query's own budget still bounds the join.
		if owned != nil {
			owned.stop(t)
		}
		if !joined {
			select {
			case <-answer:
			case <-time.After(queryBudget + time.Second):
				t.Error("the owned conformance worker did not join during cleanup")
			}
		}
	}()
	var pids [2]int
	runQueryAwait(t, 5*time.Second, func() (bool, error) {
		for i, name := range []string{"query-owner.pid", "query-child.pid"} {
			content, err := os.ReadFile(filepath.Join(workDir, name))
			if errors.Is(err, fs.ErrNotExist) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			pid, err := strconv.Atoi(strings.TrimSpace(string(content)))
			if err != nil {
				return false, nil //nolint:nilerr // A partially published PID line is not ready; bounded poll retries.
			}
			if pid <= 1 {
				return false, nil
			}
			pids[i] = pid
		}
		return true, nil
	}, "the query and its descendant must start before cancellation")
	group, err := syscall.Getpgid(pids[0])
	require.NoError(t, err)
	require.Equal(t, pids[0], group, "the query must own an isolated process group")
	require.NotEqual(t, syscall.Getpgrp(), group, "the test's own process group must never be signaled")
	childGroup, err := syscall.Getpgid(pids[1])
	require.NoError(t, err)
	require.Equal(t, group, childGroup, "the descendant must belong to the query's group")
	owned = &runQueryGroup{group: group, pids: pids}

	cancel()
	select {
	case completed := <-answer:
		joined = true
		require.ErrorIs(t, completed.err, context.Canceled)
		assert.NotErrorIs(t, completed.err, context.DeadlineExceeded)
		assert.Equal(t, Report{}, completed.report, "caller cancellation supplies no conformance verdict")
	case <-time.After(returnBudget):
		t.Fatal("the caller canceled a started query, but Run waited for its independent query budget")
	}
	_, err = syscall.Getpgid(pids[0])
	assert.ErrorIs(t, err, syscall.ESRCH, "Run must reap its direct child before returning")
	runQueryAwait(t, returnBudget, func() (bool, error) {
		running, err := owned.running()
		if err != nil {
			return false, err
		}
		return !running, nil
	}, "the canceled query must leave no live descendant")
}

// runQueryGroup records the process group the kernel assigned to both observed
// query processes. A reused PID in another group is no longer test-owned.
type runQueryGroup struct {
	group int
	pids  [2]int
}

func (g *runQueryGroup) running() (bool, error) {
	for _, pid := range g.pids {
		group, err := syscall.Getpgid(pid)
		if errors.Is(err, syscall.ESRCH) {
			continue
		}
		if err != nil {
			return false, err
		}
		if group == g.group {
			return true, nil
		}
	}
	return false, nil
}

// stop rechecks recorded group membership before sending a group-only stop.
// Unknown kernel state never authorizes a signal or proves successful cleanup.
func (g *runQueryGroup) stop(t *testing.T) {
	t.Helper()
	running, err := g.running()
	if err != nil {
		t.Errorf("read this test's query group %d before cleanup: %v", g.group, err)
		return
	}
	if !running {
		return
	}
	if err := plugin.KillProcessGroup(g.group); err != nil {
		if !errors.Is(err, syscall.ESRCH) {
			t.Errorf("stop this test's verified query group %d: %v", g.group, err)
		}
	}
}

// runQueryAwait polls observable state on the test goroutine. An unreadable
// kernel or readiness state is an error, never evidence that a process stopped.
func runQueryAwait(t *testing.T, budget time.Duration, check func() (bool, error), message string) {
	t.Helper()
	deadline := time.NewTimer(budget)
	defer deadline.Stop()
	ticks := time.NewTicker(time.Millisecond)
	defer ticks.Stop()
	for {
		ready, err := check()
		require.NoError(t, err, message)
		if ready {
			return
		}
		select {
		case <-ticks.C:
		case <-deadline.C:
			t.Fatal(message)
		}
	}
}
