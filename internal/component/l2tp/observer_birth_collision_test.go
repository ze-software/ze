// Design: docs/architecture/l2tp/l2tp-9-observer.md -- independent original session history
// Related: observer.go -- actual typed lifecycle subscriptions and bounded rings

package l2tp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// TestObserverKeepsSameTupleOnTwoListenersApart publishes two valid original
// births with identical numeric labels through the real typed EventBus. Their
// event rings must remain distinct, including the second peer's useful echo.
// MUTATION: sessions keyed by uint16 SID merge both admitted histories.
func TestObserverKeepsSameTupleOnTwoListenersApart(t *testing.T) {
	bus := newTestBus()
	observer := newObserver(testObserverConfig())
	subsystem := &Subsystem{observer: observer}
	subsystem.wireObserverSubscriptions(bus)
	t.Cleanup(observer.Stop)
	first, err := subscriberid.NewL2TPBirth()
	require.NoError(t, err)
	second, err := subscriberid.NewL2TPBirth()
	require.NoError(t, err)
	require.NotEqual(t, first, second)
	generation, err := l2tpevents.NewObservationGeneration()
	require.NoError(t, err)
	_, err = l2tpevents.SessionUp.Emit(bus, &l2tpevents.SessionUpPayload{
		Birth: first, SubscriberID: first.String(), Generation: generation, TunnelID: 17, SessionID: 7,
	})
	require.NoError(t, err)
	_, err = l2tpevents.SessionUp.Emit(bus, &l2tpevents.SessionUpPayload{
		Birth: second, SubscriberID: second.String(), Generation: generation, TunnelID: 17, SessionID: 7,
	})
	require.NoError(t, err)
	_, err = l2tpevents.EchoRTT.Emit(bus, &l2tpevents.EchoRTTPayload{
		Birth: second, SubscriberID: second.String(), Generation: generation,
		TunnelID: 17, SessionID: 7, RTT: 13 * time.Millisecond,
	})
	require.NoError(t, err)
	summaries := observer.SessionSummaries()
	require.Len(t, summaries, 2, "equal labels cannot merge two original births")
	require.ElementsMatch(t, []int{1, 2}, []int{summaries[0].EventCount, summaries[1].EventCount})
	for _, summary := range summaries {
		require.EqualValues(t, 7, summary.SessionID)
	}
}

// TestObserverDownKeepsOtherListenerBirthHistory ends one of two equal-label
// births through the bus, then delivers another useful echo for the survivor.
// Exactly its own Up and two echoes must remain; Down cannot release its ring.
// MUTATION: releasing the SID ring discards the other listener's history.
func TestObserverDownKeepsOtherListenerBirthHistory(t *testing.T) {
	bus := newTestBus()
	observer := newObserver(testObserverConfig())
	subsystem := &Subsystem{observer: observer}
	subsystem.wireObserverSubscriptions(bus)
	t.Cleanup(observer.Stop)
	first, err := subscriberid.NewL2TPBirth()
	require.NoError(t, err)
	second, err := subscriberid.NewL2TPBirth()
	require.NoError(t, err)
	require.NotEqual(t, first, second)
	generation, err := l2tpevents.NewObservationGeneration()
	require.NoError(t, err)
	for _, birth := range []subscriberid.L2TPBirth{first, second} {
		_, err = l2tpevents.SessionUp.Emit(bus, &l2tpevents.SessionUpPayload{
			Birth: birth, SubscriberID: birth.String(), Generation: generation, TunnelID: 17, SessionID: 7,
		})
		require.NoError(t, err)
	}
	_, err = l2tpevents.EchoRTT.Emit(bus, &l2tpevents.EchoRTTPayload{
		Birth: second, SubscriberID: second.String(), Generation: generation,
		TunnelID: 17, SessionID: 7, RTT: 13 * time.Millisecond,
	})
	require.NoError(t, err)
	_, err = l2tpevents.SessionDown.Emit(bus, &l2tpevents.SessionDownPayload{
		Birth: first, SubscriberID: first.String(), Generation: generation, TunnelID: 17, SessionID: 7,
	})
	require.NoError(t, err)
	_, err = l2tpevents.EchoRTT.Emit(bus, &l2tpevents.EchoRTTPayload{
		Birth: second, SubscriberID: second.String(), Generation: generation,
		TunnelID: 17, SessionID: 7, RTT: 17 * time.Millisecond,
	})
	require.NoError(t, err)
	summaries := observer.SessionSummaries()
	require.Len(t, summaries, 1)
	require.EqualValues(t, 7, summaries[0].SessionID)
	require.Equal(t, 3, summaries[0].EventCount, "the survivor must keep its Up and both useful echoes")
}
