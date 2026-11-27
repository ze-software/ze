// Design: docs/architecture/api/process-protocol.md -- plugin runtime inspection router and non-destructive host fallback
// Related: server.go -- Server holds inspectMu and inspectInFlight
// Related: ../process/host_inspect.go -- HostProcessSnapshot and Process.HostInspect

package server

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	// runtimeInspectMaxInFlight bounds concurrent runtime-inspect calls against
	// one plugin process, well below the SDK worker's 16-slot queue.
	runtimeInspectMaxInFlight = 4

	processModeInternal = "internal"
	processModeExternal = "external"

	fallbackReasonTimeout        = "timeout"
	fallbackReasonUnsupported    = "unsupported"
	fallbackReasonTransportError = "transport-error"
)

// PluginInspectResult holds the outcome of one Server.InspectPlugin call.
// When Fallback is true, Output is nil and Host carries a strictly read-only,
// non-destructive host-side snapshot of the external plugin process.
type PluginInspectResult struct {
	Plugin         string
	ProcessMode    string
	PID            int
	Output         *rpc.RuntimeInspectOutput
	Fallback       bool
	FallbackReason string
	RPCError       string
	Host           *process.HostProcessSnapshot
}

// PluginHostInspectResult holds the host-side process mode and OS snapshot
// returned by Server.InspectPluginHost for show system memory | plugin <name>.
type PluginHostInspectResult struct {
	Plugin      string
	ProcessMode string
	Host        process.HostProcessSnapshot
}

// InspectPlugin sends a bounded read-only runtime-inspect callback to the
// running plugin endpoint named name. When an external plugin's RPC fails or
// times out, it never signals the child and instead returns a non-destructive
// host-side process snapshot with Fallback set to true.
func (s *Server) InspectPlugin(ctx context.Context, name string, input rpc.RuntimeInspectInput) (*PluginInspectResult, *opererr.Error) {
	if s == nil {
		return nil, opererr.New(
			"plugin inspection is unavailable",
			"the daemon plugin server is not running in this context",
			"ze show system subsystem list",
		)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	pm := s.ProcessManager()
	if pm == nil {
		return nil, unknownPluginInspectError(name)
	}
	proc := pm.GetProcess(name)
	if proc == nil {
		return nil, unknownPluginInspectError(name)
	}

	release, ok := s.acquireInspectSlot(proc)
	if !ok {
		return nil, opererr.New(
			"plugin "+strconv.Quote(name)+" runtime inspection is busy",
			"four runtime inspections are already in flight for this plugin",
			"Wait for an in-flight inspection to finish and retry",
		)
	}
	defer release()

	workCtx, cancel := rpc.RuntimeInspectCallContext(ctx, &input)
	defer cancel()
	if err := input.Validate(time.Now()); err != nil {
		return nil, opererr.Wrap(
			err,
			"invalid runtime inspection request for plugin "+strconv.Quote(name),
			"the runtime inspection parameters failed validation",
			"ze show system subsystem list",
		)
	}

	mode := pluginProcessMode(proc)

	var rpcErr error
	if conn := proc.Conn(); conn != nil {
		out, err := conn.SendRuntimeInspect(workCtx, input)
		if err == nil && out != nil {
			return &PluginInspectResult{
				Plugin:      name,
				ProcessMode: mode,
				PID:         out.PID,
				Output:      out,
			}, nil
		}
		if err != nil {
			rpcErr = err
		} else {
			rpcErr = errors.New("runtime-inspect returned no output")
		}
	} else {
		rpcErr = ErrPluginConnectionClosed
	}

	if proc.IsExternal() {
		reason := classifyInspectFallback(rpcErr)
		if shouldFallbackToHostInspect(rpcErr, reason) {
			host := proc.HostInspect()
			return &PluginInspectResult{
				Plugin:         name,
				ProcessMode:    processModeExternal,
				PID:            host.PID,
				Fallback:       true,
				FallbackReason: reason,
				RPCError:       rpcErr.Error(),
				Host:           &host,
			}, nil
		}
	}

	return nil, opererr.Wrap(
		rpcErr,
		"cannot inspect plugin "+strconv.Quote(name),
		"the runtime-inspect callback failed",
		"ze show system subsystem list",
	)
}

// InspectPluginHost returns a non-destructive host-side snapshot of the running
// plugin process named name without issuing an RPC or signaling the process.
func (s *Server) InspectPluginHost(name string) (*PluginHostInspectResult, *opererr.Error) {
	if s == nil {
		return nil, opererr.New(
			"plugin inspection is unavailable",
			"the daemon plugin server is not running in this context",
			"ze show system subsystem list",
		)
	}
	pm := s.ProcessManager()
	if pm == nil {
		return nil, unknownPluginInspectError(name)
	}
	proc := pm.GetProcess(name)
	if proc == nil {
		return nil, unknownPluginInspectError(name)
	}
	return &PluginHostInspectResult{
		Plugin:      name,
		ProcessMode: pluginProcessMode(proc),
		Host:        proc.HostInspect(),
	}, nil
}

func (s *Server) acquireInspectSlot(proc *process.Process) (func(), bool) {
	s.inspectMu.Lock()
	if s.inspectInFlight[proc] >= runtimeInspectMaxInFlight {
		s.inspectMu.Unlock()
		return nil, false
	}
	if s.inspectInFlight == nil {
		s.inspectInFlight = make(map[*process.Process]int)
	}
	s.inspectInFlight[proc]++
	s.inspectMu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			s.inspectMu.Lock()
			if s.inspectInFlight != nil {
				n := s.inspectInFlight[proc] - 1
				if n <= 0 {
					delete(s.inspectInFlight, proc)
				} else {
					s.inspectInFlight[proc] = n
				}
			}
			s.inspectMu.Unlock()
		})
	}, true
}

func pluginProcessMode(proc *process.Process) string {
	if proc.IsExternal() {
		return processModeExternal
	}
	return processModeInternal
}

func unknownPluginInspectError(name string) *opererr.Error {
	quoted := strconv.Quote(name)
	return opererr.New(
		"plugin "+quoted+" is not running",
		"no running plugin process matches endpoint "+quoted,
		"ze show system subsystem list",
	)
}

func shouldFallbackToHostInspect(err error, reason string) bool {
	var callErr *rpc.RPCCallError
	if !errors.As(err, &callErr) {
		return true
	}
	if reason == fallbackReasonUnsupported {
		return true
	}
	return reason == fallbackReasonTimeout
}

func classifyInspectFallback(err error) string {
	if err == nil {
		return fallbackReasonTransportError
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fallbackReasonTimeout
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "deadline") {
		return fallbackReasonTimeout
	}
	if strings.Contains(lower, "timeout") {
		return fallbackReasonTimeout
	}
	if strings.Contains(lower, "unknown method") {
		return fallbackReasonUnsupported
	}
	if strings.Contains(lower, "not supported") {
		return fallbackReasonUnsupported
	}
	if strings.Contains(lower, "unsupported") {
		return fallbackReasonUnsupported
	}
	return fallbackReasonTransportError
}
