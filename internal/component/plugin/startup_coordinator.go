// Design: docs/architecture/api/process-protocol.md — plugin process management

package plugin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// coordinatorLogger is the coordinator subsystem logger (lazy initialization).
// Controlled by ze.log.plugin.coordinator environment variable.
var coordinatorLogger = slogutil.LazyLogger("plugin.coordinator")

// StartupCoordinator synchronizes plugin startup across stages.
// All plugins must complete each stage before any can proceed to the next.
//
// Usage:
//
//	coord := NewStartupCoordinator(3)  // 3 plugins
//	go coord.Run(ctx)
//
//	// Each plugin goroutine:
//	coord.StageComplete(pluginID, StageRegistration)
//	coord.WaitForStage(ctx, StageConfig)
//	// ... receive config ...
//	coord.StageComplete(pluginID, StageConfig)
//	// etc.
type StartupCoordinator struct {
	pluginCount int

	mu             sync.Mutex
	currentStage   PluginStage
	stageStartTime time.Time     // when current stage began
	lastProgress   time.Time     // when any plugin last completed a stage
	stageComplete  []bool        // which plugins completed current stage
	stageCh        chan struct{} // closed when stage advances
	failedPlugin   int           // -1 if none failed
	failedMsg      string
	err            error
}

// NewStartupCoordinator creates a coordinator for the given number of plugins.
func NewStartupCoordinator(pluginCount int) *StartupCoordinator {
	now := time.Now()
	return &StartupCoordinator{
		pluginCount:    pluginCount,
		currentStage:   StageRegistration,
		stageStartTime: now,
		lastProgress:   now,
		stageComplete:  make([]bool, pluginCount),
		stageCh:        make(chan struct{}),
		failedPlugin:   -1,
	}
}

// noteProgressLocked records that the tier advanced in some observable way.
//
// It deliberately does NOT wake the waiters. They are already sleeping on a
// timer sized to the remaining stall window and re-read lastProgress when it
// fires, which gives the same "extend on progress" semantics. Broadcasting
// every progress event instead would wake every waiter once per plugin per
// stage -- O(N^2) wakeups for a 20+ plugin tier -- and that scheduling churn
// perturbs startup timing for no benefit.
//
// Must be called with the lock held.
func (c *StartupCoordinator) noteProgressLocked(at time.Time) {
	c.lastProgress = at
}

// StageStartTime returns when the current stage began.
//
// This is reporting/diagnostic state, NOT the barrier's deadline base: the
// barrier measures a stall from lastProgress (WaitForStageProgress), so a stage
// legitimately outlives stageStartTime+timeout while plugins keep completing.
func (c *StartupCoordinator) StageStartTime() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stageStartTime
}

// SetStartTime sets the stage start time for the initial stage (Registration).
// Called after ProcessManager.StartWithContext returns, so the Registration
// timeout includes process fork time, not time before processes exist.
func (c *StartupCoordinator) SetStartTime(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stageStartTime = t
	// The stall window for the first stage runs from here too: before any
	// plugin has completed anything, "last progress" is the stage start.
	c.lastProgress = t
}

// StageComplete signals that a plugin completed a stage.
// Must be called with the current stage - calls with wrong stage are ignored.
func (c *StartupCoordinator) StageComplete(pluginID int, stage PluginStage) {
	c.mu.Lock()
	defer c.mu.Unlock()

	coordinatorLogger().Debug("coordinator: StageComplete", "plugin", pluginID, "stage", stage, "current", c.currentStage)

	// Ignore if not current stage or already failed
	if stage != c.currentStage || c.failedPlugin >= 0 {
		coordinatorLogger().Debug("coordinator: StageComplete IGNORED", "plugin", pluginID, "stage", stage, "current", c.currentStage, "failed", c.failedPlugin)
		return
	}

	// Ignore invalid plugin ID
	if pluginID < 0 || pluginID >= c.pluginCount {
		return
	}

	// Only the FIRST completion for this plugin in this stage is progress.
	// Re-sending it must not extend the stall window, or a looping plugin
	// could hold the barrier open forever and remove its bound entirely.
	if c.stageComplete[pluginID] {
		return
	}

	// Mark complete
	c.stageComplete[pluginID] = true
	c.noteProgressLocked(time.Now())
	coordinatorLogger().Debug("coordinator: StageComplete marked", "plugin", pluginID, "complete", fmt.Sprintf("%v", c.stageComplete))

	// Check if all plugins completed
	if c.allComplete() {
		c.advanceStage()
	}
}

// WaitForStage blocks until all plugins reach the given stage.
// Returns error on context cancellation or if a plugin failed.
func (c *StartupCoordinator) WaitForStage(ctx context.Context, stage PluginStage) error {
	coordinatorLogger().Debug("coordinator: WaitForStage START", "waiting_for", stage)
	for {
		c.mu.Lock()
		// Check if failed
		if c.failedPlugin >= 0 {
			err := c.err
			c.mu.Unlock()
			coordinatorLogger().Debug("coordinator: WaitForStage FAILED", "waiting_for", stage, "err", err)
			return err
		}

		// Check if already at or past requested stage
		if c.currentStage >= stage {
			c.mu.Unlock()
			coordinatorLogger().Debug("coordinator: WaitForStage DONE", "waiting_for", stage, "current", c.currentStage)
			return nil
		}

		currentForLog := c.currentStage
		// Deep copy slice for logging (avoid race with writer)
		completeForLog := make([]bool, len(c.stageComplete))
		copy(completeForLog, c.stageComplete)

		// Get channel to wait on
		ch := c.stageCh
		c.mu.Unlock()

		coordinatorLogger().Debug("coordinator: WaitForStage BLOCKING", "waiting_for", stage, "current", currentForLog, "complete", fmt.Sprintf("%v", completeForLog))

		// Wait for stage advance or context cancel
		select {
		case <-ch:
			coordinatorLogger().Debug("coordinator: WaitForStage UNBLOCKED", "waiting_for", stage)
			// Stage advanced, loop and check again
		case <-ctx.Done():
			coordinatorLogger().Debug("coordinator: WaitForStage TIMEOUT", "waiting_for", stage)
			return ctx.Err()
		}
	}
}

// WaitForStageProgress blocks until all plugins reach the given stage, failing
// only when the WHOLE TIER goes `stall` without any observable progress.
//
// This is deliberately NOT a wall-clock budget for the stage. A tier is
// routinely 20+ plugins (bgp plus every bgp-* plugin); on a loaded host they
// all slow down together, so a flat per-stage budget measured from
// StageStartTime expires while every plugin is still handshaking normally, and
// the engine then stops all of them. Load must not be able to reach this
// barrier; only a genuinely wedged plugin should.
//
// It stays BOUNDED, which is the point of having a barrier at all: progress
// events are finite (each plugin completes each stage at most once -- repeated
// StageComplete calls are ignored), so the wait cannot exceed
// (pluginCount+1) * stall for a stage, and ctx still ends it at any time.
//
// A non-positive stall disables the stall check and waits on ctx alone.
func (c *StartupCoordinator) WaitForStageProgress(ctx context.Context, stage PluginStage, stall time.Duration) error {
	if stall <= 0 {
		return c.WaitForStage(ctx, stage)
	}

	for {
		c.mu.Lock()
		if c.failedPlugin >= 0 {
			err := c.err
			c.mu.Unlock()
			return err
		}
		if c.currentStage >= stage {
			c.mu.Unlock()
			return nil
		}
		idle := time.Since(c.lastProgress)
		stageCh := c.stageCh
		c.mu.Unlock()

		if idle >= stall {
			return fmt.Errorf("no plugin progress for %v while waiting for stage %d", stall, stage)
		}

		// A fresh timer per iteration (rather than Reset on a shared one) keeps
		// the wait free of the stale-fire race Reset has, with no drain needed.
		// The loop is bounded by the number of progress events, so this cannot
		// churn: at most pluginCount+1 timers per stage.
		timer := time.NewTimer(stall - idle)
		select {
		case <-stageCh:
			// Stage advanced or a plugin failed; re-evaluate.
		case <-timer.C:
			// The window elapsed. The next iteration re-reads lastProgress: if a
			// plugin completed meanwhile the window simply restarts from there,
			// otherwise the tier is stalled and the loop returns the error.
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
		timer.Stop()
	}
}

// PluginFailed signals that a plugin failed during startup.
// This aborts the entire startup process.
func (c *StartupCoordinator) PluginFailed(pluginID int, message string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Only record first failure
	if c.failedPlugin >= 0 {
		return
	}

	c.failedPlugin = pluginID
	c.failedMsg = message
	c.err = fmt.Errorf("plugin %d failed: %s", pluginID, message)

	// Unblock all waiters
	close(c.stageCh)
}

// Run runs the coordinator until all plugins are ready or an error occurs.
// This is typically run in a goroutine.
func (c *StartupCoordinator) Run(ctx context.Context) error {
	// Wait for all stages to complete
	finalStage := StageReady

	for {
		c.mu.Lock()
		// Check if failed
		if c.failedPlugin >= 0 {
			err := c.err
			c.mu.Unlock()
			return err
		}

		// Check if done
		if c.currentStage > finalStage {
			c.mu.Unlock()
			return nil
		}

		ch := c.stageCh
		c.mu.Unlock()

		// Wait for stage advance, failure, or context cancel
		select {
		case <-ch:
			// Something changed, loop and check
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for plugins to complete stage %d", c.currentStage)
		}
	}
}

// CurrentStage returns the current stage.
func (c *StartupCoordinator) CurrentStage() PluginStage {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.currentStage
}

// Failed returns true if a plugin has failed.
func (c *StartupCoordinator) Failed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.failedPlugin >= 0
}

// allComplete returns true if all plugins completed current stage.
// Must be called with lock held.
func (c *StartupCoordinator) allComplete() bool {
	for i, done := range c.stageComplete {
		if !done {
			coordinatorLogger().Debug("coordinator: allComplete FALSE", "waiting_for_plugin", i)
			return false
		}
	}
	coordinatorLogger().Debug("coordinator: allComplete TRUE")
	return true
}

// advanceStage moves to the next stage.
// Must be called with lock held.
func (c *StartupCoordinator) advanceStage() {
	oldStage := c.currentStage
	// Reset completion tracking
	for i := range c.stageComplete {
		c.stageComplete[i] = false
	}

	// Advance stage and record when it began. The new stage's stall window
	// starts here: reaching a new stage is itself progress.
	c.currentStage++
	c.stageStartTime = time.Now()
	c.lastProgress = c.stageStartTime
	coordinatorLogger().Debug("coordinator: advanceStage", "from", oldStage, "to", c.currentStage)

	// Notify waiters by closing old channel and creating new one
	close(c.stageCh)
	c.stageCh = make(chan struct{})
}
