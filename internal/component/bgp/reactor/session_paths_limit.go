// Design: docs/guide/add-path.md -- negotiated per-prefix sender limits
// RFC: rfc/drafts/draft-abraitis-idr-addpath-paths-limit.txt -- Section 3
// Related: session_write.go -- all normal UPDATE writers share this admission point
package reactor

import (
	"context"
	"encoding/binary"
	"errors"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

var errPathsLimitNLRI = errors.New("PATHS-LIMIT: cannot identify NLRI")

// pathsLimitAdmission is the per-direction admission state of one connection:
// the send bound under the peer's limit and the receive ceiling under Ze's own
// share it, so the section filter, the body rewrite and the rollback journal
// are declared once. A nil admission admits everything. The owner serializes
// every access: Session.writeMu for send, the session read loop for receive.
type pathsLimitAdmission map[family.Family]*pathsLimitFamily

// pathsLimitFamily tracks only paths admitted on this connection. Its bound is
// the limit times the number of distinct admitted prefixes. Withdrawals remove
// entries; a new Session starts empty. Suppressed paths are not a second route
// queue: a later announcement can retry.
type pathsLimitFamily struct {
	family        family.Family
	limit         uint16
	split         nlrisplit.Splitter
	withdrawSplit nlrisplit.Splitter
	key           nlrisplit.PrefixKeyFunc
	keyScratch    [nlrisplit.PrefixKeyScratchSize]byte
	prefixes      map[string]*pathsLimitPrefix
	// auditRefusals is set on the receive side only: a refused identifier is
	// then journaled, so it can be counted, warned about once and tracked.
	auditRefusals bool
}

// pathsLimitPrefix holds the admitted identifiers of one route, and on the
// receive side up to pathsLimitRefusedHeadroom refused ones. A non-empty
// refused set is the warned marker: the first refusal into an empty set warns.
type pathsLimitPrefix struct {
	key            string
	ids            map[uint32]struct{}
	refused        [pathsLimitRefusedHeadroom]uint32
	refusedEntries uint8
}

// pathsLimitChangeKind names what one journal entry did, so rollback can undo
// it and the receive side can report it. The zero value is never journaled.
type pathsLimitChangeKind uint8

const (
	pathsLimitChangeUnspecified pathsLimitChangeKind = iota
	// pathsLimitAdmitted counted a new identifier against the limit.
	pathsLimitAdmitted
	// pathsLimitAdmittedRefused counted an identifier that was tracked as
	// refused, and released its tracking slot in the same step.
	pathsLimitAdmittedRefused
	// pathsLimitReleased removed an admitted identifier on withdrawal.
	pathsLimitReleased
	// pathsLimitRefusalTracked refused an identifier and tracked it.
	pathsLimitRefusalTracked
	// pathsLimitRefusalCounted refused an identifier already tracked, or one
	// the full tracking set had no slot for. It changes no state.
	pathsLimitRefusalCounted
	// pathsLimitRefusalReleased removed a tracked refused identifier on
	// withdrawal, which frees its slot and clears the warned marker.
	pathsLimitRefusalReleased
)

// Changes are undone if the UPDATE cannot be written or delivered. The input
// body owns the wire bytes, but these entries reference owned prefix keys,
// not pooled input.
type pathsLimitChange struct {
	family *pathsLimitFamily
	prefix *pathsLimitPrefix
	id     uint32
	kind   pathsLimitChangeKind
	// warn marks the refusal that made an empty refused set non-empty.
	warn bool
}

// Even the smallest ADD-PATH NLRI needs five octets, and one NLRI journals at
// most one entry. One extended UPDATE cannot change more entries than this.
// The scratch belongs to one message, not a session.
const pathsLimitChangesMax = message.ExtMsgLen / 5

type pathsLimitChanges struct {
	entries [pathsLimitChangesMax]pathsLimitChange
	n       int
	// refusals counts the refusal entries, so a message that refused nothing
	// is reported without walking the journal.
	refusals int
}

var pathsLimitScratch = sync.Pool{New: func() any { return new(pathsLimitChanges) }}

// journal records one change. A full journal is an error the caller returns,
// never a change applied without its undo.
func (c *pathsLimitChanges) journal(change pathsLimitChange) bool {
	if c.n == len(c.entries) {
		return false
	}
	c.entries[c.n] = change
	c.n++
	if change.kind == pathsLimitRefusalTracked {
		c.refusals++
	}
	if change.kind == pathsLimitRefusalCounted {
		c.refusals++
	}
	return true
}

func (c *pathsLimitChanges) rollback() {
	for i := c.n - 1; i >= 0; i-- {
		change := &c.entries[i]
		switch change.kind {
		case pathsLimitAdmitted, pathsLimitAdmittedRefused:
			delete(change.prefix.ids, change.id)
			if change.kind == pathsLimitAdmittedRefused {
				change.prefix.trackRefused(change.id)
			}
			if len(change.prefix.ids) == 0 {
				delete(change.family.prefixes, change.prefix.key)
			}
		case pathsLimitReleased:
			change.family.prefixes[change.prefix.key] = change.prefix
			change.prefix.ids[change.id] = struct{}{}
		case pathsLimitRefusalTracked:
			change.prefix.releaseRefused(change.id)
		case pathsLimitRefusalCounted:
			// Counting changed no state.
		case pathsLimitRefusalReleased:
			change.prefix.trackRefused(change.id)
		case pathsLimitChangeUnspecified:
			panic("BUG: PATHS-LIMIT journal holds an unspecified change")
		}
	}
}

func (c *pathsLimitChanges) release() {
	clear(c.entries[:c.n])
	c.n = 0
	c.refusals = 0
	pathsLimitScratch.Put(c)
}

// newPathsLimitAdmission builds the admission for one direction from the
// negotiated limits. A limit applies only where ADD-PATH is negotiated in that
// direction; nil means no family is limited, and nothing is filtered.
func newPathsLimitAdmission(limits map[family.Family]uint16, modes map[family.Family]capability.AddPathMode, direction capability.AddPathMode) pathsLimitAdmission {
	var admission pathsLimitAdmission
	for fam, limit := range limits {
		if limit == 0 {
			continue
		}
		if modes[fam]&direction == 0 {
			continue
		}
		if admission == nil {
			admission = make(pathsLimitAdmission)
		}
		admission[fam] = &pathsLimitFamily{
			family:        fam,
			limit:         limit,
			split:         nlrisplit.Get(fam),
			withdrawSplit: nlrisplit.GetWithdraw(fam),
			key:           nlrisplit.GetPrefixKey(fam),
			prefixes:      make(map[string]*pathsLimitPrefix),
			auditRefusals: direction == capability.AddPathReceive,
		}
	}
	return admission
}

// initPathsLimit runs once during OPEN negotiation, with writeMu held. Only the
// receiver's limit in a negotiated ADD-PATH send direction constrains this side.
func (s *Session) initPathsLimit(encoding *capability.EncodingCaps) {
	s.pathsLimit = nil
	s.updateSendTotals = updateSendCounts{}
	if encoding == nil {
		return
	}
	s.pathsLimit = newPathsLimitAdmission(encoding.PathsLimitSend, encoding.AddPathMode, capability.AddPathSend)
}

// section copies accepted NLRIs into dst. Withdrawals always pass, including
// unknown identifiers; only a NEW path consumes a slot. Replacing attributes on
// an existing identifier therefore works even at the limit.
//
// draft-abraitis-idr-addpath-paths-limit Section 3: "A sender advertising
// multiple paths for the same prefix SHOULD send only the specified maximum
// number of paths indicated in the PATHS-LIMIT capability."
// draft-abraitis-idr-addpath-paths-limit Section 3: "An implementation SHOULD
// provide a configuration knob to specify the maximum number of paths to
// accept from a sender."
func (a pathsLimitAdmission) section(dst, data []byte, fam family.Family, withdraw bool, changes *pathsLimitChanges) (int, int, error) {
	state := a[fam]
	if state == nil || len(data) == 0 {
		return copy(dst, data), 0, nil
	}
	split := state.split
	if withdraw {
		split = state.withdrawSplit
	}
	if split == nil {
		return 0, 0, errPathsLimitNLRI
	}
	n, dropped := 0, 0
	var keyErr error
	_, err := split(data, true, func(raw []byte) bool {
		if keyErr != nil {
			return true
		}
		if len(raw) < 5 {
			keyErr = errPathsLimitNLRI
			return true
		}
		key, cause := state.key(raw[4:], state.keyScratch[:], withdraw)
		if cause != nil {
			keyErr = cause
			return true
		}
		id := binary.BigEndian.Uint32(raw)
		pass, journaled := state.admitPath(state.prefixes[string(key)], key, id, withdraw, changes)
		if !journaled {
			keyErr = errPathsLimitNLRI
			return true
		}
		if !pass {
			dropped++
			return true
		}
		if dst != nil {
			n += copy(dst[n:], raw)
		}
		return true
	})
	if err != nil {
		return 0, 0, err
	}
	return n, dropped, keyErr
}

// admitPath applies one NLRI to the state and journals the change. pass says
// whether the NLRI stays in the message: every withdrawal does, and an
// announcement does when its identifier is held or a slot is free. journaled
// is false only when the journal is full, before any state changed.
func (f *pathsLimitFamily) admitPath(prefix *pathsLimitPrefix, key []byte, id uint32, withdraw bool, changes *pathsLimitChanges) (pass, journaled bool) {
	held := false
	if prefix != nil {
		_, held = prefix.ids[id]
	}
	if withdraw {
		if held {
			if !changes.journal(pathsLimitChange{family: f, prefix: prefix, id: id, kind: pathsLimitReleased}) {
				return true, false
			}
			delete(prefix.ids, id)
			if len(prefix.ids) == 0 {
				delete(f.prefixes, prefix.key)
			}
			return true, true
		}
		// A withdrawal of an identifier never admitted frees no admitted slot.
		if prefix == nil {
			return true, true
		}
		if !prefix.refusedHolds(id) {
			return true, true
		}
		if !changes.journal(pathsLimitChange{family: f, prefix: prefix, id: id, kind: pathsLimitRefusalReleased}) {
			return true, false
		}
		prefix.releaseRefused(id)
		return true, true
	}
	if held {
		return true, true
	}
	if prefix != nil && len(prefix.ids) >= int(f.limit) {
		return false, f.refusePath(prefix, id, changes)
	}
	kind := pathsLimitAdmitted
	if prefix != nil && prefix.refusedHolds(id) {
		kind = pathsLimitAdmittedRefused
	}
	if prefix == nil {
		prefix = &pathsLimitPrefix{key: string(key), ids: make(map[uint32]struct{})}
	}
	if !changes.journal(pathsLimitChange{family: f, prefix: prefix, id: id, kind: kind}) {
		return false, false
	}
	f.prefixes[prefix.key] = prefix
	prefix.ids[id] = struct{}{}
	if kind == pathsLimitAdmittedRefused {
		prefix.releaseRefused(id)
	}
	return true, true
}

// refusePath records one identifier refused at the limit. The send side
// journals nothing: its writer reports only the dropped count.
func (f *pathsLimitFamily) refusePath(prefix *pathsLimitPrefix, id uint32, changes *pathsLimitChanges) bool {
	if !f.auditRefusals {
		return true
	}
	counted := pathsLimitChange{family: f, prefix: prefix, id: id, kind: pathsLimitRefusalCounted}
	if prefix.refusedHolds(id) {
		return changes.journal(counted)
	}
	if int(prefix.refusedEntries) == len(prefix.refused) {
		return changes.journal(counted)
	}
	warn := prefix.refusedEntries == 0
	if !changes.journal(pathsLimitChange{family: f, prefix: prefix, id: id, kind: pathsLimitRefusalTracked, warn: warn}) {
		return false
	}
	prefix.trackRefused(id)
	return true
}

func (p *pathsLimitPrefix) refusedHolds(id uint32) bool {
	return slices.Contains(p.refused[:p.refusedEntries], id)
}

// trackRefused MUST be called only when the set has a free slot and does not
// hold id; refusePath and rollback are its only callers and establish both.
func (p *pathsLimitPrefix) trackRefused(id uint32) {
	p.refused[p.refusedEntries] = id
	p.refusedEntries++
}

func (p *pathsLimitPrefix) releaseRefused(id uint32) {
	for i, refused := range p.refused[:p.refusedEntries] {
		if refused != id {
			continue
		}
		last := int(p.refusedEntries) - 1
		p.refused[i] = p.refused[last]
		p.refusedEntries--
		return
	}
}

// pathsLimitMP reads the NLRI offset without decoding attributes or routes.
func pathsLimitMP(value []byte, reach bool) (family.Family, int, error) {
	if len(value) < 3 {
		return family.Family{}, 0, errPathsLimitNLRI
	}
	fam := family.Family{AFI: family.AFI(binary.BigEndian.Uint16(value)), SAFI: family.SAFI(value[2])}
	if !reach {
		return fam, 3, nil
	}
	if len(value) < 5 {
		return fam, 0, errPathsLimitNLRI
	}
	off := 5 + int(value[3])
	if off > len(value) {
		return fam, 0, errPathsLimitNLRI
	}
	return fam, off, nil
}

// filter rewrites into a distinct pooled buffer. This is needed even for
// same-context forwarding: mutating the shared input would drop paths for other
// peers. Lengths and callbacks describe the filtered message, never the offered
// batch. A zero length means all announcements were withheld, not EOR. Only
// whole NLRIs are removed and lengths fixed; attributes are untouched.
func (a pathsLimitAdmission) filter(dst, body []byte, changes *pathsLimitChanges) (int, int, error) {
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		return 0, 0, err
	}
	if len(dst) < len(body) {
		return 0, 0, wire.ErrUpdateTruncated
	}
	withdrawn := sections.Withdrawn(body)
	attrs := sections.Attrs(body)
	n, _, err := a.section(dst[2:], withdrawn, family.IPv4Unicast, true, changes)
	if err != nil {
		return 0, 0, err
	}
	binary.BigEndian.PutUint16(dst, uint16(n))
	n += 2
	attrLengthOffset := n
	n += 2
	withdrawSignal := len(withdrawn) != 0

	// Apply every withdrawal before admitting any announcement. This also handles
	// a mixed legacy/MP UPDATE without making attribute order decide capacity.
	iter := attribute.NewAttrIterator(attrs)
	for code, _, value, ok := iter.Next(); ok; code, _, value, ok = iter.Next() {
		if code != attribute.AttrMPUnreachNLRI {
			continue
		}
		fam, off, cause := pathsLimitMP(value, false)
		if cause != nil {
			return 0, 0, cause
		}
		withdrawSignal = true
		if _, _, cause = a.section(nil, value[off:], fam, true, changes); cause != nil {
			return 0, 0, cause
		}
	}
	if iter.Remaining() != 0 {
		return 0, 0, wire.ErrUpdateTruncated
	}

	dropped := 0
	reachable := false
	iter.Reset()
	start := 0
	for code, flags, value, ok := iter.Next(); ok; code, flags, value, ok = iter.Next() {
		end := iter.Offset()
		if code != attribute.AttrMPReachNLRI {
			n += copy(dst[n:], attrs[start:end])
			start = end
			continue
		}
		fam, off, cause := pathsLimitMP(value, true)
		if cause != nil {
			return 0, 0, cause
		}
		headerLen := end - start - len(value)
		outStart := n
		n += copy(dst[n:], attrs[start:start+headerLen+off])
		count, removed, cause := a.section(dst[n:], value[off:], fam, false, changes)
		if cause != nil {
			return 0, 0, cause
		}
		dropped += removed
		if count == 0 && removed != 0 {
			n = outStart
			start = end
			continue
		}
		n += count
		reachable = reachable || count != 0
		length := off + count
		if flags&attribute.FlagExtLength == 0 {
			dst[outStart+2] = byte(length)
		} else {
			binary.BigEndian.PutUint16(dst[outStart+2:], uint16(length))
		}
		start = end
	}
	binary.BigEndian.PutUint16(dst[attrLengthOffset:], uint16(n-attrLengthOffset-2))
	count, removed, err := a.section(dst[n:], sections.NLRI(body), family.IPv4Unicast, false, changes)
	if err != nil {
		return 0, 0, err
	}
	n += count
	dropped += removed
	reachable = reachable || count != 0
	if dropped != 0 && !reachable && !withdrawSignal {
		return 0, dropped, nil
	}
	return n, dropped, nil
}

// These totals count the final writer's accepted messages and route directions.
// A commit retains its serialized write's delta even beside a later refusal.
type updateSendCounts struct {
	announcedRoutes uint64
	withdrawnRoutes uint64
	updates         uint64
}

type commitUpdateSender struct {
	peer   *Peer
	counts updateSendCounts
}

func (c *commitUpdateSender) SendUpdate(update *message.Update) error {
	c.peer.mu.RLock()
	session := c.peer.session
	c.peer.mu.RUnlock()
	if session == nil {
		return ErrNotConnected
	}
	var counts updateSendCounts
	err := session.sendUpdateCounted(context.Background(), update, &counts, false, nil)
	c.counts.announcedRoutes += counts.announcedRoutes
	c.counts.withdrawnRoutes += counts.withdrawnRoutes
	c.counts.updates += counts.updates
	return err
}
