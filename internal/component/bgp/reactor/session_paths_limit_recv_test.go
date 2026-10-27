package reactor

import (
	"bytes"
	"encoding/binary"
	"log/slog"
	"net/netip"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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

const pathsLimitRecvOther = "203.0.113.0/24"

// recvCeilingSession is an established session whose receive ceiling is 2 for
// every family named, built the way OPEN negotiation builds it, with an
// installed prefix maximum, and whose delivered UPDATEs are recorded.
func recvCeilingSession(t *testing.T, maximum uint32, fams ...family.Family) (*Session, *[][]byte) {
	t.Helper()
	s := prefixReceiveSession(t, fams[0], PrefixCountInstalled, maximum, true, fams[1:]...)
	limits := make(map[family.Family]uint16)
	modes := make(map[family.Family]capability.AddPathMode)
	for _, fam := range fams {
		limits[fam] = 2
		modes[fam] = capability.AddPathBoth
	}
	s.initPathsLimitRecv(&capability.EncodingCaps{PathsLimitRecv: limits, AddPathMode: modes})
	if len(s.pathsLimitRecv) != len(fams) {
		t.Fatalf("receive ceiling holds %d families, want %d", len(s.pathsLimitRecv), len(fams))
	}
	var delivered [][]byte
	s.onMessageReceived = func(_ netip.Addr, _ msgtype.MessageType, _ []byte,
		wu *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string, _ uint64) bool {
		delivered = append(delivered, bytes.Clone(wu.Payload()))
		return false
	}
	return s, &delivered
}

// recvProcess hands one UPDATE body to the session's real receive entry point
// and requires the session to survive it.
func recvProcess(t *testing.T, s *Session, body []byte) {
	t.Helper()
	header := message.Header{Type: msgtype.TypeUPDATE, Length: uint16(message.HeaderLen + len(body))}
	if err, kept := s.processMessage(&header, body, BufHandle{ID: noPoolBufID, Buf: body}); err != nil || kept {
		t.Fatalf("receive returned err=%v kept=%v", err, kept)
	}
	if s.State() != fsm.StateEstablished {
		t.Fatalf("session left Established: %v", s.State())
	}
}

// recvSessionTracked answers what the session's ceiling holds for one route.
func recvSessionTracked(t *testing.T, s *Session, fam family.Family, prefix string) (admitted, refused int) {
	t.Helper()
	state := s.pathsLimitRecv[fam]
	key, err := state.key(pathsLimitNLRI(0, prefix)[4:], state.keyScratch[:], false)
	if err != nil {
		t.Fatal(err)
	}
	entry := state.prefixes[string(key)]
	if entry == nil {
		return 0, 0
	}
	return len(entry.ids), int(entry.refusedEntries)
}

// mpReachPaths answers the path identifier and prefix of every MP_REACH NLRI
// in a delivered UPDATE, in wire order.
func mpReachPaths(t *testing.T, fam family.Family, body []byte) []string {
	t.Helper()
	nlri, ok := mpReachNLRIOf(t, body)
	if !ok {
		return nil
	}
	var paths []string
	if _, err := nlrisplit.Get(fam)(nlri, true, func(raw []byte) bool {
		for _, prefix := range []string{pathsLimitRecvPrefix, pathsLimitRecvOther, "2001:db8:1::/48"} {
			for id := range uint32(8) {
				if bytes.Equal(raw, pathsLimitNLRI(id, prefix)) {
					paths = append(paths, prefix+"#"+strconv.FormatUint(uint64(id), 10))
					return true
				}
			}
		}
		t.Fatalf("unexpected NLRI %x", raw)
		return false
	}); err != nil {
		t.Fatal(err)
	}
	return paths
}

// TestReceiveCeilingRollsBackOnPrefixLimitDrop admits a path, then sends an
// UPDATE the ceiling admits in part but the prefix maximum drops whole. The
// ceiling must be left as before that UPDATE, so the same path is admitted
// when it arrives alone (D-10).
func TestReceiveCeilingRollsBackOnPrefixLimitDrop(t *testing.T) {
	s, delivered := recvCeilingSession(t, 2, family.IPv4Unicast)
	recvProcess(t, s, prefixReceiveBody(family.IPv4Unicast, pathsLimitNLRI(1, pathsLimitRecvPrefix), nil))

	// The ceiling admits #2 and the other route and refuses #3; three
	// installed routes then pass the maximum of 2, so the UPDATE is dropped.
	recvProcess(t, s, prefixReceiveBody(family.IPv4Unicast, bytes.Join([][]byte{
		pathsLimitNLRI(2, pathsLimitRecvPrefix),
		pathsLimitNLRI(3, pathsLimitRecvPrefix),
		pathsLimitNLRI(1, pathsLimitRecvOther),
	}, nil), nil))
	if len(*delivered) != 1 {
		t.Fatalf("delivered %d UPDATEs, want 1: the over-maximum UPDATE must be dropped", len(*delivered))
	}
	if admitted, refused := recvSessionTracked(t, s, family.IPv4Unicast, pathsLimitRecvPrefix); admitted != 1 || refused != 0 {
		t.Fatalf("after the drop the ceiling holds %d admitted, %d refused; want 1, 0", admitted, refused)
	}
	if admitted, _ := recvSessionTracked(t, s, family.IPv4Unicast, pathsLimitRecvOther); admitted != 0 {
		t.Fatalf("after the drop the other route holds %d admitted paths, want 0", admitted)
	}

	recvProcess(t, s, prefixReceiveBody(family.IPv4Unicast, pathsLimitNLRI(3, pathsLimitRecvPrefix), nil))
	if len(*delivered) != 2 {
		t.Fatalf("delivered %d UPDATEs, want 2", len(*delivered))
	}
	if got := strings.Join(mpReachPaths(t, family.IPv4Unicast, (*delivered)[1]), " "); got != pathsLimitRecvPrefix+"#3" {
		t.Fatalf("delivered paths %q, want %s#3", got, pathsLimitRecvPrefix)
	}
}

// TestReceiveCeilingSeesTreatAsWithdrawExtras fills an IPv6 route to its
// ceiling, then sends a malformed UPDATE that also withdraws an IPv4 route, so
// RFC 7606 treat-as-withdraw carries the IPv6 withdrawal in its own extra body.
// That withdrawal must free the slot: a new path is then admitted.
func TestReceiveCeilingSeesTreatAsWithdrawExtras(t *testing.T) {
	const route6 = "2001:db8:1::/48"
	s, delivered := recvCeilingSession(t, 100, family.IPv6Unicast, family.IPv4Unicast)
	recvProcess(t, s, prefixReceiveBody(family.IPv6Unicast, bytes.Join([][]byte{
		pathsLimitNLRI(1, route6),
		pathsLimitNLRI(2, route6),
	}, nil), nil))

	malformed := prefixReceiveBody(family.IPv6Unicast, pathsLimitNLRI(1, route6), nil)
	sections, err := wire.ParseUpdateSections(malformed)
	if err != nil {
		t.Fatal(err)
	}
	// The IPv4 withdrawal comes first, so IPv6 rides the extra body; ORIGIN 3
	// is undefined (RFC 7606 Section 7.1: treat-as-withdraw).
	attrs := append(mpUnreachAttrs(family.IPv4Unicast, pathsLimitNLRI(1, pathsLimitRecvOther)), sections.Attrs(malformed)...)
	origin := bytes.Index(attrs, []byte{0x40, 1, 1, 0})
	if origin < 0 {
		t.Fatal("test body carries no ORIGIN")
	}
	attrs[origin+3] = 3
	recvProcess(t, s, makeUpdateBody(nil, attrs, nil))
	if len(*delivered) != 3 {
		t.Fatalf("delivered %d UPDATEs, want 3: the fill, then the primary and one extra body", len(*delivered))
	}
	if admitted, _ := recvSessionTracked(t, s, family.IPv6Unicast, route6); admitted != 1 {
		t.Fatalf("after the treat-as-withdraw extra the ceiling holds %d paths, want 1", admitted)
	}

	recvProcess(t, s, prefixReceiveBody(family.IPv6Unicast, pathsLimitNLRI(3, route6), nil))
	if got := strings.Join(mpReachPaths(t, family.IPv6Unicast, (*delivered)[len(*delivered)-1]), " "); got != route6+"#3" {
		t.Fatalf("delivered paths %q, want %s#3", got, route6)
	}
}

// TestReceiveCeilingSeesTypedWithdrawExtras is the sibling of
// TestReceiveCeilingSeesTreatAsWithdrawExtras for the other extra-body loop of
// processValidatedMessage: validation.typedWithdrawBodies, the RFC 7606
// withdrawal of a malformed typed NLRI by its held well-formed generation.
// Method: fill a BGP-MUP T1ST route to its ceiling of 2 under ADD-PATH
// receive (paths 1 and 2), then send a malformed generation of path 1 in an
// UPDATE that already withdraws an IPv6 route. The MP_UNREACH_NLRI is taken,
// so the typed withdrawal of path 1 rides its own withdraw-only body, and that
// body MUST free path 1's slot: the ceiling then holds 1 path, and a new path
// 3 is admitted and delivered.
func TestReceiveCeilingSeesTypedWithdrawExtras(t *testing.T) {
	k0 := mupWireNLRI(3, t1stBody(2, 0x00)...)
	k := mupWireNLRI(3, t1stBody(2, 0x00, 0xf0)...)
	withID := func(id uint32, nlri []byte) []byte {
		return append(binary.BigEndian.AppendUint32(nil, id), nlri...)
	}
	s, delivered := recvCeilingSession(t, 100, mupFam, family.IPv6Unicast)
	tracked := func() int {
		t.Helper()
		state := s.pathsLimitRecv[mupFam]
		key, err := state.key(k0, state.keyScratch[:], false)
		if err != nil {
			t.Fatal(err)
		}
		if entry := state.prefixes[string(key)]; entry != nil {
			return len(entry.ids)
		}
		return 0
	}
	recvProcess(t, s, prefixReceiveBody(mupFam, append(withID(1, k0), withID(2, k0)...), nil))
	if admitted := tracked(); admitted != 2 {
		t.Fatalf("the fill admitted %d paths, want 2", admitted)
	}

	malformed := prefixReceiveBody(mupFam, withID(1, k), nil)
	sections, err := wire.ParseUpdateSections(malformed)
	if err != nil {
		t.Fatal(err)
	}
	attrs := append(mpUnreachAttrs(family.IPv6Unicast, pathsLimitNLRI(1, "2001:db8:9::/48")), sections.Attrs(malformed)...)
	recvProcess(t, s, makeUpdateBody(nil, attrs, nil))
	if len(*delivered) != 3 {
		t.Fatalf("delivered %d UPDATEs, want 3: the fill, the typed withdrawal body and the primary", len(*delivered))
	}
	if admitted := tracked(); admitted != 1 {
		t.Fatalf("after the typed withdrawal body the ceiling holds %d paths, want 1", admitted)
	}

	recvProcess(t, s, prefixReceiveBody(mupFam, withID(3, k0), nil))
	if len(*delivered) != 4 {
		t.Fatalf("delivered %d UPDATEs, want 4: path 3 takes the freed slot", len(*delivered))
	}
	if admitted := tracked(); admitted != 2 {
		t.Fatalf("after path 3 the ceiling holds %d paths, want 2", admitted)
	}
}

// TestReceiveCeilingEmptyResultIsNotEndOfRIB fills a route to its ceiling,
// then sends an UPDATE whose only path is over it. Nothing may be delivered:
// an empty body would read as End-of-RIB. The session survives and the
// refusal is tracked.
func TestReceiveCeilingEmptyResultIsNotEndOfRIB(t *testing.T) {
	s, delivered := recvCeilingSession(t, 100, family.IPv4Unicast)
	recvProcess(t, s, prefixReceiveBody(family.IPv4Unicast, bytes.Join([][]byte{
		pathsLimitNLRI(1, pathsLimitRecvPrefix),
		pathsLimitNLRI(2, pathsLimitRecvPrefix),
	}, nil), nil))

	recvProcess(t, s, prefixReceiveBody(family.IPv4Unicast, pathsLimitNLRI(3, pathsLimitRecvPrefix), nil))
	if len(*delivered) != 1 {
		t.Fatalf("delivered %d UPDATEs, want 1: a fully refused UPDATE is consumed", len(*delivered))
	}
	if admitted, refused := recvSessionTracked(t, s, family.IPv4Unicast, pathsLimitRecvPrefix); admitted != 2 || refused != 1 {
		t.Fatalf("ceiling holds %d admitted, %d refused; want 2, 1", admitted, refused)
	}
}

// TestReceiveCeilingAbsentWithoutReceiveDirection builds the ceiling from the
// negotiated encoding: a limit on a family without ADD-PATH receive enforces
// nothing (AC-11), and each receive family keeps its own limit (AC-3-receive).
func TestReceiveCeilingAbsentWithoutReceiveDirection(t *testing.T) {
	s := NewSession(NewPeerSettings(netip.MustParseAddr("192.0.2.9"), 65001, 65002, 0x01020309))
	s.initPathsLimitRecv(&capability.EncodingCaps{
		PathsLimitRecv: map[family.Family]uint16{family.IPv4Unicast: 2},
		AddPathMode:    map[family.Family]capability.AddPathMode{family.IPv4Unicast: capability.AddPathSend},
	})
	if s.pathsLimitRecv != nil {
		t.Fatalf("a send-only family built a receive ceiling: %v", s.pathsLimitRecv)
	}

	s.initPathsLimitRecv(&capability.EncodingCaps{
		PathsLimitRecv: map[family.Family]uint16{family.IPv4Unicast: 2, family.IPv6Unicast: 10, family.IPv4Multicast: 10},
		AddPathMode: map[family.Family]capability.AddPathMode{
			family.IPv4Unicast:   capability.AddPathBoth,
			family.IPv6Unicast:   capability.AddPathReceive,
			family.IPv4Multicast: capability.AddPathSend,
		},
	})
	if len(s.pathsLimitRecv) != 2 {
		t.Fatalf("receive ceiling holds %d families, want 2", len(s.pathsLimitRecv))
	}
	for fam, limit := range map[family.Family]uint16{family.IPv4Unicast: 2, family.IPv6Unicast: 10} {
		if state := s.pathsLimitRecv[fam]; state == nil || state.limit != limit {
			t.Fatalf("%s ceiling %+v, want limit %d", fam, state, limit)
		}
	}

	s.initPathsLimitRecv(nil)
	if s.pathsLimitRecv != nil {
		t.Fatal("a session with no negotiated encoding kept a receive ceiling")
	}
}

// TestPathsLimitUnsplittableFamilyIsNamedNotEnforced proves a family Ze holds
// no NLRI splitter for neither panics nor drops UPDATEs: the admission leaves
// it out, its bytes pass unchanged, and the session warns that its negotiated
// limit is not enforced. Method: negotiate a limit on IPv4 unicast and on a
// private SAFI with no splitter, then feed the private family's NLRI through
// the admission and read the warning.
//
// VALIDATES: a limit Ze cannot count is reported, never silently dropped or
// silently unlimited.
func TestPathsLimitUnsplittableFamilyIsNamedNotEnforced(t *testing.T) {
	unsplittable := family.Family{AFI: family.AFIIPv4, SAFI: 241}
	if nlrisplit.Get(unsplittable) != nil {
		t.Fatalf("SAFI 241 gained a splitter; pick a family with none")
	}
	limits := map[family.Family]uint16{family.IPv4Unicast: 2, unsplittable: 2}
	modes := map[family.Family]capability.AddPathMode{
		family.IPv4Unicast: capability.AddPathBoth, unsplittable: capability.AddPathBoth}
	log := captureSessionLog(t, slog.LevelWarn)
	s := NewSession(NewPeerSettings(netip.MustParseAddr("192.0.2.41"), 65001, 65002, 0x01020341))
	s.initPathsLimitRecv(&capability.EncodingCaps{PathsLimitRecv: limits, AddPathMode: modes})
	if s.pathsLimitRecv[family.IPv4Unicast] == nil {
		t.Fatal("IPv4 unicast must keep its receive ceiling")
	}
	if s.pathsLimitRecv[unsplittable] != nil {
		t.Fatal("a family with no splitter must not be admitted path by path")
	}
	if got := strings.Count(log.String(), "negotiated but not enforced"); got != 1 {
		t.Fatalf("logged %d unenforced warnings, want 1:\n%s", got, log.String())
	}
	if !strings.Contains(log.String(), unsplittable.String()) {
		t.Fatalf("the warning does not name %s:\n%s", unsplittable, log.String())
	}

	changes, _ := pathsLimitScratch.Get().(*pathsLimitChanges)
	defer changes.release()
	data := []byte{0, 0, 0, 1, 0xde, 0xad, 0xbe, 0xef}
	dst := make([]byte, len(data))
	n, dropped, err := s.pathsLimitRecv.section(dst, data, unsplittable, false, changes)
	if err != nil {
		t.Fatalf("an unenforced family refused its NLRI: %v", err)
	}
	if n != len(data) || dropped != 0 || !bytes.Equal(dst, data) {
		t.Fatalf("section copied %d of %d octets, dropped %d: the bytes must pass unchanged", n, len(data), dropped)
	}
}
