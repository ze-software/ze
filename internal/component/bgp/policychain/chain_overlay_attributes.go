// Design: docs/architecture/core-design.md -- The in-process BGP chain, a later member reads an earlier set
// Related: sets_attributes.go -- the attribute sets these overlays show a later member
// Related: chain.go -- overlayNumeric and overlayCommunity, the same overlays for the first fields

package policychain

import (
	"encoding/binary"
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// attributeWire holds the values the attribute overlays answer a later member
// for the rest of a run. The list attributes alternate two buffers, because
// each overlay is built from the one before it; each keeps its storage across
// runs.
type attributeWire struct {
	origin    [1]byte
	nextHop   [4]byte
	aigp      [attribute.AIGPWireLen]byte
	large     [2][]byte
	largeNext uint8
	ext       [2][]byte
	extNext   uint8
}

// compileAttributeReaders resolves the readers the attribute overlays use.
func (c *Chain) compileAttributeReaders() error {
	for _, bind := range [...]struct {
		reader *policyview.Reader
		field  policy.Field
	}{
		{&c.origin, policy.FieldOrigin}, {&c.nextHop, policy.FieldNextHop}, {&c.aigp, policy.FieldAIGP},
		{&c.large, policy.FieldLargeCommunity}, {&c.extended, policy.FieldExtendedCommunity},
	} {
		reader, err := policyview.ReaderFor(bind.field)
		if err != nil {
			return fmt.Errorf("BUG: policychain: %w", err)
		}
		*bind.reader = reader
	}
	return nil
}

// overlayAttributes overlays the view with what the member's attribute sets
// make of the route, so a later member reads the set value (read-after-set),
// as overlayNumeric and overlayCommunity do for the first fields.
func (c *Chain) overlayAttributes(view *policyview.View, sets *RunSets) error {
	member := &sets.member
	wire := &sets.attributes
	if member.written&setKindOrigin.bit() != 0 {
		wire.origin[0] = byte(member.origin)
		if err := view.SetWireNumber(c.origin, wire.origin[:], uint64(member.origin)); err != nil {
			return fmt.Errorf("origin: %w", err)
		}
	}
	if member.written&setKindNextHop.bit() != 0 {
		wire.nextHop = member.nextHop
		if err := view.SetWire(c.nextHop, wire.nextHop[:]); err != nil {
			return fmt.Errorf("next-hop: %w", err)
		}
	}
	if err := c.overlayAIGP(view, member, wire); err != nil {
		return fmt.Errorf("aigp: %w", err)
	}
	if member.written&setKindLargeCommunity.bit() != 0 {
		out, err := overlayList(view, c.large, &wire.large, &wire.largeNext, &member.large, 12, largeFrom, largeAppend)
		if err != nil {
			return fmt.Errorf("large-community: %w", err)
		}
		if err := view.SetWire(c.large, out); err != nil {
			return fmt.Errorf("large-community: %w", err)
		}
	}
	if member.written&setKindExtendedCommunity.bit() != 0 {
		out, err := overlayList(view, c.extended, &wire.ext, &wire.extNext, &member.extended, 8, extendedFrom, extendedAppend)
		if err != nil {
			return fmt.Errorf("extended-community: %w", err)
		}
		if err := view.SetWire(c.extended, out); err != nil {
			return fmt.Errorf("extended-community: %w", err)
		}
	}
	return nil
}

// overlayAIGP resolves the member's AIGP set against the metric the view
// answers now and records it as a plain set, as overlayNumeric records a
// numeric set. A set that does not run (an increment of an absent metric,
// which has no configured base) leaves the route alone and is forgotten, so
// the folded sets carry only what applies.
func (c *Chain) overlayAIGP(view *policyview.View, member *Sets, wire *attributeWire) error {
	if member.written&setKindAIGP.bit() == 0 {
		return nil
	}
	current, err := view.Read(c.aigp)
	if err != nil {
		return err
	}
	number, numbered := current.Number()
	metric, run := resolveAIGP(member.aigp, number, current.Present() && numbered)
	if !run {
		member.written &^= setKindAIGP.bit()
		member.aigp = numericSet{}
		return nil
	}
	attribute.WriteAIGPMetric(wire.aigp[:], 0, metric)
	member.aigp = numericSet{replace: true, metric: metric}
	return view.SetWireNumber(c.aigp, wire.aigp[:], metric)
}

// overlayList answers a list attribute's value after a member's adds and
// removes: the current values no remove names, in order, then the additions,
// which is what the attribute's handler writes for the same operations. size
// is the octets of one value.
func overlayList[T comparable](view *policyview.View, reader policyview.Reader, bufs *[2][]byte, next *uint8,
	list *valueList[T], size int, from func([]byte) T, appendValue func([]byte, T) []byte,
) ([]byte, error) {
	current, err := view.Read(reader)
	if err != nil {
		return nil, err
	}
	buf := &bufs[*next]
	*next ^= 1
	out := (*buf)[:0]
	values := current.Wire()
	for i := 0; i+size <= len(values); i += size {
		if !list.removes(from(values[i : i+size])) {
			out = append(out, values[i:i+size]...)
		}
	}
	for _, value := range list.add[:list.addN] {
		out = appendValue(out, value)
	}
	*buf = out
	return out, nil
}

// largeFrom reads one large community.
//
// RFC 8092 Section 3: "Each BGP Large Community value is encoded as a 12-octet quantity, as follows:"
func largeFrom(b []byte) attribute.LargeCommunity {
	return attribute.LargeCommunity{
		GlobalAdmin: binary.BigEndian.Uint32(b), LocalData1: binary.BigEndian.Uint32(b[4:]),
		LocalData2: binary.BigEndian.Uint32(b[8:]),
	}
}

func largeAppend(out []byte, community attribute.LargeCommunity) []byte {
	out = binary.BigEndian.AppendUint32(out, community.GlobalAdmin)
	out = binary.BigEndian.AppendUint32(out, community.LocalData1)
	return binary.BigEndian.AppendUint32(out, community.LocalData2)
}

// extendedFrom reads one extended community.
//
// RFC 4360 Section 2: "Each Extended Community is encoded as an 8-octet quantity, as follows:"
func extendedFrom(b []byte) attribute.ExtendedCommunity {
	return attribute.ExtendedCommunity(b[:8])
}

func extendedAppend(out []byte, community attribute.ExtendedCommunity) []byte {
	return append(out, community[:]...)
}
