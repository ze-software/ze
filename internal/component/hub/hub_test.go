package hub

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrchestratorNew verifies Orchestrator creates with config.
//
// VALIDATES: Orchestrator can be created with configuration.
// PREVENTS: Nil pointer on creation.
func TestOrchestratorNew(t *testing.T) {
	cfg := &HubConfig{
		Plugins: []PluginDef{
			{Name: "test", Run: "echo hello"},
		},
	}

	o := NewOrchestrator(cfg)
	require.NotNil(t, o)
	assert.NotNil(t, o.subsystems)
	assert.NotNil(t, o.registry)
}

// TestOrchestratorNewEmpty verifies Orchestrator creates with empty config.
//
// VALIDATES: Orchestrator works with no plugins.
// PREVENTS: Panic on empty config.
func TestOrchestratorNewEmpty(t *testing.T) {
	cfg := &HubConfig{}
	o := NewOrchestrator(cfg)
	require.NotNil(t, o)
}

// TestOrchestratorNewNil verifies Orchestrator handles nil config.
//
// VALIDATES: Nil config defaults to empty.
// PREVENTS: Panic on nil config.
func TestOrchestratorNewNil(t *testing.T) {
	o := NewOrchestrator(nil)
	require.NotNil(t, o)
}

// TestOrchestratorStartStopEmpty verifies Orchestrator starts and stops with no plugins.
//
// VALIDATES: Orchestrator lifecycle works without plugins.
// PREVENTS: Hang on shutdown, leaked goroutines.
func TestOrchestratorStartStopEmpty(t *testing.T) {
	cfg := &HubConfig{}
	o := NewOrchestrator(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := o.Start(ctx)
	require.NoError(t, err)

	o.Stop()
}

// TestOrchestratorShutdownClean verifies clean shutdown without hanging.
//
// VALIDATES: Shutdown completes within timeout.
// PREVENTS: Deadlock on shutdown.
func TestOrchestratorShutdownClean(t *testing.T) {
	cfg := &HubConfig{}
	o := NewOrchestrator(cfg)

	ctx := context.Background()
	err := o.Start(ctx)
	require.NoError(t, err)

	// Should not hang
	done := make(chan struct{})
	go func() {
		o.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown timed out")
	}
}

// TestOrchestratorRegistry verifies registry is accessible.
//
// VALIDATES: Registry method returns valid registry.
// PREVENTS: Nil registry access.
func TestOrchestratorRegistry(t *testing.T) {
	o := NewOrchestrator(nil)
	require.NotNil(t, o.Registry())
}

// TestOrchestratorSubsystems verifies subsystems are accessible.
//
// VALIDATES: Subsystems method returns valid manager.
// PREVENTS: Nil subsystems access.
func TestOrchestratorSubsystems(t *testing.T) {
	o := NewOrchestrator(nil)
	require.NotNil(t, o.Subsystems())
}
