package sample

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/web/fleetui"
)

// testStart is the moment the test sources are made at.
var testStart = time.Date(2026, time.November, 18, 15, 0, 0, 0, time.UTC)

// fixedClock is a clock that does not move.
func fixedClock() time.Time { return testStart }

// newTestEstate builds a sample source on the fixed clock, with its script
// held so that only a test's own Step changes it.
func newTestEstate(t *testing.T) *estate {
	t.Helper()

	source := newEstate(fixedClock)
	source.Hold()
	t.Cleanup(source.Close)

	return source
}

// devices answers the sample devices at the fixed clock.
func devices() []fleetui.Device {
	return devicesAt(newEstate(fixedClock).snapshot())
}

// estateFacts is the lists every invariant is counted over, read once
// through the source, and the views the fleet package derives from them.
type estateFacts struct {
	now      time.Time
	devices  []fleetui.Device
	alarms   []fleetui.Alarm
	tasks    []fleetui.Task
	rollouts []fleetui.Rollout
	views    []fleetui.DeviceView
}

// readEstate asks the source for its facts and derives the views.
func readEstate(t *testing.T, source *estate) *estateFacts {
	t.Helper()

	ctx := t.Context()
	facts := &estateFacts{}

	var err error
	facts.now, err = source.Now(ctx)
	require.NoError(t, err)
	facts.devices, err = source.Devices(ctx)
	require.NoError(t, err)
	facts.alarms, err = source.Alarms(ctx)
	require.NoError(t, err)
	facts.tasks, err = source.Tasks(ctx)
	require.NoError(t, err)
	facts.rollouts, err = source.Rollouts(ctx)
	require.NoError(t, err)

	facts.views = fleetui.DeriveDevices(facts.devices, facts.alarms, facts.tasks, facts.now)

	return facts
}

// view answers the view of the device named name.
func (facts *estateFacts) view(t *testing.T, name string) *fleetui.DeviceView {
	t.Helper()

	found, err := fleetui.FindDevice(facts.views, name)
	require.NoError(t, err)

	return found
}

// documentationRanges are the address ranges reserved for documentation.
var documentationRanges = []netip.Prefix{
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("2001:db8::/32"),
}

// documented reports whether addr is in a documentation range.
func documented(addr netip.Addr) bool {
	for _, prefix := range documentationRanges {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}

// TestSampleEstateInvariants counts the sample estate through the fleet
// package's own derivation functions.
//
// VALIDATES: every count of the "sample estate" table of the fleet-ui-1
// spec, and that the lists agree: an alarm, a task, a rollout and an audit
// entry name devices the estate holds, and the paused rollout names the
// device whose configuration is rejected.
// PREVENTS: a dashboard whose cards disagree with each other because two
// tables of the sample estate drifted apart.
func TestSampleEstateInvariants(t *testing.T) {
	source := newTestEstate(t)
	facts := readEstate(t, source)

	t.Run("devices, sites, roles and labels", func(t *testing.T) {
		require.Len(t, facts.devices, 44)
		assert.Equal(t, testStart, facts.now)

		names := map[string]struct{}{}
		sites := map[string]int{}
		roles := map[string]int{}
		unsited := 0
		for at := range facts.views {
			view := &facts.views[at]
			names[view.Device.Name] = struct{}{}

			site, sited := view.Label(labelSite)
			if sited {
				sites[site]++
			} else {
				unsited++
			}

			role, labeled := view.Label(labelRole)
			require.True(t, labeled, view.Device.Name)
			roles[role]++

			assert.True(t, documented(view.Device.Inventory.ManagementAddr), view.Device.Name)
			assert.True(t, documented(view.Device.Connection.SourceAddr), view.Device.Name)
		}

		assert.Len(t, names, 44, "every device name is unique")
		assert.Equal(t, map[string]int{"lon": 12, "ams": 10, "fra": 10, "par": 9}, sites)
		assert.Equal(t, 3, unsited)
		assert.Equal(t, map[string]int{"edge": 12, "core": 8, "route-server": 4, "customer-premises": 20}, roles)
		assert.ElementsMatch(t, []string{labelSite, labelRole, labelModel, labelCustomer}, fleetui.LabelKeys(facts.views))
	})

	t.Run("display states", func(t *testing.T) {
		counts := map[fleetui.DisplayState]int{}
		for _, count := range fleetui.CountByState(facts.views) {
			counts[count.State] = count.Count
		}

		assert.Equal(t, map[fleetui.DisplayState]int{
			fleetui.DisplayStateHealthy:  35,
			fleetui.DisplayStateDegraded: 3,
			fleetui.DisplayStateDown:     1,
			fleetui.DisplayStateOffline:  3,
			fleetui.DisplayStateDiverged: 1,
			fleetui.DisplayStateUnknown:  1,
		}, counts)

		sixDays := 0
		for at := range facts.views {
			view := &facts.views[at]
			if view.State == fleetui.DisplayStateOffline {
				require.True(t, view.SeenKnown, view.Device.Name)
				if view.SeenAgo >= 6*day {
					sixDays++
				}
			}
			if view.State == fleetui.DisplayStateUnknown {
				assert.True(t, view.Device.Connection.Connected, "the unknown device is connected")
				assert.False(t, view.Device.HealthReported, "and has sent no health report")
			}
		}
		assert.Equal(t, 1, sixDays, "one device has been offline for six days")
	})

	t.Run("software and configuration", func(t *testing.T) {
		assert.Equal(t, []string{"1.4.2", "1.4.1", "1.3.9"}, fleetui.Versions(facts.views))

		perVersion := []int{}
		for _, count := range fleetui.CountByVersion(facts.views) {
			require.Equal(t, fleetui.VersionBucketNamed, count.Bucket)
			perVersion = append(perVersion, count.Count)
		}
		assert.Equal(t, []int{31, 12, 1}, perVersion, "one device is two versions behind")

		config := map[fleetui.ConfigState]int{}
		for _, count := range fleetui.CountByConfig(facts.views) {
			config[count.State] = count.Count
		}
		assert.Equal(t, map[fleetui.ConfigState]int{
			fleetui.ConfigStateInSync:       39,
			fleetui.ConfigStatePending:      2,
			fleetui.ConfigStateRejected:     1,
			fleetui.ConfigStateDiverged:     1,
			fleetui.ConfigStateNeverFetched: 1,
		}, config)
	})

	t.Run("needs attention", func(t *testing.T) {
		lines, err := fleetui.NeedsAttention(facts.views, facts.rollouts)
		require.NoError(t, err)

		counts := map[fleetui.AttentionKind]int{}
		for _, line := range lines {
			counts[line.Kind] = line.Count
		}
		assert.Equal(t, map[fleetui.AttentionKind]int{
			fleetui.AttentionRolloutPaused:  1,
			fleetui.AttentionDeviceDiverged: 1,
			fleetui.AttentionConfigRejected: 1,
			fleetui.AttentionOfflineLong:    1,
			fleetui.AttentionSoftwareBehind: 1,
		}, counts)
	})

	t.Run("alarms", func(t *testing.T) {
		require.Len(t, facts.alarms, 9)

		severities := map[fleetui.AlarmSeverity]int{}
		open := 0
		for at := range facts.alarms {
			alarm := &facts.alarms[at]
			severities[alarm.Severity]++
			assertAlarmFitsDevice(t, alarm, facts.view(t, alarm.Device))
		}
		for at := range facts.views {
			open += facts.views[at].AlarmsOpen
		}

		assert.Equal(t, map[fleetui.AlarmSeverity]int{
			fleetui.AlarmSeverityCritical:      1,
			fleetui.AlarmSeverityWarning:       5,
			fleetui.AlarmSeverityInformational: 3,
		}, severities)
		assert.Equal(t, 9, open, "every alarm is counted on a device")
	})

	t.Run("alarm history", func(t *testing.T) {
		for at := range facts.alarms {
			device := facts.alarms[at].Device

			history, err := source.AlarmHistory(t.Context(), device)
			require.NoError(t, err)
			require.NotEmpty(t, history, device)

			uncleared := 0
			for index := range history {
				event := &history[index]
				assert.LessOrEqual(t, facts.now.Sub(event.Opened), historyRetention, device)
				if index > 0 {
					assert.False(t, event.Opened.After(history[index-1].Opened), "%s: newest first", device)
				}
				if !event.Cleared {
					uncleared++
				}
			}
			assert.Equal(t, facts.view(t, device).AlarmsOpen, uncleared, device)
		}
	})

	t.Run("tasks", func(t *testing.T) {
		states := map[fleetui.TaskState]int{}
		for at := range facts.tasks {
			task := &facts.tasks[at]
			states[task.State]++
			assert.Contains(t, []string{actorThomas, actorAmelie, actorSofia}, task.Actor, task.ID)
			assertTaskHolds(t, source, facts, task)
		}

		assert.Equal(t, map[fleetui.TaskState]int{
			fleetui.TaskStateRunning:      2,
			fleetui.TaskStateFailed:       1,
			fleetui.TaskStateSucceeded:    11,
			fleetui.TaskStatePartlyFailed: 1,
		}, states, "2 running, 1 failed, 12 finished of which one partly failed")
	})

	t.Run("rollouts", func(t *testing.T) {
		require.Len(t, facts.rollouts, 2)

		states := map[fleetui.RolloutState]int{}
		for at := range facts.rollouts {
			states[facts.rollouts[at].State]++
		}
		assert.Equal(t, map[fleetui.RolloutState]int{
			fleetui.RolloutStatePaused:   1,
			fleetui.RolloutStateComplete: 1,
		}, states)

		assertPausedRollout(t, source, facts)
		assertCompleteRollout(t, source, facts)
	})

	t.Run("audit trail", func(t *testing.T) {
		entries, err := source.AuditEntries(t.Context())
		require.NoError(t, err)
		require.NotEmpty(t, entries)

		taskIDs := map[string]struct{}{}
		for at := range facts.tasks {
			taskIDs[facts.tasks[at].ID] = struct{}{}
		}

		written := 0
		for at := range entries {
			entry := &entries[at]
			assert.False(t, entry.At.After(facts.now), entry.ID)
			assert.LessOrEqual(t, facts.now.Sub(entry.At), historyRetention, entry.ID)
			if at > 0 {
				assert.False(t, entry.At.After(entries[at-1].At), "%s: newest first", entry.ID)
			}
			if entry.TaskID != "" {
				assert.Contains(t, taskIDs, entry.TaskID, entry.ID)
				written++
			}
		}
		assert.Equal(t, len(facts.tasks), written, "every task wrote one entry")

		for at := range auditRows {
			if auditRows[at].device != "" {
				facts.view(t, auditRows[at].device)
			}
		}
	})

	t.Run("templates", func(t *testing.T) {
		templates, err := source.Templates(t.Context())
		require.NoError(t, err)
		require.Len(t, templates, 2)

		fromTemplate := 0
		for at := range facts.devices {
			if facts.devices[at].Config.Template != "" {
				fromTemplate++
			}
		}

		counted := 0
		for at := range templates {
			assert.NotEmpty(t, templates[at].Groups, templates[at].Name)
			counted += templates[at].Devices
		}
		assert.Equal(t, fromTemplate, counted)
	})
}

// assertAlarmFitsDevice checks that an alarm reports a condition its device
// is in.
func assertAlarmFitsDevice(t *testing.T, alarm *fleetui.Alarm, view *fleetui.DeviceView) {
	t.Helper()

	switch alarm.Kind {
	case alarmComponentDown:
		assert.Equal(t, fleetui.DisplayStateDown, view.State, alarm.ID)
		assert.Equal(t, fleetui.AlarmSeverityCritical, alarm.Severity, alarm.ID)
	case alarmComponentDegraded:
		assert.Equal(t, fleetui.DisplayStateDegraded, view.State, alarm.ID)
	case alarmConfigDiverged:
		assert.Equal(t, fleetui.DisplayStateDiverged, view.State, alarm.ID)
	case alarmConfigRejected:
		assert.Equal(t, fleetui.ConfigStateRejected, view.Device.Config.State, alarm.ID)
	case alarmSoftwareBehind:
		assert.Equal(t, releaseTable[releaseOldest].version, view.Device.Inventory.SoftwareVersion, alarm.ID)
	case alarmRelayUngranted:
		assert.False(t, view.Device.Connection.RelayGranted, alarm.ID)
	default:
		t.Errorf("%s: the table holds an alarm kind %q this test does not know", alarm.ID, alarm.Kind)
	}
}

// assertTaskHolds checks one task against its detail and against the estate.
func assertTaskHolds(t *testing.T, source *estate, facts *estateFacts, task *fleetui.Task) {
	t.Helper()

	for _, name := range task.Devices {
		facts.view(t, name)
	}
	if !task.State.Active() {
		assert.True(t, task.Ended.After(task.Started), task.ID)
		assert.LessOrEqual(t, facts.now.Sub(task.Ended), day, "%s ended in the last day", task.ID)
	}

	detail, err := source.Task(t.Context(), task.ID)
	require.NoError(t, err)
	require.Len(t, detail.Results, task.Finished, task.ID)
	require.Len(t, detail.AuditIDs, 1, task.ID)

	failed := 0
	for at := range detail.Results {
		assert.Contains(t, task.Devices, detail.Results[at].Device, task.ID)
		if detail.Results[at].State == fleetui.TaskStateFailed {
			failed++
		}
	}
	assert.Equal(t, task.Failed, failed, task.ID)

	if task.State == fleetui.TaskStatePartlyFailed {
		assert.Positive(t, failed, task.ID)
		assert.Less(t, failed, len(detail.Results), task.ID)
	}
}

// assertPausedRollout checks that the paused rollout stopped at its second
// batch on the one device whose configuration is rejected.
func assertPausedRollout(t *testing.T, source *estate, facts *estateFacts) {
	t.Helper()

	detail, err := source.Rollout(t.Context(), rolloutEdge)
	require.NoError(t, err)
	require.Equal(t, fleetui.RolloutStatePaused, detail.Rollout.State)
	require.Equal(t, 1, detail.Rollout.BatchesDone)
	require.Len(t, detail.Batches, detail.Rollout.BatchesTotal)
	assert.Equal(t, fleetui.RolloutStatePaused, detail.Batches[1].State, "paused at the second batch")

	rejected := []string{}
	for at := range facts.devices {
		if facts.devices[at].Config.State == fleetui.ConfigStateRejected {
			rejected = append(rejected, facts.devices[at].Name)
		}
	}
	require.Len(t, rejected, 1)
	assert.Equal(t, rejected, detail.PauseDevices)
	assert.Contains(t, detail.Rollout.PauseReason, rejected[0])

	require.Len(t, detail.Devices, detail.Rollout.DevicesTotal)
	for at := range detail.Devices {
		device := &detail.Devices[at]
		facts.view(t, device.Device)
		if device.Device == rejected[0] {
			assert.Equal(t, 2, device.Batch)
			assert.True(t, device.Answered)
			assert.Equal(t, facts.view(t, device.Device).Device.Config.LastFetchResult, device.Answer)
		}
	}
}

// assertCompleteRollout checks that every batch and device of the complete
// rollout finished.
func assertCompleteRollout(t *testing.T, source *estate, facts *estateFacts) {
	t.Helper()

	detail, err := source.Rollout(t.Context(), rolloutCore)
	require.NoError(t, err)
	require.Equal(t, fleetui.RolloutStateComplete, detail.Rollout.State)
	assert.Equal(t, detail.Rollout.BatchesTotal, detail.Rollout.BatchesDone)
	assert.Equal(t, detail.Rollout.DevicesTotal, detail.Rollout.DevicesDone)

	for at := range detail.Batches {
		assert.Equal(t, fleetui.RolloutStateComplete, detail.Batches[at].State)
	}
	for at := range detail.Devices {
		facts.view(t, detail.Devices[at].Device)
		assert.True(t, detail.Devices[at].Answered, detail.Devices[at].Device)
	}
}

// TestSampleConfigDivergesByThreeChanges reads both sides of the diverged
// device's configuration.
//
// VALIDATES: the hub's copy and the device's copy differ by the three local
// changes the divergence states, every device's versions are distinct and
// newest first, and a configuration text closes every block it opens.
// PREVENTS: a divergence banner that announces changes its diff does not
// show.
func TestSampleConfigDivergesByThreeChanges(t *testing.T) {
	source := newTestEstate(t)
	ctx := t.Context()

	divergence, err := source.Divergence(ctx, deviceRsFra01)
	require.NoError(t, err)
	assert.Equal(t, 3, divergence.Changes)
	assert.NotEqual(t, divergence.HubVersion, divergence.DeviceVersion)

	for _, change := range []string{"receive-hold-time 30;", "maximum 500000;", "peer lab-probe {"} {
		assert.Contains(t, divergence.DeviceText, change)
		assert.NotContains(t, divergence.HubText, change)
	}

	sampled := devices()
	for at := range sampled {
		name := sampled[at].Name

		versions, listErr := source.ConfigVersions(ctx, name)
		require.NoError(t, listErr)
		require.NotEmpty(t, versions, name)
		assert.Equal(t, sampled[at].Config.HubVersion, versions[0].Version, name)

		seen := map[string]struct{}{}
		for index := range versions {
			seen[versions[index].Version] = struct{}{}
			if index > 0 {
				assert.True(t, versions[index].At.Before(versions[index-1].At), "%s: newest first", name)
			}

			text, textErr := source.ConfigText(ctx, name, versions[index].Version)
			require.NoError(t, textErr)
			assert.Equal(t, strings.Count(text, "{"), strings.Count(text, "}"), name)
		}
		assert.Len(t, seen, len(versions), "%s: every version has its own identifier", name)
	}
}

// TestSampleHistoryIsTheSameAtEveryStart reads a week of one device's
// history from two sources made at different moments.
//
// VALIDATES: the values are the same at every start, the series is bounded
// and in time order, it has a gap where the device was offline, and its
// highest traffic is the spike two days back.
// PREVENTS: charts that change from one review to the next, and a history
// request whose cost grows with the period asked for.
func TestSampleHistoryIsTheSameAtEveryStart(t *testing.T) {
	later := testStart.Add(37*time.Hour + 11*time.Minute)

	first := newTestEstate(t)
	second := newEstate(func() time.Time { return later })
	second.Hold()
	t.Cleanup(second.Close)

	week := func(now time.Time) fleetui.TimeSpan {
		return fleetui.TimeSpan{From: now.Add(-historyRetention), To: now}
	}

	ctx := t.Context()
	series, err := first.DeviceSeries(ctx, deviceEdgePar02, fleetui.MetricTrafficIn, week(testStart))
	require.NoError(t, err)
	again, err := second.DeviceSeries(ctx, deviceEdgePar02, fleetui.MetricTrafficIn, week(later))
	require.NoError(t, err)

	require.NotEmpty(t, series.Points)
	require.LessOrEqual(t, len(series.Points), historyPointsMax)
	require.Len(t, again.Points, len(series.Points))

	gaps := 0
	highest := series.Points[0]
	for at := range series.Points {
		point := series.Points[at]
		assert.InDelta(t, point.Value, again.Points[at].Value, 0, "the same value at every start")
		assert.Equal(t, point.At.Sub(testStart), again.Points[at].At.Sub(later))

		if point.Value > highest.Value {
			highest = point
		}
		if at == 0 {
			continue
		}

		step := point.At.Sub(series.Points[at-1].At)
		require.Positive(t, step, "in time order")
		if step > series.Interval {
			gaps++
		}
	}

	assert.Equal(t, 1, gaps, "one gap, where the device was offline")
	assert.LessOrEqual(t, (testStart.Sub(highest.At) - spikeAge).Abs(), time.Hour, "the highest traffic is the spike")

	connected, err := first.EstateSeries(ctx, []string{deviceEdgePar02, deviceCoreLon01},
		fleetui.MetricDevicesConnected, week(testStart))
	require.NoError(t, err)
	require.NotEmpty(t, connected.Points)
	assert.InDelta(t, 2, connected.Points[len(connected.Points)-1].Value, 0, "both devices report now")
}
