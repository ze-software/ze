// Design: docs/guide/graceful-restart.md -- the retention decision is a same-process call.
package rs

import (
	"net"
	"testing"
	"time"
)

// VALIDATES: bgp-rs refuses to start when its engine runs as a forked process.
// handleStateDown asks retention.Family, a same-process call that a forked
// route server answers with "not retained" for every family, so it would withdraw
// routes RFC 4724 Section 4.2 requires kept.
// PREVENTS: an operator running bgp-rs with `run` getting a route server that
// withdraws a restarting peer's routes from every other peer.
//
// Method: a plain net.Pipe end carries no DirectBridge, which is exactly what
// a forked plugin's connection looks like to the SDK (sdk.Plugin.IsInternal).
// Without the refusal the engine waits on the pipe for the startup protocol,
// so the bounded wait below reports it instead of hanging the package.
func TestRefusesExternalProcess(t *testing.T) {
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() {
		pluginEnd.Close() //nolint:errcheck // test cleanup
		engineEnd.Close() //nolint:errcheck // test cleanup
	})

	code := make(chan int, 1)
	go func() { code <- RunRouteServer(pluginEnd) }()

	select {
	case got := <-code:
		if got != 1 {
			t.Fatalf("RunRouteServer(external conn) = %d, want 1", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunRouteServer(external conn) did not refuse: it started the plugin protocol on a forked connection")
	}
}
