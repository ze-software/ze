package adj_rib_in

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
)

// notifyTestBus counts ValidationChange emissions.
type notifyTestBus struct {
	routes []ribevents.ValidationRoute
}

func (b *notifyTestBus) Emit(_, _ string, payload any) (int, error) {
	if routes, ok := payload.([]ribevents.ValidationRoute); ok {
		b.routes = append(b.routes, routes...)
	}
	return 0, nil
}

func (b *notifyTestBus) Subscribe(_, _ string, _ func(any)) func() { return func() {} }

// TestValidationChangeNeedsRegisteredGate proves a received route emits no
// ValidationChange while no eligibility gate is registered, and does emit one
// once the gate exists.
//
// VALIDATES: noteValidationChange emits only when this store's lookup is the
// gate that the forward path and selection consult.
// PREVENTS: the route server and reflector replaying every received route a
// second time. Each emission makes bgp-rs call replay-path for every target, so
// with validation never enabled every UPDATE reached the destination twice,
// and a replayed announcement could follow the withdrawal of the same prefix.
func TestValidationChangeNeedsRegisteredGate(t *testing.T) {
	body := rfc4271Announce(1, 0, 0, 0, 1, 24, 203, 0, 113)

	t.Run("no gate, no change", func(t *testing.T) {
		bus := &notifyTestBus{}
		r := newTestManager(t)
		r.ingestTracked = true
		r.validationBus = bus
		retainedReceive(t, r, 40, body)
		require.Empty(t, bus.routes, "an install with no registered gate changed no verdict")
	})

	t.Run("gate registered, change emitted", func(t *testing.T) {
		t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
		bus := &notifyTestBus{}
		r := newTestManager(t)
		r.ingestTracked = true
		r.validationBus = bus
		_, _, err := r.handleCommand("request bgp adj-rib-in enable-validation", nil, "")
		require.NoError(t, err)
		_, _, err = r.handleCommand("request bgp adj-rib-in disable-validation", nil, "")
		require.NoError(t, err)
		bus.routes = nil
		retainedReceive(t, r, 41, body)
		require.Len(t, bus.routes, 1, "the registered gate fences generations, so an install wakes selection")
	})
}
