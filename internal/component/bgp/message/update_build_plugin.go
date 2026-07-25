// Design: docs/architecture/update-building.md -- generic plugin UPDATE builders
// Overview: update_build.go -- core UpdateBuilder struct and unicast builders

package message

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// PluginParams contains parameters for building a generic plugin route UPDATE.
//
// The plugin's config parser pre-builds the family-specific NLRI and any
// family-specific path attributes (ext-communities, prefix-sid, NEXT_HOP code-3,
// communities, MED, originator-id, cluster-list, ...) as RawAttrs wire bytes.
// BuildPlugin owns only the session-context attributes: ORIGIN (default IGP
// unless the plugin supplied code 1), AS_PATH (ASN4-encoded from ASPath),
// LOCAL_PREF (iBGP-gated), and MP_REACH_NLRI.
type PluginParams struct {
	AFI      uint16
	SAFI     byte
	IsIPv6   bool
	NLRI     []byte // One or more concatenated NLRIs for this family/SAFI.
	NextHop  netip.Addr
	RawAttrs [][]byte // Pre-built extra attribute wire bytes (flags+code+len+value).

	// ASPath is the configured AS_PATH (nil = local-origin default).
	ASPath []uint32
	// LocalPreference is the configured LOCAL_PREF value (0 = default 100 on iBGP).
	LocalPreference uint32
	// MapV4NextHop maps an IPv4 next-hop to IPv4-mapped IPv6 in MP_REACH for IPv6
	// families (MUP / SR-Policy). MVPN/VPLS/FlowSpec leave it false.
	MapV4NextHop bool
}

// pluginMaxAttrs bounds the per-route attribute count: AS_PATH + ORIGIN +
// LOCAL_PREF + MP_REACH + up to pluginMaxRawAttrs plugin-supplied attributes.
const (
	pluginMaxRawAttrs = 16
	pluginMaxAttrs    = pluginMaxRawAttrs + 4
)

// BuildPlugin builds an UPDATE message for a plugin-registered route.
// Multiprotocol routes carry next-hop in MP_REACH_NLRI; a family that also needs
// a legacy NEXT_HOP (code 3) attribute (IPv4 MUP/MVPN) supplies it via RawAttrs.
//
// Wire order is fixed by OrderAttributes (MP_UNREACH first, regular attrs by
// code, MP_REACH last), so the order attributes are added here is irrelevant.
func (ub *UpdateBuilder) BuildPlugin(p PluginParams) *Update {
	ub.resetScratch()

	var attrBuf [pluginMaxAttrs]attribute.Attribute
	var rawBuf [pluginMaxRawAttrs]fullRawAttribute
	n := 0

	// AS_PATH (code 2) is always built here: the plugin cannot encode it without
	// the negotiated ASN4 capability. The plugin must never supply code 2.
	asPath := ub.buildASPath(p.ASPath)
	asn4 := ub.ASN4
	asPathData := ub.alloc(asPath.LenWithASN4(asn4))
	asPath.WriteToWithASN4(asPathData, 0, asn4)
	attrBuf[n] = &rawAttribute{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathData,
	}
	n++

	// Plugin-supplied raw attributes. Track ORIGIN and drop any AS_PATH/LOCAL_PREF
	// (those are owned by BuildPlugin / session context).
	pluginOrigin := false
	rb := 0
	for _, raw := range p.RawAttrs {
		if len(raw) < 3 || rb >= len(rawBuf) {
			continue
		}
		switch attribute.AttributeCode(raw[1]) { //nolint:exhaustive // only AS_PATH/LOCAL_PREF/ORIGIN need special handling
		case attribute.AttrASPath, attribute.AttrLocalPref:
			continue
		case attribute.AttrOrigin:
			pluginOrigin = true
		}
		rawBuf[rb] = fullRawAttribute{data: raw}
		attrBuf[n] = &rawBuf[rb]
		rb++
		n++
	}

	// ORIGIN (code 1): default IGP unless the plugin supplied its own.
	if !pluginOrigin {
		attrBuf[n] = attribute.OriginIGP
		n++
	}

	// LOCAL_PREF (code 5): well-known discretionary, iBGP only (RFC 4271 Section 5.1.5).
	if ub.IsIBGP {
		lp := p.LocalPreference
		if lp == 0 {
			lp = 100
		}
		attrBuf[n] = attribute.LocalPref(lp)
		n++
	}

	attrBuf[n] = ub.buildMPReachPlugin(p)
	n++

	return &Update{
		PathAttributes: ub.packAttributesOrderedInto(attrBuf[:n], nil),
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

	afi := p.AFI
	if afi == 0 {
		// Backstop for callers that only set IsIPv6 (AFI 1/2 families).
		afi = 1
		if p.IsIPv6 {
			afi = 2
		}
	}

	var nhBytes []byte
	if p.MapV4NextHop && p.IsIPv6 && p.NextHop.Is4() {
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
