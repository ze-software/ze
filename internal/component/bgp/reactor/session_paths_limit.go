// Design: docs/guide/add-path.md -- negotiated per-prefix sender limits
// RFC: rfc/drafts/draft-abraitis-idr-addpath-paths-limit.txt -- Section 3
// Related: session_write.go -- all normal UPDATE writers share this admission point
package reactor

import (
	"encoding/binary"
	"errors"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

var errPathsLimitNLRI = errors.New("PATHS-LIMIT: cannot identify outbound NLRI")

// pathsLimitFamily tracks only paths admitted on this connection. Its bound is
// the peer's limit times the number of distinct advertised prefixes. Withdrawals
// remove entries; a new Session starts empty. Session.writeMu owns every access.
// Suppressed paths are not a second route queue: a later announcement can retry.
type pathsLimitFamily struct {
	limit         uint16
	split         nlrisplit.Splitter
	withdrawSplit nlrisplit.Splitter
	key           nlrisplit.PrefixKeyFunc
	keyScratch    [nlrisplit.PrefixKeyScratchSize]byte
	prefixes      map[string]*pathsLimitPrefix
}

type pathsLimitPrefix struct {
	key string
	ids map[uint32]struct{}
}

// Changes are undone if the UPDATE cannot be written. The input body owns the
// wire bytes, but these entries reference owned prefix keys, not pooled input.
type pathsLimitChange struct {
	family *pathsLimitFamily
	prefix *pathsLimitPrefix
	id     uint32
	added  bool
}

// Even the smallest ADD-PATH NLRI needs five octets. One extended UPDATE cannot
// change more entries than this. The scratch belongs to a write, not a session.
const pathsLimitChangesMax = message.ExtMsgLen / 5

type pathsLimitChanges struct {
	entries [pathsLimitChangesMax]pathsLimitChange
	n       int
}

var pathsLimitScratch = sync.Pool{New: func() any { return new(pathsLimitChanges) }}

func (c *pathsLimitChanges) rollback() {
	for i := c.n - 1; i >= 0; i-- {
		change := &c.entries[i]
		if change.added {
			delete(change.prefix.ids, change.id)
			if len(change.prefix.ids) == 0 {
				delete(change.family.prefixes, change.prefix.key)
			}
			continue
		}
		change.family.prefixes[change.prefix.key] = change.prefix
		change.prefix.ids[change.id] = struct{}{}
	}
}

func (c *pathsLimitChanges) release() {
	clear(c.entries[:c.n])
	c.n = 0
	pathsLimitScratch.Put(c)
}

// initPathsLimit runs once during OPEN negotiation, with writeMu held. Only the
// receiver's limit in a negotiated ADD-PATH send direction constrains this side.
func (s *Session) initPathsLimit(encoding *capability.EncodingCaps) {
	s.pathsLimit = nil
	s.pathsLimitTotals = pathsLimitSendCounts{}
	if encoding == nil {
		return
	}
	for fam, limit := range encoding.PathsLimitSend {
		if limit == 0 || encoding.AddPathMode[fam]&capability.AddPathSend == 0 {
			continue
		}
		if s.pathsLimit == nil {
			s.pathsLimit = make(map[family.Family]*pathsLimitFamily)
		}
		s.pathsLimit[fam] = &pathsLimitFamily{
			limit:         limit,
			split:         nlrisplit.Get(fam),
			withdrawSplit: nlrisplit.GetWithdraw(fam),
			key:           nlrisplit.GetPrefixKey(fam),
			prefixes:      make(map[string]*pathsLimitPrefix),
		}
	}
}

// pathsLimitSection copies accepted NLRIs into dst. Withdrawals always pass,
// including unknown identifiers; only a NEW path consumes a slot. Replacing
// attributes on an existing identifier therefore works even at the limit.
func (s *Session) pathsLimitSection(dst, data []byte, fam family.Family, withdraw bool, changes *pathsLimitChanges) (int, int, error) {
	state := s.pathsLimit[fam]
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
	_, err := split(data, true, func(raw []byte) {
		if keyErr != nil {
			return
		}
		if len(raw) < 5 {
			keyErr = errPathsLimitNLRI
			return
		}
		key, cause := state.key(raw[4:], state.keyScratch[:], withdraw)
		if cause != nil {
			keyErr = cause
			return
		}
		id := binary.BigEndian.Uint32(raw)
		prefix := state.prefixes[string(key)]
		held := false
		if prefix != nil {
			_, held = prefix.ids[id]
		}
		if !withdraw && !held && prefix != nil && len(prefix.ids) >= int(state.limit) {
			dropped++
			return
		}
		if withdraw == held {
			if changes.n == len(changes.entries) {
				keyErr = errPathsLimitNLRI
				return
			}
			if prefix == nil {
				prefix = &pathsLimitPrefix{key: string(key), ids: make(map[uint32]struct{})}
				state.prefixes[prefix.key] = prefix
			}
			changes.entries[changes.n] = pathsLimitChange{family: state, prefix: prefix, id: id, added: !withdraw}
			changes.n++
			if withdraw {
				delete(prefix.ids, id)
				if len(prefix.ids) == 0 {
					delete(state.prefixes, prefix.key)
				}
			} else {
				prefix.ids[id] = struct{}{}
			}
		}
		if dst != nil {
			n += copy(dst[n:], raw)
		}
	})
	if err != nil {
		return 0, 0, err
	}
	return n, dropped, keyErr
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

// filterPathsLimit rewrites into a distinct pooled buffer. This is needed even
// for same-context forwarding: mutating the shared input would drop paths for
// other peers. Lengths and callbacks describe the filtered message, never the
// offered batch. A zero length means all announcements were withheld, not EOR.
func (s *Session) filterPathsLimit(dst, body []byte, changes *pathsLimitChanges) (int, int, error) {
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		return 0, 0, err
	}
	if len(dst) < len(body) {
		return 0, 0, wire.ErrUpdateTruncated
	}
	withdrawn := sections.Withdrawn(body)
	attrs := sections.Attrs(body)
	n, _, err := s.pathsLimitSection(dst[2:], withdrawn, family.IPv4Unicast, true, changes)
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
		if _, _, cause = s.pathsLimitSection(nil, value[off:], fam, true, changes); cause != nil {
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
		count, removed, cause := s.pathsLimitSection(dst[n:], value[off:], fam, false, changes)
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
		if flags&attribute.FlagExtLength != 0 {
			binary.BigEndian.PutUint16(dst[outStart+2:], uint16(length))
		} else {
			dst[outStart+2] = byte(length)
		}
		start = end
	}
	binary.BigEndian.PutUint16(dst[attrLengthOffset:], uint16(n-attrLengthOffset-2))
	count, removed, err := s.pathsLimitSection(dst[n:], sections.NLRI(body), family.IPv4Unicast, false, changes)
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

// recordAnnounced keeps the API duplicate cache from remembering a path the
// session withheld. Otherwise an identical retry would never reach admission
// after a withdrawal frees its slot.
func (p *Peer) recordAnnounced(fam family.Family, key, signature []byte) {
	ctx := p.sendCtx.Load()
	if ctx == nil || ctx.PathsLimit(fam) == 0 {
		p.adjOut.record(fam, key, signature)
		return
	}
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()
	if session == nil {
		return
	}
	session.writeMu.Lock()
	defer session.writeMu.Unlock()
	state := session.pathsLimit[fam]
	if state == nil || len(key) < 5 {
		return
	}
	prefixKey, err := state.key(key[4:], state.keyScratch[:], false)
	if err != nil {
		return
	}
	prefix := state.prefixes[string(prefixKey)]
	if prefix == nil {
		return
	}
	if _, held := prefix.ids[binary.BigEndian.Uint32(key)]; held {
		p.adjOut.record(fam, key, signature)
	}
}

// These totals count only PATHS-LIMIT decisions. A commit subtracts its own
// serialized write's delta, not changes made by concurrent plugin writers.
type pathsLimitSendCounts struct {
	routes  uint64
	updates uint64
}

type pathsLimitCommitSender struct {
	peer     *Peer
	withheld pathsLimitSendCounts
}

func (c *pathsLimitCommitSender) SendUpdate(update *message.Update) error {
	c.peer.mu.RLock()
	session := c.peer.session
	c.peer.mu.RUnlock()
	if session == nil {
		return ErrNotConnected
	}
	var counts pathsLimitSendCounts
	if err := session.sendUpdateCounted(update, &counts); err != nil {
		return err
	}
	c.withheld.routes += counts.routes
	c.withheld.updates += counts.updates
	return nil
}
