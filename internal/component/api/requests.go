// Design: docs/architecture/api/architecture.md -- domain request types for API transports

package api

import (
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ExecuteRequest is the domain request for command execution.
type ExecuteRequest struct {
	Caller  CallerIdentity
	Command string
}

// StreamRequest is the domain request for streaming command execution.
type StreamRequest struct {
	Caller  CallerIdentity
	Command string
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

// BuildCommand appends params as "key value" pairs to a command string.
// Returns an error if any key or value contains whitespace.
func BuildCommand(command string, params map[string]string) (string, error) {
	if len(params) == 0 {
		return command, nil
	}
	var b textbuf.Buffer
	b.Reset()
	b.Str(command)
	for key, val := range params {
		if strings.ContainsAny(key, " \t\n\r") {
			return "", fmt.Errorf("parameter key %q must not contain whitespace", key)
		}
		if val == "" {
			continue
		}
		if strings.ContainsAny(val, " \t\n\r") {
			return "", fmt.Errorf("parameter %q must not contain whitespace", key)
		}
		b.Byte(' ').Str(key).Byte(' ').Str(val)
	}
	return b.String(), nil
}
