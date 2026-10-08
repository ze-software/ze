// Design: docs/architecture/api/architecture.md -- API engine core
// Related: types.go -- shared types used by the engine
// Related: config_session.go -- config session manager for API use

package api

import (
	"context"
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Errors returned by the engine.
var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrNotFound     = errors.New("command not found")
)

// Executor runs a command on behalf of a caller and returns the typed
// response. It is an alias for the unified plugin.CommandDispatcher: the API
// engine is one more consumer of the single command dispatcher every surface
// shares. Returning *plugin.Response (not a flattened string) lets typed Data
// flow to the REST/gRPC edge without an intermediate marshal-to-string then
// unmarshal-to-any round trip.
type Executor = plugin.CommandDispatcher

// CommandSource returns all available commands with metadata, or the error
// that kept the metadata from being built (a YANG schema the loader refused).
// An error is never answered as an empty list.
type CommandSource func() ([]CommandMeta, error)

// AuthChecker checks whether a user is allowed to run a command.
// Returns true if authorized.
type AuthChecker func(username, command string) bool

// StreamSource creates a streaming event channel for a command.
// Returns the event channel, a cancel function, and any error.
// The channel is closed when the stream ends or cancel is called.
// Caller MUST call the cancel function when done to release resources.
type StreamSource func(ctx context.Context, caller CallerIdentity, command string) (<-chan string, func(), error)

// APIEngine is the shared backend for REST and gRPC transports.
// Transports call engine methods only -- never the dispatcher directly.
// MUST call NewAPIEngine to construct.
type APIEngine struct {
	executor Executor
	commands CommandSource
	auth     AuthChecker
	stream   StreamSource
}

// NewAPIEngine creates an API engine.
// All dependencies are provided as functions so the engine package has no
// import dependency on the dispatcher, plugin server, or YANG packages.
// stream may be nil if streaming is not supported.
func NewAPIEngine(exec Executor, cmds CommandSource, auth AuthChecker, stream StreamSource) *APIEngine {
	return &APIEngine{
		executor: exec,
		commands: cmds,
		auth:     auth,
		stream:   stream,
	}
}

// ListCommands returns all available commands with metadata.
// If prefix is non-empty, only commands whose name starts with prefix are returned.
// This is a byte-level prefix match, not word-boundary: "peer" matches "peering" too.
// It returns the command source's error when the metadata cannot be built.
func (e *APIEngine) ListCommands(req *ListCommandsRequest) ([]CommandMeta, error) {
	all, err := e.commands()
	if err != nil {
		return nil, err
	}
	if req.Prefix == "" {
		return all, nil
	}
	var filtered []CommandMeta
	for _, cmd := range all {
		if strings.HasPrefix(cmd.Name, req.Prefix) {
			filtered = append(filtered, cmd)
		}
	}
	return filtered, nil
}

// DescribeCommand returns metadata for a single command.
// Returns ErrNotFound if the command does not exist, and the command source's
// error when the metadata cannot be built.
func (e *APIEngine) DescribeCommand(req *DescribeCommandRequest) (CommandMeta, error) {
	all, err := e.commands()
	if err != nil {
		return CommandMeta{}, err
	}
	for _, cmd := range all {
		if cmd.Name == req.Path {
			return cmd, nil
		}
	}
	return CommandMeta{}, ErrNotFound
}

// unauthorizedError builds the operator-facing text for a command this engine
// refused. It quotes the command because REST/gRPC callers send arbitrary
// strings, and an unquoted empty or whitespace command reads as a truncated
// message.
func unauthorizedError(command string) string {
	var tb textbuf.Buffer
	return tb.Str(plugin.UnauthorizedMessage).Str(": ").Quoted(command).String()
}

// Execute runs a command and returns the result.
// Returns ErrUnauthorized if the auth checker denies the request.
func (e *APIEngine) Execute(ctx context.Context, req *ExecuteRequest) (*ExecResult, error) {
	readOnly, err := e.commandReadOnly(req.Command)
	if err != nil {
		return &ExecResult{
			Status: StatusError,
			Error:  err.Error(),
		}, err
	}
	if req.Caller.ReadOnly && !readOnly {
		return &ExecResult{
			Status: StatusError,
			Error:  unauthorizedError(req.Command),
		}, ErrUnauthorized
	}
	if e.auth != nil && !e.auth(req.Caller.Username, req.Command) {
		return &ExecResult{
			Status: StatusError,
			Error:  unauthorizedError(req.Command),
		}, ErrUnauthorized
	}

	resp, err := e.executor(ctx, req.Caller, req.Command)
	if err != nil {
		return &ExecResult{
			Status: StatusError,
			Error:  err.Error(),
		}, err
	}
	// The executor is the unified command dispatcher: it already returns the
	// typed envelope with typed Data. Carry it straight to the transport edge
	// (finding 3) -- no marshal-to-string then unmarshal-to-any round trip.
	if resp == nil {
		return &ExecResult{Status: StatusDone}, nil
	}
	return resp, nil
}

// Stream starts a streaming command and returns a channel that delivers events.
// The caller MUST call the returned cancel function when done.
// Returns ErrUnauthorized if the auth checker denies the request.
// Returns an error if streaming is not configured.
func (e *APIEngine) Stream(ctx context.Context, req *StreamRequest) (<-chan string, func(), error) {
	if e.stream == nil {
		return nil, nil, errors.New("streaming not supported")
	}
	readOnly, err := e.commandReadOnly(req.Command)
	if err != nil {
		return nil, nil, err
	}
	if req.Caller.ReadOnly && !readOnly {
		return nil, nil, ErrUnauthorized
	}
	if e.auth != nil && !e.auth(req.Caller.Username, req.Command) {
		return nil, nil, ErrUnauthorized
	}
	return e.stream(ctx, req.Caller, req.Command)
}

// commandReadOnly answers whether the longest registered command prefixing
// command is read-only. It returns the command source's error rather than
// false, because false would refuse every read-only caller with a misleading
// "unauthorized".
func (e *APIEngine) commandReadOnly(command string) (bool, error) {
	if e.commands == nil {
		return false, nil
	}
	all, err := e.commands()
	if err != nil {
		return false, err
	}
	input := strings.ToLower(strings.TrimSpace(command))
	longest := 0
	readOnly := false
	for _, cmd := range all {
		name := strings.ToLower(strings.TrimSpace(cmd.Name))
		if name == "" || len(name) <= longest {
			continue
		}
		if !strings.HasPrefix(input, name) {
			continue
		}
		if len(input) != len(name) && input[len(name)] != ' ' {
			continue
		}
		longest = len(name)
		readOnly = cmd.ReadOnly
	}
	return readOnly, nil
}
