// Design: docs/architecture/plugin/rib-storage-design.md -- R-12, a recovery write whose survivor was purged.
// Related: rfc7947_rs_withdrawal_replacement_test.go -- the route-server replacement probes this extends.
// Related: rs_withdrawal_ownership_test.go -- the recipient TCP projection these tests read.
// RFC 4271 Section 9.2 -- see rfc/short/rfc4271.md.
package reactor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"net/netip"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/retention"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/family"
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

// adjOutPrefixOwner returns the source owning destination's Adj-RIB-Out
// entry for an IPv4 unicast prefix, or nil when it holds none.
func adjOutPrefixOwner(t *testing.T, destination *Peer, prefix netip.Prefix) *Peer {
	t.Helper()
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	var key adjOutKey
	raw := ownershipLiveNLRI([]netip.Prefix{prefix})
	if err := fwdPathKeyFor(&key.path, family.IPv4Unicast, 0, raw, false, scratch[:]); err != nil {
		t.Fatalf("key for %s: %v", prefix, err)
	}
	table := &destination.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()
	entry := table.routes[key]
	if entry == nil {
		return nil
	}
	return entry.owner.source
}

// adjOutOwedCount counts the destination's owed keys.
func adjOutOwedCount(destination *Peer) int {
	table := &destination.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()
	return len(table.owed)
}

// TestOwedKeyLivenessUnderChurn proves AC-15. A owns 10.9.0.0/24 at client C
// and withdraws it; B is the survivor of choice. C's first lookup for the key
// is held after bgp-rib answered, and while it is held:
//
//   - "other-keys": B announces 1000 other prefixes and E announces 1000
//     more, all relayed to C, so B's session accepts UPDATEs after the round's
//     cut and C's writer keeps writing;
//   - "same-prefix": D, which announced the key earlier with a worse route
//     and is not the chosen survivor, and E, which never announced it, each
//     send 1000 withdrawals of the key, every one dropped at C's writer as a
//     non-owner's.
//
// None of that traffic withdrew the chosen candidate's route, merged a
// departure or ended a session, so the key is decided by its first round: it
// reaches bgp-rib in exactly one lookup, and C receives B's route and no
// withdrawal of the key.
//
// VALIDATES: AC-15, an owed key is decided within one round whatever the
// traffic on other keys, other sources and non-candidate withdrawals.
// PREVENTS: a busy survivor or a busy client postponing a replacement forever.
func TestOwedKeyLivenessUnderChurn(t *testing.T) {
	t.Run("other-keys", func(t *testing.T) { livenessUnderChurn(t, false) })
	t.Run("same-prefix", func(t *testing.T) { livenessUnderChurn(t, true) })
}

func livenessUnderChurn(t *testing.T, samePrefix bool) {
	const churn = 1000
	settings := []*PeerSettings{
		lowLiveSettings("192.0.2.1", 65000, 65002),
		lowLiveSettings("192.0.2.2", 65000, 65003),
		lowLiveSettings("192.0.2.3", 65000, 65004),
		lowLiveSettings("192.0.2.4", 65000, 65005),
		lowLiveSettings("192.0.2.5", 65000, 65006),
	}
	for _, setting := range settings {
		setting.RSClient = true
	}
	peers := rsLiveRouter(t, settings...)
	a, b, c, d, e := peers[0], peers[1], peers[2], peers[3], peers[4]
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	// RFC 4271 Section 9.1.2.2, step a: A's path is the shortest, then B's,
	// then D's, so B is the survivor of choice once A withdraws.
	attrsA := firstASAttrs(4, 65002)
	attrsB := firstASAttrs(4, 65003, 65003)
	attrsD := firstASAttrs(4, 65005, 65005, 65005)
	attrsE := firstASAttrs(4, 65006)

	ownershipLiveSend(t, d, nil, attrsD, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefix, attrsD)
	ownershipLiveSend(t, b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefix, attrsB)
	ownershipLiveSend(t, a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefix, attrsA)

	tap := &keyLookups{destination: c.peer.Settings().Address, key: ownershipLiveNLRI([]netip.Prefix{prefix}),
		held: make(chan struct{}), hold: make(chan struct{})}
	publishLookupTap(t, tap.after)

	// C is sent thousands of churn UPDATEs, more than its wire capture
	// keeps, so what C is sent for the key after A's withdrawal is read from
	// the reactor's sent events.
	sent := observeSentKey(t, a.peer.reactor, c.peer.Settings().Address, prefix)
	ownershipLiveSend(t, a, []netip.Prefix{prefix}, nil) // RFC 4271 Section 4.3.
	select {
	case <-tap.held:
	case <-time.After(5 * time.Second):
		t.Fatal("C's recovery lookup for the owed key never ran")
	}

	churnB := churnPrefixes(40, churn)
	churnE := churnPrefixes(50, churn)
	for i := range churn {
		if samePrefix {
			ownershipLiveSend(t, d, []netip.Prefix{prefix}, nil) // RFC 4271 Section 4.3.
			ownershipLiveSend(t, e, []netip.Prefix{prefix}, nil) // RFC 4271 Section 4.3.
			continue
		}
		ownershipLiveSend(t, b, nil, attrsB, churnB[i]) // RFC 4271 Section 4.3.
		ownershipLiveSend(t, e, nil, attrsE, churnE[i]) // RFC 4271 Section 4.3.
	}
	if !samePrefix {
		// Every churn route is held at C: the churn reached C's writer while
		// the round was held. The table is read, not the wire, because
		// decoding thousands of frames per poll would stall C's reader.
		waitOwnedBy(t, c.peer, b.peer, churn)
		waitOwnedBy(t, c.peer, e.peer, churn)
	}
	close(tap.hold)

	deadline := time.Now().Add(30 * time.Second)
	for adjOutOwedCount(c.peer) != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	for len(sent.announced()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	// RFC 4271 Section 9.2: "All newly installed routes and all newly
	// unfeasible routes for which there is no replacement route SHALL be
	// advertised to its peers by means of an UPDATE message."
	announced, withdrawn := sent.announced(), sent.withdrawn()
	if len(announced) != 1 || !bytes.Equal(announced[0], attrsB) {
		t.Errorf("C was sent %x for %s after A's withdrawal, want B's route %x once", announced, prefix, attrsB)
	}
	if withdrawn != 0 {
		t.Errorf("C was sent %d withdrawals of %s, want none", withdrawn, prefix)
	}
	if got := adjOutOwedCount(c.peer); got != 0 {
		t.Errorf("C still owes %d keys", got)
	}
	if got := tap.count(); got != 1 {
		t.Errorf("the owed key reached bgp-rib in %d lookups for C, want 1 (a retry without a fence reason)", got)
	}
	ownershipLiveEstablished(t, peers)
}

// waitOwnedBy waits up to 60 seconds, the churn's cost under -race, until
// source owns want entries at destination.
func waitOwnedBy(t *testing.T, destination, source *Peer, want int) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for adjOutOwnedBy(destination, source) != want {
		if time.Now().After(deadline) {
			t.Fatalf("%s owns %d entries at %s, want %d", source.Settings().Address,
				adjOutOwnedBy(destination, source), destination.Settings().Address, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// sentKeyObserver records what the reactor sends one destination for one
// IPv4 prefix: each announcement's path attributes and the withdrawal count.
// It wraps the reactor's message receiver and changes nothing; sent events
// arrive on the reactor's goroutines, so its state is locked.
type sentKeyObserver struct {
	MessageReceiver
	destination netip.Addr
	tuple       []byte

	mu          sync.Mutex
	attrs       [][]byte
	withdrawals int
}

// observeSentKey installs a sentKeyObserver until the test ends.
func observeSentKey(t *testing.T, r *Reactor, destination netip.Addr, prefix netip.Prefix) *sentKeyObserver {
	t.Helper()
	r.mu.RLock()
	prior := r.messageReceiver
	r.mu.RUnlock()
	observer := &sentKeyObserver{MessageReceiver: prior, destination: destination,
		tuple: ownershipLiveNLRI([]netip.Prefix{prefix})}
	r.setMessageReceiver(observer)
	t.Cleanup(func() { r.setMessageReceiver(prior) })
	return observer
}

// OnMessageSent decodes RFC 4271 Section 4.3's UPDATE body:
//
//	| Withdrawn Routes Length (2) | Withdrawn Routes | Total Path Attribute Length (2) | Path Attributes | NLRI |
func (o *sentKeyObserver) OnMessageSent(peer *plugin.PeerInfo, msg bgptypes.RawMessage) {
	o.MessageReceiver.OnMessageSent(peer, msg)
	body := msg.RawBytes
	if peer.Address != o.destination || len(body) < 4 {
		return
	}
	withdrawnLen := int(body[0])<<8 | int(body[1])
	if len(body) < 4+withdrawnLen {
		return
	}
	withdrawn := body[2 : 2+withdrawnLen]
	attrsLen := int(body[2+withdrawnLen])<<8 | int(body[3+withdrawnLen])
	if len(body) < 4+withdrawnLen+attrsLen {
		return
	}
	attrs := body[4+withdrawnLen : 4+withdrawnLen+attrsLen]
	nlriField := body[4+withdrawnLen+attrsLen:]
	o.mu.Lock()
	defer o.mu.Unlock()
	if sentFieldHolds(withdrawn, o.tuple) {
		o.withdrawals++
	}
	if sentFieldHolds(nlriField, o.tuple) {
		o.attrs = append(o.attrs, bytes.Clone(attrs))
	}
}

func (o *sentKeyObserver) announced() [][]byte {
	o.mu.Lock()
	defer o.mu.Unlock()
	return slices.Clone(o.attrs)
}

func (o *sentKeyObserver) withdrawn() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.withdrawals
}

// sentFieldHolds reports whether an IPv4 prefix field (RFC 4271 Section 4.3,
// length octet then prefix octets) holds tuple. A malformed field holds
// nothing.
func sentFieldHolds(field, tuple []byte) bool {
	for len(field) > 0 {
		octets := 1 + (int(field[0])+7)/8
		if octets > len(field) {
			return false
		}
		if bytes.Equal(field[:octets], tuple) {
			return true
		}
		field = field[octets:]
	}
	return false
}

// churnPrefixes returns count distinct /24s under 10.<base>.0.0, one per
// churn UPDATE.
func churnPrefixes(base, count int) []netip.Prefix {
	prefixes := make([]netip.Prefix, count)
	for i := range prefixes {
		prefixes[i] = netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(base + i/256), byte(i % 256), 0}), 24)
	}
	return prefixes
}

// keyLookups counts the recovery lookups for one destination that name one
// key, and holds the first of them until the test closes hold, at most 30
// seconds. It is a publishLookupTap hook, so its state is locked.
type keyLookups struct {
	destination netip.Addr
	key         []byte
	// held is closed when the first lookup naming the key is parked.
	held chan struct{}
	hold chan struct{}

	mu      sync.Mutex
	lookups int
}

func (k *keyLookups) after(request *ribevents.RecoveryRequest, outcomes []ribevents.RecoveryOutcome, err error) ([]ribevents.RecoveryOutcome, error) {
	if request.Destination != k.destination {
		return outcomes, err
	}
	named := false
	for i := range request.Keys {
		if bytes.Equal(request.Keys[i].NLRI, k.key) {
			named = true
		}
	}
	if !named {
		return outcomes, err
	}
	k.mu.Lock()
	k.lookups++
	first := k.lookups == 1
	k.mu.Unlock()
	if first {
		close(k.held)
		select {
		case <-k.hold:
		case <-time.After(30 * time.Second):
		}
	}
	return outcomes, err
}

func (k *keyLookups) count() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.lookups
}

// TestOwedKeyBufferShortageDefersKeys proves that a read-buffer shortage
// inside a recovery admission defers keys rather than retiring the
// destination. Each staged key holds a buffer of the 4K pool, which every
// session's reads share, until its write completes, so a chunk of owed keys
// can find the pool dry. A owns 300 prefixes at C and withdraws them while
// C's first lookup is held; the test then pins the pool's byte budget at what
// is allocated and holds every free buffer:
//
//   - "partial": it gives 50 buffers back before releasing the lookup, so
//     each admission decides the keys it found buffers for and the next round
//     takes the rest;
//   - "stalled": it gives none back, so the first rounds decide nothing and
//     the worker backs off; 300 ms later it returns them all.
//
// Either way every key is decided (B owns all 300 at C) and every session
// stays Established: before this, the admission's buffer error retired C.
//
// VALIDATES: AC-12 and AC-21, a transient pool shortage costs time, never the
// client's session.
// PREVENTS: a route-server client reset because other sessions held buffers.
func TestOwedKeyBufferShortageDefersKeys(t *testing.T) {
	t.Run("partial", func(t *testing.T) { bufferShortageDefersKeys(t, 50) })
	t.Run("stalled", func(t *testing.T) { bufferShortageDefersKeys(t, 0) })
}

func bufferShortageDefersKeys(t *testing.T, spare int) {
	const keys = 300
	settings := []*PeerSettings{
		lowLiveSettings("192.0.2.1", 65000, 65002),
		lowLiveSettings("192.0.2.2", 65000, 65003),
		lowLiveSettings("192.0.2.3", 65000, 65004),
	}
	for _, setting := range settings {
		setting.RSClient = true
	}
	// B's export policy denies A's routes, so A's withdrawal owes keys at C
	// alone. Otherwise B's own recovery, which this test does not hold,
	// returns the buffers its writes staged while the pool is meant to be dry.
	settings[1].ExportPolicy = mustPolicy(t, []filterapi.FilterRef{{Name: "test:deny"}}, filterapi.FinalAccept)
	peers := rsLiveRouter(t, settings...)
	a, b, c := peers[0], peers[1], peers[2]
	policyDenyASNs(a.peer.reactor, b.peer.Settings().Address, "65002")
	attrsA := firstASAttrs(4, 65002)
	attrsB := firstASAttrs(4, 65003, 65003)
	prefixes := churnPrefixes(60, keys)
	ownershipLiveSend(t, b, nil, attrsB, prefixes...) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefixes[keys-1], attrsB)
	ownershipLiveSend(t, a, nil, attrsA, prefixes...) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefixes[keys-1], attrsA)
	lowEventually(t, func() bool { return adjOutOwnedBy(c.peer, a.peer) == keys }, "A owns every prefix at C")

	tap := &lookupSizes{destination: c.peer.Settings().Address, seen: make(map[string]struct{}),
		hold: make(chan struct{})}
	publishLookupTap(t, tap.after)
	ownershipLiveSend(t, a, prefixes, nil) // RFC 4271 Section 4.3.
	lowEventually(t, func() bool { return adjOutOwedCount(c.peer) == keys }, "C owes every withdrawn key")
	// The pool is drained only once C's first round is held at its lookup:
	// the round's source and RIB barriers before the lookup can take longer
	// than the 300 ms window below under -race, and every round the window
	// counts must find the pool already dry.
	lowEventually(t, func() bool {
		requests, _, _ := tap.result()
		return requests >= 1
	}, "C's recovery lookup is held")

	held := drainReadPool(t)
	for range min(spare, len(held)) {
		returnReadBuffer(held[len(held)-1])
		held = held[:len(held)-1]
	}
	close(tap.hold)
	if spare == 0 {
		time.Sleep(300 * time.Millisecond)
		// The backoff (10, 20, 40, 80, 160 ms) allows about six rounds in
		// 300 ms: an immediate retry would ask hundreds of times.
		if requests, _, _ := tap.result(); requests < 2 || requests > 8 {
			t.Errorf("C's worker asked bgp-rib %d times while the pool was dry, want 2 to 8", requests)
		}
		if got := adjOutOwedCount(c.peer); got != keys {
			t.Errorf("C decided %d keys with no buffer free, want 0", keys-got)
		}
		if got := c.peer.SessionState(); got != fsm.StateEstablished {
			t.Fatalf("C session = %s while the pool was dry, want Established", got)
		}
		for _, h := range held {
			returnReadBuffer(h)
		}
		held = nil
	}
	waitOwnedBy(t, c.peer, b.peer, keys)
	for _, h := range held {
		returnReadBuffer(h)
	}
	// A record leaves the owed set once its write is flushed, just after
	// the entry changed owner.
	lowEventually(t, func() bool { return adjOutOwedCount(c.peer) == 0 }, "C owes nothing")
	ownershipLiveEstablished(t, peers)
}

// rsLiveRouter is lowLiveRouter for the scenarios of this spec that hold
// many read buffers in flight, under an explicit read-pool byte budget far
// above anything they allocate. The read pools are process globals, and a
// block an earlier test of this binary grew stays allocated (collapse keeps a
// pool's last block): an extended-message test leaves a 64K block of 8 MiB,
// and New seeds the new reactor's shared budget with it. The auto-sized
// budget is then the floor of one 4K block and one 64K block, so this
// router's sessions would share 128 4K buffers. Lifting the budget after
// start does not hold either: the weight tracker re-sizes it on the peers'
// family and End-of-RIB events. With ze.fwd.pool.maxbytes set the reactor
// installs the budget once and never re-sizes it, so a test that pins it
// (drainReadPool) keeps what it pinned (plan/journal/registry-contamination.md).
func rsLiveRouter(t *testing.T, settings ...*PeerSettings) []*lowLivePeer {
	t.Helper()
	previous := env.Get("ze.fwd.pool.maxbytes")
	if err := env.Set("ze.fwd.pool.maxbytes", strconv.FormatInt(rsLiveReadBudget, 10)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := env.Set("ze.fwd.pool.maxbytes", previous); err != nil {
			t.Error(err)
		}
	})
	return lowLiveRouter(t, settings...)
}

// rsLiveReadBudget is the explicit read-pool byte budget of rsLiveRouter:
// 1 GiB, about 250 000 4K buffers, where the largest scenario holds a few
// thousand.
const rsLiveReadBudget = 1 << 30

// drainReadPool pins the shared byte budget at what the pools have allocated,
// so neither can grow, and takes every free 4K buffer. Cleanup returns any
// buffer the test still holds and restores the budget.
func drainReadPool(t *testing.T) []BufHandle {
	t.Helper()
	bufMuxStd.mux.mu.Lock()
	budget := bufMuxStd.mux.budget
	bufMuxStd.mux.mu.Unlock()
	if budget == nil {
		t.Fatal("the read pools carry no shared budget")
	}
	previous := budget.maxBytes.Load()
	budget.maxBytes.Store(budget.allocated.Load())
	var held []BufHandle
	for {
		h := getReadBuf(false)
		if h.Buf == nil {
			break
		}
		held = append(held, h)
	}
	t.Cleanup(func() { budget.maxBytes.Store(previous) })
	return held
}

// TestOwedKeyStormKeepsSourceAndForwardWorkerMoving proves AC-16 at the
// reactor boundary. A owns 200 prefixes at client C (B offers them too); C's
// recovery lookup is held inside bgp-rib's answer until the test releases it.
// A then withdraws all 200 in a storm of UPDATEs, on bgp-rs's cached rail and,
// separately, on the reactor fast path. While C's worker is held:
//
//   - A's read goroutine keeps reading: A's later announcement of X reaches
//     the other client D's wire;
//   - C's forward worker keeps writing: B's announcement of Y reaches C's
//     wire;
//   - no goroutine is created per withdrawal.
//
// Once released, every owed key is decided: B owns all 200 at C. The provider
// is released before the sessions are torn down, because the in-process
// lookup takes no context.
//
// VALIDATES: AC-16, the owe never blocks the source or the destination's
// forward worker.
// PREVENTS: one slow election stalling a source's reads or a client's writes.
func TestOwedKeyStormKeepsSourceAndForwardWorkerMoving(t *testing.T) {
	t.Run("cached-rail", func(t *testing.T) { stormKeepsMoving(t, false) })
	t.Run("fast-path", func(t *testing.T) { stormKeepsMoving(t, true) })
}

func stormKeepsMoving(t *testing.T, fastPath bool) {
	const keys = 200
	settings := []*PeerSettings{
		lowLiveSettings("192.0.2.1", 65000, 65002),
		lowLiveSettings("192.0.2.2", 65000, 65003),
		lowLiveSettings("192.0.2.3", 65000, 65004),
		lowLiveSettings("192.0.2.4", 65000, 65005),
	}
	for _, setting := range settings {
		setting.RSClient = true
		setting.RSFastPath = fastPath
	}
	peers := rsLiveRouter(t, settings...)
	a, b, c, d := peers[0], peers[1], peers[2], peers[3]
	attrsA := firstASAttrs(4, 65002)
	attrsB := firstASAttrs(4, 65003, 65003)
	prefixes := churnPrefixes(70, keys)
	ownershipLiveSend(t, b, nil, attrsB, prefixes...) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefixes[keys-1], attrsB)
	ownershipLiveSend(t, a, nil, attrsA, prefixes...) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefixes[keys-1], attrsA)
	lowEventually(t, func() bool { return adjOutOwnedBy(c.peer, a.peer) == keys }, "A owns every prefix at C")

	tap := &lookupSizes{destination: c.peer.Settings().Address, seen: make(map[string]struct{}),
		hold: make(chan struct{})}
	publishLookupTap(t, tap.after)
	released := false
	release := func() {
		if !released {
			released = true
			close(tap.hold)
		}
	}
	t.Cleanup(release)

	// The storm: one UPDATE per ten withdrawals.
	for start := 0; start < keys; start += 10 {
		ownershipLiveSend(t, a, prefixes[start:start+10], nil) // RFC 4271 Section 4.3.
	}
	lowEventually(t, func() bool {
		requests, _, _ := tap.result()
		return requests >= 1
	}, "C's recovery lookup is held")
	baseline := runtime.NumGoroutine()
	for start := 0; start < keys; start += 10 {
		// A second storm over the same keys merges into the records.
		ownershipLiveSend(t, a, prefixes[start:start+10], nil) // RFC 4271 Section 4.3.
	}

	x := netip.MustParsePrefix("10.9.200.0/24")
	// D and C are each sent hundreds of UPDATEs, more than their wire
	// captures keep, so arrival is read from their Adj-RIB-Out tables.
	ownershipLiveSend(t, a, nil, attrsA, x) // RFC 4271 Section 4.3.
	lowEventually(t, func() bool { return adjOutPrefixOwner(t, d.peer, x) == a.peer }, "A's X at D")
	y := netip.MustParsePrefix("10.9.201.0/24")
	ownershipLiveSend(t, b, nil, attrsB, y) // RFC 4271 Section 4.3.
	lowEventually(t, func() bool { return adjOutPrefixOwner(t, c.peer, y) == b.peer }, "B's Y at C")
	if got := adjOutOwnedBy(c.peer, b.peer); got != 1 {
		t.Errorf("B owns %d entries at C while C's lookup is held, want 1 (Y only)", got)
	}
	// A goroutine per withdrawal would add 200 here; the bound leaves room
	// for the transient workers of the two forwards above.
	if grown := runtime.NumGoroutine() - baseline; grown > 20 {
		t.Errorf("goroutines grew by %d during a storm of %d withdrawals", grown, keys)
	}

	release()
	waitOwnedBy(t, c.peer, b.peer, keys+1)
	lowEventually(t, func() bool { return adjOutOwedCount(c.peer) == 0 }, "C owes nothing")
	ownershipLiveEstablished(t, peers)
}

// TestOwedKeyBufferStallsCountOnlyConsecutive proves the stall count, and
// with it the backoff and the retire bound, covers only consecutive rounds
// that decided nothing. Six times over, A withdraws 100 prefixes C holds from
// A while C's lookups are held, the test holds every free read buffer,
// releases the lookups for 300 ms of stalled rounds, then returns the buffers
// so the round decides B, and A re-announces. Each phase's backoff restarts
// at 10 ms, so each phase asks about five times; a count kept across phases
// would wait a second per round by the third phase and, over a long session,
// reach recoveryStallMax and reset a healthy client.
//
// VALIDATES: a decided round resets the stall count.
// PREVENTS: stalls accumulating over a session's life until a healthy client
// is reset.
func TestOwedKeyBufferStallsCountOnlyConsecutive(t *testing.T) {
	const keys = 100
	settings := []*PeerSettings{
		lowLiveSettings("192.0.2.1", 65000, 65002),
		lowLiveSettings("192.0.2.2", 65000, 65003),
		lowLiveSettings("192.0.2.3", 65000, 65004),
	}
	for _, setting := range settings {
		setting.RSClient = true
	}
	// B's export policy denies A's routes, so each phase's withdrawal owes
	// keys at C alone: B's own recovery would return the buffers its writes
	// staged while the pool is meant to be dry.
	settings[1].ExportPolicy = mustPolicy(t, []filterapi.FilterRef{{Name: "test:deny"}}, filterapi.FinalAccept)
	peers := rsLiveRouter(t, settings...)
	a, b, c := peers[0], peers[1], peers[2]
	policyDenyASNs(a.peer.reactor, b.peer.Settings().Address, "65002")
	attrsA := firstASAttrs(4, 65002)
	attrsB := firstASAttrs(4, 65003, 65003)
	prefixes := churnPrefixes(80, keys)
	ownershipLiveSend(t, b, nil, attrsB, prefixes...) // RFC 4271 Section 4.3.
	ownershipLiveThrough(t, c, prefixes[keys-1], attrsB)
	gate := &lookupGate{destination: c.peer.Settings().Address}
	publishLookupTap(t, gate.after)
	t.Cleanup(gate.open)

	for phase := range 6 {
		ownershipLiveSend(t, a, nil, attrsA, prefixes...) // RFC 4271 Section 4.3.
		waitOwnedBy(t, c.peer, a.peer, keys)
		gate.close()
		parked := gate.count()
		ownershipLiveSend(t, a, prefixes, nil) // RFC 4271 Section 4.3.
		lowEventually(t, func() bool { return adjOutOwedCount(c.peer) == keys }, "C owes every withdrawn key")
		// The pool is drained only once the phase's first round is parked at
		// its lookup, so the window below counts rounds that all found the
		// pool dry rather than the barriers before a slow first lookup.
		lowEventually(t, func() bool { return gate.count() > parked }, "C's recovery lookup is parked")
		held := drainReadPool(t)
		before := gate.count()
		gate.open()
		time.Sleep(300 * time.Millisecond)
		// A backoff that restarts at 10 ms asks about five times in 300 ms;
		// one that kept growing across phases would ask once a second.
		if dry := gate.count() - before; dry < 3 {
			t.Errorf("phase %d: %d lookups while the pool was dry, want at least 3 (the backoff did not restart)", phase, dry)
		}
		if got := c.peer.SessionState(); got != fsm.StateEstablished {
			t.Fatalf("phase %d: C session = %s while the pool was dry, want Established", phase, got)
		}
		for _, h := range held {
			returnReadBuffer(h)
		}
		waitOwnedBy(t, c.peer, b.peer, keys)
		lowEventually(t, func() bool { return adjOutOwedCount(c.peer) == 0 }, "C owes nothing")
	}
	ownershipLiveEstablished(t, peers)
}

// lookupGate parks every recovery lookup for one destination while closed.
// It is a publishLookupTap hook, so its state is locked; a parked lookup
// waits at most 10 seconds.
type lookupGate struct {
	destination netip.Addr

	mu      sync.Mutex
	gate    chan struct{}
	lookups int
}

func (g *lookupGate) close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.gate == nil {
		g.gate = make(chan struct{})
	}
}

func (g *lookupGate) count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.lookups
}

func (g *lookupGate) open() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.gate != nil {
		close(g.gate)
		g.gate = nil
	}
}

func (g *lookupGate) after(request *ribevents.RecoveryRequest, outcomes []ribevents.RecoveryOutcome, err error) ([]ribevents.RecoveryOutcome, error) {
	if request.Destination != g.destination {
		return outcomes, err
	}
	g.mu.Lock()
	gate := g.gate
	g.lookups++
	g.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-time.After(10 * time.Second):
		}
	}
	return outcomes, err
}

// TestOwedKeyPartialRoundSkipsBackoff proves the conversion at the end of
// recoveryWorker.round: a round whose later chunk finds the read pool dry
// after an earlier chunk was decided is partial, not a stall, so the stall
// count resets and the next round starts at once. A owns 1000 prefixes at C,
// two full lookup chunks of 500; B's export policy denies A's routes, so C
// alone owes. The test steps every lookup of C's worker and holds every free
// read buffer, so the first six rounds stall on their first chunk and the
// backoff reaches 640 ms. A chunk is sized from the buffers the pool can lend
// (recoveryChunkKeys), so every round after the first asks one key over the
// dry pool. The seventh round's one-key first chunk gets the buffers back and
// is decided; its second chunk is sized over them, and before its admission
// the test holds the pool dry again. The next lookup MUST follow within
// 300 ms: a round counted as the seventh stall would wait 640 ms first.
//
// VALIDATES: a round that decided a chunk resets the stall count.
// PREVENTS: a destination making progress backing off, and reset at
// recoveryStallMax, as if it had decided nothing.
func TestOwedKeyPartialRoundSkipsBackoff(t *testing.T) {
	const keys = 2 * recoveryLookupKeysMax
	settings := []*PeerSettings{
		lowLiveSettings("192.0.2.1", 65000, 65002),
		lowLiveSettings("192.0.2.2", 65000, 65003),
		lowLiveSettings("192.0.2.3", 65000, 65004),
	}
	for _, setting := range settings {
		setting.RSClient = true
	}
	// B's export policy denies A's routes, so A's withdrawal owes keys at C
	// alone (bufferShortageDefersKeys).
	settings[1].ExportPolicy = mustPolicy(t, []filterapi.FilterRef{{Name: "test:deny"}}, filterapi.FinalAccept)
	peers := rsLiveRouter(t, settings...)
	a, b, c := peers[0], peers[1], peers[2]
	policyDenyASNs(a.peer.reactor, b.peer.Settings().Address, "65002")
	attrsA := firstASAttrs(4, 65002)
	attrsB := firstASAttrs(4, 65003, 65003)
	prefixes := churnPrefixes(90, keys)
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
	waitOwnedBy(t, c.peer, a.peer, keys)

	// The pool is grown first, so the buffers returned to the seventh round
	// decide its whole first chunk.
	growReadPool(t, 4*recoveryLookupKeysMax)
	step := &lookupStepper{destination: c.peer.Settings().Address,
		arrived: make(chan int), proceed: make(chan struct{}), free: make(chan struct{})}
	publishLookupTap(t, step.after)
	t.Cleanup(step.release)

	for start := 0; start < keys; start += perUpdate {
		end := min(start+perUpdate, keys)
		ownershipLiveSend(t, a, prefixes[start:end], nil) // RFC 4271 Section 4.3.
	}
	lowEventually(t, func() bool { return adjOutOwedCount(c.peer) == keys }, "C owes every withdrawn key")

	// Six rounds stall on their first chunk; their waits are 10 to 320 ms.
	// The pool is held dry only once the first is parked at its lookup:
	// every session's reads share it, and A's withdrawal must be read first.
	// A chunk is sized from what the pool can lend when the round reaches
	// it (recoveryChunkKeys), so the first round asks a full chunk and every
	// later one, sized over the dry pool, asks the one key it always asks.
	var held []BufHandle
	for round := range 6 {
		want := 1
		if round == 0 {
			want = recoveryLookupKeysMax
		}
		if got := step.next(t); got != want {
			t.Fatalf("stalled round %d asked for %d keys, want %d", round+1, got, want)
		}
		if round == 0 {
			held = drainReadPool(t)
		}
		step.proceed <- struct{}{}
	}
	// The seventh round's first chunk, sized over the dry pool at one key,
	// is decided with the buffers back.
	if got := step.next(t); got != 1 {
		t.Fatalf("the seventh round's first chunk asked for %d keys, want 1", got)
	}
	if got := adjOutOwedCount(c.peer); got != keys {
		t.Fatalf("C decided %d keys while the pool was dry, want 0", keys-got)
	}
	for _, h := range held {
		returnReadBuffer(h)
	}
	step.proceed <- struct{}{}
	// Its second chunk, sized over the returned buffers, finds the pool dry
	// again.
	step.next(t)
	if got := adjOutOwedCount(c.peer); got != keys-1 {
		t.Fatalf("C owes %d keys after the seventh round's first chunk, want %d", got, keys-1)
	}
	held = holdReadPoolDry(t)
	asked := time.Now()
	step.proceed <- struct{}{}
	step.next(t)
	gap := time.Since(asked)
	t.Logf("C's next round started %s after the partial round", gap)
	if gap > 300*time.Millisecond {
		t.Errorf("C's next round started %s after the partial round, want under 300ms (the stall count did not reset)", gap)
	}
	for _, h := range held {
		returnReadBuffer(h)
	}
	step.release()
	waitOwnedBy(t, c.peer, b.peer, keys)
	lowEventually(t, func() bool { return adjOutOwedCount(c.peer) == 0 }, "C owes nothing")
	ownershipLiveEstablished(t, peers)
}

// growReadPool takes count 4K read buffers and returns them, so the pool
// holds at least that many before a test pins its budget (drainReadPool).
func growReadPool(t *testing.T, count int) {
	t.Helper()
	taken := make([]BufHandle, 0, count)
	for range count {
		h := getReadBuf(false)
		if h.Buf == nil {
			break
		}
		taken = append(taken, h)
	}
	for _, h := range taken {
		returnReadBuffer(h)
	}
	if len(taken) < count {
		t.Fatalf("the read pool gave %d of %d buffers", len(taken), count)
	}
}

// holdReadPoolDry takes every free 4K read buffer under the budget
// drainReadPool pinned, and keeps taking until 50 ms pass with none
// returned: a write the last admission staged returns its buffer when it
// completes. The caller returns what it holds. It gives up after 10 seconds.
func holdReadPoolDry(t *testing.T) []BufHandle {
	t.Helper()
	var held []BufHandle
	deadline := time.Now().Add(10 * time.Second)
	for {
		if time.Now().After(deadline) {
			for _, h := range held {
				returnReadBuffer(h)
			}
			t.Fatalf("the read pool never settled dry: %d buffers taken", len(held))
		}
		taken := 0
		for {
			h := getReadBuf(false)
			if h.Buf == nil {
				break
			}
			held = append(held, h)
			taken++
		}
		if taken == 0 && len(held) > 0 {
			return held
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// lookupStepper parks every recovery lookup for one destination until the
// test lets it proceed, and reports each lookup's key count on arrived. Once
// released it parks nothing. It is a publishLookupTap hook; a parked lookup
// waits at most 10 seconds.
type lookupStepper struct {
	destination netip.Addr
	arrived     chan int
	proceed     chan struct{}
	free        chan struct{}
	once        sync.Once
}

// next waits up to 10 seconds for the next lookup and returns its key count.
func (s *lookupStepper) next(t *testing.T) int {
	t.Helper()
	select {
	case n := <-s.arrived:
		return n
	case <-time.After(10 * time.Second):
		t.Fatal("C's next recovery lookup never ran")
		return 0
	}
}

// release lets every parked and later lookup through. Safe to call twice.
func (s *lookupStepper) release() {
	s.once.Do(func() { close(s.free) })
}

func (s *lookupStepper) after(request *ribevents.RecoveryRequest, outcomes []ribevents.RecoveryOutcome, err error) ([]ribevents.RecoveryOutcome, error) {
	if request.Destination != s.destination {
		return outcomes, err
	}
	select {
	case s.arrived <- len(request.Keys):
	case <-s.free:
		return outcomes, err
	}
	select {
	case <-s.proceed:
	case <-s.free:
	case <-time.After(10 * time.Second):
	}
	return outcomes, err
}

// convergenceLoss is how one source stops offering the prefix.
type convergenceLoss uint8

const (
	convergenceLossUnspecified convergenceLoss = iota
	// convergenceLossWithdraw: the source withdraws the prefix.
	convergenceLossWithdraw
	// convergenceLossDown: the source's session ends with no NOTIFICATION.
	convergenceLossDown
)

// TestOwedKeyRealRIBConvergence proves AC-12 against the real bgp-rib. Four
// sources S1 to S4 announce 10.9.0.0/24 with AS_PATHs of one to four ASNs,
// worst first, so S1 is the RIB's best and the newest: it owns client C's
// route. Then K of them stop offering it, by withdrawal or by session down,
// in every order of every K-subset, mixed both ways, issued back-to-back
// (the next loss waits until C holds the best survivor and owes nothing,
// which a loss of a non-owner already satisfies before it is processed) and
// concurrently (issued together):
//
//   - K = 2: C ends on the best remaining source's route, and is sent no
//     withdrawal of the prefix;
//   - K = 4: C holds no route for the prefix, and is sent exactly one
//     withdrawal.
//
// VALIDATES: AC-12, C converges on the RIB's best survivor whatever the order
// and the timing of the losses, with no bound on how many are in flight.
// PREVENTS: a client withdrawn, or left on a gone source's route, when two of
// several sources leave.
func TestOwedKeyRealRIBConvergence(t *testing.T) {
	const sources = 4
	for _, losers := range convergenceOrders(sources, 2) {
		for _, kinds := range [][2]convergenceLoss{
			{convergenceLossWithdraw, convergenceLossDown},
			{convergenceLossDown, convergenceLossWithdraw},
		} {
			for _, concurrent := range []bool{false, true} {
				name := convergenceCaseName(losers, kinds[:], concurrent)
				t.Run(name, func(t *testing.T) {
					convergeOnSurvivor(t, sources, losers, func(i int) convergenceLoss { return kinds[i%2] }, concurrent)
				})
			}
		}
	}
	for _, losers := range convergenceOrders(sources, sources) {
		kinds := func(i int) convergenceLoss {
			if (i+losers[0])%2 == 0 {
				return convergenceLossWithdraw
			}
			return convergenceLossDown
		}
		name := convergenceCaseName(losers, []convergenceLoss{kinds(0), kinds(1)}, true)
		t.Run("all/"+name, func(t *testing.T) { convergeOnSurvivor(t, sources, losers, kinds, true) })
	}
}

// convergenceOrders returns every ordered selection of k distinct indexes
// out of n. Bounded: n is at most 4 here, so at most 24 selections.
func convergenceOrders(n, k int) [][]int {
	var out [][]int
	var walk func(prefix []int)
	walk = func(prefix []int) {
		if len(prefix) == k {
			out = append(out, slices.Clone(prefix))
			return
		}
		for i := range n {
			if slices.Contains(prefix, i) {
				continue
			}
			walk(append(prefix, i))
		}
	}
	walk(nil)
	return out
}

func convergenceCaseName(losers []int, kinds []convergenceLoss, concurrent bool) string {
	name := ""
	for i, loser := range losers {
		if i > 0 {
			name += "-"
		}
		name += "S" + strconv.Itoa(loser+1)
		if kinds[i%len(kinds)] == convergenceLossDown {
			name += "down"
		} else {
			name += "wd"
		}
	}
	if concurrent {
		return name + "/concurrent"
	}
	return name + "/back-to-back"
}

// convergeOnSurvivor runs one AC-12 case: sources S1..Sn (S1 the best), the
// losers in order with kind(i) for the i-th loss, then judges C. A marker
// source M, which never offers the prefix, carries the causal marker.
func convergeOnSurvivor(t *testing.T, sources int, losers []int, kind func(int) convergenceLoss, concurrent bool) {
	settings := make([]*PeerSettings, 0, sources+2)
	for i := range sources {
		settings = append(settings, lowLiveSettings("192.0.2."+strconv.Itoa(10+i), 65000, uint32(65001+i)))
	}
	settings = append(settings, lowLiveSettings("192.0.2.3", 65000, 65100), lowLiveSettings("192.0.2.6", 65000, 65101))
	for _, setting := range settings {
		setting.RSClient = true
	}
	peers := rsLiveRouter(t, settings...)
	c, marker := peers[sources], peers[sources+1]
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	attrs := make([][]byte, sources)
	for i := range sources {
		path := slices.Repeat([]uint32{uint32(65001 + i)}, i+1)
		attrs[i] = firstASAttrs(4, path...)
	}
	// RFC 4271 Section 9.1.2.2, step a: S1's AS_PATH is the shortest. Worst
	// first, each written at C before the next, so S1, the newest, owns it.
	for i := sources - 1; i >= 0; i-- {
		ownershipLiveSend(t, peers[i], nil, attrs[i], prefix) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, c, prefix, attrs[i])
	}
	before := ownershipLiveRead(t, c).withdrawals[prefix]

	lost := make([]bool, sources)
	for i, loser := range losers {
		switch kind(i) {
		case convergenceLossWithdraw:
			ownershipLiveSend(t, peers[loser], []netip.Prefix{prefix}, nil) // RFC 4271 Section 4.3.
		case convergenceLossDown:
			_ = peers[loser].remote.Close() //nolint:errcheck // A departure: the session ends with no NOTIFICATION.
		case convergenceLossUnspecified:
			t.Fatal("BUG: a loss with no kind")
		}
		lost[loser] = true
		if concurrent {
			continue
		}
		best := slices.Index(lost, false)
		lowEventually(t, func() bool { return convergenceSettled(t, c, prefix, attrs, best) }, "C decided the loss")
	}
	best := slices.Index(lost, false)
	deadline := time.Now().Add(30 * time.Second)
	for !convergenceSettled(t, c, prefix, attrs, best) && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	// Let any late write for the prefix land before the history is judged.
	later := netip.MustParsePrefix("10.9.254.0/24")
	attrsM := firstASAttrs(4, 65101)
	ownershipLiveSend(t, marker, nil, attrsM, later) // RFC 4271 Section 4.3.
	view := ownershipLiveThrough(t, c, later, attrsM)
	if best < 0 {
		ownershipLiveAbsent(t, view, prefix)
		ownershipLiveWithdrawals(t, view, prefix, before+1)
	} else {
		ownershipLiveRoute(t, view, prefix, attrs[best])
		ownershipLiveWithdrawals(t, view, prefix, before)
	}
	if got := adjOutOwedCount(c.peer); got != 0 {
		t.Errorf("C still owes %d keys", got)
	}
	remaining := []*lowLivePeer{c, marker}
	for i := range sources {
		if !lost[i] || kind(slices.Index(losers, i)) == convergenceLossWithdraw {
			remaining = append(remaining, peers[i])
		}
	}
	ownershipLiveEstablished(t, remaining)
}

// convergenceSettled reports whether C holds the best survivor's route for
// prefix and owes nothing, or holds no route when best is negative.
func convergenceSettled(t *testing.T, c *lowLivePeer, prefix netip.Prefix, attrs [][]byte, best int) bool {
	if adjOutOwedCount(c.peer) != 0 {
		return false
	}
	route, held := ownershipLiveRead(t, c).routes[prefix]
	if best < 0 {
		return !held
	}
	return bytes.Equal(route, attrs[best])
}

// TestOwedKeyRealRIBConvergenceProperty is AC-12's property form over the
// real bgp-rib: twelve seeded cases, each with N sources in [3, 12], K losers
// in [1, N] in a random order, each loss a withdrawal or a session down at
// random, issued back-to-back or concurrently at random. C ends on the best
// survivor's route with no withdrawal, or, when K = N, holds nothing after
// exactly one. The seed and each case (its N, losers, kinds and timing, in
// the subtest name) are logged, so a failing case can be rebuilt.
// Re-announcements by a lost source are Phase 3a's (Q-3): before it, the
// newest write owns C's entry, not the RIB's best.
//
// VALIDATES: AC-12 with no fixed N, K or order.
// PREVENTS: a convergence that holds only for the hand-picked cases.
func TestOwedKeyRealRIBConvergenceProperty(t *testing.T) {
	seed := time.Now().UnixNano()
	t.Logf("seed %d", seed)
	random := rand.New(rand.NewPCG(uint64(seed), 0)) //nolint:gosec // A test schedule, not a secret.
	for run := range 12 {
		sources := 3 + random.IntN(10)
		losers := random.Perm(sources)[:1+random.IntN(sources)]
		kinds := make([]convergenceLoss, len(losers))
		for i := range kinds {
			kinds[i] = convergenceLossWithdraw + convergenceLoss(random.IntN(2))
		}
		concurrent := random.IntN(2) == 1
		name := "run" + strconv.Itoa(run) + "/N" + strconv.Itoa(sources) + "/" + convergenceCaseName(losers, kinds, concurrent)
		t.Run(name, func(t *testing.T) {
			convergeOnSurvivor(t, sources, losers, func(i int) convergenceLoss { return kinds[i] }, concurrent)
		})
	}
}

// grPurge is how a retained source's routes leave bgp-rib in
// TestOwedKeyGRPurgeRecoversSurvivor.
type grPurge uint8

const (
	grPurgeUnspecified grPurge = iota
	// grPurgeEndOfRIB: "request bgp rib purge-stale", as the source's
	// End-of-RIB after a reconnect sends it.
	grPurgeEndOfRIB
	// grPurgeRelease: "request bgp rib release-routes", as bgp-gr sends it.
	grPurgeRelease
	// grPurgeExpiry: the restart time given to mark-stale runs out.
	grPurgeExpiry
)

// TestOwedKeyGRPurgeRecoversSurvivor proves AC-19 for bgp-rs against the real
// bgp-rib, the lifecycle half. A owns 10.9.0.0/24 at C and B announces it
// too. A's routes are retained (retain-routes, as bgp-gr asks before the
// session drops), A's session ends with no NOTIFICATION, and its routes are
// marked GR stale: C keeps A's route. Then the retained route is purged by
// purge-stale, by release-routes, or by the restart time running out. C
// receives B's route with B's received attributes and no withdrawal; with no
// B, C receives the withdrawal exactly once. The negotiated-capability half
// (bgp-gr driving these commands) is the functional test
// bgp-rs-withdrawal-replacement-gr.
//
// VALIDATES: AC-19, a GR purge of the owner's retained route is decided like
// an owner withdrawal.
// PREVENTS: a client sent a bare withdrawal when a retained route expires
// while a survivor exists.
func TestOwedKeyGRPurgeRecoversSurvivor(t *testing.T) {
	for _, purge := range []struct {
		name string
		how  grPurge
	}{{"end-of-rib", grPurgeEndOfRIB}, {"release", grPurgeRelease}, {"expiry", grPurgeExpiry}} {
		t.Run(purge.name+"/survivor", func(t *testing.T) { grPurgeRecovers(t, purge.how, true) })
		t.Run(purge.name+"/no-survivor", func(t *testing.T) { grPurgeRecovers(t, purge.how, false) })
	}
}

func grPurgeRecovers(t *testing.T, how grPurge, withB bool) {
	settings := []*PeerSettings{
		lowLiveSettings("192.0.2.1", 65000, 65002),
		lowLiveSettings("192.0.2.2", 65000, 65003),
		lowLiveSettings("192.0.2.3", 65000, 65004),
	}
	for _, setting := range settings {
		setting.RSClient = true
	}
	peers := rsLiveRouter(t, settings...)
	a, b, c := peers[0], peers[1], peers[2]
	r := a.peer.reactor
	prefix := netip.MustParsePrefix("10.9.0.0/24")
	attrsA := firstASAttrs(4, 65002)
	attrsB := firstASAttrs(4, 65003, 65003)
	if withB {
		ownershipLiveSend(t, b, nil, attrsB, prefix) // RFC 4271 Section 4.3.
		ownershipLiveThrough(t, c, prefix, attrsB)
	}
	ownershipLiveSend(t, a, nil, attrsA, prefix) // RFC 4271 Section 4.3.
	before := ownershipLiveThrough(t, c, prefix, attrsA).withdrawals[prefix]

	source := a.peer.Settings().Address.String()
	// RFC 4724 Section 4.2: "it MUST retain the routes received from the
	// peer for all the address families that were previously received in
	// the Graceful Restart Capability and MUST mark them as stale routing
	// information."
	purgeRIBCommand(t, r, "request bgp rib retain-routes", source, "ipv4/unicast")
	// bgp-gr publishes which families it retains; bgp-rs asks it at
	// departure and leaves a retained family to bgp-rib's purge.
	retained := retention.Publish(func(peer string, fam family.Family) bool {
		return peer == source && fam == family.IPv4Unicast
	})
	t.Cleanup(retained.Close)
	_ = a.remote.Close() //nolint:errcheck // A departure: the session ends with no NOTIFICATION.
	lowEventually(t, func() bool { return a.peer.SessionState() != fsm.StateEstablished }, "A's session ended")
	restart := "0"
	if how == grPurgeExpiry {
		restart = "1"
	}
	// RFC 4724 Section 4.2, the marking half of the sentence above. Level 1
	// is GR stale.
	purgeRIBCommand(t, r, "request bgp rib mark-stale", source, restart, "1", "ipv4/unicast")
	// The retained route stays at C until it is purged.
	held := netip.MustParsePrefix("10.9.253.0/24")
	ownershipLiveSend(t, b, nil, attrsB, held) // RFC 4271 Section 4.3.
	view := ownershipLiveThrough(t, c, held, attrsB)
	ownershipLiveRoute(t, view, prefix, attrsA)

	switch how {
	case grPurgeEndOfRIB:
		purged, err := purgeRIBStale(r, a.peer.Settings().Address)
		if err != nil {
			t.Fatal(err)
		}
		if purged != 1 {
			t.Fatalf("purge-stale purged %d of A's routes, want 1", purged)
		}
	case grPurgeRelease:
		purgeRIBCommand(t, r, "request bgp rib release-routes", source)
	case grPurgeExpiry:
		// The restart time of one second runs out inside bgp-rib.
	case grPurgeUnspecified:
		t.Fatal("BUG: a purge with no kind")
	}
	// bgp-rib's expiry fires after the restart time plus its 5 s margin
	// (grTimerMargin), so the wait allows 20 s.
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		route, present := ownershipLiveRead(t, c).routes[prefix]
		if withB && bytes.Equal(route, attrsB) {
			break
		}
		if !withB && !present {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	lowEventually(t, func() bool { return adjOutOwedCount(c.peer) == 0 }, "C owes nothing")
	later := netip.MustParsePrefix("10.9.254.0/24")
	ownershipLiveSend(t, b, nil, attrsB, later) // RFC 4271 Section 4.3.
	view = ownershipLiveThrough(t, c, later, attrsB)
	// RFC 4271 Section 9.2: "All newly installed routes and all newly
	// unfeasible routes for which there is no replacement route SHALL be
	// advertised to its peers by means of an UPDATE message."
	if withB {
		ownershipLiveRoute(t, view, prefix, attrsB)
		ownershipLiveWithdrawals(t, view, prefix, before)
	} else {
		ownershipLiveAbsent(t, view, prefix)
		ownershipLiveWithdrawals(t, view, prefix, before+1)
	}
	ownershipLiveEstablished(t, []*lowLivePeer{b, c})
}
