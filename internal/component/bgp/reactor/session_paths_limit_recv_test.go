package reactor

import (
	"bytes"
	"encoding/binary"
	"log/slog"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

const pathsLimitRecvPrefix = "198.51.100.0/24"

// recvCeiling is the receive admission at ceiling 2 for IPv4 unicast, with a
// session whose refusals are logged and counted.
type recvCeiling struct {
	admission pathsLimitAdmission
	session   *Session
	registry  *spyRegistry
}

func newRecvCeiling(t *testing.T) *recvCeiling {
	t.Helper()
	admission := newPathsLimitAdmission(
		map[family.Family]uint16{family.IPv4Unicast: 2},
		map[family.Family]capability.AddPathMode{family.IPv4Unicast: capability.AddPathBoth},
		capability.AddPathReceive)
	if admission == nil {
		t.Fatal("a negotiated receive direction with a limit must build a ceiling")
	}
	registry := newSpyRegistry()
	session := NewSession(NewPeerSettings(netip.MustParseAddr("192.0.2.9"), 65001, 65002, 0x01020309))
	session.prefixMetrics = initReactorMetrics(registry, "test", "192.0.2.1", "65001")
	return &recvCeiling{admission: admission, session: session, registry: registry}
}

// receive runs one UPDATE body through the ceiling, keeps its journal, and
// answers the announced and withdrawn path identifiers left in the rewrite.
func (r *recvCeiling) receive(t *testing.T, body []byte) (announced, withdrawn []uint32) {
	t.Helper()
	changes, _ := pathsLimitScratch.Get().(*pathsLimitChanges)
	defer changes.release()
	dst := make([]byte, message.ExtMsgLen)
	n, _, err := r.admission.filter(dst, body, changes)
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	r.session.reportPathsLimitRefusals(changes)
	if n == 0 {
		return nil, nil
	}
	return recvBodyPathIDs(t, dst[:n])
}

func recvBodyPathIDs(t *testing.T, body []byte) (announced, withdrawn []uint32) {
	t.Helper()
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		t.Fatal(err)
	}
	read := func(data []byte, split nlrisplit.Splitter) []uint32 {
		var ids []uint32
		if _, err := split(data, true, func(raw []byte) bool {
			ids = append(ids, binary.BigEndian.Uint32(raw))
			return true
		}); err != nil {
			t.Fatal(err)
		}
		return ids
	}
	return read(sections.NLRI(body), nlrisplit.Get(family.IPv4Unicast)),
		read(sections.Withdrawn(body), nlrisplit.GetWithdraw(family.IPv4Unicast))
}

func recvAnnounce(routes ...[]byte) []byte {
	return makeUpdateBody(nil, nil, bytes.Join(routes, nil))
}

func recvWithdraw(routes ...[]byte) []byte {
	return makeUpdateBody(bytes.Join(routes, nil), nil, nil)
}

// tracked answers how many identifiers the ceiling holds for
// pathsLimitRecvPrefix, admitted and refused.
func (r *recvCeiling) tracked(t *testing.T) (admitted, refused int) {
	t.Helper()
	state := r.admission[family.IPv4Unicast]
	key, err := state.key(pathsLimitNLRI(0, pathsLimitRecvPrefix)[4:], state.keyScratch[:], false)
	if err != nil {
		t.Fatal(err)
	}
	entry := state.prefixes[string(key)]
	if entry == nil {
		return 0, 0
	}
	return len(entry.ids), int(entry.refusedEntries)
}

func (r *recvCeiling) refusedCount(t *testing.T) float64 {
	t.Helper()
	counter := r.registry.counterVecs["ze_bgp_paths_limit_refused_total"]
	if counter == nil {
		t.Fatal("ze_bgp_paths_limit_refused_total must be registered")
	}
	series := counter.get(r.session.peerLabel(), family.IPv4Unicast.String())
	if series == nil {
		return 0
	}
	return series.Value()
}

func equalIDs(got []uint32, want ...uint32) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestReceiveCeilingAdmitsUpToLimit proves that ceiling 2 admits two path
// identifiers for one prefix, refuses the third, and leaves every other prefix
// in the same UPDATE untouched. Method: one UPDATE carrying three paths for one
// prefix and one path for another, read back from the rewritten body.
//
// VALIDATES: AC-8 at unit level.
func TestReceiveCeilingAdmitsUpToLimit(t *testing.T) {
	r := newRecvCeiling(t)
	announced, _ := r.receive(t, recvAnnounce(
		pathsLimitNLRI(1, pathsLimitRecvPrefix),
		pathsLimitNLRI(2, pathsLimitRecvPrefix),
		pathsLimitNLRI(3, pathsLimitRecvPrefix),
		pathsLimitNLRI(7, "203.0.113.0/24"),
	))
	if !equalIDs(announced, 1, 2, 7) {
		t.Fatalf("admitted %v, want [1 2 7]: the third path is refused and the other prefix kept", announced)
	}
	if got := r.refusedCount(t); got != 1 {
		t.Fatalf("refused counter %v, want 1", got)
	}
	// A replacement on an admitted identifier at the ceiling consumes no slot.
	announced, _ = r.receive(t, recvAnnounce(pathsLimitNLRI(2, pathsLimitRecvPrefix)))
	if !equalIDs(announced, 2) {
		t.Fatalf("replacement of admitted id 2 gave %v, want [2]", announced)
	}
}

// TestReceiveCeilingAcceptsWithdrawOfRefusedPath proves that a WITHDRAW of an
// identifier the ceiling refused is accepted without error and frees no slot
// that was never taken. Method: fill the ceiling, refuse id 3, withdraw it,
// then offer a new id 4, which must still be refused.
//
// VALIDATES: AC-9, R-4.
func TestReceiveCeilingAcceptsWithdrawOfRefusedPath(t *testing.T) {
	r := newRecvCeiling(t)
	r.receive(t, recvAnnounce(
		pathsLimitNLRI(1, pathsLimitRecvPrefix),
		pathsLimitNLRI(2, pathsLimitRecvPrefix),
		pathsLimitNLRI(3, pathsLimitRecvPrefix),
	))
	_, withdrawn := r.receive(t, recvWithdraw(pathsLimitNLRI(3, pathsLimitRecvPrefix)))
	if !equalIDs(withdrawn, 3) {
		t.Fatalf("withdrawal of the refused id gave %v, want it passed through as [3]", withdrawn)
	}
	if admitted, _ := r.tracked(t); admitted != 2 {
		t.Fatalf("admitted %d after withdrawing a refused id, want 2", admitted)
	}
	announced, _ := r.receive(t, recvAnnounce(pathsLimitNLRI(4, pathsLimitRecvPrefix)))
	if len(announced) != 0 {
		t.Fatalf("new id 4 admitted as %v: the refused withdrawal freed a slot it never held", announced)
	}
}

// TestReceiveCeilingReadmitsAfterWithdraw proves that withdrawing an admitted
// identifier frees a slot, so the previously refused identifier is admitted
// when re-announced. Method: fill, refuse 3, withdraw 1, re-announce 3.
//
// VALIDATES: AC-10.
func TestReceiveCeilingReadmitsAfterWithdraw(t *testing.T) {
	r := newRecvCeiling(t)
	r.receive(t, recvAnnounce(
		pathsLimitNLRI(1, pathsLimitRecvPrefix),
		pathsLimitNLRI(2, pathsLimitRecvPrefix),
		pathsLimitNLRI(3, pathsLimitRecvPrefix),
	))
	r.receive(t, recvWithdraw(pathsLimitNLRI(1, pathsLimitRecvPrefix)))
	announced, _ := r.receive(t, recvAnnounce(pathsLimitNLRI(3, pathsLimitRecvPrefix)))
	if !equalIDs(announced, 3) {
		t.Fatalf("re-announced id 3 gave %v, want it admitted into the freed slot", announced)
	}
	admitted, refused := r.tracked(t)
	if admitted != 2 {
		t.Fatalf("admitted %d, want 2", admitted)
	}
	if refused != 0 {
		t.Fatalf("id 3 still tracked as refused after admission (%d refused)", refused)
	}
}

// TestReceiveCeilingBoundsTrackedPaths proves the tracked identifier set for
// one prefix never exceeds the limit plus one headroom slot, however many
// excess paths the peer sends. Method: five excess paths in separate UPDATEs.
//
// VALIDATES: AC-19, R-3.
func TestReceiveCeilingBoundsTrackedPaths(t *testing.T) {
	r := newRecvCeiling(t)
	r.receive(t, recvAnnounce(pathsLimitNLRI(1, pathsLimitRecvPrefix), pathsLimitNLRI(2, pathsLimitRecvPrefix)))
	for id := uint32(3); id <= 7; id++ {
		r.receive(t, recvAnnounce(pathsLimitNLRI(id, pathsLimitRecvPrefix)))
		admitted, refused := r.tracked(t)
		if admitted+refused > 2+pathsLimitRefusedHeadroom {
			t.Fatalf("after id %d the prefix tracks %d identifiers, want at most %d", id, admitted+refused, 2+pathsLimitRefusedHeadroom)
		}
	}
	admitted, refused := r.tracked(t)
	if admitted != 2 {
		t.Fatalf("admitted %d, want 2", admitted)
	}
	if refused != pathsLimitRefusedHeadroom {
		t.Fatalf("refused tracked %d, want %d", refused, pathsLimitRefusedHeadroom)
	}
}

// TestReceiveCeilingWarnsOncePerPrefix proves five excess paths for one prefix
// produce one warning and five counter increments. Method: capture the session
// log and the spy registry across five separate UPDATEs.
//
// VALIDATES: AC-19.
func TestReceiveCeilingWarnsOncePerPrefix(t *testing.T) {
	log := captureSessionLog(t, slog.LevelWarn)
	r := newRecvCeiling(t)
	r.receive(t, recvAnnounce(pathsLimitNLRI(1, pathsLimitRecvPrefix), pathsLimitNLRI(2, pathsLimitRecvPrefix)))
	for id := uint32(3); id <= 7; id++ {
		r.receive(t, recvAnnounce(pathsLimitNLRI(id, pathsLimitRecvPrefix)))
	}
	if got := strings.Count(log.String(), "the excess path is dropped"); got != 1 {
		t.Fatalf("logged %d refusal lines, want 1:\n%s", got, log.String())
	}
	if !strings.Contains(log.String(), "path-id=3") {
		t.Fatalf("the warning must name the first refused path id 3:\n%s", log.String())
	}
	if got := r.refusedCount(t); got != 5 {
		t.Fatalf("refused counter %v, want 5", got)
	}
}

// TestReceiveCeilingClearsWarnedMarkerOnWithdraw proves that withdrawing the
// tracked refused identifier frees its tracking slot and clears the warned
// marker, so the next excess path warns again. Method: saturate, withdraw the
// tracked id 3, then send excess id 8.
//
// VALIDATES: AC-20.
func TestReceiveCeilingClearsWarnedMarkerOnWithdraw(t *testing.T) {
	log := captureSessionLog(t, slog.LevelWarn)
	r := newRecvCeiling(t)
	r.receive(t, recvAnnounce(pathsLimitNLRI(1, pathsLimitRecvPrefix), pathsLimitNLRI(2, pathsLimitRecvPrefix)))
	for id := uint32(3); id <= 7; id++ {
		r.receive(t, recvAnnounce(pathsLimitNLRI(id, pathsLimitRecvPrefix)))
	}
	// An untracked refused id frees nothing and keeps the marker.
	r.receive(t, recvWithdraw(pathsLimitNLRI(5, pathsLimitRecvPrefix)))
	if _, refused := r.tracked(t); refused != 1 {
		t.Fatalf("withdrawing untracked id 5 changed the tracked set to %d", refused)
	}
	r.receive(t, recvWithdraw(pathsLimitNLRI(3, pathsLimitRecvPrefix)))
	if _, refused := r.tracked(t); refused != 0 {
		t.Fatalf("withdrawing tracked id 3 left %d tracked", refused)
	}
	r.receive(t, recvAnnounce(pathsLimitNLRI(8, pathsLimitRecvPrefix)))
	if got := strings.Count(log.String(), "the excess path is dropped"); got != 2 {
		t.Fatalf("logged %d refusal lines, want 2 (the new excess warns again):\n%s", got, log.String())
	}
	if !strings.Contains(log.String(), "path-id=8") {
		t.Fatalf("the fresh warning must name path id 8:\n%s", log.String())
	}
}

// TestReceiveCeilingRollbackRestoresRefusals proves the journal undoes every
// receive change kind, so a message dropped after admission leaves the state as
// it was before it. Method: one UPDATE that withdraws the tracked refused id
// and an admitted id, admits the refused id into the freed slot and refuses a
// new one, then rollback, then compare.
func TestReceiveCeilingRollbackRestoresRefusals(t *testing.T) {
	r := newRecvCeiling(t)
	r.receive(t, recvAnnounce(
		pathsLimitNLRI(1, pathsLimitRecvPrefix),
		pathsLimitNLRI(2, pathsLimitRecvPrefix),
		pathsLimitNLRI(3, pathsLimitRecvPrefix),
	))
	state := r.admission[family.IPv4Unicast]
	key, err := state.key(pathsLimitNLRI(0, pathsLimitRecvPrefix)[4:], state.keyScratch[:], false)
	if err != nil {
		t.Fatal(err)
	}
	before := *state.prefixes[string(key)]
	beforeIDs := len(before.ids)

	changes, _ := pathsLimitScratch.Get().(*pathsLimitChanges)
	defer changes.release()
	body := makeUpdateBody(pathsLimitNLRI(1, pathsLimitRecvPrefix), nil, bytes.Join([][]byte{
		pathsLimitNLRI(3, pathsLimitRecvPrefix),
		pathsLimitNLRI(4, pathsLimitRecvPrefix),
	}, nil))
	dst := make([]byte, message.ExtMsgLen)
	if _, _, err := r.admission.filter(dst, body, changes); err != nil {
		t.Fatal(err)
	}
	if changes.refusals != 1 {
		t.Fatalf("journal counted %d refusals, want 1 (id 4)", changes.refusals)
	}
	changes.rollback()

	after := state.prefixes[string(key)]
	if after == nil {
		t.Fatal("rollback lost the prefix")
	}
	if len(after.ids) != beforeIDs {
		t.Fatalf("rollback left %d admitted, want %d", len(after.ids), beforeIDs)
	}
	for _, id := range []uint32{1, 2} {
		if _, ok := after.ids[id]; !ok {
			t.Fatalf("rollback did not restore admitted id %d", id)
		}
	}
	if after.refusedEntries != before.refusedEntries || after.refused != before.refused {
		t.Fatalf("rollback left refused %v/%d, want %v/%d", after.refused, after.refusedEntries, before.refused, before.refusedEntries)
	}
}
