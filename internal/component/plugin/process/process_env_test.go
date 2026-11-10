// Design: docs/architecture/config/environment.md -- OS spellings, the shell spelling
// Overview: process.go -- startExternal, the live start this test exercises
//
// Goal: prove a forked plugin reads every ze key the daemon holds, whatever
// spelling the daemon holds it under. Method: start a real external plugin the
// way the daemon starts one, have its run string write the environment it was
// given, and read that file.

package process

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
)

// TestForkedPluginReadsEveryZeKeySpelling holds what an operator or a test
// that sets a ze key for a forked plugin relies on: the plugin reads it.
//
// The run string goes through /bin/sh -c, and dash drops every variable whose
// name is not a shell identifier. A dotted key, the spelling env.Set writes,
// and a hyphenated key had no name that survived, so the plugin read the
// key's default and said nothing.
//
// VALIDATES: the plugin's environment carries a dotted key, a dotted and
// hyphenated key, and the dotted value of a key also set in its underscore
// spelling, each once under its shell spelling; a connect-back variable the
// daemon inherited is replaced by the one the daemon writes.
// PREVENTS: a forked plugin silently running on a key's default.
func TestForkedPluginReadsEveryZeKeySpelling(t *testing.T) {
	t.Setenv("ze.test.fork.dotted", "dotted")
	t.Setenv("ZE_TEST_FORK_DOTTED", "underscore")
	t.Setenv("ze.log.fork.shell-spelling", "debug")
	t.Setenv("ze.plugin.hub.token", "an-inherited-token-the-plugin-must-not-read")

	var config net.ListenConfig
	listener, err := config.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for the plugin acceptor: %v", err)
	}
	acceptor := ipc.NewPluginAcceptor(listener, "a-plugin-secret-of-at-least-32-chars", nil)
	t.Cleanup(acceptor.Stop)

	dir := t.TempDir()
	envPath := filepath.Join(dir, "plugin.env")
	pidPath := filepath.Join(dir, "plugin.pid")
	run := "env > " + envPath + "; sh -c 'echo $$ > " + pidPath + "; exec sleep 30' & wait"

	proc := NewProcess(plugin.PluginConfig{Name: "env-spelling-plugin", Run: run, Encoder: "json"})
	proc.SetAcceptor(acceptor)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := make(chan error, 1)
	go func() { started <- proc.StartWithContext(ctx) }()

	pid := waitForPluginPid(t, pidPath)
	t.Cleanup(func() {
		_ = syscall.Kill(pid, syscall.SIGKILL) //nolint:errcheck // the process is already gone when the product stopped it
	})
	cancel()
	select {
	case startErr := <-started:
		if !errors.Is(startErr, context.Canceled) {
			t.Errorf("the canceled start returned %v, want context.Canceled", startErr)
		}
	case <-time.After(pluginStartBudget):
		t.Fatal("the start never returned after the stop")
	}
	proc.Stop()
	stopCtx, stopCancel := context.WithTimeout(context.Background(), pluginStartBudget)
	defer stopCancel()
	if err := proc.Wait(stopCtx); err != nil {
		t.Errorf("the process goroutines did not finish after the stop: %v", err)
	}

	written, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("the plugin wrote no environment: %v", err)
	}
	child := make(map[string][]string)
	for line := range strings.SplitSeq(string(written), "\n") {
		name, value, named := strings.Cut(line, "=")
		if named {
			child[name] = append(child[name], value)
		}
	}

	for name, want := range map[string]string{
		"ZE_TEST_FORK_DOTTED":        "dotted",
		"ZE_LOG_FORK_SHELL_SPELLING": "debug",
	} {
		values := child[name]
		if len(values) != 1 || values[0] != want {
			t.Errorf("the plugin reads %s as %q, want exactly %q", name, values, want)
		}
	}
	tokens := child["ZE_PLUGIN_HUB_TOKEN"]
	if len(tokens) != 1 {
		t.Fatalf("the plugin reads ZE_PLUGIN_HUB_TOKEN %d times, want once", len(tokens))
	}
	if tokens[0] == "an-inherited-token-the-plugin-must-not-read" {
		t.Errorf("the plugin reads the inherited hub token instead of the one the daemon issued")
	}
}
