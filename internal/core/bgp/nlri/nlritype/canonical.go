// Design: docs/architecture/buffer-architecture.md -- When a copy is deliberate
// Related: nlritype.go -- RetainUpdate, which applies the canonical form to the routes it keeps
//
// A family can register the form Ze sends one of its routes in, for a route a peer
// sent in an earlier encoding of the family. Retain rewrites a kept route into that
// form on receipt, so the Adj-RIB-In, every relay and every event carry the one
// encoding. The MUP plugin registers the draft-ietf-bess-mup-safi -02 Type 1 ST
// route, written without its Source Address Length octet.

package nlritype

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/family"
)

// ErrNoRuling reports a canonical form for a family with no recognizer and no
// UPDATE-level check. Retain runs only for a family that has one of the two, so
// such a form would never be applied.
var ErrNoRuling = errors.New("nlritype: canonical form for a family with no recognizer or update check")

// Canonical is the form a family sends one received route in.
//
// Grow answers how many octets one carved NLRI grows by in that form, 0 when it is
// already in it; nlriBytes and addPath are as for Recognizer. WriteTo writes the
// NLRI in that form at buf[off:], len(nlriBytes) plus Grow octets, and returns the
// count. Retain calls WriteTo only for an NLRI Grow answered above 0, and only for a
// route the family's rulings kept, so it never sees a malformed one. Neither
// allocates.
type Canonical struct {
	Grow    func(nlriBytes []byte, addPath bool) int
	WriteTo func(buf []byte, off int, nlriBytes []byte, addPath bool) int
}

var canonicals = map[family.Family]Canonical{}

// RegisterCanonical installs c as fam's canonical form.
//
// Returns ErrNoRuling when fam has neither a recognizer nor an UPDATE-level check,
// so the plugin registers those first, and ErrDuplicate on a second registration.
// A c with a nil Grow or WriteTo is a no-op.
func RegisterCanonical(fam family.Family, c Canonical) error {
	if c.Grow == nil {
		return nil
	}
	if c.WriteTo == nil {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	_, recognized := recognizers[fam]
	_, checked := updateChecks[fam]
	if !recognized && !checked {
		return fmt.Errorf("%w: %s", ErrNoRuling, fam)
	}
	if _, ok := canonicals[fam]; ok {
		return fmt.Errorf("%w: %s canonical form", ErrDuplicate, fam)
	}
	canonicals[fam] = c
	return nil
}

// canonicalGrow is c.Grow for one NLRI, and 0 for a family with no canonical form.
func canonicalGrow(c Canonical, nlriBytes []byte, addPath bool) int {
	if c.Grow == nil {
		return 0
	}
	return c.Grow(nlriBytes, addPath)
}

// canonicalFor returns fam's canonical form, with a nil Grow when it has none.
func canonicalFor(fam family.Family) Canonical {
	mu.RLock()
	defer mu.RUnlock()
	return canonicals[fam]
}
