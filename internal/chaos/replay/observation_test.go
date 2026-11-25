// Design: docs/architecture/chaos-web-dashboard.md -- replay the pre-shutdown observation

package replay

import (
	"bytes"
	"net/netip"
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
