// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc4760.md — MP_REACH_NLRI / MP_UNREACH_NLRI chunking
// Related: update_split.go — UPDATE splitting and chunking

package message

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// ErrNLRIMalformed is returned when NLRI structure is invalid.
var ErrNLRIMalformed = errors.New("malformed NLRI")

// nlriChunkWalk keeps the registered walk's callback outside the per-call
// allocation path. The callback is created once per pooled holder, not per NLRI.
// A caller MUST clear borrowed slices before returning the holder to the pool.
type nlriChunkWalk struct {
	data        []byte
	chunks      [][]byte
	maxSize     int
	offset      int
	chunkStart  int
	tooLarge    int
	stopAtLimit bool
	visit       func([]byte) bool
}

var nlriChunkWalkPool = sync.Pool{
	New: func() any {
		w := new(nlriChunkWalk)
		w.visit = w.next
		return w
	},
}

// next consumes only the boundary the registered family walk supplied.
func (w *nlriChunkWalk) next(part []byte) bool {
	if len(part) > w.maxSize {
		w.tooLarge = len(part)
		return false
	}
	if w.offset-w.chunkStart+len(part) > w.maxSize {
		if w.stopAtLimit {
			return false
		}
		w.chunks = append(w.chunks, w.data[w.chunkStart:w.offset])
		w.chunkStart = w.offset
	}
	w.offset += len(part)
	return true
}

// ChunkMPNLRI splits MP family NLRIs respecting maxSize.
//
// Uses registered native family framing, including negotiated ADD-PATH.
// Returns subslices of nlriData (zero-copy).
//
// Returns error if:
//   - Single NLRI exceeds maxSize (ErrNLRITooLarge)
//   - NLRI is truncated/malformed (ErrNLRIMalformed)
//
// RFC 4271 Section 4.3 - UPDATE message format, max 4096 bytes.
// RFC 8654 - Extended Message raises max to 65535 bytes.
// RFC 4760 - MP_REACH_NLRI / MP_UNREACH_NLRI wire format.
// RFC 7911 - ADD-PATH adds 4-byte path-id before each NLRI.
// RFC 8277 - Labeled unicast: length includes label bits.
// RFC 4364 - VPN: labels + 8-byte RD + prefix.
// RFC 7432 - EVPN: [route-type:1][length:1][payload].
// RFC 5575 - FlowSpec: max 4095 bytes per NLRI (CAN split).
// RFC 7752 - BGP-LS: 2-byte length, single NLRI can exceed 4096.
func ChunkMPNLRI(nlriData []byte, afi family.AFI, safi family.SAFI, addPath bool, maxSize int, dst [][]byte) ([][]byte, error) {
	if len(nlriData) == 0 {
		return dst, nil
	}

	// RFC 8277 Section 2.4: the Compatibility field is not a label stack.
	// The withdrawal framer reads the same envelope boundary for announcements,
	// without interpreting labels when this section contains withdrawals.
	walk := nlrisplit.GetWithdraw(family.Family{AFI: afi, SAFI: safi})
	if walk == nil {
		return dst, nlrisplit.ErrUnsupported
	}
	w := nlriChunkWalkPool.Get().(*nlriChunkWalk)
	w.data = nlriData
	w.chunks = dst
	w.maxSize = maxSize
	w.offset = 0
	w.chunkStart = 0
	w.tooLarge = 0
	w.stopAtLimit = false
	_, err := walk(nlriData, addPath, w.visit)
	dst = w.chunks
	if w.tooLarge != 0 {
		err = fmt.Errorf("%w: %d bytes, max %d", ErrNLRITooLarge, w.tooLarge, maxSize)
	} else if err != nil {
		err = fmt.Errorf("%w: %w", ErrNLRIMalformed, err)
	} else if w.offset > w.chunkStart {
		dst = append(dst, nlriData[w.chunkStart:w.offset])
	}
	// The pool MUST NOT retain the caller's wire buffer or result backing array.
	w.data = nil
	w.chunks = nil
	nlriChunkWalkPool.Put(w)
	if err != nil {
		return dst, err
	}

	return dst, nil
}

// SplitMPNLRI splits MP family NLRIs, returning fitting slice and remaining.
// Returns subslices for zero-copy efficiency.
// This enables O(n) splitting across multiple calls instead of O(n²).
//
// Used when forwarding wire UPDATEs to peers with smaller buffers:
// - Extended Message peer (RFC 8654: 65535) → standard peer (RFC 4271: 4096)
//
// Returns:
//   - (data, nil, nil) if all data fits within maxSize
//   - (fitting, remaining, nil) if split was needed
//   - (nil, nil, error) if NLRI is malformed or single NLRI exceeds maxSize
//
// RFC 4271 Section 4.3 - UPDATE max 4096 bytes.
// RFC 8654 - Extended Message raises to 65535 bytes.
// RFC 4760 - MP_REACH_NLRI / MP_UNREACH_NLRI wire format.
// RFC 7911 - ADD-PATH: 4-byte path-id before each NLRI.
func SplitMPNLRI(nlriData []byte, afi family.AFI, safi family.SAFI, addPath bool, maxSize int) (fitting, remaining []byte, err error) {
	if maxSize <= 0 {
		return nil, nil, fmt.Errorf("invalid maxSize: %d", maxSize)
	}
	if len(nlriData) == 0 {
		return nil, nil, nil
	}
	// RFC 8277 Section 2.4: frame Compatibility by length, not its S bit.
	walk := nlrisplit.GetWithdraw(family.Family{AFI: afi, SAFI: safi})
	if walk == nil {
		return nil, nil, nlrisplit.ErrUnsupported
	}
	w := nlriChunkWalkPool.Get().(*nlriChunkWalk)
	w.maxSize = maxSize
	w.offset = 0
	w.chunkStart = 0
	w.tooLarge = 0
	w.stopAtLimit = true
	_, err = walk(nlriData, addPath, w.visit)
	offset := w.offset
	if w.tooLarge != 0 {
		err = fmt.Errorf("%w: %d bytes, max %d", ErrNLRITooLarge, w.tooLarge, maxSize)
	} else if err != nil {
		err = fmt.Errorf("%w: %w", ErrNLRIMalformed, err)
	}
	nlriChunkWalkPool.Put(w)
	if err != nil {
		return nil, nil, err
	}
	if offset == len(nlriData) {
		return nlriData, nil, nil
	}
	return nlriData[:offset], nlriData[offset:], nil
}
