// Design: rfc/short/rfc7606.md -- Section 2 treat-as-withdraw, per route
// RFC: rfc/short/draft-ietf-bess-mup-safi.md -- Section 3.3.6, the first family that registers a check
// Overview: nlritype.go -- the per-family recognizer registry this check sits beside
//
// A Recognizer answers about one NLRI alone: its type, and for some families its
// body. Some families' specifications also judge a route by the UPDATE carrying
// it. draft-ietf-bess-mup-safi Section 3.3.6 withdraws a Direct Segment Discovery
// route that arrives with no Prefix-SID attribute. The recognizer cannot answer
// that, because it never sees the path attributes, and giving it them would
// change its contract for every family.
//
// So the second ruling is a sibling registry: an UpdateCheck, registered per
// family by the plugin that owns the family, next to its Recognizer. Retain
// consults it only for an announced NLRI the recognizer kept. A family with no
// check is unaffected, and nothing on the receive path names a family.

package nlritype

import (
	"fmt"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// UpdateAttrs is the path attribute section an UpdateCheck reads, as the UPDATE
// will stand once RFC 7606 attribute-discard has run.
//
// The receive path judges the NLRIs before it rewrites the attribute section for
// attribute-discard, so a discarded attribute is still in the bytes here. A check
// that asked the bytes alone would see an attribute the UPDATE no longer carries.
// Discarding records those codes, and Has answers absent for them.
//
// The zero value holds no attributes. Not safe for concurrent mutation.
type UpdateAttrs struct {
	section    []byte
	discarding [256 / 8]byte // one bit per attribute type code
}

// NewUpdateAttrs wraps section, the UPDATE's path attribute section, without copying it.
func NewUpdateAttrs(section []byte) UpdateAttrs {
	return UpdateAttrs{section: section}
}

// Discarding records that RFC 7606 attribute-discard removes code from the UPDATE
// after the NLRIs are judged.
func (a *UpdateAttrs) Discarding(code attribute.AttributeCode) {
	a.discarding[code/8] |= 1 << (code % 8)
}

// Has reports whether the UPDATE carries an attribute of type code once
// attribute-discard has run.
func (a *UpdateAttrs) Has(code attribute.AttributeCode) bool {
	if a.discarding[code/8]&(1<<(code%8)) != 0 {
		return false
	}
	_, _, _, found := attribute.AttrFind(a.section, code)
	return found
}

// UpdateCheck judges one carved announced NLRI against the UPDATE carrying it.
// nlriBytes and addPath are as for Recognizer. It answers Keep, or Withdraw when
// the family's specification answers this route in this UPDATE with
// treat-as-withdraw. Retain calls it only for an NLRI the family's recognizer
// kept, so the NLRI's type is one ze implements.
type UpdateCheck func(nlriBytes []byte, addPath bool, attrs *UpdateAttrs) Verdict

var updateChecks = map[family.Family]UpdateCheck{}

// RegisterUpdateCheck installs fn as fam's UPDATE-level check.
//
// Returns ErrNoSplitter when fam has no registered nlrisplit splitter, for the
// reason Register gives, and ErrDuplicate on a second registration. A nil fn is a
// no-op.
func RegisterUpdateCheck(fam family.Family, fn UpdateCheck) error {
	if fn == nil {
		return nil
	}
	if !nlrisplit.Supported(fam) {
		return fmt.Errorf("%w: %s", ErrNoSplitter, fam)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := updateChecks[fam]; ok {
		return fmt.Errorf("%w: %s update check", ErrDuplicate, fam)
	}
	updateChecks[fam] = fn
	return nil
}

// Rulings returns fam's recognizer and UPDATE-level check from one registry read.
// Either is nil when the family has no such ruling.
func Rulings(fam family.Family) (Recognizer, UpdateCheck) {
	mu.RLock()
	defer mu.RUnlock()
	return recognizers[fam], updateChecks[fam]
}
