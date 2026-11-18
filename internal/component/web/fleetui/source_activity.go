// Design: docs/architecture/fleet-interface.md -- the alarm and task questions
// Overview: source.go -- the data source seam
// Related: state.go -- AlarmSeverity and TaskState

package fleetui

import (
	"context"
	"time"
)

// The bounds of the alarm and task answers.
const (
	// AlarmsMax bounds the open alarms of one estate.
	AlarmsMax = 1024
	// AlarmHistoryMax bounds the alarm history of one device.
	AlarmHistoryMax = 512
	// TasksMax bounds the tasks the hub keeps.
	TasksMax = 512
	// TaskAnswerBytesMax bounds the answer of one device to one task. A
	// longer answer is cut and marked.
	TaskAnswerBytesMax = 16384
)

// ActivitySource answers what is wrong in the estate now and what the hub is
// doing to it.
type ActivitySource interface {
	// Alarms answers every open alarm, at most AlarmsMax, in no stated
	// order: TopAlarms orders them.
	// Asked by: the alarms tabs, the alarms cards, the tree badges, the
	// device side panel. Answered by: fleet-ui-5.
	Alarms(ctx context.Context) ([]Alarm, error)

	// AlarmHistory answers the alarms that opened on one device over the
	// retention period, at most AlarmHistoryMax, newest first.
	// Asked by: the side panel of an alarm row. Answered by: fleet-ui-5.
	AlarmHistory(ctx context.Context, device string) ([]AlarmEvent, error)

	// Tasks answers every task the hub keeps, at most TasksMax, newest
	// first, without the per-device results.
	// Asked by: the task panel, the tasks tabs, the tasks page, the header
	// count, the spinners of the tree and the object header.
	// Answered by: fleet-ui-5.
	Tasks(ctx context.Context) ([]Task, error)

	// Task answers one task with its per-device results. An unknown
	// identifier wraps ErrObjectUnknown.
	// Asked by: the expanded row of the task panel and the side panel of a
	// task row. Answered by: fleet-ui-5.
	Task(ctx context.Context, id string) (TaskDetail, error)
}

// Alarm is one condition that is true now and needs attention.
type Alarm struct {
	// ID identifies the alarm on the hub.
	ID string
	// Kind names the registered alarm kind, such as device-offline.
	Kind     string
	Severity AlarmSeverity
	// Device is the device the alarm is about. It is empty for an alarm of
	// the hub itself.
	Device  string
	Summary string
	Detail  string
	// Since is when the condition became true.
	Since time.Time
}

// AlarmEvent is one past or present alarm of a device.
type AlarmEvent struct {
	Kind     string
	Severity AlarmSeverity
	Summary  string
	Opened   time.Time
	// Cleared reports whether the condition stopped being true.
	Cleared bool
	// ClearedAt is when it stopped. It means nothing while Cleared is false.
	ClearedAt time.Time
}

// Task is one operation the hub performs on one or more devices, without its
// per-device results.
type Task struct {
	// ID identifies the task on the hub.
	ID string
	// Kind names the registered task kind, such as config-push.
	Kind string
	// Actor is the hub user who started the task, or the hub itself.
	Actor string
	// Target says what the devices were chosen from: the estate, a group
	// node or a selection.
	Target string
	// Devices names the devices targeted, at most DevicesMax.
	Devices []string
	State   TaskState
	// Finished counts the devices that have a result, and Failed those among
	// them that failed.
	Finished int
	Failed   int
	Started  time.Time
	// Ended is when the task ended. It means nothing until State has ended.
	Ended time.Time
}

// TaskDetail is one task with its per-device results.
type TaskDetail struct {
	Task Task
	// Results holds one result per device that has one, at most DevicesMax.
	Results []TaskResult
	// AuditIDs names the audit entries the task wrote.
	AuditIDs []string
}

// TaskResult is what one device answered to a task.
type TaskResult struct {
	Device string
	State  TaskState
	// Answer is the device's answer or refusal, at most TaskAnswerBytesMax.
	Answer string
	// Truncated reports whether the answer was cut to fit.
	Truncated bool
	Duration  time.Duration
}
