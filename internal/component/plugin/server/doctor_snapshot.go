// Design: docs/architecture/doctor-and-health-checks.md -- the runtime phase
// Related: server.go -- StartWithContext publishes the snapshot, Stop withdraws it
// Related: ../process/manager.go -- ConfiguredNames, GetProcess, Disabled, the state it reads
//
// The plugin doctor snapshot is how a runtime doctor check reads the state of
// the plugins without a handle on the running Server: the check context is
// shared core and carries none (diagnostic.DoctorCheckContext). It follows
// the core-leaf registry shape (internal/core/ipsecinventory): the running
// Server publishes itself as the source, a reader asks PluginDoctorSnapshot
// and gets value types, and with no server published the answer is
// ErrDoctorSnapshotNotPublished, never an empty list that reads as "every
// plugin is fine" (ai/rules/principles.md).

package server

import (
	"errors"
	"slices"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
)

// ErrDoctorSnapshotNotPublished is the answer when no plugin server is
// running in this process: the local `ze doctor` process, a test, or a daemon
// before its server started or after it stopped.
var ErrDoctorSnapshotNotPublished = errors.New("plugin state is not published: no plugin server runs in this process")

// ErrDoctorSnapshotNotStarted is the answer when the server is published but
// has not handed its plugins to a process manager yet, so it holds no plugin
// state to report.
var ErrDoctorSnapshotNotStarted = errors.New("plugin state is not available yet: plugin startup has not begun")

// DoctorSnapshot is the state of every plugin the running server knows of, as
// value types: it crosses into the doctor checks and carries no pointer into
// server state. Later problem classes add their records here, beside Plugins.
type DoctorSnapshot struct {
	// Plugins holds one entry per configured plugin, in configuration order,
	// then one per process the manager runs that no configuration names.
	Plugins []PluginDoctorState
}

// PluginDoctorState is one plugin's state at the moment of the snapshot.
type PluginDoctorState struct {
	// Name is the plugin's configured name (its process name).
	Name string
	// Configured is true when the plugin is in the manager's configuration.
	Configured bool
	// Running is true while the plugin's engine runs (process.Process.Running).
	Running bool
	// HasProcess is true when the manager holds a process for the name. A
	// configured plugin with none was never spawned, or was removed.
	HasProcess bool
	// Stage is the startup stage the process last reached; meaningful only
	// when HasProcess is true.
	Stage plugin.PluginStage
	// Disabled is true when the respawn limit disabled the plugin.
	Disabled bool
	// LastError is the cause recorded for the plugin's failed startup
	// handshake (process.Process.StartupError). Empty means no cause was
	// recorded, never that the plugin is healthy: a reader decides that from
	// Running, Stage and Disabled.
	LastError string
	// Down is true when a path that leaves a configured plugin not running
	// recorded it (process.ProcessManager.RecordStopped), and Stop then holds
	// why: the cause, the failure policy applied, the exit status, the
	// restart failure. A start that reached running, and a removal, clear it.
	Down bool
	Stop process.PluginStop
}

// doctorSnapshotSource is the server PluginDoctorSnapshot reads, or nil.
var doctorSnapshotSource atomic.Pointer[Server]

// publishDoctorSnapshot makes s the source PluginDoctorSnapshot reads. The
// most recently started server wins, which in the daemon is the only one.
func publishDoctorSnapshot(s *Server) {
	doctorSnapshotSource.Store(s)
}

// withdrawDoctorSnapshot removes s as the source, and leaves a later server's
// publication in place.
func withdrawDoctorSnapshot(s *Server) {
	doctorSnapshotSource.CompareAndSwap(s, nil)
}

// PluginDoctorSnapshot answers the running server's plugin state, or
// ErrDoctorSnapshotNotPublished when no server runs in this process, or
// ErrDoctorSnapshotNotStarted when its plugins have not been started. Safe for
// concurrent use.
func PluginDoctorSnapshot() (DoctorSnapshot, error) {
	s := doctorSnapshotSource.Load()
	if s == nil {
		return DoctorSnapshot{}, ErrDoctorSnapshotNotPublished
	}
	return s.doctorSnapshot()
}

// doctorSnapshot reads the state of every plugin from the process manager.
func (s *Server) doctorSnapshot() (DoctorSnapshot, error) {
	pm := s.procManager.Load()
	if pm == nil {
		return DoctorSnapshot{}, ErrDoctorSnapshotNotStarted
	}
	configured := pm.ConfiguredNames()
	snapshot := DoctorSnapshot{Plugins: make([]PluginDoctorState, 0, len(configured))}
	for _, name := range configured {
		snapshot.Plugins = append(snapshot.Plugins, s.pluginDoctorState(name, true))
	}
	for _, proc := range pm.AllProcesses() {
		if slices.Contains(configured, proc.Name()) {
			continue
		}
		snapshot.Plugins = append(snapshot.Plugins, s.pluginDoctorState(proc.Name(), false))
	}
	return snapshot, nil
}

// pluginDoctorState reads one plugin's state.
func (s *Server) pluginDoctorState(name string, configured bool) PluginDoctorState {
	pm := s.procManager.Load()
	state := PluginDoctorState{Name: name, Configured: configured, Disabled: pm.Disabled(name)}
	state.Stop, state.Down = pm.Stopped(name)
	proc := pm.GetProcess(name)
	if proc == nil {
		return state
	}
	state.HasProcess = true
	state.Running = proc.Running()
	state.Stage = proc.Stage()
	if err := proc.StartupError(); err != nil {
		state.LastError = err.Error()
	}
	return state
}
