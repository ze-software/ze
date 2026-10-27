package reactor

import (
	"log/slog"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/source"
)

const pathsLimitElectPrefix = "203.0.113.0/24"

// sendBound is the send admission under a peer PATHS-LIMIT of 2 for IPv4
// unicast, as initPathsLimit builds it from a negotiated ADD-PATH send.
func sendBound(t *testing.T) pathsLimitAdmission {
	t.Helper()
	admission := newPathsLimitAdmission(
		map[family.Family]uint16{family.IPv4Unicast: 2},
		map[family.Family]capability.AddPathMode{family.IPv4Unicast: capability.AddPathSend},
		capability.AddPathSend)
	if admission == nil {
		t.Fatal("a negotiated send direction with a peer limit must build a bound")
	}
	return admission
}

// sendThrough runs one body through the bound and keeps its journal; the
// caller MUST release the returned journal.
func sendThrough(t *testing.T, admission pathsLimitAdmission, body []byte) (*pathsLimitChanges, []uint32) {
	t.Helper()
	changes, _ := pathsLimitScratch.Get().(*pathsLimitChanges)
	dst := make([]byte, message.ExtMsgLen)
	n, _, err := admission.filter(dst, body, changes)
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	if n == 0 {
		return changes, nil
	}
	announced, _ := recvBodyPathIDs(t, dst[:n])
	return changes, announced
}

func electPrefix(t *testing.T, admission pathsLimitAdmission, prefix string) *pathsLimitPrefix {
	t.Helper()
	state := admission[family.IPv4Unicast]
	key, err := state.key(pathsLimitNLRI(0, prefix)[4:], state.keyScratch[:], false)
	if err != nil {
		t.Fatal(err)
	}
	return state.prefixes[string(key)]
}

func electPeer(addr string) *Peer {
	return NewPeer(NewPeerSettings(netip.MustParseAddr(addr), 65001, 65002, 0x01020304))
}

// heldFrom is a rankable held path: Ze sent id for source's path received.
func heldFrom(id uint32, source *Peer, received uint32) pathsLimitHeldPath {
	return pathsLimitHeldPath{id: id, owner: adjOutPath{source: source, received: received, addPath: true}, rankable: true}
}

func survivorOf(source *Peer, received uint32) ribevents.RecoveryRoute {
	return ribevents.RecoveryRoute{Source: source.Settings().Address, PathID: received, AddPath: true}
}

// TestPathsLimitNoBoundNoElection proves AC-7: with no peer limit there is no
// send state, nothing is journaled, nothing is owed, and the recovery round
// asks the RIB no limited question. Method: build the admission from an empty
// limit set and push three paths for one route through it.
//
// VALIDATES: AC-7.
func TestPathsLimitNoBoundNoElection(t *testing.T) {
	admission := newPathsLimitAdmission(nil,
		map[family.Family]capability.AddPathMode{family.IPv4Unicast: capability.AddPathSend},
		capability.AddPathSend)
	if admission != nil {
		t.Fatal("no peer limit must build no send state")
	}
	changes, announced := sendThrough(t, admission, recvAnnounce(
		pathsLimitNLRI(1, pathsLimitElectPrefix), pathsLimitNLRI(2, pathsLimitElectPrefix), pathsLimitNLRI(3, pathsLimitElectPrefix)))
	defer changes.release()
	if !equalIDs(announced, 1, 2, 3) {
		t.Fatalf("sent %v, want every path", announced)
	}
	if changes.elections != 0 {
		t.Fatalf("owed %d elections with no bound", changes.elections)
	}
	session := NewSession(NewPeerSettings(netip.MustParseAddr("192.0.2.20"), 65001, 65002, 0x01020304))
	if bound, limited := session.pathsLimitBound(family.IPv4Unicast); limited {
		t.Fatalf("an unlimited session answered limited with bound %d, want no limited request", bound)
	}
}

// TestPathsLimitUncontestedPrefixNoElection proves AC-6 and D-3: at or under
// the bound nothing is contested, nothing is journaled for an election, and
// the route keeps no copy of its NLRI. Method: send exactly two paths.
//
// VALIDATES: AC-6, D-3.
func TestPathsLimitUncontestedPrefixNoElection(t *testing.T) {
	admission := sendBound(t)
	changes, announced := sendThrough(t, admission, recvAnnounce(
		pathsLimitNLRI(1, pathsLimitElectPrefix), pathsLimitNLRI(2, pathsLimitElectPrefix)))
	defer changes.release()
	if !equalIDs(announced, 1, 2) {
		t.Fatalf("sent %v, want [1 2]", announced)
	}
	if changes.elections != 0 {
		t.Fatalf("an uncontested route owed %d elections", changes.elections)
	}
	prefix := electPrefix(t, admission, pathsLimitElectPrefix)
	if prefix.contested {
		t.Fatal("a route at its bound must not be contested")
	}
	if prefix.route != nil {
		t.Fatal("an uncontested route must not copy its NLRI")
	}
}

// TestPathsLimitElectionAdmitsBestOrder proves AC-17-peer: paths that arrive
// worst-first fill the bound, the third arrival contests the route, and the
// election replaces the two arrivals with the RIB's top two. Method: drive
// the send admission, then plan against the RIB's ranked answer.
//
// VALIDATES: AC-17-peer, AC-6.
func TestPathsLimitElectionAdmitsBestOrder(t *testing.T) {
	admission := sendBound(t)
	changes, announced := sendThrough(t, admission, recvAnnounce(
		pathsLimitNLRI(1, pathsLimitElectPrefix), pathsLimitNLRI(2, pathsLimitElectPrefix), pathsLimitNLRI(3, pathsLimitElectPrefix)))
	defer changes.release()
	if !equalIDs(announced, 1, 2) {
		t.Fatalf("the bound sent %v, want the first two arrivals [1 2]", announced)
	}
	if changes.elections != 1 {
		t.Fatalf("the refusal owed %d elections, want 1", changes.elections)
	}
	prefix := electPrefix(t, admission, pathsLimitElectPrefix)
	if !prefix.contested {
		t.Fatal("a refusal at the bound must contest the route")
	}
	if !slices.Equal(prefix.route, pathsLimitNLRI(0, pathsLimitElectPrefix)[4:]) {
		t.Fatalf("contested route %x, want the NLRI without its Path Identifier", prefix.route)
	}

	worst, worse, better, best := electPeer("192.0.2.1"), electPeer("192.0.2.2"), electPeer("192.0.2.3"), electPeer("192.0.2.4")
	held := []pathsLimitHeldPath{heldFrom(1, worst, 0), heldFrom(2, worse, 0)}
	plan := planPathsLimitElection(held, []ribevents.RecoveryRoute{survivorOf(best, 0), survivorOf(better, 0)}, 2)
	if !slices.Equal(plan.announce, []int{0, 1}) {
		t.Fatalf("announce %v, want the RIB's top two [0 1]", plan.announce)
	}
	if got := plan.withdrawals(len(plan.announce)); !slices.Equal(got, []uint32{1, 2}) {
		t.Fatalf("withdraw %v, want both arrivals [1 2]", got)
	}
}

// TestPathsLimitElectionWithdrawsDemotedPath proves AC-18-peer and D-5: a
// better path demotes the held second-best, whose withdrawal is staged ahead
// of the announcement, and only that order lets the bound admit the new path.
// Method: plan, reorder staged items, then replay both orders through a bound
// that holds the two old paths.
//
// VALIDATES: AC-18-peer, D-5.
func TestPathsLimitElectionWithdrawsDemotedPath(t *testing.T) {
	first, second, newcomer := electPeer("192.0.2.1"), electPeer("192.0.2.2"), electPeer("192.0.2.3")
	held := []pathsLimitHeldPath{heldFrom(1, first, 0), heldFrom(2, second, 0)}
	plan := planPathsLimitElection(held, []ribevents.RecoveryRoute{survivorOf(newcomer, 0), survivorOf(first, 0)}, 2)
	if !slices.Equal(plan.announce, []int{0}) {
		t.Fatalf("announce %v, want the newcomer [0]", plan.announce)
	}
	if got := plan.withdrawals(1); !slices.Equal(got, []uint32{2}) {
		t.Fatalf("withdraw %v, want the demoted second-best [2]", got)
	}

	items := []fwdItem{{}, {pathsLimitDemotion: true}}
	pathsLimitWithdrawFirst(items, 1)
	if !items[0].pathsLimitDemotion {
		t.Fatal("the demotion withdrawal must be written before the announcement")
	}

	admission := sendBound(t)
	held2, _ := sendThrough(t, admission, recvAnnounce(pathsLimitNLRI(1, pathsLimitElectPrefix), pathsLimitNLRI(2, pathsLimitElectPrefix)))
	held2.release()
	wrongOrder, announced := sendThrough(t, admission, recvAnnounce(pathsLimitNLRI(3, pathsLimitElectPrefix)))
	if len(announced) != 0 {
		t.Fatalf("announce-first sent %v: the peer would hold 3 paths", announced)
	}
	wrongOrder.rollback()
	wrongOrder.release()
	withdrawn, _ := sendThrough(t, admission, recvWithdraw(pathsLimitNLRI(2, pathsLimitElectPrefix)))
	withdrawn.release()
	rightOrder, announced := sendThrough(t, admission, recvAnnounce(pathsLimitNLRI(3, pathsLimitElectPrefix)))
	defer rightOrder.release()
	if !equalIDs(announced, 3) {
		t.Fatalf("withdraw-first sent %v, want the newcomer [3]", announced)
	}
}

// TestPathsLimitElectionReusesMintedIdentifier proves AC-18-peer's second half
// and D-6: a survivor the destination holds stays under its identifier, and a
// path demoted then re-elected leaves under the identifier the forward rail
// minted for its source path before. Method: plan with the survivor held, and
// mint the same source path twice around a recipient release.
//
// VALIDATES: AC-18-peer, D-6.
func TestPathsLimitElectionReusesMintedIdentifier(t *testing.T) {
	kept, other := electPeer("192.0.2.1"), electPeer("192.0.2.2")
	held := []pathsLimitHeldPath{heldFrom(7, kept, 4)}
	plan := planPathsLimitElection(held, []ribevents.RecoveryRoute{survivorOf(kept, 4), survivorOf(other, 0)}, 2)
	if !slices.Equal(plan.announce, []int{1}) {
		t.Fatalf("announce %v, want only the path not held [1]", plan.announce)
	}
	if got := plan.withdrawals(1); len(got) != 0 {
		t.Fatalf("withdraw %v: a held survivor keeps its identifier", got)
	}

	table := newFwdPathIDTable()
	var key fwdPathKey
	var scratch [64]byte
	if err := fwdPathKeyFor(&key, family.IPv4Unicast, 4, pathsLimitNLRI(0, pathsLimitElectPrefix)[4:], false, scratch[:]); err != nil {
		t.Fatal(err)
	}
	minted := table.generatePath(source.SourceID(9), &key)
	table.retain(minted)
	table.release(minted) // The destination's slot leaves on demotion.
	if again := table.generatePath(source.SourceID(9), &key); again != minted {
		t.Fatalf("re-elected path minted %d, want its earlier identifier %d", again, minted)
	}
}

// TestPathsLimitElectionStaleResultRequeues proves D-4 and R-10: an election
// answered for an older question is not written; the route stays owed for the
// next round. Method: move the owed record's epoch between the lookup and the
// admission, then admit and settle.
//
// VALIDATES: D-4.
func TestPathsLimitElectionStaleResultRequeues(t *testing.T) {
	peer := electPeer("192.0.2.30")
	session := NewSession(peer.Settings())
	route := pathsLimitNLRI(0, pathsLimitElectPrefix)[4:]
	key := adjOutKey{addPath: true}
	var scratch [64]byte
	if err := fwdPathKeyFor(&key.path, family.IPv4Unicast, 0, route, false, scratch[:]); err != nil {
		t.Fatal(err)
	}
	peer.adjOut.mu.Lock()
	record := peer.adjOut.owe(key, session, family.IPv4Unicast, route)
	peer.adjOut.mu.Unlock()
	admission := &recoveryAdmission{peer: peer, session: session, pathsLimit: 2, done: make(chan struct{}),
		keys: []recoveryAdmitKey{{key: key, record: record, epoch: record.epoch}}}
	record.epoch++ // A newer refusal moved the question after the lookup.
	if !admission.admit(session) {
		t.Fatal("the admission belongs to this session")
	}
	if got := admission.keys[0].outcome; got != recoveryKeyLeftOwed {
		t.Fatalf("stale election outcome %d, want left owed", got)
	}
	admission.settle()
	if peer.adjOut.owed[key] != record {
		t.Fatal("a stale election must leave the route owed for the next round")
	}
}

// TestPathsLimitElectionKeepsNonRIBPaths proves D-7: an originated path the
// RIB does not rank keeps its slot, so the election fills only the rest.
// Method: plan with one originated and one forwarded path held.
//
// VALIDATES: D-7.
func TestPathsLimitElectionKeepsNonRIBPaths(t *testing.T) {
	forwarded, best, next := electPeer("192.0.2.1"), electPeer("192.0.2.2"), electPeer("192.0.2.3")
	held := []pathsLimitHeldPath{{id: 5}, heldFrom(6, forwarded, 0)}
	plan := planPathsLimitElection(held, []ribevents.RecoveryRoute{survivorOf(best, 0), survivorOf(next, 0)}, 2)
	if !slices.Equal(plan.announce, []int{0}) {
		t.Fatalf("announce %v, want one slot for the RIB's best [0]", plan.announce)
	}
	got := plan.withdrawals(1)
	if slices.Contains(got, 5) {
		t.Fatalf("withdraw %v names the originated path", got)
	}
	if !slices.Equal(got, []uint32{6}) {
		t.Fatalf("withdraw %v, want the outranked forwarded path [6]", got)
	}
}

// TestPathsLimitElectionRetainedSurvivorFillsOneSlot proves a path graceful
// restart retains is counted once: it pins its slot, and when the RIB ranks it
// among the survivors (its source is back up, and RFC 4724 forbids telling a
// stale route apart) the next survivor still takes the remaining slot, so the
// peer receives its limit of paths rather than one fewer. Method: limit 2, the
// destination holds the retained path and one outranked path, and the RIB
// answers the retained path first, then a better newcomer.
//
// VALIDATES: draft-abraitis-idr-addpath-paths-limit Section 1, RFC 4724
// Section 4.2.
func TestPathsLimitElectionRetainedSurvivorFillsOneSlot(t *testing.T) {
	restarted, outranked, newcomer := electPeer("192.0.2.1"), electPeer("192.0.2.2"), electPeer("192.0.2.3")
	retained := pathsLimitHeldPath{id: 1, owner: adjOutPath{source: restarted, addPath: true}}
	held := []pathsLimitHeldPath{retained, heldFrom(2, outranked, 0)}
	plan := planPathsLimitElection(held, []ribevents.RecoveryRoute{survivorOf(restarted, 0), survivorOf(newcomer, 0)}, 2)
	if !slices.Equal(plan.announce, []int{1}) {
		t.Fatalf("announce %v, want the newcomer [1] in the slot the retained path does not pin", plan.announce)
	}
	got := plan.withdrawals(len(plan.announce))
	if slices.Contains(got, 1) {
		t.Fatalf("withdraw %v names the retained path", got)
	}
	if !slices.Equal(got, []uint32{2}) {
		t.Fatalf("withdraw %v, want the outranked path [2]", got)
	}
}

// TestPathsLimitElectionFailureWarnsOnce proves D-8: with no RIB to rank,
// arrival order stands, every contested route is counted, and the session
// warns once for the family. Method: contest two routes on a session with no
// recovery owner and owe their elections.
//
// VALIDATES: D-8, R-12.
func TestPathsLimitElectionFailureWarnsOnce(t *testing.T) {
	log := captureSessionLog(t, slog.LevelWarn)
	registry := newSpyRegistry()
	session := NewSession(NewPeerSettings(netip.MustParseAddr("192.0.2.40"), 65001, 65002, 0x01020304))
	session.prefixMetrics = initReactorMetrics(registry, "test", "192.0.2.1", "65001")
	admission := sendBound(t)
	for _, prefix := range []string{pathsLimitElectPrefix, "203.0.113.128/25"} {
		changes, announced := sendThrough(t, admission, recvAnnounce(
			pathsLimitNLRI(1, prefix), pathsLimitNLRI(2, prefix), pathsLimitNLRI(3, prefix)))
		if !equalIDs(announced, 1, 2) {
			t.Fatalf("arrival order sent %v, want [1 2]", announced)
		}
		// The writer owes the elections under its held table mutex.
		ownership, err := session.beginAdjOut(false)
		if err != nil {
			t.Fatalf("begin adj-RIB-Out write: %v", err)
		}
		ownership.owePathsLimitElections(changes)
		ownership.release()
		changes.release()
	}
	if got := strings.Count(log.String(), "could not rank the paths"); got != 1 {
		t.Fatalf("logged %d election warnings, want 1:\n%s", got, log.String())
	}
	counter := registry.counterVecs["ze_bgp_paths_limit_election_failed_total"]
	if counter == nil {
		t.Fatal("ze_bgp_paths_limit_election_failed_total must be registered")
	}
	series := counter.get(session.peerLabel(), family.IPv4Unicast.String())
	if series == nil {
		t.Fatal("the failed election was not counted")
	}
	if got := series.Value(); got != 2 {
		t.Fatalf("failed elections counted %v, want 2", got)
	}
}
