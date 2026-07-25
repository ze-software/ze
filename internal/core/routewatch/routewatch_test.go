package routewatch

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/rtproto"
)

func TestWatcherFanout(t *testing.T) {
	w := New()

	var events1, events2 []RouteEvent
	w.Register(func(ev RouteEvent) { events1 = append(events1, ev) })
	w.Register(func(ev RouteEvent) { events2 = append(events2, ev) })

	ev := RouteEvent{
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:  netip.MustParseAddr("192.168.1.1"),
		Protocol: 16,
		Action:   ActionAdd,
	}
	w.deliver(ev)

	require.Len(t, events1, 1)
	require.Len(t, events2, 1)
	assert.Equal(t, ev, events1[0])
	assert.Equal(t, ev, events2[0])
}

func TestWatcherFilterZeOwned(t *testing.T) {
	w := New()
	var events []RouteEvent
	w.Register(func(ev RouteEvent) { events = append(events, ev) })

	for _, proto := range []int{rtproto.FIBKernel, rtproto.Static, rtproto.PolicyRoute} {
		w.deliver(RouteEvent{
			Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
			Protocol: proto,
			Action:   ActionAdd,
		})
	}

	assert.Empty(t, events)
}

func TestWatcherFilterNilDst(t *testing.T) {
	w := New()
	var events []RouteEvent
	w.Register(func(ev RouteEvent) { events = append(events, ev) })

	w.deliver(RouteEvent{
		Protocol: 16,
		Action:   ActionAdd,
	})

	assert.Empty(t, events)
}

func TestWatcherUnregister(t *testing.T) {
	w := New()
	var events1, events2 []RouteEvent
	unreg1 := w.Register(func(ev RouteEvent) { events1 = append(events1, ev) })
	w.Register(func(ev RouteEvent) { events2 = append(events2, ev) })

	ev := RouteEvent{
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		Protocol: 16,
		Action:   ActionAdd,
	}

	w.deliver(ev)
	require.Len(t, events1, 1)
	require.Len(t, events2, 1)

	unreg1()

	w.deliver(ev)
	assert.Len(t, events1, 1)
	assert.Len(t, events2, 2)
}

func TestWatcherLateRegistration(t *testing.T) {
	w := New()

	ev := RouteEvent{
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		Protocol: 16,
		Action:   ActionAdd,
	}
	w.deliver(ev)

	var events []RouteEvent
	w.Register(func(ev RouteEvent) { events = append(events, ev) })

	w.deliver(ev)
	require.Len(t, events, 1)
	assert.Equal(t, ev, events[0])
}

func TestWatcherRemoveAction(t *testing.T) {
	w := New()
	var events []RouteEvent
	w.Register(func(ev RouteEvent) { events = append(events, ev) })

	ev := RouteEvent{
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:  netip.MustParseAddr("192.168.1.1"),
		Protocol: 3,
		Action:   ActionRemove,
	}
	w.deliver(ev)

	require.Len(t, events, 1)
	assert.Equal(t, ActionRemove, events[0].Action)
}
