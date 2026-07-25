// Design: docs/architecture/testing/ci-format.md -- engine-step executor plugin

package cli

import (
	"context"
	"log/slog"
	"time"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/test/runner"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// engineStepsPluginName is the plugin name .ci files must declare:
//
//	plugin { external engine-steps { run "ze-test engine-steps ./engine-steps.json" } }
//
// The spawn env binds the connect-back token to this exact name.
const engineStepsPluginName = "engine-steps"

// cmdEngineSteps is the spawned executor for .ci engine-step directives
// (command=/stream=/expect=output|event|stream, see
// internal/test/runner/engine_steps.go). It connects back to the daemon as a
// regular external plugin, waits for OnAllPluginsReady -- the engine's
// sanctioned point for cross-plugin dispatch -- runs the steps from the JSON
// file the runner materialized into the tmpfs dir, and reports failures via
// the ZE-OBSERVER-FAIL sentinel the runner already gates on
// (internal/test/runner/runner_validate.go). On completion (pass or fail) it
// asks the daemon to shut down so the test finishes.
func cmdEngineSteps(args []string) int {
	if len(args) != 1 {
		slog.Error("engine-steps: usage: ze-test engine-steps <steps.json>")
		return 1
	}
	data, err := cliio.ReadFile(args[0]) // "-" reads stdin
	if err != nil {
		slog.Error("ZE-OBSERVER-FAIL: engine-steps: read steps file", "path", args[0], "error", err)
		return 1
	}
	steps, err := runner.UnmarshalEngineSteps(data)
	if err != nil {
		slog.Error("ZE-OBSERVER-FAIL: engine-steps: parse steps file", "path", args[0], "error", err)
		return 1
	}

	conn, err := sdk.DialTLSEnvRaw(engineStepsPluginName)
	if err != nil {
		slog.Error("ZE-OBSERVER-FAIL: engine-steps: TLS connect-back failed", "error", err)
		return 1
	}

	p := sdk.NewWithConn(engineStepsPluginName, conn)
	buf := runner.NewEngineEventBuffer()
	p.OnEvent(buf.OnEvent)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stepResult := make(chan error, 1)
	p.OnAllPluginsReady(func() error {
		// The handler must return promptly (it answers an engine RPC); the
		// steps dispatch further RPCs, so they run in their own goroutine.
		go func() {
			var dispatch runner.EngineDispatch = func(dctx context.Context, command string) (string, string, error) {
				callCtx, callCancel := context.WithTimeout(dctx, 30*time.Second)
				defer callCancel()
				status, data, derr := p.DispatchCommand(callCtx, command)
				return status, string(data), derr
			}
			err := runner.RunEngineSteps(ctx, dispatch, buf, steps)
			if err != nil {
				slog.Error("ZE-OBSERVER-FAIL: engine-steps", "error", err)
			} else {
				slog.Info("engine-steps: all steps passed", "steps", len(steps))
			}
			// Pass or fail, end the test: ask the daemon to shut down.
			shutCtx, shutCancel := context.WithTimeout(ctx, 10*time.Second)
			if _, _, shutErr := p.DispatchCommand(shutCtx, "request shutdown"); shutErr != nil {
				slog.Warn("engine-steps: shutdown dispatch failed", "error", shutErr)
			}
			shutCancel()
			stepResult <- err
		}()
		return nil
	})

	runErr := p.Run(ctx, sdk.Registration{})

	select {
	case err := <-stepResult:
		if err != nil {
			return 1
		}
	default:
		// Run ended before the steps ran (daemon died early); the runner's
		// own expectations report the daemon-side failure.
		if runErr != nil {
			slog.Error("ZE-OBSERVER-FAIL: engine-steps: plugin loop ended before steps ran", "error", runErr)
			return 1
		}
	}
	return 0
}
