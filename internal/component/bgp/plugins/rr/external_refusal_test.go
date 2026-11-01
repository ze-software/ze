// Design: docs/guide/graceful-restart.md -- the forked refusal kept until its forked tests land.
package rr

import (
	"net"
	"testing"
	"time"
)

// VALIDATES: bgp-rr refuses to start when its engine runs as a forked process.
// The refusal stands until a forked route reflector is proven to keep the routes
// RFC 4724 Section 4.2 requires kept; the verdict itself now arrives as the
// bgp-gr/retention event, which crosses the process split.
// PREVENTS: an operator running bgp-rr with `run` getting a route reflector that
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
	go func() { code <- runRouteReflector(pluginEnd) }()

	select {
	case got := <-code:
		if got != 1 {
			t.Fatalf("runRouteReflector(external conn) = %d, want 1", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runRouteReflector(external conn) did not refuse: it started the plugin protocol on a forked connection")
	}
}
