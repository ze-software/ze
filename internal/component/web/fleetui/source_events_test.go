package fleetui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFeedDeliversToEverySubscription publishes events into a feed with two
// subscriptions.
//
// VALIDATES: each subscription receives every event, in the order published.
// PREVENTS: an event that reaches one open page and not another.
func TestFeedDeliversToEverySubscription(t *testing.T) {
	var feed Feed

	first, err := feed.Subscribe()
	require.NoError(t, err)
	defer first.Close()

	second, err := feed.Subscribe()
	require.NoError(t, err)
	defer second.Close()

	offline := Event{Kind: EventDeviceChanged, At: testNow, Device: edgeLon01}
	opened := Event{Kind: EventAlarmOpened, At: testNow, Device: edgeLon01, Subject: "a9"}
	feed.Publish(offline)
	feed.Publish(opened)

	for _, subscription := range []*Subscription{first, second} {
		assert.Equal(t, offline, <-subscription.Events())
		assert.Equal(t, opened, <-subscription.Events())
		assert.False(t, subscription.Lagged())
	}
}

// TestFeedEndsASubscriptionThatFallsBehind publishes more events than one
// subscription holds unread, beside a subscription that is read.
//
// VALIDATES: a subscription whose buffer is full is ended and marked as
// having fallen behind, its channel is closed after the events it held, the
// publisher never blocks, and the other subscription keeps receiving.
// PREVENTS: one page that stopped reading holding back every other page.
func TestFeedEndsASubscriptionThatFallsBehind(t *testing.T) {
	var feed Feed

	stalled, err := feed.Subscribe()
	require.NoError(t, err)
	defer stalled.Close()

	reader, err := feed.Subscribe()
	require.NoError(t, err)
	defer reader.Close()

	for range EventBufferSize + 1 {
		feed.Publish(Event{Kind: EventTaskProgressed, At: testNow, Subject: "t1"})
		<-reader.Events()
	}

	assert.True(t, stalled.Lagged())
	assert.False(t, reader.Lagged())

	held := 0
	for range stalled.Events() {
		held++
	}
	assert.Equal(t, EventBufferSize, held, "the events held before the subscription ended stay readable")

	feed.Publish(Event{Kind: EventTaskEnded, At: testNow, Subject: "t1"})
	assert.Equal(t, EventTaskEnded, (<-reader.Events()).Kind)
}

// TestFeedRefusesBeyondItsMaximum opens the most subscriptions a feed
// serves, and one more.
//
// VALIDATES: the subscription beyond the maximum is refused with
// ErrStreamsFull, and closing one makes room again.
// PREVENTS: an unbounded number of open event streams.
func TestFeedRefusesBeyondItsMaximum(t *testing.T) {
	var feed Feed

	open := make([]*Subscription, 0, EventSubscriptionsMax)
	for range EventSubscriptionsMax {
		subscription, err := feed.Subscribe()
		require.NoError(t, err)
		open = append(open, subscription)
	}

	_, err := feed.Subscribe()
	requireRefusal(t, err, ErrStreamsFull)

	open[0].Close()

	again, err := feed.Subscribe()
	require.NoError(t, err)
	again.Close()

	for _, subscription := range open[1:] {
		subscription.Close()
	}
}

// TestSubscriptionCloseIsSafeToRepeat closes a subscription twice, then
// publishes.
//
// VALIDATES: Close closes the channel once, a second Close does nothing, and
// a closed subscription receives nothing and did not fall behind.
// PREVENTS: a double close of the channel panicking the request that ends a
// stream.
func TestSubscriptionCloseIsSafeToRepeat(t *testing.T) {
	var feed Feed

	subscription, err := feed.Subscribe()
	require.NoError(t, err)

	subscription.Close()
	subscription.Close()
	feed.Publish(Event{Kind: EventRegistryChanged, At: testNow})

	_, open := <-subscription.Events()
	assert.False(t, open)
	assert.False(t, subscription.Lagged())
}

// TestFeedCloseEndsEverySubscription closes a feed that has subscriptions.
//
// VALIDATES: every subscription's channel is closed, a later Subscribe is
// refused with ErrFeedClosed, and a second Close and a later Publish do
// nothing.
// PREVENTS: a page waiting forever on a source that stopped.
func TestFeedCloseEndsEverySubscription(t *testing.T) {
	var feed Feed

	subscription, err := feed.Subscribe()
	require.NoError(t, err)

	feed.Close()
	feed.Close()
	feed.Publish(Event{Kind: EventDeviceChanged, At: testNow})

	_, open := <-subscription.Events()
	assert.False(t, open)

	subscription.Close()

	_, err = feed.Subscribe()
	requireRefusal(t, err, ErrFeedClosed)
}

// TestFailureOfReadsTheCause wraps each cause a page shows a state for.
//
// VALIDATES: each cause reads back as its state through the operator error
// that wraps it, and an error with no known cause reads as failed.
// PREVENTS: a page choosing its state by matching error text.
func TestFailureOfReadsTheCause(t *testing.T) {
	assert.Equal(t, FailureNotFound, FailureOf(DeviceUnknownError("edge-lon-99")))
	assert.Equal(t, FailureOffline, FailureOf(ErrDeviceOffline))
	assert.Equal(t, FailureRelayRefused, FailureOf(ErrRelayRefused))
	assert.Equal(t, FailureUnsupported, FailureOf(ErrViewUnsupported))
	assert.Equal(t, FailureDeadline, FailureOf(ErrDeadline))
	assert.Equal(t, FailureUnauthorized, FailureOf(ErrUnauthorized))
	assert.Equal(t, FailureFailed, FailureOf(ErrStreamsFull))
	assert.Equal(t, FailureFailed, FailureOf(assert.AnError))
}
