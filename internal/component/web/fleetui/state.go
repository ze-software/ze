// Design: docs/architecture/fleet-interface.md -- the display state and the other state words
// Related: source_devices.go -- the facts the display state is derived from
// Related: derive.go -- the device view that carries the display state
//
// Every state a page shows is a typed value with a word, and where an address
// or a filter names it, a key. The word is what an operator reads; the key is
// what a filter term and an address carry. Neither is ever compared in code:
// code compares the typed value.

package fleetui

import "slices"

// enumInvalid is what a state outside its named set renders as. Only a
// defective source produces one.
const enumInvalid = "invalid"

// The words and keys several enums of this package share. The first entry of
// every table is the invalid zero value's.
const (
	enumWordUnspecified = "Unspecified"
	enumKeyUnspecified  = "unspecified"
	enumWordPending     = "Pending"
	enumKeyPending      = "pending"
	enumKeyFailed       = "failed"
)

// enumText answers the entry of table for value, or enumInvalid for a value
// the table does not hold.
func enumText[E ~uint8](table []string, value E) string {
	if int(value) < len(table) {
		return table[value]
	}

	return enumInvalid
}

// enumParse answers the value whose entry in table is text, or false. The
// first entry is the invalid zero value's and is never matched.
func enumParse[E ~uint8](table []string, text string) (E, bool) {
	at := slices.Index(table[1:], text)
	if at >= 0 {
		return E(at + 1), true
	}

	return 0, false
}

// DisplayState is the one state a device is shown in, everywhere. The zero
// value is not a state.
type DisplayState uint8

// The display states, in the order of precedence DeviceDisplayState applies.
const (
	// DisplayStateUnspecified is the invalid zero value.
	DisplayStateUnspecified DisplayState = iota
	// DisplayStateOffline means the device has no managed connection.
	DisplayStateOffline
	// DisplayStateDiverged means the device is connected and holds a locally
	// edited configuration.
	DisplayStateDiverged
	// DisplayStateDown means the device is connected and at least one health
	// component reports down.
	DisplayStateDown
	// DisplayStateDegraded means the device is connected and at least one
	// health component reports degraded.
	DisplayStateDegraded
	// DisplayStateHealthy means the device is connected and every reported
	// component is healthy.
	DisplayStateHealthy
	// DisplayStateUnknown means the device is connected and its health
	// cannot be told: no report has arrived, or a component reports a state
	// this hub does not know.
	DisplayStateUnknown
)

var (
	displayStateWords = []string{enumWordUnspecified, "Offline", "Diverged", "Down", "Degraded", "Healthy", "Unknown"}
	displayStateKeys  = []string{enumKeyUnspecified, "offline", "diverged", "down", "degraded", "healthy", "unknown"}
	// displayStateSeverities ranks the states for sorting, the worst highest:
	// down, offline, diverged, degraded, unknown, healthy. It follows the
	// alarm severities: a component down and a device offline are critical,
	// a divergence and a degradation are warnings.
	displayStateSeverities = []int{0, 5, 4, 6, 3, 1, 2}
)

// String answers the word a page writes beside the state's icon.
func (state DisplayState) String() string {
	return enumText(displayStateWords, state)
}

// Key answers the state's name in a filter term and in a group address.
func (state DisplayState) Key() string {
	return enumText(displayStateKeys, state)
}

// Severity ranks the state for sorting: a worse state has a higher rank. A
// state outside the named set ranks lowest.
func (state DisplayState) Severity() int {
	if int(state) < len(displayStateSeverities) {
		return displayStateSeverities[state]
	}

	return 0
}

// ParseDisplayState answers the state a filter term or an address names, or
// false.
func ParseDisplayState(key string) (DisplayState, bool) {
	return enumParse[DisplayState](displayStateKeys, key)
}

// DisplayStates answers every display state in the order a legend lists
// them. A legend lists a state whose count is zero too, so its shape never
// changes.
func DisplayStates() []DisplayState {
	return []DisplayState{
		DisplayStateHealthy, DisplayStateDegraded, DisplayStateDown,
		DisplayStateOffline, DisplayStateDiverged, DisplayStateUnknown,
	}
}

// DeviceDisplayState derives the display state of a device from its facts, in
// the order of precedence the fleet interface fixes: offline, diverged, down,
// degraded, then healthy or unknown.
func DeviceDisplayState(device *Device) DisplayState {
	if !device.Connection.Connected {
		return DisplayStateOffline
	}
	if device.Config.State == ConfigStateDiverged {
		return DisplayStateDiverged
	}

	var down, degraded, unjudged bool
	for _, component := range device.Health {
		switch component.State {
		case HealthStateDown:
			down = true
		case HealthStateDegraded:
			degraded = true
		case HealthStateHealthy:
			// A healthy component changes nothing.
		case HealthStateUnspecified:
			unjudged = true
		default:
			// The set is open: a device running newer software may report a
			// state this hub does not know.
			unjudged = true
		}
	}

	if down {
		return DisplayStateDown
	}
	if degraded {
		return DisplayStateDegraded
	}
	if !device.HealthReported {
		return DisplayStateUnknown
	}
	if unjudged {
		return DisplayStateUnknown
	}

	return DisplayStateHealthy
}

// HealthState is what one health component reports. The zero value is not a
// state.
type HealthState uint8

const (
	// HealthStateUnspecified is the invalid zero value.
	HealthStateUnspecified HealthState = iota
	// HealthStateHealthy means the component works.
	HealthStateHealthy
	// HealthStateDegraded means the component works with a fault.
	HealthStateDegraded
	// HealthStateDown means the component does not work.
	HealthStateDown
)

var healthStateWords = []string{enumWordUnspecified, "Healthy", "Degraded", "Down"}

// String answers the word a page shows for the component's state.
func (state HealthState) String() string {
	return enumText(healthStateWords, state)
}

// ConfigState is where a device's configuration stands against the hub's
// copy. The zero value is not a state.
type ConfigState uint8

const (
	// ConfigStateUnspecified is the invalid zero value.
	ConfigStateUnspecified ConfigState = iota
	// ConfigStateInSync means the device runs the version the hub holds.
	ConfigStateInSync
	// ConfigStatePending means the hub holds a version the device has not
	// applied yet.
	ConfigStatePending
	// ConfigStateRejected means the device refused the version the hub sent.
	ConfigStateRejected
	// ConfigStateDiverged means the device holds a locally edited
	// configuration.
	ConfigStateDiverged
	// ConfigStateNeverFetched means the device never fetched a configuration.
	ConfigStateNeverFetched
)

var (
	configStateWords = []string{enumWordUnspecified, "In sync", enumWordPending, "Rejected", "Diverged", "Never fetched"}
	configStateKeys  = []string{enumKeyUnspecified, "in-sync", enumKeyPending, "rejected", "diverged", "never-fetched"}
)

// String answers the word a page shows for the configuration state.
func (state ConfigState) String() string {
	return enumText(configStateWords, state)
}

// Key answers the state's name in a filter term.
func (state ConfigState) Key() string {
	return enumText(configStateKeys, state)
}

// RolloutResult is where one device of a rollout stands: its result word is
// drawn beside a mark in the result's colour, never as a bare word.
type RolloutResult uint8

const (
	// RolloutResultUnspecified is the invalid zero value.
	RolloutResultUnspecified RolloutResult = iota
	// RolloutResultWaiting means the device has not been sent the version yet.
	RolloutResultWaiting
	// RolloutResultApplied means the device applied the version.
	RolloutResultApplied
	// RolloutResultRejected means the device refused the version.
	RolloutResultRejected
)

var rolloutResultWords = []string{enumWordUnspecified, "Waiting", "Applied", "Rejected"}

// String answers the result's word.
func (result RolloutResult) String() string {
	return enumText(rolloutResultWords, result)
}

// ConfigVersionResult is what a device answered when the hub sent it one
// version of its configuration. The zero value is not a result.
type ConfigVersionResult uint8

const (
	// ConfigVersionResultUnspecified is the invalid zero value.
	ConfigVersionResultUnspecified ConfigVersionResult = iota
	// ConfigVersionResultApplied means the device applied the version.
	ConfigVersionResultApplied
	// ConfigVersionResultRejected means the device refused the version.
	ConfigVersionResultRejected
	// ConfigVersionResultNotFetched means the device has not fetched the
	// version yet.
	ConfigVersionResultNotFetched
)

var configVersionResultWords = []string{enumWordUnspecified, "Applied", "Rejected", "Not fetched yet"}

// String answers the result's word.
func (result ConfigVersionResult) String() string {
	return enumText(configVersionResultWords, result)
}

// AuditResult is how the event an audit entry records turned out. The zero
// value is not a result.
type AuditResult uint8

const (
	// AuditResultUnspecified is the invalid zero value.
	AuditResultUnspecified AuditResult = iota
	// AuditResultRecorded means the change took effect when it was recorded.
	AuditResultRecorded
	// AuditResultStarted means the event started work that goes on.
	AuditResultStarted
	// AuditResultPending means the work waits to start.
	AuditResultPending
	// AuditResultRunning means the work is under way.
	AuditResultRunning
	// AuditResultPaused means the work waits for an operator.
	AuditResultPaused
	// AuditResultSucceeded means the work finished on every device.
	AuditResultSucceeded
	// AuditResultFailed means the work failed.
	AuditResultFailed
	// AuditResultCanceled means an operator stopped the work.
	AuditResultCanceled
	// AuditResultPartlyFailed means the work failed on some devices.
	AuditResultPartlyFailed
)

var (
	auditResultWords = []string{
		enumWordUnspecified, "Recorded", "Started", enumWordPending, "Running", "Paused",
		"Succeeded", "Failed", "Canceled", "Partly failed",
	}
	auditResultKeys = []string{
		enumKeyUnspecified, "recorded", "started", enumKeyPending, "running", "paused",
		"succeeded", enumKeyFailed, "canceled", "partly-failed",
	}
)

// String answers the result's word.
func (result AuditResult) String() string {
	return enumText(auditResultWords, result)
}

// Key answers the result's name in a filter term and an export.
func (result AuditResult) Key() string {
	return enumText(auditResultKeys, result)
}

// ParseConfigState answers the state a filter term names, or false.
func ParseConfigState(key string) (ConfigState, bool) {
	return enumParse[ConfigState](configStateKeys, key)
}

// ConfigStates answers every configuration state in the order a legend lists
// them.
func ConfigStates() []ConfigState {
	return []ConfigState{
		ConfigStateInSync, ConfigStatePending, ConfigStateRejected,
		ConfigStateDiverged, ConfigStateNeverFetched,
	}
}

// AlarmSeverity is how much an alarm matters. The zero value is not a
// severity.
type AlarmSeverity uint8

// The severities, the most severe first.
const (
	// AlarmSeverityUnspecified is the invalid zero value.
	AlarmSeverityUnspecified AlarmSeverity = iota
	// AlarmSeverityCritical means something does not work.
	AlarmSeverityCritical
	// AlarmSeverityWarning means something works with a fault.
	AlarmSeverityWarning
	// AlarmSeverityInformational means something an operator may want to
	// know.
	AlarmSeverityInformational
)

var (
	alarmSeverityWords = []string{enumWordUnspecified, "Critical", "Warning", "Informational"}
	alarmSeverityKeys  = []string{enumKeyUnspecified, "critical", "warning", "informational"}
)

// String answers the word a page writes beside the severity's icon.
func (severity AlarmSeverity) String() string {
	return enumText(alarmSeverityWords, severity)
}

// Key answers the severity's name in a filter term.
func (severity AlarmSeverity) Key() string {
	return enumText(alarmSeverityKeys, severity)
}

// ParseAlarmSeverity answers the severity a filter term names, or false.
func ParseAlarmSeverity(key string) (AlarmSeverity, bool) {
	return enumParse[AlarmSeverity](alarmSeverityKeys, key)
}

// AlarmSeverities answers every severity, the most severe first.
func AlarmSeverities() []AlarmSeverity {
	return []AlarmSeverity{AlarmSeverityCritical, AlarmSeverityWarning, AlarmSeverityInformational}
}

// TaskState is where a task, or one device of a task, stands. The zero value
// is not a state.
type TaskState uint8

const (
	// TaskStateUnspecified is the invalid zero value.
	TaskStateUnspecified TaskState = iota
	// TaskStatePending means the task has not started.
	TaskStatePending
	// TaskStateRunning means the task is at work.
	TaskStateRunning
	// TaskStateSucceeded means every device succeeded.
	TaskStateSucceeded
	// TaskStateFailed means the task failed.
	TaskStateFailed
	// TaskStateCanceled means an operator stopped the task.
	TaskStateCanceled
	// TaskStatePartlyFailed means some devices succeeded and some failed.
	TaskStatePartlyFailed
)

var (
	taskStateWords = []string{enumWordUnspecified, enumWordPending, "Running", "Succeeded", "Failed", "Canceled", "Partly failed"}
	taskStateKeys  = []string{enumKeyUnspecified, enumKeyPending, "running", "succeeded", enumKeyFailed, "canceled", "partly-failed"}
	// taskStateActive marks the states of a task that is not over: pending
	// and running.
	taskStateActive = []bool{false, true, true, false, false, false, false}
)

// String answers the word a page shows for the task state.
func (state TaskState) String() string {
	return enumText(taskStateWords, state)
}

// Key answers the state's name in a filter term.
func (state TaskState) Key() string {
	return enumText(taskStateKeys, state)
}

// Active reports whether a task in this state is still pending or running.
// It is what the header count, the tree spinners and the has:task filter
// term count.
func (state TaskState) Active() bool {
	if int(state) < len(taskStateActive) {
		return taskStateActive[state]
	}

	return false
}

// ParseTaskState answers the state a filter term names, or false.
func ParseTaskState(key string) (TaskState, bool) {
	return enumParse[TaskState](taskStateKeys, key)
}
