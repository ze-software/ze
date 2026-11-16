// Design: docs/architecture/behavior/fsm-open-sent.md -- connection diagnostics preserve selected-peer state.
// Related: session_problem_diagnostic.go -- the bounded event owner.

package reactor

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bfd/api"
	_ "github.com/ze-software/ze/internal/component/bfd/yang"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/test/sim"
)

// sessionProblemLog observes production WARN output and the report bus. Its
// callers MUST run serially because the logger and report bus are shared.
func sessionProblemLog(t *testing.T) *syncBuffer {
	t.Helper()
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	return closeLogSink(t)
}

// sessionProblemEvents returns only the owner's connection-history events;
// delivered notifications keep their separate existing wire-exchange reports.
func sessionProblemEvents(code string) []report.Issue {
	var events []report.Issue
	for _, event := range report.Errors(0) {
		if event.Code == code {
			events = append(events, event)
		}
	}
	return events
}

// TestOpenConnectionRefusalDiagnostic drives both real OPEN rails. An
// unselected invalid connection must still reach doctor while the incumbent's
// last reason and notification-exchanged flag remain intact. A valid OPEN is
// the control: it advances with no refusal diagnostic.
func TestOpenConnectionRefusalDiagnostic(t *testing.T) {
	for _, replay := range []bool{false, true} {
		for _, selected := range []bool{false, true} {
			for _, scenario := range []struct {
				name       string
				asn        uint16
				identifier uint32
				want       error
			}{
				{"AS mismatch", 65003, 0x01020302, ErrPeerASMismatch},
				{"bad identifier", 65002, 0, ErrBadBGPIdentifier},
				{"valid OPEN", 65002, 0x01020302, nil},
			} {
				rail := "handleOpen"
				if replay {
					rail = "processOpen"
				}
				owner := "unselected"
				if selected {
					owner = "selected"
				}
				t.Run(rail+"/"+owner+"/"+scenario.name, func(t *testing.T) {
					runOpenConnectionRefusalDiagnostic(t, replay, selected, scenario.asn, scenario.identifier, scenario.want)
				})
			}
		}
	}
}

// runOpenConnectionRefusalDiagnostic checks one OPEN rail and ownership case.
// Its sockets and timers belong to this subtest and are retired in cleanup.
func runOpenConnectionRefusalDiagnostic(t *testing.T, replay, selected bool, asn uint16, identifier uint32, want error) {
	t.Helper()
	sink := sessionProblemLog(t)
	session, _ := openSentSessionAS(t, 65002, 0x01020301)
	t.Cleanup(session.stopSessionTimers)
	peer := NewPeer(session.settings)
	incumbent := NewSession(session.settings)
	peer.session = incumbent
	if selected {
		peer.session = session
	}
	beforeSession := peer.session
	peer.recordNotification(6, 2, plugin.NotifReceived)
	before := peer.Stats()
	peer.attachStatsCallbacks(session)
	var err error
	if replay {
		err = session.processOpen(&message.Open{Version: 4, MyAS: asn,
			HoldTime: 90, BGPIdentifier: identifier})
	} else {
		err = session.handleOpen(openBodyWithIdentifier(asn, identifier))
	}
	require.Same(t, beforeSession, peer.session, "diagnosis cannot select or retire the incumbent")
	events := sessionProblemEvents(codeOpenConnectionRefused)
	logs := closeRecordsWith(closeLogRecords(t, sink), "code", codeOpenConnectionRefused)
	if want == nil {
		require.NoError(t, err)
		require.Equal(t, fsm.StateOpenConfirm, session.State())
		require.Empty(t, events)
		require.Empty(t, logs)
		return
	}
	require.ErrorIs(t, err, want)
	require.Nil(t, session.Conn())
	require.Len(t, events, 1)
	require.Equal(t, reportSourceBGP, events[0].Source)
	require.Equal(t, session.settings.Address.String(), events[0].Subject)
	require.Equal(t, openConnectionRefusalFix, events[0].Fix)
	require.NotEmpty(t, events[0].Detail)
	require.Len(t, logs, 1)
	require.Equal(t, "WARN", logs[0]["level"])
	require.Equal(t, events[0].Fix, logs[0]["fix"])
	if selected {
		require.True(t, peer.notificationExchanged.Load())
		require.Equal(t, before.NotificationsSent+1, peer.Stats().NotificationsSent)
		return
	}
	after := peer.Stats()
	require.Equal(t, before.LastNotifCode, after.LastNotifCode)
	require.Equal(t, before.LastNotifSubcode, after.LastNotifSubcode)
	require.Equal(t, before.LastNotifDirection, after.LastNotifDirection)
	require.Equal(t, before.LastNotifTime, after.LastNotifTime)
	require.Equal(t, before.NotificationsSent, after.NotificationsSent)
	require.False(t, peer.notificationExchanged.Load())
	require.Empty(t, sessionProblemEvents(reportCodeSessionDropped))
	require.Empty(t, sessionProblemEvents(reportCodeNotificationSent))
}

// TestOpenConnectionRefusalDiagnosticSurvivesUnsentNotification proves the
// rejection cause reaches the report bus even when the NOTIFICATION write
// fails. Failed delivery remains distinct from a notification actually sent.
func TestOpenConnectionRefusalDiagnosticSurvivesUnsentNotification(t *testing.T) {
	sink := sessionProblemLog(t)
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	session := NewSession(settings)
	t.Cleanup(session.stopSessionTimers)
	require.NoError(t, session.Start())
	conn := &pendingRefusalConn{}
	require.NoError(t, session.Accept(conn))
	peer := NewPeer(settings)
	peer.session = session
	peer.attachStatsCallbacks(session)
	conn.writeErr = errors.New("peer stopped reading")
	err := session.handleOpen(openBodyWithIdentifier(65003, 0x01020302))
	require.ErrorIs(t, err, ErrPeerASMismatch)
	require.True(t, conn.closed)
	require.Len(t, sessionProblemEvents(codeOpenConnectionRefused), 1)
	require.Len(t, closeRecordsWith(closeLogRecords(t, sink), "code", codeOpenConnectionRefused), 1)
	require.Equal(t, plugin.NotifSendFailed, peer.Stats().LastNotifDirection)
	require.Zero(t, peer.Stats().NotificationsSent)
	require.Empty(t, sessionProblemEvents(reportCodeNotificationSent))
}

// TestBFDWaitHoldExpiryDiagnostic expires the actual BGP timer in each strict
// wait. A plain BGP expiry is the control and must not claim a BFD wait. The
// terminal seal also prevents a second history event for one expiry.
func TestBFDWaitHoldExpiryDiagnostic(t *testing.T) {
	for _, wait := range []string{"pending", "hold-down", "ordinary"} {
		t.Run(wait, func(t *testing.T) {
			sink := sessionProblemLog(t)
			fc := sim.NewFakeClock(time.Now())
			state := api.StateDown
			var holdDown uint32
			if wait == "hold-down" {
				state = api.StateUp
				holdDown = 40000
			}
			session, client, _ := newStrictSessionOn(t, 90*time.Second, holdDown, state, true, fc)
			t.Cleanup(session.stopSessionTimers)
			messages := drainMessages(t, client)
			body := openBodyWithBFDStrict(30)
			if wait == "ordinary" {
				body = openBodyWithIdentifier(65002, 0x01020302)
				body[3], body[4] = 0, 30
			}
			require.NoError(t, session.handleOpen(body))
			if wait == "hold-down" {
				require.NoError(t, session.handleKeepalive())
				require.True(t, session.timers.IsBfdHoldDownTimerRunning())
			}
			fc.Add(30 * time.Second)
			require.Equal(t, fsm.StateIdle, session.State())
			select {
			case err := <-session.errChan:
				require.ErrorIs(t, err, ErrHoldTimerExpired)
			default:
				t.Fatal("hold expiry must signal its cancellation owner")
			}
			// The callback already returned before inspecting the shared sinks.
			session.openMu.Lock()
			session.holdTimerExpiredLocked()
			session.unlockOpen()
			events := sessionProblemEvents(codeBFDWaitHoldExpired)
			logs := closeRecordsWith(closeLogRecords(t, sink), "code", codeBFDWaitHoldExpired)
			if wait == "ordinary" {
				require.Empty(t, events)
				require.Empty(t, logs)
				return
			}
			require.Len(t, events, 1)
			require.Equal(t, session.settings.Address.String(), events[0].Subject)
			require.Equal(t, bfdWaitHoldExpiryFix, events[0].Fix)
			require.Equal(t, 30*time.Second, events[0].Detail["hold-time"])
			require.Len(t, logs, 1)
			require.Equal(t, "WARN", logs[0]["level"])
			require.Equal(t, events[0].Fix, logs[0]["fix"])
			deadline := time.NewTimer(2 * time.Second)
			defer deadline.Stop()
			var notification bool
			for !notification {
				select {
				case msg := <-messages:
					if len(msg) >= 21 {
						if msg[18] == 3 {
							notification = msg[19] == byte(message.NotifyHoldTimerExpired)
						}
					}
				case <-deadline.C:
					t.Fatal("diagnosis must retain the existing hold-expiry notification")
				}
			}
		})
	}
}

// TestSessionProblemDiagnosticRegistration checks the actual metadata/event
// registry and both exact next-step commands against their owning YANG paths.
func TestSessionProblemDiagnosticRegistration(t *testing.T) {
	for _, code := range []string{codeOpenConnectionRefused, codeBFDWaitHoldExpired} {
		require.NotNil(t, diagnostic.Lookup(code))
		require.Contains(t, diagnostic.EventCodes(), code)
	}
	schema, err := yang.DefaultLoader()
	require.NoError(t, err)
	paths := yang.WireMethodToPaths(schema)
	args := yang.PathToArgDefs(schema)
	for method, fix := range map[string]string{
		"ze-bgp:peer-list":     openConnectionRefusalFix,
		"ze-bfd:show-sessions": bfdWaitHoldExpiryFix,
	} {
		path := strings.TrimPrefix(fix, "ze ")
		require.Contains(t, paths[method], path)
		require.Empty(t, args[path], "the next step must omit no required selector")
		require.LessOrEqual(t, len(fix), report.FixMax)
	}
}
