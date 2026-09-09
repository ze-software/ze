// Design: docs/architecture/api/process-protocol.md -- how an external plugin is started and stopped
// Overview: process.go -- startExternal, the live start these tests exercise
//
// Goal: prove the daemon's stop reaches the plugin, and not only the shell the
// run string was given to. Method: start a real external plugin whose run
// string leaves the plugin a grandchild of the daemon, stop it the way the
// daemon stops it, and ask the kernel whether the plugin is still there.

package process

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
)

// pluginStartBudget bounds the wait for the fake plugin to write its pid. The
// fork is two shells and a sleep, so the budget covers a loaded machine rather
// than any work the plugin does.
const pluginStartBudget = 10 * time.Second

// processStopBudget bounds how long a test waits for a stopped process to leave
// the process table. The signal is delivered before the start returns, so the
// wait covers the kernel's own reaping and nothing else.
const processStopBudget = 2 * time.Second

// TestStopReachesThePluginTheShellStarted holds the stop the daemon owes: a
// plugin the daemon started is gone when the daemon has stopped it.
//
// The run string is one a shell cannot exec-optimize, which is the shape that
// separates a stop of the group from a stop of the direct child: the shell
// stays alive with the plugin as its own child, so a kill aimed at the shell
// leaves the plugin running and connected to nothing. An operator then meets a
// plugin holding its listener, its socket and its lock after the daemon that
// started it has gone.
//
// The plugin never connects back, so the start fails at the TLS wait. That is
// the point at which a stop must already have reached the whole group, and it
// is reached here by canceling the context the process context derives from,
// which is what (*Process).Stop cancels.
//
// VALIDATES: the daemon's stop of an external plugin reaches every process the
// run string started, not the shell alone.
// PREVENTS: a plugin left running, holding its listeners and its locks, after
// the daemon stopped it or died.
func TestStopReachesThePluginTheShellStarted(t *testing.T) {
	var config net.ListenConfig
	listener, err := config.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for the plugin acceptor: %v", err)
	}
	acceptor := ipc.NewPluginAcceptor(listener, "a-plugin-secret-of-at-least-32-chars", nil)
	t.Cleanup(acceptor.Stop)

	pidPath := filepath.Join(t.TempDir(), "plugin.pid")
	run := "sh -c 'echo $$ > " + pidPath + "; exec sleep 30' & wait"

	proc := NewProcess(plugin.PluginConfig{Name: "stop-group-plugin", Run: run, Encoder: "json"})
	proc.SetAcceptor(acceptor)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := make(chan error, 1)
	go func() { started <- proc.StartWithContext(ctx) }()

	pid := waitForPluginPid(t, pidPath)
	t.Cleanup(func() {
		// The test stops what the product failed to stop, so a failure here
		// leaves no process sleeping for another 30 seconds. The signal fails
		// when the product did stop it, which is the passing case.
		_ = syscall.Kill(pid, syscall.SIGKILL) //nolint:errcheck // the process is already gone when the product stopped it
	})

	cancel()

	select {
	case <-started:
	case <-time.After(pluginStartBudget):
		t.Fatal("the start never returned after the stop")
	}

	proc.Stop()
	stopCtx, stopCancel := context.WithTimeout(context.Background(), pluginStartBudget)
	defer stopCancel()
	if err := proc.Wait(stopCtx); err != nil {
		t.Errorf("the process goroutines did not finish after the stop: %v", err)
	}

	if processAlive(pid, processStopBudget) {
		t.Fatalf("the plugin at pid %d is still running after the stop: the stop reached the shell and not the plugin", pid)
	}
}

// waitForPluginPid answers the pid the fake plugin wrote, and fails the test
// when nothing writes one within the budget: a plugin that never started proves
// nothing about stopping it.
func waitForPluginPid(t *testing.T, path string) int {
	t.Helper()

	deadline := time.Now().Add(pluginStartBudget)
	for {
		written, err := os.ReadFile(path)
		if err == nil {
			pid, convErr := strconv.Atoi(strings.TrimSpace(string(written)))
			if convErr == nil && pid > 0 {
				return pid
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("the fake plugin wrote no pid within %s, so it never started", pluginStartBudget)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// processAlive answers whether pid is still in the process table, waiting up to
// budget for it to go. Signal 0 asks the kernel about the process and delivers
// nothing to it.
func processAlive(pid int, budget time.Duration) bool {
	deadline := time.Now().Add(budget)
	for {
		if err := syscall.Kill(pid, 0); err != nil {
			return false
		}
		if time.Now().After(deadline) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
}
