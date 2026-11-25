// Design: docs/plugin-development/testing.md -- active configured query ownership and accepted reload authority.
// Related: configured_plugin_conformance_test.go -- initial admission and report controls.
// Related: internal/component/plugin/declarations.go -- the bounded process-group query.

package hub

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

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
)

const configuredQueryBudget = 20 * time.Second
const configuredQueryStopBound = 3 * time.Second

type configuredQueryResult struct {
	response *plugin.Response
	err      error
}

// configuredQueryGroup identifies the two test-owned shells the kernel placed
// in the query's new process group. A reused PID in another group is no longer
// our process. The test never sends a signal to a bare PID read from a file.
type configuredQueryGroup struct {
	group int
	pids  [2]int
}

func (g *configuredQueryGroup) running() (bool, error) {
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

// stop rechecks kernel group membership before cleanup signals the test-owned
// group. If neither recorded process still belongs to it, no signal is sent.
func (g *configuredQueryGroup) stop(t *testing.T) {
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

// configuredConformanceAwait polls an observable process state on the test
// goroutine. Errors are unknown state and fail, never a successful stop.
func configuredConformanceAwait(t *testing.T, check func() (bool, error), message string) {
	t.Helper()
	deadline := time.NewTimer(configuredQueryStopBound)
	defer deadline.Stop()
	ticks := time.NewTicker(10 * time.Millisecond)
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

type configuredPendingQuery struct {
	done   chan configuredQueryResult
	group  *configuredQueryGroup
	joined bool
}

func (q *configuredPendingQuery) join(t *testing.T, bound time.Duration) configuredQueryResult {
	t.Helper()
	select {
	case result := <-q.done:
		q.joined = true
		return result
	case <-time.After(bound):
		t.Fatal("the admitted query did not finish inside its cancellation/join bound")
		return configuredQueryResult{err: assert.AnError}
	}
}

// beginConfiguredBlockingQuery owns one request goroutine until join. Cleanup
// cancels, stops only the verified child group if needed, and joins before the
// fixture's server/store/temporary files can be released.
func beginConfiguredBlockingQuery(t *testing.T, f *configuredConformanceFixture, ctx context.Context, cancel context.CancelFunc) *configuredPendingQuery {
	t.Helper()
	previousBudget := env.Get("ze.plugin.query.timeout")
	require.NoError(t, env.Set("ze.plugin.query.timeout", configuredQueryBudget.String()))
	t.Cleanup(func() { require.NoError(t, env.Set("ze.plugin.query.timeout", previousBudget)) })
	require.NoError(t, syscall.Mkfifo(filepath.Join(f.dir, "query-gate"), 0o600))
	// Both shells publish their own IDs before the child blocks opening the
	// FIFO. The parent waits for it, so killing only the outer query shell
	// cannot make the child disappear and satisfy the group check.
	script := "#!/bin/sh\nprintf '%s\\n' \"$$\" > query.pid\n" +
		"sh -c 'printf \"%s\\n\" \"$$\" > query-child.pid; IFS= read -r blocked < query-gate' &\nwait\n"
	require.NoError(t, os.WriteFile(filepath.Join(f.dir, "accepted.sh"), []byte(script), 0o600))
	q := &configuredPendingQuery{done: make(chan configuredQueryResult, 1)}
	go func() {
		response, err := f.request(ctx, "conformance-admin", "accepted")
		q.done <- configuredQueryResult{response: response, err: err}
	}()
	t.Cleanup(func() {
		cancel()
		if q.group != nil {
			q.group.stop(t)
		}
		if !q.joined {
			q.join(t, configuredQueryBudget+configuredQueryStopBound)
		}
	})
	var pids [2]int
	for i, name := range []string{"query.pid", "query-child.pid"} {
		path := filepath.Join(f.dir, name)
		configuredConformanceAwait(t, func() (bool, error) {
			written, err := os.ReadFile(path)
			if errors.Is(err, fs.ErrNotExist) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			pid, err := strconv.Atoi(strings.TrimSpace(string(written)))
			if err != nil {
				return false, nil //nolint:nilerr // A partially published PID line is not ready; bounded poll retries.
			}
			if pid <= 1 {
				return false, nil
			}
			pids[i] = pid
			return true, nil
		}, "the query must actually start before cancellation is judged")
	}
	group, err := syscall.Getpgid(pids[0])
	require.NoError(t, err)
	require.Greater(t, group, 1)
	require.NotEqual(t, syscall.Getpgrp(), group, "the test's own process group must never be signaled")
	childGroup, err := syscall.Getpgid(pids[1])
	require.NoError(t, err)
	require.Equal(t, group, childGroup, "the PID files must identify this query's single isolated group")
	q.group = &configuredQueryGroup{group: group, pids: pids}
	return q
}

// TestConfiguredPluginConformanceActiveCallerCancellation first admits a real
// target, then cancels only after its shell and child are observed running.
// The request must finish before the independent query budget, retain the
// typed cancellation, produce no fabricated failed-plugin report and stop both.
func TestConfiguredPluginConformanceActiveCallerCancellation(t *testing.T) {
	f := newConfiguredConformanceFixture(t)
	f.requireAccepted(t)
	ctx, cancel := context.WithCancel(t.Context())
	q := beginConfiguredBlockingQuery(t, f, ctx, cancel)
	cancel()
	result := q.join(t, configuredQueryStopBound)
	require.ErrorIs(t, result.err, context.Canceled)
	require.NotNil(t, result.response)
	assert.Nil(t, result.response.Data, "caller cancellation is not a completed failed-plugin report")
	configuredConformanceAwait(t, func() (bool, error) {
		running, err := q.group.running()
		return !running, err
	}, "cancel must stop the query's shell and child, not leave a live plugin behind")
}

// TestConfiguredPluginConformanceStopJoinsActiveQuery gives the request an
// independent live context, then stops its owning server while its shell and
// child run. Server stop and the request must finish inside the cancellation
// bound, rather than waiting for the independent twenty-second query budget.
func TestConfiguredPluginConformanceStopJoinsActiveQuery(t *testing.T) {
	f := newConfiguredConformanceFixture(t)
	f.requireAccepted(t)
	ctx, cancel := context.WithCancel(t.Context())
	q := beginConfiguredBlockingQuery(t, f, ctx, cancel)
	stopped := make(chan struct{})
	go func() { f.server.Stop(); close(stopped) }()
	t.Cleanup(func() {
		q.group.stop(t)
		select {
		case <-stopped:
		case <-time.After(configuredQueryBudget + configuredQueryStopBound):
			t.Error("the test's server stop goroutine did not join")
		}
	})
	select {
	case <-stopped:
	case <-time.After(configuredQueryStopBound):
		t.Fatal("server stop did not cancel and join the active query before its independent budget")
	}
	result := q.join(t, configuredQueryStopBound)
	require.ErrorIs(t, result.err, context.Canceled)
	require.NotNil(t, result.response)
	assert.Nil(t, result.response.Data, "server cancellation must not invent failed-plugin rows")
	configuredConformanceAwait(t, func() (bool, error) {
		running, err := q.group.running()
		return !running, err
	}, "server shutdown must leave neither query shell nor child running")
}

type configuredConformanceRejectListener struct{ calls int }

func (*configuredConformanceRejectListener) Addresses() []string { return []string{"127.0.0.1:3443"} }
func (l *configuredConformanceRejectListener) Reconfigure(context.Context, []string) error {
	l.calls++
	return assert.AnError
}

// TestConfiguredPluginConformanceRejectedReloadRetainsAcceptedTarget fails
// listener migration after candidate application/provider refresh, then checks
// actual source/provider rollback and the old target's unchanged authority.
func TestConfiguredPluginConformanceRejectedReloadRetainsAcceptedTarget(t *testing.T) {
	f := newConfiguredConformanceFixture(t)
	f.requireAccepted(t)
	before, err := storage.ReadActiveConfig(f.store, f.path)
	require.NoError(t, err)
	priorProvider, err := snapshotProvider(f.provider)
	require.NoError(t, err)
	candidate := configuredConformanceConfig("accepted", "candidate-only") +
		"environment { web { enabled true; server main { ip 127.0.0.1; port 3444; } } }\n"
	parsed, err := zeconfig.ReadConfig(candidate, f.path, nil)
	require.NoError(t, err)
	_, err = storage.WriteCandidateVersion(f.store, f.path, []byte(candidate), time.Now())
	require.NoError(t, err)
	// As in the initial fixture, the domain diff is empty, but the complete
	// reload still refreshes the provider before the injected listener failure.
	f.coordinator.SetConfigTree(parsed.Tree.ToRunningMap())
	load := func() (map[string]any, *zeconfig.Tree, reloadSelection, error) {
		return parsed.Tree.ToRunningMap(), parsed.Tree, nil, nil
	}
	listener := &configuredConformanceRejectListener{}
	migrator := newListenerMigrator()
	migrator.web = listener
	err = doReload(f.server, f.provider, f.store, f.path, load, migrator)
	require.ErrorIs(t, err, assert.AnError)
	require.Equal(t, 1, listener.calls, "the rejection must reach the late listener step")
	active, err := storage.ReadActiveConfig(f.store, f.path)
	require.NoError(t, err)
	require.Equal(t, before, active, "a refused reload must retain its accepted source")
	afterProvider, err := snapshotProvider(f.provider)
	require.NoError(t, err)
	require.Equal(t, priorProvider, afterProvider, "candidate provider state must be rolled back")
	_, _, present, err := storage.ReadCandidateConfig(f.store, f.path)
	require.NoError(t, err)
	require.False(t, present, "the rejected candidate must not stay staged")
	f.requireAccepted(t)
	response, err := f.request(t.Context(), "conformance-admin", "candidate-only")
	require.Error(t, err)
	require.NotNil(t, response)
	assert.NotContains(t, err.Error(), "unknown command")
	assert.Nil(t, response.Data)
	f.requireStarts(t, "")
}
