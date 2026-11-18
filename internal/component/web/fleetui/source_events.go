// Design: docs/architecture/fleet-interface.md -- the live event subscription
// Overview: source.go -- the data source seam
// Related: source_errors.go -- ErrStreamsFull and ErrFeedClosed
//
// A source tells its subscribers what changed, and a page then fetches again
// what the change made stale. The events carry no data of their own beyond
// what names the change, so the browser holds no model of the estate.

package fleetui

import (
	"context"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
)

// The bounds of the live events.
const (
	// EventBufferSize is how many events one subscription holds unread. A
	// subscriber that falls further behind is ended.
	EventBufferSize = 64
	// EventSubscriptionsMax is how many subscriptions one feed serves at
	// once.
	EventSubscriptionsMax = 64
)

// EventSource delivers what changes in the estate while a page is open.
type EventSource interface {
	// Subscribe opens a subscription to every change of the estate. The
	// caller MUST call Close on the subscription when it stops reading. A
	// source that already serves its maximum of subscriptions wraps
	// ErrStreamsFull. The context bounds the call only: it does not end the
	// subscription.
	// Asked by: the event stream of every open page.
	// Answered by: fleet-ui-5.
	Subscribe(ctx context.Context) (*Subscription, error)
}

// EventKind is what changed. The zero value is not a kind.
type EventKind uint8

const (
	// EventUnspecified is the invalid zero value.
	EventUnspecified EventKind = iota
	// EventDeviceChanged means a fact of one device changed: its connection,
	// its health, its configuration state.
	EventDeviceChanged
	// EventAlarmOpened means an alarm opened.
	EventAlarmOpened
	// EventAlarmCleared means an alarm cleared.
	EventAlarmCleared
	// EventTaskCreated means a task was created.
	EventTaskCreated
	// EventTaskProgressed means a device of a task got its result.
	EventTaskProgressed
	// EventTaskEnded means a task ended.
	EventTaskEnded
	// EventRegistryChanged means a device was added, removed or relabeled.
	EventRegistryChanged
)

var eventKindKeys = []string{
	enumKeyUnspecified, "device-changed", "alarm-opened", "alarm-cleared",
	"task-created", "task-progressed", "task-ended", "registry-changed",
}

// String answers the kind's name, which is also its name on the event stream.
func (kind EventKind) String() string {
	return enumText(eventKindKeys, kind)
}

// Event is one change of the estate.
type Event struct {
	Kind EventKind
	// At is when the change happened, on the hub's clock.
	At time.Time
	// Device names the device the change is about. It is empty for a change
	// about no single device, such as a task over several.
	Device string
	// Subject identifies the alarm or the task of an alarm or task event. It
	// is empty for the other kinds.
	Subject string
}

// Feed fans events out to subscriptions. A source holds one and publishes
// into it. The zero value is an open feed with no subscription, ready for
// use. Safe for concurrent use.
//
// A Feed starts no goroutine. Publish runs on the publisher's goroutine and
// never blocks: a subscription whose buffer is full is ended instead, so one
// page that stopped reading cannot hold back the others.
//
// Ownership: the Feed creates every subscription's channel and is the only
// one that closes it, once, when the subscription ends. A subscription ends
// when its subscriber calls Close, when it falls behind, or when the Feed is
// closed. A source that stops MUST call Close on its Feed.
type Feed struct {
	mu            sync.Mutex
	subscriptions map[*Subscription]struct{}
	closed        bool
}

// Subscribe opens a subscription. The caller MUST call Close on it when it
// stops reading. It refuses with ErrStreamsFull when EventSubscriptionsMax
// subscriptions are open, and with ErrFeedClosed after Close.
func (feed *Feed) Subscribe() (*Subscription, error) {
	feed.mu.Lock()
	defer feed.mu.Unlock()

	if feed.closed {
		return nil, opererr.Wrap(ErrFeedClosed, "Live updates are not available.",
			"The hub stopped sending events, so this page shows what it read when it opened and does not change.",
			"reload the page")
	}
	if len(feed.subscriptions) >= EventSubscriptionsMax {
		return nil, opererr.Wrap(ErrStreamsFull, "Live updates are not available for this page.",
			"The hub already serves its maximum of live pages, so this page does not change until it is reloaded.",
			"close a fleet page that is no longer needed, then reload this one")
	}

	if feed.subscriptions == nil {
		feed.subscriptions = make(map[*Subscription]struct{})
	}
	subscription := &Subscription{feed: feed, events: make(chan Event, EventBufferSize)}
	feed.subscriptions[subscription] = struct{}{}

	return subscription, nil
}

// Publish hands event to every open subscription. It never blocks: a
// subscription with no room left is ended and marked as having fallen behind.
// After Close it reaches nobody.
func (feed *Feed) Publish(event Event) {
	feed.mu.Lock()
	defer feed.mu.Unlock()

	for subscription := range feed.subscriptions {
		select {
		case subscription.events <- event:
		default:
			subscription.lagged = true
			feed.end(subscription)
		}
	}
}

// Close ends every subscription and refuses new ones. A second Close does
// nothing.
func (feed *Feed) Close() {
	feed.mu.Lock()
	defer feed.mu.Unlock()

	feed.closed = true
	for subscription := range feed.subscriptions {
		feed.end(subscription)
	}
}

// end removes one open subscription and closes its channel. The caller MUST
// hold feed.mu, and subscription MUST be in feed.subscriptions: that is what
// makes the close happen once.
func (feed *Feed) end(subscription *Subscription) {
	delete(feed.subscriptions, subscription)
	close(subscription.events)
}

// Subscription is one subscriber's view of a Feed. Only Feed.Subscribe makes
// one, so a nil or zero Subscription is never valid. Safe for concurrent use.
type Subscription struct {
	feed   *Feed
	events chan Event
	// lagged is guarded by feed.mu.
	lagged bool
}

// Events answers the channel the events arrive on. The Feed closes it when
// the subscription ends: a reader that sees it closed MUST stop, and Lagged
// then says whether events were lost.
func (subscription *Subscription) Events() <-chan Event {
	return subscription.events
}

// Close ends the subscription. It MUST be called once the subscriber stops
// reading, and a second Close, or a Close after the Feed ended the
// subscription, does nothing.
func (subscription *Subscription) Close() {
	feed := subscription.feed

	feed.mu.Lock()
	defer feed.mu.Unlock()

	if _, open := feed.subscriptions[subscription]; open {
		feed.end(subscription)
	}
}

// Lagged reports whether the Feed ended the subscription because its
// subscriber fell behind. Events were lost then, so the page fetches every
// live fragment again when it subscribes anew.
func (subscription *Subscription) Lagged() bool {
	subscription.feed.mu.Lock()
	defer subscription.feed.mu.Unlock()

	return subscription.lagged
}
