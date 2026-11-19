// Design: docs/architecture/fleet-interface.md -- the alarms and tasks of the sample estate
// Overview: source.go -- the sample data source
// Related: estate.go -- the faults the component alarms report
// Related: script.go -- the alarm and the tasks the script adds
// Related: rollout.go -- the audit entries each task wrote
//
// The alarms are one table: a row that is open is an open alarm, and every
// row of a device is its alarm history. The tasks are one table too, and the
// per-device results of a task follow from its row. The script's alarm and
// tasks are added in front of both.

package sample

import (
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/web/fleetui"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The alarm kinds the sample hub raises.
const (
	alarmComponentDown     = "component-down"
	alarmComponentDegraded = "component-degraded"
	alarmConfigDiverged    = "config-diverged"
	alarmConfigRejected    = "config-rejected"
	alarmResourceHigh      = "resource-high"
	alarmSoftwareBehind    = "software-behind"
	alarmRelayUngranted    = "relay-not-granted"
)

// The alarm sentences more than one row carries.
const (
	summaryBGPDown         = "bgp is down"
	summaryIfaceDegraded   = "iface is degraded"
	summaryFIBDegraded     = "fib is degraded"
	summaryPluginsDegraded = "plugins is degraded"
	summaryRelayUngranted  = "No relay profile is granted"
	detailRelayUngranted   = "The device grants no profile to relayed commands, so its tabs cannot show live state."

	scriptAlarmSummary = "Memory above 95%"
	scriptAlarmDetail  = "Memory use has stayed above the alarm threshold of 95% for five minutes."
)

// alarmRow is one alarm of the last week. An open row is an open alarm.
type alarmRow struct {
	id       string
	kind     string
	severity fleetui.AlarmSeverity
	device   string
	summary  string
	// detail is the alarm's detail. An open component alarm leaves it empty
	// and takes the detail, and its age, from the fault of its device.
	detail string
	// opened is how long ago the alarm opened. cleared is how long ago it
	// cleared, and means nothing while open is true.
	opened  time.Duration
	cleared time.Duration
	open    bool
	// ofFault reports that the alarm reports the fault of its device.
	ofFault bool
}

// alarmRows holds the nine open alarms, one critical, five warnings and three
// informational, and the alarms that cleared during the week on the devices
// that have one open.
//
// No device-offline alarm is open although three devices are offline: the
// sample hub opens one after seven days without a connection (settings), and
// the device that has been away longest left six days ago.
var alarmRows = []alarmRow{
	{
		id: "A-1052", kind: alarmComponentDown, severity: fleetui.AlarmSeverityCritical,
		device: deviceEdgeAms02, summary: summaryBGPDown, open: true, ofFault: true,
	},
	{
		id: "A-1055", kind: alarmComponentDegraded, severity: fleetui.AlarmSeverityWarning,
		device: deviceEdgeLon04, summary: summaryIfaceDegraded, open: true, ofFault: true,
	},
	{
		id: "A-1050", kind: alarmComponentDegraded, severity: fleetui.AlarmSeverityWarning,
		device: deviceCoreFra02, summary: summaryFIBDegraded, open: true, ofFault: true,
	},
	{
		id: "A-1054", kind: alarmComponentDegraded, severity: fleetui.AlarmSeverityWarning,
		device: deviceCpeAms03, summary: summaryPluginsDegraded, open: true, ofFault: true,
	},
	{
		id: "A-1041", kind: alarmConfigDiverged, severity: fleetui.AlarmSeverityWarning,
		device: deviceRsFra01, summary: "The configuration was edited on the device",
		detail: "The device holds 3 local changes. The hub has overwritten nothing.",
		opened: divergedFor, open: true,
	},
	{
		id: "A-1049", kind: alarmConfigRejected, severity: fleetui.AlarmSeverityWarning,
		device: deviceEdgeFra01, summary: "The device refused the configuration the hub sent",
		detail: rejectionAnswer, opened: rejectedFor, open: true,
	},
	{
		id: "A-1012", kind: alarmSoftwareBehind, severity: fleetui.AlarmSeverityInformational,
		device: deviceCpe0114, summary: "Runs 1.3.9, two versions behind 1.4.2",
		detail: "The device runs a version more than one behind the newest in the estate.",
		opened: 6*day + 20*time.Hour, open: true,
	},
	{
		id: "A-1038", kind: alarmRelayUngranted, severity: fleetui.AlarmSeverityInformational,
		device: deviceCpeFra03, summary: summaryRelayUngranted, detail: detailRelayUngranted,
		opened: 2 * day, open: true,
	},
	{
		id: "A-1058", kind: alarmRelayUngranted, severity: fleetui.AlarmSeverityInformational,
		device: newcomerName, summary: summaryRelayUngranted, detail: detailRelayUngranted,
		opened: newcomerConnectedFor, open: true,
	},

	{
		id: "A-1031", kind: alarmComponentDegraded, severity: fleetui.AlarmSeverityWarning,
		device: deviceEdgeAms02, summary: summaryPluginsDegraded,
		opened: 3*day + 2*time.Hour, cleared: 3*day + time.Hour,
	},
	{
		id: "A-1019", kind: alarmComponentDegraded, severity: fleetui.AlarmSeverityWarning,
		device: deviceEdgeLon04, summary: summaryIfaceDegraded,
		opened: 5 * day, cleared: 5*day - 30*time.Minute,
	},
	{
		id: "A-1036", kind: alarmComponentDegraded, severity: fleetui.AlarmSeverityWarning,
		device: deviceEdgeLon04, summary: summaryIfaceDegraded,
		opened: 2*day + 6*time.Hour, cleared: 2*day + 5*time.Hour + 50*time.Minute,
	},
	{
		id: "A-1044", kind: alarmComponentDegraded, severity: fleetui.AlarmSeverityWarning,
		device: deviceCoreFra02, summary: summaryFIBDegraded,
		opened: 26 * time.Hour, cleared: 25 * time.Hour,
	},
	{
		id: "A-1043", kind: alarmComponentDegraded, severity: fleetui.AlarmSeverityWarning,
		device: deviceCpeAms03, summary: summaryPluginsDegraded,
		opened: 30 * time.Hour, cleared: 29 * time.Hour,
	},
	{
		id: "A-1021", kind: alarmConfigRejected, severity: fleetui.AlarmSeverityWarning,
		device: deviceRsFra01, summary: "The device refused the configuration the hub sent",
		opened: 5 * day, cleared: 4*day + 22*time.Hour,
	},
	{
		id: "A-1014", kind: alarmComponentDown, severity: fleetui.AlarmSeverityCritical,
		device: deviceCpeFra03, summary: summaryBGPDown,
		opened: 6 * day, cleared: 6*day - time.Hour,
	},
}

// alarmsAt answers the open alarms at the snapshot: the open rows of the
// table, and the script's alarm while it is open.
func alarmsAt(snap *snapshot) []fleetui.Alarm {
	alarms := make([]fleetui.Alarm, 0, len(alarmRows)+1)

	if snap.script.alarmOpen {
		alarms = append(alarms, fleetui.Alarm{
			ID:       snap.script.alarmID,
			Kind:     alarmResourceHigh,
			Severity: fleetui.AlarmSeverityWarning,
			Device:   scriptAlarmDevice,
			Summary:  scriptAlarmSummary,
			Detail:   scriptAlarmDetail,
			Since:    snap.script.alarmSince,
		})
	}

	for at := range alarmRows {
		row := &alarmRows[at]
		if !row.open {
			continue
		}

		alarm := fleetui.Alarm{
			ID: row.id, Kind: row.kind, Severity: row.severity, Device: row.device,
			Summary: row.summary, Detail: row.detail, Since: snap.ago(row.opened),
		}
		if row.ofFault {
			if fault, faulty := faultOf(row.device); faulty {
				alarm.Detail = fault.detail
				alarm.Since = snap.ago(fault.since)
			}
		}
		alarms = append(alarms, alarm)
	}

	return alarms
}

// alarmHistoryAt answers the alarms of one device over the week, the newest
// first.
func alarmHistoryAt(snap *snapshot, device string) []fleetui.AlarmEvent {
	events := make([]fleetui.AlarmEvent, 0, len(alarmRows))

	if device == scriptAlarmDevice {
		events = append(events, snap.script.alarmsPast...)
	}

	open := alarmsAt(snap)
	for at := range open {
		if open[at].Device == device {
			events = append(events, fleetui.AlarmEvent{
				Kind: open[at].Kind, Severity: open[at].Severity, Summary: open[at].Summary,
				Opened: open[at].Since,
			})
		}
	}

	for at := range alarmRows {
		row := &alarmRows[at]
		if row.open {
			continue
		}
		if row.device != device {
			continue
		}

		events = append(events, fleetui.AlarmEvent{
			Kind: row.kind, Severity: row.severity, Summary: row.summary,
			Opened: snap.ago(row.opened), Cleared: true, ClearedAt: snap.ago(row.cleared),
		})
	}

	// The comparison takes indexes: an alarm event is too large to pass by value.
	sort.SliceStable(events, func(a, b int) bool { return events[a].Opened.After(events[b].Opened) })

	return events
}

// The task kinds of the sample hub.
const (
	kindConfigPush     = "config-push"
	kindRolloutBatch   = "rollout-batch"
	kindDiagnostic     = "diagnostic-run"
	kindCommandBatch   = "command-batch"
	kindSoftwareUpdate = "software-update"
)

// The users of the sample hub. They are sample names.
const (
	actorThomas = "thomas"
	actorAmelie = "amelie"
	actorSofia  = "sofia"
)

// What a device that succeeded answered, per kind of task.
const (
	answerDiagnostic = "all 14 checks passed"
	answerApplied    = "configuration applied"
	answerCommand    = "command answered, 42 lines"
	answerUpdated    = "version 1.4.2 installed, daemon restarted"
)

// finishedTasksKept is how many ended tasks the sample hub keeps. The oldest
// beyond it is dropped, so the script's ended tasks push out the table's.
const finishedTasksKept = 13

// taskFailure is the answer of one device that failed a task.
type taskFailure struct {
	device string
	answer string
}

// taskRow is one task of the last day.
type taskRow struct {
	id      string
	kind    string
	actor   string
	target  string
	devices []string
	state   fleetui.TaskState
	// started is how long ago the task started, and lasted how long it ran.
	// lasted means nothing while the task is running.
	started time.Duration
	lasted  time.Duration
	// answered is how many devices have a result: the first of devices.
	answered int
	// answer is what a device that succeeded answered.
	answer   string
	failures []taskFailure
}

// taskRows holds the tasks of the table, the newest first: one running, one
// failed and twelve that finished in the last day, one of them partly failed.
// The script's running task is the second running one.
var taskRows = []taskRow{
	{
		id: "T-2050", kind: kindSoftwareUpdate, actor: actorAmelie, target: "device cpe-0114",
		devices: []string{deviceCpe0114}, state: fleetui.TaskStateRunning,
		started: 6 * time.Minute, answer: answerUpdated,
	},
	{
		id: "T-2049", kind: kindDiagnostic, actor: actorThomas, target: "group site=lon, role=core",
		devices: []string{deviceCoreLon01, deviceCoreLon02}, state: fleetui.TaskStateSucceeded,
		started: 35 * time.Minute, lasted: 20 * time.Second, answered: 2, answer: answerDiagnostic,
	},
	{
		id: "T-2048", kind: kindCommandBatch, actor: actorAmelie, target: "selection of 3 devices",
		devices: []string{deviceRsLon01, deviceRsAms01, deviceRsPar01}, state: fleetui.TaskStateSucceeded,
		started: 70 * time.Minute, lasted: 8 * time.Second, answered: 3, answer: answerCommand,
	},
	{
		id: "T-2047", kind: kindConfigPush, actor: actorThomas, target: "device core-ams-01",
		devices: []string{deviceCoreAms01}, state: fleetui.TaskStateSucceeded,
		started: 2 * time.Hour, lasted: 14 * time.Second, answered: 1, answer: answerApplied,
	},
	{
		id: taskRejectedBatch, kind: kindRolloutBatch, actor: actorThomas, target: "rollout edge-refresh, batch 2",
		devices: []string{deviceEdgeFra01, deviceEdgeFra02}, state: fleetui.TaskStateFailed,
		started: rejectedFor + 20*time.Second, lasted: 50 * time.Second, answered: 1, answer: answerApplied,
		failures: []taskFailure{{deviceEdgeFra01, rejectionAnswer}},
	},
	{
		id: "T-2045", kind: kindRolloutBatch, actor: actorThomas, target: "rollout edge-refresh, batch 1",
		devices: []string{deviceEdgeLon01, deviceEdgeLon02}, state: fleetui.TaskStateSucceeded,
		started: rolloutEdgeAge - 5*time.Second, lasted: 70 * time.Second, answered: 2, answer: answerApplied,
	},
	{
		id: "T-2044", kind: kindCommandBatch, actor: actorSofia, target: "selection of 5 devices",
		devices: []string{deviceCpeLon01, deviceCpeLon02, deviceCpeAms01, deviceCpeFra03, deviceCpePar03},
		state:   fleetui.TaskStatePartlyFailed,
		started: 6 * time.Hour, lasted: 12 * time.Second, answered: 5, answer: answerCommand,
		failures: []taskFailure{{deviceCpeFra03, relayRefusal}},
	},
	{
		id: "T-2043", kind: kindDiagnostic, actor: actorSofia, target: "device edge-ams-02",
		devices: []string{deviceEdgeAms02}, state: fleetui.TaskStateSucceeded,
		started: 7 * time.Hour, lasted: 9 * time.Second, answered: 1, answer: answerDiagnostic,
	},
	{
		id: "T-2042", kind: kindConfigPush, actor: actorAmelie, target: "device core-ams-02",
		devices: []string{deviceCoreAms02}, state: fleetui.TaskStateSucceeded,
		started: 9 * time.Hour, lasted: 13 * time.Second, answered: 1, answer: answerApplied,
	},
	{
		id: "T-2041", kind: kindSoftwareUpdate, actor: actorAmelie, target: "selection of 2 devices",
		devices: []string{deviceCpeLon04, deviceCpeLon05}, state: fleetui.TaskStateSucceeded,
		started: 11 * time.Hour, lasted: 6 * time.Minute, answered: 2, answer: answerUpdated,
	},
	{
		id: "T-2040", kind: kindDiagnostic, actor: actorThomas, target: "group site=par",
		devices: []string{deviceEdgePar01, deviceEdgePar02, deviceCorePar01}, state: fleetui.TaskStateSucceeded,
		started: 13 * time.Hour, lasted: 25 * time.Second, answered: 3, answer: answerDiagnostic,
	},
	{
		id: "T-2039", kind: kindConfigPush, actor: actorSofia, target: "device core-fra-01",
		devices: []string{deviceCoreFra01}, state: fleetui.TaskStateSucceeded,
		started: 15 * time.Hour, lasted: 12 * time.Second, answered: 1, answer: answerApplied,
	},
	{
		id: "T-2038", kind: kindCommandBatch, actor: actorThomas, target: "group role=route-server",
		devices: []string{deviceRsLon01, deviceRsAms01, deviceRsFra01, deviceRsPar01},
		state:   fleetui.TaskStateSucceeded,
		started: 17 * time.Hour, lasted: 10 * time.Second, answered: 4, answer: answerCommand,
	},
	{
		id: "T-2037", kind: kindConfigPush, actor: actorAmelie, target: "device cpe-ams-04",
		devices: []string{deviceCpeAms04}, state: fleetui.TaskStateSucceeded,
		started: 20 * time.Hour, lasted: 11 * time.Second, answered: 1, answer: answerApplied,
	},
}

// The time one device of a task takes to answer: the first, and each next
// one longer by a step.
const (
	resultBase = 900 * time.Millisecond
	resultStep = 350 * time.Millisecond
)

// failureOf answers what the device answered when it failed the task, or
// false when it did not fail.
func failureOf(row *taskRow, device string) (string, bool) {
	for _, failure := range row.failures {
		if failure.device == device {
			return failure.answer, true
		}
	}

	return "", false
}

// auditIDOf answers the identifier of the audit entry a task wrote: the
// task's number under the audit prefix.
func auditIDOf(taskID string) string {
	return auditPrefix + strings.TrimPrefix(taskID, taskPrefix)
}

// tableTask answers one task of the table with its per-device results.
func tableTask(row *taskRow, snap *snapshot) fleetui.TaskDetail {
	detail := fleetui.TaskDetail{
		Task: fleetui.Task{
			ID: row.id, Kind: row.kind, Actor: row.actor, Target: row.target,
			Devices: slices.Clone(row.devices), State: row.state,
			Finished: row.answered, Started: snap.ago(row.started),
		},
		Results:  make([]fleetui.TaskResult, 0, row.answered),
		AuditIDs: []string{auditIDOf(row.id)},
	}
	if !row.state.Active() {
		detail.Task.Ended = snap.ago(row.started - row.lasted)
	}

	for at, device := range row.devices[:row.answered] {
		result := fleetui.TaskResult{
			Device:   device,
			State:    fleetui.TaskStateSucceeded,
			Answer:   row.answer,
			Duration: resultBase + time.Duration(at)*resultStep,
		}
		if answer, failed := failureOf(row, device); failed {
			result.State = fleetui.TaskStateFailed
			result.Answer = answer
			detail.Task.Failed++
		}
		detail.Results = append(detail.Results, result)
	}

	return detail
}

// scriptedTask answers one task the script created with its results.
func scriptedTask(task *scriptTask) fleetui.TaskDetail {
	detail := fleetui.TaskDetail{
		Task: fleetui.Task{
			ID: task.id, Kind: kindDiagnostic, Actor: actorThomas, Target: "selection of 4 devices",
			Devices: scriptTaskDevices(), State: task.state,
			Finished: len(task.results), Started: task.started, Ended: task.ended,
		},
		Results:  slices.Clone(task.results),
		AuditIDs: []string{auditIDOf(task.id)},
	}
	for at := range task.results {
		if task.results[at].State == fleetui.TaskStateFailed {
			detail.Task.Failed++
		}
	}

	return detail
}

// taskDetailsAt answers every task the sample hub keeps at the snapshot, the
// newest first: the script's tasks, then the table's. It keeps every task
// that is still active and the newest finishedTasksKept that ended.
func taskDetailsAt(snap *snapshot) []fleetui.TaskDetail {
	details := make([]fleetui.TaskDetail, 0, len(snap.script.tasks)+len(taskRows))

	ended := 0
	keep := func(detail *fleetui.TaskDetail) {
		if !detail.Task.State.Active() {
			if ended == finishedTasksKept {
				return
			}
			ended++
		}
		details = append(details, *detail)
	}

	for at := len(snap.script.tasks) - 1; at >= 0; at-- {
		detail := scriptedTask(&snap.script.tasks[at])
		keep(&detail)
	}
	for at := range taskRows {
		detail := tableTask(&taskRows[at], snap)
		keep(&detail)
	}

	return details
}

// tasksAt answers the tasks at the snapshot without their results.
func tasksAt(snap *snapshot) []fleetui.Task {
	details := taskDetailsAt(snap)

	tasks := make([]fleetui.Task, 0, len(details))
	for at := range details {
		tasks = append(tasks, details[at].Task)
	}

	return tasks
}

// taskAt answers one task with its results, or refuses an identifier the hub
// does not keep.
func taskAt(snap *snapshot, id string) (fleetui.TaskDetail, error) {
	details := taskDetailsAt(snap)
	for at := range details {
		if details[at].Task.ID == id {
			return details[at], nil
		}
	}

	problem := textbuf.New().Str("The hub keeps no task ").Quoted(id).Str(".").Slice()

	return fleetui.TaskDetail{}, opererr.Wrap(fleetui.ErrObjectUnknown, problem,
		"The identifier names no task, or names one that ended long enough ago to have been dropped.",
		"open /fleet/tasks/")
}
