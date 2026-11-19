// Design: docs/architecture/fleet-interface.md -- the sample data source of the review build
// Detail: estate.go -- the devices of the sample estate, as tables
// Detail: history.go -- the week of samples, generated per request
// Detail: activity.go -- the alarms and the tasks
// Detail: rollout.go -- the rollouts and the audit trail
// Detail: config.go -- the configuration texts, their versions, and the administration answers
// Detail: views.go -- what a sample device answers through the relay
// Detail: console.go -- the commands the sample console answers
// Detail: script.go -- the scripted events and the state they change
// Related: register/register.go -- the registration that makes this the build's data source
//
// Package sample answers the fleet pages from a fixed estate, so the interface
// can be opened and judged before any real estate exists. Importing it
// registers nothing: New builds a source for a caller to serve, and the
// register sub-package makes one the build's source. Only a build made with
// the ze_fleet_ui_sample tag links that sub-package, and no shipped build sets
// that tag: sample data in a shipped build would be shown as real. le links
// this package to record the pages, because le is development tooling.
//
// The estate is written as tables. Every time in them is an age, measured
// back from the moment the source was made, so "last seen" and a chart range
// read the same whenever the build is started. The only state that changes is
// the script's: see script.go.
package sample

import (
	"context"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/web/fleetui"
)

// estateName is the name of the sample hub, and so of the sample estate.
const estateName = "hub-lon-01"

// Stepper is the stepped mode of the sample script. The fleet package finds it
// by a type assertion on the registered source, for its prototype-only route
// and for tests, and a source that is not the sample one does not have it.
type Stepper interface {
	// Step applies the next event of the script, publishes it, and answers
	// it. It also holds the script, as Hold does. After Close it refuses.
	Step() (fleetui.Event, error)
	// Hold stops the timer from advancing the script, for good. A test calls
	// it before it opens a page, so no event arrives that it did not step.
	Hold()
}

// The interfaces the sample source fills.
var (
	_ fleetui.Source       = (*estate)(nil)
	_ Stepper              = (*estate)(nil)
	_ fleetui.SampleSource = (*estate)(nil)
)

// estate is the sample data source. Safe for concurrent use.
//
// It owns one goroutine, the script timer. The sequence is: the first
// Subscribe starts it, each tick applies one event of the script, and Close
// stops it and waits for it. A caller that subscribed MUST call Close when the
// source is no longer used; the registered source of a review build lives as
// long as the process.
type estate struct {
	// clock is the wall clock, and start is what it read when the source was
	// made. Neither changes.
	clock func() time.Time
	start time.Time
	// period is the time between two scripted events in timed mode. It is set
	// before the first Subscribe and never after.
	period time.Duration

	feed fleetui.Feed
	// stop is closed by Close to end the timer, and done is closed by the
	// timer when it has ended.
	stop chan struct{}
	done chan struct{}

	// mu guards every field below.
	mu     sync.Mutex
	script scriptState
	// held reports that the timer no longer advances the script.
	held bool
	// started reports that the timer goroutine was started.
	started bool
	closed  bool
}

// Source is the sample data source as New answers it: every question of the
// fleet pages, the stepped script, and Close. A caller that subscribed, or
// stepped the script, MUST call Close when it no longer uses the source.
type Source interface {
	fleetui.Source
	Stepper
	Close()
}

// New builds the sample source over clock: time.Now for a build, a pinned
// function for a recording or a test. It registers nothing, so two sources can
// be served in one process; the register sub-package makes one the build's.
func New(clock func() time.Time) Source {
	return newEstate(clock)
}

// newEstate builds the sample source. clock is the wall clock: time.Now in a
// build, a fixed function in a test.
func newEstate(clock func() time.Time) *estate {
	start := clock()

	return &estate{
		clock:  clock,
		start:  start,
		period: scriptPeriod,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
		script: newScriptState(start),
	}
}

// snapshot is everything an answer is computed from: the two clocks and the
// script's state, copied under the lock. The tables never change, so an
// answer built from one snapshot is consistent with itself.
type snapshot struct {
	start  time.Time
	now    time.Time
	script scriptState
}

// snapshot copies the state an answer is computed from.
func (source *estate) snapshot() *snapshot {
	source.mu.Lock()
	defer source.mu.Unlock()

	return &snapshot{start: source.start, now: source.clock(), script: source.script.clone()}
}

// ago answers the moment age before the source was made.
func (snap *snapshot) ago(age time.Duration) time.Time {
	return snap.start.Add(-age)
}

// Close stops the script timer, waits for it, and closes the event feed. A
// second Close does nothing.
func (source *estate) Close() {
	source.mu.Lock()
	if source.closed {
		source.mu.Unlock()
		return
	}
	source.closed = true
	started := source.started
	source.mu.Unlock()

	close(source.stop)
	if started {
		<-source.done
	}
	source.feed.Close()
}

// Now answers the clock of the sample hub.
func (source *estate) Now(context.Context) (time.Time, error) {
	return source.clock(), nil
}

// SampleData marks the source as answering an invented estate, which the
// fleet header says with its "Sample data" tag (fleetui.SampleSource).
func (*estate) SampleData() {}

// Estate answers the name of the sample estate.
func (*estate) Estate(context.Context) (fleetui.Estate, error) {
	return fleetui.Estate{Name: estateName}, nil
}

// Devices answers the sample devices. Each call builds its own.
func (source *estate) Devices(context.Context) ([]fleetui.Device, error) {
	return devicesAt(source.snapshot()), nil
}

// Device answers one sample device by name.
func (source *estate) Device(_ context.Context, name string) (fleetui.Device, error) {
	row, err := findRow(name)
	if err != nil {
		return fleetui.Device{}, err
	}

	return deviceAt(row, source.snapshot()), nil
}

// TreeViews answers the tree views the sample hub declares.
func (*estate) TreeViews(context.Context) ([]fleetui.TreeView, error) {
	return declaredTreeViews(), nil
}

// SavedFilters answers the same saved filters for every user: the sample hub
// keeps no per-user state.
func (*estate) SavedFilters(context.Context, string) ([]fleetui.SavedFilter, error) {
	return savedFilters(), nil
}

// DeviceSeries answers the generated history of one metric of one device.
func (source *estate) DeviceSeries(
	_ context.Context, device string, metric fleetui.Metric, span fleetui.TimeSpan,
) (fleetui.Series, error) {
	return deviceSeries(source.snapshot(), device, metric, span)
}

// EstateSeries answers the generated history of one metric over devices.
func (source *estate) EstateSeries(
	_ context.Context, devices []string, metric fleetui.Metric, span fleetui.TimeSpan,
) (fleetui.Series, error) {
	return estateSeries(source.snapshot(), devices, metric, span)
}

// HistoryUsage answers how much the sample history would hold.
func (source *estate) HistoryUsage(context.Context) (fleetui.HistoryUsage, error) {
	return historyUsage(source.snapshot()), nil
}

// Alarms answers the open alarms.
func (source *estate) Alarms(context.Context) ([]fleetui.Alarm, error) {
	return alarmsAt(source.snapshot()), nil
}

// AlarmHistory answers the alarms of one device over the last week.
func (source *estate) AlarmHistory(_ context.Context, device string) ([]fleetui.AlarmEvent, error) {
	row, err := findRow(device)
	if err != nil {
		return nil, err
	}

	return alarmHistoryAt(source.snapshot(), row.name), nil
}

// Tasks answers the tasks the sample hub keeps.
func (source *estate) Tasks(context.Context) ([]fleetui.Task, error) {
	return tasksAt(source.snapshot()), nil
}

// Task answers one task with its per-device results.
func (source *estate) Task(_ context.Context, id string) (fleetui.TaskDetail, error) {
	return taskAt(source.snapshot(), id)
}

// DeviceConfig answers the configuration the sample hub holds for a device.
func (*estate) DeviceConfig(_ context.Context, device string) (fleetui.DeviceConfig, error) {
	row, err := findRow(device)
	if err != nil {
		return fleetui.DeviceConfig{}, err
	}

	return deviceConfig(row), nil
}

// ConfigVersions answers the versions the sample hub has held for a device.
func (source *estate) ConfigVersions(_ context.Context, device string) ([]fleetui.ConfigVersion, error) {
	row, err := findRow(device)
	if err != nil {
		return nil, err
	}

	return configVersions(row, source.snapshot()), nil
}

// ConfigText answers the text of one version of a device's configuration.
func (*estate) ConfigText(_ context.Context, device, version string) (string, error) {
	row, err := findRow(device)
	if err != nil {
		return "", err
	}

	return configTextOf(row, version)
}

// Divergence answers both sides of the diverged device's configuration.
func (source *estate) Divergence(_ context.Context, device string) (fleetui.Divergence, error) {
	row, err := findRow(device)
	if err != nil {
		return fleetui.Divergence{}, err
	}

	return divergenceOf(row, source.snapshot())
}

// Rollouts answers the rollouts.
func (source *estate) Rollouts(context.Context) ([]fleetui.Rollout, error) {
	return rolloutsAt(source.snapshot()), nil
}

// Rollout answers one rollout with its batches and devices.
func (source *estate) Rollout(_ context.Context, name string) (fleetui.RolloutDetail, error) {
	return rolloutAt(source.snapshot(), name)
}

// AuditEntries answers the audit trail of the last week.
func (source *estate) AuditEntries(context.Context) ([]fleetui.AuditEntry, error) {
	return auditAt(source.snapshot()), nil
}

// Templates answers the configuration templates.
func (source *estate) Templates(context.Context) ([]fleetui.Template, error) {
	return templatesAt(source.snapshot()), nil
}

// AccessProfiles answers the access table.
func (*estate) AccessProfiles(context.Context) ([]fleetui.AccessProfile, error) {
	return accessProfiles(), nil
}

// Settings answers the fleet settings of the sample hub.
func (*estate) Settings(context.Context) (fleetui.Settings, error) {
	return settings(), nil
}

// DeviceSystem answers the hardware, the resources and the clock of a device.
func (source *estate) DeviceSystem(_ context.Context, device string) (fleetui.DeviceSystem, error) {
	snap := source.snapshot()

	row, err := reach(snap, device)
	if err != nil {
		return fleetui.DeviceSystem{}, err
	}

	return systemAt(row, snap), nil
}

// DeviceInterfaces answers the interfaces of a device.
func (source *estate) DeviceInterfaces(_ context.Context, device string) ([]fleetui.Interface, error) {
	snap := source.snapshot()

	row, err := reach(snap, device)
	if err != nil {
		return nil, err
	}

	return interfacesAt(row, snap), nil
}

// DevicePeers answers the BGP peers of a device.
func (source *estate) DevicePeers(_ context.Context, device string) ([]fleetui.Peer, error) {
	snap := source.snapshot()

	row, err := reach(snap, device)
	if err != nil {
		return nil, err
	}

	return peersAt(row, snap), nil
}

// DeviceFamilies answers the BGP summary of a device. The device that runs
// the oldest release does not answer it.
func (source *estate) DeviceFamilies(_ context.Context, device string) ([]fleetui.FamilySummary, error) {
	snap := source.snapshot()

	row, err := reach(snap, device)
	if err != nil {
		return nil, err
	}
	if row.release == releaseOldest {
		return nil, viewUnsupported(row)
	}

	return familiesAt(row), nil
}

// DeviceAdjacencies answers the adjacencies of the other routing protocols a
// device runs. A device that runs none answers an empty list.
func (source *estate) DeviceAdjacencies(
	_ context.Context, device string,
) ([]fleetui.ProtocolAdjacencies, error) {
	snap := source.snapshot()

	row, err := reach(snap, device)
	if err != nil {
		return nil, err
	}

	return adjacenciesAt(row, snap), nil
}

// DeviceLogs answers the recent log lines of a device.
func (source *estate) DeviceLogs(_ context.Context, device string) ([]fleetui.LogLine, error) {
	snap := source.snapshot()

	row, err := reach(snap, device)
	if err != nil {
		return nil, err
	}

	return logsAt(row, snap), nil
}

// Console answers one of the commands the sample console knows.
func (source *estate) Console(_ context.Context, device, _, command string) (fleetui.ConsoleAnswer, error) {
	snap := source.snapshot()

	row, err := reach(snap, device)
	if err != nil {
		return fleetui.ConsoleAnswer{}, err
	}

	return consoleAnswer(row, snap, command)
}

// Subscribe opens a subscription to the scripted events, and starts the
// script timer when it is the first.
func (source *estate) Subscribe(context.Context) (*fleetui.Subscription, error) {
	source.startScript()

	return source.feed.Subscribe()
}
