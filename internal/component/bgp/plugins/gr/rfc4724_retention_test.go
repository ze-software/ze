package gr

import (
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
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
// session-down path into the real RIB engine and reads its received routes.
//
// RFC 4724 Section 4.2: "When the Receiving Speaker detects termination of the TCP
// session for a BGP session with a peer that has advertised the Graceful Restart
// Capability, it MUST retain the routes received from the peer for all the address
// families that were previously received in the Graceful Restart Capability and
// MUST mark them as stale routing information."
//
// VALIDATES: the advertised family allowlist reaches the received inventory
// before RIB processes DOWN, and each retained route becomes stale.
// PREVENTS: state-machine activation without retention or stale marking, and
// peer-wide retention of a negotiated family absent from the GR capability.
//
// RFC requirement: RFC4724-4.2-3 positive -- a GR-capable peer whose TCP session
// terminates has its advertised families' received routes retained and marked stale.
func TestRFC4724SessionDownRetainsAndMarksRoutesStale(t *testing.T) {
	for _, both := range []bool{false, true} {
		t.Run(map[bool]string{false: "ipv4-only", true: "both-families"}[both], func(t *testing.T) {
			gp, rib := newGRWithRealRIB(t)
			cap := testCap(120, famIPv4)
			if both {
				cap.Families = append(cap.Families, famIPv6)
			}
			gp.peerCaps[testPeer] = cap
			rib.received("18c63364", grReceivedAttrs)
			rib.command("request bgp rib inject", testPeer, "ipv6/unicast", "2001:db8::/32", "nexthop", "::1")
			require.Len(t, rib.routes("2001:db8::/32"), 1)
			gp.handleStateEvent(testPeer, map[string]any{"state": "down", "reason": "tcp-failure"})
			rib.down()
			rib.requireStale("198.51.100.0/24", 1)
			if both {
				rib.requireStale("2001:db8::/32", 1)
			} else {
				require.Empty(t, rib.routes("2001:db8::/32"))
			}
		})
	}
}

// The no-capability control observes normal DOWN deletion in the same real RIB.
//
// RFC requirement: RFC4724-4.2-3 negative -- retention is confined to a peer that
// advertised the Graceful Restart Capability; a peer without it loses its routes.
func TestRFC4724SessionDownWithoutCapabilityRetainsNothing(t *testing.T) {
	gp, rib := newGRWithRealRIB(t)
	rib.received("18c63364", grReceivedAttrs)
	require.Len(t, rib.routes("198.51.100.0/24"), 1)
	gp.handleStateEvent(testPeer, map[string]any{"state": "down", "reason": "tcp-failure"})
	rib.down()
	require.Empty(t, rib.routes("198.51.100.0/24"))
}

// TestRFC4724ZeroRestartTimeExpiresAfterStaleMarking lets every runnable timer
// finish before the first DOWN command, then checks command order and the real RIB.
//
// RFC 4724 Section 4.2: "If the session does not get re-established within the
// "Restart Time" that the peer advertised previously, the Receiving Speaker MUST
// delete all the stale routes from the peer that it is retaining."
//
// RFC requirement: RFC4724-4.2-7 positive -- a zero received Restart Time releases the retained routes after stale marking on both event paths, leaving no received route or active GR state.
func TestRFC4724ZeroRestartTimeExpiresAfterStaleMarking(t *testing.T) {
	for _, path := range rfc9494Paths {
		t.Run(path.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				gp, rib := newGRWithRealRIB(t)
				gp.peerCaps[testPeer] = testCap(0, famIPv4)
				rib.received("18c63364", grReceivedAttrs)

				var rec dispatchRecorder
				record := rec.hook()
				gp.dispatchHook = func(command string, args ...string) {
					if command == "request bgp rib purge-stale" {
						// A prematurely armed zero timer would finish before retain.
						synctest.Wait()
					}
					record(command, args...)
					rib.command(command, args...)
				}

				// RFC 4724 Section 4.2.
				rfc9494Down(gp, path.structured)
				synctest.Wait()
				require.Equal(t, []string{
					"request bgp rib purge-stale " + testPeer,
					"request bgp rib retain-routes " + testPeer + " ipv4/unicast",
					"request bgp rib mark-stale " + testPeer + " 0",
					"request bgp rib release-routes " + testPeer,
				}, rec.all())
				require.Empty(t, rib.routes("198.51.100.0/24"), "GR expiry must delete the retained route")
				assert.False(t, gp.state.peerActive(testPeer))
				rib.down()
				require.Empty(t, rib.routes("198.51.100.0/24"), "RIB DOWN must not restore retention")
			})
		})
	}
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
