package process

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// These tests each drive one producer line of a doctor record through the
// function that holds it, and assert the record the doctor snapshot reads, so
// deleting that line turns its test red (spec-doctor-coverage-12-plugin-infra,
// R-10).

// TestSafeDeliverBatchPanicRecordsProjectionError delivers a batch whose send
// panics.
//
// VALIDATES: safeDeliverBatch's recovery answers the waiting caller with the
// panic and records it as the projection error the doctor reads.
// PREVENTS: a delivery panic that stops a plugin applying events reaching
// only the log.
// METHOD: a process given a connection but never started has no lifecycle
// context, so deliverViaConn's context.WithTimeout panics: the shape of the
// Ze defect the recovery exists to catch. A bridge call would not do, because
// safeBridgeCall recovers its own panics before safeDeliverBatch sees them.
func TestSafeDeliverBatchPanicRecordsProjectionError(t *testing.T) {
	engineEnd, pluginEnd := net.Pipe()
	engineMux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	t.Cleanup(func() {
		engineMux.Close() //nolint:errcheck // test cleanup
		engineEnd.Close() //nolint:errcheck // test cleanup
		pluginEnd.Close() //nolint:errcheck // test cleanup
	})
	p := NewProcess(plugin.PluginConfig{Name: "delivery-panic", Encoder: "json"})
	p.SetConn(ipc.NewMuxPluginConn(engineMux))

	waiting := make(chan EventResult, 1)
	p.safeDeliverBatch([]EventDelivery{{Output: "event one", Result: waiting}}, nil, time.Second)

	select {
	case result := <-waiting:
		require.ErrorContains(t, result.Err, "delivery panic", "the waiting caller was not told of the panic")
	default:
		t.Fatal("the recovery did not answer the waiting caller")
	}
	require.Contains(t, p.DoctorRecord(time.Now()).ProjectionError, "delivery panic",
		"the recovered panic was not recorded as the projection error")
}

// TestStopKeepsTheEnginesThatMissedTheirGrace stops a manager holding one
// engine that returns once its connection closes and one that never does.
//
// VALIDATES: ProcessManager.Stop stores the late engine, and only it, where
// CleanupLate reads it; nothing is stored before a Stop.
// PREVENTS: a lost plugin cleanup the server can never write to the last-stop
// record, because the manager only logged it.
// METHOD: the pluginStopGrace seam at 500ms, wide enough that the prompt
// engine returns inside it on a loaded machine.
func TestStopKeepsTheEnginesThatMissedTheirGrace(t *testing.T) {
	stuck := make(chan struct{})
	t.Cleanup(func() { close(stuck) })
	registerRunEngine(t, "test-cleanup-late-stuck", func(conn net.Conn) int {
		drainUntilClosed(conn)
		<-stuck
		return 0
	})
	registerRunEngine(t, "test-cleanup-late-prompt", func(conn net.Conn) int {
		drainUntilClosed(conn)
		return 0
	})
	origGrace := pluginStopGrace
	pluginStopGrace = 500 * time.Millisecond
	t.Cleanup(func() { pluginStopGrace = origGrace })

	pm := NewProcessManager([]plugin.PluginConfig{
		{Name: "test-cleanup-late-stuck", Internal: true, Encoder: "json"},
		{Name: "test-cleanup-late-prompt", Internal: true, Encoder: "json"},
	})
	require.NoError(t, pm.Start())
	require.Empty(t, pm.CleanupLate(), "a late engine was named before any Stop")

	pm.Stop()

	require.Equal(t, []string{"test-cleanup-late-stuck"}, pm.CleanupLate(),
		"Stop did not keep exactly the engine that missed its grace")
}
