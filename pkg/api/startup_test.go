package api

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
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Complete stage 1
			stage1Complete.Add(1)
			coordinator.StageComplete(0, StageRegistration)

			// Wait for stage 2
			err := coordinator.WaitForStage(context.Background(), StageConfig)
			require.NoError(t, err)
			stage2Started.Add(1)
		}()

		// Plugin 2: slow
		wg.Add(1)
		go func() {
			defer wg.Done()
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
		}()

		// Coordinator advances stages when all complete
		go coordinator.Run(context.Background())

		wg.Wait()
		assert.Equal(t, int32(2), stage1Complete.Load())
		assert.Equal(t, int32(2), stage2Started.Load())
	})

	t.Run("three_plugins_all_stages", func(t *testing.T) {
		coordinator := NewStartupCoordinator(3)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var completedStages [3]atomic.Int32
		var wg sync.WaitGroup

		for i := 0; i < 3; i++ {
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

		go coordinator.Run(ctx)
		wg.Wait()

		// All plugins should complete all 5 stages
		for i := 0; i < 3; i++ {
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
