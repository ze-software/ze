// Design: docs/architecture/fleet-interface.md -- the scripted events of the sample estate
// Overview: source.go -- the sample data source
// Related: estate.go -- the device the script takes offline
// Related: activity.go -- the alarms and tasks the script adds to
//
// The script is what makes the sample estate move: a device goes offline and
// comes back, an alarm opens and clears, a task progresses across its devices
// and ends with one failure, and the next task is created. It is a table of
// steps, applied one at a time, in a loop.
//
// A step changes the script's state and publishes the matching event while
// the source's lock is held, so the order of the events is the order of the
// changes, and every other answer of the source is computed from a copy of
// that state. A device an event showed offline is offline in Devices.
//
// The script runs in two modes. Timed: the source's one goroutine applies a
// step every scriptPeriod. Stepped: Step applies one step, and from the first
// Step or Hold the timer applies none.

package sample

import (
	"slices"
	"time"

	"github.com/ze-software/ze/internal/component/web/fleetui"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// What the script acts on, and its bounds. Each list the script grows is cut
// to its bound, oldest first, so the state stays the same size however long
// the build runs.
const (
	// scriptPeriod is the time between two steps in timed mode.
	scriptPeriod = 5 * time.Second
	// scriptDevice is the device the script takes offline and brings back.
	scriptDevice = deviceCpeLon03
	// scriptAlarmDevice is the device whose memory alarm the script opens and
	// clears, and scriptAlarmMemory the memory use it reports meanwhile.
	scriptAlarmDevice = deviceCoreLon01
	scriptAlarmMemory = 96.4
	// scriptTasksMax bounds the scripted tasks kept, scriptOutagesMax the
	// outages of the script device, and scriptAlarmsPastMax its cleared
	// alarms.
	scriptTasksMax      = 4
	scriptOutagesMax    = 4
	scriptAlarmsPastMax = 8
	// scriptTaskFirst and scriptAlarmFirst number the first scripted task and
	// alarm, after the last ones of the tables.
	scriptTaskFirst  uint64 = 2051
	scriptAlarmFirst uint64 = 1061
	// scriptTaskLead is how long before the start the first scripted task
	// was created.
	scriptTaskLead = 40 * time.Second
)

// The prefixes of the identifiers of the sample hub.
const (
	taskPrefix  = "T-"
	alarmPrefix = "A-"
)

// outage is one period the script device was offline. to means nothing while
// open is true.
type outage struct {
	from time.Time
	to   time.Time
	open bool
}

// scriptTask is one task the script created.
type scriptTask struct {
	id      string
	started time.Time
	state   fleetui.TaskState
	// ended means nothing while state is running.
	ended   time.Time
	results []fleetui.TaskResult
}

// scriptState is everything the script changes. The source guards it with its
// lock and hands out copies.
type scriptState struct {
	// next is the index of the step to apply.
	next int

	// offline reports that the script device is offline, since offlineAt,
	// with its offline alarm open under offlineAlarmID.
	offline        bool
	offlineAt      time.Time
	offlineAlarmID string
	// returned reports that the script device came back at least once, the
	// last time at backAt.
	returned bool
	backAt   time.Time
	outages  []outage

	// alarmOpen reports that the scripted alarm is open, under alarmID, since
	// alarmSince.
	alarmOpen   bool
	alarmID     string
	alarmSince  time.Time
	alarmsPast  []fleetui.AlarmEvent
	alarmNumber uint64

	// tasks holds the scripted tasks, the newest last. It is never empty: the
	// last one is the task the script works on.
	tasks      []scriptTask
	taskNumber uint64
}

// newScriptState answers the state the script starts from: one running task
// with no result yet.
func newScriptState(start time.Time) scriptState {
	state := scriptState{alarmNumber: scriptAlarmFirst, taskNumber: scriptTaskFirst}
	state.createTask(start.Add(-scriptTaskLead))

	return state
}

// clone answers a copy that shares no slice with state.
func (state *scriptState) clone() scriptState {
	copied := *state
	copied.outages = slices.Clone(state.outages)
	copied.alarmsPast = slices.Clone(state.alarmsPast)

	copied.tasks = make([]scriptTask, len(state.tasks))
	for at := range state.tasks {
		copied.tasks[at] = state.tasks[at]
		copied.tasks[at].results = slices.Clone(state.tasks[at].results)
	}

	return copied
}

// createTask adds a running task with the next number, and drops the oldest
// beyond scriptTasksMax.
func (state *scriptState) createTask(now time.Time) *scriptTask {
	state.tasks = append(state.tasks, scriptTask{
		id:      textbuf.New().Str(taskPrefix).Uint(state.taskNumber).Slice(),
		started: now,
		state:   fleetui.TaskStateRunning,
	})
	state.taskNumber++

	if len(state.tasks) > scriptTasksMax {
		state.tasks = slices.Delete(state.tasks, 0, len(state.tasks)-scriptTasksMax)
	}

	return state.currentTask()
}

// currentTask answers the task the script works on.
func (state *scriptState) currentTask() *scriptTask {
	return &state.tasks[len(state.tasks)-1]
}

// nextAlarmID answers the identifier of the next alarm the script opens: the
// offline alarm of its device and its memory alarm share one numbering.
func (state *scriptState) nextAlarmID() string {
	id := textbuf.New().Str(alarmPrefix).Uint(state.alarmNumber).Slice()
	state.alarmNumber++

	return id
}

// scriptStep is one step of the script. apply changes the state and answers
// the event that says so.
type scriptStep struct {
	apply func(state *scriptState, step *scriptStep, now time.Time) fleetui.Event
	// device is the device the step is about.
	device string
	// result, answer and duration are the result a task step records. result
	// is the zero value for every other step, and scriptTaskDevices reads
	// that to tell a task step: a result is never unspecified.
	result   fleetui.TaskState
	answer   string
	duration time.Duration
}

// scriptSteps is the script. After the last step it starts again, on the
// task the last step created.
var scriptSteps = []scriptStep{
	{apply: stepDeviceOffline, device: scriptDevice},
	{apply: stepAlarmOpen, device: scriptAlarmDevice},
	{
		apply: stepTaskResult, device: deviceCpeAms01, result: fleetui.TaskStateSucceeded,
		answer: answerDiagnostic, duration: 2100 * time.Millisecond,
	},
	{
		apply: stepTaskResult, device: deviceCpeAms02, result: fleetui.TaskStateSucceeded,
		answer: answerDiagnostic, duration: 2400 * time.Millisecond,
	},
	{apply: stepDeviceBack, device: scriptDevice},
	{apply: stepAlarmClear, device: scriptAlarmDevice},
	{
		apply: stepTaskResult, device: deviceCpeFra03, result: fleetui.TaskStateFailed,
		answer: relayRefusal, duration: 300 * time.Millisecond,
	},
	{
		apply: stepTaskResult, device: deviceCpeFra01, result: fleetui.TaskStateSucceeded,
		answer: answerDiagnostic, duration: 1900 * time.Millisecond,
	},
	{apply: stepTaskEnd},
	{apply: stepTaskCreate},
}

// scriptTaskDevices answers the devices a scripted task targets: the devices
// of the task steps, in their order.
func scriptTaskDevices() []string {
	devices := make([]string, 0, len(scriptSteps))
	for at := range scriptSteps {
		if scriptSteps[at].result != fleetui.TaskStateUnspecified {
			devices = append(devices, scriptSteps[at].device)
		}
	}

	return devices
}

// stepDeviceOffline takes the script device offline.
func stepDeviceOffline(state *scriptState, step *scriptStep, now time.Time) fleetui.Event {
	state.offline = true
	state.offlineAt = now
	state.offlineAlarmID = state.nextAlarmID()

	state.outages = append(state.outages, outage{from: now, open: true})
	if len(state.outages) > scriptOutagesMax {
		state.outages = slices.Delete(state.outages, 0, len(state.outages)-scriptOutagesMax)
	}

	return fleetui.Event{Kind: fleetui.EventDeviceChanged, At: now, Device: step.device}
}

// stepDeviceBack brings the script device back.
func stepDeviceBack(state *scriptState, step *scriptStep, now time.Time) fleetui.Event {
	state.offline = false
	state.returned = true
	state.backAt = now

	if len(state.outages) > 0 {
		last := &state.outages[len(state.outages)-1]
		last.to = now
		last.open = false
	}

	return fleetui.Event{Kind: fleetui.EventDeviceChanged, At: now, Device: step.device}
}

// stepAlarmOpen opens the scripted alarm under the next number.
func stepAlarmOpen(state *scriptState, step *scriptStep, now time.Time) fleetui.Event {
	state.alarmOpen = true
	state.alarmSince = now
	state.alarmID = state.nextAlarmID()

	return fleetui.Event{Kind: fleetui.EventAlarmOpened, At: now, Device: step.device, Subject: state.alarmID}
}

// stepAlarmClear clears the scripted alarm and keeps it as history.
func stepAlarmClear(state *scriptState, step *scriptStep, now time.Time) fleetui.Event {
	state.alarmOpen = false

	state.alarmsPast = append(state.alarmsPast, fleetui.AlarmEvent{
		Kind:      alarmResourceHigh,
		Severity:  fleetui.AlarmSeverityWarning,
		Summary:   scriptAlarmSummary,
		Opened:    state.alarmSince,
		Cleared:   true,
		ClearedAt: now,
	})
	if len(state.alarmsPast) > scriptAlarmsPastMax {
		state.alarmsPast = slices.Delete(state.alarmsPast, 0, len(state.alarmsPast)-scriptAlarmsPastMax)
	}

	return fleetui.Event{Kind: fleetui.EventAlarmCleared, At: now, Device: step.device, Subject: state.alarmID}
}

// stepTaskResult records the result of one device of the current task.
func stepTaskResult(state *scriptState, step *scriptStep, now time.Time) fleetui.Event {
	task := state.currentTask()
	task.results = append(task.results, fleetui.TaskResult{
		Device:   step.device,
		State:    step.result,
		Answer:   step.answer,
		Duration: step.duration,
	})

	return fleetui.Event{Kind: fleetui.EventTaskProgressed, At: now, Device: step.device, Subject: task.id}
}

// stepTaskEnd ends the current task in the state its results give it.
func stepTaskEnd(state *scriptState, _ *scriptStep, now time.Time) fleetui.Event {
	task := state.currentTask()
	task.state = endedState(task.results)
	task.ended = now

	return fleetui.Event{Kind: fleetui.EventTaskEnded, At: now, Subject: task.id}
}

// stepTaskCreate creates the task the next round of the script works on.
func stepTaskCreate(state *scriptState, _ *scriptStep, now time.Time) fleetui.Event {
	task := state.createTask(now)

	return fleetui.Event{Kind: fleetui.EventTaskCreated, At: now, Subject: task.id}
}

// endedState answers the state of a task that ended with results: succeeded
// when none failed, failed when all did, partly failed otherwise.
func endedState(results []fleetui.TaskResult) fleetui.TaskState {
	failed := 0
	for at := range results {
		if results[at].State == fleetui.TaskStateFailed {
			failed++
		}
	}

	if failed == 0 {
		return fleetui.TaskStateSucceeded
	}
	if failed == len(results) {
		return fleetui.TaskStateFailed
	}

	return fleetui.TaskStatePartlyFailed
}

// Step applies the next step of the script and answers its event. It holds
// the script too, so the timer and a caller that steps never interleave.
func (source *estate) Step() (fleetui.Event, error) {
	source.mu.Lock()
	defer source.mu.Unlock()

	if source.closed {
		return fleetui.Event{}, opererr.Wrap(fleetui.ErrFeedClosed, "The sample script has stopped.",
			"The sample source was closed, so it applies no more steps and publishes no more events.",
			"restart the review build")
	}
	source.held = true

	return source.stepLocked(), nil
}

// Hold stops the timer from advancing the script, for good.
func (source *estate) Hold() {
	source.mu.Lock()
	defer source.mu.Unlock()

	source.held = true
}

// stepLocked applies the next step and publishes its event. The caller MUST
// hold source.mu. Publish never blocks, so holding the lock across it is what
// keeps the events in the order of the changes.
func (source *estate) stepLocked() fleetui.Event {
	state := &source.script

	step := &scriptSteps[state.next]
	event := step.apply(state, step, source.clock())
	state.next = (state.next + 1) % len(scriptSteps)

	source.feed.Publish(event)

	return event
}

// startScript starts the script timer, once. A closed source starts none.
func (source *estate) startScript() {
	source.mu.Lock()
	defer source.mu.Unlock()

	if source.started {
		return
	}
	if source.closed {
		return
	}
	source.started = true

	go source.runScript()
}

// runScript is the script timer: the one goroutine of the source. It applies
// a step every period until Close closes source.stop, and closes source.done
// when it ends.
func (source *estate) runScript() {
	defer close(source.done)

	ticker := time.NewTicker(source.period)
	defer ticker.Stop()

	// This loop has no bound of its own: it ends when Close closes stop.
	for {
		select {
		case <-source.stop:
			return
		case <-ticker.C:
			source.tick()
		}
	}
}

// tick applies one step, unless the script is held or the source is closed.
func (source *estate) tick() {
	source.mu.Lock()
	defer source.mu.Unlock()

	if source.held {
		return
	}
	if source.closed {
		return
	}

	source.stepLocked()
}
