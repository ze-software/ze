// Design: docs/architecture/testing/ci-format.md -- engine-step executor plugin

package cli

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/test/runner"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// engineStepsPluginName is the plugin name .ci files must declare:
//
//	plugin { external engine-steps { run "le mock engine-steps ./engine-steps.json" } }
//
// The spawn env binds the connect-back token to this exact name.
const engineStepsPluginName = "engine-steps"

// CmdEngineSteps is the harness command `le mock engine-steps`, registered by
// internal/le/test/enginesteps. It answers the process exit code.
//
// CmdEngineSteps is the spawned executor for .ci engine-step directives
// (command=/stream=/expect=output|event|stream, see
// internal/test/runner/engine_steps.go). It connects back to the daemon as a
// regular external plugin, declares the one event subscription its expect=event
// steps imply, waits for OnAllPluginsReady -- the engine's
// sanctioned point for cross-plugin dispatch -- runs the steps from the JSON
// file the runner materialized into the tmpfs dir, and reports failures via
// the ZE-OBSERVER-FAIL sentinel the runner already gates on
// (internal/test/runner/runner_validate.go). On completion (pass or fail) it
// asks the daemon to shut down so the test finishes.
func CmdEngineSteps(args []string) int {
	if len(args) != 1 {
		slog.Error("engine-steps: usage: le mock engine-steps <steps.json>")
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

	// The steps themselves say which events this run must observe, so nothing
	// else declares it. Derived before the dial so a .ci that names two
	// namespaces is refused whether or not the daemon comes up.
	sub, err := runner.EngineStepSubscriptionFor(steps)
	if err != nil {
		slog.Error("ZE-OBSERVER-FAIL: engine-steps: startup event subscription", "path", args[0], "error", err)
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

	// A startup subscription rides the "ready" RPC, so it is registered before
	// any plugin's OnAllPluginsReady runs. That is what lets an expect=event
	// step observe the FIRST delivery of an event the daemon emits while it
	// starts, which a subscribe dispatched from a step can never see.
	//
	// Envelope delivery tags each event with its (namespace, event) identity,
	// which is what tells one subscribed event from another once several share
	// this one subscription (rpc.EventEnvelope). Format is deliberately empty:
	// it is the BGP wire format (hex, base64, parsed, full), not an encoding,
	// and these steps subscribe to whichever namespace they named.
	if len(sub.Events) > 0 {
		p.SetStartupSubscriptionsIn(sub.Namespace, sub.Events, nil, "")
		p.SetEncoding("json")
		p.SetEnvelope(true)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stepResult := make(chan error, 1)
	p.OnAllPluginsReady(func() error {
		// The handler must return promptly (it answers an engine RPC); the
		// steps dispatch further RPCs, so they run in their own goroutine.
		go func() {
			dispatch := pipedEngineDispatch(func(dctx context.Context, line string) (string, []byte, error) {
				callCtx, callCancel := context.WithTimeout(dctx, 30*time.Second)
				defer callCancel()
				return p.DispatchCommand(callCtx, line)
			})
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

	runErr := p.Run(ctx, &sdk.Registration{})

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

// pipedEngineDispatch is the executor's dispatch: it sends a step's command
// line to the daemon the way SSH sends one, so a step reads what `ze cli -c`
// prints for the same line.
//
// The daemon's dispatcher answers data and applies no pipe chain: every
// operator surface splits the chain off before it dispatches and renders the
// answer through it (the SSH exec channel, the web CLI and `ze cli` all call
// command.ProcessPipesDefaultFormatChecked). Sent whole, `show doctor config f
// | json` is refused by the dispatcher, which names the line to send instead,
// so the step would read that refusal and never the rendered answer.
//
// A line with no pipe keeps the raw JSON answer: no default format is added,
// because json= predicates walk that answer (runner.RunEngineSteps). A refused
// chain and a chain the answer cannot satisfy are errors, as `ze cli -c` exits
// 1 for them, so an expect=command-error step can read them.
//
// send MUST return the daemon's status and its raw answer data.
func pipedEngineDispatch(send func(ctx context.Context, line string) (string, []byte, error)) runner.EngineDispatch {
	return func(ctx context.Context, line string) (string, string, error) {
		if !strings.Contains(line, "|") {
			status, data, err := send(ctx, line)
			return status, string(data), err
		}
		dispatched, format, pipeErr := command.ProcessPipesDefaultFormatChecked(line, "")
		if pipeErr != "" {
			return "", "", errors.New(pipeErr)
		}
		status, data, err := send(ctx, dispatched)
		if err != nil {
			return status, string(data), err
		}
		rendered := format(string(data))
		if command.IsPipeError(rendered) {
			return "", "", errors.New(rendered)
		}
		return status, rendered, nil
	}
}
