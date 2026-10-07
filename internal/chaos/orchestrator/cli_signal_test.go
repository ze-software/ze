// Design: docs/architecture/chaos-web-dashboard.md -- in-process run shutdown on SIGTERM

//go:build unix

package orchestrator

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestCLIRunInProcessExitsOnSIGTERM proves that `le chaos run --in-process
// --web` exits promptly on one SIGTERM once the in-process reactor is up.
//
// Method: start CLIRun in-process with the web dashboard, so the run paces at
// one real second per step. Poll the dashboard until routes have been
// announced, which means the reactor has started and installed its own
// SIGTERM handler. Then send SIGTERM to this process and require CLIRun to
// return within the budget. The defect this guards: the CLI registered for
// SIGTERM only after the run returned, so the reactor's handler took the
// signal, Ze stopped, and the run kept pacing until a second signal arrived.
//
// The test registers its own SIGTERM sink before sending, so the signal can
// never take the default action and kill the test binary whichever handlers
// are live at that instant. A signal is delivered to every registered channel,
// so the sink does not hide the signal from the code under test.
func TestCLIRunInProcessExitsOnSIGTERM(t *testing.T) {
	sink := make(chan os.Signal, 4)
	signal.Notify(sink, syscall.SIGTERM)
	defer signal.Stop(sink)

	webAddr := freeLoopbackAddr(t)

	done := make(chan int, 1)
	go func() {
		done <- CLIRun([]string{
			"--in-process",
			"--web", webAddr,
			"--duration", "30s",
			"--peers", "2",
			"--seed", "42",
			"--routes", "5",
			"--quiet",
		})
	}()

	waitRoutesAnnounced(t, webAddr, done)

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}

	// The budget covers Ze's own shutdown, which inprocess.Run bounds with a
	// 5s reactor.Wait. Measured under -race on a loaded host: 0.7s to 2.0s
	// usually, 5.5s when the bgp-rs plugin overran its 3s cleanup grace
	// (plan/journal/plugin-startup-barrier-deadlock.md). The defect this
	// guards never exits on one signal at all: it paced on until the run's
	// duration, and the functional runner's terminateGracefully SIGKILLed it
	// after its 10s grace.
	const budget = 8 * time.Second
	sent := time.Now()
	select {
	case code := <-done:
		t.Logf("CLIRun returned %d, %s after SIGTERM", code, time.Since(sent))
		if code != 0 {
			t.Fatalf("CLIRun = %d after SIGTERM, want 0", code)
		}
	case <-time.After(budget):
		// Wait on so the failure says how long the run really took.
		select {
		case <-done:
			t.Fatalf("CLIRun took %s after SIGTERM, budget %s", time.Since(sent), budget)
		case <-time.After(20 * time.Second):
			t.Fatalf("CLIRun still running %s after SIGTERM: one signal must end the in-process run", time.Since(sent))
		}
	}
}

// freeLoopbackAddr returns a loopback address whose port was free a moment
// ago. The dashboard binds it shortly after, so a collision is possible but
// would fail the dashboard start loudly rather than pass the test.
func freeLoopbackAddr(t *testing.T) string {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("release port: %v", err)
	}
	return addr
}

// waitRoutesAnnounced polls the dashboard's stats fragment until the outbound
// message counter is non-zero. A route announced means a session reached
// Established, which needs the reactor running, and the reactor installs its
// signal handler before it accepts a peer.
func waitRoutesAnnounced(t *testing.T, webAddr string, done <-chan int) {
	t.Helper()
	const zeroAnnounced = `Msgs</span><span class="stat-value">0<`
	client := &http.Client{Timeout: time.Second}
	url := "http://" + webAddr + "/sidebar/stats"
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case code := <-done:
			t.Fatalf("CLIRun returned %d before the run was observed", code)
		case <-time.After(200 * time.Millisecond):
		}
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			continue // Dashboard not listening yet.
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close() //nolint:errcheck // read already complete
		if err != nil {
			continue
		}
		page := string(body)
		if strings.Contains(page, "Msgs</span>") && !strings.Contains(page, zeroAnnounced) {
			return
		}
	}
	t.Fatal("no route announced within 30s: the in-process run never reached Established")
}
