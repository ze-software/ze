// Design: docs/architecture/config/environment.md -- the `environment` block and a plugin forked before the full parse
// Overview: process.go -- SetConfigEnvironment, inheritedEnvironment and startExternal
//
// Goal: prove a plugin forked before the daemon applied the config's
// `environment` block reads the block, under the operator's own environment,
// and that the daemon's environment is not written for it. Method: start a
// real external plugin the way the early start does, have its run string
// write the environment it was given, and read that file.

package process

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
)

// TestPluginForkedBeforeTheParseReadsTheEnvironmentBlock holds what a declared
// external plugin relies on since it is started before the full parse: the
// settings of the config's `environment` block reach it.
//
// VALIDATES: a block setting whose key the daemon's environment leaves unset
// reaches the plugin, once, under its shell spelling; a setting whose key the
// operator's environment already holds does not replace the operator's value;
// the daemon's own environment holds neither setting afterwards.
// PREVENTS: an ExaBGP bridge declared in the config reading exabgp.api.ack
// from its default because it was forked before the block was applied.
func TestPluginForkedBeforeTheParseReadsTheEnvironmentBlock(t *testing.T) {
	t.Setenv("ze.test.block.operator", "from-the-operator")

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

	proc := NewProcess(plugin.PluginConfig{Name: "block-env-plugin", Run: run, Encoder: "json"})
	proc.SetAcceptor(acceptor)
	// MUTATION: drop this call, or make startExternal fork from
	// env.ShellSafe(os.Environ()) again, and the plugin has no
	// ZE_TEST_BLOCK_FILLED.
	proc.SetConfigEnvironment([]string{
		"ze.test.block.filled=from-the-config",
		"ze.test.block.operator=from-the-config",
	})

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
		"ZE_TEST_BLOCK_FILLED":   "from-the-config",
		"ZE_TEST_BLOCK_OPERATOR": "from-the-operator",
	} {
		values := child[name]
		if len(values) != 1 || values[0] != want {
			t.Errorf("the plugin reads %s as %q, want exactly %q", name, values, want)
		}
	}
	if value, set := os.LookupEnv("ze.test.block.filled"); set {
		t.Errorf("the fork wrote ze.test.block.filled=%q into the daemon's own environment", value)
	}
	if value := os.Getenv("ze.test.block.operator"); value != "from-the-operator" {
		t.Errorf("the fork changed the daemon's ze.test.block.operator to %q", value)
	}
}

// TestRespawnReadsTheBlockFromTheDaemonEnvironment proves the two forks of one
// declared plugin read the same settings: the first, before the full parse,
// from the block it was given; a respawn, which is a new Process given none,
// from the daemon's environment, which holds the block once the parse applied
// it. Method: inheritedEnvironment over the daemon's environment before and
// after the block was applied.
//
// VALIDATES: the respawned plugin's environment equals the first fork's, and a
// first fork over an environment that already holds the block carries the
// setting once.
// PREVENTS: a restarted plugin losing the block, or reading it twice.
func TestRespawnReadsTheBlockFromTheDaemonEnvironment(t *testing.T) {
	cfg := plugin.PluginConfig{Name: "block-env-plugin", Run: "block-env-plugin", Encoder: "json"}
	block := []string{"ze.bgp.openwait=75", "ze.log.relay=debug"}
	beforeTheParse := []string{"PATH=/bin", "ZE_LOG_RELAY=warn"}
	// What env.Set leaves once the full parse applied the block: the key the
	// operator set keeps its value, the other is written under its dotted key.
	afterTheParse := []string{"PATH=/bin", "ZE_LOG_RELAY=warn", "ze.bgp.openwait=75"}

	first := NewProcess(cfg)
	first.SetConfigEnvironment(block)
	respawned := NewProcess(cfg)

	want := []string{"PATH=/bin", "ZE_LOG_RELAY=warn", "ZE_BGP_OPENWAIT=75"}
	if got := first.inheritedEnvironment(beforeTheParse); !slices.Equal(got, want) {
		t.Errorf("the fork before the parse inherits %q, want %q", got, want)
	}
	if got := respawned.inheritedEnvironment(afterTheParse); !slices.Equal(got, want) {
		t.Errorf("the respawn inherits %q, want %q", got, want)
	}
	if got := first.inheritedEnvironment(afterTheParse); !slices.Equal(got, want) {
		t.Errorf("a fork given the block over an environment that holds it inherits %q, want %q", got, want)
	}
	if got := respawned.inheritedEnvironment(beforeTheParse); !slices.Equal(got, beforeTheParse) {
		t.Errorf("a fork given no block inherits %q, want the daemon's environment %q", got, beforeTheParse)
	}
}
