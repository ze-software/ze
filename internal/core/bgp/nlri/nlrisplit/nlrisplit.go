// Design: docs/architecture/rib/unified-locrib.md -- per-family NLRI split
//
// Package nlrisplit holds a registry of family-specific NLRI splitters.
// Concatenated NLRI wire bytes arrive from the peer; each family has its
// own per-NLRI framing (CIDR-style [prefix-len][addr], EVPN's
// [route-type][length][body], flowspec's component tuple, ...). The BGP
// RIB dispatches through this registry so new families only need a
// splitter registration -- no edits to the RIB hot path.
//
// A splitter is a WALK, not a builder: it visits each NLRI in wire order
// and returns how many it visited. The walk is the one declaration of a
// family's framing, and every shape a caller wants is derived from it.
// Split materializes one slice per NLRI for a caller that needs them all;
// a caller that needs only the number walks with a nil fn and allocates
// nothing, which is what the per-UPDATE prefix maximum does.
//
// Registration is via init() in each NLRI plugin package. In-process and
// forked-plugin runs both import internal/component/plugin/all, so every
// registered splitter is available in either mode.
package nlrisplit

import (
	"errors"
	"sync"

	"github.com/ze-software/ze/internal/core/family"
)

// ErrUnsupported is returned by Split when no splitter is registered for
// the given family. Callers treat this as "drop the input" (the peer
// advertised a family we cannot parse).
var ErrUnsupported = errors.New("nlrisplit: no splitter registered for family")

// Splitter carves concatenated NLRI wire bytes into individual NLRIs and
// visits them in wire order. It calls fn once per NLRI, when fn is non-nil,
// and returns the number of NLRIs it visited.
//
// A nil fn walks the same bytes to the same verdict and allocates nothing.
// That is the count pass: a prefix maximum compares a number and never looks
// at an NLRI, so it MUST NOT pay for a slice it will not read.
//
// Under ADD-PATH (RFC 7911) each NLRI is prefixed by a 4-byte path-id and the
// splitter includes those bytes in the slice it hands to fn, so downstream
// consumers use the exact wire representation as their key.
//
// The slice fn receives aliases the input data (zero-copy). fn MUST copy the
// bytes it needs to retain past the call.
//
// A well-formed empty input visits nothing and returns 0, nil. A malformed
// input visits every NLRI before the corruption, counts those, and returns a
// non-nil error describing the corruption; callers choose whether to use the
// partial result.
type Splitter func(data []byte, addPath bool, fn func(nlri []byte)) (int, error)

var (
	mu        sync.RWMutex
	splitters = map[family.Family]Splitter{}
)

// Register installs fn as the splitter for fam. Panics on duplicate
// registration -- splitters are registered once at init time. Passing a
// nil fn is a no-op, useful for families that are explicitly "not yet
// supported".
func Register(fam family.Family, fn Splitter) {
	if fn == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := splitters[fam]; ok {
		panic("BUG: nlrisplit.Register: duplicate splitter for " + fam.String())
	}
	splitters[fam] = fn
}

// Get returns the registered splitter for fam, or nil if none is
// registered. Useful for "can we handle this family" probes.
func Get(fam family.Family) Splitter {
	mu.RLock()
	defer mu.RUnlock()
	return splitters[fam]
}

// Supported reports whether fam has a registered splitter.
func Supported(fam family.Family) bool {
	return Get(fam) != nil
}

// Split dispatches to the family's registered splitter and materializes one
// slice per NLRI. Returns ErrUnsupported when no splitter is registered; the
// input slice is unchanged in that case.
//
// The family is walked twice: a count pass with a nil fn sizes the result
// exactly, and a fill pass collects it. Sizing beats growing here because a
// section carries hundreds of NLRIs and the count pass reads only length
// fields. A caller that wants the number alone walks once through Get.
//
// The returned slices alias data, and the partial-result contract is the
// Splitter's: a malformed input returns the NLRIs parsed before the
// corruption plus a non-nil error.
func Split(fam family.Family, data []byte, addPath bool) ([][]byte, error) {
	fn := Get(fam)
	if fn == nil {
		return nil, ErrUnsupported
	}
	count, err := fn(data, addPath, nil)
	if count == 0 {
		return nil, err
	}

	// The fill pass reads the same bytes and stops at the same NLRI, so it
	// visits exactly count entries and reaches the same error. Only the count
	// pass's error is kept, because two readings of one section that disagreed
	// would mean the walk is not a function of its input.
	result := make([][]byte, 0, count)
	_, _ = fn(data, addPath, func(nlri []byte) {
		result = append(result, nlri)
	})
	return result, err
}

// ResetForTest clears every registered splitter. Tests call this to start
// from a clean slate before registering their own fixtures. NOT for
// production use.
func ResetForTest() {
	mu.Lock()
	defer mu.Unlock()
	splitters = map[family.Family]Splitter{}
}
