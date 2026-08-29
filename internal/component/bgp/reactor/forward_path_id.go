// Design: docs/architecture/bgp/structural-forwarding.md -- one egress transform, both rails
// RFC: rfc/short/rfc7911.md — Section 2, a re-advertised route carries the speaker's own Path Identifier
// Related: forward_body.go -- buildFwdBody (raw rail) and fwdReencodeNLRIs (re-encode rail)
package reactor

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/source"
)

// fwdPathIDs holds ze's own RFC 7911 Path Identifier for every path it
// re-advertises. Both forward rails read it, so a replayed route and a live
// forward of one path carry the same identifier.
//
// RFC 7911 Section 2: "A BGP speaker that re-advertises a route MUST generate
// its own Path Identifier to be associated with the re-advertised route", and
// "the Path Identifier MUST be assigned in such a way that the BGP speaker is
// able to use the (Prefix, Path Identifier) to uniquely identify a path
// advertised to a neighbor". Relaying the identifier the source chose satisfies
// neither. Each route-server client picks its identifiers alone, so two clients
// that both pick 1 for one prefix reach a third client as one
// (prefix, identifier) pair, and RFC 7911 Section 5 makes the receiver treat the
// second as a replacement for the first. One path is lost.
//
// The key is the path at INGRESS: the source that sent it, and the identifier
// that source used for it. It is never the message and never the attributes.
//
// A withdraw is what makes that key the only workable one. A withdrawn route
// carries a prefix and a Path Identifier and no path attributes at all, so an
// identifier derived from attribute bytes could not be recomputed when the path
// leaves, and the route would stay in the destination's table forever. The same
// key is also what makes a re-advertisement replace rather than duplicate: a
// source that re-announces one path with changed attributes is replacing it
// (RFC 7911 Section 5), and it must leave ze under the identifier it already
// has.
//
// How much of the path the key holds depends on what the SOURCE framed, because
// ze mirrors the key that source uses to name a path. A source that negotiated
// no ADD-PATH names a path by its prefix alone and sends every one of them under
// received identifier 0, so ze holds ONE identifier for the whole session and
// gives every prefix of that source the same one: (prefix, identifier) still
// names one path at the destination. A source that negotiated ADD-PATH names a
// path by (prefix, identifier), so ze holds one entry for each such pair and
// frees it when it has relayed that pair's withdraw.
var fwdPathIDs = newFwdPathIDTable()

// fwdPathNLRIMax bounds the NLRI octets one path key holds: the prefix-length
// octet, then the 32 octets nlri.PrefixBytes returns for the largest length that
// octet can state.
const fwdPathNLRIMax = 1 + 32

// fwdPathKey names one path a source that frames Path Identifiers advertised:
// the family it arrived in, the identifier the source chose for it, and the NLRI
// bytes it chose them for. The octets of nlri past the ones the source sent stay
// zero, so two NLRIs of different lengths never compare equal.
//
// The family is part of the key because NLRI bytes alone are ambiguous across
// families: 10.0.0.0/24 and 0a00::/24 carry the same length octet and the same
// three significant octets.
type fwdPathKey struct {
	family   family.Family
	received uint32
	nlri     [fwdPathNLRIMax]byte
}

// fwdPathKeyFor writes the key of one ingress path into out. raw is the NLRI as
// the source framed it, prefix-length octet included.
func fwdPathKeyFor(out *fwdPathKey, fam family.Family, received uint32, raw []byte) error {
	if len(raw) > len(out.nlri) {
		return fmt.Errorf("nlri of %d octets exceeds the %d a path key holds", len(raw), len(out.nlri))
	}
	*out = fwdPathKey{family: fam, received: received}
	copy(out.nlri[:], raw)
	return nil
}

// fwdPathIDTable maps ingress paths to the identifiers ze advertises for them.
//
// bySource holds the sources that frame NO Path Identifier. Their every path
// arrives under received identifier 0, which is a value rather than an absence:
// RFC 7911 Section 3 makes 0 legal. One entry serves such a source for its whole
// session, and nothing but peer removal ends it.
//
// byPath holds the sources that DO frame one, keyed by the path itself. Keying
// them on (source, received identifier) alone was the leak this table shipped
// with: one such key carried every prefix the source sent under that identifier,
// so no withdraw could free it without renumbering the prefixes still
// advertised under it, and a client that churns identifiers grew the daemon
// without bound. With the path in the key a withdraw frees exactly the path it
// withdraws, and the table is bounded by the paths ze advertises.
//
// Both maps group by source so a removed peer's entries go in one delete rather
// than a scan. used holds every identifier currently assigned, whichever map
// holds it, so a wrapped counter cannot hand a live path's identifier to a
// second path: (prefix, identifier) would then name two paths at the
// destination, which is the route loss this table exists to remove.
type fwdPathIDTable struct {
	mu       sync.RWMutex
	next     uint32
	bySource map[source.SourceID]map[uint32]uint32
	byPath   map[source.SourceID]map[fwdPathKey]uint32
	used     map[uint32]struct{}
}

func newFwdPathIDTable() *fwdPathIDTable {
	return &fwdPathIDTable{
		bySource: make(map[source.SourceID]map[uint32]uint32),
		byPath:   make(map[source.SourceID]map[fwdPathKey]uint32),
		used:     make(map[uint32]struct{}),
	}
}

// generate returns ze's identifier for the paths of a source that frames none,
// assigning one on first sight and returning the same one every time after.
// received is the identifier the source sent, which is 0 for every path such a
// source sends.
func (t *fwdPathIDTable) generate(src source.SourceID, received uint32) uint32 {
	t.mu.RLock()
	id, ok := t.bySource[src][received]
	t.mu.RUnlock()
	if ok {
		return id
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	perSource, ok := t.bySource[src]
	if !ok {
		perSource = make(map[uint32]uint32, 1)
		t.bySource[src] = perSource
	}
	if id, ok := perSource[received]; ok {
		return id
	}
	id = t.mintLocked()
	perSource[received] = id
	t.used[id] = struct{}{}
	return id
}

// generatePath returns ze's identifier for one path a source that frames Path
// Identifiers advertised, assigning one on first sight and returning the same
// one every time after. The path is the key, so a re-advertisement under the
// identifier the source already used keeps the identifier ze already gave it,
// which is the replacement RFC 7911 Section 5 defines.
func (t *fwdPathIDTable) generatePath(src source.SourceID, key *fwdPathKey) uint32 {
	t.mu.RLock()
	id, ok := t.byPath[src][*key]
	t.mu.RUnlock()
	if ok {
		return id
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	perSource, ok := t.byPath[src]
	if !ok {
		perSource = make(map[fwdPathKey]uint32, 1)
		t.byPath[src] = perSource
	}
	if id, ok := perSource[*key]; ok {
		return id
	}
	id = t.mintLocked()
	perSource[*key] = id
	t.used[id] = struct{}{}
	return id
}

// releasePath frees the identifier ze advertised for one path, so the value
// returns to the pool. A path ze holds no identifier for is not an error: a
// source may withdraw a pair it never announced, and RFC 7911 Section 5 has the
// receiver silently ignore such a withdraw.
//
// The caller MUST be fwdReleaseWithdrawnPathIDs, and it MUST run only once every
// destination has been given the identifier the withdraw carries. Freeing the
// entry inside the per-destination rewrite would mint a fresh identifier for
// every destination the fan-out had not reached yet, and each of those would
// hold a route ze can never withdraw.
func (t *fwdPathIDTable) releasePath(src source.SourceID, key *fwdPathKey) {
	t.mu.Lock()
	defer t.mu.Unlock()
	perSource, ok := t.byPath[src]
	if !ok {
		return
	}
	id, ok := perSource[*key]
	if !ok {
		return
	}
	delete(t.used, id)
	delete(perSource, *key)
	if len(perSource) == 0 {
		delete(t.byPath, src)
	}
}

// mintLocked returns an identifier no live path holds.
//
// The counter starts at 0 and 0 is issued like any other value (RFC 7911
// Section 3 makes it legal). The skip loop ends because it advances through a
// space of 2^32 values that only 2^32 concurrently advertised paths could fill,
// and holding that many entries would need tens of gigabytes of table.
func (t *fwdPathIDTable) mintLocked() uint32 {
	for {
		id := t.next
		t.next++
		if _, live := t.used[id]; !live {
			return id
		}
	}
}

// releaseSource drops every identifier assigned to a source's paths, so the
// values return to the pool.
//
// The call site is peer REMOVAL (reactor_peers.go doRemovePeer), not session
// down. A peer that reconnects re-announces the same paths under the same
// received identifiers, and keeping its entries means it also re-announces them
// under the same identifiers ze already used, which a destination reads as the
// replacement it is. Removal is the point where ze has withdrawn the peer's
// routes and will not send them again, which is what RFC 7911 needs before a
// value is reused (AC-4).
func (t *fwdPathIDTable) releaseSource(src source.SourceID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, id := range t.bySource[src] {
		delete(t.used, id)
	}
	delete(t.bySource, src)
	for _, id := range t.byPath[src] {
		delete(t.used, id)
	}
	delete(t.byPath, src)
}

// fwdPathIDMemo answers one NLRI walk's identifier lookups for the source that
// sent it.
//
// unframed holds the last answer, so a walk over a source that negotiated no
// ADD-PATH -- every prefix arriving under identifier 0 -- takes one table lock
// for the whole section rather than one per prefix. framed holds nothing,
// because a source that frames identifiers names a different path with every
// NLRI in the section.
type fwdPathIDMemo struct {
	source    source.SourceID
	have      bool
	received  uint32
	generated uint32
}

// unframed returns ze's identifier for a path whose source framed none.
func (m *fwdPathIDMemo) unframed(received uint32) uint32 {
	if m.have && m.received == received {
		return m.generated
	}
	id := fwdPathIDs.generate(m.source, received)
	m.have, m.received, m.generated = true, received, id
	return id
}

// framed returns ze's identifier for a path whose source framed one. raw is the
// NLRI the source sent, prefix-length octet included.
func (m *fwdPathIDMemo) framed(fam family.Family, received uint32, raw []byte) (uint32, error) {
	var key fwdPathKey
	if err := fwdPathKeyFor(&key, fam, received, raw); err != nil {
		return 0, err
	}
	return fwdPathIDs.generatePath(m.source, &key), nil
}

// fwdRegenerateRawPathIDs rewrites every Path Identifier a same-context payload
// carries with ze's own, into a pooled copy of that payload.
//
// The same-context forward is the route server: clients that negotiated the
// same capabilities share one encoding context, so a received frame is already
// framed the way the destination reads it and ze relays it without a parse. The
// identifiers inside that frame are the SOURCE's, which is the defect RFC 7911
// Section 2 names. The rewrite is length-preserving, because a Path Identifier
// is four octets before and after, so the copy keeps the frame's shape and the
// raw split that may follow it.
//
// Returns nil bytes when the shared context negotiated ADD-PATH for no family
// this UPDATE carries. That is every session without ADD-PATH, and it keeps the
// zero-copy forward it has today.
//
// The result is a payload rather than a WireUpdate because the caller's common
// path appends payload bytes and needs no wrapper. One wrapper is still built
// here to find the sections of the copy, and it does reach the heap: its lazy
// fields are guarded by sync.Once, whose closure escapes whatever the caller
// does (measured with -gcflags=-m). That is the same per-forward object both
// rails already allocate when a filter rebuilds an UPDATE (forward_rs.go,
// reactor_api_forward.go), and it accompanies a payload copy that is up to
// forty times its size.
//
// Lifetime contract A (docs/architecture/memory/lifetime-contracts.md): the
// returned payload ALIASES the returned BufHandle, and a forward-pool worker
// writes those bytes to TCP after buildFwdBody returns. The caller MUST carry
// the handle out for the ReceivedUpdate to adopt, never release it at end of
// call. A zero handle means the copy fell back to the heap and needs none.
func fwdRegenerateRawPathIDs(peerWire *wireu.WireUpdate, ctx *bgpctx.EncodingContext) ([]byte, BufHandle, error) {
	if !ctx.AnyAddPath() {
		return nil, BufHandle{}, nil
	}

	announced, err := peerWire.NLRI()
	if err != nil {
		return nil, BufHandle{}, err
	}
	withdrawn, err := peerWire.Withdrawn()
	if err != nil {
		return nil, BufHandle{}, err
	}
	needV4 := ctx.AddPath(family.IPv4Unicast) && (len(announced) > 0 || len(withdrawn) > 0)

	mpReach, err := peerWire.MPReach()
	if err != nil {
		return nil, BufHandle{}, err
	}
	needReach := mpReach != nil && ctx.AddPath(mpReach.Family()) && len(mpReach.NLRIBytes()) > 0

	mpUnreach, err := peerWire.MPUnreach()
	if err != nil {
		return nil, BufHandle{}, err
	}
	needUnreach := mpUnreach != nil && ctx.AddPath(mpUnreach.Family()) && len(mpUnreach.WithdrawnBytes()) > 0

	if !needV4 && !needReach && !needUnreach {
		return nil, BufHandle{}, nil
	}

	payload := peerWire.Payload()
	handle := getReadBuf(len(payload) > message.MaxMsgLen-message.HeaderLen)
	dst := handle.Buf
	if len(dst) < len(payload) {
		// pool-fallback, for the reason the RFC 6793 transcode takes one
		// (fwdUpdateForDestination): a collector-owned buffer is safe to alias
		// into the async write without a handle, and an allocation on the
		// exhausted-pool path is the correct trade against dropping a route.
		ReturnReadBuffer(handle)
		handle = BufHandle{}
		dst = make([]byte, len(payload))
	}
	dst = dst[:len(payload)]
	copy(dst, payload)

	// A reader over the COPY: its section accessors return slices into dst, so
	// every write below lands in the bytes the destination will read. Initialized
	// in place rather than through NewWireUpdate, which would put it on the heap
	// once per forward.
	var copied wireu.WireUpdate
	wireu.InitWireUpdate(&copied, dst, peerWire.SourceCtxID())

	// SINGLE return point for handle: ownership leaves with a successful return
	// or the buffer goes back to the pool here.
	failed := true
	defer func() {
		if failed {
			ReturnReadBuffer(handle)
		}
	}()

	memo := fwdPathIDMemo{source: peerWire.SourceID()}
	if needV4 {
		section, sectionErr := copied.NLRI()
		if sectionErr != nil {
			return nil, BufHandle{}, sectionErr
		}
		if err := fwdPatchPathIDs(section, family.IPv4Unicast, &memo); err != nil {
			return nil, BufHandle{}, fmt.Errorf("nlri: %w", err)
		}
		section, sectionErr = copied.Withdrawn()
		if sectionErr != nil {
			return nil, BufHandle{}, sectionErr
		}
		if err := fwdPatchPathIDs(section, family.IPv4Unicast, &memo); err != nil {
			return nil, BufHandle{}, fmt.Errorf("withdrawn routes: %w", err)
		}
	}
	if needReach {
		section, sectionErr := copied.MPReach()
		if sectionErr != nil {
			return nil, BufHandle{}, sectionErr
		}
		if err := fwdPatchPathIDs(section.NLRIBytes(), section.Family(), &memo); err != nil {
			return nil, BufHandle{}, fmt.Errorf("mp_reach nlri: %w", err)
		}
	}
	if needUnreach {
		section, sectionErr := copied.MPUnreach()
		if sectionErr != nil {
			return nil, BufHandle{}, sectionErr
		}
		if err := fwdPatchPathIDs(section.WithdrawnBytes(), section.Family(), &memo); err != nil {
			return nil, BufHandle{}, fmt.Errorf("mp_unreach withdrawn: %w", err)
		}
	}

	failed = false
	return dst, handle, nil
}

// fwdPatchPathIDs replaces the Path Identifier of every NLRI in data with ze's
// own, in place. data must be ADD-PATH framed (RFC 7911 Section 3): four octets
// of identifier, one octet of prefix length, then the prefix. Its source framed
// those identifiers, which is what the same-context branch above established, so
// every path here is keyed on the path rather than on the source alone.
func fwdPatchPathIDs(data []byte, fam family.Family, memo *fwdPathIDMemo) error {
	for off := 0; off < len(data); {
		if off+5 > len(data) {
			return fmt.Errorf("truncated path identifier at offset %d", off)
		}
		received := binary.BigEndian.Uint32(data[off:])
		start := off + 4
		end := start + 1 + nlri.PrefixBytes(int(data[start]))
		if end > len(data) {
			return fmt.Errorf("truncated prefix at offset %d", start)
		}
		id, err := memo.framed(fam, received, data[start:end])
		if err != nil {
			return err
		}
		binary.BigEndian.PutUint32(data[off:], id)
		off = end
	}
	return nil
}

// fwdReleaseWithdrawnPathIDs frees ze's identifier for every path this UPDATE
// withdrew.
//
// The caller MUST be the recent-update cache, at the eviction of the entry
// (recent_cache.go evictLocked and Delete), and MUST NOT be either forward rail.
// Eviction is the first moment at which no rail can still forward this UPDATE,
// and one UPDATE reaches BOTH rails: reactorForwardRS takes the destinations it
// can serve and hands the rest to the rs plugin as FastPathSkipped, which
// forwards them through forwardUpdateCore. A release at the end of the first
// rail would mint a fresh identifier for the second rail's destinations, and
// each of those would hold a route ze can never withdraw.
//
// Only a source that framed Path Identifiers has anything to free. A source that
// framed none holds one entry for its whole session, which peer removal ends
// (releaseSource). So an UPDATE from a session that negotiated ADD-PATH for
// nothing costs one context-registry read and returns.
//
// Lock ordering: the caller holds the cache mutex and this takes the identifier
// table's, so cache.mu -> fwdPathIDs.mu is the only nesting. The forward path
// takes fwdPathIDs.mu holding neither, and nothing takes cache.mu while holding
// fwdPathIDs.mu.
func fwdReleaseWithdrawnPathIDs(peerWire *wireu.WireUpdate) {
	if peerWire == nil {
		return
	}
	srcCtx := bgpctx.Registry.Get(peerWire.SourceCtxID())
	if srcCtx == nil || !srcCtx.AnyAddPath() {
		return
	}

	src := peerWire.SourceID()
	if err := fwdReleaseIPv4Withdrawn(peerWire, srcCtx, src); err != nil {
		fwdLogger().Warn("forward path identifier release failed", "src", src, "err", err)
	}
	if err := fwdReleaseMPWithdrawn(peerWire, srcCtx, src); err != nil {
		fwdLogger().Warn("forward path identifier release failed", "src", src, "err", err)
	}
}

// fwdReleaseIPv4Withdrawn frees the identifiers of the Withdrawn Routes field.
func fwdReleaseIPv4Withdrawn(peerWire *wireu.WireUpdate, srcCtx *bgpctx.EncodingContext, src source.SourceID) error {
	if !srcCtx.AddPath(family.IPv4Unicast) {
		return nil
	}
	withdrawn, err := peerWire.Withdrawn()
	if err != nil {
		return fmt.Errorf("withdrawn routes: %w", err)
	}
	if len(withdrawn) == 0 {
		return nil
	}
	announced, err := peerWire.NLRI()
	if err != nil {
		return fmt.Errorf("nlri: %w", err)
	}
	return fwdReleaseSection(src, family.IPv4Unicast, withdrawn, announced)
}

// fwdReleaseMPWithdrawn frees the identifiers of the MP_UNREACH_NLRI attribute.
func fwdReleaseMPWithdrawn(peerWire *wireu.WireUpdate, srcCtx *bgpctx.EncodingContext, src source.SourceID) error {
	mpUnreach, err := peerWire.MPUnreach()
	if err != nil {
		return fmt.Errorf("mp_unreach: %w", err)
	}
	if mpUnreach == nil {
		return nil
	}
	fam := mpUnreach.Family()
	if !srcCtx.AddPath(fam) {
		return nil
	}
	withdrawn := mpUnreach.WithdrawnBytes()
	if len(withdrawn) == 0 {
		return nil
	}

	// The announced half of the same family, when this UPDATE carries one. A
	// different family cannot hold the paths this one withdraws.
	var announced []byte
	mpReach, err := peerWire.MPReach()
	if err != nil {
		return fmt.Errorf("mp_reach: %w", err)
	}
	if mpReach != nil && mpReach.Family() == fam {
		announced = mpReach.NLRIBytes()
	}
	return fwdReleaseSection(src, fam, withdrawn, announced)
}

// fwdReleaseSection frees ze's identifier for every path one withdrawn section
// names, except a path the same UPDATE also announces.
//
// The exception is what keeps an UPDATE that both withdraws and announces one
// (prefix, identifier) pair from stranding it: the destination ends holding that
// pair, so ze must keep the identifier that named it. RFC 7606 Section 5.1
// forbids a conforming sender to put both fields in one UPDATE, so the scan of
// announced costs nothing for every peer that obeys it, and for a peer that does
// not it is bounded by the two sections of one message.
func fwdReleaseSection(src source.SourceID, fam family.Family, withdrawn, announced []byte) error {
	for off := 0; off < len(withdrawn); {
		if off+5 > len(withdrawn) {
			return fmt.Errorf("truncated path identifier at offset %d", off)
		}
		received := binary.BigEndian.Uint32(withdrawn[off:])
		start := off + 4
		end := start + 1 + nlri.PrefixBytes(int(withdrawn[start]))
		if end > len(withdrawn) {
			return fmt.Errorf("truncated prefix at offset %d", start)
		}
		raw := withdrawn[start:end]
		if !fwdSectionCarries(announced, received, raw) {
			var key fwdPathKey
			// An NLRI too long to key is an NLRI fwdPatchPathIDs and
			// fwdReencodeNLRIs could not key either, so no entry exists to free
			// and the forward that met it was already dropped.
			if err := fwdPathKeyFor(&key, fam, received, raw); err != nil {
				return err
			}
			fwdPathIDs.releasePath(src, &key)
		}
		off = end
	}
	return nil
}

// fwdSectionCarries reports whether an ADD-PATH framed section names the exact
// (identifier, NLRI) pair given. A malformed tail reads as absent: the section
// is the one the forward already walked, so it is well formed on every path that
// reaches here.
func fwdSectionCarries(section []byte, received uint32, raw []byte) bool {
	for off := 0; off+5 <= len(section); {
		start := off + 4
		end := start + 1 + nlri.PrefixBytes(int(section[start]))
		if end > len(section) {
			return false
		}
		if binary.BigEndian.Uint32(section[off:]) == received && bytes.Equal(section[start:end], raw) {
			return true
		}
		off = end
	}
	return false
}
