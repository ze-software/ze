// Design: docs/architecture/plugin/rib-storage-design.md -- R-12, a recovery write whose survivor was purged.
// Related: rs_withdrawal_replacement_test.go -- the route-server replacement probes this extends.
// Related: rs_withdrawal_ownership_test.go -- the recipient TCP projection these tests read.
// RFC 4271 Section 9.2 -- see rfc/short/rfc4271.md.
package reactor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
)

// TestOwedKeyPurgeBetweenLookupAndAdmission proves R-12 of
// spec-bgp-rs-replacement-on-withdrawal over real sessions and the real
// bgp-rib. Sources A and B (and, in one case, D) announce 10.9.0.0/24; A is
// the newest, so A owns the prefix at client C (no ADD-PATH). B's routes are
// marked GR stale. A withdraws: C's writer owes the key, and bgp-rib's lookup
// elects B's stale path. Inside that lookup, after the RIB answered, the test
// purges B's stale routes exactly as End-of-RIB does (`request bgp rib
// purge-stale`), so the purge lands between the lookup and the admission.
//
// The purge emits no removal for C, because B does not own C's entry yet, so
// the admission writes B's purged route. R-12 then removes it at the sent
// projection: the recovery write's sent event carries recovery-write, bgp-rib
// finds B's received path gone and sends the removal, which C's writer owes
// again. C must end on the next survivor (D) or with one withdrawal, never on
// the purged route.
//
// VALIDATES: R-12, a survivor purged between lookup and admission never stays
// at the destination.
// PREVENTS: C keeping a route its source no longer has.
func TestOwedKeyPurgeBetweenLookupAndAdmission(t *testing.T) {
	t.Run("next-survivor", func(t *testing.T) { purgeBetweenLookupAndAdmission(t, true) })
	t.Run("no-survivor", func(t *testing.T) { purgeBetweenLookupAndAdmission(t, false) })
}

func purgeBetweenLookupAndAdmission(t *testing.T, withD bool) {
	settings := []*PeerSettings{
		lowLiveSettings("192.0.2.1", 65000, 65002),
		lowLiveSettings("192.0.2.2", 65000, 65003),
		lowLiveSettings("192.0.2.3", 65000, 65004),
	}
	if withD {
		settings = append(settings, lowLiveSettings("192.0.2.4", 65000, 65005))
	}
	for _, setting := range settings {
		setting.RSClient = true
	}
	peers := lowLiveRouter(t, settings...)
	a, b, c := peers[0], peers[1], peers[2]
	r := a.peer.reactor
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	// Each AS_PATH is one AS longer than the last, so A, C's owner, is also
	// the RIB's best, B is elected once A withdraws, and D only once B is gone
	// (RFC 4271 Section 9.1.2.2, step a).
	attrsA := firstASAttrs(4, 65002)
	attrsB := firstASAttrs(4, 65003, 65003)
	attrsD := firstASAttrs(4, 65005, 65005, 65005)

	if withD {
		ownershipLiveSend(t, peers[3], nil, attrsD, prefix) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, c, prefix, attrsD)
	}
	ownershipLiveSend(t, b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefix, attrsB)
	ownershipLiveSend(t, a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefix, attrsA)

	source := b.peer.Settings().Address
	// RFC 4724 Section 4.2: the Receiving Speaker MUST retain the routes and
	// mark them stale. Level 1 is GR stale, which the decision does not
	// depreference, and restart time 0 starts no expiry timer.
	purgeRIBCommand(t, r, "request bgp rib mark-stale", source.String(), "0", "1", "ipv4/unicast")

	purge := &lookupPurge{r: r, destination: c.peer.Settings().Address, source: source}
	publishLookupTap(t, purge.after)

	marker := netip.MustParsePrefix("10.9.255.0/24")
	ownershipLiveSend(t, a, []netip.Prefix{prefix}, nil) // RFC 4271 Section 4.3.
	ownershipLiveSend(t, a, nil, attrsA, marker)         // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, marker, attrsA)

	deadline := time.Now().Add(5 * time.Second)
	view := ownershipLiveRead(t, c)
	for !purgeConverged(view, prefix, withD, attrsD) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
		view = ownershipLiveRead(t, c)
	}

	purged, err := purge.result()
	if err != nil {
		t.Fatalf("purge inside C's lookup: %v", err)
	}
	if purged != 1 {
		t.Fatalf("purge-stale inside C's lookup purged %d routes of %s, want 1 (the lookup never elected B first?)", purged, source)
	}
	// RFC 4271 Section 9.2: "All newly installed routes and all newly
	// unfeasible routes for which there is no replacement route SHALL be
	// advertised to its peers by means of an UPDATE message."
	if withD {
		ownershipLiveRoute(t, view, prefix, attrsD)
		ownershipLiveWithdrawals(t, view, prefix, 0)
		// B's write is the admission of the purged survivor; R-12 replaces
		// it with D.
		ownershipLiveAnnouncements(t, view, prefix, attrsD, attrsB, attrsA, attrsB, attrsD)
	} else {
		ownershipLiveAbsent(t, view, prefix)
		ownershipLiveWithdrawals(t, view, prefix, 1)
		ownershipLiveAnnouncements(t, view, prefix, attrsB, attrsA, attrsB)
	}

	// Nothing writes the purged route back: a later marker from A passes C's
	// writer after every decision for the prefix, and C's view is unchanged.
	later := netip.MustParsePrefix("10.9.254.0/24")
	ownershipLiveSend(t, a, nil, attrsA, later) // RFC 4271 Section 4.3.
	settled := ownershipLiveThrough(t, c, later, attrsA)
	if withD {
		ownershipLiveRoute(t, settled, prefix, attrsD)
	} else {
		ownershipLiveAbsent(t, settled, prefix)
	}
	if got := len(settled.announcements[prefix]); got != len(view.announcements[prefix]) {
		t.Errorf("C received %d announcements of %s after convergence, want %d", got, prefix, len(view.announcements[prefix]))
	}
	ownershipLiveEstablished(t, peers)
}

// purgeConverged reports whether C holds its expected final state: D's route,
// or no route once the purged write was withdrawn.
func purgeConverged(view ownershipLiveProjection, prefix netip.Prefix, withD bool, attrsD []byte) bool {
	if withD {
		return bytes.Equal(view.routes[prefix], attrsD) && len(view.announcements[prefix]) >= 5
	}
	_, held := view.routes[prefix]
	return !held && view.withdrawals[prefix] >= 1
}

// lookupPurge wraps bgp-rib's in-process recovery lookup. The first lookup for
// the destination whose first survivor is the stale source purges that
// source's stale routes after the RIB answered and before the reactor admits
// the answer. It is a publishLookupTap hook, called from recovery workers, so
// the state is locked.
type lookupPurge struct {
	r           *Reactor
	destination netip.Addr
	source      netip.Addr

	mu     sync.Mutex
	done   bool
	purged int
	err    error
}

func (p *lookupPurge) after(request *ribevents.RecoveryRequest, outcomes []ribevents.RecoveryOutcome, err error) ([]ribevents.RecoveryOutcome, error) {
	if err != nil {
		return outcomes, err
	}
	if request.Destination != p.destination {
		return outcomes, nil
	}
	if !electsSource(outcomes, p.source) {
		return outcomes, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done {
		return outcomes, nil
	}
	p.done = true
	p.purged, p.err = purgeRIBStale(p.r, p.source)
	return outcomes, nil
}

func (p *lookupPurge) result() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.done {
		return 0, errors.New("C's recovery lookup never elected the stale source first")
	}
	return p.purged, p.err
}

// lookupTapHook sees each recovery lookup after bgp-rib answered it, and
// returns the answer the reactor receives. It runs on recovery workers, so it
// MUST be safe for concurrent use and MUST NOT call testing.T's fatal methods.
type lookupTapHook func(request *ribevents.RecoveryRequest, outcomes []ribevents.RecoveryOutcome, err error) ([]ribevents.RecoveryOutcome, error)

// publishLookupTap replaces bgp-rib's in-process recovery owner with one that
// calls the real lookup, then hook. The tap is closed at cleanup, before the
// router stops, so no later test sees it; bgp-rib's own Close then finds
// another owner gone and leaves the slot empty.
func publishLookupTap(t *testing.T, hook lookupTapHook) {
	t.Helper()
	ribOwner := ribevents.RecoveryProvider()
	if ribOwner == nil {
		t.Fatal("bgp-rib published no in-process recovery owner")
	}
	tap := ribevents.PublishRecovery(func(request ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
		outcomes, err := ribOwner.Lookup(request)
		return hook(&request, outcomes, err)
	})
	t.Cleanup(tap.Close)
}

// electsSource reports whether the first survivor of any outcome is source.
func electsSource(outcomes []ribevents.RecoveryOutcome, source netip.Addr) bool {
	for i := range outcomes {
		if len(outcomes[i].Survivors) == 0 {
			continue
		}
		if outcomes[i].Survivors[0].Source == source {
			return true
		}
	}
	return false
}

// purgeRIBStale runs GR's End-of-RIB command for source's IPv4 unicast stale
// routes and returns the purged count.
func purgeRIBStale(r *Reactor, source netip.Addr) (int, error) {
	data, err := dispatchRIBCommand(r, "request bgp rib purge-stale", source.String(), "ipv4/unicast")
	if err != nil {
		return 0, err
	}
	var result struct {
		Purged int `json:"purged"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return 0, err
	}
	return result.Purged, nil
}

func purgeRIBCommand(t *testing.T, r *Reactor, command string, args ...string) {
	t.Helper()
	if _, err := dispatchRIBCommand(r, command, args...); err != nil {
		t.Fatalf("%s %v: %v", command, args, err)
	}
}

// dispatchRIBCommand sends one registered bgp-rib command through the same
// dispatcher the reactor's recovery lookup uses over IPC.
func dispatchRIBCommand(r *Reactor, command string, args ...string) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	commandContext := &pluginserver.CommandContext{Server: r.api, RequestContext: ctx}
	response, err := r.api.Dispatcher().ForwardToPlugin(commandContext, command, args, "*")
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, errors.New(command + ": no answer")
	}
	if response.Error != "" {
		return nil, errors.New(command + ": " + response.Error)
	}
	data, ok := response.Data.(plugin.RawJSON)
	if !ok {
		return nil, errors.New(command + ": answer is not a JSON document")
	}
	return json.RawMessage(data), nil
}

// TestOwedKeyLookupChunked proves AC-21: C owes 2000 keys of one family, and
// bgp-rib receives every lookup for them in requests of at most
// recoveryLookupKeysMax keys, and every key is decided. A owns all 2000
// prefixes at C (B announced them first, A's AS_PATH is shorter).
//
// "departure" is AC-21 as stated: A's session ends with no NOTIFICATION and
// bgp-rs runs departure recovery. bgp-rs already sends a departure in batches
// of 500 (withdrawalBatchSize), and the reactor waits on each batch's records,
// so that path alone never hands one round more than 500 keys.
// "withdrawal-burst" makes the reactor's own bound the one that decides: A
// withdraws all 2000 in three UPDATEs while C's first lookup is held, so the
// next round snapshots more than 500 owed keys at once.
//
// VALIDATES: AC-21, a bounded lookup per request and an unbounded owed set.
// PREVENTS: one unbounded RIB document per round, or keys left undecided past
// the first chunk.
func TestOwedKeyLookupChunked(t *testing.T) {
	t.Run("departure", func(t *testing.T) { lookupChunked(t, false) })
	t.Run("withdrawal-burst", func(t *testing.T) { lookupChunked(t, true) })
}

func lookupChunked(t *testing.T, burst bool) {
	const keys = 2000
	settings := []*PeerSettings{
		lowLiveSettings("192.0.2.1", 65000, 65002),
		lowLiveSettings("192.0.2.2", 65000, 65003),
		lowLiveSettings("192.0.2.3", 65000, 65004),
	}
	for _, setting := range settings {
		setting.RSClient = true
	}
	peers := lowLiveRouter(t, settings...)
	a, b, c := peers[0], peers[1], peers[2]
	attrsA := firstASAttrs(4, 65002)
	attrsB := firstASAttrs(4, 65003, 65003)
	prefixes := make([]netip.Prefix, keys)
	for i := range prefixes {
		prefixes[i] = netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(20 + i/256), byte(i % 256), 0}), 24)
	}
	// RFC 4271 Section 4.3: 900 four-octet NLRI tuples keep each UPDATE
	// under the 4096-octet maximum of Section 4.
	const perUpdate = 900
	for _, source := range []struct {
		peer  *lowLivePeer
		attrs []byte
	}{{b, attrsB}, {a, attrsA}} {
		for start := 0; start < keys; start += perUpdate {
			end := min(start+perUpdate, keys)
			ownershipLiveSend(t, source.peer, nil, source.attrs, prefixes[start:end]...) // RFC 4271 Section 4.3.
			ownershipLiveThrough(t, c, prefixes[end-1], source.attrs)
		}
	}
	lowEventually(t, func() bool { return adjOutOwnedBy(c.peer, a.peer) == keys }, "A owns every prefix at C")

	destination := c.peer.Settings().Address
	tap := &lookupSizes{destination: destination, seen: make(map[string]struct{})}
	if burst {
		tap.hold = make(chan struct{})
	}
	publishLookupTap(t, tap.after)

	if burst {
		for start := 0; start < keys; start += perUpdate {
			end := min(start+perUpdate, keys)
			ownershipLiveSend(t, a, prefixes[start:end], nil) // RFC 4271 Section 4.3.
		}
		lowEventually(t, func() bool { return adjOutOwedCount(c.peer) == keys }, "C owes every withdrawn key")
		close(tap.hold)
	} else {
		_ = a.remote.Close() // A's session ends with no NOTIFICATION: a departure.
	}

	// Under -race a departure of 2000 keys takes up to 20 seconds here.
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if adjOutOwnedBy(c.peer, b.peer) == keys && adjOutOwedCount(c.peer) == 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	// An admission of more keys than the bound at once exhausted the read
	// buffer pool and reset sessions (the recorded red, with the chunk
	// removed), so every session that did not depart must still be up.
	remaining := []*lowLivePeer{b, c}
	if burst {
		remaining = peers
	}
	for _, peer := range remaining {
		if got := peer.peer.SessionState(); got != fsm.StateEstablished {
			t.Errorf("peer %s session = %s, want Established", peer.peer.Settings().Address, got)
		}
	}
	if got := adjOutOwnedBy(c.peer, b.peer); got != keys {
		t.Fatalf("B owns %d of %d prefixes at C", got, keys)
	}
	if got := adjOutOwedCount(c.peer); got != 0 {
		t.Fatalf("C still owes %d keys", got)
	}
	requests, largest, distinct := tap.result()
	t.Logf("C's lookups: %d requests, largest %d keys, %d distinct keys", requests, largest, distinct)
	if largest > recoveryLookupKeysMax {
		t.Errorf("largest lookup for C held %d keys, want at most %d", largest, recoveryLookupKeysMax)
	}
	if distinct != keys {
		t.Errorf("C's lookups named %d distinct keys, want %d", distinct, keys)
	}
	if requests < keys/recoveryLookupKeysMax {
		t.Errorf("C's keys reached bgp-rib in %d requests, want at least %d", requests, keys/recoveryLookupKeysMax)
	}
}

// lookupSizes records, for one destination, how many recovery lookups reached
// bgp-rib, the largest key count of one, and every distinct key named. It is
// a publishLookupTap hook, so its state is locked.
type lookupSizes struct {
	destination netip.Addr
	// hold, when set, parks the first lookup for the destination until the
	// test closes it, at most 10 seconds.
	hold chan struct{}

	mu       sync.Mutex
	requests int
	largest  int
	seen     map[string]struct{}
}

func (s *lookupSizes) after(request *ribevents.RecoveryRequest, outcomes []ribevents.RecoveryOutcome, err error) ([]ribevents.RecoveryOutcome, error) {
	if request.Destination != s.destination {
		return outcomes, err
	}
	s.mu.Lock()
	s.requests++
	first := s.requests == 1
	s.largest = max(s.largest, len(request.Keys))
	for i := range request.Keys {
		s.seen[string(request.Keys[i].NLRI)] = struct{}{}
	}
	s.mu.Unlock()
	if first && s.hold != nil {
		select {
		case <-s.hold:
		case <-time.After(10 * time.Second):
		}
	}
	return outcomes, err
}

func (s *lookupSizes) result() (requests, largest, distinct int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests, s.largest, len(s.seen)
}

// adjOutOwnedBy counts the destination's Adj-RIB-Out entries owned by source.
func adjOutOwnedBy(destination, source *Peer) int {
	table := &destination.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()
	owned := 0
	for _, route := range table.routes {
		if route.owner.source == source {
			owned++
		}
	}
	return owned
}

// adjOutOwedCount counts the destination's owed keys.
func adjOutOwedCount(destination *Peer) int {
	table := &destination.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()
	return len(table.owed)
}
