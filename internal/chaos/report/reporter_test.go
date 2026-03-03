package report

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/chaos/peer"
)

// mockConsumer records all events it receives.
type mockConsumer struct {
	events []peer.Event
}

func (m *mockConsumer) ProcessEvent(ev peer.Event) {
	m.events = append(m.events, ev)
}

func (m *mockConsumer) Close() error { return nil }

// TestReporterFanOut verifies Reporter calls all enabled consumers for each event.
//
// VALIDATES: Each event is delivered to every registered consumer.
// PREVENTS: Events silently dropped when multiple consumers are attached.
func TestReporterFanOut(t *testing.T) {
	c1 := &mockConsumer{}
	c2 := &mockConsumer{}
	c3 := &mockConsumer{}

	r := NewReporter(c1, c2, c3)

	ev := peer.Event{
		Type:      peer.EventEstablished,
		PeerIndex: 0,
		Time:      time.Now(),
	}
	r.Process(ev)

	assert.Len(t, c1.events, 1)
	assert.Len(t, c2.events, 1)
	assert.Len(t, c3.events, 1)
	assert.Equal(t, peer.EventEstablished, c1.events[0].Type)
}

// TestReporterMultipleEvents verifies multiple events are all delivered.
//
// VALIDATES: Event ordering is preserved across consumers.
// PREVENTS: Reporter losing events after the first one.
func TestReporterMultipleEvents(t *testing.T) {
	c := &mockConsumer{}
	r := NewReporter(c)

	events := []peer.Event{
		{Type: peer.EventEstablished, PeerIndex: 0, Time: time.Now()},
		{Type: peer.EventRouteSent, PeerIndex: 1, Time: time.Now(), Prefix: netip.MustParsePrefix("10.0.0.0/24")},
		{Type: peer.EventDisconnected, PeerIndex: 2, Time: time.Now()},
	}

	for _, ev := range events {
		r.Process(ev)
	}

	assert.Len(t, c.events, 3)
	assert.Equal(t, peer.EventEstablished, c.events[0].Type)
	assert.Equal(t, peer.EventRouteSent, c.events[1].Type)
	assert.Equal(t, peer.EventDisconnected, c.events[2].Type)
}

// TestReporterNilConsumers verifies Reporter handles no consumers gracefully.
//
// VALIDATES: Reporter with zero consumers does not panic.
// PREVENTS: Nil pointer dereference when no reporting is configured.
func TestReporterNilConsumers(t *testing.T) {
	r := NewReporter()

	// Should not panic.
	r.Process(peer.Event{Type: peer.EventEstablished, Time: time.Now()})

	// Close with no consumers should not error or panic.
	assert.NoError(t, r.Close())
}

// TestReporterClose verifies Close calls Close on all consumers.
//
// VALIDATES: All consumers receive Close() call.
// PREVENTS: File handles or HTTP servers leaked on shutdown.
func TestReporterClose(t *testing.T) {
	c1 := &closeTracker{}
	c2 := &closeTracker{}

	r := NewReporter(c1, c2)
	assert.NoError(t, r.Close())

	assert.True(t, c1.closed)
	assert.True(t, c2.closed)
}

// closeTracker tracks Close() calls while implementing Consumer.
type closeTracker struct {
	closed bool
	events []peer.Event
}

func (c *closeTracker) ProcessEvent(ev peer.Event) {
	c.events = append(c.events, ev)
}

func (c *closeTracker) Close() error {
	c.closed = true
	return nil
}
