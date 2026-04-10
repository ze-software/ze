// Design: docs/architecture/api/architecture.md -- API engine core
// Related: types.go -- shared types used by the engine
// Related: config_session.go -- config session manager for API use

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Errors returned by the engine.
var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrNotFound     = errors.New("command not found")
)

// Executor runs a command and returns its raw output.
// The username is passed so the implementation can set CommandContext.Username
// for per-command authorization. The composition root MUST wire this through.
type Executor func(username, command string) (string, error)

// CommandSource returns all available commands with metadata.
type CommandSource func() []CommandMeta

// AuthChecker checks whether a user is allowed to run a command.
// Returns true if authorized.
type AuthChecker func(username, command string) bool

// StreamSource creates a streaming event channel for a command.
// Returns the event channel, a cancel function, and any error.
// The channel is closed when the stream ends or cancel is called.
// Caller MUST call the cancel function when done to release resources.
type StreamSource func(ctx context.Context, username, command string) (<-chan string, func(), error)

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
func (e *APIEngine) ListCommands(prefix string) []CommandMeta {
	all := e.commands()
	if prefix == "" {
		return all
	}
	var filtered []CommandMeta
	for _, cmd := range all {
		if strings.HasPrefix(cmd.Name, prefix) {
			filtered = append(filtered, cmd)
		}
	}
	return filtered
}

// DescribeCommand returns metadata for a single command.
// Returns ErrNotFound if the command does not exist.
func (e *APIEngine) DescribeCommand(path string) (CommandMeta, error) {
	for _, cmd := range e.commands() {
		if cmd.Name == path {
			return cmd, nil
		}
	}
	return CommandMeta{}, ErrNotFound
}

// Execute runs a command and returns the result.
// Returns ErrUnauthorized if the auth checker denies the request.
func (e *APIEngine) Execute(auth AuthContext, command string) (*ExecResult, error) {
	if e.auth != nil && !e.auth(auth.Username, command) {
		return &ExecResult{
			Status: StatusError,
			Error:  fmt.Sprintf("authorization denied for %q", command),
		}, ErrUnauthorized
	}

	output, err := e.executor(auth.Username, command)
	if err != nil {
		return &ExecResult{
			Status: StatusError,
			Error:  err.Error(),
		}, err
	}

	// Try to parse output as JSON for structured responses.
	var structured any
	if json.Valid([]byte(output)) {
		if jsonErr := json.Unmarshal([]byte(output), &structured); jsonErr == nil {
			return &ExecResult{Status: StatusDone, Data: structured}, nil
		}
	}

	return &ExecResult{Status: StatusDone, Data: output}, nil
}

// Stream starts a streaming command and returns a channel that delivers events.
// The caller MUST call the returned cancel function when done.
// Returns ErrUnauthorized if the auth checker denies the request.
// Returns an error if streaming is not configured.
func (e *APIEngine) Stream(ctx context.Context, auth AuthContext, command string) (<-chan string, func(), error) {
	if e.stream == nil {
		return nil, nil, errors.New("streaming not supported")
	}
	if e.auth != nil && !e.auth(auth.Username, command) {
		return nil, nil, ErrUnauthorized
	}
	return e.stream(ctx, auth.Username, command)
}
