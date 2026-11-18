// Design: docs/architecture/l2tp/l2tp-9-observer.md -- independent session history
// Related: observer.go -- typed lifecycle subscriptions and event rings

package l2tp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// TestObserverKeepsSameSessionIDOnTwoTunnelsApart drives the real typed
// lifecycle subscriptions and checks that equal wire IDs get separate rings.
func TestObserverKeepsSameSessionIDOnTwoTunnelsApart(t *testing.T) {
	bus := newTestBus()
	observer := newObserver(testObserverConfig())
	subsystem := &Subsystem{observer: observer}
	subsystem.wireObserverSubscriptions(bus)
	t.Cleanup(observer.Stop)

	_, err := l2tpevents.SessionUp.Emit(bus, &l2tpevents.SessionUpPayload{TunnelID: 101, SessionID: 7})
	require.NoError(t, err)
	_, err = l2tpevents.SessionUp.Emit(bus, &l2tpevents.SessionUpPayload{TunnelID: 202, SessionID: 7})
	require.NoError(t, err)

	summaries := observer.SessionSummaries()
	require.Len(t, summaries, 2, "two tunnels' subscribers must not share one event ring")
	for _, summary := range summaries {
		require.Equal(t, uint16(7), summary.SessionID)
		require.Equal(t, 1, summary.EventCount, "each ring must contain only its own session-up")
	}
}

// TestObserverSessionDownKeepsOtherTunnelHistory checks through the event bus
// that ending one session returns only its ring, preserving the other session's
// own session-up and echo history without events from the departed subscriber.
func TestObserverSessionDownKeepsOtherTunnelHistory(t *testing.T) {
	bus := newTestBus()
	observer := newObserver(testObserverConfig())
	subsystem := &Subsystem{observer: observer}
	subsystem.wireObserverSubscriptions(bus)
	t.Cleanup(observer.Stop)

	_, err := l2tpevents.SessionUp.Emit(bus, &l2tpevents.SessionUpPayload{TunnelID: 101, SessionID: 7})
	require.NoError(t, err)
	_, err = l2tpevents.SessionUp.Emit(bus, &l2tpevents.SessionUpPayload{TunnelID: 202, SessionID: 7})
	require.NoError(t, err)
	_, err = l2tpevents.EchoRTT.Emit(bus, &l2tpevents.EchoRTTPayload{
		TunnelID: 202, SessionID: 7, RTT: 13 * time.Millisecond,
	})
	require.NoError(t, err)
	_, err = l2tpevents.SessionDown.Emit(bus, &l2tpevents.SessionDownPayload{TunnelID: 101, SessionID: 7})
	require.NoError(t, err)

	summaries := observer.SessionSummaries()
	require.Len(t, summaries, 1, "ending one tunnel's session must retain the other tunnel's event ring")
	require.Equal(t, uint16(7), summaries[0].SessionID)
	require.Equal(t, 2, summaries[0].EventCount)
	history := observer.SessionEvents(7)
	require.Len(t, history, 2)
	require.Equal(t, ObserverEventSessionUp, history[0].Type)
	require.Equal(t, ObserverEventEchoRTT, history[1].Type)
	for _, event := range history {
		require.Equal(t, uint16(202), event.TunnelID, "the survivor's ring must not contain another tunnel's events")
		require.Equal(t, uint16(7), event.SessionID)
	}
	require.Equal(t, 13*time.Millisecond, history[1].RTT)
}
