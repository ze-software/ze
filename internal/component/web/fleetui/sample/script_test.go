package sample

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/web/fleetui"
)

// received answers the event waiting on the subscription, which Step
// published before it returned.
func received(t *testing.T, subscription *fleetui.Subscription) fleetui.Event {
	t.Helper()

	select {
	case event, open := <-subscription.Events():
		require.True(t, open, "the subscription ended")
		return event
	default:
		require.Fail(t, "Step published no event")
		return fleetui.Event{}
	}
}

// findAlarm answers the open alarm with the identifier, or false.
func findAlarm(t *testing.T, source *estate, id string) (fleetui.Alarm, bool) {
	t.Helper()

	alarms, err := source.Alarms(t.Context())
	require.NoError(t, err)

	for at := range alarms {
		if alarms[at].ID == id {
			return alarms[at], true
		}
	}

	return fleetui.Alarm{}, false
}

// TestSampleScriptStepsAgree steps the script through two rounds and, after
// each step, compares the event published with the facts the source answers.
//
// VALIDATES: a device an event shows offline is offline in Devices and
// refuses its views, an alarm an event opens is in Alarms and one it clears
// is in the device's history, and a task an event progresses, ends or creates
// reads so in Tasks and Task. The second round works on the task the first
// created.
// PREVENTS: a live page that redraws on an event and shows the old state,
// because the event and the facts came from two places.
func TestSampleScriptStepsAgree(t *testing.T) {
	source := newTestEstate(t)
	ctx := t.Context()

	subscription, err := source.Subscribe(ctx)
	require.NoError(t, err)
	t.Cleanup(subscription.Close)

	var stepper Stepper = source

	offline := false
	results := 0
	ended := 0
	taskIDs := map[string]struct{}{}

	for step := range 2 * len(scriptSteps) {
		event, stepErr := stepper.Step()
		require.NoError(t, stepErr)
		require.Equal(t, event, received(t, subscription), "step %d", step)
		assert.Equal(t, testStart, event.At)

		switch event.Kind {
		case fleetui.EventDeviceChanged:
			offline = !offline

			device, deviceErr := source.Device(ctx, event.Device)
			require.NoError(t, deviceErr)
			assert.Equal(t, !offline, device.Connection.Connected, "step %d", step)

			_, viewErr := source.DeviceInterfaces(ctx, event.Device)
			if offline {
				assert.Equal(t, fleetui.DisplayStateOffline, fleetui.DeviceDisplayState(&device))
				assert.Equal(t, event.At, device.Connection.LastSeen)
				require.Error(t, viewErr)
				assert.Equal(t, fleetui.FailureOffline, fleetui.FailureOf(viewErr))
			} else {
				assert.Equal(t, fleetui.DisplayStateHealthy, fleetui.DeviceDisplayState(&device))
				assert.Equal(t, event.At, device.Connection.Since)
				require.NoError(t, viewErr)
			}

		case fleetui.EventAlarmOpened:
			alarm, open := findAlarm(t, source, event.Subject)
			require.True(t, open, "step %d", step)
			assert.Equal(t, event.Device, alarm.Device)

			device, deviceErr := source.Device(ctx, event.Device)
			require.NoError(t, deviceErr)
			assert.Greater(t, device.Latest.MemoryPercent, float64(resourceAlarmPercent))

		case fleetui.EventAlarmCleared:
			_, open := findAlarm(t, source, event.Subject)
			assert.False(t, open, "step %d", step)

			history, historyErr := source.AlarmHistory(ctx, event.Device)
			require.NoError(t, historyErr)
			require.NotEmpty(t, history)
			assert.True(t, history[0].Cleared)

			device, deviceErr := source.Device(ctx, event.Device)
			require.NoError(t, deviceErr)
			assert.Less(t, device.Latest.MemoryPercent, float64(resourceAlarmPercent))

		case fleetui.EventTaskProgressed:
			results++

			detail, taskErr := source.Task(ctx, event.Subject)
			require.NoError(t, taskErr)
			require.Len(t, detail.Results, results, "step %d", step)
			assert.Equal(t, results, detail.Task.Finished)
			assert.Equal(t, event.Device, detail.Results[results-1].Device)
			assert.Equal(t, fleetui.TaskStateRunning, detail.Task.State)
			assert.Contains(t, detail.Task.Devices, event.Device)

		case fleetui.EventTaskEnded:
			ended++

			detail, taskErr := source.Task(ctx, event.Subject)
			require.NoError(t, taskErr)
			assert.Equal(t, fleetui.TaskStatePartlyFailed, detail.Task.State)
			assert.Equal(t, 1, detail.Task.Failed)
			assert.Len(t, detail.Results, len(detail.Task.Devices))
			assert.Equal(t, event.At, detail.Task.Ended)

		case fleetui.EventTaskCreated:
			results = 0
			taskIDs[event.Subject] = struct{}{}

			detail, taskErr := source.Task(ctx, event.Subject)
			require.NoError(t, taskErr)
			assert.Equal(t, fleetui.TaskStateRunning, detail.Task.State)
			assert.Empty(t, detail.Results)

		case fleetui.EventRegistryChanged, fleetui.EventUnspecified:
			require.Fail(t, "the script published an event it does not hold", "step %d: %s", step, event.Kind)
		}
	}

	assert.False(t, offline, "the device came back at the end of each round")
	assert.Equal(t, 2, ended, "one task ended in each round")
	assert.Len(t, taskIDs, 2, "each round created its own task")

	tasks, err := source.Tasks(ctx)
	require.NoError(t, err)
	running := 0
	for at := range tasks {
		if tasks[at].State == fleetui.TaskStateRunning {
			running++
		}
	}
	assert.Equal(t, 2, running, "the task the script created and the one of the table")
}

// TestSampleScriptRunsOnItsTimer subscribes to a source whose script is not
// held, and reads the first events its timer publishes.
//
// VALIDATES: the first Subscribe starts the one timer, the timer applies the
// steps in order, Hold stops it from applying more, and Close ends it and the
// subscription.
// PREVENTS: a review build whose pages never move, and a timer that outlives
// its source.
func TestSampleScriptRunsOnItsTimer(t *testing.T) {
	source := newEstate(fixedClock)
	source.period = 20 * time.Millisecond

	subscription, err := source.Subscribe(t.Context())
	require.NoError(t, err)

	want := []fleetui.EventKind{
		fleetui.EventDeviceChanged, fleetui.EventAlarmOpened, fleetui.EventTaskProgressed,
	}
	// The deadline bounds a timer that never fires. It is not a wait: each
	// read returns as soon as its event is there.
	deadline := time.After(30 * time.Second)
	for _, kind := range want {
		select {
		case event, open := <-subscription.Events():
			require.True(t, open, "the subscription ended early")
			assert.Equal(t, kind, event.Kind)
		case <-deadline:
			require.Fail(t, "the timer published no event")
		}
	}

	source.Hold()
	source.Close()

	// Close ended the subscription: the events the timer published before Hold
	// are still there, at most a buffer of them, and then the channel is
	// closed.
	closed := false
	for range fleetui.EventBufferSize + 1 {
		if _, open := <-subscription.Events(); !open {
			closed = true
			break
		}
	}
	assert.True(t, closed, "Close ends the subscription")

	_, err = source.Step()
	require.Error(t, err, "a closed source refuses to step")
}

// openOfflineAlarm answers the open device-offline alarm of the device, or
// false.
func openOfflineAlarm(t *testing.T, source *estate, device string) (fleetui.Alarm, bool) {
	t.Helper()

	alarms, err := source.Alarms(t.Context())
	require.NoError(t, err)

	for at := range alarms {
		if alarms[at].Device == device && alarms[at].Kind == alarmDeviceOffline {
			return alarms[at], true
		}
	}

	return fleetui.Alarm{}, false
}

// TestSampleScriptOfflineAlarmFollowsTheDevice steps the script through one
// outage of its device.
//
// VALIDATES: the script device's critical device-offline alarm opens when
// the script takes it offline and clears, into its history, when the script
// brings it back, as the estate's own offline devices each have one open.
// PREVENTS: a device the page shows offline with no offline alarm, and an
// offline alarm left open on a device that came back.
func TestSampleScriptOfflineAlarmFollowsTheDevice(t *testing.T) {
	source := newTestEstate(t)
	var stepper Stepper = source

	_, open := openOfflineAlarm(t, source, scriptDevice)
	assert.False(t, open, "the script device starts online")

	changes := 0
	for range scriptSteps {
		event, err := stepper.Step()
		require.NoError(t, err)
		if event.Kind != fleetui.EventDeviceChanged {
			continue
		}

		changes++
		alarm, opened := openOfflineAlarm(t, source, scriptDevice)
		if changes == 1 {
			require.True(t, opened, "the alarm opens with the outage")
			assert.Equal(t, fleetui.AlarmSeverityCritical, alarm.Severity)
			assert.Equal(t, event.At, alarm.Since)

			continue
		}

		assert.False(t, opened, "the alarm clears when the device comes back")

		history, historyErr := source.AlarmHistory(t.Context(), scriptDevice)
		require.NoError(t, historyErr)
		require.NotEmpty(t, history)
		assert.Equal(t, alarmDeviceOffline, history[0].Kind)
		assert.True(t, history[0].Cleared)
	}

	assert.Equal(t, 2, changes, "one round takes the device offline and brings it back")
}

// TestSampleScriptHeldBeforeItsFirstSubscription holds a new source before
// anything subscribes, as the register package does when the review build is
// started held, then subscribes, which starts the timer, and fires the
// timer's tick by hand rather than waiting for it.
//
// VALIDATES: a source held before its first subscription applies no step on
// a tick, and the first Step is the first step of the script, published as
// the first event the subscription carries.
// PREVENTS: a held review build whose timer still moves the estate, so the
// step a browser test sees depends on how loaded the machine is.
func TestSampleScriptHeldBeforeItsFirstSubscription(t *testing.T) {
	source := newEstate(fixedClock)
	source.Hold()

	subscription, err := source.Subscribe(t.Context())
	require.NoError(t, err)
	t.Cleanup(source.Close)

	source.tick()

	select {
	case event := <-subscription.Events():
		require.Fail(t, "a tick of a held script published an event", "%s", event.Kind)
	default:
	}

	source.mu.Lock()
	next := source.script.next
	source.mu.Unlock()
	require.Zero(t, next, "a tick of a held script applies no step")

	event, err := source.Step()
	require.NoError(t, err)
	assert.Equal(t, fleetui.EventDeviceChanged, event.Kind, "the first step takes a device offline")
	assert.Equal(t, scriptDevice, event.Device)
	assert.Equal(t, event, received(t, subscription), "the first event published is the stepped one")
}
