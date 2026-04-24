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

// MP_REACH_NLRI attribute type code.
const mpReachAttrCode = byte(14)

// isPayloadUnicast checks if an UPDATE payload carries IPv4 or IPv6 unicast.
// RFC 9234 Section 5: OTC processing is scoped to AFI 1/2 (IPv4/IPv6), SAFI 1 (Unicast).
//
// If no MP_REACH_NLRI attribute is found, the UPDATE is IPv4 unicast (RFC 4271 implicit).
// If MP_REACH_NLRI is found, reads AFI (2 bytes) and SAFI (1 byte) from the attribute value.
func isPayloadUnicast(payload []byte) bool {
	attrs := extractAttrsFromPayload(payload)
	if attrs == nil {
		return true // Malformed or empty: treat as unicast (fail-open for OTC)
	}

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

		if code == mpReachAttrCode {
			// MP_REACH_NLRI: AFI (2 bytes) + SAFI (1 byte) at start of value.
			if attrLen < 3 {
				return true // Malformed MP_REACH: treat as unicast
			}
			valStart := off + hdrLen
			afi := binary.BigEndian.Uint16(attrs[valStart : valStart+2])
			safi := attrs[valStart+2]
			return (afi == 1 || afi == 2) && safi == 1
		}

		off += hdrLen + int(attrLen)
	}

	// No MP_REACH_NLRI found: IPv4 unicast (RFC 4271).
	return true
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
		return nil // Malformed: signal no modification.
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrOffset := 2 + withdrawnLen
	if len(payload) < attrOffset+2 {
		return nil // Malformed: signal no modification.
	}
	attrLen := int(binary.BigEndian.Uint16(payload[attrOffset : attrOffset+2]))
	attrEnd := attrOffset + 2 + attrLen
	if len(payload) < attrEnd {
		return nil // Malformed: signal no modification.
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

// payloadToWithdrawal converts an UPDATE payload to a pure withdrawal.
// RFC 7606 Section 2: treat-as-withdraw moves announced NLRIs to the withdrawn
// section and clears path attributes. Returns nil if the payload is malformed
// or carries no announcements to withdraw.
func payloadToWithdrawal(payload []byte) []byte {
	if len(payload) < 4 {
		return nil
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrOffset := 2 + withdrawnLen
	if len(payload) < attrOffset+2 {
		return nil
	}
	attrLen := int(binary.BigEndian.Uint16(payload[attrOffset : attrOffset+2]))
	nlriStart := attrOffset + 2 + attrLen
	if nlriStart > len(payload) {
		return nil
	}
	existingWD := payload[2 : 2+withdrawnLen]
	trailingNLRI := payload[nlriStart:]

	totalWDLen := len(existingWD) + len(trailingNLRI)
	if totalWDLen == 0 {
		return nil
	}

	// Build: withdrawnLen(2) + withdrawn + trailingNLRI + attrLen=0(2)
	result := make([]byte, 2+totalWDLen+2)
	binary.BigEndian.PutUint16(result[0:2], uint16(totalWDLen)) //nolint:gosec // G115: bounded by input
	copy(result[2:], existingWD)
	copy(result[2+len(existingWD):], trailingNLRI)
	binary.BigEndian.PutUint16(result[2+totalWDLen:], 0) // empty path attributes
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

	// RFC 9234 Section 5: OTC MUST NOT be applied to other address families by default.
	if !isPayloadUnicast(payload) {
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
		logger().Info("OTC treat-as-withdraw: malformed OTC",
			"peer", src.Address)
		if wd := payloadToWithdrawal(payload); wd != nil {
			return true, wd
		}
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
// Two independent egress checks:
//  1. Wire-bytes OTC check (unconditional): if route has OTC, MUST NOT propagate to Provider/Peer/RS.
//  2. Meta-based Gao-Rexford check: if source role is Provider/Peer/RS, suppress to Provider/Peer/RS.
func OTCEgressFilter(src, dest registry.PeerFilterInfo, payload []byte, meta map[string]any, mods *registry.ModAccumulator) bool {
	// RFC 9234 Section 5: OTC MUST NOT be applied to other address families by default.
	if !isPayloadUnicast(payload) {
		return true
	}

	srcCfg, _ := getFilterConfig(src.Address.String())
	_, destRemoteRole := getFilterConfig(dest.Address.String())

	// RFC 9234 Section 5 egress rule 2 (unconditional, wire-bytes):
	// "If a route already contains the OTC Attribute, it MUST NOT be
	// propagated to Providers, Peers, or RSes."
	// This check does not depend on source peer configuration.
	if checkOTCEgress(destRemoteRole, extractAttrsFromPayload(payload)) {
		logger().Debug("OTC egress suppress (wire-bytes)",
			"src", src.Address, "dest", dest.Address, "dest-role", destRemoteRole)
		return false
	}

	// Gao-Rexford leak prevention (meta-based safety net):
	// Routes from a Provider/Peer/RS source must not be sent to a Provider/Peer/RS destination.
	// meta["src-role"] stores our LOCAL role for the source peer (from config "import" keyword).
	// Our local role maps to the source peer's type:
	//   customer  → source IS Provider    peer     → source IS Peer
	//   rs-client → source IS RS          provider → source IS Customer (allowed to transit)
	if destRemoteRole == roleProvider || destRemoteRole == rolePeer || destRemoteRole == roleRS {
		srcRole, _ := meta["src-role"].(string)
		if srcRole == roleCustomer || srcRole == rolePeer || srcRole == roleRSClient {
			logger().Debug("OTC egress suppress (src-role)",
				"src", src.Address, "src-role", srcRole, "dest", dest.Address, "dest-role", destRemoteRole)
			return false
		}
	}

	// Source has no role config: no export role filtering.
	if srcCfg == nil {
		return true
	}

	// Export role filtering: check if destination role is in the allowed set.
	// Uses pre-computed resolvedExport (resolved at config time, not per-UPDATE).
	if len(srcCfg.resolvedExport) > 0 {
		destRole := destRemoteRole
		if destRole == "" {
			destRole = roleUnknown
		}
		if !slices.Contains(srcCfg.resolvedExport, destRole) {
			return false // Destination role not in export set.
		}
	}

	// RFC 9234 Section 5: "If a route is to be advertised to a Customer, a Peer,
	// or an RS-Client [...] and the OTC Attribute is not present, then [...]
	// an OTC Attribute MUST be added with a value equal to the AS number of the local AS."
	if mods != nil && (destRemoteRole == roleCustomer || destRemoteRole == rolePeer || destRemoteRole == roleRSClient) {
		attrs := extractAttrsFromPayload(payload)
		_, hasOTC, _ := findOTC(attrs)
		if !hasOTC {
			localASN := getLocalASN()
			if localASN > 0 {
				var asnBuf [otcAttrLen]byte
				binary.BigEndian.PutUint32(asnBuf[:], localASN)
				mods.Op(otcAttrCode, registry.AttrModSet, asnBuf[:]) // value bytes only (4-byte ASN)
				logger().Debug("OTC egress stamp mod",
					"src", src.Address, "dest", dest.Address, "dest-role", destRemoteRole, "otc-asn", localASN)
			}
		}
	}

	return true
}

// otcAttrModHandler is the AttrModHandler for OTC (type 35).
// Called by the progressive build during the attribute walk.
// src is the source OTC attribute bytes (nil if absent).
// ops contains the set operation with pre-built 4-byte ASN value bytes.
// Writes the complete 7-byte OTC attribute (header + value) into buf at off.
// Returns the new offset after written bytes.
//
// RFC 9234 Section 5: "Once the OTC Attribute has been set, it MUST be preserved unchanged."
// If source already has OTC, it is copied unchanged (set op is ignored).
func otcAttrModHandler(src []byte, ops []registry.AttrOp, buf []byte, off int) int {
	// OTC already present in source: preserve unchanged.
	if len(src) > 0 {
		if off+len(src) > len(buf) {
			return off
		}
		copy(buf[off:], src)
		return off + len(src)
	}
	// OTC absent: create from the first set op's value bytes.
	for _, op := range ops {
		if op.Action != registry.AttrModSet || len(op.Buf) != otcAttrLen {
			continue
		}
		if off+otcWireLen > len(buf) {
			logger().Warn("OTC attr mod handler: buffer overflow, skipping stamp")
			return off
		}
		buf[off] = otcAttrFlags
		buf[off+1] = otcAttrCode
		buf[off+2] = otcAttrLen
		copy(buf[off+3:], op.Buf)
		return off + otcWireLen
	}
	return off
}
