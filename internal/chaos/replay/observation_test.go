// Design: docs/architecture/chaos-web-dashboard.md -- replay the pre-shutdown observation

package replay

import (
	"bytes"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/chaos/peer"
	"github.com/ze-software/ze/internal/chaos/report"
)

// VALIDATES: A replay keeps a completed deadline violation after teardown.
// PREVENTS: Replaying a failed capture as a pass.
func TestReplayKeepsLateDelivery(t *testing.T) {
	var input, output bytes.Buffer
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	log := report.NewJSONLog(&input, report.JSONLogConfig{
		Start: start, Peers: 2, ConvergenceDeadline: time.Second,
	})
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	for index := range 2 {
		log.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: index, Time: start})
		log.ProcessEvent(peer.Event{Type: peer.EventEORSent, PeerIndex: index, Time: start})
	}
	log.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: start, Prefix: prefix})
	log.ProcessEvent(peer.Event{
		Type: peer.EventRouteReceived, PeerIndex: 1, Time: start.Add(time.Second + time.Nanosecond), Prefix: prefix,
	})
	end := start.Add(2 * time.Second)
	log.EndObservation(end)
	log.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 0, Time: end, PlannedShutdown: true})
	log.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 1, Time: end, PlannedShutdown: true})

	assert.Equal(t, 1, Run(&input, &output))
	assert.Contains(t, output.String(), "1 announced, 1 received, 0 missing, 0 extra, 1 slow")
}

// VALIDATES: The recorded cutoff is required for a current-format replay.
// PREVENTS: A partial capture being reported as a full observation.
func TestReplayRequiresObservationEnd(t *testing.T) {
	var input, output bytes.Buffer
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	log := report.NewJSONLog(&input, report.JSONLogConfig{
		Start: start, Peers: 1, ConvergenceDeadline: time.Second,
	})
	log.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: start})
	assert.Equal(t, 2, Run(&input, &output))
	assert.Contains(t, output.String(), "ended before the observation checkpoint")
}

// VALIDATES: A receipt clears only an expectation for the same address family.
// PREVENTS: A route in the wrong family satisfying final state or its deadline.
func TestReplayDistinguishesRouteFamilies(t *testing.T) {
	for _, tc := range []struct {
		receivedFamily string
		status         int
		routes         string
	}{
		{"ipv4/unicast", 0, "0 missing, 0 extra"},
		{"ipv4/multicast", 1, "1 missing, 1 extra, 1 slow"},
	} {
		t.Run(tc.receivedFamily, func(t *testing.T) {
			var input, output bytes.Buffer
			start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			log := report.NewJSONLog(&input, report.JSONLogConfig{
				Start: start, Peers: 2, ConvergenceDeadline: time.Second,
			})
			for index := range 2 {
				log.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: index, Time: start})
				log.ProcessEvent(peer.Event{Type: peer.EventEORSent, PeerIndex: index, Time: start})
			}
			prefix := netip.MustParsePrefix("239.1.2.0/24")
			log.ProcessEvent(peer.Event{
				Type: peer.EventRouteSent, PeerIndex: 0, Time: start, Prefix: prefix, Family: "ipv4/unicast",
			})
			log.ProcessEvent(peer.Event{
				Type: peer.EventRouteReceived, PeerIndex: 1, Time: start.Add(time.Millisecond), Prefix: prefix, Family: tc.receivedFamily,
			})
			log.EndObservation(start.Add(2 * time.Second))

			assert.Equal(t, tc.status, Run(&input, &output))
			assert.Contains(t, output.String(), tc.routes)
		})
	}
}

// VALIDATES: Withdrawals preserve a delivery deadline that has already expired.
// PREVENTS: A source withdrawing an undelivered route erasing the failure.
func TestReplayKeepsOverdueWithdrawal(t *testing.T) {
	for _, tc := range []struct {
		name     string
		withdraw time.Duration
		status   int
		routes   string
	}{
		{"before-deadline", time.Millisecond, 0, "0 missing, 0 extra"},
		{"after-deadline", 2 * time.Second, 1, "0 missing, 0 extra, 1 slow"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var input, output bytes.Buffer
			start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			log := report.NewJSONLog(&input, report.JSONLogConfig{
				Start: start, Peers: 2, ConvergenceDeadline: time.Second,
			})
			for index := range 2 {
				log.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: index, Time: start})
				log.ProcessEvent(peer.Event{Type: peer.EventEORSent, PeerIndex: index, Time: start})
			}
			withdrawn := netip.MustParsePrefix("192.0.2.0/24")
			delivered := netip.MustParsePrefix("198.51.100.0/24")
			log.ProcessEvent(peer.Event{
				Type: peer.EventRouteSent, PeerIndex: 0, Time: start, Prefix: withdrawn, Family: "ipv4/unicast",
			})
			log.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: start, Prefix: delivered})
			log.ProcessEvent(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 1, Time: start, Prefix: delivered})
			log.ProcessEvent(peer.Event{
				Type: peer.EventWithdrawalSent, PeerIndex: 0, Time: start.Add(tc.withdraw), WithdrawnPrefixes: []netip.Prefix{withdrawn},
			})
			log.EndObservation(start.Add(3 * time.Second))

			assert.Equal(t, tc.status, Run(&input, &output))
			assert.Contains(t, output.String(), tc.routes)
		})
	}
}

// VALIDATES: Current-format replay checks every sequence number, including controls and suffixes.
// PREVENTS: An incomplete or reordered capture receiving a routing verdict.
func TestReplayRequiresContinuousSequence(t *testing.T) {
	const complete = `{"record-type":"header","version":2,"peers":1,"start-time":"2026-01-01T00:00:00Z","convergence-deadline-ns":1000000000}
{"record-type":"event","seq":1,"time-offset-ns":0,"event-type":"established","peer-index":0}
{"record-type":"event","seq":2,"time-offset-ns":0,"event-type":"route-sent","peer-index":0,"prefix":"192.0.2.0/24"}
{"record-type":"control","seq":3,"time-offset-ms":0,"command":"pause"}
{"record-type":"event","seq":4,"time-offset-ns":0,"event-type":"eor-sent","peer-index":0}
{"record-type":"observation-end","seq":5,"time-offset-ns":10000000}
{"record-type":"event","seq":6,"time-offset-ns":10000000,"event-type":"disconnected","peer-index":0,"planned-shutdown":true}
`
	for _, tc := range []struct {
		name   string
		input  string
		status int
	}{
		{"complete", complete, 0},
		{"missing-middle", strings.Replace(complete, `{"record-type":"control","seq":3,"time-offset-ms":0,"command":"pause"}`+"\n", "", 1), 2},
		{"duplicate", strings.Replace(complete, `"seq":3`, `"seq":2`, 1), 2},
		{"out-of-order", strings.Replace(complete, `"seq":3`, `"seq":1`, 1), 2},
		{"gap-after-cutoff", strings.Replace(complete, `"seq":6`, `"seq":7`, 1), 2},
		{"unknown-record", strings.Replace(complete, `"control"`, `"unknown"`, 1), 2},
		{"unknown-event", strings.Replace(complete, `"eor-sent"`, `"unknown"`, 1), 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			assert.Equal(t, tc.status, Run(strings.NewReader(tc.input), &output), output.String())
		})
	}
}
