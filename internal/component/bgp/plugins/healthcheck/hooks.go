// Design: plan/spec-healthcheck-0-umbrella.md -- hook execution
package healthcheck

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

const hookTimeout = 30 * time.Second

// runHooks executes hook commands for a state transition.
// State-specific hooks run first (in config order), then on-change hooks.
// Each hook runs in its own goroutine with a 30s timeout + process group kill.
// Hooks do NOT block the FSM. Failures are logged.
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

	for _, cmd := range stateHooks {
		go runSingleHook(cfg.Name, cmd, sName)
	}
	for _, cmd := range cfg.OnChange {
		go runSingleHook(cfg.Name, cmd, sName)
	}
}

// runSingleHook executes a single hook command with 30s timeout.
func runSingleHook(probeName, command, sName string) {
	ctx, cancel := context.WithTimeout(context.Background(), hookTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command) //nolint:gosec // admin-controlled config value
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("STATE=%s", sName))

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
