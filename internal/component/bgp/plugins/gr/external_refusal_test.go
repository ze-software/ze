// Design: docs/guide/graceful-restart.md -- the retention decision is a same-process call.
package gr

import (
	"net"
	"testing"
	"time"
)

// VALIDATES: bgp-gr refuses to start when its engine runs as a forked process.
// runGRPlugin publishes the retention answer (retention.Publish) and the LLGR
// egress state (setEgressState) into its own process's memory. Forked, the
// daemon's bgp-rr and bgp-rs read every family as not retained and withdraw
// routes RFC 4724 Section 4.2 requires kept.
// PREVENTS: an operator running bgp-gr with `run` getting a daemon whose route
// reflector or route server withdraws a restarting peer's routes from every
// other peer, with no error.
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
	go func() { code <- runGRPlugin(pluginEnd) }()

	select {
	case got := <-code:
		if got != 1 {
			t.Fatalf("runGRPlugin(external conn) = %d, want 1", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runGRPlugin(external conn) did not refuse: it started the plugin protocol on a forked connection")
	}
}
