// Design: docs/architecture/core-design.md -- BGP role plugin OTC processing
// RFC: rfc/short/rfc9234.md
// Overview: role.go -- role plugin entry point

package role

import (
	"encoding/binary"
	"slices"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// OTC attribute constants.
// RFC 9234 Section 5: OTC is type 35, Optional Transitive (flags 0xC0), 4-byte ASN value.
const (
	otcAttrCode  = byte(35)       // RFC 9234: Only to Customer
	otcAttrFlags = byte(0xC0)     // Optional + Transitive
	otcAttrLen   = 4              // 4-byte ASN
	otcWireLen   = 3 + otcAttrLen // flags(1) + type(1) + len(1) + value(4) = 7
)

// findOTC scans raw path attributes for OTC (type 35).
// Returns the 4-byte ASN value and true if found.
// Returns 0, false if not present or malformed.
//
// RFC 9234 Section 5: malformed OTC (length != 4) uses treat-as-withdraw.
func findOTC(attrs []byte) (asn uint32, found, malformed bool) {
	off := 0
	for off < len(attrs) {
		if off+3 > len(attrs) {
			break
		}

		flags := attribute.AttributeFlags(attrs[off])
		code := attrs[off+1]
		var attrLen uint16
		var hdrLen int

		if flags.IsExtLength() {
			if off+4 > len(attrs) {
				break
			}
			attrLen = binary.BigEndian.Uint16(attrs[off+2 : off+4])
			hdrLen = 4
		} else {
			attrLen = uint16(attrs[off+2])
			hdrLen = 3
		}

		if off+hdrLen+int(attrLen) > len(attrs) {
			break
		}

		if code == otcAttrCode {
			// RFC 9234: OTC length MUST be 4.
			if attrLen != otcAttrLen {
				return 0, false, true
			}
			val := binary.BigEndian.Uint32(attrs[off+hdrLen : off+hdrLen+otcAttrLen])
			return val, true, false
		}

		off += hdrLen + int(attrLen)
	}

	return 0, false, false
}

// buildOTCAttr returns the 7-byte wire encoding of an OTC attribute.
// RFC 9234: flags=0xC0 (Optional Transitive), type=35, length=4, value=ASN.
func buildOTCAttr(asn uint32) [otcWireLen]byte {
	var buf [otcWireLen]byte
	buf[0] = otcAttrFlags
	buf[1] = otcAttrCode
	buf[2] = otcAttrLen
	binary.BigEndian.PutUint32(buf[3:], asn)
	return buf
}

// appendOTCToAttrs creates a new attribute byte slice with OTC appended.
// Used for ingress stamping: the original attributes plus the new OTC attribute.
func appendOTCToAttrs(attrs []byte, asn uint32) []byte {
	otc := buildOTCAttr(asn)
	result := make([]byte, len(attrs)+otcWireLen)
	copy(result, attrs)
	copy(result[len(attrs):], otc[:])
	return result
}

// isUnicastFamily returns true for IPv4/IPv6 unicast families.
// RFC 9234 Section 5: OTC processing is scoped to AFI 1/2 (IPv4/IPv6), SAFI 1 (Unicast).
func isUnicastFamily(family string) bool {
	return family == "ipv4/unicast" || family == "ipv6/unicast"
}

// OTC ingress filter result.
const (
	otcAccept        = 0 // Route accepted (possibly with OTC stamped)
	otcRejectLeak    = 1 // Route rejected: leak detection
	otcTreatWithdraw = 2 // Route rejected: malformed OTC (treat-as-withdraw)
)

// checkOTCIngress applies RFC 9234 Section 5 ingress rules.
// remoteRole is the peer's declared role (from their OPEN capability code 9).
// remoteASN is the peer's AS number.
// attrs is the raw path attributes from the UPDATE.
//
// Returns:
//   - result: otcAccept, otcRejectLeak, or otcTreatWithdraw
//   - stampASN: if non-zero, OTC should be stamped with this ASN
//
// RFC 9234 Section 5: ingress rules are non-overridable by the operator.
func checkOTCIngress(remoteRole string, remoteASN uint32, attrs []byte) (result int, stampASN uint32) {
	otcASN, hasOTC, isMalformed := findOTC(attrs)

	// RFC 9234 Section 5: malformed OTC -> treat-as-withdraw.
	if isMalformed {
		return otcTreatWithdraw, 0
	}

	if hasOTC {
		// RFC 9234 Section 5: "If a route with the OTC Attribute is received
		// from a Customer or an RS-Client, then it is a route leak and MUST
		// be considered ineligible."
		if remoteRole == roleCustomer || remoteRole == roleRSClient {
			return otcRejectLeak, 0
		}

		// RFC 9234 Section 5: "If a route with the OTC Attribute is received
		// from a Peer and the OTC Attribute does not have a value equal to the
		// Peer's AS number, then it is a route leak and MUST be considered
		// ineligible."
		if remoteRole == rolePeer && otcASN != remoteASN {
			return otcRejectLeak, 0
		}
	} else if remoteRole == roleProvider || remoteRole == rolePeer || remoteRole == roleRS {
		// RFC 9234 Section 5: "If a route without the OTC Attribute is
		// received from a Provider, a Peer, or an RS, then it MUST
		// be added with a value equal to the AS number of the remote peer."
		return otcAccept, remoteASN
	}

	return otcAccept, 0
}

// checkOTCEgress applies RFC 9234 Section 5 egress rules.
// Returns true if the route should be suppressed (not sent to this destination peer).
//
// RFC 9234 Section 5: routes with OTC must not propagate to Provider/Peer/RS.
func checkOTCEgress(destRemoteRole string, attrs []byte) bool {
	_, hasOTC, _ := findOTC(attrs)

	// RFC 9234 Section 5: suppress routes with OTC to Provider, Peer, or RS.
	return hasOTC && (destRemoteRole == roleProvider || destRemoteRole == rolePeer || destRemoteRole == roleRS)
}

// extractAttrsFromPayload extracts path attributes from an UPDATE payload.
// RFC 4271: payload = withdrawnLen(2) + withdrawn + attrLen(2) + attrs + nlri.
func extractAttrsFromPayload(payload []byte) []byte {
	if len(payload) < 4 {
		return nil
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrOffset := 2 + withdrawnLen
	if len(payload) < attrOffset+2 {
		return nil
	}
	attrLen := int(binary.BigEndian.Uint16(payload[attrOffset : attrOffset+2]))
	attrStart := attrOffset + 2
	if len(payload) < attrStart+attrLen {
		return nil
	}
	return payload[attrStart : attrStart+attrLen]
}

// insertOTCInPayload creates a new UPDATE payload with OTC appended to path attributes.
// Updates the attrLen field to account for the added OTC attribute.
func insertOTCInPayload(payload []byte, otcASN uint32) []byte {
	if len(payload) < 4 {
		return payload
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrOffset := 2 + withdrawnLen
	if len(payload) < attrOffset+2 {
		return payload
	}
	attrLen := int(binary.BigEndian.Uint16(payload[attrOffset : attrOffset+2]))
	attrEnd := attrOffset + 2 + attrLen
	if len(payload) < attrEnd {
		return payload
	}

	otc := buildOTCAttr(otcASN)
	newAttrLen := attrLen + otcWireLen

	// Guard against uint16 overflow (Extended Message UPDATEs can have large attrs).
	// Returns nil to signal failure -- caller should accept route without modification.
	if newAttrLen > 65535 {
		return nil
	}

	result := make([]byte, len(payload)+otcWireLen)
	// Copy withdrawn section (including 2-byte length).
	copy(result, payload[:attrOffset])
	// Write new attrLen.
	binary.BigEndian.PutUint16(result[attrOffset:], uint16(newAttrLen)) //nolint:gosec // G115: bounded by check above
	// Copy original attrs.
	copy(result[attrOffset+2:], payload[attrOffset+2:attrEnd])
	// Append OTC after attrs.
	copy(result[attrEnd:], otc[:])
	// Copy NLRI section.
	copy(result[attrEnd+otcWireLen:], payload[attrEnd:])

	return result
}

// OTCIngressFilter is the ingress filter function registered in the plugin registry.
// Called by the reactor for each received UPDATE before caching and dispatching.
// Checks OTC ingress rules per RFC 9234 Section 5.
//
// Sets meta["src-role"] to the source peer's role from our config (e.g., "provider", "customer").
// The egress filter uses this for suppression decisions -- our configured knowledge of the
// peer relationship, independent of whether OTC is in the wire bytes.
// If we don't configure a role for a peer, we don't filter its routes.
func OTCIngressFilter(src registry.PeerFilterInfo, payload []byte, meta map[string]any) (bool, []byte) {
	cfg, remoteRole := getFilterConfig(src.Address.String())

	// Always record source peer's role in metadata from our configuration.
	if cfg != nil && cfg.role != "" {
		meta["src-role"] = cfg.role
	}

	// No role config or no remote role: no OTC filtering.
	if cfg == nil || remoteRole == "" {
		return true, nil
	}

	attrs := extractAttrsFromPayload(payload)
	if attrs == nil {
		return true, nil
	}

	result, stampASN := checkOTCIngress(remoteRole, src.PeerAS, attrs)

	switch result {
	case otcRejectLeak:
		logger().Debug("OTC ingress reject: route leak",
			"peer", src.Address, "remote-role", remoteRole)
		return false, nil
	case otcTreatWithdraw:
		logger().Info("OTC ingress reject: malformed OTC",
			"peer", src.Address)
		return false, nil
	}

	// Stamp OTC if needed. insertOTCInPayload returns nil on overflow.
	if stampASN > 0 {
		modified := insertOTCInPayload(payload, stampASN)
		if modified != nil {
			logger().Debug("OTC ingress stamp",
				"peer", src.Address, "remote-role", remoteRole, "otc-asn", stampASN)
			return true, modified
		}
		logger().Warn("OTC ingress stamp failed: attribute overflow", "peer", src.Address)
	}

	return true, nil
}

// OTCEgressFilter is the egress filter function registered in the plugin registry.
// Called by the reactor per destination peer during ForwardUpdate.
// Checks both export role filtering and OTC egress suppression per RFC 9234 Section 5.
//
// Uses meta["src-role"] (our configured knowledge of the source peer's role) for
// suppression decisions. If we don't configure a role, we don't filter.
func OTCEgressFilter(src, dest registry.PeerFilterInfo, payload []byte, meta map[string]any, _ *registry.ModAccumulator) bool {
	srcCfg, _ := getFilterConfig(src.Address.String())
	_, destRemoteRole := getFilterConfig(dest.Address.String())

	// RFC 9234 Section 5 OTC egress suppression (non-overridable).
	// A route from a Provider/Peer/RS source must not be sent to a Provider/Peer/RS destination.
	// Based on our configured role for the source peer (meta["src-role"]).
	if destRemoteRole == roleProvider || destRemoteRole == rolePeer || destRemoteRole == roleRS {
		srcRole, _ := meta["src-role"].(string)
		if srcRole == roleProvider || srcRole == rolePeer || srcRole == roleRS {
			logger().Debug("OTC egress suppress",
				"src", src.Address, "src-role", srcRole, "dest", dest.Address, "dest-role", destRemoteRole)
			return false
		}
	}

	// Source has no role config: no export role filtering.
	if srcCfg == nil {
		return true
	}

	// Export role filtering: check if destination role is in the allowed set.
	if len(srcCfg.export) > 0 {
		destRole := destRemoteRole
		if destRole == "" {
			destRole = roleUnknown
		}
		allowed := resolveExport(srcCfg.role, srcCfg.export)
		if !slices.Contains(allowed, destRole) {
			return false // Destination role not in export set.
		}
	}

	return true
}
