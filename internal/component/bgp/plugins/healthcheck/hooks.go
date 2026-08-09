// Design: docs/architecture/bgp/healthcheck-plugin.md -- hook execution
// Overview: healthcheck.go -- plugin lifecycle and probe management
package healthcheck

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	hookTimeout  = 30 * time.Second
	maxConcHooks = 10 // max concurrent hook goroutines per runHooks call (#5)
)

// runHooks executes hook commands for a state transition.
// State-specific hooks run first (in config order), then on-change hooks.
// Concurrency is bounded by maxConcHooks to prevent goroutine accumulation (#5).
// Hooks do NOT block the FSM (each runs in its own goroutine). Failures are
// logged. Tests observe completion through each hook's own side effect (e.g. a
// marker file), so no completion signal is exposed on the production path.
func runHooks(cfg ProbeConfig, state State) {
	var stateHooks []string
	switch state {
	case StateUp:
		stateHooks = cfg.OnUp
	case StateDown:
		stateHooks = cfg.OnDown
	case StateDisabled:
		stateHooks = cfg.OnDisabled
	case StateInit, StateRising, StateFalling, StateExit, StateEnd:
		// No state-specific hooks for these states.
	}

	sName := stateName(state)
	sem := make(chan struct{}, maxConcHooks)

	for _, cmd := range stateHooks {
		sem <- struct{}{}
		go func(c string) {
			defer func() { <-sem }()
			runSingleHook(cfg.Name, c, sName)
		}(cmd)
	}
	for _, cmd := range cfg.OnChange {
		sem <- struct{}{}
		go func(c string) {
			defer func() { <-sem }()
			runSingleHook(cfg.Name, c, sName)
		}(cmd)
	}
}

// runSingleHook executes a single hook command with 30s timeout.
func runSingleHook(probeName, command, sName string) {
	ctx, cancel := context.WithTimeout(context.Background(), hookTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command) //nolint:gosec // admin-controlled config value
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	// Minimal environment for hooks (#16): only PATH, HOME, USER, STATE.
	var tb textbuf.Buffer
	cmd.Env = []string{
		tb.Str("PATH=").Str(os.Getenv("PATH")).String(),
		tb.Reset().Str("HOME=").Str(os.Getenv("HOME")).String(),
		tb.Reset().Str("USER=").Str(os.Getenv("USER")).String(),
		tb.Reset().Str("STATE=").Str(sName).String(),
	}

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			logger().Warn("hook timed out", "probe", probeName, "command", command, "timeout", hookTimeout)
			return
		}
		logger().Warn("hook failed", "probe", probeName, "command", command, "error", err)
	}
}

func stateName(s State) string {
	switch s {
	case StateInit:
		return "INIT"
	case StateRising:
		return "RISING"
	case StateUp:
		return "UP"
	case StateFalling:
		return "FALLING"
	case StateDown:
		return "DOWN"
	case StateDisabled:
		return "DISABLED"
	case StateExit:
		return "EXIT"
	case StateEnd:
		return "END"
	}
	return "UNKNOWN"
}
