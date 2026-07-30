// Design: docs/architecture/mcp/overview.md -- MCP task state machine (io.modelcontextprotocol/tasks)
// Related: tasks.go -- the registry that owns every entry carrying one of these states

package mcp

import "errors"

// TaskState is the typed state-machine enum for MCP task lifecycle.
// Zero value (TaskUnspecified) is invalid; a read that encounters it
// indicates corruption.
//
// Reference: https://github.com/modelcontextprotocol/ext-tasks
//
// # The deliberately absent state
//
// The extension defines a fifth state, `input_required`, which a task enters
// when its work needs more information from the client before it can finish.
// Ze cannot enter it, so it is not modeled here and `TaskInputRequired` does
// not exist.
//
// This is an omission with a reason, not an oversight. A task raises
// `inputRequests` only if the work it runs elicits, and no Ze task can: task
// eligibility is decided by the `ze:task-support` annotation (lookupTaskSupport,
// streamable_tools.go), every annotated command dispatches through
// dispatchGenerated, and the task worker is handed a zero capability set so an
// elicitation degrades to the missing-argument path rather than an interim
// result (createTask, streamable_tools.go). `tasks/update` is implemented in
// full regardless, because for a server that raises no input requests the
// extension's own tolerance rule -- verify ownership, acknowledge empty, ignore
// unknown keys -- IS the complete implementation.
//
// The trigger that reintroduces the state: a command annotated
// `ze:task-support required` gains an elicitation, or a handcrafted tool that
// elicits becomes task-eligible. At that point `TaskInputRequired`, the
// `inputRequests` payload on `tasks/get` and real `inputResponses` matching in
// `tasks/update` all become reachable and must be implemented together, reusing
// the MRTR types in mrtr.go. See docs/guide/mcp/tasks.md "Known limitations".
type TaskState uint8

const (
	TaskUnspecified TaskState = 0
	TaskWorking     TaskState = 1
	TaskCompleted   TaskState = 2
	TaskFailed      TaskState = 3
	TaskCancelled   TaskState = 4
)

var errInvalidTaskState = errors.New("mcp: invalid task state")

// Wire-format state names. The extension mandates the double-L spelling for the
// terminal cancel state; the wire value is not negotiable.
const (
	taskStateWireWorking   = "working"
	taskStateWireCompleted = "completed"
	taskStateWireFailed    = "failed"
	taskStateWireCancelled = "cancelled" //nolint:misspell // MCP spec wire value
)

var taskStateNames = [5]string{
	"",
	taskStateWireWorking,
	taskStateWireCompleted,
	taskStateWireFailed,
	taskStateWireCancelled,
}

var taskStateFromName = map[string]TaskState{
	taskStateWireWorking:   TaskWorking,
	taskStateWireCompleted: TaskCompleted,
	taskStateWireFailed:    TaskFailed,
	taskStateWireCancelled: TaskCancelled,
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
