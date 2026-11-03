// Design: plan/post-release/engineering/spec-filter-wire-0-umbrella.md -- D-5, set fields with an action
// Related: sets.go -- the Sets type these fields extend
// Related: chain_overlay_attributes.go -- the read-after-set overlays of these fields
//
// The attribute fields a modify filter changes beyond LOCAL_PREF, MED and
// COMMUNITY: ORIGIN and NEXT_HOP replaced, AIGP set, incremented or
// decremented, LARGE_COMMUNITY and EXTENDED_COMMUNITIES values added or
// removed, and MULTI_EXIT_DISC removed on import. Each is written as the same
// ModAccumulator operation, so one apply engine writes every change.

package policychain

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// errMEDRemoveOnExport refuses a MULTI_EXIT_DISC removal answered on an
// export chain.
//
// RFC 4271 Section 5.1.4: "If a BGP speaker is configured to remove the
// MULTI_EXIT_DISC attribute from a route, then this removal MUST be done prior
// to determining the degree of preference of the route and prior to performing
// route selection (Decision Process phases 1 and 2)."
var errMEDRemoveOnExport = errors.New("med remove is honored on an import chain only (RFC 4271 Section 5.1.4)")

// valueList is the values one Sets adds to and removes from one list
// attribute, each side bounded by communitiesMax.
type valueList[T comparable] struct {
	add, remove   [communitiesMax]T
	addN, removeN uint8
}

func (l *valueList[T]) addValue(value T) error {
	if int(l.addN) == communitiesMax {
		return ErrTooManyCommunities
	}
	l.add[l.addN] = value
	l.addN++
	return nil
}

func (l *valueList[T]) removeValue(value T) error {
	if int(l.removeN) == communitiesMax {
		return ErrTooManyCommunities
	}
	l.remove[l.removeN] = value
	l.removeN++
	return nil
}

// removes reports whether l removes value.
func (l *valueList[T]) removes(value T) bool {
	return slices.Contains(l.remove[:l.removeN], value)
}

// merge folds a later member's list onto l as Sets.merge folds COMMUNITY: a
// later remove cancels an earlier add, and the later adds come after.
func (l *valueList[T]) merge(later *valueList[T]) error {
	for _, value := range later.remove[:later.removeN] {
		kept := uint8(0)
		for _, added := range l.add[:l.addN] {
			if added != value {
				l.add[kept] = added
				kept++
			}
		}
		l.addN = kept
		if err := l.removeValue(value); err != nil {
			return err
		}
	}
	for _, value := range later.add[:later.addN] {
		if err := l.addValue(value); err != nil {
			return err
		}
	}
	return nil
}

// Origin records an ORIGIN replacement (RFC 4271 Section 5.1.1).
func (s *Sets) Origin(origin attribute.Origin) error {
	if origin > attribute.OriginIncomplete {
		return fmt.Errorf("origin %d is not IGP, EGP or INCOMPLETE", origin)
	}
	s.origin = origin
	s.written |= setKindOrigin.bit()
	return nil
}

// NextHop records a NEXT_HOP replacement (RFC 4271 Section 5.1.3). NEXT_HOP
// carries an IPv4 address only.
func (s *Sets) NextHop(addr netip.Addr) error {
	if !addr.Is4() {
		return fmt.Errorf("next-hop %s is not an IPv4 address", addr)
	}
	s.nextHop = addr.As4()
	s.written |= setKindNextHop.bit()
	return nil
}

// AIGP records an AIGP metric change (RFC 7311 Section 3).
func (s *Sets) AIGP(action NumericAction, value uint32) error {
	if err := s.aigp.fold(action, value); err != nil {
		return err
	}
	if action == NumericActionSet {
		s.aigp.metric = uint64(value)
	}
	s.written |= setKindAIGP.bit()
	return nil
}

// RemoveMED records the configured MULTI_EXIT_DISC removal (RFC 4271 Section
// 5.1.4). A later MED set in the same member replaces it.
func (s *Sets) RemoveMED() {
	s.med = numericSet{remove: true}
	s.written |= setKindMED.bit()
}

// AddLargeCommunity records one RFC 8092 large community to add.
func (s *Sets) AddLargeCommunity(community attribute.LargeCommunity) error {
	if err := s.large.addValue(community); err != nil {
		return err
	}
	s.written |= setKindLargeCommunity.bit()
	return nil
}

// RemoveLargeCommunity records one RFC 8092 large community to remove.
func (s *Sets) RemoveLargeCommunity(community attribute.LargeCommunity) error {
	if err := s.large.removeValue(community); err != nil {
		return err
	}
	s.written |= setKindLargeCommunity.bit()
	return nil
}

// AddExtendedCommunity records one RFC 4360 extended community to add.
func (s *Sets) AddExtendedCommunity(community attribute.ExtendedCommunity) error {
	if err := s.extended.addValue(community); err != nil {
		return err
	}
	s.written |= setKindExtendedCommunity.bit()
	return nil
}

// RemoveExtendedCommunity records one RFC 4360 extended community to remove.
func (s *Sets) RemoveExtendedCommunity(community attribute.ExtendedCommunity) error {
	if err := s.extended.removeValue(community); err != nil {
		return err
	}
	s.written |= setKindExtendedCommunity.bit()
	return nil
}

// mergeAttributes folds a later member's attribute sets onto s: a later
// replacement wins, AIGP folds as the numeric fields do, and the two lists
// fold as COMMUNITY does.
func (s *Sets) mergeAttributes(later *Sets) error {
	if later.written&setKindOrigin.bit() != 0 {
		s.origin = later.origin
	}
	if later.written&setKindNextHop.bit() != 0 {
		s.nextHop = later.nextHop
	}
	if later.written&setKindAIGP.bit() != 0 {
		s.aigp = foldNumeric(s.aigp, later.aigp)
	}
	if err := s.large.merge(&later.large); err != nil {
		return err
	}
	return s.extended.merge(&later.extended)
}

// resolveAIGP answers the AIGP metric set gives a route whose metric is
// current (present when it carries one). An increment or decrement of an
// absent metric does not run, because AIGP has no configured base, and
// answers false; the result is clamped to the eight-octet range.
func resolveAIGP(set numericSet, current uint64, present bool) (uint64, bool) {
	start := current
	switch {
	case set.replace:
		start = set.metric
	case !present || set.absent:
		return 0, false
	}
	if set.delta >= 0 {
		if start > math.MaxUint64-uint64(set.delta) {
			return math.MaxUint64, true
		}
		return start + uint64(set.delta), true
	}
	if uint64(-set.delta) >= start {
		return 0, true
	}
	return start - uint64(-set.delta), true
}

// applyAttributesTo writes the attribute sets as ModAccumulator operations,
// which the reactor's registered attribute handlers apply to the wire.
func (s *Sets) applyAttributesTo(acc *filterapi.ModAccumulator, current Current) {
	if s.written&setKindOrigin.bit() != 0 {
		// RFC 4271 Section 4.3: "ORIGIN is a well-known mandatory attribute that defines the origin of the path information."
		acc.OpCopy(uint8(attribute.AttrOrigin), filterapi.AttrModSet, []byte{byte(s.origin)})
	}
	if s.written&setKindNextHop.bit() != 0 {
		// RFC 4271 Section 5.1.3: "The NEXT_HOP is a well-known mandatory attribute that defines the IP address of the router that SHOULD be used as the next hop to the destinations listed in the UPDATE message."
		acc.OpCopy(uint8(attribute.AttrNextHop), filterapi.AttrModSet, s.nextHop[:])
	}
	if s.written&setKindAIGP.bit() != 0 {
		if metric, run := resolveAIGP(s.aigp, current.AIGP, current.HasAIGP); run {
			var value [attribute.AIGPWireLen]byte
			attribute.WriteAIGPMetric(value[:], 0, metric)
			acc.OpCopy(uint8(attribute.AttrAIGP), filterapi.AttrModSet, value[:])
		}
	}
	var large [12]byte
	for _, community := range s.large.add[:s.large.addN] {
		acc.OpCopy(uint8(attribute.AttrLargeCommunity), filterapi.AttrModAdd, largeWire(&large, community))
	}
	for _, community := range s.large.remove[:s.large.removeN] {
		acc.OpCopy(uint8(attribute.AttrLargeCommunity), filterapi.AttrModRemove, largeWire(&large, community))
	}
	for _, community := range s.extended.add[:s.extended.addN] {
		acc.OpCopy(uint8(attribute.AttrExtCommunity), filterapi.AttrModAdd, community[:])
	}
	for _, community := range s.extended.remove[:s.extended.removeN] {
		acc.OpCopy(uint8(attribute.AttrExtCommunity), filterapi.AttrModRemove, community[:])
	}
}

// largeWire writes community in its twelve-octet wire form into buf.
//
// RFC 8092 Section 3: "Each BGP Large Community value is encoded as a 12-octet quantity, as follows:"
func largeWire(buf *[12]byte, community attribute.LargeCommunity) []byte {
	binary.BigEndian.PutUint32(buf[0:], community.GlobalAdmin)
	binary.BigEndian.PutUint32(buf[4:], community.LocalData1)
	binary.BigEndian.PutUint32(buf[8:], community.LocalData2)
	return buf[:]
}

// appendAttributesText appends the attribute sets in the text filter answer's
// directive words, for AppendText.
func (s *Sets) appendAttributesText(buf []byte) []byte {
	if s.written&setKindOrigin.bit() != 0 {
		buf = append(appendWord(buf, "origin "), s.origin.String()...)
	}
	if s.written&setKindNextHop.bit() != 0 {
		buf = netip.AddrFrom4(s.nextHop).AppendTo(appendWord(buf, "next-hop "))
	}
	if s.written&setKindAIGP.bit() != 0 {
		buf = appendNumericText(buf, "aigp", s.aigp)
	}
	for _, community := range s.large.remove[:s.large.removeN] {
		buf = appendLargeText(appendWord(buf, "large-community remove"), community)
	}
	for _, community := range s.large.add[:s.large.addN] {
		buf = appendLargeText(appendWord(buf, "large-community add"), community)
	}
	for _, community := range s.extended.remove[:s.extended.removeN] {
		buf = appendHexText(appendWord(buf, "extended-community remove"), community[:])
	}
	for _, community := range s.extended.add[:s.extended.addN] {
		buf = appendHexText(appendWord(buf, "extended-community add"), community[:])
	}
	return buf
}

// appendLargeText appends " global:local1:local2", RFC 8092's display form.
func appendLargeText(buf []byte, community attribute.LargeCommunity) []byte {
	buf = strconv.AppendUint(append(buf, ' '), uint64(community.GlobalAdmin), 10)
	buf = strconv.AppendUint(append(buf, ':'), uint64(community.LocalData1), 10)
	return strconv.AppendUint(append(buf, ':'), uint64(community.LocalData2), 10)
}

// appendHexText appends " 0x" and value in hexadecimal.
func appendHexText(buf, value []byte) []byte {
	const digits = "0123456789abcdef"
	buf = append(buf, " 0x"...)
	for _, octet := range value {
		buf = append(buf, digits[octet>>4], digits[octet&0x0f])
	}
	return buf
}
