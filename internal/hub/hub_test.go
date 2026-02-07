package hub

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Shared test binary setup - built once, used by all tests.
var (
	hubBinaryPath string
	hubBuildOnce  sync.Once
	hubBuildErr   error
	hubTestTmpDir string
)

// TestMain handles cleanup of shared test resources.
func TestMain(m *testing.M) {
	code := m.Run()

	if hubTestTmpDir != "" {
		_ = os.RemoveAll(hubTestTmpDir)
	}

	os.Exit(code)
}

// buildHubBinary builds the ze-subsystem binary once for all tests.
func buildHubBinary(t *testing.T) {
	t.Helper()

	hubBuildOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		hubTestTmpDir, hubBuildErr = os.MkdirTemp("", "ze-hub-test-*")
		if hubBuildErr != nil {
			hubBuildErr = fmt.Errorf("create temp dir: %w", hubBuildErr)
			return
		}

		hubBinaryPath = filepath.Join(hubTestTmpDir, "ze-subsystem")

		listCmd := exec.CommandContext(ctx, "go", "list", "-m", "-f", "{{.Dir}}")
		output, err := listCmd.Output()
		if err != nil {
			hubBuildErr = fmt.Errorf("find project root: %w", err)
			return
		}
		projectRoot := strings.TrimSpace(string(output))

		buildCmd := exec.CommandContext(ctx, "go", "build", "-o", hubBinaryPath, "./cmd/ze-subsystem") //nolint:gosec // test code
		buildCmd.Dir = projectRoot
		buildOutput, err := buildCmd.CombinedOutput()
		if err != nil {
			hubBuildErr = fmt.Errorf("build ze-subsystem: %w\n%s", err, buildOutput)
			return
		}
	})

	if hubBuildErr != nil {
		t.Skipf("skipping test requiring ze-subsystem binary: %v", hubBuildErr)
	}
}

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

// TestOrchestratorStartStop verifies Orchestrator starts and stops cleanly.
//
// VALIDATES: Orchestrator lifecycle works with ze-subsystem binary.
// PREVENTS: Hang on shutdown, leaked goroutines.
func TestOrchestratorStartStop(t *testing.T) {
	buildHubBinary(t)

	cfg := &HubConfig{
		Plugins: []PluginDef{
			{
				Name: "echo-test",
				Run:  hubBinaryPath + " --mode=session",
			},
		},
	}

	o := NewOrchestrator(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := o.Start(ctx)
	require.NoError(t, err)

	// Brief delay to let things settle
	time.Sleep(100 * time.Millisecond)

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
