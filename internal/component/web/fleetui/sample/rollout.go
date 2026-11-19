// Design: docs/architecture/fleet-interface.md -- the rollouts and the audit trail of the sample estate
// Overview: source.go -- the sample data source
// Related: config.go -- the revision a rollout sends and the device that refused it
// Related: activity.go -- the tasks a rollout's batches ran as
//
// Two rollouts: one that sends the newest revision to the edge routers and
// paused at its second batch when a device refused it, and one that finished
// three days ago. A rollout is a row holding its batches, and what each
// device answered follows from the row.
//
// The audit trail is every task's entry, derived from the task, and the rows
// of auditRows for what no task did.

package sample

import (
	"slices"
	"sort"
	"time"

	"github.com/ze-software/ze/internal/component/web/fleetui"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The names and words of the rollouts.
const (
	rolloutEdge = "edge-refresh"
	rolloutCore = "core-timers"

	// taskRejectedBatch is the task that ran the batch a device refused.
	taskRejectedBatch = "T-2046"

	wordApplied  = "Applied"
	wordRejected = "Rejected"
	wordWaiting  = "Waiting"

	// answerInterval is the time between the answers of two devices of one
	// batch.
	answerInterval = 20 * time.Second
)

// rolloutRow is one rollout.
type rolloutRow struct {
	name   string
	target string
	actor  string
	state  fleetui.RolloutState
	// started is how long ago the rollout started, and pace the time from
	// one batch to the next.
	started time.Duration
	pace    time.Duration
	batches [][]string
	// batchesDone counts the batches that finished: the first of batches.
	batchesDone int
	// pauseReason and refused mean nothing unless state is paused. refused
	// names the devices of the batch after the last finished one that
	// refused the configuration.
	pauseReason string
	refused     []string
}

// rolloutRows holds the rollouts, the newest first.
var rolloutRows = []rolloutRow{
	{
		name: rolloutEdge, target: "role:edge", actor: actorThomas,
		state: fleetui.RolloutStatePaused, started: rolloutEdgeAge, pace: 22 * time.Minute,
		batches: [][]string{
			{deviceEdgeLon01, deviceEdgeLon02},
			{deviceEdgeFra01, deviceEdgeFra02},
			{deviceEdgeLon03, deviceEdgeLon04, deviceEdgeAms01},
			{deviceEdgeAms02, deviceEdgeAms03, deviceEdgeFra03},
			{deviceEdgePar01, deviceEdgePar02},
		},
		batchesDone: 1,
		pauseReason: "edge-fra-01 refused the configuration, which reached the failure threshold of one device. " +
			"Batch 2 stopped before edge-fra-02 was sent it.",
		refused: []string{deviceEdgeFra01},
	},
	{
		name: rolloutCore, target: "role:core", actor: actorAmelie,
		state: fleetui.RolloutStateComplete, started: 3 * day, pace: 30 * time.Minute,
		batches: [][]string{
			{deviceCoreLon01, deviceCoreLon02},
			{deviceCoreAms01, deviceCoreAms02},
			{deviceCoreFra01, deviceCoreFra02},
			{deviceCorePar01, deviceCorePar02},
		},
		batchesDone: 4,
	},
}

// rolloutSummary answers a rollout without its batches and devices.
func rolloutSummary(row *rolloutRow, snap *snapshot) fleetui.Rollout {
	rollout := fleetui.Rollout{
		Name:         row.name,
		Target:       row.target,
		State:        row.state,
		Actor:        row.actor,
		Started:      snap.ago(row.started),
		BatchesTotal: len(row.batches),
		BatchesDone:  row.batchesDone,
	}
	if row.state == fleetui.RolloutStatePaused {
		rollout.PauseReason = row.pauseReason
	}

	for at := range row.batches {
		rollout.DevicesTotal += len(row.batches[at])
		if at < row.batchesDone {
			rollout.DevicesDone += len(row.batches[at])
		}
	}

	return rollout
}

// rolloutsAt answers the rollouts at the snapshot, the newest first.
func rolloutsAt(snap *snapshot) []fleetui.Rollout {
	rollouts := make([]fleetui.Rollout, 0, len(rolloutRows))
	for at := range rolloutRows {
		rollouts = append(rollouts, rolloutSummary(&rolloutRows[at], snap))
	}

	return rollouts
}

// batchState answers the state of the batch at position at of a rollout.
func batchState(row *rolloutRow, at int) fleetui.RolloutState {
	if at < row.batchesDone {
		return fleetui.RolloutStateComplete
	}
	if at == row.batchesDone {
		if row.state == fleetui.RolloutStatePaused {
			return fleetui.RolloutStatePaused
		}
	}

	return fleetui.RolloutStatePending
}

// refusedBy reports whether the device refused the rollout's configuration.
func refusedBy(row *rolloutRow, device string) bool {
	return slices.Contains(row.refused, device)
}

// rolloutDevice answers what one device of a rollout answered. batch is the
// position of its batch and place its position in the batch.
func rolloutDevice(row *rolloutRow, snap *snapshot, batch, place int) fleetui.RolloutDevice {
	device := fleetui.RolloutDevice{Device: row.batches[batch][place], Batch: batch + 1, Result: wordWaiting}

	if batch < row.batchesDone {
		sent := row.started - time.Duration(batch)*row.pace - time.Duration(place+1)*answerInterval

		device.Result = wordApplied
		device.Answer = answerApplied
		device.Answered = true
		device.At = snap.ago(sent)

		return device
	}
	if batch > row.batchesDone {
		return device
	}
	if !refusedBy(row, device.Device) {
		return device
	}

	device.Result = wordRejected
	device.Answer = rejectionAnswer
	device.Answered = true
	device.At = snap.ago(rejectedFor)

	return device
}

// rolloutAt answers one rollout with its batches and devices, or refuses a
// name the hub does not keep.
func rolloutAt(snap *snapshot, name string) (fleetui.RolloutDetail, error) {
	for at := range rolloutRows {
		row := &rolloutRows[at]
		if row.name != name {
			continue
		}

		detail := fleetui.RolloutDetail{
			Rollout:      rolloutSummary(row, snap),
			Batches:      make([]fleetui.RolloutBatch, 0, len(row.batches)),
			Devices:      make([]fleetui.RolloutDevice, 0, len(deviceRows)),
			PauseDevices: slices.Clone(row.refused),
		}
		for batch := range row.batches {
			detail.Batches = append(detail.Batches,
				fleetui.RolloutBatch{Number: batch + 1, State: batchState(row, batch)})
			for place := range row.batches[batch] {
				detail.Devices = append(detail.Devices, rolloutDevice(row, snap, batch, place))
			}
		}

		return detail, nil
	}

	problem := textbuf.New().Str("The hub keeps no rollout named ").Quoted(name).Str(".").Slice()

	return fleetui.RolloutDetail{}, opererr.Wrap(fleetui.ErrObjectUnknown, problem,
		"The address names a rollout this hub never ran, so there is nothing to show.",
		"open /fleet/estate/rollouts/")
}

// auditPrefix starts the identifier of an audit entry, and actorHub is the
// actor of what the hub did on its own.
const (
	auditPrefix = "AU-"
	actorHub    = "hub"
)

// auditRow is one audit entry no task wrote.
type auditRow struct {
	id string
	// age is how long ago the event happened.
	age   time.Duration
	actor string
	event string
	// device is the device the entry is about. It is empty for an entry
	// about several, which target then describes.
	device  string
	target  string
	devices int
	result  string
	summary string
}

// The words of the audit trail more than one row carries.
const (
	eventRolloutStarted = "rollout-started"
	resultStarted       = "started"
	resultRecorded      = "recorded"
)

// auditRows holds the audit entries of the last week that no task wrote.
var auditRows = []auditRow{
	{
		id: "AU-3016", age: 2*time.Hour + 45*time.Minute, actor: actorAmelie, event: "config-edited",
		device: deviceCpePar07, devices: 1, result: "pending",
		summary: "Edited the configuration. The device is offline and has not fetched it.",
	},
	{
		id: "AU-3015", age: rejectedFor - 10*time.Second, actor: actorHub, event: "rollout-paused",
		target: rolloutEdge, devices: 12, result: "paused",
		summary: "Paused at batch 2: edge-fra-01 refused the configuration.",
	},
	{
		id: "AU-3013", age: rolloutEdgeAge, actor: actorThomas, event: eventRolloutStarted,
		target: rolloutEdge, devices: 12, result: resultStarted,
		summary: "Started over role:edge, 5 batches, failure threshold of one device.",
	},
	{
		id: "AU-3012", age: rolloutEdgeAge + 3*time.Minute, actor: actorThomas, event: "template-changed",
		target: templateEdge, devices: 12, result: resultRecorded,
		summary: "Raised the IPv4 prefix maximum to 1200000.",
	},
	{
		id: "AU-3009", age: divergedFor, actor: actorHub, event: "divergence-detected",
		device: deviceRsFra01, devices: 1, result: resultRecorded,
		summary: "The device reported 3 local changes. Nothing was overwritten.",
	},
	{
		id: "AU-3004", age: 26 * time.Hour, actor: actorSofia, event: "device-declared",
		device: newcomerName, devices: 1, result: resultRecorded,
		summary: "Declared the device and set its secret.",
	},
	{
		id: "AU-2996", age: 3*day - 2*time.Hour, actor: actorHub, event: "rollout-completed",
		target: rolloutCore, devices: 8, result: "complete",
		summary: "Every batch finished: 8 of 8 devices applied the configuration.",
	},
	{
		id: "AU-2990", age: 3 * day, actor: actorAmelie, event: eventRolloutStarted,
		target: rolloutCore, devices: 8, result: resultStarted,
		summary: "Started over role:core, 4 batches.",
	},
	{
		id: "AU-2981", age: 6*day + 12*time.Hour, actor: actorThomas, event: "label-changed",
		device: deviceCpe0127, devices: 1, result: resultRecorded,
		summary: "Set customer=acme-logistics.",
	},
}

// taskEntry answers the audit entry a task wrote: when it ended, or when it
// started while it still runs.
func taskEntry(task *fleetui.Task) fleetui.AuditEntry {
	entry := fleetui.AuditEntry{
		ID:      auditIDOf(task.ID),
		At:      task.Started,
		Actor:   task.Actor,
		Event:   task.Kind,
		Target:  task.Target,
		Devices: len(task.Devices),
		Result:  task.State.String(),
		TaskID:  task.ID,
	}
	if !task.State.Active() {
		entry.At = task.Ended
	}

	entry.Summary = textbuf.New().Int(int64(task.Finished)).Str(" of ").Int(int64(len(task.Devices))).
		Str(" devices answered, ").Int(int64(task.Failed)).Str(" failed.").Slice()

	return entry
}

// auditAt answers the audit trail at the snapshot, the newest first. The
// trail outlives the task list: it holds an entry for every task of the
// table and of the script, kept or dropped.
func auditAt(snap *snapshot) []fleetui.AuditEntry {
	entries := make([]fleetui.AuditEntry, 0, len(auditRows)+len(taskRows)+len(snap.script.tasks))

	for at := range snap.script.tasks {
		detail := scriptedTask(&snap.script.tasks[at])
		entries = append(entries, taskEntry(&detail.Task))
	}
	for at := range taskRows {
		detail := tableTask(&taskRows[at], snap)
		entries = append(entries, taskEntry(&detail.Task))
	}

	for at := range auditRows {
		row := &auditRows[at]

		target := row.target
		if row.device != "" {
			target = row.device
		}

		entries = append(entries, fleetui.AuditEntry{
			ID: row.id, At: snap.ago(row.age), Actor: row.actor, Event: row.event,
			Target: target, Devices: row.devices, Result: row.result, Summary: row.summary,
		})
	}

	// The comparison takes indexes: an audit entry is too large to pass by value.
	sort.SliceStable(entries, func(a, b int) bool { return entries[a].At.After(entries[b].At) })

	return entries
}
