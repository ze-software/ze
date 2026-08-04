// Design: rfc/short/rfc7606.md -- Section 5.4, typed NLRI
// RFC: rfc/short/rfc7606.md -- Section 5.4 discard of unrecognized NLRI types
// Related: ../nlrisplit/nlrisplit.go -- the per-family framing this registry carves with
//
// Package nlritype holds a registry of per-family NLRI route-type recognizers.
//
// RFC 7606 Section 5.4: "A BGP speaker advertising support for such a typed
// address family MUST handle routes with unrecognized NLRI types within that
// address family by discarding them, unless the relevant specification for that
// address family specifies otherwise."
//
// The escape clause is the whole design. It is per family, and at least one
// family uses it: RFC 9552 Section 5.2 deviates from Section 5.4 for BGP-LS and
// requires unknown Link-State NLRI types to be preserved and propagated. A
// blanket discard would violate that MUST. So the ruling is registered per
// family by the plugin that owns the family, and a family nobody has ruled on
// discards nothing.
//
// Registration is via init() in the owning NLRI plugin, beside that plugin's
// family.MustRegister call. The recognizer's presence therefore tracks the
// family's advertisement: compile the plugin out and ze neither advertises the
// family nor owes Section 5.4 for it.
package nlritype

import (
	"errors"
	"fmt"
	"maps"
	"sync"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// Registration errors.
var (
	// ErrNoSplitter reports a recognizer for a family whose NLRI section cannot
	// be carved into individual NLRIs.
	ErrNoSplitter = errors.New("nlritype: no NLRI splitter registered for family")
	// ErrDuplicate reports a second ruling for a family that already has one.
	ErrDuplicate = errors.New("nlritype: duplicate recognizer for family")
)

// Recognizer reports whether one carved NLRI carries a route type this speaker
// implements. nlriBytes is a single NLRI exactly as it appears on the wire; when
// addPath is true it starts with the 4-byte RFC 7911 path identifier, which the
// recognizer must skip before reading the type.
//
// A recognizer answers about the TYPE only. It never validates the body: a
// well-typed but malformed NLRI is a Section 5.3 concern, not a Section 5.4 one.
type Recognizer func(nlriBytes []byte, addPath bool) bool

var (
	mu          sync.RWMutex
	recognizers = map[family.Family]Recognizer{}
)

// Register installs fn as Section 5.4's recognizer for fam.
//
// Returns ErrNoSplitter when fam has no registered nlrisplit splitter. Judging
// route types requires carving the NLRI section into individual NLRIs, so a
// recognizer without a splitter is a contradiction with no permitted runtime
// resolution: failing open would violate Section 5.4, and failing closed would
// discard every route in the family. Reporting it here makes it a startup
// failure the caller must act on rather than a silent wire-visible one. Package
// initialization order guarantees the splitter is present first, because this
// package imports nlrisplit.
//
// Returns ErrDuplicate on a second registration. A family has one ruling.
// A nil fn is a no-op, so a family whose specification overrides Section 5.4 can
// say so at its own registration site without a special case here.
func Register(fam family.Family, fn Recognizer) error {
	if fn == nil {
		return nil
	}
	if !nlrisplit.Supported(fam) {
		return fmt.Errorf("%w: %s", ErrNoSplitter, fam)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := recognizers[fam]; ok {
		return fmt.Errorf("%w: %s", ErrDuplicate, fam)
	}
	recognizers[fam] = fn
	return nil
}

// Get returns the registered recognizer for fam, or nil when no ruling exists.
func Get(fam family.Family) Recognizer {
	mu.RLock()
	defer mu.RUnlock()
	return recognizers[fam]
}

// Retain returns data with every unrecognized-type NLRI removed, and the number
// removed.
//
// fn is the family's recognizer, which the caller has already looked up with Get.
// Passing it rather than looking it up again is what lets the receive path answer
// "does Section 5.4 bind this family at all" and "which routes survive" from ONE
// registry read. A nil fn means no ruling, and the answer is to change nothing.
//
// It returns data itself, sharing the caller's backing array, whenever nothing
// was removed. That covers every family with no ruling, every conforming UPDATE,
// and every malformed section, so the caller allocates and rewrites the wire
// only when a route is genuinely being discarded.
//
// A non-nil error means the NLRI framing could not be trusted, and data is
// returned unchanged with a zero count. When the length fields do not agree with
// the section, the boundaries between NLRIs are unknowable, so no discard
// decision can be made and inventing one would rewrite the wire from a guess.
// Framing errors belong to RFC 7606 Section 5.3, which runs before this.
func Retain(fn Recognizer, fam family.Family, data []byte, addPath bool) (kept []byte, dropped int, err error) {
	if len(data) == 0 || fn == nil {
		return data, 0, nil
	}

	nlris, splitErr := nlrisplit.Split(fam, data, addPath)
	if splitErr != nil {
		return data, 0, splitErr
	}

	// One pass to count, so the common "nothing dropped" answer costs no
	// allocation and the rewrite below is sized exactly.
	keepBytes := 0
	for _, one := range nlris {
		if fn(one, addPath) {
			keepBytes += len(one)
			continue
		}
		dropped++
	}
	if dropped == 0 {
		return data, 0, nil
	}
	if keepBytes == 0 {
		return nil, dropped, nil
	}

	out := make([]byte, 0, keepBytes)
	for _, one := range nlris {
		if fn(one, addPath) {
			out = append(out, one...)
		}
	}
	return out, dropped, nil
}

// ResetForTest clears every registered recognizer. Tests call it to start from a
// clean slate. NOT for production use.
//
// Prefer SnapshotForTest in a package whose test binary links real NLRI plugins: this
// leaves the registry EMPTY, so a later test in the same binary finds no recognizer for a
// family the daemon really does rule on, its Section 5.4 filter silently does nothing, and
// the test passes proving nothing.
func ResetForTest() {
	mu.Lock()
	defer mu.Unlock()
	recognizers = map[family.Family]Recognizer{}
}

// SnapshotForTest empties the registry and returns a function that puts back exactly what
// was registered. NOT for production use.
//
// Registration happens in plugin init(), so what a test binary starts with depends on which
// plugins it links, and that changes when an unrelated file adds an import. Restoring is the
// only teardown that cannot leave the next test in a state no daemon is ever in.
//
// Two limits, both deliberate and neither hidden. It does not NEST: a second call inside one
// test empties the registry again, so the first caller's registrations are gone for the rest
// of that test (the restores themselves are correct, since t.Cleanup runs last-in-first-out).
// And it is not safe under t.Parallel: two overlapping snapshots make the second save an
// already-empty map, and restoring it loses what the plugins registered. Register everything
// one test needs through ONE call, and do not mark such a test parallel.
func SnapshotForTest() func() {
	mu.Lock()
	defer mu.Unlock()
	saved := make(map[family.Family]Recognizer, len(recognizers))
	maps.Copy(saved, recognizers)
	recognizers = map[family.Family]Recognizer{}
	return func() {
		mu.Lock()
		defer mu.Unlock()
		recognizers = saved
	}
}
