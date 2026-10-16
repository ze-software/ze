// Tests for releaseForceSignals (drain.go) through runYANGConfig (main.go).
// VALIDATES: the stop's force-exit watcher (stopSignals.run over forceCh) ends
// when runYANGConfig returns, and a SIGTERM delivered to the process afterwards
// does not reach the force exit.
// PREVENTS: a watcher that outlives runYANGConfig and turns a later signal to
// the same process (a test binary, an embedding caller) into os.Exit(1).

package hub

import (
	"bytes"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/env"
)

// stopWatcherFrame is the stack frame of the stop's force-exit watcher.
const stopWatcherFrame = "hub.(*stopSignals).run("

// stopWatcherRunning reports whether any goroutine is in stopSignals.run.
func stopWatcherRunning() bool {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return bytes.Contains(buf[:n], []byte(stopWatcherFrame))
		}
		buf = make([]byte, 2*len(buf))
	}
}

// awaitSignal waits for one signal on sink until deadline.
func awaitSignal(t *testing.T, sink <-chan os.Signal, deadline time.Time, what string) {
	t.Helper()
	select {
	case <-sink:
	case <-time.After(time.Until(deadline)):
		t.Fatalf("%s: SIGTERM never reached the test's own handler", what)
	}
}

// TestStopReleasesForceSignalsOnReturn boots runYANGConfig over an empty
// config, stops it with SIGTERM once it is ready, and after it returns checks
// that no stopSignals.run goroutine is left and that a second SIGTERM never
// calls the force exit (stopForceExit, replaced here by a recorder). A test
// sink holds SIGTERM throughout, so no signal takes the default action and
// kills the test binary.
func TestStopReleasesForceSignalsOnReturn(t *testing.T) {
	resetAAABundleForTest(t)
	clearAPIEnv(t)
	require.False(t, stopWatcherRunning(), "a stop watcher from an earlier run is still running")

	forced := make(chan struct{}, 4)
	originalForceExit := stopForceExit
	stopForceExit = func() { forced <- struct{}{} }
	t.Cleanup(func() { stopForceExit = originalForceExit })

	readyFile := filepath.Join(t.TempDir(), "ready")
	originalConfigDir := env.Get("ze.config.dir")
	originalReadyFile := env.Get("ze.ready.file")
	require.NoError(t, env.Set("ze.config.dir", t.TempDir()))
	require.NoError(t, env.Set("ze.ready.file", readyFile))
	t.Cleanup(func() {
		require.NoError(t, env.Set("ze.config.dir", originalConfigDir))
		require.NoError(t, env.Set("ze.ready.file", originalReadyFile))
	})
	store := newTestStore(t)

	sink := make(chan os.Signal, 4)
	signal.Notify(sink, syscall.SIGTERM)
	t.Cleanup(func() { signal.Stop(sink) })

	// One deadline for every wait: boot writes the store and each write
	// fsyncs, so the bound is the test's own (storeWriteBound).
	deadline := time.Now().Add(storeWriteBound(t))
	exitResult := make(chan int, 1)
	go func() {
		exitResult <- runYANGConfig(store, "-", []byte(""), nil, 0, -1, false, false, "", false, "", "", false, nil)
	}()
	// Join: runYANGConfig returns before the test does on every path. Until
	// the ready file exists its stop handler may not be installed, so a
	// failure before then leaves the boot to finish on its own and waits.
	exited := false
	t.Cleanup(func() {
		if exited {
			return
		}
		if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
			t.Errorf("stop runYANGConfig: %v", err)
			return
		}
		select {
		case <-exitResult:
		case <-time.After(time.Until(deadline)):
			t.Error("runYANGConfig did not return after SIGTERM")
		}
	})

	// The ready file is written once runYANGConfig handles its signals.
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for ready := false; !ready; {
		select {
		case exit := <-exitResult:
			exited = true
			t.Fatalf("runYANGConfig exited with %d before it was ready", exit)
		case <-tick.C:
			_, err := os.Stat(readyFile)
			ready = err == nil
		case <-time.After(time.Until(deadline)):
			t.Fatal("runYANGConfig never wrote its ready file")
		}
	}

	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGTERM))
	awaitSignal(t, sink, deadline, "stop")
	select {
	case exit := <-exitResult:
		exited = true
		require.Zero(t, exit)
	case <-time.After(time.Until(deadline)):
		t.Fatal("runYANGConfig did not return after SIGTERM")
	}

	// The watcher's channel is closed before runYANGConfig returns, so the
	// goroutine ends within a scheduling round; 5s is room for a loaded host.
	watcherBound := time.Now().Add(5 * time.Second)
	for stopWatcherRunning() && time.Now().Before(watcherBound) {
		<-tick.C
	}
	if stopWatcherRunning() {
		t.Error("stopSignals.run is still running after runYANGConfig returned")
	}

	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGTERM))
	awaitSignal(t, sink, deadline, "after return")
	// Absence cannot be waited on, so the watcher gets a window to act on the
	// signal the sink already saw: a live watcher calls the recorder within
	// microseconds of the delivery.
	select {
	case <-forced:
		t.Error("a SIGTERM after runYANGConfig returned reached the force exit")
	case <-time.After(time.Second):
	}
}
