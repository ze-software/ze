// Design: docs/architecture/bgp/structural-forwarding.md -- one egress transform, both rails
// RFC: rfc/short/rfc7911.md — Section 2, a re-advertised route carries the speaker's own Path Identifier
// Related: forward_body.go -- buildFwdBody (raw rail) and fwdReencodeNLRIs (re-encode rail)
package reactor

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
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

// fwdPathNLRIMax keeps ordinary IP path keys inline. Larger native NLRIs use
// overflow rather than truncating FlowSpec's octet-length framing to a prefix.
const fwdPathNLRIMax = 1 + 32

// fwdPathKey names one path a source that frames Path Identifiers advertised:
// the family it arrived in, the identifier the source chose, and the family's
// canonical route key. length distinguishes a native key from inline padding.
//
// The family is part of the key because NLRI bytes alone are ambiguous across
// families: 10.0.0.0/24 and 0a00::/24 carry the same length octet and the same
// three significant octets.
type fwdPathKey struct {
	family   family.Family
	received uint32
	nlri     [fwdPathNLRIMax]byte
	length   uint16
	overflow string
}

// fwdPathKeyFor writes the key of one ingress path into out. raw includes native
// NLRI framing but excludes ADD-PATH. The family removes non-key fields, such as
// label stacks and FlowSpec length octets, before identity is assigned.
func fwdPathKeyFor(out *fwdPathKey, fam family.Family, received uint32, raw []byte, withdraw bool, scratch []byte) error {
	key, err := nlrisplit.GetPrefixKey(fam)(raw, scratch, withdraw)
	if err != nil {
		return err
	}
	if len(key) > message.ExtMsgLen {
		return fmt.Errorf("nlri key of %d octets exceeds the BGP message limit", len(key))
	}
	*out = fwdPathKey{family: fam, received: received, length: uint16(len(key))}
	if len(key) <= len(out.nlri) {
		copy(out.nlri[:], key)
	} else {
		out.overflow = string(key)
	}
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
func (t *fwdPathIDTable) releasePath(src source.SourceID, key *fwdPathKey, peer netip.Addr, raw []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Query while holding the identifier lock: no concurrent forward can take
	// the old identifier between this presence check and its removal.
	if validationRetainsPath(peer, key.family, key.received, raw) {
		return
	}
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
// for the whole section rather than one per prefix. Framed lookups reuse key
// scratch, but cache no answer: each NLRI can name a different path.
type fwdPathIDMemo struct {
	source     source.SourceID
	have       bool
	received   uint32
	generated  uint32
	keyScratch [nlrisplit.PrefixKeyScratchSize]byte
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
// NLRI the source sent, native framing included.
func (m *fwdPathIDMemo) framed(fam family.Family, received uint32, raw []byte, withdraw bool) (uint32, error) {
	var key fwdPathKey
	if err := fwdPathKeyFor(&key, fam, received, raw, withdraw, m.keyScratch[:]); err != nil {
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
		if err := fwdPatchPathIDs(section, family.IPv4Unicast, &memo, false); err != nil {
			return nil, BufHandle{}, fmt.Errorf("nlri: %w", err)
		}
		section, sectionErr = copied.Withdrawn()
		if sectionErr != nil {
			return nil, BufHandle{}, sectionErr
		}
		if err := fwdPatchPathIDs(section, family.IPv4Unicast, &memo, true); err != nil {
			return nil, BufHandle{}, fmt.Errorf("withdrawn routes: %w", err)
		}
	}
	if needReach {
		section, sectionErr := copied.MPReach()
		if sectionErr != nil {
			return nil, BufHandle{}, sectionErr
		}
		if err := fwdPatchPathIDs(section.NLRIBytes(), section.Family(), &memo, false); err != nil {
			return nil, BufHandle{}, fmt.Errorf("mp_reach nlri: %w", err)
		}
	}
	if needUnreach {
		section, sectionErr := copied.MPUnreach()
		if sectionErr != nil {
			return nil, BufHandle{}, sectionErr
		}
		if err := fwdPatchPathIDs(section.WithdrawnBytes(), section.Family(), &memo, true); err != nil {
			return nil, BufHandle{}, fmt.Errorf("mp_unreach withdrawn: %w", err)
		}
	}

	failed = false
	return dst, handle, nil
}

// fwdPatchPathIDs replaces the Path Identifier of every native NLRI in data.
// The registered family splitter owns framing; only the leading four octets
// belong to ADD-PATH. The walk is bounded by the UPDATE section's length.
func fwdPatchPathIDs(data []byte, fam family.Family, memo *fwdPathIDMemo, withdraw bool) error {
	split := nlrisplit.Get(fam)
	if withdraw {
		split = nlrisplit.GetWithdraw(fam)
	}
	if split == nil {
		return nlrisplit.ErrUnsupported
	}
	var keyErr error
	_, err := split(data, true, func(raw []byte) {
		if keyErr != nil {
			return
		}
		received := binary.BigEndian.Uint32(raw[:4])
		var id uint32
		id, keyErr = memo.framed(fam, received, raw[4:], withdraw)
		if keyErr == nil {
			binary.BigEndian.PutUint32(raw[:4], id)
		}
	})
	if err != nil {
		return err
	}
	return keyErr
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
// Lock ordering: cache.mu -> fwdPathIDs.mu -> Adj-RIB-In read lock. The receive
// store MUST release its lock before cache operations or validation events.
// Retained paths keep their identifiers even if a stale withdrawal cache entry
// is evicted after a replacement has become eligible.
func fwdReleaseWithdrawnPathIDs(update *ReceivedUpdate) {
	peerWire := update.WireUpdate
	if peerWire == nil {
		return
	}
	srcCtx := bgpctx.Registry.Get(peerWire.SourceCtxID())
	if srcCtx == nil || !srcCtx.AnyAddPath() {
		return
	}

	src := peerWire.SourceID()
	if err := fwdReleaseIPv4Withdrawn(peerWire, srcCtx, src, update.SourcePeerIP); err != nil {
		fwdLogger().Warn("forward path identifier release failed", "src", src, "err", err)
	}
	if err := fwdReleaseMPWithdrawn(peerWire, srcCtx, src, update.SourcePeerIP); err != nil {
		fwdLogger().Warn("forward path identifier release failed", "src", src, "err", err)
	}
}

// fwdReleaseIPv4Withdrawn frees the identifiers of the Withdrawn Routes field.
func fwdReleaseIPv4Withdrawn(peerWire *wireu.WireUpdate, srcCtx *bgpctx.EncodingContext, src source.SourceID, peer netip.Addr) error {
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
	return fwdReleaseSection(src, peer, family.IPv4Unicast, withdrawn, announced)
}

// fwdReleaseMPWithdrawn frees the identifiers of the MP_UNREACH_NLRI attribute.
func fwdReleaseMPWithdrawn(peerWire *wireu.WireUpdate, srcCtx *bgpctx.EncodingContext, src source.SourceID, peer netip.Addr) error {
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
	return fwdReleaseSection(src, peer, fam, withdrawn, announced)
}

// fwdReleaseSection frees ze's identifier for every path one withdrawn section
// names, except a path the same UPDATE also announces.
//
// The exception is what keeps an UPDATE that both withdraws and announces one
// (prefix, identifier) pair from stranding it: the destination ends holding that
// pair, so ze must keep the identifier that named it. RFC 7606 Section 5.1
// forbids a conforming sender to put both fields in one UPDATE, so an ordinary
// withdraw reaches an empty announced section and builds no set at all.
func fwdReleaseSection(src source.SourceID, peer netip.Addr, fam family.Family, withdrawn, announced []byte) error {
	alsoAnnounced, err := fwdAnnouncedPaths(fam, announced)
	if err != nil {
		return fmt.Errorf("announced section: %w", err)
	}

	split := nlrisplit.GetWithdraw(fam)
	if split == nil {
		return nlrisplit.ErrUnsupported
	}
	var keyErr error
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	_, err = split(withdrawn, true, func(raw []byte) {
		if keyErr != nil {
			return
		}
		var key fwdPathKey
		keyErr = fwdPathKeyFor(&key, fam, binary.BigEndian.Uint32(raw[:4]), raw[4:], true, scratch[:])
		if keyErr != nil {
			return
		}
		if _, both := alsoAnnounced[key]; !both {
			fwdPathIDs.releasePath(src, &key, peer, raw[4:])
		}
	})
	if err != nil {
		return err
	}
	return keyErr
}

// fwdAnnouncedPaths keys every path an ADD-PATH framed announced section names,
// so the release above answers "does this same UPDATE announce the pair" with
// one lookup. It returns a nil map for an empty section, which every conforming
// withdraw carries (RFC 7606 Section 5.1), and a nil map answers every lookup
// with "no".
//
// The set replaces a walk of the announced section per withdrawn NLRI. That walk
// was quadratic in one message's NLRI count, it ran with the recent-update
// cache mutex held (recent_cache.go evictLocked), and a peer chose both counts:
// two 32000-octet sections of an RFC 8654 extended UPDATE (message.ExtMsgLen)
// cost 41 million comparisons, measured at 137ms per UPDATE on the developer
// machine, repeatable at line rate. The set costs one pass over each section.
//
// The set is bounded by the number of NLRIs in one BGP message. The registered
// splitter rejects malformed native framing before an unrelated key is freed.
func fwdAnnouncedPaths(fam family.Family, section []byte) (map[fwdPathKey]struct{}, error) {
	if len(section) == 0 {
		return nil, nil
	}
	split := nlrisplit.Get(fam)
	if split == nil {
		return nil, nlrisplit.ErrUnsupported
	}
	var paths map[fwdPathKey]struct{}
	var keyErr error
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	_, err := split(section, true, func(raw []byte) {
		if keyErr != nil {
			return
		}
		var key fwdPathKey
		keyErr = fwdPathKeyFor(&key, fam, binary.BigEndian.Uint32(raw[:4]), raw[4:], false, scratch[:])
		if keyErr != nil {
			return
		}
		if paths == nil {
			paths = make(map[fwdPathKey]struct{})
		}
		paths[key] = struct{}{}
	})
	if err != nil {
		return nil, err
	}
	return paths, keyErr
}
