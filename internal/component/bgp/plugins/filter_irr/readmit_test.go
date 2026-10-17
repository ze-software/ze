package filter_irr

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// fakeReadmitEngine answers the worker's two engine questions and records
// every ROUTE-REFRESH it was asked for on sent.
type fakeReadmitEngine struct {
	mu         sync.Mutex
	sess       peerSession
	sessErr    error
	refreshErr error
	sent       chan peerFamily

	// afterRefresh, when set, replaces sess once a ROUTE-REFRESH was sent: the
	// peer went down while the request was in flight.
	afterRefresh *peerSession

	// afterRefreshErr, when set, is what session answers once a ROUTE-REFRESH
	// was sent: the confirming read failed.
	afterRefreshErr error

	// duringRefresh, when set, runs inside refresh before it answers: an
	// UPDATE the filter judged while the request was in flight.
	duringRefresh func()
}

func newFakeReadmitEngine() *fakeReadmitEngine {
	return &fakeReadmitEngine{
		sess: peerSession{established: true, routeRefresh: true},
		sent: make(chan peerFamily, 16),
	}
}

func (e *fakeReadmitEngine) session(context.Context, netip.Addr) (peerSession, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sess, e.sessErr
}

func (e *fakeReadmitEngine) refresh(_ context.Context, peer netip.Addr, family string) error {
	e.mu.Lock()
	err := e.refreshErr
	during := e.duringRefresh
	e.mu.Unlock()
	if during != nil {
		during()
	}
	if err != nil {
		return err
	}
	e.mu.Lock()
	if e.afterRefresh != nil {
		e.sess = *e.afterRefresh
	}
	if e.afterRefreshErr != nil {
		e.sessErr = e.afterRefreshErr
	}
	e.mu.Unlock()
	e.sent <- peerFamily{peer: peer, family: family}
	return nil
}

func (e *fakeReadmitEngine) setRefreshErr(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.refreshErr = err
}

func (e *fakeReadmitEngine) setSession(sess peerSession) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sess = sess
}

const readmitTestPeer = "127.0.0.1"

// readmitPlugin builds a plugin enrolled for AS65001 / AS-TEST whose first
// resolution window is already over, so handleFilterUpdate decides at once.
func readmitPlugin(t *testing.T, irrAddr string, engine readmitEngine, interval time.Duration) *irrPlugin {
	t.Helper()
	st := newASNState(65001, "AS-TEST")
	st.signalFirstResolved()
	plug := &irrPlugin{
		byASN:       map[uint32]*asnState{65001: st},
		prefixStore: store.New(irr.NewIRR(irrAddr), nil, nil),
		stopCh:      make(chan struct{}),
	}
	plug.readmit = newReadmitter(engine, plug, interval)
	t.Cleanup(func() { close(plug.stopCh) })
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		plug.readmit.run(stop)
		close(done)
	}()
	t.Cleanup(func() {
		close(stop)
		<-done
	})
	return plug
}

// holdPrefixes holds prefixes for peer as AS65001's filter would.
func holdPrefixes(r *readmitter, peer netip.Addr, family string, prefixes []netip.Prefix) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, prefix := range prefixes {
		r.insert(heldKey{peer: peer, family: family, prefix: prefix}, 65001)
	}
	r.changed()
}

func announce(plug *irrPlugin, prefix string) sdk.FilterAction {
	out := plug.handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "bgp-filter-irr:65001",
		Peer:   readmitTestPeer,
		Update: "ipv4/unicast announce nlri add " + prefix,
	})
	return out.Action
}

func waitRefresh(t *testing.T, engine *fakeReadmitEngine) peerFamily {
	t.Helper()
	const within = 5 * time.Second
	select {
	case pf := <-engine.sent:
		return pf
	case <-time.After(within):
		t.Fatalf("no ROUTE-REFRESH was asked for within %s", within)
		return peerFamily{}
	}
}

func expectNoRefresh(t *testing.T, engine *fakeReadmitEngine, during time.Duration) {
	t.Helper()
	select {
	case pf := <-engine.sent:
		t.Fatalf("unexpected ROUTE-REFRESH for %v", pf)
	case <-time.After(during):
	}
}

// VALIDATES: a prefix rejected because its ASN's list was not resolved yet is
// asked for again once the list arrives, without the peer sending anything.
// METHOD: the UPDATE is rejected as no-prefix-list and held; the list then
// arrives through the real refresh path (refreshASN against a fake IRR); the
// worker must ask the peer for ipv4/unicast, and the peer's answer is accepted.
func TestReadmitWhenListArrives(t *testing.T) {
	engine := newFakeReadmitEngine()
	plug := readmitPlugin(t, fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"}), engine, time.Hour)

	if got := announce(plug, "10.0.0.0/24"); got != sdk.FilterReject {
		t.Fatalf("action before the list = %v, want reject", got)
	}
	if got := plug.readmit.heldCount(); got != 1 {
		t.Fatalf("held = %d, want 1", got)
	}
	expectNoRefresh(t, engine, 50*time.Millisecond)

	plug.refreshASN(65001)

	pf := waitRefresh(t, engine)
	if pf.peer != netip.MustParseAddr(readmitTestPeer) || pf.family != "ipv4/unicast" {
		t.Fatalf("refresh asked for %v, want 127.0.0.1 ipv4/unicast", pf)
	}
	if got := plug.readmit.heldCount(); got != 0 {
		t.Errorf("held after the refresh = %d, want 0", got)
	}
	if got := announce(plug, "10.0.0.0/24"); got != sdk.FilterAccept {
		t.Errorf("the route the peer sent again = %v, want accept", got)
	}
}

// VALIDATES: a prefix rejected by a resolved list is asked for again when a
// refresh grows the list to include it, and not before.
func TestReadmitWhenListGrows(t *testing.T) {
	engine := newFakeReadmitEngine()
	plug := readmitPlugin(t, fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"}), engine, time.Hour)
	plug.refreshASN(65001)

	if got := announce(plug, "192.0.2.0/24"); got != sdk.FilterReject {
		t.Fatalf("out-of-list action = %v, want reject", got)
	}
	// A refresh that does not change the verdict asks for nothing.
	plug.refreshASN(65001)
	expectNoRefresh(t, engine, 100*time.Millisecond)

	plug.mu.Lock()
	plug.prefixStore = store.New(irr.NewIRR(fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24 192.0.2.0/24"})), nil, nil)
	plug.mu.Unlock()
	plug.refreshASN(65001)

	waitRefresh(t, engine)
}

// VALIDATES: the worker's timer retries a failed IRR lookup for an ASN that
// holds a prefix, and asks for the prefix once the lookup succeeds.
func TestReadmitTimerRetriesFailedLookup(t *testing.T) {
	engine := newFakeReadmitEngine()
	plug := readmitPlugin(t, fakeIRRv4(t, nil), engine, 20*time.Millisecond)
	plug.refreshASN(65001)
	if plug.byASN[65001].lastErr == "" {
		t.Fatal("the seed lookup did not fail")
	}
	if got := announce(plug, "10.0.0.0/24"); got != sdk.FilterReject {
		t.Fatalf("action = %v, want reject", got)
	}

	// The IRR server recovers. Nothing but the worker's timer refreshes.
	plug.mu.Lock()
	plug.prefixStore = store.New(irr.NewIRR(fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"})), nil, nil)
	plug.mu.Unlock()

	waitRefresh(t, engine)
}

// VALIDATES: the held set never grows past heldPrefixMax.
func TestReadmitHeldSetCapped(t *testing.T) {
	r := newReadmitter(newFakeReadmitEngine(), nil, time.Hour)
	peer := netip.MustParseAddr(readmitTestPeer)
	prefixes := make([]netip.Prefix, 0, heldPrefixMax+10)
	for i := range heldPrefixMax + 10 {
		addr := netip.AddrFrom4([4]byte{10, byte(i >> 16), byte(i >> 8), byte(i)})
		prefixes = append(prefixes, netip.PrefixFrom(addr, 32))
	}
	holdPrefixes(r, peer, "ipv4/unicast", prefixes)
	if got := r.heldCount(); got != heldPrefixMax {
		t.Fatalf("held = %d, want the cap %d", got, heldPrefixMax)
	}
	r.forget(peer, "ipv4/unicast", prefixes[:1])
	holdPrefixes(r, peer, "ipv4/unicast", prefixes[heldPrefixMax:heldPrefixMax+1])
	if got := r.heldCount(); got != heldPrefixMax {
		t.Fatalf("held after one freed slot = %d, want %d", got, heldPrefixMax)
	}
}

// VALIDATES: a withdrawal, an accept and a reconfigure that stops filtering the
// ASN each drop the held prefix.
func TestReadmitHeldPrefixDropped(t *testing.T) {
	engine := newFakeReadmitEngine()
	plug := readmitPlugin(t, fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"}), engine, time.Hour)
	plug.refreshASN(65001)

	announce(plug, "192.0.2.0/24")
	plug.handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "bgp-filter-irr:65001",
		Peer:   readmitTestPeer,
		Update: "ipv4/unicast withdraw nlri del 192.0.2.0/24",
	})
	if got := plug.readmit.heldCount(); got != 0 {
		t.Errorf("held after the withdrawal = %d, want 0", got)
	}

	peer := netip.MustParseAddr(readmitTestPeer)
	holdPrefixes(plug.readmit, peer, "ipv4/unicast", []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")})
	if got := announce(plug, "10.0.0.0/24"); got != sdk.FilterAccept {
		t.Fatalf("in-list action = %v, want accept", got)
	}
	if got := plug.readmit.heldCount(); got != 0 {
		t.Errorf("held after the accept = %d, want 0", got)
	}

	announce(plug, "192.0.2.0/24")
	plug.handleConfigure(map[string]any{})
	if got := plug.readmit.heldCount(); got != 0 {
		t.Errorf("held after a reconfigure without AS65001 = %d, want 0", got)
	}
}

// VALIDATES: the timer drops what a peer that is no longer established holds,
// and asks it for nothing.
func TestReadmitPeerDownDropsHeld(t *testing.T) {
	engine := newFakeReadmitEngine()
	engine.setSession(peerSession{established: false, routeRefresh: true})
	plug := readmitPlugin(t, fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"}), engine, 20*time.Millisecond)
	plug.refreshASN(65001)

	announce(plug, "192.0.2.0/24")
	deadline := time.Now().Add(5 * time.Second)
	for plug.readmit.heldCount() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("the held prefix of a peer that is down was not dropped")
		}
		time.Sleep(10 * time.Millisecond) // sleep(poll): heldCount has no change notification
	}
	expectNoRefresh(t, engine, 50*time.Millisecond)
}

// VALIDATES: a peer without the Route Refresh capability, or a refused
// ROUTE-REFRESH, drops the admitted prefixes instead of asking forever.
func TestReadmitCannotAskDrops(t *testing.T) {
	for _, tc := range []struct {
		name string
		sess peerSession
		err  error
	}{
		{name: "no capability", sess: peerSession{established: true}},
		{name: "refused", sess: peerSession{established: true, routeRefresh: true}, err: errors.New(sendNotPermittedText + " (peers=127.0.0.1)")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := newFakeReadmitEngine()
			engine.setSession(tc.sess)
			engine.refreshErr = tc.err
			plug := readmitPlugin(t, fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"}), engine, time.Hour)
			announce(plug, "10.0.0.0/24")
			plug.refreshASN(65001)
			deadline := time.Now().Add(5 * time.Second)
			for plug.readmit.heldCount() != 0 {
				if time.Now().After(deadline) {
					t.Fatal("the admitted prefix stayed held")
				}
				time.Sleep(10 * time.Millisecond) // sleep(poll): heldCount has no change notification
			}
			expectNoRefresh(t, engine, 50*time.Millisecond)
		})
	}
}

// VALIDATES: the worker returns when its stop channel closes, mid-timer.
func TestReadmitWorkerStops(t *testing.T) {
	r := newReadmitter(newFakeReadmitEngine(), nil, 10*time.Millisecond)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		r.run(stop)
		close(done)
	}()
	r.notify()
	close(stop)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the worker did not stop")
	}
}

// waitHeld polls until the held set has want prefixes.
func waitHeld(t *testing.T, plug *irrPlugin, want int, why string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for plug.readmit.heldCount() != want {
		if time.Now().After(deadline) {
			t.Fatalf("%s: held = %d, want %d", why, plug.readmit.heldCount(), want)
		}
		time.Sleep(10 * time.Millisecond) // sleep(poll): heldCount has no change notification
	}
}

// VALIDATES: a reconfigure that removes a peer drops what that peer holds even
// when its ASN stays enrolled through another peer.
// METHOD: 127.0.0.1 holds a prefix for AS65001; the new config enrolls AS65001
// for 127.0.0.2 only. Keyed on the ASN alone, the stale prefix stayed held.
func TestReadmitRetainDropsRemovedPeer(t *testing.T) {
	irrAddr := fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"})
	plug := readmitPlugin(t, irrAddr, newFakeReadmitEngine(), time.Hour)
	plug.refreshASN(65001)
	announce(plug, "192.0.2.0/24")
	if got := plug.readmit.heldCount(); got != 1 {
		t.Fatalf("held = %d, want 1", got)
	}
	plug.handleConfigure(map[string]any{
		"policy": map[string]any{"irr": map[string]any{"server": irrAddr}},
		"peer": map[string]any{
			"peer2": map[string]any{
				"connection": map[string]any{"remote": map[string]any{"ip": "127.0.0.2"}},
				"session": map[string]any{
					"asn": map[string]any{"remote": float64(65001)},
					"irr": map[string]any{"as-set": "AS-TEST"},
				},
			},
		},
	})
	if got := plug.readmit.heldCount(); got != 0 {
		t.Errorf("held after the peer left the config = %d, want 0", got)
	}
}

// VALIDATES: an UPDATE for an ASN this config does not enroll holds nothing: no
// refresh of that ASN ever runs, so the hold could only leak.
func TestReadmitUnenrolledASNHoldsNothing(t *testing.T) {
	plug := readmitPlugin(t, fakeIRRv4(t, nil), newFakeReadmitEngine(), time.Hour)
	out := plug.handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "bgp-filter-irr:65099",
		Peer:   readmitTestPeer,
		Update: "nlri add 10.0.0.0/24",
	})
	if out.Action != sdk.FilterReject {
		t.Fatalf("action = %v, want reject", out.Action)
	}
	if got := plug.readmit.heldCount(); got != 0 {
		t.Errorf("held for an unenrolled ASN = %d, want 0", got)
	}
}

// VALIDATES: the timer drops what a peer the engine no longer knows holds.
func TestReadmitAbsentPeerDropsHeld(t *testing.T) {
	engine := newFakeReadmitEngine()
	engine.sessErr = errReadmitPeerAbsent
	plug := readmitPlugin(t, fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"}), engine, 20*time.Millisecond)
	plug.refreshASN(65001)
	announce(plug, "192.0.2.0/24")
	waitHeld(t, plug, 0, "a peer absent from the engine")
	expectNoRefresh(t, engine, 50*time.Millisecond)
}

// VALIDATES: a ROUTE-REFRESH that fails for a reason other than a refused
// permission (a timeout) keeps the admitted prefix held, and the next pass
// asks again.
func TestReadmitRefreshFailureRetries(t *testing.T) {
	engine := newFakeReadmitEngine()
	engine.setRefreshErr(context.DeadlineExceeded)
	plug := readmitPlugin(t, fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"}), engine, time.Hour)
	announce(plug, "10.0.0.0/24")
	plug.refreshASN(65001)
	expectNoRefresh(t, engine, 100*time.Millisecond)
	if got := plug.readmit.heldCount(); got != 1 {
		t.Fatalf("held after a timed-out refresh = %d, want 1", got)
	}
	engine.setRefreshErr(nil)
	plug.readmit.notify()
	waitRefresh(t, engine)
	waitHeld(t, plug, 0, "after the refresh that worked")
}

// VALIDATES: a peer that went down while its ROUTE-REFRESH was in flight is
// treated as down, because the engine answers a refresh to a peer it skipped
// with success.
// METHOD: the peer holds an admitted ipv4 prefix and an ipv6 prefix the list
// does not admit. After the refresh the session reads not established, so the
// ipv6 prefix must go too; without the re-read it stayed held.
func TestReadmitPeerDownDuringRefresh(t *testing.T) {
	engine := newFakeReadmitEngine()
	engine.afterRefresh = &peerSession{established: false, routeRefresh: true}
	plug := readmitPlugin(t, fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"}), engine, time.Hour)
	peer := netip.MustParseAddr(readmitTestPeer)
	holdPrefixes(plug.readmit, peer, "ipv6/unicast", []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")})
	announce(plug, "10.0.0.0/24")
	plug.refreshASN(65001)
	waitRefresh(t, engine)
	waitHeld(t, plug, 0, "a peer that went down during the refresh")
}

// VALIDATES: an UPDATE carrying an announce block and a withdraw block is
// tracked block by block: the withdrawal forgets what it names, and the
// announced prefix the list rejects is held.
func TestReadmitMultiBlockUpdate(t *testing.T) {
	plug := readmitPlugin(t, fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"}), newFakeReadmitEngine(), time.Hour)
	plug.refreshASN(65001)
	peer := netip.MustParseAddr(readmitTestPeer)
	withdrawn := heldKey{peer: peer, family: "ipv4/unicast", prefix: netip.MustParsePrefix("198.51.100.0/24")}
	rejected := heldKey{peer: peer, family: "ipv4/unicast", prefix: netip.MustParsePrefix("192.0.2.0/24")}
	holdPrefixes(plug.readmit, peer, withdrawn.family, []netip.Prefix{withdrawn.prefix})
	out := plug.handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "bgp-filter-irr:65001",
		Peer:   readmitTestPeer,
		Update: "origin igp nlri add 10.0.0.0/24 192.0.2.0/24 nlri del 198.51.100.0/24",
	})
	if out.Action != sdk.FilterModify {
		t.Fatalf("action = %v, want modify", out.Action)
	}
	plug.readmit.mu.Lock()
	_, stillWithdrawn := plug.readmit.held[withdrawn]
	_, heldRejected := plug.readmit.held[rejected]
	plug.readmit.mu.Unlock()
	if stillWithdrawn {
		t.Error("the withdrawn prefix is still held")
	}
	if !heldRejected {
		t.Error("the rejected announced prefix is not held")
	}
}

// VALIDATES: a refresh that fails does not hold again a prefix the filter
// accepted while the request was in flight.
// PREVENTS: restore re-holding a key an accept deleted between take and
// restore, so the worker later asked the peer for a route it already has.
// METHOD: readmit is called directly, without the worker; the fake engine
// tracks an accept of the prefix inside refresh and then answers a timeout.
func TestReadmitRestoreSkipsAcceptedDuringRefresh(t *testing.T) {
	engine := newFakeReadmitEngine()
	plug := readmitPlugin(t, fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"}), engine, time.Hour)
	peer := netip.MustParseAddr(readmitTestPeer)
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	holdPrefixes(plug.readmit, peer, "ipv4/unicast", []netip.Prefix{prefix})
	engine.refreshErr = context.DeadlineExceeded
	engine.duringRefresh = func() {
		accepted := nlriBlock{family: "ipv4/unicast", op: nlriOpAnnounce, prefixes: []netip.Prefix{prefix}, admitted: []bool{true}}
		plug.readmit.track(peer, 65001, true, &accepted, false)
	}
	key := heldKey{peer: peer, family: "ipv4/unicast", prefix: prefix}
	plug.readmit.readmit(peerFamily{peer: peer, family: "ipv4/unicast"}, []heldKey{key})
	if got := plug.readmit.heldCount(); got != 0 {
		t.Fatalf("held after a failed refresh of a prefix accepted meanwhile = %d, want 0", got)
	}
}

// VALIDATES: after a ROUTE-REFRESH went out, only an absent or a down peer
// drops what it holds; a session read that fails for another reason does not.
// PREVENTS: a failed confirming read, a timeout for one, dropping the peer's
// held prefixes of every other family, which nothing then asks for again.
// METHOD: readmit is called directly, without the worker, for the admitted
// ipv4 prefix; the peer also holds an ipv6 prefix the list does not admit.
func TestReadmitConfirmReadFailureKeepsHeld(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{name: "read failed", err: context.DeadlineExceeded, want: 1},
		{name: "peer absent", err: errReadmitPeerAbsent, want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := newFakeReadmitEngine()
			engine.afterRefreshErr = tc.err
			plug := readmitPlugin(t, fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"}), engine, time.Hour)
			peer := netip.MustParseAddr(readmitTestPeer)
			holdPrefixes(plug.readmit, peer, "ipv6/unicast", []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")})
			prefix := netip.MustParsePrefix("10.0.0.0/24")
			holdPrefixes(plug.readmit, peer, "ipv4/unicast", []netip.Prefix{prefix})
			key := heldKey{peer: peer, family: "ipv4/unicast", prefix: prefix}
			plug.readmit.readmit(peerFamily{peer: peer, family: "ipv4/unicast"}, []heldKey{key})
			waitRefresh(t, engine)
			if got := plug.readmit.heldCount(); got != tc.want {
				t.Fatalf("held after the confirming read = %d, want %d", got, tc.want)
			}
		})
	}
}

// VALIDATES: an UPDATE rejected whole holds every prefix it announced, the
// ones the list admits too, so the next pass asks the peer for them again.
// PREVENTS: the admitted prefix of a whole reject (an MP_REACH_NLRI prefix the
// list rejects beside it) being dropped from the held set as if accepted, so
// nothing asked for it again.
// METHOD: the list admits 10.0.0.0/24 only; the UPDATE announces it beside a
// FlowSpec rule whose destination the list rejects, a block the engine cannot
// rewrite from a modify (non-CIDR family), so the filter rejects the UPDATE; a
// pass must then ask for ipv4/unicast.
func TestReadmitWholeRejectHoldsEveryAnnounced(t *testing.T) {
	engine := newFakeReadmitEngine()
	plug := readmitPlugin(t, fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"}), engine, time.Hour)
	plug.refreshASN(65001)
	out := plug.handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "bgp-filter-irr:65001",
		Peer:   readmitTestPeer,
		Update: "origin igp nlri add 10.0.0.0/24 mp ipv4/flow add 192.0.2.0/24",
	})
	if out.Action != sdk.FilterReject {
		t.Fatalf("action = %v, want reject", out.Action)
	}
	plug.readmit.notify()
	if pf := waitRefresh(t, engine); pf.family != "ipv4/unicast" {
		t.Fatalf("refresh asked for %q, want ipv4/unicast", pf.family)
	}
	peer := netip.MustParseAddr(readmitTestPeer)
	flow := heldKey{peer: peer, family: "ipv4/flow", prefix: netip.MustParsePrefix("192.0.2.0/24")}
	plug.readmit.mu.Lock()
	_, heldFlow := plug.readmit.held[flow]
	plug.readmit.mu.Unlock()
	if !heldFlow {
		t.Error("the rejected FlowSpec destination is not held")
	}
}

// VALIDATES: an UPDATE announcing an admitted legacy prefix beside an
// MP_REACH_NLRI block of a CIDR family that holds one refused prefix is a
// modify whose delta keeps the admitted MP prefix and drops the refused one,
// and only the refused prefix is held, so no pass asks the peer again.
// PREVENTS: the whole reject such an UPDATE used to get, which held the
// admitted legacy prefix; the list admitted it, so every pass asked the peer
// for ipv4/unicast, the peer resent the same UPDATE, and it was rejected again,
// once a minute for as long as the session lasted.
// METHOD: the list admits 10.0.0.0/24 and 10.1.0.0/24; the UPDATE announces
// 10.0.0.0/24 legacy and 10.1.0.0/24 plus 192.0.2.0/24 in MP_REACH_NLRI; a
// pass is forced and no ROUTE-REFRESH may follow.
func TestReadmitMixedMPModifyHoldsOnlyRefused(t *testing.T) {
	engine := newFakeReadmitEngine()
	plug := readmitPlugin(t, fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24 10.1.0.0/24"}), engine, time.Hour)
	plug.refreshASN(65001)
	out := plug.handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "bgp-filter-irr:65001",
		Peer:   readmitTestPeer,
		Update: "origin igp nlri add 10.0.0.0/24 mp ipv4/unicast add 10.1.0.0/24 192.0.2.0/24",
	})
	if out.Action != sdk.FilterModify {
		t.Fatalf("action = %v, want modify", out.Action)
	}
	if want := "nlri add 10.0.0.0/24 mp ipv4/unicast add 10.1.0.0/24"; out.Update != want {
		t.Fatalf("delta = %q, want %q", out.Update, want)
	}
	peer := netip.MustParseAddr(readmitTestPeer)
	refused := heldKey{peer: peer, family: "ipv4/unicast", prefix: netip.MustParsePrefix("192.0.2.0/24")}
	plug.readmit.mu.Lock()
	_, heldRefused := plug.readmit.held[refused]
	held := len(plug.readmit.held)
	plug.readmit.mu.Unlock()
	if !heldRefused || held != 1 {
		t.Fatalf("held = %d (refused prefix held: %v), want the refused prefix alone", held, heldRefused)
	}
	plug.readmit.notify()
	expectNoRefresh(t, engine, 300*time.Millisecond)
}
