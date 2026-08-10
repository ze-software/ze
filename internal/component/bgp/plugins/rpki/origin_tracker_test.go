package rpki

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOriginTrackerRevalidate verifies the tracker detects origin-validation state changes when
// the ROA cache changes, and prunes removed routes.
func TestOriginTrackerRevalidate(t *testing.T) {
	cache := newROACache()
	tr := newOriginTracker()

	key := routeKey{peerAddr: "10.0.0.1", family: "ipv4/unicast", prefix: "10.0.0.0/24", pathID: 0}
	tr.Track(key, 65001, ValidationNotFound, aspaStateNone)
	assert.Equal(t, 1, tr.count())

	// Empty cache: the route is still NotFound, so re-validation reports no change.
	assert.Empty(t, tr.revalidate(cache))

	// A VRP authorizing the route arrives: re-validation flips it to Valid and reports the change.
	cache.Add(makeVRP("10.0.0.0/8", 24, 65001))
	changed := tr.revalidate(cache)
	require.Len(t, changed, 1)
	assert.Equal(t, key, changed[0].key)
	assert.Equal(t, ValidationValid, changed[0].state)

	// The updated state is stored: a second re-validation with no further change reports nothing.
	assert.Empty(t, tr.revalidate(cache))

	tr.Remove(key)
	assert.Equal(t, 0, tr.count())
	assert.Empty(t, tr.revalidate(cache))
}

// TestHandleROAChangeReValidates verifies RFC 6811 Section 4 origin re-validation: a VRP mapping
// change re-validates installed routes and re-runs the decision process for those whose state
// changed, and does nothing for a change that does not alter any route's state.
//
// RFC requirement: RFC6811-4-1 positive -- when a mapping is added, the implementation re-validates
// the affected prefix and runs the decision process: adding a covering VRP flips the tracked route
// from NotFound to Valid and enqueues a fresh validation decision for it.
// RFC requirement: RFC6811-4-1 negative -- the decision process runs only "if needed": a
// re-validation that changes no route's state enqueues nothing, so a VRP change that does not
// affect a route does not spuriously re-dispatch it.
func TestHandleROAChangeReValidates(t *testing.T) {
	rp := &rPKIPlugin{
		cache:         newROACache(),
		originTracker: newOriginTracker(),
		validateCh:    make(chan validationRequest, 16),
		stopCh:        make(chan struct{}),
	}
	key := routeKey{peerAddr: "10.0.0.1", family: "ipv4/unicast", prefix: "10.0.0.0/24", pathID: 0}
	rp.originTracker.Track(key, 65001, ValidationNotFound, aspaStateNone)

	// A VRP authorizing the route arrives: handleROAChange re-validates and enqueues a decision.
	rp.cache.Add(makeVRP("10.0.0.0/8", 24, 65001))
	rp.handleROAChange()
	select {
	case req := <-rp.validateCh:
		assert.Equal(t, "10.0.0.0/24", req.prefix)
		assert.Equal(t, ValidationValid, req.state)
	default:
		t.Fatal("a VRP change that flips a route's state must re-dispatch a validation decision")
	}

	// No further cache change: the route's state is stable, so nothing is re-dispatched.
	rp.handleROAChange()
	select {
	case <-rp.validateCh:
		t.Fatal("no state change: the decision process must not be re-run for an unaffected route")
	default:
	}
}
