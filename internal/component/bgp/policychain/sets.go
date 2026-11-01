// Design: plan/post-release/engineering/spec-filter-wire-0-umbrella.md -- D-5, set fields with an action
// Related: chain.go -- the chain run that merges each member's sets
//
// A chain member answers what it changes as set fields of the route-policy
// vocabulary, each with an action, never as a rewritten UPDATE. The chain
// folds the members' sets in order, and the reactor turns the result into
// ModAccumulator operations, so one apply engine (buildModifiedPayload)
// writes every change.

package policychain

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// communitiesMax bounds the communities one Sets adds, and separately the
// ones it removes. A member that asks for more fails the route under its
// on-error rather than growing a per-route allocation.
const communitiesMax = 16

// ErrTooManyCommunities is answered by AddCommunity and RemoveCommunity past
// communitiesMax.
var ErrTooManyCommunities = errors.New("a filter may add or remove at most 16 communities per route")

// setKind is one settable field the Sets type can carry. The zero kind names
// none, so a kind is always one of the constants.
type setKind uint8

const (
	setKindUnspecified setKind = iota
	setKindLocalPreference
	setKindMED
	setKindCommunity
	setKindASPath // the AS-path intents: prepend and remove-private
	setKindPrefix // the NLRI subset intent
	setKindFamily // the family-strip intent
	setKindCount  // the number of kinds plus one; never a kind
)

// setKindFields is the vocabulary field each kind writes. The chain compiles a
// member's declared sets into a mask over these kinds, so a run checks a
// member's writes against what it may set with one AND (D-9).
var setKindFields = [setKindCount]policy.Field{
	setKindLocalPreference: policy.FieldLocalPreference,
	setKindMED:             policy.FieldMED,
	setKindCommunity:       policy.FieldCommunity,
	setKindASPath:          policy.FieldASPath,
	setKindPrefix:          policy.FieldPrefix,
	setKindFamily:          policy.FieldFamily,
}

// intentsOfReads answers the per-route intents a member that reads fields may
// answer (D-5): an NLRI subset for a reader of prefix, a family strip for a
// reader of family. Each drops only what the member was shown, so it changes
// no field: neither needs a set declaration, nor the attachment to allow
// setting prefix or family.
func intentsOfReads(reads []policy.Field) setMask {
	var mask setMask
	for _, field := range reads {
		switch field {
		case policy.FieldPrefix:
			mask |= setKindPrefix.bit()
		case policy.FieldFamily:
			mask |= setKindFamily.bit()
		}
	}
	return mask
}

// setMask holds one bit per setKind.
type setMask uint8

func (k setKind) bit() setMask { return 1 << k }

// maskOf answers the kinds whose field is in fields. A field no kind carries
// adds nothing: it is a field this type cannot hold, so no member can write it
// through Sets.
func maskOf(fields []policy.Field) setMask {
	var mask setMask
	for kind := setKindLocalPreference; kind < setKindCount; kind++ {
		for _, field := range fields {
			if field == setKindFields[kind] {
				mask |= kind.bit()
			}
		}
	}
	return mask
}

// NumericAction is what a member does to a numeric attribute (D-5).
type NumericAction uint8

const (
	NumericActionUnspecified NumericAction = iota
	NumericActionSet
	NumericActionIncrement
	NumericActionDecrement
)

// numericSet is the folded change to one numeric attribute: replace it with
// base when replace is set, then add delta. Folding keeps a later member's set
// overriding an earlier increment, and an increment after a set adding to it.
type numericSet struct {
	replace bool
	base    uint32
	delta   int64
}

// fold applies one more action on top of what the earlier members asked.
func (n *numericSet) fold(action NumericAction, value uint32) error {
	switch action {
	case NumericActionSet:
		n.replace, n.base, n.delta = true, value, 0
		return nil
	case NumericActionIncrement:
		n.delta += int64(value)
		return nil
	case NumericActionDecrement:
		n.delta -= int64(value)
		return nil
	case NumericActionUnspecified:
	}
	return fmt.Errorf("BUG: policychain: numeric action %d is not one of set, increment, decrement", action)
}

// resolve answers the attribute's new value. current is the route's value and
// present whether it carries one; absentBase is the configured base an
// arithmetic action on an absent attribute starts from (bgp defaults
// attribute), with hasBase false when none is configured. The result is
// clamped to the attribute's range: a decrement never wraps to 4294967295.
func (n numericSet) resolve(current uint32, present bool, absentBase uint32, hasBase bool) (uint32, error) {
	start := current
	switch {
	case n.replace:
		start = n.base
	case present:
	case hasBase:
		start = absentBase
	default:
		return 0, errors.New("an increment or decrement of an absent attribute needs a base: set bgp defaults attribute")
	}
	result := int64(start) + n.delta
	if result < 0 {
		return 0, nil
	}
	if result > math.MaxUint32 {
		return math.MaxUint32, nil
	}
	return uint32(result), nil
}

// Sets is the change one member, or the folded chain, asks of a route.
// The zero Sets asks nothing. NOT safe for concurrent use: one chain run
// writes it.
type Sets struct {
	written         setMask
	localPreference numericSet
	med             numericSet
	communityAdd    [communitiesMax]uint32
	communityRemove [communitiesMax]uint32
	communityAddN   uint8
	communityRemN   uint8
	intents         intents
}

// Reset forgets every change, so one Sets serves the next member or route.
// The NLRI drop lists keep their storage, so a reused Sets allocates only
// when a route drops more NLRI than any route before it.
func (s *Sets) Reset() {
	drops := s.intents.drops
	*s = Sets{}
	for section := range drops {
		s.intents.drops[section] = drops[section][:0]
	}
}

// Empty reports that the Sets asks no change.
func (s *Sets) Empty() bool { return s.written == 0 }

// LocalPreference records a LOCAL_PREF change (RFC 4271 Section 5.1.5).
func (s *Sets) LocalPreference(action NumericAction, value uint32) error {
	if err := s.localPreference.fold(action, value); err != nil {
		return err
	}
	s.written |= setKindLocalPreference.bit()
	return nil
}

// MED records a MULTI_EXIT_DISC change (RFC 4271 Section 5.1.4).
func (s *Sets) MED(action NumericAction, value uint32) error {
	if err := s.med.fold(action, value); err != nil {
		return err
	}
	s.written |= setKindMED.bit()
	return nil
}

// AddCommunity records one RFC 1997 community to add.
func (s *Sets) AddCommunity(community uint32) error {
	if int(s.communityAddN) == communitiesMax {
		return ErrTooManyCommunities
	}
	s.communityAdd[s.communityAddN] = community
	s.communityAddN++
	s.written |= setKindCommunity.bit()
	return nil
}

// RemoveCommunity records one RFC 1997 community to remove.
func (s *Sets) RemoveCommunity(community uint32) error {
	if int(s.communityRemN) == communitiesMax {
		return ErrTooManyCommunities
	}
	s.communityRemove[s.communityRemN] = community
	s.communityRemN++
	s.written |= setKindCommunity.bit()
	return nil
}

// merge folds a later member's sets onto s: numeric sets fold in order, and
// community additions and removals accumulate, a later remove cancelling an
// earlier add.
func (s *Sets) merge(later *Sets) error {
	if later.written&setKindLocalPreference.bit() != 0 {
		s.localPreference = foldNumeric(s.localPreference, later.localPreference)
	}
	if later.written&setKindMED.bit() != 0 {
		s.med = foldNumeric(s.med, later.med)
	}
	for _, community := range later.communityRemove[:later.communityRemN] {
		// A later remove cancels an earlier add: the handler removes from the
		// source alone and appends every add, so the add would survive it.
		s.cancelCommunityAdd(community)
		if err := s.RemoveCommunity(community); err != nil {
			return err
		}
	}
	// The later member's own adds come after its removes, as the handler
	// applies one member's operations: a value it both removes and adds stays.
	for _, community := range later.communityAdd[:later.communityAddN] {
		if err := s.AddCommunity(community); err != nil {
			return err
		}
	}
	if err := s.intents.merge(&later.intents); err != nil {
		return err
	}
	s.written |= later.written
	return nil
}

// foldNumeric answers earlier followed by later.
func foldNumeric(earlier, later numericSet) numericSet {
	if later.replace {
		return later
	}
	earlier.delta += later.delta
	return earlier
}

// Current is what ApplyTo needs to know about the route beyond the sets: the
// numeric attributes' present values and the configured absent bases.
type Current struct {
	LocalPreference, MED               uint32
	HasLocalPreference, HasMED         bool
	LocalPreferenceBase, MEDBase       uint32
	HasLocalPreferenceBase, HasMEDBase bool
}

// ApplyTo writes the sets as ModAccumulator operations, one per attribute
// value, through the same handlers the text delta reaches
// (attrModHandlersWithDefaults): a numeric set replaces LOCAL_PREF or MED, a
// community add or remove names one 4-octet value. It allocates nothing: the
// values are copied into the accumulator's inline arena.
func (s *Sets) ApplyTo(acc *filterapi.ModAccumulator, current Current) error {
	var value [4]byte
	if s.written&setKindLocalPreference.bit() != 0 {
		v, err := s.localPreference.resolve(current.LocalPreference, current.HasLocalPreference,
			current.LocalPreferenceBase, current.HasLocalPreferenceBase)
		if err != nil {
			return fmt.Errorf("local-preference: %w", err)
		}
		// RFC 4271 Section 4.3: "LOCAL_PREF is a well-known attribute that is a four-octet unsigned integer."
		binary.BigEndian.PutUint32(value[:], v)
		acc.OpCopy(uint8(attribute.AttrLocalPref), filterapi.AttrModSet, value[:])
	}
	if s.written&setKindMED.bit() != 0 {
		v, err := s.med.resolve(current.MED, current.HasMED, current.MEDBase, current.HasMEDBase)
		if err != nil {
			return fmt.Errorf("med: %w", err)
		}
		// RFC 4271 Section 5.1.4: "The value of the MULTI_EXIT_DISC attribute is a four-octet unsigned number, called a metric."
		binary.BigEndian.PutUint32(value[:], v)
		acc.OpCopy(uint8(attribute.AttrMED), filterapi.AttrModSet, value[:])
	}
	for _, community := range s.communityAdd[:s.communityAddN] {
		binary.BigEndian.PutUint32(value[:], community)
		acc.OpCopy(uint8(attribute.AttrCommunity), filterapi.AttrModAdd, value[:])
	}
	for _, community := range s.communityRemove[:s.communityRemN] {
		binary.BigEndian.PutUint32(value[:], community)
		acc.OpCopy(uint8(attribute.AttrCommunity), filterapi.AttrModRemove, value[:])
	}
	return nil
}

// removesCommunity reports whether s removes community.
func (s *Sets) removesCommunity(community uint32) bool {
	for _, removed := range s.communityRemove[:s.communityRemN] {
		if removed == community {
			return true
		}
	}
	return false
}

// cancelCommunityAdd forgets every add of community, keeping the others in
// order.
func (s *Sets) cancelCommunityAdd(community uint32) {
	kept := uint8(0)
	for _, added := range s.communityAdd[:s.communityAddN] {
		if added != community {
			s.communityAdd[kept] = added
			kept++
		}
	}
	s.communityAddN = kept
}
