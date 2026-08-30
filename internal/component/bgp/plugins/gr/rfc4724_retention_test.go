package gr

import (
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// dispatchRecorder records every RIB command the GR plugin dispatches, in order.
// The plugin dispatches from timer goroutines as well as from the event loop, so
// the recorder locks.
type dispatchRecorder struct {
	mu   sync.Mutex
	sent []string
}

func (d *dispatchRecorder) hook() func(string, ...string) {
	return func(command string, args ...string) {
		d.mu.Lock()
		defer d.mu.Unlock()
		d.sent = append(d.sent, strings.TrimSpace(command+" "+strings.Join(args, " ")))
	}
}

func (d *dispatchRecorder) all() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string{}, d.sent...)
}

// newRecordedGRPlugin builds a GR plugin whose state callbacks are the PRODUCTION
// ones (wireStateCallbacks, gr.go) and whose dispatch is captured rather than sent.
// A test that installs its own callbacks proves the state machine and nothing else:
// it cannot see whether the plugin turns a transition into a RIB command at all.
func newRecordedGRPlugin() (*grPlugin, *dispatchRecorder) {
	rec := &dispatchRecorder{}
	gp := &grPlugin{
		peerCaps:     make(map[string]*grPeerCap),
		peerLLGRCaps: make(map[string]*llgrPeerCap),
		removedPeers: make(map[string]bool),
		dispatchHook: rec.hook(),
	}
	gp.wireStateCallbacks()
	return gp, rec
}

// TestRFC4724SessionDownRetainsAndMarksRoutesStale drives the plugin's own
// session-down path and reads the RIB commands it produces.
//
// RFC 4724 Section 4.2: "When the Receiving Speaker detects termination of the TCP
// session for a BGP session with a peer that has advertised the Graceful Restart
// Capability, it MUST retain the routes received from the peer for all the address
// families that were previously received in the Graceful Restart Capability and
// MUST mark them as stale routing information."
//
// VALIDATES: handleStateEvent's activated branch (gr.go) dispatches
// "request bgp rib retain-routes <peer>" and then
// "request bgp rib mark-stale <peer> <restart-time>", which are the two commands
// that perform the retention and the stale marking the requirement names.
// PREVENTS: the state machine activating while neither command is sent. Deleting
// both dispatch lines left the whole gr package green before this test existed:
// the requirement's only positive asserted that onSessionDown returned true and
// that the peer was active, neither of which retains or marks anything.
//
// RFC requirement: RFC4724-4.2-3 positive -- a GR-capable peer whose TCP session
// terminates has its routes retained and marked stale: the plugin dispatches
// retain-routes and mark-stale, in that order, carrying the peer's advertised
// Restart Time.
func TestRFC4724SessionDownRetainsAndMarksRoutesStale(t *testing.T) {
	gp, rec := newRecordedGRPlugin()
	gp.peerCaps[testPeer] = testCap(120, famIPv4, famIPv6)

	gp.handleStateEvent(testPeer, map[string]any{"state": "down", "reason": "tcp-failure"})

	require.Equal(t, []string{
		"request bgp rib purge-stale " + testPeer,
		"request bgp rib retain-routes " + testPeer,
		"request bgp rib mark-stale " + testPeer + " 120",
	}, rec.all(),
		"a GR-capable peer's TCP failure must retain its routes and mark them stale")
}

// TestRFC4724SessionDownWithoutCapabilityRetainsNothing is the pair that makes the
// test above discriminate: an implementation that dispatched retain-routes and
// mark-stale for every session drop would pass the positive and violate the
// capability condition the same sentence carries.
//
// RFC requirement: RFC4724-4.2-3 negative -- retention is confined to a peer that
// advertised the Graceful Restart Capability: a peer that advertised none has no
// routes retained and none marked stale, so no RIB command is dispatched at all.
func TestRFC4724SessionDownWithoutCapabilityRetainsNothing(t *testing.T) {
	gp, rec := newRecordedGRPlugin()
	// No entry in gp.peerCaps: this peer's OPEN carried no GR capability.

	gp.handleStateEvent(testPeer, map[string]any{"state": "down", "reason": "tcp-failure"})

	assert.Empty(t, rec.all(),
		"a peer that never advertised Graceful Restart must have its routes deleted by normal BGP procedures")
}

// TestRFC4724RetentionCoversEveryAdvertisedFamily reads the family set the state
// machine builds on a session drop.
//
// RFC 4724 Section 4.2 scopes the retention to "all the address families that were
// previously received in the Graceful Restart Capability". The set is therefore the
// requirement, not an implementation detail: retention that covers one family of two
// leaves the other's routes deleted, which is what the clause forbids.
//
// VALIDATES: onSessionDown (gr_state.go) builds staleFamilies from cap.Families, and
// the timer that bounds the retention is armed.
// PREVENTS: the family set being built from anything but the peer's capability.
// TestGRStateManagerRouteRetention asserted only that onSessionDown returned true and
// that the peer was active. Both stay true when the loop over cap.Families is replaced
// by a hardcoded single-family set, so that test could not see this clause at all.
//
// RFC requirement: RFC4724-4.2-3 positive -- a GR-capable peer's session drop marks
// stale exactly the address families its Graceful Restart Capability carried.
func TestRFC4724RetentionCoversEveryAdvertisedFamily(t *testing.T) {
	mgr := newGRStateManager(nil)

	require.True(t, mgr.onSessionDown(testPeer, testCap(120, famIPv4, famIPv6), nil, false))

	mgr.mu.Lock()
	state := mgr.peers[testPeer]
	require.NotNil(t, state)
	stale := make(map[family.Family]bool, len(state.staleFamilies))
	maps.Copy(stale, state.staleFamilies)
	timerArmed := state.restartTimer != nil
	mgr.mu.Unlock()

	assert.Equal(t, map[family.Family]bool{
		family.IPv4Unicast: true,
		family.IPv6Unicast: true,
	}, stale,
		"every family the peer carried in its Graceful Restart Capability must be retained, and no other")
	assert.True(t, timerArmed,
		"retention is bounded by the Restart Time, so the timer that ends it must be armed")
}

// TestRFC9494LLSTExpiryDeletesTheFamilysStaleRoutes drives the Long-Lived Stale
// Time to expiry with the production callbacks installed.
//
// RFC 9494 Section 4.2: "If the timer for the Long-Lived Stale Time for a given
// AFI/SAFI expires before the session is re-established, the helper MUST delete all
// stale routes of that AFI/SAFI from the neighbor that it is retaining."
//
// VALIDATES: onLLGRFamilyExpired (gr.go wireStateCallbacks) turns the expiry into
// "request bgp rib purge-stale <peer> <family>", which is the command that performs
// the deletion. TestRIBPurgeStaleFamilyCommand (plugins/rib) proves that command
// deletes the family's stale routes and leaves the other family alone.
// PREVENTS: the expiry firing into a callback nobody wired. Emptying that one
// callback body left every RFC9494-4.2-3 test green, because each installed its own
// collector over the same field and so could never see the production assignment.
//
// RFC requirement: RFC9494-4.2-3 positive -- the LLST elapsing while the session is
// still down deletes that family's retained stale routes, and only that family's.
func TestRFC9494LLSTExpiryDeletesTheFamilysStaleRoutes(t *testing.T) {
	gp, rec := newRecordedGRPlugin()

	// restart-time 0 enters the LLGR period at once, so the 1-second LLST timer is
	// armed now. ipv6's LLST is long enough that it cannot elapse during the test.
	llgrCap := &llgrPeerCap{
		Families: []llgrCapFamily{
			{Family: family.IPv4Unicast, ForwardState: true, LLST: 1},
			{Family: family.IPv6Unicast, ForwardState: true, LLST: 3600},
		},
	}
	gp.state.onSessionDown(testPeer, testCap(0, famIPv4, famIPv6), llgrCap, false)

	wantPurge := "request bgp rib purge-stale " + testPeer + " " + family.IPv4Unicast.String()
	require.Eventually(t, func() bool {
		return slices.Contains(rec.all(), wantPurge)
	}, 5*time.Second, 20*time.Millisecond,
		"the ipv4 Long-Lived Stale Time elapsed and its stale routes were never deleted")

	assert.NotContains(t, rec.all(),
		"request bgp rib purge-stale "+testPeer+" "+family.IPv6Unicast.String(),
		"ipv6's Long-Lived Stale Time has not elapsed, so its stale routes must still be retained")
}
