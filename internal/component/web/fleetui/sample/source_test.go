package sample

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/internal/component/web/fleetui"
)

// TestSampleNewRegistersNothing builds a sample source in a test binary that
// links this package and not its register sub-package.
//
// VALIDATES: importing the sample package and calling New leave the build's
// source slot empty: every registered fleet route still answers that it is
// not enabled, which it reads from the slot.
// PREVENTS: a tool that links the sample to record the pages, le, making a
// wall-clock sample estate the source of its process.
func TestSampleNewRegistersNothing(t *testing.T) {
	source := New(fixedClock)
	t.Cleanup(source.Close)

	fleetRoutes := 0
	for _, route := range web.RegisteredWebRoutes() {
		_, path, _ := strings.Cut(route.Pattern, " ")
		if !strings.HasPrefix(path, "/fleet/") {
			continue
		}
		fleetRoutes++
		require.NotNil(t, route.Enabled, route.Pattern)
		assert.False(t, route.Enabled(), "%s is enabled with no source registered", route.Pattern)
	}
	require.Positive(t, fleetRoutes, "the fleet package registers its routes, so the check read some")
}

// TestSampleAnswersEveryQuestion asks the sample source every question of
// the data source interface, for a device that can answer it.
//
// VALIDATES: every method answers, without an error, something a page can
// show.
// PREVENTS: a page of the prototype that opens on a refusal because the
// sample estate records nothing for it.
func TestSampleAnswersEveryQuestion(t *testing.T) {
	source := newTestEstate(t)
	ctx := t.Context()

	const device = deviceEdgeLon01
	week := fleetui.TimeSpan{From: testStart.Add(-historyRetention), To: testStart}

	questions := []struct {
		name string
		ask  func() (answered bool, err error)
	}{
		{"Now", func() (bool, error) {
			now, err := source.Now(ctx)
			return !now.IsZero(), err
		}},
		{"Estate", func() (bool, error) {
			named, err := source.Estate(ctx)
			return named.Name == estateName, err
		}},
		{"Devices", func() (bool, error) {
			found, err := source.Devices(ctx)
			return len(found) > 0, err
		}},
		{"Device", func() (bool, error) {
			found, err := source.Device(ctx, device)
			return found.Name == device, err
		}},
		{"TreeViews", func() (bool, error) {
			views, err := source.TreeViews(ctx)
			return len(views) > 0, err
		}},
		{"SavedFilters", func() (bool, error) {
			filters, err := source.SavedFilters(ctx, actorThomas)
			return len(filters) > 0, err
		}},
		{"DeviceSeries", func() (bool, error) {
			series, err := source.DeviceSeries(ctx, device, fleetui.MetricCPU, week)
			return len(series.Points) > 0, err
		}},
		{"EstateSeries", func() (bool, error) {
			series, err := source.EstateSeries(ctx, []string{device, deviceCoreLon01}, fleetui.MetricTrafficIn, week)
			return len(series.Points) > 0, err
		}},
		{"HistoryUsage", func() (bool, error) {
			usage, err := source.HistoryUsage(ctx)
			return len(usage.Largest) == fleetui.HistoryDevicesMax && usage.SizeBytes < usage.CapBytes, err
		}},
		{"Alarms", func() (bool, error) {
			alarms, err := source.Alarms(ctx)
			return len(alarms) > 0, err
		}},
		{"AlarmHistory", func() (bool, error) {
			history, err := source.AlarmHistory(ctx, deviceEdgeLon04)
			return len(history) > 0, err
		}},
		{"Tasks", func() (bool, error) {
			tasks, err := source.Tasks(ctx)
			return len(tasks) > 0, err
		}},
		{"Task", func() (bool, error) {
			detail, err := source.Task(ctx, taskRejectedBatch)
			return len(detail.Results) > 0, err
		}},
		{"DeviceConfig", func() (bool, error) {
			config, err := source.DeviceConfig(ctx, device)
			return config.Text != "" && config.Version != "", err
		}},
		{"ConfigVersions", func() (bool, error) {
			versions, err := source.ConfigVersions(ctx, device)
			return len(versions) > 0, err
		}},
		{"ConfigText", func() (bool, error) {
			config, err := source.DeviceConfig(ctx, device)
			if err != nil {
				return false, err
			}
			text, err := source.ConfigText(ctx, device, config.Version)
			return text == config.Text, err
		}},
		{"Divergence", func() (bool, error) {
			divergence, err := source.Divergence(ctx, deviceRsFra01)
			return divergence.HubText != divergence.DeviceText, err
		}},
		{"Rollouts", func() (bool, error) {
			rollouts, err := source.Rollouts(ctx)
			return len(rollouts) > 0, err
		}},
		{"Rollout", func() (bool, error) {
			detail, err := source.Rollout(ctx, rolloutEdge)
			return len(detail.Devices) > 0, err
		}},
		{"AuditEntries", func() (bool, error) {
			entries, err := source.AuditEntries(ctx)
			return len(entries) > 0, err
		}},
		{"Templates", func() (bool, error) {
			templates, err := source.Templates(ctx)
			return len(templates) > 0, err
		}},
		{"AccessProfiles", func() (bool, error) {
			profiles, err := source.AccessProfiles(ctx)
			return len(profiles) > 0, err
		}},
		{"Settings", func() (bool, error) {
			found, err := source.Settings(ctx)
			return found.OfflineAlarmDelay > 0 && found.FinishedTasksKept > 0, err
		}},
		{"DeviceSystem", func() (bool, error) {
			system, err := source.DeviceSystem(ctx, device)
			return len(system.Hardware) > 0 && system.Resources.MemoryTotalBytes > 0, err
		}},
		{"DeviceInterfaces", func() (bool, error) {
			interfaces, err := source.DeviceInterfaces(ctx, device)
			return len(interfaces) > 0, err
		}},
		{"DevicePeers", func() (bool, error) {
			peers, err := source.DevicePeers(ctx, device)
			return len(peers) > 0, err
		}},
		{"DeviceFamilies", func() (bool, error) {
			families, err := source.DeviceFamilies(ctx, device)
			return len(families) > 0, err
		}},
		{"DeviceAdjacencies", func() (bool, error) {
			protocols, err := source.DeviceAdjacencies(ctx, device)
			return len(protocols) > 0, err
		}},
		{"DeviceLogs", func() (bool, error) {
			lines, err := source.DeviceLogs(ctx, device)
			return len(lines) > 0, err
		}},
		{"Console", func() (bool, error) {
			answer, err := source.Console(ctx, device, actorThomas, "show version")
			return answer.Output != "", err
		}},
		{"Subscribe", func() (bool, error) {
			subscription, err := source.Subscribe(ctx)
			if err != nil {
				return false, err
			}
			subscription.Close()
			return true, nil
		}},
	}

	for _, question := range questions {
		t.Run(question.name, func(t *testing.T) {
			answered, err := question.ask()
			require.NoError(t, err)
			assert.True(t, answered, "the answer holds nothing a page can show")
		})
	}
}

// TestSampleDeviceViewsAgreeWithTheFacts compares what a faulty device
// answers through the relay with its last sample and its health report.
//
// VALIDATES: the peers and interfaces a device answers are the ones its last
// sample counts, and a fault shows in both.
// PREVENTS: a device page whose Routing tab and dashboard disagree.
func TestSampleDeviceViewsAgreeWithTheFacts(t *testing.T) {
	source := newTestEstate(t)
	ctx := t.Context()

	for _, name := range []string{deviceEdgeLon01, deviceEdgeLon04, deviceEdgeAms02, deviceCoreFra02, deviceCpeAms03} {
		device, err := source.Device(ctx, name)
		require.NoError(t, err)

		peers, err := source.DevicePeers(ctx, name)
		require.NoError(t, err)
		established := 0
		for at := range peers {
			if peers[at].Established {
				established++
			} else {
				assert.NotEmpty(t, peers[at].LastError, "%s: %s", name, peers[at].Name)
			}
		}
		assert.Equal(t, device.Latest.PeersEstablished, established, name)
		assert.Equal(t, device.Latest.PeersDown, len(peers)-established, name)

		interfaces, err := source.DeviceInterfaces(ctx, name)
		require.NoError(t, err)
		up := 0
		for at := range interfaces {
			if interfaces[at].LinkUp {
				up++
			}
			for _, address := range interfaces[at].Addresses {
				assert.True(t, documented(address.Addr()), "%s: %s", name, interfaces[at].Name)
			}
		}
		assert.Equal(t, device.Latest.InterfacesUp, up, name)
	}

	down, err := source.Device(ctx, deviceEdgeAms02)
	require.NoError(t, err)
	assert.Zero(t, down.Latest.PeersEstablished, "the device whose BGP is down has no established peer")
}

// TestSampleDeviceViewsRefuse asks the devices that cannot answer a view.
//
// VALIDATES: an offline device answers the offline failure, a device that
// grants no relay profile answers the relay refusal with the line that
// grants it, the device on the oldest release answers not supported for the
// BGP summary only, and the console refuses an unknown command with the list
// of those it answers.
// PREVENTS: a prototype in which the failure states of a device tab cannot
// be reached.
func TestSampleDeviceViewsRefuse(t *testing.T) {
	source := newTestEstate(t)
	ctx := t.Context()

	_, err := source.DeviceInterfaces(ctx, deviceCpe0127)
	require.Error(t, err)
	assert.Equal(t, fleetui.FailureOffline, fleetui.FailureOf(err))

	_, err = source.DevicePeers(ctx, deviceCpeFra03)
	require.Error(t, err)
	assert.Equal(t, fleetui.FailureRelayRefused, fleetui.FailureOf(err))
	assert.Contains(t, err.Error(), "set fleet relay profile")

	_, err = source.DeviceFamilies(ctx, deviceCpe0114)
	require.Error(t, err)
	assert.Equal(t, fleetui.FailureUnsupported, fleetui.FailureOf(err))

	_, err = source.DevicePeers(ctx, deviceCpe0114)
	require.NoError(t, err, "the other views of that device answer")

	_, err = source.DeviceLogs(ctx, "no-such-device")
	require.Error(t, err)
	assert.Equal(t, fleetui.FailureNotFound, fleetui.FailureOf(err))

	_, err = source.Console(ctx, deviceEdgeLon01, actorThomas, "show everything")
	require.Error(t, err)
	assert.Equal(t, fleetui.FailureFailed, fleetui.FailureOf(err))
	for _, name := range consoleNames() {
		assert.Contains(t, err.Error(), name)
	}

	for _, name := range consoleNames() {
		answer, answerErr := source.Console(ctx, deviceEdgeLon01, actorThomas, "  "+name+" ")
		require.NoError(t, answerErr, name)
		assert.NotEmpty(t, answer.Output, name)
	}
}

// TestSampleClockAdvances reads the same device from a source whose clock
// has moved on since it was made.
//
// VALIDATES: every time of the estate is measured from the moment the source
// was made, and Now advances with the clock, so an age grows as time passes.
// PREVENTS: a "last seen" that is anchored to the build date and reads as
// years ago, or one that never grows while the review build stays open.
func TestSampleClockAdvances(t *testing.T) {
	now := testStart
	source := newEstate(func() time.Time { return now })
	source.Hold()
	t.Cleanup(source.Close)

	ctx := t.Context()
	before, err := source.Device(ctx, deviceCpe0127)
	require.NoError(t, err)

	now = testStart.Add(time.Hour)

	clock, err := source.Now(ctx)
	require.NoError(t, err)
	assert.Equal(t, testStart.Add(time.Hour), clock)

	after, err := source.Device(ctx, deviceCpe0127)
	require.NoError(t, err)
	assert.Equal(t, before.Connection.LastSeen, after.Connection.LastSeen, "a fact does not move with the clock")
	assert.Equal(t, 6*day+3*time.Hour, clock.Sub(after.Connection.LastSeen))

	connected, err := source.Device(ctx, deviceEdgeLon01)
	require.NoError(t, err)
	assert.Equal(t, clock, connected.Latest.At, "a connected device's last sample follows the clock")
}

// TestSampleAnswersEveryConsoleSuggestion runs each command the console tab
// suggests on a healthy sample device.
//
// VALIDATES: every suggestion of fleetui.ConsoleSuggestions is a command the
// sample answers with output.
// PREVENTS: a console tab whose suggested commands each answer "unknown
// command" in the prototype.
func TestSampleAnswersEveryConsoleSuggestion(t *testing.T) {
	source := newTestEstate(t)
	ctx := t.Context()

	for _, command := range fleetui.ConsoleSuggestions() {
		answer, err := source.Console(ctx, deviceEdgeLon01, actorThomas, command)
		require.NoError(t, err, command)
		assert.NotEmpty(t, answer.Output, command)
	}
}
