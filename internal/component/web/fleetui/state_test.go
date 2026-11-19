package fleetui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeviceDisplayStatePrecedence derives the display state of one device per
// boundary of the precedence: each case holds the facts of two states at
// once, and expects the one that comes first.
//
// VALIDATES: the display state follows the order offline, diverged, down,
// degraded, then healthy or unknown.
// PREVENTS: a device shown healthy while a component is down, a diverged
// device hidden behind its health, and an offline device shown with the
// state it had when it left.
func TestDeviceDisplayStatePrecedence(t *testing.T) {
	healthy := HealthComponent{Name: "bgp", State: HealthStateHealthy}
	degraded := HealthComponent{Name: "disk", State: HealthStateDegraded}
	down := HealthComponent{Name: "fib", State: HealthStateDown}
	unjudged := HealthComponent{Name: "new", State: HealthState(99)}
	unset := HealthComponent{Name: "unset"}

	cases := []struct {
		name      string
		connected bool
		config    ConfigState
		reported  bool
		health    []HealthComponent
		want      DisplayState
	}{
		{"offline beats diverged and down", false, ConfigStateDiverged, true, []HealthComponent{down}, DisplayStateOffline},
		{"offline with no report at all", false, ConfigStateNeverFetched, false, nil, DisplayStateOffline},
		{"diverged beats down", true, ConfigStateDiverged, true, []HealthComponent{down}, DisplayStateDiverged},
		{"diverged beats an absent report", true, ConfigStateDiverged, false, nil, DisplayStateDiverged},
		{"down beats degraded", true, ConfigStateInSync, true, []HealthComponent{degraded, down}, DisplayStateDown},
		{"down beats a state the hub does not know", true, ConfigStateInSync, true, []HealthComponent{unjudged, down}, DisplayStateDown},
		{"degraded beats healthy", true, ConfigStateInSync, true, []HealthComponent{healthy, degraded}, DisplayStateDegraded},
		{"degraded beats a state the hub does not know", true, ConfigStateInSync, true, []HealthComponent{unjudged, degraded}, DisplayStateDegraded},
		{"every component healthy", true, ConfigStateInSync, true, []HealthComponent{healthy, healthy}, DisplayStateHealthy},
		{"a report with no component", true, ConfigStateInSync, true, nil, DisplayStateHealthy},
		{"no report yet", true, ConfigStateInSync, false, nil, DisplayStateUnknown},
		{"a state the hub does not know", true, ConfigStateInSync, true, []HealthComponent{healthy, unjudged}, DisplayStateUnknown},
		{"a component with no state", true, ConfigStateInSync, true, []HealthComponent{unset}, DisplayStateUnknown},
		{"a pending configuration is not a divergence", true, ConfigStatePending, true, []HealthComponent{healthy}, DisplayStateHealthy},
		{"a rejected configuration is not a divergence", true, ConfigStateRejected, true, []HealthComponent{healthy}, DisplayStateHealthy},
	}

	for index := range cases {
		tc := &cases[index]
		t.Run(tc.name, func(t *testing.T) {
			device := Device{
				Name:           "device",
				Connection:     Connection{Connected: tc.connected},
				Config:         ConfigFacts{State: tc.config},
				HealthReported: tc.reported,
				Health:         tc.health,
			}

			assert.Equal(t, tc.want, DeviceDisplayState(&device))
		})
	}
}

// TestDisplayStateSeverityOrder ranks every display state.
//
// VALIDATES: the order a state column sorts by is down, offline, diverged,
// degraded, unknown, healthy, the worst first.
// PREVENTS: a sort by state that puts a healthy device above a failing one.
func TestDisplayStateSeverityOrder(t *testing.T) {
	worstFirst := []DisplayState{
		DisplayStateDown, DisplayStateOffline, DisplayStateDiverged,
		DisplayStateDegraded, DisplayStateUnknown, DisplayStateHealthy,
	}

	for index := 1; index < len(worstFirst); index++ {
		worse, better := worstFirst[index-1], worstFirst[index]
		assert.Greater(t, worse.Severity(), better.Severity(), "%s must rank above %s", worse, better)
	}
}

// TestStateWordsAndKeys reads the word and the key of every state a page
// shows, and parses each key back.
//
// VALIDATES: every display, configuration, alarm and task state has the word
// the fleet interface fixes, and a key that parses back to it.
// PREVENTS: a state rendered as "invalid", and a filter value that names no
// state.
func TestStateWordsAndKeys(t *testing.T) {
	displayWords := map[DisplayState]string{
		DisplayStateOffline: "Offline", DisplayStateDiverged: "Diverged", DisplayStateDown: "Down",
		DisplayStateDegraded: "Degraded", DisplayStateHealthy: "Healthy", DisplayStateUnknown: "Unknown",
	}
	for state, word := range displayWords {
		assert.Equal(t, word, state.String())
		parsed, known := ParseDisplayState(state.Key())
		assert.True(t, known, state.Key())
		assert.Equal(t, state, parsed)
	}

	configWords := map[ConfigState]string{
		ConfigStateInSync: "In sync", ConfigStatePending: "Pending", ConfigStateRejected: "Rejected",
		ConfigStateDiverged: "Diverged", ConfigStateNeverFetched: "Never fetched",
	}
	for state, word := range configWords {
		assert.Equal(t, word, state.String())
		parsed, known := ParseConfigState(state.Key())
		assert.True(t, known, state.Key())
		assert.Equal(t, state, parsed)
	}

	for _, severity := range AlarmSeverities() {
		parsed, known := ParseAlarmSeverity(severity.Key())
		assert.True(t, known, severity.Key())
		assert.Equal(t, severity, parsed)
	}

	taskWords := map[TaskState]string{
		TaskStatePending: "Pending", TaskStateRunning: "Running", TaskStateSucceeded: "Succeeded",
		TaskStateFailed: "Failed", TaskStateCanceled: "Canceled", TaskStatePartlyFailed: "Partly failed",
	}
	for state, word := range taskWords {
		assert.Equal(t, word, state.String())
		parsed, known := ParseTaskState(state.Key())
		assert.True(t, known, state.Key())
		assert.Equal(t, state, parsed)
	}
}

// TestStateZeroValueIsNotAState parses the key of the invalid zero value and
// renders a value outside every named set.
//
// VALIDATES: no key parses to the zero value, and a value outside the named
// set renders as invalid, never as a state.
// PREVENTS: a field nobody set passing for a real state.
func TestStateZeroValueIsNotAState(t *testing.T) {
	_, known := ParseDisplayState(DisplayStateUnspecified.Key())
	assert.False(t, known)

	_, known = ParseConfigState("")
	assert.False(t, known)

	assert.Equal(t, enumInvalid, DisplayState(200).String())
	assert.Equal(t, enumInvalid, TaskState(200).Key())
}

// TestTaskStateActive asks every task state whether its task is still at
// work.
//
// VALIDATES: pending and running are active, and every ended state is not.
// PREVENTS: a finished task counted as running in the header and the tree.
func TestTaskStateActive(t *testing.T) {
	assert.True(t, TaskStatePending.Active())
	assert.True(t, TaskStateRunning.Active())

	for _, state := range []TaskState{
		TaskStateUnspecified, TaskStateSucceeded, TaskStateFailed, TaskStateCanceled, TaskStatePartlyFailed, TaskState(200),
	} {
		assert.False(t, state.Active(), state.String())
	}
}

// TestDeriveDevicesJoinsAlarmsAndTasks derives the views of the test estate.
//
// VALIDATES: each view carries its display state, the count of its open
// alarms, the count of the active tasks that target it, its uptime and how
// long ago it was seen.
// PREVENTS: a finished task counted as active, and an offline device given
// an uptime.
func TestDeriveDevicesJoinsAlarmsAndTasks(t *testing.T) {
	views := testViews()

	states := make(map[string]DisplayState, len(views))
	for index := range views {
		states[views[index].Device.Name] = views[index].State
	}
	assert.Equal(t, map[string]DisplayState{
		edgeLon01: DisplayStateHealthy, edgeLon02: DisplayStateHealthy, coreLon01: DisplayStateDown,
		coreAms01: DisplayStateOffline, rsAms01: DisplayStateDiverged, cpe0113: DisplayStateDegraded,
		cpe0200: DisplayStateUnknown,
	}, states)

	down, err := FindDevice(views, coreLon01)
	require.NoError(t, err)
	assert.Equal(t, 2, down.AlarmsOpen)
	assert.Equal(t, 0, down.TasksActive, "the task that targeted it has ended")

	busy, err := FindDevice(views, edgeLon01)
	require.NoError(t, err)
	assert.Equal(t, 1, busy.TasksActive)
	assert.True(t, busy.UptimeKnown)
	assert.Equal(t, 41*24, int(busy.Uptime.Hours()))

	offline, err := FindDevice(views, coreAms01)
	require.NoError(t, err)
	assert.False(t, offline.UptimeKnown)
	assert.True(t, offline.SeenKnown)
	assert.Equal(t, 6*24, int(offline.SeenAgo.Hours()))
}

// TestFindDeviceRefusesUnknownName looks for a device the estate does not
// hold.
//
// VALIDATES: the refusal is an operator error that names the device and
// reads as not found.
// PREVENTS: an unknown device answered as an empty device.
func TestFindDeviceRefusesUnknownName(t *testing.T) {
	_, err := FindDevice(testViews(), "no-such-device")

	refusal := requireRefusalOf(t, err, ErrObjectUnknown)
	assert.Contains(t, refusal.Problem(), `"no-such-device"`)
	assert.Equal(t, FailureNotFound, FailureOf(err))
}
