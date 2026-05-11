// Design: docs/architecture/mcp/overview.md -- MCP task state machine (2025-11-25)

package mcp

import "errors"

// TaskState is the typed state-machine enum for MCP task lifecycle.
// Zero value (TaskUnspecified) is invalid; a read that encounters it
// indicates corruption.
//
// Reference: https://modelcontextprotocol.io/specification/2025-11-25/basic/utilities/tasks
type TaskState uint8

const (
	TaskUnspecified   TaskState = 0
	TaskWorking       TaskState = 1
	TaskInputRequired TaskState = 2
	TaskCompleted     TaskState = 3
	TaskFailed        TaskState = 4
	TaskCancelled     TaskState = 5
)

var errInvalidTaskState = errors.New("mcp: invalid task state")

// Wire-format state names per MCP 2025-11-25. The spec mandates the
// double-L spelling for the terminal cancel state; the wire value is
// not negotiable.
const (
	taskStateWireWorking       = "working"
	taskStateWireInputRequired = "input_required"
	taskStateWireCompleted     = "completed"
	taskStateWireFailed        = "failed"
	taskStateWireCancelled     = "cancelled" //nolint:misspell // MCP spec wire value
)

var taskStateNames = [6]string{
	"",
	taskStateWireWorking,
	taskStateWireInputRequired,
	taskStateWireCompleted,
	taskStateWireFailed,
	taskStateWireCancelled,
}

var taskStateFromName = map[string]TaskState{
	taskStateWireWorking:       TaskWorking,
	taskStateWireInputRequired: TaskInputRequired,
	taskStateWireCompleted:     TaskCompleted,
	taskStateWireFailed:        TaskFailed,
	taskStateWireCancelled:     TaskCancelled,
}

func (s TaskState) String() string {
	if int(s) < len(taskStateNames) {
		return taskStateNames[s]
	}
	return ""
}

func (s TaskState) IsTerminal() bool {
	return s == TaskCompleted || s == TaskFailed || s == TaskCancelled
}

func (s TaskState) MarshalText() ([]byte, error) {
	n := s.String()
	if n == "" {
		return nil, errInvalidTaskState
	}
	return []byte(n), nil
}

func (s *TaskState) UnmarshalText(b []byte) error {
	v, ok := taskStateFromName[string(b)]
	if !ok {
		return errInvalidTaskState
	}
	*s = v
	return nil
}
