// Design: docs/architecture/api/architecture.md -- domain request types for API transports
// Design: docs/architecture/api/commands.md -- the ordered pipe list of execute and stream

package api

import (
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
)

// ExecuteRequest is the domain request for command execution. Pipe is the
// optional ordered pipe list (command.PipeListParameter), written after the
// parameters as the chain of the invocation.
type ExecuteRequest struct {
	Caller  CallerIdentity
	Command string
	Params  map[string]string
	Pipe    []command.PipeListElement
}

// StreamRequest is the domain request for streaming command execution. Pipe
// is as on ExecuteRequest.
type StreamRequest struct {
	Caller  CallerIdentity
	Command string
	Params  map[string]string
	Pipe    []command.PipeListElement
}

// ListCommandsRequest is the domain request for listing commands.
type ListCommandsRequest struct {
	Caller CallerIdentity
	Prefix string
}

// DescribeCommandRequest is the domain request for describing a command.
type DescribeCommandRequest struct {
	Caller CallerIdentity
	Path   string
}

// ConfigSetRequest is the domain request for setting a config value.
type ConfigSetRequest struct {
	Username  string
	SessionID string
	Path      string
	Value     string
}

// ConfigDeleteRequest is the domain request for deleting a config path.
type ConfigDeleteRequest struct {
	Username  string
	SessionID string
	Path      string
}

// ConfigDiffRequest is the domain request for showing session diff.
type ConfigDiffRequest struct {
	Username  string
	SessionID string
}

// ConfigCommitRequest is the domain request for committing a session.
type ConfigCommitRequest struct {
	Username  string
	SessionID string
}

// ConfigDiscardRequest is the domain request for discarding a session.
type ConfigDiscardRequest struct {
	Username  string
	SessionID string
}

// ValidateParams preserves the transport's parameter refusal before it
// constructs a domain request. The engine also validates direct callers.
func ValidateParams(params map[string]string) error {
	for key, val := range params {
		if strings.ContainsAny(key, " \t\n\r") {
			return fmt.Errorf("parameter key %q must not contain whitespace", key)
		}
		if strings.ContainsAny(val, " \t\n\r") {
			return fmt.Errorf("parameter %q must not contain whitespace", key)
		}
	}
	return nil
}
