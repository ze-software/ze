// Design: docs/architecture/testing/interop.md -- exact watchdog completion receipt and session scope

package bgp

import (
	"strings"
	"testing"
)

// TestWatchdogReadyReceipt accepts one exact producer record, never a dropped
// report, another peer/process, or matching fragments spread across records.
func TestWatchdogReadyReceipt(t *testing.T) {
	const peer = "172.30.0.3"
	const credited = `time=2026-11-15T12:00:00Z level=DEBUG msg="plugin session ready credited to this peer's barrier" subsystem=bgp.routes peer=172.30.0.3 process=bgp-watchdog reported=1 expected=2`
	for _, tc := range []struct {
		name   string
		output string
		ready  bool
	}{
		{"credited", credited, true},
		{"missing", "", false},
		{"wrong-peer", strings.Replace(credited, "peer=172.30.0.3", "peer=172.30.0.30", 1), false},
		{"wrong-process", strings.Replace(credited, "process=bgp-watchdog", "process=bgp-rib", 1), false},
		{"wrong-subsystem", strings.Replace(credited, "subsystem=bgp.routes", "subsystem=bgp.fake", 1), false},
		{"missing-process", strings.Replace(credited, " process=bgp-watchdog", "", 1), false},
		{"not-credited", strings.Replace(credited, "credited to this peer's barrier", "not credited to this peer's barrier", 1), false},
		{"wrong-level", strings.Replace(credited, "level=DEBUG", "level=INFO", 1), false},
		{"malformed", `level=DEBUG msg="plugin session ready credited`, false},
		{"split-records", strings.Replace(credited, "peer=172.30.0.3", "peer=172.30.0.4", 1) + "\n" + strings.Replace(credited, "process=bgp-watchdog", "process=bgp-rib", 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := watchdogReadyReceipt(tc.output, peer); got != tc.ready {
				t.Fatalf("receipt accepted = %v, want %v", got, tc.ready)
			}
		})
	}
}

// TestWatchdogFirstSession rejects a stale credited record from a previous
// session, as well as missing counters that cannot establish session scope.
func TestWatchdogFirstSession(t *testing.T) {
	const address = "172.30.0.2"
	for _, tc := range []struct {
		name   string
		answer string
		first  bool
	}{
		{"first", `{"172.30.0.2":{"bgpState":"Established","connectionsEstablished":1,"connectionsDropped":0}}`, true},
		{"replacement", `{"172.30.0.2":{"bgpState":"Established","connectionsEstablished":2,"connectionsDropped":1}}`, false},
		{"drop", `{"172.30.0.2":{"bgpState":"Established","connectionsEstablished":1,"connectionsDropped":1}}`, false},
		{"missing-established", `{"172.30.0.2":{"bgpState":"Established","connectionsDropped":0}}`, false},
		{"missing-dropped", `{"172.30.0.2":{"bgpState":"Established","connectionsEstablished":1}}`, false},
		{"not-established", `{"172.30.0.2":{"bgpState":"Connect","connectionsEstablished":1,"connectionsDropped":0}}`, false},
		{"wrong-peer", `{"172.30.0.3":{"bgpState":"Established","connectionsEstablished":1,"connectionsDropped":0}}`, false},
		{"malformed", `{`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := watchdogFirstSession(tc.answer, address)
			if tc.first {
				if err != nil {
					t.Fatalf("first session refused: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("unproven first-session scope passed")
			}
		})
	}
}
