// Design: docs/architecture/doctor-and-health-checks.md -- plugin state a runtime check reads
// Related: manager.go -- ProcessManager, which holds the stop record of each plugin
// Related: process.go -- the two exits that record the exit status
// Related: ../server/failure_policy.go -- applyFailurePolicy, which records a stop the policy chose
//
// A configured plugin that is not running is a fault an operator has to be
// told about, with the reason, or `ze doctor` reads a stopped plugin as one
// nobody configured. Every path that leaves a configured plugin down records
// it here, and the record raises doctor-plugin-stopped on the report bus, so
// the `plugins` health row reads down for every one of those paths, not only
// for the respawn limit. A start that reaches running clears it, and so does
// a removal, because a plugin the configuration no longer names is not down.

package process

import (
	"context"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// StopCause says why a configured plugin is not running. The zero value is a
// cause nobody set, which RecordStopped refuses.
type StopCause uint8

const (
	// StopUnspecified is no cause: a ze defect if it is ever recorded.
	StopUnspecified StopCause = iota
	// StopRespawnLimit: the per-window or cumulative respawn limit disabled
	// the plugin (Respawn).
	StopRespawnLimit
	// StopPolicyExit: the plugin exited and its failure policy (ignore, or
	// none declared) does not start it again.
	StopPolicyExit
	// StopRestartFailed: the plugin exited, its policy asked for a restart,
	// and the restart failed.
	StopRestartFailed
	// StopHandshakeFailed: the plugin's startup handshake did not reach the
	// running stage, so ze runs without it.
	StopHandshakeFailed
)

// String is the cause as an operator reads it in a doctor row.
func (c StopCause) String() string {
	switch c {
	case StopUnspecified:
		return "unspecified"
	case StopRespawnLimit:
		return "the respawn limit disabled it"
	case StopPolicyExit:
		return "it exited and its failure policy does not start it again"
	case StopRestartFailed:
		return "it exited and could not be started again"
	case StopHandshakeFailed:
		return "its startup handshake failed"
	}
	return "unknown stop cause"
}

// PluginStop is the record of one configured plugin that is down. It is a
// value type: the doctor snapshot copies it across into the checks.
type PluginStop struct {
	// Cause is why the plugin is down.
	Cause StopCause
	// Detail is what the path that stopped it knew: the error, or the limit.
	Detail string
	// Policy is the failure policy ze applied, empty where none was applied
	// (the respawn limit, a failed handshake).
	Policy string
	// ExitStatus is how the last process ended, empty where no process ended
	// (a handshake that never started one).
	ExitStatus string
	// Time is when the stop was recorded.
	Time time.Time
}

// RestartCommand is the command that starts a stopped plugin again
// (request plugin restart, handlePluginRestart in plugin/server). The doctor
// row and the bus warning name it as the fix.
func RestartCommand(name string) string {
	return "ze request plugin restart " + name
}

// Reenable lifts a respawn-limit disable and forgets the respawn history of
// name, so the next Respawn runs. Only an operator's order calls it (request
// plugin restart): the limit guards against a crash loop nobody asked for.
// Safe for concurrent use.
func (pm *ProcessManager) Reenable(name string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.disabled, name)
	delete(pm.respawnTimes, name)
	delete(pm.totalRespawns, name)
}

// RecordStopped records that the configured plugin name is down and raises
// doctor-plugin-stopped for it on the report bus, naming the cause and the
// command that starts it again. A later record replaces an earlier one: the
// latest cause is the one an operator acts on. Safe for concurrent use.
func (pm *ProcessManager) RecordStopped(name string, stop PluginStop) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.recordStoppedLocked(name, stop)
}

// recordStoppedLocked is RecordStopped for a caller that holds pm.mu.
func (pm *ProcessManager) recordStoppedLocked(name string, stop PluginStop) {
	if stop.Cause == StopUnspecified {
		panic("BUG: ProcessManager.RecordStopped: no stop cause for plugin " + name)
	}
	if stop.Time.IsZero() {
		stop.Time = time.Now()
	}
	pm.stopped[name] = stop
	report.RaiseWarningFix(reportSourcePlugin, diagnostic.CodeDoctorPluginStopped, name,
		StoppedMessage(name, &stop), RestartCommand(name))
	logger().Warn("configured plugin is not running; start it again with the fix command",
		"plugin", name, "cause", stop.Cause.String(), "detail", stop.Detail,
		"code", diagnostic.CodeDoctorPluginStopped, "fix", RestartCommand(name))
}

// StoppedMessage is the sentence that names a down plugin and why, shared by
// the bus warning and the doctor row so both say the same thing.
func StoppedMessage(name string, stop *PluginStop) string {
	var tb textbuf.Buffer
	tb.Str("configured plugin ").Str(name).Str(" is not running: ").Str(stop.Cause.String())
	if stop.Policy != "" {
		tb.Str(" (failure policy ").Str(stop.Policy).Byte(')')
	}
	if stop.ExitStatus != "" {
		tb.Str("; its process ended with ").Str(stop.ExitStatus)
	}
	if stop.Detail != "" {
		tb.Str("; ").Str(stop.Detail)
	}
	return tb.String()
}

// Stopped answers the stop record of name, false when the plugin is not
// recorded down. Safe for concurrent use.
func (pm *ProcessManager) Stopped(name string) (PluginStop, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	stop, down := pm.stopped[name]
	return stop, down
}

// ClearStopped removes the stop record of name and its bus warning. The
// engine calls it when a generation of the plugin reaches running. Safe for
// concurrent use.
func (pm *ProcessManager) ClearStopped(name string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.clearStoppedLocked(name)
}

// clearStoppedLocked is ClearStopped for a caller that holds pm.mu.
func (pm *ProcessManager) clearStoppedLocked(name string) {
	delete(pm.stopped, name)
	report.ClearWarning(reportSourcePlugin, diagnostic.CodeDoctorPluginStopped, name)
}

// exitRecord holds how a process ended. The zero value is ready to use. The
// first status recorded stands, because the exit that ended the process is
// the cause, and a later path (a reap after a panic) only repeats it. Safe
// for concurrent use.
type exitRecord struct {
	mu     sync.Mutex
	status string
	done   chan struct{}
}

// channelLocked answers the channel closed once a status is recorded. MUST be
// called with r.mu held.
func (r *exitRecord) channelLocked() chan struct{} {
	if r.done == nil {
		r.done = make(chan struct{})
	}
	return r.done
}

// record stores status unless one is already stored.
func (r *exitRecord) record(status string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status != "" {
		return
	}
	r.status = status
	close(r.channelLocked())
}

// wait answers the recorded status once there is one, or false when ctx ends
// first.
func (r *exitRecord) wait(ctx context.Context) (string, bool) {
	r.mu.Lock()
	done := r.channelLocked()
	r.mu.Unlock()
	select {
	case <-done:
	case <-ctx.Done():
		return "", false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status, true
}

// exitStatusWaitMax bounds how long ExitStatus waits for an exit to be
// recorded. A plugin's connection can close before its process is reaped, and
// a plugin that closed its connection and kept running is never reaped by
// this wait.
const exitStatusWaitMax = 2 * time.Second

// ExitStatus answers how this plugin's process ended: "exit code N" or
// "panic" for an internal plugin, the reaped process state ("exit status 1",
// "signal: killed") for a forked one. When no exit is recorded within
// exitStatusWaitMax it says so, rather than answering an empty status that
// reads as a clean exit. Safe for concurrent use.
func (p *Process) ExitStatus() string {
	ctx, cancel := context.WithTimeout(context.Background(), exitStatusWaitMax)
	defer cancel()
	if status, ok := p.exit.wait(ctx); ok {
		return status
	}
	return "no recorded exit: the process had not ended 2s after its connection closed"
}
