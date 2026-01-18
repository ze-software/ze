package plugin

import (
	"context"
	"fmt"
	"sync"

	"codeberg.org/thomas-mangin/zebgp/pkg/slogutil"
)

// coordinatorLogger is the coordinator subsystem logger.
// Controlled by zebgp.log.coordinator environment variable.
var coordinatorLogger = slogutil.Logger("coordinator")

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

	mu            sync.Mutex
	currentStage  PluginStage
	stageComplete []bool        // which plugins completed current stage
	stageCh       chan struct{} // closed when stage advances
	failedPlugin  int           // -1 if none failed
	failedMsg     string
	err           error
}

// NewStartupCoordinator creates a coordinator for the given number of plugins.
func NewStartupCoordinator(pluginCount int) *StartupCoordinator {
	return &StartupCoordinator{
		pluginCount:   pluginCount,
		currentStage:  StageRegistration,
		stageComplete: make([]bool, pluginCount),
		stageCh:       make(chan struct{}),
		failedPlugin:  -1,
	}
}

// StageComplete signals that a plugin completed a stage.
// Must be called with the current stage - calls with wrong stage are ignored.
func (c *StartupCoordinator) StageComplete(pluginID int, stage PluginStage) {
	c.mu.Lock()
	defer c.mu.Unlock()

	coordinatorLogger.Debug("coordinator: StageComplete", "plugin", pluginID, "stage", stage, "current", c.currentStage)

	// Ignore if not current stage or already failed
	if stage != c.currentStage || c.failedPlugin >= 0 {
		coordinatorLogger.Debug("coordinator: StageComplete IGNORED", "plugin", pluginID, "stage", stage, "current", c.currentStage, "failed", c.failedPlugin)
		return
	}

	// Ignore invalid plugin ID
	if pluginID < 0 || pluginID >= c.pluginCount {
		return
	}

	// Mark complete
	c.stageComplete[pluginID] = true

	// Check if all plugins completed
	if c.allComplete() {
		c.advanceStage()
	}
}

// WaitForStage blocks until all plugins reach the given stage.
// Returns error on context cancellation or if a plugin failed.
func (c *StartupCoordinator) WaitForStage(ctx context.Context, stage PluginStage) error {
	coordinatorLogger.Debug("coordinator: WaitForStage START", "waiting_for", stage)
	for {
		c.mu.Lock()
		// Check if failed
		if c.failedPlugin >= 0 {
			err := c.err
			c.mu.Unlock()
			coordinatorLogger.Debug("coordinator: WaitForStage FAILED", "waiting_for", stage, "err", err)
			return err
		}

		// Check if already at or past requested stage
		if c.currentStage >= stage {
			c.mu.Unlock()
			coordinatorLogger.Debug("coordinator: WaitForStage DONE", "waiting_for", stage, "current", c.currentStage)
			return nil
		}

		currentForLog := c.currentStage
		// Deep copy slice for logging (avoid race with writer)
		completeForLog := make([]bool, len(c.stageComplete))
		copy(completeForLog, c.stageComplete)

		// Get channel to wait on
		ch := c.stageCh
		c.mu.Unlock()

		coordinatorLogger.Debug("coordinator: WaitForStage BLOCKING", "waiting_for", stage, "current", currentForLog, "complete", fmt.Sprintf("%v", completeForLog))

		// Wait for stage advance or context cancel
		select {
		case <-ch:
			coordinatorLogger.Debug("coordinator: WaitForStage UNBLOCKED", "waiting_for", stage)
			// Stage advanced, loop and check again
		case <-ctx.Done():
			coordinatorLogger.Debug("coordinator: WaitForStage TIMEOUT", "waiting_for", stage)
			return ctx.Err()
		}
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
	for _, done := range c.stageComplete {
		if !done {
			return false
		}
	}
	return true
}

// advanceStage moves to the next stage.
// Must be called with lock held.
func (c *StartupCoordinator) advanceStage() {
	// Reset completion tracking
	for i := range c.stageComplete {
		c.stageComplete[i] = false
	}

	// Advance stage
	c.currentStage++

	// Notify waiters by closing old channel and creating new one
	close(c.stageCh)
	c.stageCh = make(chan struct{})
}
