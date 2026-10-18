// Design: docs/architecture/update-building.md -- one final ownership boundary
// RFC: rfc/short/rfc4271.md -- source-scoped withdrawal and duplicate advertisements
// Related: adj_rib_out.go -- the existing per-recipient table
// Related: session_write.go -- caller holds writeMu through admission, write and flush
package reactor

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

var errAdjOutProvenance = errors.New("outbound path has no valid source ownership")
var errAdjOutSession = errors.New("outbound work belongs to a retired session")
var errRawOwnership = errors.New("raw diagnostic bytes sent; session retired because outbound ownership cannot be determined")

// adjOutWrite borrows one UPDATE's provenance. The pooled maps are bounded by
// native paths in one extended message and cleared before release. They are not
// a second recipient inventory. Callers MUST release after record or refusal.
type adjOutWrite struct {
	session   *Session
	table     *adjRIBOut
	encoding  *bgpctx.EncodingContext
	owners    map[adjOutKey]adjOutPath
	withdrawn map[adjOutKey]struct{}
	// owes, movers and refused stage this write's effects on owed keys:
	// the owner withdrawals it leaves to recovery, the forwarded withdrawals
	// of keys already owed, and the replays refused at an owed key. allow
	// runs twice for a filtered body, so it stages per key, never applies;
	// applyStaged applies once after the last pass. The bytes point into the
	// body.
	owes        map[adjOutKey]adjOutOwe
	movers      map[adjOutKey]adjOutPath
	refused     map[adjOutKey]struct{}
	keyScratch  [nlrisplit.PrefixKeyScratchSize]byte
	removed     int
	suppressed  int
	source      *Peer
	sourceSet   bool
	pathSources []wireu.SentPathSource
	raw         bool
	ordinals    map[family.Family]uint32
	// Counts come from the existing final-body record walk, before acceptance.
	announcedRoutes uint64
	withdrawnRoutes uint64
}

var adjOutWritePool = sync.Pool{New: func() any {
	return &adjOutWrite{
		owners:    make(map[adjOutKey]adjOutPath),
		withdrawn: make(map[adjOutKey]struct{}),
		owes:      make(map[adjOutKey]adjOutOwe),
		movers:    make(map[adjOutKey]adjOutPath),
		refused:   make(map[adjOutKey]struct{}),
		ordinals:  make(map[family.Family]uint32),
	}
}}

// beginAdjOut MUST be called under writeMu and paired with release. Captured
// Session identity and the teardown seal fence queued writes without acquiring
// Peer.mu (which precedes Session locks). The table's monotonic Session token
// additionally prevents an old writer from rebinding a replacement's frontier.
// Its mutex makes an old session's failure unable to clear replacement state.
func (s *Session) beginAdjOut(raw bool) (*adjOutWrite, error) {
	if s.writeFailed != nil {
		return nil, s.writeFailed
	}
	if s.tearingDown.Load() {
		return nil, errAdjOutSession
	}
	if item := s.sentForward; item != nil {
		if item.session != s {
			return nil, errAdjOutSession
		}
		if item.authority == adjOutUnspecified && !item.originated && item.endOfRIB == nil {
			return nil, errAdjOutProvenance
		}
		if item.authority == adjOutForwarded && !forwardSourceCurrent(item.receivedPeer, item.receivedGeneration) {
			return nil, errAdjOutProvenance
		}
		if item.authority == adjOutInitial {
			if err := s.checkInitialWrite(item); err != nil {
				return nil, err
			}
		}
		if item.peer != nil {
			s.adjOut = &item.peer.adjOut
		}
	}
	if s.adjOut == nil {
		s.adjOut = new(adjRIBOut)
	}
	s.adjOut.mu.Lock()
	if prior := s.adjOut.session; prior != nil && prior != s && prior.initialReplay > s.initialReplay {
		s.adjOut.mu.Unlock()
		return nil, errAdjOutSession
	}
	if s.adjOut.session != s {
		s.adjOut.session = s
		s.adjOut.routes = nil
		s.adjOut.dropOwed()
		s.adjOut.pending = false
	}
	work, _ := adjOutWritePool.Get().(*adjOutWrite)
	work.session, work.table = s, s.adjOut
	work.raw = raw
	work.encoding = bgpctx.Registry.Get(s.sendCtxID)
	return work, nil
}

// release MUST follow beginAdjOut on every path, while writeMu is still held.
func (w *adjOutWrite) release() {
	w.table.mu.Unlock()
	clear(w.owners)
	clear(w.withdrawn)
	w.clearStaged()
	clear(w.ordinals)
	w.session, w.table = nil, nil
	w.encoding = nil
	w.removed, w.suppressed = 0, 0
	w.source, w.pathSources = nil, nil
	w.sourceSet = false
	w.announcedRoutes, w.withdrawnRoutes = 0, 0
	adjOutWritePool.Put(w)
}

// key follows registered native identity rather than a family enumeration.
// RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility field
// MUST be ignored." RFC 7911 Section 3: "In order to carry the Path Identifier
// in an UPDATE message, the NLRI encoding MUST be extended by prepending the
// Path Identifier field, which is of four octets".
func (w *adjOutWrite) key(fam family.Family, raw []byte, withdraw bool) (adjOutKey, error) {
	key := adjOutKey{addPath: w.encoding.AddPathFor(fam)}
	id := uint32(0)
	if key.addPath {
		if len(raw) < 4 {
			return key, errAdjOutProvenance
		}
		id = binary.BigEndian.Uint32(raw)
		raw = raw[4:]
	}
	// RFC 8277 Section 2.4 and RFC 7911 Section 5.
	err := fwdPathKeyFor(&key.path, fam, id, raw, withdraw, w.keyScratch[:])
	return key, err
}

func adjOutSameOwner(a, b adjOutPath) bool {
	return a.source == b.source && a.received == b.received && a.addPath == b.addPath
}

func (w *adjOutWrite) sourceBound() bool {
	item := w.session.sentForward
	return item != nil && item.authority != adjOutUnspecified
}

// allow keeps intent per path. Ordinary ownership deliberately ignores the
// current message ID and source session generation: retained GR paths remain
// withdrawable by the same source on its new session.
// RFC 7911 Section 5: "If a BGP speaker receives a message to withdraw a
// prefix with a Path Identifier not seen before, it SHOULD silently ignore it".
func (w *adjOutWrite) allow(fam family.Family, raw []byte, withdraw, multiprotocol bool, occurrence int, sig adjRIBOutSignature, readable bool) (bool, error) {
	key, err := w.key(fam, raw, withdraw)
	if err != nil {
		return false, err
	}
	entry := w.table.routes[key]
	var owner adjOutPath
	if w.sourceBound() {
		item := w.session.sentForward
		owner, err = item.writePath(fam, raw, withdraw, multiprotocol, occurrence)
		if err != nil {
			return false, err
		}
		// The owner's session generation tells a departure which session of
		// the source wrote the path. An expected write keeps entry.owner.
		owner.generation = item.receivedGeneration
		switch item.authority {
		case adjOutUnspecified:
			return false, errAdjOutProvenance
		case adjOutForwarded:
			if owner.source == nil {
				return false, errAdjOutProvenance
			}
			if withdraw {
				// RFC 4271 Section 9.2: a withdrawal of a key whose
				// replacement is being elected may kill the chosen route.
				w.stageMover(key, owner)
				if entry == nil {
					return owner.synthesized, nil
				}
				if !adjOutSameOwner(entry.owner, owner) {
					return false, nil
				}
			}
		case adjOutExpected:
			if entry == nil {
				return false, nil
			}
			if !withdraw && w.table.owed[key] != nil {
				// A bgp-rib replay of an owed key replays the route being
				// replaced; the recovery write answers the replay instead.
				w.refused[key] = struct{}{}
				return false, nil
			}
			if owner.revision == 0 {
				return false, nil
			}
			if entry.revision != owner.revision {
				return false, nil
			}
			owner = entry.owner
		case adjOutInitial:
			if owner.source != item.receivedPeer {
				return false, errAdjOutProvenance
			}
			if entry != nil {
				return false, nil
			}
			if withdraw {
				return false, nil
			}
		case adjOutRecovery:
			// The admission's per-key fence ran under writeMu before any
			// section. A denied survivor withdraws this old owner, not a path
			// attributed to the denied source.
			if withdraw {
				if entry == nil {
					return false, nil
				}
				if key.addPath && forwardSourceCurrent(entry.owner.source, entry.owner.generation) {
					// A departure withdraws an ADD-PATH identifier only while
					// the departed source's ended session owns it: the
					// reconnected session's newer path, or an originated
					// one, is never withdrawn for an old DOWN.
					return false, nil
				}
				owner = entry.owner
			}
		}
	}
	if withdraw {
		if w.owesWithdrawal(key, entry) {
			w.owes[key] = adjOutOwe{family: fam, nlri: raw}
			return false, nil
		}
		w.withdrawn[key] = struct{}{}
		return true, nil
	}
	if !w.raw && !w.sourceBound() && readable && entry != nil && entry.owner.source == nil {
		_, withdrawn := w.withdrawn[key]
		_, announced := w.owners[key]
		replay, _ := w.session.sentMeta["replay"].(bool)
		if !withdrawn && !announced && !replay && sig.equal(entry.signature) && adjOutExactNLRI(entry, key, raw) {
			w.suppressed++
			return false, nil
		}
	}
	w.owners[key] = owner
	return true, nil
}

// adjOutOwe is one staged owner withdrawal: the family and the
// destination-framed NLRI, still pointing into the body being written.
type adjOutOwe struct {
	family family.Family
	nlri   []byte
}

// owesWithdrawal reports whether this admitted withdrawal is left to the
// destination's recovery worker instead of being emitted. It is, for a
// source-owned key at a destination without ADD-PATH send for the family,
// when the writer is source-bound (a forwarded owner withdrawal, received or
// synthesized, or a bgp-rib lifecycle removal) and a recovery owner is
// registered. A recovery write is the decision itself and is never owed.
// RFC 4271 Section 9.2: "All newly installed routes and all newly unfeasible
// routes for which there is no replacement route SHALL be advertised to its
// peers by means of an UPDATE message." Whether a replacement exists is the
// RIB's election, so the bare withdrawal is not this writer's to send.
func (w *adjOutWrite) owesWithdrawal(key adjOutKey, entry *adjRIBOutRoute) bool {
	if entry == nil {
		return false
	}
	if entry.owner.source == nil {
		return false
	}
	if key.addPath {
		return false
	}
	if !w.sourceBound() {
		return false
	}
	switch w.session.sentForward.authority {
	case adjOutForwarded, adjOutExpected:
	case adjOutUnspecified, adjOutRecovery, adjOutInitial:
		return false
	}
	return w.session.recoveryOwed()
}

// stageMover notes a forwarded withdrawal of a key owed before this write.
// The owe this write itself stages is applied after the last pass, so the
// withdrawal that creates a record is never its own mover.
func (w *adjOutWrite) stageMover(key adjOutKey, owner adjOutPath) {
	if w.table.owed[key] == nil {
		return
	}
	w.movers[key] = owner
}

// clearStaged drops the staged effects of a filter pass.
func (w *adjOutWrite) clearStaged() {
	clear(w.owes)
	clear(w.movers)
	clear(w.refused)
}

// applyStaged MUST run once, under the table mutex, after the last filter
// pass of a write that returned no error. An owe merges into the key's record
// when the key is already owed. A key the same body also announces is not
// owed: RFC 4271 Section 4.3 reads "A BGP speaker SHOULD treat an UPDATE
// message of this form as though the WITHDRAWN ROUTES do not contain the
// address prefix." Every owe wakes the worker. The refusals are counted once
// per write, on the destination Peer, whose counter outlives the table.
func (w *adjOutWrite) applyStaged() {
	for key, owner := range w.movers {
		if record := w.table.owed[key]; record != nil {
			record.noteMover(owner)
		}
	}
	if len(w.refused) != 0 {
		if peer := w.session.sentForward.peer; peer != nil {
			peer.owedRefused.Add(uint64(len(w.refused)))
		}
	}
	if len(w.owes) == 0 {
		return
	}
	for key, owe := range w.owes {
		if _, announced := w.owners[key]; announced {
			continue
		}
		w.table.owe(key, w.session, owe.family, owe.nlri)
	}
	w.session.wakeRecovery()
}

// adjOutExactNLRI avoids a second owned copy when the canonical key already
// contains the advertisement's exact native bytes.
func adjOutExactNLRI(entry *adjRIBOutRoute, key adjOutKey, raw []byte) bool {
	if entry.exactNLRI != nil {
		return bytes.Equal(entry.exactNLRI, raw)
	}
	if key.addPath {
		raw = raw[4:]
	}
	if key.path.overflow != "" {
		return key.path.overflow == string(raw)
	}
	return bytes.Equal(key.path.nlri[:key.path.length], raw)
}

func (w *adjOutWrite) section(dst, data []byte, fam family.Family, withdraw, multiprotocol bool, sig adjRIBOutSignature, readable bool) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	split := nlrisplit.Get(fam)
	if withdraw {
		split = nlrisplit.GetWithdraw(fam)
	}
	if split == nil {
		return 0, errAdjOutProvenance
	}
	n, occurrence := 0, 0
	var cause error
	_, err := split(data, w.encoding.AddPathFor(fam), func(raw []byte) bool {
		if cause != nil {
			return true
		}
		// RFC 7911 Section 5; final destination framing, original provenance.
		allowed, admissionErr := w.allow(fam, raw, withdraw, multiprotocol, occurrence, sig, readable)
		occurrence++
		if admissionErr != nil {
			cause = admissionErr
			return true
		}
		if !allowed {
			w.removed++
			return true
		}
		if dst != nil {
			copy(dst[n:], raw)
		}
		n += len(raw)
		return true
	})
	if err != nil {
		return 0, err
	}
	return n, cause
}

// filter first runs without a destination; unchanged bodies need no copy. A
// second pass materializes only a changed body into caller-owned pooled storage.
// RFC 4760 Section 4: "An UPDATE message that contains the MP_UNREACH_NLRI is
// not required to carry any other path attributes".
func (w *adjOutWrite) filter(dst, body []byte) (int, error) {
	sec, err := wire.ParseUpdateSections(body)
	if err != nil {
		return 0, err
	}
	attrs := sec.Attrs(body)
	n, err := w.section(adjOutSlice(dst, 2), sec.Withdrawn(body), family.IPv4Unicast, true, false, adjRIBOutSignature{}, false)
	if err != nil {
		return 0, err
	}
	if dst != nil {
		binary.BigEndian.PutUint16(dst, uint16(n))
	}
	n += 2
	attrOffset := n
	n += 2
	// Populate withdrawal admission before announcements, irrespective of the
	// ordering of MP attributes. The second traversal writes their wire form.
	iter := attribute.NewAttrIterator(attrs)
	for code, _, value, ok := iter.Next(); ok; code, _, value, ok = iter.Next() {
		if code != attribute.AttrMPUnreachNLRI {
			continue
		}
		fam, off, cause := pathsLimitMP(value, false)
		if cause != nil {
			return 0, cause
		}
		if _, cause = w.section(nil, value[off:], fam, true, true, adjRIBOutSignature{}, false); cause != nil {
			return 0, cause
		}
	}
	if iter.Remaining() != 0 {
		return 0, wire.ErrUpdateTruncated
	}
	// Stage legacy announcements before MP_REACH, matching the sent RIB's
	// native replacement order. Reserve their original tail position until
	// attribute filtering determines the final offset; filtering never grows it.
	var sig adjRIBOutSignature
	var readable bool
	if !w.sourceBound() {
		sig, readable = announceSignature(attrs, family.IPv4Unicast, len(sec.NLRI(body)))
	}
	legacyOffset := len(body) - len(sec.NLRI(body))
	count, err := w.section(adjOutSlice(dst, legacyOffset), sec.NLRI(body), family.IPv4Unicast, false, false, sig, readable)
	if err != nil {
		return 0, err
	}
	iter.Reset()
	start := 0
	paths := n - 4
	for code, flags, value, ok := iter.Next(); ok; code, flags, value, ok = iter.Next() {
		end := iter.Offset()
		if code != attribute.AttrMPReachNLRI && code != attribute.AttrMPUnreachNLRI {
			if dst != nil {
				copy(dst[n:], attrs[start:end])
			}
			n += end - start
			start = end
			continue
		}
		reach := code == attribute.AttrMPReachNLRI
		fam, off, cause := pathsLimitMP(value, reach)
		if cause != nil {
			return 0, cause
		}
		var sig adjRIBOutSignature
		var readable bool
		if reach && !w.sourceBound() {
			sig, readable = announceSignature(attrs, fam, len(value)-off)
		}
		header := end - start - len(value)
		outStart := n
		if dst != nil {
			copy(dst[n:], attrs[start:start+header+off])
		}
		n += header + off
		removed := w.removed
		count, cause := w.section(adjOutSlice(dst, n), value[off:], fam, !reach, true, sig, readable)
		if cause != nil {
			return 0, cause
		}
		if count == 0 && w.removed != removed {
			n = outStart
			start = end
			continue
		}
		paths += count
		n += count
		if dst != nil {
			if flags&attribute.FlagExtLength == 0 {
				dst[outStart+2] = byte(off + count)
			} else {
				binary.BigEndian.PutUint16(dst[outStart+2:], uint16(off+count))
			}
		}
		start = end
	}
	if dst != nil {
		binary.BigEndian.PutUint16(dst[attrOffset:], uint16(n-attrOffset-2))
		if n != legacyOffset {
			copy(dst[n:], dst[legacyOffset:legacyOffset+count])
		}
	}
	if w.removed != 0 && paths+count == 0 {
		return 0, nil
	}
	return n + count, nil
}

func adjOutSlice(dst []byte, off int) []byte {
	if dst == nil {
		return nil
	}
	return dst[off:]
}

// record applies withdrawals before announcements from only the final body.
// RFC 4271 Section 9: "If the UPDATE message contains a feasible route, the
// Adj-RIB-In will be updated with this route as follows: if the NLRI of the
// new route is identical to the one the route currently has stored in the
// Adj-RIB-In, then the new route SHALL replace the older route in the
// Adj-RIB-In, thus implicitly withdrawing the older route from service".
func (w *adjOutWrite) record(body []byte) error {
	sec, err := wire.ParseUpdateSections(body)
	if err != nil {
		return err
	}
	attrs := sec.Attrs(body)
	if err := w.recordSection(sec.Withdrawn(body), attrs, family.IPv4Unicast, true); err != nil {
		return err
	}
	for _, reach := range []bool{false, true} {
		if reach {
			if err := w.recordSection(sec.NLRI(body), attrs, family.IPv4Unicast, false); err != nil {
				return err
			}
		}
		iter := attribute.NewAttrIterator(attrs)
		for code, _, value, ok := iter.Next(); ok; code, _, value, ok = iter.Next() {
			wanted := attribute.AttrMPUnreachNLRI
			if reach {
				wanted = attribute.AttrMPReachNLRI
			}
			if code != wanted {
				continue
			}
			fam, off, cause := pathsLimitMP(value, reach)
			if cause != nil {
				return cause
			}
			if cause := w.recordSection(value[off:], attrs, fam, !reach); cause != nil {
				return cause
			}
		}
	}
	return nil
}

func (w *adjOutWrite) recordSection(data, attrs []byte, fam family.Family, withdraw bool) error {
	if len(data) == 0 {
		return nil
	}
	split := nlrisplit.Get(fam)
	if withdraw {
		split = nlrisplit.GetWithdraw(fam)
	}
	if split == nil {
		return errAdjOutProvenance
	}
	var sig adjRIBOutSignature
	var readable, signatureRead bool
	var stored []byte
	var cause error
	_, err := split(data, w.encoding.AddPathFor(fam), func(raw []byte) bool {
		if cause != nil {
			return true
		}
		key, keyErr := w.key(fam, raw, withdraw)
		if keyErr != nil {
			cause = keyErr
			return true
		}
		if withdraw {
			delete(w.table.routes, key)
			w.withdrawnRoutes++
			return true
		}
		owner, ok := w.owners[key]
		if !ok {
			cause = errAdjOutProvenance
			return true
		}
		if w.sourceSet && w.source != owner.source {
			cause = errAdjOutProvenance
			return true
		}
		w.source, w.sourceSet = owner.source, true
		if w.session.onMessageReceived != nil {
			ordinal := w.ordinals[fam]
			if owner.addPath {
				if w.pathSources == nil {
					w.pathSources = make([]wireu.SentPathSource, 0, len(w.owners))
				}
				w.pathSources = append(w.pathSources, wireu.SentPathSource{
					Family: fam, Ordinal: ordinal, PathID: owner.received,
				})
			}
			w.ordinals[fam] = ordinal + 1
		}
		if w.table.routes == nil {
			w.table.routes = make(map[adjOutKey]*adjRIBOutRoute)
		}
		entry := w.table.routes[key]
		if entry == nil {
			entry = new(adjRIBOutRoute)
			w.table.routes[key] = entry
		}
		entry.owner = owner
		if !w.sourceBound() {
			// An originated route is not a candidate of the election and is
			// not overridden by it: it decides an owed key itself.
			w.table.removeOwed(key, w.table.owed[key])
		}
		if w.session.sentForward == nil || w.session.sentForward.authority != adjOutExpected {
			replay, _ := w.session.sentMeta["replay"].(bool)
			if !replay || entry.revision == 0 {
				entry.revision = w.session.sentReceipt.MessageID()
			}
		}
		if item := w.session.sentForward; item != nil && item.authority == adjOutInitial {
			entry.revision = item.expectedMessage
		}
		entry.signature, entry.exactNLRI = nil, nil
		if owner.source == nil && !signatureRead {
			sig, readable = announceSignature(attrs, fam, len(data))
			signatureRead = true
		}
		if owner.source == nil && readable {
			if stored == nil {
				stored = sig.store()
			}
			entry.signature = stored
			if !adjOutExactNLRI(entry, key, raw) {
				entry.exactNLRI = append([]byte(nil), raw...)
			}
		}
		w.announcedRoutes++
		return true
	})
	if err != nil {
		return err
	}
	w.table.pending = true
	return cause
}

// checkInitialWrite runs under writeMu, including before originated policy can
// suppress a replay. Expired authority must remain an error, not become an
// apparently successful policy no-op.
func (s *Session) checkInitialWrite(item *fwdItem) error {
	if item.initialReplay == 0 {
		return errAdjOutProvenance
	}
	if item.session != s {
		return errAdjOutSession
	}
	if item.initialReplay != s.initialReplay {
		return errAdjOutSession
	}
	if item.peer == nil {
		return errAdjOutProvenance
	}
	if !item.peer.initialSyncEOROwed.Load() && !item.peer.initialUpdateOwed.Load() {
		return errAdjOutSession
	}
	if !item.initialUpdate {
		return errAdjOutProvenance
	}
	if item.expectedMessage == 0 {
		return errAdjOutProvenance
	}
	if item.initialLocal == (item.receivedPeer != nil) {
		return errAdjOutProvenance
	}
	if item.receivedPeer != nil && !initialSourceCurrent(item.receivedPeer, item.receivedGeneration, item.initialSourceState) {
		return errAdjOutProvenance
	}
	return nil
}
