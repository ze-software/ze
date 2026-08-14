// Design: docs/architecture/bgp/forward-rails.md -- one egress transform, both rails
// RFC: rfc/short/rfc7911.md — Section 2, a re-advertised route carries the speaker's own Path Identifier
// Related: forward_body.go -- buildFwdBody (raw rail) and fwdReencodeNLRIs (re-encode rail)
package reactor

import (
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
// The key holds no prefix. Identifiers need to be unique per prefix, and two
// paths for one prefix from one source always arrive with different source
// identifiers, so (source, received identifier) separates them already. A
// source that reuses one identifier across prefixes therefore costs one entry
// rather than one per prefix, which keeps the ordinary case -- a client that
// negotiated no ADD-PATH, whose every path arrives with identifier 0 -- at a
// single entry for its whole session.
var fwdPathIDs = newFwdPathIDTable()

// fwdPathIDTable maps ingress paths to the identifiers ze advertises for them.
// A path at ingress is (source, received identifier), and a received identifier
// of 0 is a value rather than an absence: RFC 7911 Section 3 makes 0 legal, and
// a source that negotiated no ADD-PATH sends every path under it.
//
// bySource groups by source so a removed peer's entries go in one delete rather
// than a scan. used holds every identifier currently assigned, so a wrapped
// counter cannot hand a live path's identifier to a second path: (prefix,
// identifier) would then name two paths at the destination, which is the route
// loss this table exists to remove.
type fwdPathIDTable struct {
	mu       sync.RWMutex
	next     uint32
	bySource map[source.SourceID]map[uint32]uint32
	used     map[uint32]struct{}
}

func newFwdPathIDTable() *fwdPathIDTable {
	return &fwdPathIDTable{
		bySource: make(map[source.SourceID]map[uint32]uint32),
		used:     make(map[uint32]struct{}),
	}
}

// generate returns ze's identifier for the path the given source advertised
// under the given identifier, assigning one on first sight and returning the
// same one every time after.
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
}

// fwdPathIDMemo answers one NLRI walk's identifier lookups, holding the last
// answer so a walk over a source that negotiated no ADD-PATH -- every prefix
// arriving under identifier 0 -- takes one table lock for the whole section
// rather than one per prefix.
type fwdPathIDMemo struct {
	source    source.SourceID
	have      bool
	received  uint32
	generated uint32
}

func (m *fwdPathIDMemo) generate(received uint32) uint32 {
	if m.have && m.received == received {
		return m.generated
	}
	id := fwdPathIDs.generate(m.source, received)
	m.have, m.received, m.generated = true, received, id
	return id
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
		if err := fwdPatchPathIDs(section, &memo); err != nil {
			return nil, BufHandle{}, fmt.Errorf("nlri: %w", err)
		}
		section, sectionErr = copied.Withdrawn()
		if sectionErr != nil {
			return nil, BufHandle{}, sectionErr
		}
		if err := fwdPatchPathIDs(section, &memo); err != nil {
			return nil, BufHandle{}, fmt.Errorf("withdrawn routes: %w", err)
		}
	}
	if needReach {
		section, sectionErr := copied.MPReach()
		if sectionErr != nil {
			return nil, BufHandle{}, sectionErr
		}
		if err := fwdPatchPathIDs(section.NLRIBytes(), &memo); err != nil {
			return nil, BufHandle{}, fmt.Errorf("mp_reach nlri: %w", err)
		}
	}
	if needUnreach {
		section, sectionErr := copied.MPUnreach()
		if sectionErr != nil {
			return nil, BufHandle{}, sectionErr
		}
		if err := fwdPatchPathIDs(section.WithdrawnBytes(), &memo); err != nil {
			return nil, BufHandle{}, fmt.Errorf("mp_unreach withdrawn: %w", err)
		}
	}

	failed = false
	return dst, handle, nil
}

// fwdPatchPathIDs replaces the Path Identifier of every NLRI in data with ze's
// own, in place. data must be ADD-PATH framed (RFC 7911 Section 3): four octets
// of identifier, one octet of prefix length, then the prefix.
func fwdPatchPathIDs(data []byte, memo *fwdPathIDMemo) error {
	for off := 0; off < len(data); {
		if off+5 > len(data) {
			return fmt.Errorf("truncated path identifier at offset %d", off)
		}
		binary.BigEndian.PutUint32(data[off:], memo.generate(binary.BigEndian.Uint32(data[off:])))
		off += 4
		end := off + 1 + nlri.PrefixBytes(int(data[off]))
		if end > len(data) {
			return fmt.Errorf("truncated prefix at offset %d", off)
		}
		off = end
	}
	return nil
}
