// Design: docs/architecture/l2tp/subscriber-session-model.md -- original assignment refusal retains its cause.
package l2tp

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// The genuine Driver prepares a real original receipt and waits for its
// assignment ACK. Corrupt only the copied notice's epoch, not its Control or
// producer receipt. A malformed notice must be canceled before that ACK, leave
// no address state or publication and retain an actionable doctor warning.
// The useful original notice completes actual IPCP and answers LCP Echo.
// MUTATION: omit the coded retained report in handleSessionIPAssigned.
func TestAssignmentWithoutOriginalCounterReceiptRefusesAndReports(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()
	const subject = "subscriber network assignment"
	const tunnelID uint16 = 64127
	const sessionID uint16 = 64128
	for _, mode := range []string{"missing lifetime", "wrong lifetime", "useful original"} {
		t.Run(mode, func(t *testing.T) {
			preserveSessionBirthDiagnostic(t, subject, tunnelID, sessionID)
			previous, previousPresent := sessionBirthDiagnostic(subject)
			reactor := newReactorForSnapshot(t)
			bus := newTestBus()
			reactor.SetEventBus(bus)
			session := &L2TPSession{localSID: sessionID, state: L2TPSessionEstablished, pppInterface: "ppp-diagnostic"}
			insertEstablishedTunnel(t, reactor, tunnelID, 200, netip.MustParseAddrPort("192.0.2.1:1701"), session)
			wire := newCurrentCoreAdmission(t, reactor, &currentCoreAdmissionConfig{
				ids: currentCoreSessionIDs{tunnelID: tunnelID, sessionID: sessionID},
			})
			reactor.handlePPPEvent(wire.opened)
			wire.prepareNetwork(t, "counter-diagnostic", ppp.IPResponseArgs{
				Accept: true, Family: ppp.AddressFamilyIPv4,
				Local: netip.MustParseAddr("192.0.2.1"), Peer: netip.MustParseAddr("192.0.2.42"),
			}, [8]byte{})
			assignment := wire.assign(t, ppp.AddressFamilyIPv4)
			receipt, err := assignment.Control.CaptureNetworkCounters(assignment.NetworkLifetime)
			require.NoError(t, err)
			require.True(t, receipt.IsValid(), "the real producer must have prepared an original receipt")
			require.True(t, assignment.Control.Active())
			switch mode {
			case "missing lifetime":
				assignment.NetworkLifetime = l2tpevents.NetworkLifetime{}
			case "wrong lifetime":
				assignment.NetworkLifetime, err = assignment.NetworkLifetime.Next()
				require.NoError(t, err)
			}
			var logs bytes.Buffer
			reactor.logger = slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))
			publications := 0
			t.Cleanup(l2tpevents.SessionIPAssigned.Subscribe(bus, func(event *l2tpevents.SessionIPAssignedPayload) {
				if event.Birth == session.Birth() {
					publications++
				}
			}))
			reactor.handleSessionIPAssigned(assignment)
			select {
			case <-assignment.Acknowledged():
			default:
				t.Fatal("the consumer omitted the original assignment acknowledgement")
			}
			if mode == "useful original" {
				up := wire.nextUp(t)
				require.Equal(t, assignment.Control, up.Control)
				require.Equal(t, 1, publications)
				require.True(t, session.assignedAddr.IsValid())
				require.NotNil(t, session.timeoutLifetime)
				actorRouteEcho(t, wire)
				require.Empty(t, logs.String(), "the useful original is not a warning")
				current, currentPresent := sessionBirthDiagnostic(subject)
				require.Equal(t, previousPresent, currentPresent)
				require.Equal(t, previous, current, "healthy assignment must not clear an earlier defect")
				return
			}
			// The next actual lifecycle result must be Down, rather than a
			// time-only assertion that Up has not happened yet.
			event := wire.event(t)
			require.IsType(t, ppp.EventSessionDown{}, event, "the refused assignment must not enable SessionUp")
			down := event.(ppp.EventSessionDown)
			require.Equal(t, assignment.Control, down.Control)
			require.False(t, assignment.Control.Active())
			require.Zero(t, publications)
			require.False(t, session.assignedAddr.IsValid())
			require.Nil(t, session.timeoutLifetime)
			var warning map[string]any
			require.NoError(t, json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &warning))
			require.Equal(t, diagnostic.CodeDoctorInternalDefect, warning["code"])
			require.NotNil(t, diagnostic.Lookup(diagnostic.CodeDoctorInternalDefect))
			require.Equal(t, "ze show doctor", warning["fix"])
			require.Contains(t, warning["msg"], "forwarding remains disabled")
			finding, present := sessionBirthDiagnostic(subject)
			require.True(t, present, "runtime doctor must retain this original refusal")
			require.Equal(t, tunnelID, finding.Detail["tunnel-id"])
			require.Equal(t, sessionID, finding.Detail["session-id"])
			require.Equal(t, assignment.SubscriberID, finding.Detail["subscriber-id"])
			require.NotEmpty(t, finding.Detail["cause"])
			require.Equal(t, "ze show doctor", finding.Fix)
		})
	}
}
