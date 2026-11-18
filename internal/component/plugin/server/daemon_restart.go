// Design: docs/architecture/api/commands.md -- shared daemon lifecycle commands.
// Related: system.go -- shutdown and reboot response completion contract.
package server

import (
	"context"
	"errors"
	"sync"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

// CaptureRestartCompletion captures the installed restart action without
// running it. The caller MUST complete its accepted response before invoking
// the returned function. A canceled or stopping caller receives no action.
func (s *Server) CaptureRestartCompletion(ctx context.Context) (func(), error) {
	if s == nil {
		return nil, daemonRestartUnavailableError()
	}
	if ctx == nil {
		return nil, daemonRestartUnavailableError()
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, opererr.Wrap(err, "daemon restart canceled",
			"the caller's request ended before restart was accepted", "ze request restart")
	}
	if err := s.CheckAdmission(); err != nil {
		return nil, opererr.Wrap(err, "daemon restart not accepted",
			"the daemon already entered its terminal shutdown", "ze status")
	}
	action := s.daemonRestart.Load()
	if action == nil {
		return nil, daemonRestartUnavailableError()
	}
	return sync.OnceFunc(func() {
		(*action)()
		s.SignalShutdownRequested()
	}), nil
}

// SetRestartFunc installs the daemon's in-place restart action. It MUST record
// the restart cause and prepare the graceful stop; it MUST NOT reboot the host.
// Command transports MUST complete the accepted response before this action
// runs. The daemon wires it before serving restart commands. Safe for concurrent
// use; each installed action runs at most once across accepted requests.
// A nil function withdraws restart admission without revoking accepted actions.
func (s *Server) SetRestartFunc(action func()) {
	if action == nil {
		s.daemonRestart.Store(nil)
		return
	}
	once := sync.OnceFunc(action)
	s.daemonRestart.Store(&once)
}

// handleDaemonRestart captures the installed action at acceptance. A caller
// canceled before acceptance cannot request teardown; cancellation after the
// success answer cannot retract an action whose receipt can already be lost.
// The transport MUST call TransportComplete after its response write attempt.
func handleDaemonRestart(ctx *CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	if len(validated.Tokens()) > 0 {
		refusal := opererr.New("request restart does not take arguments",
			"no daemon restart was accepted", "ze request restart")
		return plugin.OperatorErrorResponse(refusal), refusal
	}
	if ctx == nil {
		return daemonRestartUnavailable()
	}
	complete, err := ctx.Server.CaptureRestartCompletion(ctx.Context())
	if err != nil {
		refusal, ok := errors.AsType[*opererr.Error](err)
		if !ok {
			refusal = opererr.Wrap(err, "daemon restart was not accepted",
				"the restart action did not run", "ze request restart")
		}
		return plugin.OperatorErrorResponse(refusal), err
	}
	resp := &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{fieldMessage: "restart initiated"},
	}
	resp.OnTransportComplete(complete)
	return resp, nil
}

func daemonRestartUnavailable() (*plugin.Response, error) {
	refusal := daemonRestartUnavailableError()
	return plugin.OperatorErrorResponse(refusal), refusal
}

func daemonRestartUnavailableError() *opererr.Error {
	return opererr.New("daemon restart not available",
		"daemon startup has not installed a restart action; wait for startup to finish",
		"ze request restart")
}
