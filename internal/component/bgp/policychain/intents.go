// Design: docs/architecture/core-design.md -- the in-process BGP chain
// Related: sets.go -- Sets, which carries the intents beside the set fields
// RFC: rfc/short/rfc4271.md -- AS_PATH prepend (Section 5.1.2)
// RFC: rfc/short/rfc6996.md -- private-use ASN removal (Section 4)
// RFC: rfc/short/rfc4760.md -- MP_REACH_NLRI and MP_UNREACH_NLRI (Section 2)
//
// The named intents of D-5 are the changes a member asks that need state only
// the reactor holds: the local AS for a prepend, the peer AS and the AS
// number width for remove-private, the encoding context for an NLRI subset.
// A member names the intent; the reactor resolves it through the same wire
// cores the text delta reaches (reactor/filter_delta.go,
// reactor/filter_nlri_delta.go).

package policychain

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/core/family"
)

// ASPathPrependMax bounds the local AS copies one route's chain prepends, the
// range filter_modify's as-path-prepend leaf takes (ze-filter-modify.yang).
const ASPathPrependMax = 32

// familiesStripMax bounds the families one route strips: an UPDATE carries at
// most three, the body NLRI's IPv4 unicast, one MP_REACH_NLRI family and one
// MP_UNREACH_NLRI family.
const familiesStripMax = 3

// RemovePrivateMode is how remove-private treats a private-use AS number.
type RemovePrivateMode uint8

const (
	RemovePrivateUnspecified RemovePrivateMode = iota
	// RemovePrivateStrip removes the private AS number.
	RemovePrivateStrip
	// RemovePrivatePeerAS replaces it with the peer AS.
	RemovePrivatePeerAS
)

// NLRISection names one NLRI section of an UPDATE.
type NLRISection uint8

const (
	NLRISectionUnspecified NLRISection = iota
	// NLRISectionAnnounce is the body NLRI (RFC 4271 Section 4.3).
	NLRISectionAnnounce
	// NLRISectionWithdrawn is the body Withdrawn Routes (RFC 4271 Section 4.3).
	NLRISectionWithdrawn
	// NLRISectionMPReach is the NLRI of MP_REACH_NLRI (RFC 4760 Section 3).
	NLRISectionMPReach
	// NLRISectionMPUnreach is the Withdrawn Routes of MP_UNREACH_NLRI (RFC 4760 Section 4).
	NLRISectionMPUnreach
	nlriSectionCount // the number of sections plus one; never a section
)

// ErrTooManyFamilies is answered by StripFamily past familiesStripMax.
var ErrTooManyFamilies = errors.New("a filter may strip at most 3 families per route")

// intents is the named-intent half of a Sets. The zero value asks nothing.
type intents struct {
	prepend       uint8
	removePrivate RemovePrivateMode
	// drops holds, per section, the offset of each NLRI entry the chain drops,
	// within that section's bytes. The slices are reused across routes; their
	// length is bounded by the section, so by the UPDATE body.
	drops     [nlriSectionCount][]uint16
	families  [familiesStripMax]family.Family
	familiesN uint8
}

// PrependASPath asks count more copies of the local AS in front of the
// AS_PATH.
//
// RFC 4271 Section 5.1.2: "Whenever the modification of the AS_PATH attribute calls for
// including or prepending the AS number of the local system, the local
// system MAY include/prepend more than one instance of its own AS
// number in the AS_PATH attribute."
//
// The chain's prepends add up, and a total past ASPathPrependMax fails the
// route under the member's on-error.
func (s *Sets) PrependASPath(count uint8) error {
	if count == 0 {
		return errors.New("as-path prepend count 0: give 1 to 32")
	}
	if int(s.intents.prepend)+int(count) > ASPathPrependMax {
		return fmt.Errorf("as-path prepend of %d copies exceeds %d", int(s.intents.prepend)+int(count), ASPathPrependMax)
	}
	s.intents.prepend += count
	s.written |= setKindASPath.bit()
	return nil
}

// RemovePrivateAS asks private-use AS numbers out of AS_PATH and AS4_PATH.
// A later member's mode replaces an earlier one's.
//
// RFC 6996 Section 4: "Private Use ASNs MUST be removed from AS path attributes (including
// AS4_PATH if utilizing a four-octet AS number space) before being
// advertised to the global Internet."
func (s *Sets) RemovePrivateAS(mode RemovePrivateMode) error {
	switch mode {
	case RemovePrivateStrip, RemovePrivatePeerAS:
		s.intents.removePrivate = mode
		s.written |= setKindASPath.bit()
		return nil
	case RemovePrivateUnspecified:
	}
	return fmt.Errorf("remove-private mode %d is neither strip nor peer-as", mode)
}

// DropNLRI asks the NLRI entry at offset of section out of the UPDATE. offset
// is where the entry starts in the section's bytes as the view answers them,
// path identifier included; an offset that starts no entry fails the route
// when the reactor applies the sets.
func (s *Sets) DropNLRI(section NLRISection, offset uint16) error {
	if section == NLRISectionUnspecified || section >= nlriSectionCount {
		return fmt.Errorf("NLRI section %d is not a section of an UPDATE", section)
	}
	s.intents.drops[section] = append(s.intents.drops[section], offset)
	s.written |= setKindPrefix.bit()
	return nil
}

// StripFamily asks every NLRI section of fam out of the UPDATE: the body
// sections for IPv4 unicast, the MP_REACH_NLRI and MP_UNREACH_NLRI attributes
// that carry it for any family.
//
// RFC 4760 Section 2: "The first one (MP_REACH_NLRI) is used to carry the set of reachable
// destinations together with the next hop information to be used for
// forwarding to these destinations.  The second one (MP_UNREACH_NLRI)
// is used to carry the set of unreachable destinations."
func (s *Sets) StripFamily(fam family.Family) error {
	if slices.Contains(s.intents.families[:s.intents.familiesN], fam) {
		return nil
	}
	if int(s.intents.familiesN) == familiesStripMax {
		return ErrTooManyFamilies
	}
	s.intents.families[s.intents.familiesN] = fam
	s.intents.familiesN++
	s.written |= setKindFamily.bit()
	return nil
}

// ASPathPrepend answers how many local AS copies the sets prepend, 0 for none.
func (s *Sets) ASPathPrepend() uint8 { return s.intents.prepend }

// RemovePrivate answers the remove-private mode, Unspecified for none.
func (s *Sets) RemovePrivate() RemovePrivateMode { return s.intents.removePrivate }

// NLRIDrops answers the offsets the sets drop from section, ascending and
// each once. It sorts the stored list in place, so it is valid until the next
// change to s.
func (s *Sets) NLRIDrops(section NLRISection) []uint16 {
	if section == NLRISectionUnspecified || section >= nlriSectionCount {
		return nil
	}
	drops := s.intents.drops[section]
	slices.Sort(drops)
	drops = slices.Compact(drops)
	s.intents.drops[section] = drops
	return drops
}

// StripsFamily reports whether the sets strip fam.
func (s *Sets) StripsFamily(fam family.Family) bool {
	return slices.Contains(s.intents.families[:s.intents.familiesN], fam)
}

// merge folds a later member's intents onto i: prepends add up, a later
// remove-private mode replaces an earlier one, drops and stripped families
// accumulate.
func (i *intents) merge(later *intents) error {
	if later.prepend > 0 {
		if int(i.prepend)+int(later.prepend) > ASPathPrependMax {
			return fmt.Errorf("as-path prepend of %d copies exceeds %d", int(i.prepend)+int(later.prepend), ASPathPrependMax)
		}
		i.prepend += later.prepend
	}
	if later.removePrivate != RemovePrivateUnspecified {
		i.removePrivate = later.removePrivate
	}
	for section := range later.drops {
		i.drops[section] = append(i.drops[section], later.drops[section]...)
	}
	for _, fam := range later.families[:later.familiesN] {
		if slices.Contains(i.families[:i.familiesN], fam) {
			continue
		}
		if int(i.familiesN) == familiesStripMax {
			return ErrTooManyFamilies
		}
		i.families[i.familiesN] = fam
		i.familiesN++
	}
	return nil
}
