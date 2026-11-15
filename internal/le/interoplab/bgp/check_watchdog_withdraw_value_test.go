// Design: docs/architecture/testing/interop.md -- fail-closed initial synchronization fence

package bgp

import "testing"

// TestWatchdogInitialEORFence distinguishes FRR's received IPv4 marker from a
// sent marker, absent state, malformed JSON and values of the wrong JSON type.
func TestWatchdogInitialEORFence(t *testing.T) {
	const address = "172.30.0.2"
	for _, tc := range []struct {
		name   string
		answer string
		ready  bool
	}{
		{"received", `{"172.30.0.2":{"bgpState":"Established","connectionsEstablished":1,"gracefulRestartInfo":{"endOfRibRecv":{"ipv4Unicast":true}}}}`, true},
		{"sent-only", `{"172.30.0.2":{"bgpState":"Established","connectionsEstablished":1,"gracefulRestartInfo":{"endOfRibSend":{"ipv4Unicast":true},"endOfRibRecv":{}}}}`, false},
		{"received-false", `{"172.30.0.2":{"bgpState":"Established","connectionsEstablished":1,"gracefulRestartInfo":{"endOfRibRecv":{"ipv4Unicast":false}}}}`, false},
		{"missing-gr", `{"172.30.0.2":{"bgpState":"Established","connectionsEstablished":1}}`, false},
		{"missing-received", `{"172.30.0.2":{"bgpState":"Established","connectionsEstablished":1,"gracefulRestartInfo":{}}}`, false},
		{"string-flag", `{"172.30.0.2":{"bgpState":"Established","connectionsEstablished":1,"gracefulRestartInfo":{"endOfRibRecv":{"ipv4Unicast":"true"}}}}`, false},
		{"number-flag", `{"172.30.0.2":{"bgpState":"Established","connectionsEstablished":1,"gracefulRestartInfo":{"endOfRibRecv":{"ipv4Unicast":1}}}}`, false},
		{"null-flag", `{"172.30.0.2":{"bgpState":"Established","connectionsEstablished":1,"gracefulRestartInfo":{"endOfRibRecv":{"ipv4Unicast":null}}}}`, false},
		{"not-established", `{"172.30.0.2":{"bgpState":"Connect","connectionsEstablished":1,"gracefulRestartInfo":{"endOfRibRecv":{"ipv4Unicast":true}}}}`, false},
		{"no-neighbor", `{}`, false},
		{"malformed", `{`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := watchdogInitialEOR(tc.answer, address)
			if tc.ready {
				if err != nil {
					t.Fatalf("received End-of-RIB refused: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("unproven initial synchronization passed the fence")
			}
		})
	}
}
