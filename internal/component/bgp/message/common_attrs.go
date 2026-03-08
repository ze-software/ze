// Design: docs/architecture/wire/messages.md — BGP message types
// Related: update_build.go — UPDATE builder infrastructure
//
// Package message provides BGP message building and parsing.
//
// This file contains shared attribute extraction helpers used by route
// encoding functions across multiple packages.
package message

import (
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
)

// CommonAttrs holds extracted common BGP path attributes.
// Used by route-to-params conversion functions to avoid duplicate extraction code.
type CommonAttrs struct {
	Origin            attribute.Origin
	LocalPreference   uint32
	MED               uint32
	ASPath            []uint32
	Communities       []uint32
	LargeCommunities  [][3]uint32
	ExtCommunityBytes []byte
}

// ExtractAttrsFromWire extracts CommonAttrs from AttributesWire.
// Returns default values (OriginIGP) for nil wire input.
//
// Iterates the attribute list once, recording known attributes via type switch.
func ExtractAttrsFromWire(wire *attribute.AttributesWire) CommonAttrs {
	attrs := CommonAttrs{Origin: attribute.OriginIGP}

	if wire == nil {
		return attrs
	}

	all, err := wire.All()
	if err != nil {
		return attrs
	}

	for _, a := range all {
		switch v := a.(type) {
		case attribute.Origin:
			attrs.Origin = v
		case attribute.LocalPref:
			attrs.LocalPreference = uint32(v)
		case attribute.MED:
			attrs.MED = uint32(v)
		case *attribute.ASPath:
			if len(v.Segments) > 0 {
				attrs.ASPath = v.Segments[0].ASNs
			}
		case attribute.Communities:
			attrs.Communities = make([]uint32, len(v))
			for i, c := range v {
				attrs.Communities[i] = uint32(c)
			}
		case attribute.LargeCommunities:
			attrs.LargeCommunities = make([][3]uint32, len(v))
			for i, c := range v {
				attrs.LargeCommunities[i] = [3]uint32{c.GlobalAdmin, c.LocalData1, c.LocalData2}
			}
		case attribute.ExtendedCommunities:
			buf := make([]byte, v.Len())
			v.WriteTo(buf, 0)
			attrs.ExtCommunityBytes = buf
		}
	}

	return attrs
}
