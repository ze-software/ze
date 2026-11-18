package fleetui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
)

// testNow is the hub's clock in every test of this package. No test reads
// the wall clock.
var testNow = time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

// The names of the devices of the test estate, in the order testDevices
// answers them.
const (
	edgeLon01 = "edge-lon-01"
	edgeLon02 = "edge-lon-02"
	coreLon01 = "core-lon-01"
	coreAms01 = "core-ams-01"
	rsAms01   = "rs-ams-01"
	cpe0113   = "cpe-0113"
	cpe0200   = "cpe-0200"
)

// testLabels builds labels from "key=value" pairs.
func testLabels(pairs ...string) []Label {
	labels := make([]Label, 0, len(pairs))
	for _, pair := range pairs {
		key, value, _ := strings.Cut(pair, "=")
		labels = append(labels, Label{Key: key, Value: value})
	}

	return labels
}

// testConnected builds a connected device that is in sync, reports a healthy
// component, runs version on a daemon started a day ago, and reported a
// sample with the given processor and memory use.
func testConnected(name, version string, cpu, memory float64, labels ...string) Device {
	return Device{
		Name:           name,
		Labels:         testLabels(labels...),
		Connection:     Connection{Connected: true, Since: testNow.Add(-time.Hour), EverConnected: true},
		Config:         ConfigFacts{State: ConfigStateInSync},
		HealthReported: true,
		Health:         []HealthComponent{{Name: "bgp", State: HealthStateHealthy}},
		Inventory: Inventory{
			Reported: true, Model: "ze-1000", SoftwareVersion: version, StartedAt: testNow.Add(-24 * time.Hour),
		},
		Latest: LatestSample{Reported: true, At: testNow, CPUPercent: cpu, MemoryPercent: memory},
	}
}

// testDevices answers the test estate: seven devices that between them hold
// every display state, every configuration state, three software versions
// and a device that reported none, two sites and two devices with no site.
//
//	edge-lon-01  site=lon role=edge          healthy   1.4.2  in sync   cpu 71  up 41 days
//	edge-lon-02  site=lon role=edge          healthy   1.4.2  pending   cpu 12
//	core-lon-01  site=lon role=core          down      1.4.1  in sync   cpu 95
//	core-ams-01  site=ams role=core          offline   1.4.1  in sync   no sample, seen 6 days ago
//	rs-ams-01    site=ams role=route-server  diverged  1.3.9  diverged  cpu 5
//	cpe-0113     role=cpe                    degraded  1.4.2  rejected  cpu 30
//	cpe-0200     role=cpe                    unknown   none   never     no sample
//
// Each call builds its own devices.
func testDevices() []Device {
	first := testConnected(edgeLon01, "1.4.2", 71, 40, "site=lon", "role=edge")
	first.Group = "edge"
	first.Inventory.StartedAt = testNow.Add(-41 * 24 * time.Hour)
	first.Latest.PeersEstablished = 10
	first.Latest.PeersDown = 1
	first.Latest.PeersAdminDown = 1

	second := testConnected(edgeLon02, "1.4.2", 12, 91, "site=lon", "role=edge")
	second.Group = "edge"
	second.Config.State = ConfigStatePending

	down := testConnected(coreLon01, "1.4.1", 95, 50, "site=lon", "role=core")
	down.Health = []HealthComponent{
		{Name: "bgp", State: HealthStateDown},
		{Name: "disk", State: HealthStateDegraded},
	}

	offline := testConnected(coreAms01, "1.4.1", 0, 0, "site=ams", "role=core")
	offline.Connection = Connection{EverConnected: true, LastSeen: testNow.Add(-6 * 24 * time.Hour)}
	offline.Latest = LatestSample{}

	diverged := testConnected(rsAms01, "1.3.9", 5, 5, "site=ams", "role=route-server")
	diverged.Config.State = ConfigStateDiverged

	degraded := testConnected(cpe0113, "1.4.2", 30, 30, "role=cpe")
	degraded.Config.State = ConfigStateRejected
	degraded.Health = []HealthComponent{{Name: "disk", State: HealthStateDegraded}}

	unknown := Device{
		Name:       cpe0200,
		Labels:     testLabels("role=cpe"),
		Connection: Connection{Connected: true, Since: testNow.Add(-time.Minute), EverConnected: true},
		Config:     ConfigFacts{State: ConfigStateNeverFetched},
	}

	return []Device{first, second, down, offline, diverged, degraded, unknown}
}

// testAlarms answers the open alarms of the test estate: two on core-lon-01
// and one, the oldest, on core-ams-01.
func testAlarms() []Alarm {
	return []Alarm{
		{ID: "a1", Kind: "component-down", Severity: AlarmSeverityCritical, Device: coreLon01, Since: testNow.Add(-2 * time.Hour)},
		{ID: "a2", Kind: "component-degraded", Severity: AlarmSeverityWarning, Device: coreLon01, Since: testNow.Add(-3 * time.Hour)},
		{ID: "a3", Kind: "device-offline", Severity: AlarmSeverityCritical, Device: coreAms01, Since: testNow.Add(-6 * 24 * time.Hour)},
	}
}

// testTasks answers the tasks of the test estate: one running over the two
// edge devices, and one that succeeded on core-lon-01.
func testTasks() []Task {
	return []Task{
		{ID: "t1", Kind: "config-push", State: TaskStateRunning, Devices: []string{edgeLon01, edgeLon02}},
		{ID: "t2", Kind: "diagnostic", State: TaskStateSucceeded, Devices: []string{coreLon01}},
	}
}

// testViews answers the views of the test estate as of testNow.
func testViews() []DeviceView {
	return DeriveDevices(testDevices(), testAlarms(), testTasks(), testNow)
}

// namesOf answers the device names of views, in their order.
func namesOf(views []DeviceView) []string {
	names := make([]string, 0, len(views))
	for index := range views {
		names = append(names, views[index].Device.Name)
	}

	return names
}

// requireRefusal asserts that err is an operator error wrapping cause, and
// answers it so a test can read its three parts.
func requireRefusal(t *testing.T, err, cause error) *opererr.Error {
	t.Helper()

	require.Error(t, err)
	require.ErrorIs(t, err, cause)

	var refusal *opererr.Error
	require.True(t, errors.As(err, &refusal), "the refusal must be an operator error, got %T", err)

	return refusal
}
