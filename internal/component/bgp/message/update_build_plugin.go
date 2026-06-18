// Design: docs/architecture/update-building.md -- generic plugin UPDATE builders
// Overview: update_build.go -- core UpdateBuilder struct and unicast builders
// Related: update_build_mup.go -- MUP UPDATE builders (same pattern)

package message

import (
	"net/netip"
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
)

// PluginParams contains parameters for building a generic plugin route UPDATE.
type PluginParams struct {
	IsIPv6   bool
	SAFI     byte
	NLRI     []byte
	NextHop  netip.Addr
	RawAttrs [][]byte // Pre-built extra attribute wire bytes (flags+code+len+value).
}

const pluginMaxAttrs = 8

// BuildPlugin builds an UPDATE message for a plugin-registered route.
// Multiprotocol routes carry next-hop in MP_REACH_NLRI, not NEXT_HOP (code 3).
func (ub *UpdateBuilder) BuildPlugin(p PluginParams) *Update {
	ub.resetScratch()

	var attrBuf [pluginMaxAttrs]attribute.Attribute
	var rawBuf [4]fullRawAttribute
	n := 0

	attrBuf[n] = attribute.OriginIGP
	n++

	asPath := ub.buildASPath(nil)
	asn4 := ub.ASN4
	asPathData := ub.alloc(asPath.LenWithASN4(asn4))
	asPath.WriteToWithASN4(asPathData, 0, asn4)
	attrBuf[n] = &rawAttribute{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathData,
	}
	n++

	if ub.IsIBGP {
		attrBuf[n] = attribute.LocalPref(100)
		n++
	}

	for i, raw := range p.RawAttrs {
		if len(raw) < 3 || i >= len(rawBuf) {
			continue
		}
		rawBuf[i] = fullRawAttribute{data: raw}
		attrBuf[n] = &rawBuf[i]
		n++
	}

	attrBuf[n] = ub.buildMPReachPlugin(p)
	n++

	attrs := attrBuf[:n]
	sort.Slice(attrs, func(i, j int) bool {
		return attrs[i].Code() < attrs[j].Code()
	})

	return &Update{
		PathAttributes: ub.packAttributesOrderedInto(attrs, nil),
	}
}

// buildMPReachPlugin constructs MP_REACH_NLRI for a plugin route.
func (ub *UpdateBuilder) buildMPReachPlugin(p PluginParams) *rawAttribute {
	if len(p.NLRI) == 0 {
		return &rawAttribute{
			flags: attribute.FlagOptional,
			code:  attribute.AttrMPReachNLRI,
		}
	}

	var afi uint16 = 1
	if p.IsIPv6 {
		afi = 2
	}

	var nhBytes []byte
	if p.IsIPv6 && p.NextHop.Is4() {
		mapped := p.NextHop.As16()
		nhBytes = mapped[:]
	} else {
		nhBytes = p.NextHop.AsSlice()
	}
	nhLen := len(nhBytes)

	valueLen := 2 + 1 + 1 + nhLen + 1 + len(p.NLRI)
	value := ub.alloc(valueLen)
	value[0] = byte(afi >> 8)
	value[1] = byte(afi)
	value[2] = p.SAFI
	value[3] = byte(nhLen)
	copy(value[4:4+nhLen], nhBytes)
	value[4+nhLen] = 0
	copy(value[5+nhLen:], p.NLRI)

	return &rawAttribute{
		flags: attribute.FlagOptional,
		code:  attribute.AttrMPReachNLRI,
		data:  value,
	}
}

// fullRawAttribute wraps a pre-built attribute wire (flags+code+len+value),
// exposing Flags/Code/Len/WriteTo for the value portion so WriteAttrTo
// can write its own header.
type fullRawAttribute struct {
	data []byte // complete wire: flags(1)+code(1)+len(1or2)+value
}

func (r *fullRawAttribute) Code() attribute.AttributeCode {
	if len(r.data) >= 2 {
		return attribute.AttributeCode(r.data[1])
	}
	return 0
}

func (r *fullRawAttribute) Flags() attribute.AttributeFlags {
	if len(r.data) >= 1 {
		return attribute.AttributeFlags(r.data[0])
	}
	return 0
}

func (r *fullRawAttribute) valueOffset() int {
	if len(r.data) >= 1 && (r.data[0]&0x10) != 0 {
		return 4 // extended length: flags(1)+code(1)+len(2)
	}
	return 3 // normal: flags(1)+code(1)+len(1)
}

func (r *fullRawAttribute) Len() int {
	off := r.valueOffset()
	if len(r.data) <= off {
		return 0
	}
	return len(r.data) - off
}

func (r *fullRawAttribute) WriteTo(buf []byte, off int) int {
	voff := r.valueOffset()
	if len(r.data) <= voff {
		return 0
	}
	return copy(buf[off:], r.data[voff:])
}

func (r *fullRawAttribute) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return r.WriteTo(buf, off)
}
