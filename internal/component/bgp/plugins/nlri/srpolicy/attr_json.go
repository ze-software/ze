// Design: docs/architecture/wire/nlri.md -- SR-Policy Tunnel Encapsulation rendering for the offline decoder
// RFC: rfc/short/rfc9830.md -- Binding SID sub-TLVs (Sections 2.4.2, 2.4.3)
// RFC: rfc/short/rfc9012.md -- Tunnel Encapsulation attribute framing
// Related: config.go -- the encoder that writes these sub-TLVs
// Related: register.go -- registers TunnelEncapToJSON for attribute 23

package srpolicy

import (
	"encoding/binary"
	"encoding/hex"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// JSON keys of the rendered sub-TLVs, kebab-case like every Ze JSON key.
const (
	jsonKeyType      = "type"
	jsonKeyName      = "name"
	jsonKeyFlags     = "flags"
	jsonKeyValue     = "value"
	jsonKeyMalformed = "malformed"
	jsonKeySRv6SID   = "srv6-sid"
)

// TunnelEncapToJSON renders a Tunnel Encapsulation attribute (code 23) for
// `ze bgp decode`. Every TLV is listed with its tunnel type and its sub-TLVs,
// and the two Binding SID sub-TLVs of an SR Policy are decoded. Any other
// sub-TLV is shown as its type and value octets in hex. It answers nil when
// the attribute's TLV framing cannot be walked, so the caller shows the
// attribute as raw hex. Safe for concurrent use: it keeps no state.
func TunnelEncapToJSON(value []byte) map[string]any {
	te, err := attribute.ParseTunnelEncap(value)
	if err != nil {
		return nil
	}
	tunnels := make([]any, 0, len(te.TLVs))
	for i := range te.TLVs {
		tunnels = append(tunnels, tunnelTLVToJSON(&te.TLVs[i]))
	}
	return map[string]any{"tunnels": tunnels}
}

// tunnelTLVToJSON renders one tunnel TLV. A TLV whose sub-TLV framing does not
// walk is shown as its value octets in hex, never as a partial list.
func tunnelTLVToJSON(tlv *attribute.TunnelTLV) map[string]any {
	out := map[string]any{"tunnel-type": tlv.TunnelType}
	subs, err := tlv.SubTLVs()
	if err != nil {
		out[jsonKeyValue] = hex.EncodeToString(tlv.Value)
		return out
	}
	rendered := make([]any, 0, len(subs))
	for i := range subs {
		rendered = append(rendered, subTLVToJSON(subs[i].Type, subs[i].Value))
	}
	out["sub-tlvs"] = rendered
	return out
}

// subTLVToJSON renders one sub-TLV. The sub-TLV type space is open (RFC 9012
// Section 13 keeps an unrecognized sub-TLV), so a type this plugin does not
// decode is shown as hex.
func subTLVToJSON(styp uint8, value []byte) map[string]any {
	switch styp {
	case subTLVBindingSID:
		return bindingSIDToJSON(value)
	case subTLVSRv6BindingSID:
		return srv6BindingSIDToJSON(value)
	}
	return map[string]any{jsonKeyType: styp, jsonKeyValue: hex.EncodeToString(value)}
}

// bindingSIDToJSON decodes the Binding SID sub-TLV (type 13). The value length
// alone tells which BSID the sub-TLV carries.
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|   Type=13     |    Length     |     Flags     |   RESERVED    |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  Binding SID: none (Length 2), label entry (6), SRv6 SID (18) |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// value holds the octets after Type and Length: Flags at 0, RESERVED at 1,
// the Binding SID from 2.
func bindingSIDToJSON(value []byte) map[string]any {
	out := map[string]any{jsonKeyType: subTLVBindingSID, jsonKeyName: "binding-sid"}
	// RFC 9830 Section 2.4.2: "The value MUST be 18 when a SRv6 BSID is
	// present, 6 when an SR-MPLS BSID is present, or 2 when no BSID is present."
	switch len(value) {
	case int(bindingSIDLenNone):
		out[jsonKeyFlags] = value[0]
	case int(bindingSIDLenMPLS):
		out[jsonKeyFlags] = value[0]
		// RFC 9830 Section 2.4.2: "Traffic Class (TC), S, and TTL (Total of
		// 12 bits) are RESERVED and MUST be set to zero and MUST be ignored."
		out["mpls-label"] = binary.BigEndian.Uint32(value[2:6]) >> 12
	case int(bindingSIDLenSRv6):
		out[jsonKeyFlags] = value[0]
		// RFC 9830 Section 2.4.2: "If the length is 18, then the BSID contains
		// a 16-octet SRv6 SID."
		out[jsonKeySRv6SID] = netip.AddrFrom16([16]byte(value[2:18])).String()
	default:
		// Any other length is none of the three forms the RFC defines.
		out[jsonKeyMalformed] = true
		out[jsonKeyValue] = hex.EncodeToString(value)
	}
	return out
}

// srv6BindingSIDToJSON decodes the SRv6 Binding SID sub-TLV (type 20). value
// holds Flags at 0, RESERVED at 1, the SRv6 SID at 2..17, and the optional
// SRv6 Endpoint Behavior and SID Structure at 18..25.
func srv6BindingSIDToJSON(value []byte) map[string]any {
	out := map[string]any{jsonKeyType: subTLVSRv6BindingSID, jsonKeyName: "srv6-binding-sid"}
	// RFC 9830 Section 2.4.3: "The value MUST be 26 when the SRv6 Endpoint
	// Behavior and SID Structure is present; else, it MUST be 18."
	if len(value) != 18 && len(value) != 26 {
		out[jsonKeyMalformed] = true
		out[jsonKeyValue] = hex.EncodeToString(value)
		return out
	}
	out[jsonKeyFlags] = value[0]
	out[jsonKeySRv6SID] = netip.AddrFrom16([16]byte(value[2:18])).String()
	return out
}
