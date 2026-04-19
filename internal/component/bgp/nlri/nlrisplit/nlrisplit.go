// Design: plan/design-rib-unified.md -- Phase 3g (per-family NLRI split)
//
// Package nlrisplit holds a registry of family-specific NLRI splitters.
// Concatenated NLRI wire bytes arrive from the peer; each family has its
// own per-NLRI framing (CIDR-style [prefix-len][addr], EVPN's
// [route-type][length][body], flowspec's component tuple, ...). The BGP
// RIB dispatches through this registry so new families only need a
// splitter registration -- no edits to the RIB hot path.
//
// Registration is via init() in each NLRI plugin package. In-process and
// forked-plugin runs both import internal/component/plugin/all, so every
// registered splitter is available in either mode.
package nlrisplit

import (
	"errors"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// ErrUnsupported is returned by Split when no splitter is registered for
// the given family. Callers treat this as "drop the input" (the peer
// advertised a family we cannot parse).
var ErrUnsupported = errors.New("nlrisplit: no splitter registered for family")

// Splitter carves concatenated NLRI wire bytes into per-NLRI slices. Under
// ADD-PATH (RFC 7911) each NLRI is prefixed by a 4-byte path-id and the
// splitter includes those bytes in the returned slice so downstream
// consumers use the exact wire representation as their key.
//
// Slices returned alias the input data (zero-copy). Callers MUST copy if
// they need to retain bytes past the scope of the call.
//
// A well-formed empty input returns nil, nil (no NLRIs, no error). A
// malformed input returns any NLRIs successfully parsed before the
// corruption plus a non-nil error; callers choose whether to use the
// partial result.
type Splitter func(data []byte, addPath bool) ([][]byte, error)

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

// Split dispatches to the family's registered splitter. Returns
// ErrUnsupported when no splitter is registered; the input slice is
// unchanged in that case.
func Split(fam family.Family, data []byte, addPath bool) ([][]byte, error) {
	fn := Get(fam)
	if fn == nil {
		return nil, ErrUnsupported
	}
	return fn(data, addPath)
}

// ResetForTest clears every registered splitter. Tests call this to start
// from a clean slate before registering their own fixtures. NOT for
// production use.
func ResetForTest() {
	mu.Lock()
	defer mu.Unlock()
	splitters = map[family.Family]Splitter{}
}
