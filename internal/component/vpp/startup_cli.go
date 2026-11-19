// Design: docs/guide/vpp.md -- synchronous original-file startup receipt
// Related: vpp.go -- managed PID binding and failed-child retirement
// API: https://github.com/FDio/vpp/blob/cbba0451bb0af02a3ab8e163f6f99062782258e6/src/vlibmemory/vlib_api.c
// vl_api_cli_inband_t_handler waits for vlib_cli_input and returns its result.
// GoVPP v0.13.0: https://github.com/FDio/govpp/blob/v0.13.0/api/api.go
package vpp

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/ze-software/ze/internal/core/opererr"
)

// ExecuteStartupCLI executes the original command file once on the supplied
// connected generation. A successful return requires the exact typed native
// completion, zero Retval, stream Close and an unchanged live generation.
// The caller MUST supply a non-nil context and retain its original connector
// and generation. On an uncertain result it MUST NOT replay the command on
// another connection. This operation grants no process-retirement authority.
// Safe for concurrent use on distinct startup lifetimes; the caller owns
// serialization and one invocation per file for each native process lifetime.
func ExecuteStartupCLI(ctx context.Context, connector *Connector, generation uint64, path string) (failure error) {
	defer func() {
		if failure != nil {
			failure = startupCLIExecutionError(path, failure)
		}
	}()
	if ctx == nil {
		return errors.New("startup CLI requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if connector == nil {
		return errors.New("startup CLI requires the original VPP connector")
	}
	if generation == 0 {
		return errors.New("startup CLI requires a connected generation")
	}
	if path == "" {
		return errors.New("startup CLI requires the original file path")
	}
	if err := validateStartupCLIPath(path); err != nil {
		return err
	}
	stream, err := connector.NewStreamForGeneration(ctx, generation)
	if err != nil {
		return err
	}
	if stream == nil {
		return errors.New("startup CLI returned no native stream")
	}
	defer func() {
		failure = errors.Join(failure, stream.Close())
		failure = errors.Join(failure, ctx.Err())
		current, err := connector.ConnectionGeneration()
		if err != nil {
			failure = errors.Join(failure, err)
			return
		}
		if current != generation {
			failure = errors.Join(failure, errVppGenerationChanged)
		}
	}()
	if err := stream.SendMsg(&startupCLIRequest{Cmd: "exec " + path}); err != nil {
		return err
	}
	message, err := stream.RecvMsg()
	if err != nil {
		return err
	}
	reply, ok := message.(*startupCLIReply)
	if !ok {
		return fmt.Errorf("startup CLI returned unexpected reply %T", message)
	}
	if reply == nil {
		return errors.New("startup CLI returned no completion reply")
	}
	if reply.Retval != 0 {
		return fmt.Errorf("native CLI result %d: %s", reply.Retval, reply.Reply)
	}
	return nil
}

func startupCLIExecutionError(path string, cause error) error {
	return opererr.Wrap(cause, "VPP startup command file "+strconv.Quote(path)+" did not complete",
		"Correct the native commands or file access before restarting VPP; managed services require successful completion.",
		"ze doctor")
}

// activateManagedRuntime is the post-bind startup barrier. Its caller MUST
// pass only the child receipt returned by this connector's bindManagedProcess.
// Failure follows runOnce's original kill, Wait and retirement branch. External
// supervision owns its own bootstrap; neither that mode nor an absent optional
// file opens or executes a supervisor's file. No worker or reconnect hook owns
// this operation, so it runs once for each manager-owned child startup.
func (m *VPPManager) activateManagedRuntime(ctx context.Context, owned managedConnection,
	connect func(string) (runtimeStatsClient, error)) (*runtimeStats, error) {
	if m.settings.External {
		return m.activateRuntimeStats(ctx, connect)
	}
	if m.settings.StartupCLIFile == "" {
		return m.activateRuntimeStats(ctx, connect)
	}
	if owned.connector != m.connector.Identity() {
		return nil, startupCLIExecutionError(m.settings.StartupCLIFile, errors.New("startup CLI belongs to another supervised connector"))
	}
	bounded, cancel := context.WithTimeout(ctx, startupReadyTimeout)
	defer cancel()
	if err := ExecuteStartupCLI(bounded, m.connector, owned.generation, m.settings.StartupCLIFile); err != nil {
		return nil, err
	}
	return m.activateRuntimeStatsGeneration(ctx, owned.generation, connect)
}
