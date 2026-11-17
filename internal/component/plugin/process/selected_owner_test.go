// Design: docs/architecture/api/process-protocol.md -- exact selecting process lifetime.
package process

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestSelectedOwnerCacheSurvivesCleanup captures one immutable token while
// alive, then proves cleanup receives that same ended incarnation.
func TestSelectedOwnerCacheSurvivesCleanup(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{Name: "sleep", Internal: true})
	_, err := proc.SelectedOwner()
	require.Error(t, err)
	_, recorded := proc.SelectedOwnerForCleanup()
	require.False(t, recorded)
	require.NoError(t, proc.StartWithContext(t.Context()))
	require.NoError(t, proc.InitConns())
	owner, err := proc.SelectedOwner()
	require.NoError(t, err)
	again, err := proc.SelectedOwner()
	require.NoError(t, err)
	require.True(t, owner.SameIncarnation(again))
	proc.Stop()
	join, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, proc.Wait(join))
	_, err = proc.SelectedOwner()
	require.Error(t, err)
	cleanup, recorded := proc.SelectedOwnerForCleanup()
	require.True(t, recorded)
	require.True(t, cleanup.SameIncarnation(owner))
	require.Error(t, cleanup.Current())
}

// TestProcessCannotReuseSpentIncarnation rejects a second Start without
// replacing Done, cleanup-once state or a previously captured owner.
func TestProcessCannotReuseSpentIncarnation(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{Name: "sleep", Internal: true})
	require.NoError(t, proc.StartWithContext(t.Context()))
	require.NoError(t, proc.InitConns())
	original := proc.Done()
	owner, err := proc.SelectedOwner()
	require.NoError(t, err)
	require.ErrorContains(t, proc.StartWithContext(t.Context()), "already started")
	require.Equal(t, original, proc.Done())
	proc.Stop()
	join, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, proc.Wait(join))
	require.ErrorContains(t, proc.StartWithContext(t.Context()), "already started")
	cleanup, recorded := proc.SelectedOwnerForCleanup()
	require.True(t, recorded)
	require.True(t, cleanup.SameIncarnation(owner))
	require.Equal(t, original, proc.Done())
}

// TestSelectedOwnerContextEndsBeforeEngineReceipt proves engine exit cancels
// selected flushes without turning that failure into an operator-requested Stop.
func TestSelectedOwnerContextEndsBeforeEngineReceipt(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{Name: "sleep", Internal: true})
	require.NoError(t, proc.StartWithContext(t.Context()))
	require.NoError(t, proc.InitConns())
	defer proc.Stop()
	owner, err := proc.SelectedOwner()
	require.NoError(t, err)
	lifetime, bound := owner.ProcessContext()
	require.True(t, bound)
	proc.CloseConnForTest()
	work, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	select {
	case <-proc.EngineDone():
	case <-work.Done():
		t.Fatal("test engine did not finish after its connection closed")
	}
	require.ErrorIs(t, lifetime.Err(), context.Canceled)
	require.Error(t, owner.Current())
	select {
	case <-proc.Done():
		t.Fatal("engine exit was misreported as an operator Stop")
	default:
	}
	proc.Stop()
	require.NoError(t, proc.Wait(work))
}

// TestFailedProcessStartReleasesSpentLifetime proves a failed one-shot start
// joins the delivery worker it already created, rather than leaking that life.
func TestFailedProcessStartReleasesSpentLifetime(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{Name: "source-lease-missing-runner", Internal: true})
	require.ErrorContains(t, proc.StartWithContext(t.Context()), "unknown internal plugin")
	join, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	require.NoError(t, proc.Wait(join))
	require.ErrorContains(t, proc.StartWithContext(t.Context()), "already started")
}
