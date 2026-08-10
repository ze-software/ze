// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc4271.md — UPDATE message format and size limit (Section 4.3)
// RFC: rfc/short/rfc7606.md — Section 5.1 one NLRI-bearing field per UPDATE
// RFC: rfc/short/rfc8654.md — extended message size
// RFC: rfc/short/rfc7911.md — ADD-PATH path identifiers in NLRI
// Overview: update.go — UPDATE message wire representation
// Related: update_build.go — UPDATE builder infrastructure
// Related: chunk_mp_nlri.go — MP NLRI chunking for multi-family splitting

package message

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

// Errors for UPDATE splitting and bounds checking.
var (
	// ErrAttributesTooLarge is returned when path attributes alone exceed maxSize.
	ErrAttributesTooLarge = errors.New("attributes exceed max message size")

	// ErrNLRITooLarge is returned when a single NLRI exceeds available space.
	ErrNLRITooLarge = errors.New("single NLRI exceeds available space")

	// ErrMPOverheadTooLarge is returned when MP attribute overhead exceeds maxAttrSize.
	ErrMPOverheadTooLarge = errors.New("MP attribute overhead exceeds max size")

	// ErrUpdateTooLarge is returned when a single-route UPDATE exceeds maxSize.
	// RFC 4271 Section 4.3: UPDATE max 4096 bytes (standard).
	// RFC 8654: Extended Message raises max to 65535 bytes.
	// Single-route builders return this when the atomic route cannot fit.
	ErrUpdateTooLarge = errors.New("UPDATE message exceeds max size")
)

// Splitter chunks an oversized UPDATE into smaller ones and delivers each chunk
// via a callback. It owns a scratch buffer that backs every emitted chunk's
// PathAttributes -- each chunk's slice is invalidated by the next emit call, so
// the callback MUST consume (WriteTo, copy out, hand to SendUpdate) before it
// returns. See the Update type doc and docs/architecture/update-building.md
// "Scratch Contract" for the full invariant.
//
// Typical ownership: one Splitter per peer (or per forward worker), retained
// across sessions to amortize the scratch allocation.
type Splitter struct {
	scratch []byte
	off     int
}

// NewSplitter returns a Splitter with a lazily-allocated scratch buffer.
func NewSplitter() *Splitter {
	return &Splitter{}
}

// splitterPool amortizes Splitter allocation across concurrent callers that do
// not own a dedicated Splitter instance. Reactor paths that call GetSplitter
// pay one atomic exchange + possibly zero allocations on the hot path; the
// scratch buffer is retained inside the Splitter across Put/Get so subsequent
// users with similar message sizes reuse it.
var splitterPool = sync.Pool{
	New: func() any { return NewSplitter() },
}

// GetSplitter returns a ready-to-use Splitter from the package pool. Call
// PutSplitter when done; do NOT retain the returned Splitter past the Put.
func GetSplitter() *Splitter {
	s, _ := splitterPool.Get().(*Splitter)
	return s
}

// PutSplitter returns s to the pool for reuse. The caller MUST ensure no
// previously-emitted chunk slices are retained past this call (per the
// scratch-aliasing invariant, those slices may be invalidated by any future
// Get+Split on the returned Splitter).
func PutSplitter(s *Splitter) {
	if s == nil {
		return
	}
	s.off = 0
	splitterPool.Put(s)
}

// resetScratch prepares the scratch buffer for a new Split call.
func (s *Splitter) resetScratch() {
	if s.scratch == nil {
		s.scratch = make([]byte, wire.StandardMaxSize) // pool-fallback
	}
	s.off = 0
}

// alloc returns a sub-slice of length n from the scratch buffer.
//
// First-grow path: scratch starts at StandardMaxSize (RFC 4271, 4096) and is
// resized once to ExtendedMaxSize (RFC 8654, 65535) when Split produces an
// extended-message chunk. A split chunk cannot exceed ExtendedMaxSize by
// RFC; caller's maxSize argument must be <= ExtendedMaxSize. Exceeding it
// means the caller passed a maxSize larger than any valid BGP UPDATE — that
// case panics.
func (s *Splitter) alloc(n int) []byte {
	end := s.off + n
	if end > len(s.scratch) {
		if end > wire.ExtendedMaxSize {
			panic("BUG: UPDATE split chunk exceeds RFC 8654 ExtendedMaxSize")
		}
		newBuf := make([]byte, wire.ExtendedMaxSize) // pool-fallback: bounded by RFC 8654
		copy(newBuf, s.scratch[:s.off])
		s.scratch = newBuf
	}
	out := s.scratch[s.off:end:end]
	s.off = end
	return out
}

// Split chunks an oversized UPDATE into multiple UPDATEs respecting maxSize
// and delivers each chunk to emit synchronously. addPath indicates whether
// Add-Path (RFC 7911) is negotiated for this family.
//
// Handles both IPv4 (NLRI field) and MP families (MP_REACH_NLRI attribute):
//   - IPv4 announcements: split u.NLRI via ChunkMPNLRI
//   - IPv4 withdrawals: split u.WithdrawnRoutes via ChunkMPNLRI
//   - MP announcements: detect MP_REACH_NLRI, split via SplitMPReachNLRI
//   - MP withdrawals: detect MP_UNREACH_NLRI, split via SplitMPUnreachNLRI
//
// WIRE CACHE PRESERVATION:
//   - For IPv4: u.PathAttributes is passed through unchanged to every chunk
//   - For MP: other attributes preserved, MP_REACH/UNREACH rebuilt per chunk
//     in the splitter's scratch
//
// Returns error if:
//   - Attributes alone exceed maxSize (ErrAttributesTooLarge)
//   - Single NLRI exceeds available space (ErrNLRITooLarge)
//   - emit returns non-nil (propagated)
//
// Note: maxSize is always 4096 or 65535 from MaxMessageLength() -- no validation needed.
//
// RFC 4271 Section 4.3: Each UPDATE is self-contained with full attributes.
// RFC 7911: Add-Path requires 4-byte path identifier before each NLRI.
func (s *Splitter) Split(u *Update, maxSize int, addPath bool, emit func(*Update) error) error {
	// Empty UPDATE (End-of-RIB) - pass through as-is.
	if u.IsEndOfRIB() {
		return emit(u)
	}

	// Calculate fixed overhead: header(19) + withdrawn_len(2) + attr_len(2) = 23
	overhead := HeaderLen + 4

	// Fast path: already fits.
	attrSize := len(u.PathAttributes)
	currentSize := overhead + len(u.WithdrawnRoutes) + attrSize + len(u.NLRI)
	if currentSize <= maxSize {
		return emit(u)
	}

	return s.splitByShape(u, maxSize, addPath, emit)
}

// SplitCompliant behaves like Split, except that it also splits an UPDATE that
// already fits when the UPDATE carries more than one NLRI-bearing field.
//
// RFC 7606 Section 5.1: "An UPDATE message MUST NOT contain more than one of the
// following: non-empty Withdrawn Routes field, non-empty Network Layer
// Reachability Information field, MP_REACH_NLRI attribute, and MP_UNREACH_NLRI
// attribute."
//
// This is a separate entry point rather than a change to Split because the two
// callers want different things. Relaying (reactor/forward_body.go) re-emits
// UPDATEs another speaker composed, so their shape is whatever arrived and has
// to be checked. Origination (reactor/peer_send.go) builds its own UPDATEs and
// is compliant by construction, so making it walk the attributes on every send
// to re-learn that would be pure cost. Everything past the shape decision is the
// same code.
func (s *Splitter) SplitCompliant(u *Update, maxSize int, addPath bool, emit func(*Update) error) error {
	if u.IsEndOfRIB() || !u.MixesNLRIFields() {
		return s.Split(u, maxSize, addPath, emit)
	}
	return s.splitByShape(u, maxSize, addPath, emit)
}

// splitByShape dispatches to the MP or IPv4 splitter. Both split entry points
// share it, so the two differ only in when they decide a split is needed.
func (s *Splitter) splitByShape(u *Update, maxSize int, addPath bool, emit func(*Update) error) error {
	// Detect MP attributes in PathAttributes.
	mpReachInfo := findMPAttribute(u.PathAttributes, attribute.AttrMPReachNLRI)
	mpUnreachInfo := findMPAttribute(u.PathAttributes, attribute.AttrMPUnreachNLRI)

	if mpReachInfo.found || mpUnreachInfo.found {
		return s.splitUpdateWithMP(u, maxSize, mpReachInfo, mpUnreachInfo, addPath, emit)
	}

	// IPv4 path (no MP attributes).
	return s.splitUpdateIPv4(u, maxSize, addPath, emit)
}

// ExtractMPFamily returns the address family from MP_REACH_NLRI or MP_UNREACH_NLRI
// in raw PathAttributes bytes. Returns false if no MP attribute is found.
// RFC 4760: MP attribute value starts with AFI(2) + SAFI(1).
func ExtractMPFamily(pathAttrs []byte) (family.Family, bool) {
	// Try MP_REACH_NLRI first (type 14), then MP_UNREACH_NLRI (type 15).
	for _, code := range []attribute.AttributeCode{attribute.AttrMPReachNLRI, attribute.AttrMPUnreachNLRI} {
		info := findMPAttribute(pathAttrs, code)
		if info.found && len(info.value) >= 3 {
			afi := family.AFI(uint16(info.value[0])<<8 | uint16(info.value[1]))
			safi := family.SAFI(info.value[2])
			return family.Family{AFI: afi, SAFI: safi}, true
		}
	}
	return family.Family{}, false
}

// mpAttrInfo holds information about an MP attribute in PathAttributes.
type mpAttrInfo struct {
	found bool   // Whether the attribute was found
	start int    // Offset where attribute starts (including TLV header)
	end   int    // Offset where attribute ends
	value []byte // Attribute value (excluding TLV header)
}

// findMPAttribute locates an MP attribute in raw PathAttributes bytes.
// Returns info about the attribute's location for later extraction/replacement.
// Uses AttrIterator to avoid duplicating TLV walk logic.
func findMPAttribute(pathAttrs []byte, code attribute.AttributeCode) mpAttrInfo {
	iter := attribute.NewAttrIterator(pathAttrs)
	for {
		start := iter.Offset()
		typeCode, _, value, ok := iter.Next()
		if !ok {
			return mpAttrInfo{found: false}
		}
		if typeCode == code {
			return mpAttrInfo{
				found: true,
				start: start,
				end:   iter.Offset(),
				value: value,
			}
		}
	}
}

// splitUpdateWithMP emits one or more UPDATE chunks when MP_REACH_NLRI or
// MP_UNREACH_NLRI is present. Each chunk's PathAttributes is written into the
// splitter's scratch: base attrs (everything except MP) at scratch[0:B),
// per-chunk MP attribute written at scratch[L:...) where L is the length of the
// base attrs that must PRECEDE the MP attribute by type code, emitted as
// scratch[0:F]. Between chunks s.off is reset to L so the next MP attribute
// overwrites the previous one while the base attrs stay valid.
//
// Splitting at L rather than at B is what keeps the chunks in ascending
// attribute-type-code order (RFC 4271 Section 5). Writing the MP attribute after
// ALL base attrs put MP_REACH (14) behind an EXTENDED_COMMUNITIES (16) the sender
// had supplied, so an announce that fitted in one message and the same announce
// split across two encoded their attributes in different orders. The base attrs
// above the MP code are stashed above the chunk region and re-copied after each
// MP attribute; when there are none (the common case) this costs nothing and the
// layout is exactly as before.
func (s *Splitter) splitUpdateWithMP(u *Update, maxSize int, mpReachInfo, mpUnreachInfo mpAttrInfo, addPath bool, emit func(*Update) error) error {
	s.resetScratch()
	overhead := HeaderLen + 4

	// Copy base attrs (u.PathAttributes minus MP_REACH and MP_UNREACH ranges)
	// into scratch in one pass.
	baseLen, lowLen := s.writeBaseAttrs(u.PathAttributes, mpReachInfo, mpUnreachInfo)
	highLen := baseLen - lowLen
	emitted := 0

	// Chunk budget, shared by both MP branches.
	//
	// highLen is subtracted even though it is already inside baseLen, so the high
	// attributes are charged twice: once as the originals in the base block and
	// once as the stash copy parked above the chunk region. That is deliberate and
	// load-bearing, not an oversight. Scratch peaks at
	// lowLen + (maxMPAttrValue + hdr) + highLen; with the double charge that is
	// exactly maxSize - overhead, and alloc panics above RFC 8654's ExtendedMaxSize
	// (65535). Drop the second charge and the peak becomes
	// maxSize - overhead + highLen, which for an extended-message peer
	// (maxSize == 65535) carrying large or IPv6 extended communities is a panic.
	//
	// The price is that each chunk carries up to highLen fewer NLRI bytes, so a
	// route with big high-coded attributes packs marginally fewer prefixes per
	// UPDATE. Bounded by the size of those attributes, correctness-neutral, and the
	// alternative layouts all reduce to carrying the same duplicate inside the same
	// bounded scratch -- moving the stash below the working area gives the same
	// peak. Anyone reclaiming those bytes must first make the scratch bound
	// independent of maxSize.
	const mpAttrHeaderSize = 4 // extended length, for safety
	maxMPAttrValue := maxSize - overhead - baseLen - mpAttrHeaderSize - highLen

	// Reserve the maximum chunk region and park the high attrs above it, so
	// writing a chunk at lowLen can never clobber the copy it needs afterwards.
	stashOff := 0
	if highLen > 0 {
		if maxMPAttrValue <= 0 {
			return ErrAttributesTooLarge
		}
		s.off = baseLen
		_ = s.alloc(maxMPAttrValue + mpAttrHeaderSize)
		stash := s.alloc(highLen)
		stashOff = baseLen + maxMPAttrValue + mpAttrHeaderSize
		copy(stash, s.scratch[lowLen:baseLen])
	}

	// The legacy IPv4 fields are carried too. They used to be dropped outright: every
	// emitted chunk was `&Update{PathAttributes: ...}` with no WithdrawnRoutes and no
	// NLRI, so an UPDATE holding BOTH an MP attribute and IPv4 routes lost the IPv4 half
	// silently on any path that re-chunked it.
	//
	// They are emitted in their own messages rather than bolted onto an MP chunk, which
	// is also what RFC 7606 Section 5.1 asks of a sender: at most one of Withdrawn Routes,
	// NLRI, MP_REACH_NLRI and MP_UNREACH_NLRI per UPDATE. Order across the whole function
	// is withdrawals then announcements: IPv4 withdrawn, MP_UNREACH, MP_REACH, IPv4 NLRI.
	if len(u.WithdrawnRoutes) > 0 {
		// Withdrawals carry no attributes, so the whole budget after the header is theirs.
		chunks, err := chunkIPv4NLRI(u.WithdrawnRoutes, maxSize-overhead, addPath)
		if err != nil {
			return fmt.Errorf("chunking withdrawn routes: %w", err)
		}
		for _, chunk := range chunks {
			if err := emit(&Update{WithdrawnRoutes: chunk}); err != nil {
				return err
			}
			emitted++
		}
	}

	// MP_UNREACH_NLRI first (withdrawals before announcements).
	if mpUnreachInfo.found {
		mpUnreach, err := attribute.ParseMPUnreachNLRI(mpUnreachInfo.value)
		if err != nil {
			return fmt.Errorf("parsing MP_UNREACH_NLRI: %w", err)
		}
		if maxMPAttrValue <= 0 {
			return ErrAttributesTooLarge
		}
		mpChunks, err := splitMPUnreachNLRIWithAddPath(mpUnreach, maxMPAttrValue, addPath)
		if err != nil {
			return fmt.Errorf("splitting MP_UNREACH_NLRI: %w", err)
		}
		for _, chunk := range mpChunks {
			if err := s.emitMPChunk(lowLen, highLen, stashOff, chunk, emit); err != nil {
				return err
			}
			emitted++
		}
	}

	// MP_REACH_NLRI (announcements).
	if mpReachInfo.found {
		mpReach, err := attribute.ParseMPReachNLRI(mpReachInfo.value)
		if err != nil {
			return fmt.Errorf("parsing MP_REACH_NLRI: %w", err)
		}
		if maxMPAttrValue <= 0 {
			return ErrAttributesTooLarge
		}
		mpChunks, err := SplitMPReachNLRIWithAddPath(mpReach, maxMPAttrValue, addPath)
		if err != nil {
			return fmt.Errorf("splitting MP_REACH_NLRI: %w", err)
		}
		for _, chunk := range mpChunks {
			if err := s.emitMPChunk(lowLen, highLen, stashOff, chunk, emit); err != nil {
				return err
			}
			emitted++
		}
	}

	// IPv4 announcements last, carrying the base attributes (the MP attributes are
	// excluded by writeBaseAttrs, so an IPv4 chunk never re-sends them).
	if len(u.NLRI) > 0 {
		space := maxSize - overhead - baseLen
		if space <= 0 {
			return ErrAttributesTooLarge
		}
		// The MP chunks above overwrote scratch[lowLen:baseLen); put the high
		// attributes back so the base block is whole again for these chunks.
		if highLen > 0 {
			copy(s.scratch[lowLen:baseLen], s.scratch[stashOff:stashOff+highLen])
		}
		chunks, err := chunkIPv4NLRI(u.NLRI, space, addPath)
		if err != nil {
			return fmt.Errorf("chunking NLRI: %w", err)
		}
		for _, chunk := range chunks {
			// s.scratch[:baseLen] is read here rather than captured earlier: alloc may have
			// grown the buffer while emitting MP chunks, and it copies the prefix across.
			if err := emit(&Update{PathAttributes: s.scratch[:baseLen], NLRI: chunk}); err != nil {
				return err
			}
			emitted++
		}
	}

	// No chunks emitted (shouldn't happen when mpInfo.found was true with NLRI).
	// Pass through the original UPDATE as a fallback.
	if emitted == 0 {
		return emit(u)
	}
	return nil
}

// writeBaseAttrs copies pathAttrs into scratch, omitting the ranges occupied by
// MP_REACH and MP_UNREACH. Returns the base-attrs length (also s.off after the
// call) and lowLen, the length of the leading run of those attributes whose type
// code is below MP_REACH_NLRI (14) -- the point at which the per-chunk MP
// attribute has to be written to keep the block in ascending type-code order
// (RFC 4271 Section 5).
//
// lowLen is a leading RUN, not "every attribute with a code below 14". The
// caller's block is copied in its original order because it may be a relayed
// UPDATE, and RFC 4271 Section 5 requires a receiver to accept attributes out of
// order; re-sorting another speaker's attributes here would be a gratuitous
// rewrite. For a block this daemon built (always ascending) the run is exactly
// the set of lower-coded attributes.
func (s *Splitter) writeBaseAttrs(pathAttrs []byte, mpReachInfo, mpUnreachInfo mpAttrInfo) (baseLen, lowLen int) {
	// Collect skip ranges sorted by start.
	var skipStart, skipEnd [2]int
	n := 0
	add := func(start, end int) {
		// Insert in sorted order (small N, simple).
		for i := 0; i < n; i++ {
			if start >= skipStart[i] {
				continue
			}
			copy(skipStart[i+1:n+1], skipStart[i:n])
			copy(skipEnd[i+1:n+1], skipEnd[i:n])
			skipStart[i] = start
			skipEnd[i] = end
			n++
			return
		}
		skipStart[n] = start
		skipEnd[n] = end
		n++
	}
	if mpUnreachInfo.found {
		add(mpUnreachInfo.start, mpUnreachInfo.end)
	}
	if mpReachInfo.found {
		add(mpReachInfo.start, mpReachInfo.end)
	}

	// Compute total base size and allocate once from scratch.
	baseSize := len(pathAttrs)
	for i := range n {
		baseSize -= skipEnd[i] - skipStart[i]
	}
	base := s.alloc(baseSize)

	// Copy surviving ranges.
	pos := 0
	off := 0
	for i := range n {
		off += copy(base[off:], pathAttrs[pos:skipStart[i]])
		pos = skipEnd[i]
	}
	copy(base[off:], pathAttrs[pos:])

	return baseSize, lowRunLen(base)
}

// lowRunLen returns the byte length of the leading run of attributes in attrs
// whose type code is below MP_REACH_NLRI (14). A malformed tail stops the walk,
// which is safe: the run only shortens, so the MP attribute is written earlier
// rather than into the middle of an attribute.
func lowRunLen(attrs []byte) int {
	iter := attribute.NewAttrIterator(attrs)
	for {
		off := iter.Offset()
		code, _, _, ok := iter.Next()
		if !ok {
			return off
		}
		if code >= attribute.AttrMPReachNLRI {
			return off
		}
	}
}

// emitMPChunk writes one MP attribute (chunk) into scratch at lowLen -- the
// type-code position, after the base attributes coded below MP_REACH_NLRI and
// before those coded above it -- follows it with the highLen stashed
// higher-coded base attributes from scratch[stashOff:], and emits an Update whose
// PathAttributes is scratch[0:F]. Resets s.off to lowLen after the callback so the
// next chunk overwrites this one's region while the low attributes stay valid.
//
// The stash is read, never written, so every chunk gets the same high attributes.
// splitUpdateWithMP reserved the maximum chunk region below stashOff, so this
// function's writes cannot reach it and its allocs cannot grow the scratch.
// highLen == 0 (nothing coded above MP) reduces to the original append.
func (s *Splitter) emitMPChunk(lowLen, highLen, stashOff int, chunk attribute.Attribute, emit func(*Update) error) error {
	attrLen := chunk.Len()
	// Extended length for >255-byte attributes.
	hdrLen := 3
	if attrLen > 255 {
		hdrLen = 4
	}
	// Reset to the MP insert position and allocate the chunk region.
	s.off = lowLen
	attrBuf := s.alloc(hdrLen + attrLen)
	attribute.WriteAttrToWithLen(chunk, attrBuf, 0, attrLen)

	if highLen > 0 {
		high := s.alloc(highLen)
		copy(high, s.scratch[stashOff:stashOff+highLen])
	}

	err := emit(&Update{
		PathAttributes: s.scratch[:s.off],
	})
	s.off = lowLen // always reset, even on error; next call starts fresh
	return err
}

// splitUpdateIPv4 emits one or more UPDATE chunks for an IPv4-only UPDATE.
// The input u.PathAttributes is passed through unchanged to each chunk. NLRI
// byte chunks come from ChunkMPNLRI (heap-allocated there; outside this
// splitter's scratch ownership).
func (s *Splitter) splitUpdateIPv4(u *Update, maxSize int, addPath bool, emit func(*Update) error) error {
	overhead := HeaderLen + 4
	attrSize := len(u.PathAttributes)

	// Check if attributes alone exceed limit (cannot split).
	if overhead+attrSize > maxSize {
		return ErrAttributesTooLarge
	}

	hasWithdrawn := len(u.WithdrawnRoutes) > 0
	hasNLRI := len(u.NLRI) > 0

	// Split withdrawals (no attributes needed for withdrawals).
	if hasWithdrawn {
		withdrawnSpace := maxSize - overhead // No attrs for withdrawals
		withdrawnChunks, err := chunkIPv4NLRI(u.WithdrawnRoutes, withdrawnSpace, addPath)
		if err != nil {
			return fmt.Errorf("chunking withdrawn routes: %w", err)
		}
		for _, chunk := range withdrawnChunks {
			if err := emit(&Update{WithdrawnRoutes: chunk}); err != nil {
				return err
			}
		}
	}

	// Split announcements (need attributes).
	if hasNLRI {
		nlriSpace := maxSize - overhead - attrSize
		if nlriSpace <= 0 {
			return ErrAttributesTooLarge
		}
		nlriChunks, err := chunkIPv4NLRI(u.NLRI, nlriSpace, addPath)
		if err != nil {
			return fmt.Errorf("chunking NLRI: %w", err)
		}
		for _, chunk := range nlriChunks {
			if err := emit(&Update{
				PathAttributes: u.PathAttributes, // Zero-copy: reuse same slice
				NLRI:           chunk,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

// chunkIPv4NLRI splits IPv4 NLRI respecting maxSize and Add-Path state.
//
// When addPath is false, validates and chunks using basic prefix format.
// When addPath is true, uses ChunkMPNLRI with Add-Path awareness.
//
// Returns ErrNLRITooLarge if a single NLRI exceeds maxSize.
// Returns ErrNLRIMalformed if NLRI structure is invalid.
//
// AFI=1 (IPv4), SAFI=1 (Unicast) is used for ChunkMPNLRI.
func chunkIPv4NLRI(nlriData []byte, maxSize int, addPath bool) ([][]byte, error) {
	if len(nlriData) == 0 {
		return nil, nil
	}

	// Use family-aware chunker for both cases (it handles Add-Path correctly)
	// AFI=IPv4, SAFI=Unicast
	return ChunkMPNLRI(nlriData, family.AFIIPv4, family.SAFIUnicast, addPath, maxSize, nil)
}

// =============================================================================
// MP_REACH_NLRI and MP_UNREACH_NLRI attribute-level splitting
// =============================================================================

// splitMPReachNLRI splits an MP_REACH_NLRI attribute into chunks.
//
// maxAttrSize is the maximum size for the attribute VALUE (not including TLV header).
// Returns multiple MPReachNLRI with chunked NLRI, preserving AFI/SAFI/NextHops.
//
// Uses ChunkMPNLRI for family-aware NLRI parsing (handles Add-Path, VPN, EVPN, etc.).
//
// RFC 4760 Section 3: MP_REACH_NLRI wire format:
//
//	AFI(2) + SAFI(1) + NH_Len(1) + NextHops + Reserved(1) + NLRI
//
// The overhead (everything except NLRI) is preserved in each chunk.
func splitMPReachNLRI(mp *attribute.MPReachNLRI, maxAttrSize int) ([]*attribute.MPReachNLRI, error) {
	return SplitMPReachNLRIWithAddPath(mp, maxAttrSize, false)
}

// SplitMPReachNLRIWithAddPath splits MP_REACH_NLRI with Add-Path awareness.
//
// addPath indicates whether Add-Path is negotiated for this family.
// Required for correct NLRI boundary detection when path-ids are present.
func SplitMPReachNLRIWithAddPath(mp *attribute.MPReachNLRI, maxAttrSize int, addPath bool) ([]*attribute.MPReachNLRI, error) {
	if mp == nil || len(mp.NLRI) == 0 {
		return []*attribute.MPReachNLRI{mp}, nil
	}

	// Calculate overhead: AFI(2) + SAFI(1) + NH_Len(1) + NextHops + Reserved(1)
	nhLen := 0
	for _, nh := range mp.NextHops.Slice() {
		if nh.Is4() {
			nhLen += 4
		} else {
			nhLen += 16
		}
	}
	overhead := 2 + 1 + 1 + nhLen + 1

	if overhead >= maxAttrSize {
		return nil, fmt.Errorf("%w: overhead %d, max %d", ErrMPOverheadTooLarge, overhead, maxAttrSize)
	}

	nlriSpace := maxAttrSize - overhead

	// Use family-aware chunking
	nlriChunks, err := ChunkMPNLRI(mp.NLRI, family.AFI(mp.AFI), family.SAFI(mp.SAFI), addPath, nlriSpace, nil)
	if err != nil {
		return nil, fmt.Errorf("chunking MP_REACH_NLRI: %w", err)
	}

	// Single chunk or empty? Return as-is
	if len(nlriChunks) <= 1 {
		return []*attribute.MPReachNLRI{mp}, nil
	}

	nhs := mp.NextHops.Slice()
	results := make([]*attribute.MPReachNLRI, len(nlriChunks)) // pool-fallback
	for i, chunk := range nlriChunks {
		results[i] = attribute.NewMPReachNLRI(mp.AFI, mp.SAFI, nhs, chunk)
	}

	return results, nil
}

// splitMPUnreachNLRI splits an MP_UNREACH_NLRI attribute into chunks.
//
// maxAttrSize is the maximum size for the attribute VALUE (not including TLV header).
// Returns multiple MPUnreachNLRI with chunked NLRI, preserving AFI/SAFI.
//
// Uses ChunkMPNLRI for family-aware NLRI parsing (handles Add-Path, VPN, EVPN, etc.).
//
// RFC 4760 Section 4: MP_UNREACH_NLRI wire format:
//
//	AFI(2) + SAFI(1) + Withdrawn_NLRI
//
// The overhead (AFI + SAFI) is preserved in each chunk.
func splitMPUnreachNLRI(mp *attribute.MPUnreachNLRI, maxAttrSize int) ([]*attribute.MPUnreachNLRI, error) {
	return splitMPUnreachNLRIWithAddPath(mp, maxAttrSize, false)
}

// splitMPUnreachNLRIWithAddPath splits MP_UNREACH_NLRI with Add-Path awareness.
//
// addPath indicates whether Add-Path is negotiated for this family.
// Required for correct NLRI boundary detection when path-ids are present.
func splitMPUnreachNLRIWithAddPath(mp *attribute.MPUnreachNLRI, maxAttrSize int, addPath bool) ([]*attribute.MPUnreachNLRI, error) {
	if mp == nil || len(mp.NLRI) == 0 {
		return []*attribute.MPUnreachNLRI{mp}, nil
	}

	// Overhead: AFI(2) + SAFI(1) = 3 bytes
	overhead := 3

	if overhead >= maxAttrSize {
		return nil, fmt.Errorf("%w: overhead %d, max %d", ErrMPOverheadTooLarge, overhead, maxAttrSize)
	}

	nlriSpace := maxAttrSize - overhead

	// Use family-aware chunking
	nlriChunks, err := ChunkMPNLRI(mp.NLRI, family.AFI(mp.AFI), family.SAFI(mp.SAFI), addPath, nlriSpace, nil)
	if err != nil {
		return nil, fmt.Errorf("chunking MP_UNREACH_NLRI: %w", err)
	}

	// Single chunk or empty? Return as-is
	if len(nlriChunks) <= 1 {
		return []*attribute.MPUnreachNLRI{mp}, nil
	}

	results := make([]*attribute.MPUnreachNLRI, 0, len(nlriChunks))
	for _, chunk := range nlriChunks {
		results = append(results, &attribute.MPUnreachNLRI{
			AFI:  mp.AFI,
			SAFI: mp.SAFI,
			NLRI: chunk,
		})
	}

	return results, nil
}
