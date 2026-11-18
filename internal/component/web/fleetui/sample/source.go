// Design: docs/architecture/fleet-interface.md -- the sample estate of the review build
// Related: register.go -- the registration that makes this the build's data source
//
// Package sample answers the fleet pages from a fixed estate, so the interface
// can be opened and judged before any real estate exists. Only a build made
// with the ze_fleet_ui_sample tag links it, and no shipped build sets that
// tag: sample data in a shipped build would be shown as real.
package sample

import (
	"context"
	"errors"
	"time"

	"github.com/ze-software/ze/internal/component/web/fleetui"
	"github.com/ze-software/ze/internal/core/opererr"
)

// estateName is the name of the sample hub, and so of the sample estate.
const estateName = "hub-lon-01"

// The label keys of the sample estate. Every sample device takes its labels
// from this set, so a key is spelled once and a tree level or a filter that
// names one matches every device carrying it.
const (
	labelSite = "site"
	labelRole = "role"
)

// errNotInSample is the cause of every question the sample estate does not
// answer yet. Each method that returns it is replaced by a sample answer as
// the estate is filled, and the error leaves with the last of them.
var errNotInSample = errors.New("not available in this sample yet")

// notInSample is the refusal of a question the sample estate does not answer
// yet. what names what was asked for, such as "The sample history".
func notInSample(what string) error {
	return opererr.Wrap(errNotInSample, what+" is not available in this sample yet.",
		"The sample estate answers this page from recorded data, and none is recorded for it, so the page has nothing to show.",
		"open /fleet/")
}

// estate is the sample data source. Its fixed answers hold no state, and its
// feed is safe for concurrent use, so the estate is.
type estate struct {
	// feed carries the scripted events of the sample estate to the open
	// pages. Nothing publishes into it yet.
	feed fleetui.Feed
}

// Now answers the clock of the sample hub, which is the clock of the machine
// the review build runs on.
func (*estate) Now(context.Context) (time.Time, error) {
	return time.Now(), nil
}

// Estate answers the name of the sample estate.
func (*estate) Estate(context.Context) (fleetui.Estate, error) {
	return fleetui.Estate{Name: estateName}, nil
}

// Devices answers the sample devices. Each call builds its own, so a caller
// that changes its answer changes no other caller's.
func (*estate) Devices(context.Context) ([]fleetui.Device, error) {
	return devices(), nil
}

// Device answers one sample device by name.
func (*estate) Device(_ context.Context, name string) (fleetui.Device, error) {
	found := devices()
	for index := range found {
		if found[index].Name == name {
			return found[index], nil
		}
	}

	return fleetui.Device{}, fleetui.DeviceUnknownError(name)
}

// Subscribe opens a subscription to the events of the sample estate.
func (source *estate) Subscribe(context.Context) (*fleetui.Subscription, error) {
	return source.feed.Subscribe()
}

func (*estate) TreeViews(context.Context) ([]fleetui.TreeView, error) {
	return nil, notInSample("The declared tree views")
}

func (*estate) SavedFilters(context.Context, string) ([]fleetui.SavedFilter, error) {
	return nil, notInSample("The saved filters")
}

func (*estate) DeviceSeries(context.Context, string, fleetui.Metric, fleetui.TimeSpan) (fleetui.Series, error) {
	return fleetui.Series{}, notInSample("The history of a device")
}

func (*estate) EstateSeries(context.Context, []string, fleetui.Metric, fleetui.TimeSpan) (fleetui.Series, error) {
	return fleetui.Series{}, notInSample("The history of the estate")
}

func (*estate) HistoryUsage(context.Context) (fleetui.HistoryUsage, error) {
	return fleetui.HistoryUsage{}, notInSample("The history usage")
}

func (*estate) Alarms(context.Context) ([]fleetui.Alarm, error) {
	return nil, notInSample("The alarm list")
}

func (*estate) AlarmHistory(context.Context, string) ([]fleetui.AlarmEvent, error) {
	return nil, notInSample("The alarm history of a device")
}

func (*estate) Tasks(context.Context) ([]fleetui.Task, error) {
	return nil, notInSample("The task list")
}

func (*estate) Task(context.Context, string) (fleetui.TaskDetail, error) {
	return fleetui.TaskDetail{}, notInSample("The detail of a task")
}

func (*estate) DeviceConfig(context.Context, string) (fleetui.DeviceConfig, error) {
	return fleetui.DeviceConfig{}, notInSample("The configuration of a device")
}

func (*estate) ConfigVersions(context.Context, string) ([]fleetui.ConfigVersion, error) {
	return nil, notInSample("The configuration history of a device")
}

func (*estate) ConfigText(context.Context, string, string) (string, error) {
	return "", notInSample("The text of a configuration version")
}

func (*estate) Divergence(context.Context, string) (fleetui.Divergence, error) {
	return fleetui.Divergence{}, notInSample("The divergence of a device")
}

func (*estate) Rollouts(context.Context) ([]fleetui.Rollout, error) {
	return nil, notInSample("The rollout list")
}

func (*estate) Rollout(context.Context, string) (fleetui.RolloutDetail, error) {
	return fleetui.RolloutDetail{}, notInSample("The detail of a rollout")
}

func (*estate) AuditEntries(context.Context) ([]fleetui.AuditEntry, error) {
	return nil, notInSample("The audit trail")
}

func (*estate) Templates(context.Context) ([]fleetui.Template, error) {
	return nil, notInSample("The template list")
}

func (*estate) AccessProfiles(context.Context) ([]fleetui.AccessProfile, error) {
	return nil, notInSample("The access table")
}

func (*estate) Settings(context.Context) (fleetui.Settings, error) {
	return fleetui.Settings{}, notInSample("The fleet settings")
}

func (*estate) DeviceSystem(context.Context, string) (fleetui.DeviceSystem, error) {
	return fleetui.DeviceSystem{}, notInSample("The system view of a device")
}

func (*estate) DeviceInterfaces(context.Context, string) ([]fleetui.Interface, error) {
	return nil, notInSample("The interfaces of a device")
}

func (*estate) DevicePeers(context.Context, string) ([]fleetui.Peer, error) {
	return nil, notInSample("The BGP peers of a device")
}

func (*estate) DeviceFamilies(context.Context, string) ([]fleetui.FamilySummary, error) {
	return nil, notInSample("The BGP summary of a device")
}

func (*estate) DeviceAdjacencies(context.Context, string) ([]fleetui.ProtocolAdjacencies, error) {
	return nil, notInSample("The routing adjacencies of a device")
}

func (*estate) DeviceLogs(context.Context, string) ([]fleetui.LogLine, error) {
	return nil, notInSample("The log of a device")
}

func (*estate) Console(context.Context, string, string, string) (fleetui.ConsoleAnswer, error) {
	return fleetui.ConsoleAnswer{}, notInSample("The console of a device")
}

// devices builds the sample devices. Between them they hold every display
// state: healthy, degraded, down, diverged, offline and unknown.
func devices() []fleetui.Device {
	healthy := []fleetui.HealthComponent{{Name: "bgp", State: fleetui.HealthStateHealthy}}

	return []fleetui.Device{
		{
			Name:           "edge-lon-01",
			Labels:         []fleetui.Label{{Key: labelSite, Value: "lon"}, {Key: labelRole, Value: "edge"}},
			Connection:     fleetui.Connection{Connected: true, EverConnected: true},
			Config:         fleetui.ConfigFacts{State: fleetui.ConfigStateInSync},
			HealthReported: true,
			Health:         healthy,
		},
		{
			Name:           "edge-lon-04",
			Labels:         []fleetui.Label{{Key: labelSite, Value: "lon"}, {Key: labelRole, Value: "edge"}},
			Connection:     fleetui.Connection{Connected: true, EverConnected: true},
			Config:         fleetui.ConfigFacts{State: fleetui.ConfigStateInSync},
			HealthReported: true,
			Health:         []fleetui.HealthComponent{{Name: "disk", State: fleetui.HealthStateDegraded}},
		},
		{
			Name:           "core-ams-01",
			Labels:         []fleetui.Label{{Key: labelSite, Value: "ams"}, {Key: labelRole, Value: "core"}},
			Connection:     fleetui.Connection{Connected: true, EverConnected: true},
			Config:         fleetui.ConfigFacts{State: fleetui.ConfigStateInSync},
			HealthReported: true,
			Health:         []fleetui.HealthComponent{{Name: "bgp", State: fleetui.HealthStateDown}},
		},
		{
			Name:           "rs-fra-01",
			Labels:         []fleetui.Label{{Key: labelSite, Value: "fra"}, {Key: labelRole, Value: "route-server"}},
			Connection:     fleetui.Connection{Connected: true, EverConnected: true},
			Config:         fleetui.ConfigFacts{State: fleetui.ConfigStateDiverged},
			HealthReported: true,
			Health:         healthy,
		},
		{
			Name:           "cpe-par-07",
			Labels:         []fleetui.Label{{Key: labelSite, Value: "par"}, {Key: labelRole, Value: "customer-premises"}},
			Connection:     fleetui.Connection{EverConnected: true},
			Config:         fleetui.ConfigFacts{State: fleetui.ConfigStateInSync},
			HealthReported: true,
			Health:         healthy,
		},
		{
			// No site label: the tree needs a device outside every site. No
			// health report either, so its state is unknown.
			Name:       "cpe-0113",
			Labels:     []fleetui.Label{{Key: labelRole, Value: "customer-premises"}},
			Connection: fleetui.Connection{Connected: true, EverConnected: true},
			Config:     fleetui.ConfigFacts{State: fleetui.ConfigStateNeverFetched},
		},
	}
}
