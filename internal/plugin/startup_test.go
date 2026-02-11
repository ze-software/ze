package plugin

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStageSynchronization verifies all plugins complete each stage before next.
//
// VALIDATES: Staged startup synchronizes across all plugins.
// PREVENTS: Race conditions where one plugin advances before others complete.
func TestStageSynchronization(t *testing.T) {
	t.Run("two_plugins_sync_stage1", func(t *testing.T) {
		coordinator := NewStartupCoordinator(2)

		var stage1Complete atomic.Int32
		var stage2Started atomic.Int32
		var wg sync.WaitGroup

		// Plugin 1: fast
		wg.Go(func() {
			// Complete stage 1
			stage1Complete.Add(1)
			coordinator.StageComplete(0, StageRegistration)

			// Wait for stage 2
			err := coordinator.WaitForStage(context.Background(), StageConfig)
			require.NoError(t, err)
			stage2Started.Add(1)
		})

		// Plugin 2: slow
		wg.Go(func() {
			time.Sleep(50 * time.Millisecond)

			// At this point, plugin 1 should NOT have started stage 2
			assert.Equal(t, int32(1), stage1Complete.Load(), "plugin 1 should have completed stage 1")
			assert.Equal(t, int32(0), stage2Started.Load(), "plugin 1 should NOT have started stage 2 yet")

			// Complete stage 1
			stage1Complete.Add(1)
			coordinator.StageComplete(1, StageRegistration)

			// Wait for stage 2
			err := coordinator.WaitForStage(context.Background(), StageConfig)
			require.NoError(t, err)
			stage2Started.Add(1)
		})

		// Coordinator advances stages when all complete
		go func() { _ = coordinator.Run(context.Background()) }()

		wg.Wait()
		assert.Equal(t, int32(2), stage1Complete.Load())
		assert.Equal(t, int32(2), stage2Started.Load())
	})

	t.Run("barrier_timeout_from_stage_start", func(t *testing.T) {
		// VALIDATES: Fast plugin does NOT timeout when slow plugin arrives late
		//   but total stage time is within timeout.
		// PREVENTS: Fast plugins failing because they wait for slow plugins at barrier.

		coordinator := NewStartupCoordinator(2)

		// Stage start time is set when coordinator is created (for Registration).
		stageStart := coordinator.StageStartTime()
		assert.False(t, stageStart.IsZero(), "stage start time should be set at construction")

		// Simulate: fast plugin completes Registration quickly
		coordinator.StageComplete(0, StageRegistration)

		// Simulate: slow plugin completes 200ms later (still within 5s default)
		time.Sleep(200 * time.Millisecond)
		coordinator.StageComplete(1, StageRegistration)

		// Now stage advances — start time should be updated
		newStart := coordinator.StageStartTime()
		assert.True(t, newStart.After(stageStart), "stage start time should advance after stage change")

		// Both should be able to wait for Config with no timeout
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := coordinator.WaitForStage(ctx, StageConfig)
		require.NoError(t, err, "should not timeout — stage advanced successfully")
	})

	t.Run("stage_start_time_advances", func(t *testing.T) {
		// VALIDATES: Each advanceStage updates stageStartTime.
		// PREVENTS: Stale start time causing accumulated timing errors.

		coordinator := NewStartupCoordinator(1)
		initialStart := coordinator.StageStartTime()
		assert.False(t, initialStart.IsZero())

		// Complete Registration → advances to Config
		time.Sleep(10 * time.Millisecond) // Ensure clock difference
		coordinator.StageComplete(0, StageRegistration)
		configStart := coordinator.StageStartTime()
		assert.True(t, configStart.After(initialStart), "Config start should be after Registration start")

		// Complete Config → advances to Capability
		time.Sleep(10 * time.Millisecond)
		coordinator.StageComplete(0, StageConfig)
		capStart := coordinator.StageStartTime()
		assert.True(t, capStart.After(configStart), "Capability start should be after Config start")
	})

	t.Run("barrier_timeout_expired", func(t *testing.T) {
		// VALIDATES: Plugin that exceeds stageStart+timeout still fails.
		// PREVENTS: Infinite waits when a plugin is truly stuck.

		coordinator := NewStartupCoordinator(2)

		// Only plugin 0 completes — plugin 1 never does
		coordinator.StageComplete(0, StageRegistration)

		// Wait with very short timeout — should expire
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		err := coordinator.WaitForStage(ctx, StageConfig)
		require.Error(t, err, "should timeout when plugin never completes")
	})

	t.Run("three_plugins_all_stages", func(t *testing.T) {
		coordinator := NewStartupCoordinator(3)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var completedStages [3]atomic.Int32
		var wg sync.WaitGroup

		for i := range 3 {
			wg.Add(1)
			go func(pluginID int) {
				defer wg.Done()

				stages := []PluginStage{
					StageRegistration,
					StageConfig,
					StageCapability,
					StageRegistry,
					StageReady,
				}

				for _, stage := range stages {
					// Vary timing to test synchronization
					time.Sleep(time.Duration(pluginID*10) * time.Millisecond)

					coordinator.StageComplete(pluginID, stage)
					completedStages[pluginID].Add(1)

					// Wait for next stage (except after Ready)
					if stage != StageReady {
						nextStage := stage + 1
						err := coordinator.WaitForStage(ctx, nextStage)
						if err != nil {
							return
						}
					}
				}
			}(i)
		}

		go func() { _ = coordinator.Run(ctx) }()
		wg.Wait()

		// All plugins should complete all 5 stages
		for i := range 3 {
			assert.Equal(t, int32(5), completedStages[i].Load(), "plugin %d stages", i)
		}
	})
}

// TestStartupCoordinatorTimeout verifies timeout kills startup.
//
// VALIDATES: Stage timeout aborts startup.
// PREVENTS: Hung plugins blocking startup forever.
func TestStartupCoordinatorTimeout(t *testing.T) {
	coordinator := NewStartupCoordinator(2)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Only plugin 0 completes - plugin 1 never does
	coordinator.StageComplete(0, StageRegistration)

	// Run coordinator - should timeout
	err := coordinator.Run(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

// TestStartupCoordinatorFailed verifies failed signal aborts startup.
//
// VALIDATES: Plugin failure aborts startup immediately.
// PREVENTS: Startup continuing after plugin reports failure.
func TestStartupCoordinatorFailed(t *testing.T) {
	coordinator := NewStartupCoordinator(2)
	ctx := context.Background()

	// Plugin 0 completes stage 1
	coordinator.StageComplete(0, StageRegistration)

	// Plugin 1 fails
	coordinator.PluginFailed(1, "config error: missing required field")

	// Run coordinator - should fail
	err := coordinator.Run(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin 1 failed")
	assert.Contains(t, err.Error(), "config error")
}

// TestTwoPluginsFullStartup verifies complete startup with two plugins.
//
// VALIDATES: Two plugins with different speeds complete all stages.
// PREVENTS: Deadlock when one plugin is slower (e.g., has config patterns).
func TestTwoPluginsFullStartup(t *testing.T) {
	coord := NewStartupCoordinator(2)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var plugin0Done, plugin1Done atomic.Bool

	// Plugin 0: Fast (no patterns, like Python test plugin)
	wg.Go(func() {

		// Stage 1: Registration
		coord.StageComplete(0, StageRegistration)
		if err := coord.WaitForStage(ctx, StageConfig); err != nil {
			t.Errorf("Plugin0 WaitForStage(Config): %v", err)
			return
		}

		// Stage 2: Config delivery (fast - no patterns)
		time.Sleep(10 * time.Millisecond)
		coord.StageComplete(0, StageConfig)
		if err := coord.WaitForStage(ctx, StageCapability); err != nil {
			t.Errorf("Plugin0 WaitForStage(Capability): %v", err)
			return
		}

		// Stage 3: Capability
		coord.StageComplete(0, StageCapability)
		if err := coord.WaitForStage(ctx, StageRegistry); err != nil {
			t.Errorf("Plugin0 WaitForStage(Registry): %v", err)
			return
		}

		// Stage 4: Registry
		coord.StageComplete(0, StageRegistry)
		if err := coord.WaitForStage(ctx, StageReady); err != nil {
			t.Errorf("Plugin0 WaitForStage(Ready): %v", err)
			return
		}

		// Stage 5: Ready
		coord.StageComplete(0, StageReady)
		plugin0Done.Store(true)
	})

	// Plugin 1: Slow (has patterns, like RIB plugin)
	wg.Go(func() {
		time.Sleep(50 * time.Millisecond) // Start slightly later

		// Stage 1: Registration
		coord.StageComplete(1, StageRegistration)
		if err := coord.WaitForStage(ctx, StageConfig); err != nil {
			t.Errorf("Plugin1 WaitForStage(Config): %v", err)
			return
		}

		// Stage 2: Config delivery (slow - has patterns, needs reactor call)
		time.Sleep(100 * time.Millisecond)
		coord.StageComplete(1, StageConfig)
		if err := coord.WaitForStage(ctx, StageCapability); err != nil {
			t.Errorf("Plugin1 WaitForStage(Capability): %v", err)
			return
		}

		// Stage 3: Capability
		coord.StageComplete(1, StageCapability)
		if err := coord.WaitForStage(ctx, StageRegistry); err != nil {
			t.Errorf("Plugin1 WaitForStage(Registry): %v", err)
			return
		}

		// Stage 4: Registry
		coord.StageComplete(1, StageRegistry)
		if err := coord.WaitForStage(ctx, StageReady); err != nil {
			t.Errorf("Plugin1 WaitForStage(Ready): %v", err)
			return
		}

		// Stage 5: Ready
		coord.StageComplete(1, StageReady)
		plugin1Done.Store(true)
	})

	// Run coordinator
	go func() { _ = coord.Run(ctx) }()

	wg.Wait()
	assert.True(t, plugin0Done.Load(), "Plugin 0 should complete")
	assert.True(t, plugin1Done.Load(), "Plugin 1 should complete")
}
