// Design: docs/architecture/core-design.md -- BGP role plugin OTC processing
// RFC: rfc/short/rfc9234.md
// Overview: role.go -- role plugin entry point

package role

import (
	"encoding/binary"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
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

// Multiprotocol NLRI attribute type codes (RFC 4760 Sections 3 and 4).
const (
	mpReachAttrCode   = byte(14)
	mpUnreachAttrCode = byte(15)
)

// attrHeaderAt decodes one attribute header at off. Returns ok=false when the
// header (or the value it declares) does not fit inside attrs.
func attrHeaderAt(attrs []byte, off int) (code byte, valStart, hdrLen, attrLen int, ok bool) {
	if off+3 > len(attrs) {
		return 0, 0, 0, 0, false
	}
	flags := attribute.AttributeFlags(attrs[off])
	code = attrs[off+1]
	if flags.IsExtLength() {
		if off+4 > len(attrs) {
			return 0, 0, 0, 0, false
		}
		attrLen = int(binary.BigEndian.Uint16(attrs[off+2 : off+4]))
		hdrLen = 4
	} else {
		attrLen = int(attrs[off+2])
		hdrLen = 3
	}
	if off+hdrLen+attrLen > len(attrs) {
		return 0, 0, 0, 0, false
	}
	return code, off + hdrLen, hdrLen, attrLen, true
}

// isPayloadUnicast reports whether an UPDATE payload is in scope for the OTC
// procedures.
//
// RFC 9234 Section 5: the procedures "are applicable only for the address
// families AFI 1 (IPv4) and AFI 2 (IPv6) with SAFI 1 (unicast) in both cases
// and MUST NOT be applied to other address families by default."
//
// The family is read from MP_REACH_NLRI (RFC 4760 Section 3) when the UPDATE
// advertises, and from MP_UNREACH_NLRI (Section 4) when it only withdraws.
// MP_REACH wins when both are present: it carries the routes being advertised,
// which is what the procedures act on.
//
// Reading MP_UNREACH is not cosmetic. Inspecting only code 14 meant EVERY
// MP_UNREACH-only UPDATE -- a VPNv4, EVPN, flowspec or multicast withdrawal --
// fell through to the "no MP_REACH" branch and was classified IPv4 unicast, so
// the RFC 9234 Section 5 procedures ran on address families the same sentence
// forbids them for. That is the MUST NOT above, not merely a scoping nicety.
//
// An UPDATE carrying neither attribute is the RFC 4271 native encoding, whose
// NLRI and Withdrawn Routes fields are IPv4 unicast by definition. Returning
// true there is a positive family determination, not an absence-of-evidence
// default (ai/rules/evidence.md).
func isPayloadUnicast(payload []byte) bool {
	attrs := extractAttrsFromPayload(payload)
	if attrs == nil {
		return true // Malformed or empty: treat as unicast (fail-open for OTC)
	}

	var unreachAFI uint16
	var unreachSAFI byte
	haveUnreach := false

	off := 0
	for off < len(attrs) {
		code, valStart, hdrLen, attrLen, ok := attrHeaderAt(attrs, off)
		if !ok {
			break
		}

		switch code {
		case mpReachAttrCode:
			// MP_REACH_NLRI: AFI (2 bytes) + SAFI (1 byte) at start of value.
			if attrLen < 3 {
				return true // Malformed MP_REACH: treat as unicast
			}
			afi := binary.BigEndian.Uint16(attrs[valStart : valStart+2])
			safi := attrs[valStart+2]
			return (afi == 1 || afi == 2) && safi == 1
		case mpUnreachAttrCode:
			// Recorded, not returned: attribute order is not guaranteed, so an
			// MP_REACH later in the same UPDATE must still win.
			if attrLen >= 3 && !haveUnreach {
				unreachAFI = binary.BigEndian.Uint16(attrs[valStart : valStart+2])
				unreachSAFI = attrs[valStart+2]
				haveUnreach = true
			}
		}

		off += hdrLen + attrLen
	}

	if haveUnreach {
		return (unreachAFI == 1 || unreachAFI == 2) && unreachSAFI == 1
	}

	// Neither MP attribute: RFC 4271 native encoding, IPv4 unicast.
	return true
}

// payloadAdvertisesNLRI reports whether an UPDATE advertises reachable NLRI:
// a non-empty Network Layer Reachability Information field (RFC 4271
// Section 4.3, IPv4 unicast) or an MP_REACH_NLRI attribute (RFC 4760
// Section 3). It is the gate on BOTH OTC stamping rules.
//
// DO NOT LOOSEN THIS. Both RFC 9234 Section 5 stamping rules are conditioned on
// a route being carried. The egress rule says "If a route is to be advertised
// ... then when advertising the route, an OTC Attribute MUST be added"; the
// ingress rule says "If a route is received ... then it MUST be added". "Is to
// be advertised" is the rule's first clause and it is a CONDITION, not
// scene-setting. An UPDATE that advertises nothing carries no route for either
// rule to act on, so the obligation never arises.
//
// Stamping one anyway is not inert, it is wire-visible damage, in three ways:
//
//  1. RFC 4271 Section 4.3: an UPDATE that only withdraws "will not include
//     path attributes or Network Layer Reachability Information". A stamped
//     withdrawal is a message the base spec says cannot exist.
//
//  2. RFC 7606 Section 5.2: an UPDATE that "does contain path attributes other
//     than MP_UNREACH_NLRI and doesn't encode any reachable NLRI" leaves a
//     conforming receiver unable to trust that the NLRI parsed, so any later
//     attribute error in it escalates from its own handling to "session
//     reset". A peer withdrawing prefixes could therefore drop the session.
//
//  3. THE END-OF-RIB MARKER. RFC 7606 Section 5.2 names the two RFC 4724 EoR
//     encodings explicitly: an UPDATE carrying only MP_UNREACH_NLRI with no
//     NLRI, and "a completely empty UPDATE message in the case of the legacy
//     encoding". Adding any attribute to either one stops it being an
//     End-of-RIB marker at all, breaking graceful-restart convergence for the
//     receiving peer. This is reachable, not theoretical: the route-server
//     fast path (internal/component/bgp/reactor/reactor_notify.go:576) admits
//     a received UPDATE to the forward rails on `msgType == TypeUPDATE` alone,
//     and an EoR is an UPDATE. (Locally ORIGINATED EoRs are safe by a
//     different route -- AnnounceEOR calls peer.SendUpdate directly at
//     reactor_api_forward.go:137, bypassing egress filters -- so this gate is
//     the only thing protecting a RELAYED one.)
//
// Both shapes therefore return false here and are forwarded untouched. The
// guard fires on positive evidence that a route IS carried, so an unreadable
// or unrecognized payload is never stamped by default
// (ai/rules/evidence.md).
//
// The wire-shape walk itself lives in wireu.PayloadAdvertisesNLRI, because the
// forward rail asks the same question before it prepends AS_PATH (RFC 4271
// Section 5.1.2 b, wireu/aspath_slot.go). One definition, so the two rails cannot
// drift into two notions of "advertises".
func payloadAdvertisesNLRI(payload []byte) bool {
	return wireu.PayloadAdvertisesNLRI(payload)
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

// OTCIngressFilter is the ingress filter function registered with the BGP filter pipeline (filterapi).
// Called by the reactor for each received UPDATE before caching and dispatching.
// Checks OTC ingress rules per RFC 9234 Section 5.
//
// Sets meta["src-role"] to the source peer's role from our config (e.g., "provider", "customer").
// The egress filter uses this for suppression decisions -- our configured knowledge of the
// peer relationship, independent of whether OTC is in the wire bytes.
// If we don't configure a role for a peer, we don't filter its routes.
func OTCIngressFilter(src filterapi.PeerFilterInfo, payload []byte, meta map[string]any) (bool, []byte) {
	cfg, capRole := getFilterConfig(src.Address.String())

	// Always record source peer's role in metadata from our configuration.
	if cfg != nil && cfg.role != "" {
		meta["src-role"] = cfg.role
	}

	// What the source peer IS to us. Falls back to the config complement when
	// the peer sent no Role capability: RFC 9234 Section 4.2 says "The locally
	// configured BGP Role is used for the procedures described in Section 5",
	// and Section 8 names the non-compliant remote as the case the local AS
	// must still stamp for. Taking capRole alone here skipped ALL THREE ingress
	// MUSTs for such a peer -- leak detection from a Customer/RS-Client, the
	// Peer ASN mismatch check, and the stamp that lets a leak be caught hops
	// away -- because remoteRole was "" and the guard below returned early.
	remoteRole := resolvePeerRole(capRole, cfg)

	// No role config or no resolvable peer role: no OTC filtering.
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
		recordDrop(dropLeak, src.Address, remoteRole)
		logger().Debug("OTC ingress reject: route leak",
			"peer", src.Address, "remote-role", remoteRole)
		return false, nil
	case otcTreatWithdraw:
		recordDrop(dropMalformedOTC, src.Address, remoteRole)
		logger().Info("OTC treat-as-withdraw: malformed OTC",
			"peer", src.Address)
		if wd := payloadToWithdrawal(payload); wd != nil {
			return true, wd
		}
		return false, nil
	}

	// Stamp OTC if needed. insertOTCInPayload returns nil on overflow.
	//
	// RFC 9234 Section 5 ingress rule 3 acts on "a route ... received". A
	// payload that advertises nothing carries none, and insertOTCInPayload
	// would rewrite a withdraw-only UPDATE (or an End-of-RIB marker) into one
	// with a path attribute and no NLRI -- the shape RFC 4271 Section 4.3 says
	// such a message must not have (see payloadAdvertisesNLRI).
	if stampASN > 0 && payloadAdvertisesNLRI(payload) {
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

// resolveSrcRole returns OUR configured role toward the source peer, which is
// what the Gao-Rexford egress check keys on.
//
// meta["src-role"] is written by OTCIngressFilter (above) from
// getFilterConfig(src).role. It is therefore a CACHE of config, not an
// independent input, which is why config is a sound fallback and not a guess:
// both sides read the same field of the same peer's config.
//
// Precisely: they are the same VALUE only at a single instant. meta is captured
// when the route is RECEIVED; this fallback reads config when the route is
// FORWARDED. An operator changing the peer's import role between the two makes
// them differ. That is not a defect here -- the relay-time role is the one that
// describes the relationship the route is being forwarded under, and is the
// safer of the two to gate a leak check on -- but it is not the identity a
// reader might assume from "recovered from the same field".
//
// The fallback exists because not every egress caller has been through the
// ingress filter. RelayStoredRoute replays out of the Adj-RIB-In with no
// ingress metadata, and treating a missing key as "no restriction" silently
// skipped an RFC 9234 Section 5 leak guard on that path -- the zero-value trap
// in ai/rules/evidence.md, where the absent value selects the
// permissive branch. A relay path is not the only caller that can lack meta,
// so the fix is at the read, not at the one caller.
//
// An unusable value (present but not a string) takes the fallback too: a
// malformed input must never be MORE permissive than a missing one. When the
// source has no role config at all there is genuinely nothing to recover, and
// "" correctly means the peer is unconfigured rather than unrestricted.
func resolveSrcRole(meta map[string]any, srcCfg *peerRoleConfig) string {
	if raw, present := meta["src-role"]; present {
		if role, ok := raw.(string); ok && role != "" {
			return role
		}
		// Fail closed AND say so (ai/rules/evidence.md). The fallback
		// below already denies correctly; without this line a producer bug is
		// unobservable. The risk is real, not theoretical: ingressMeta is one
		// map shared by every in-process ingress filter
		// (reactor_notify.go:450-453), so any filter can clobber the key.
		logger().Warn("OTC src-role present but unusable, falling back to config",
			"value", raw)
	}
	if srcCfg != nil {
		return srcCfg.role
	}
	return ""
}

// peerRoleComplement maps OUR configured local role toward a peer to what that
// peer IS to us. It is the value form of the RFC 9234 Section 4.2 Table 2 pair
// table in validate.go (validRolePairs), which is keyed by wire code.
var peerRoleComplement = map[string]string{
	roleCustomer: roleProvider,
	roleProvider: roleCustomer,
	roleRSClient: roleRS,
	roleRS:       roleRSClient,
	rolePeer:     rolePeer,
}

// resolvePeerRole returns what a peer IS to us, for the RFC 9234 Section 5
// procedures. It serves BOTH directions: the source peer on ingress and the
// destination peer on egress.
//
// capRole is the role the peer announced in its OPEN Role capability.
// setFilterRemoteRole is the ONLY writer of filterRemoteRoles (role.go:77) and
// its only caller is guarded by len(remoteRoles) > 0 (role.go:169-174), so
// capRole is empty for any peer that did not send the capability -- a case
// validateOpenRolePair deliberately ACCEPTS when strict is unset, quoting RFC
// 9234 Section 4.2's SHOULD-ignore (validate.go:88-94).
//
// Empty then selected the permissive branch of every Section 5 gate. That is
// the zero-value trap of ai/rules/evidence.md, and it was present at
// all THREE readers of getFilterConfig, not just the two that sit two lines
// apart. RFC 9234 Section 4.2 settles which value the procedures take: "The
// locally configured BGP Role is used for the procedures described in Section
// 5." So config is the prescribed input, not a consolation prize -- the RFC
// names the capability-less peer as the expected early-adopter case.
//
// Scope, deliberately narrow: this feeds the RFC MUST gates (ingress leak and
// stamp rules, egress leak suppression and stamping), NOT the export-set match.
// Export filtering treats a capability-less peer as roleUnknown, and "unknown"
// is a documented operator token meaning "also send to peers with no role
// configured" (config.go:36). Reclassifying it there would silently retarget
// that knob, which is policy, not conformance.
func resolvePeerRole(capRole string, cfg *peerRoleConfig) string {
	if capRole != "" {
		return capRole
	}
	if cfg == nil {
		return ""
	}
	role, ok := peerRoleComplement[cfg.role]
	if !ok && cfg.role != "" {
		// Fail closed AND say so (ai/rules/evidence.md). Unreachable
		// while parseRoleContainer rejects any name outside roleValues
		// (config.go:72), so this fires only if that validation and this table
		// drift apart -- exactly the case a silent "" would hide.
		logger().Warn("role has no peer-role complement, RFC 9234 gates cannot resolve this peer",
			"configured-role", cfg.role)
	}
	return role
}

// OTCEgressFilter is the egress filter function registered with the BGP filter pipeline (filterapi).
// Called by the reactor per destination peer during ForwardUpdate.
// Checks both export role filtering and OTC egress suppression per RFC 9234 Section 5.
//
// Two independent egress checks:
//  1. Wire-bytes OTC check (unconditional): if route has OTC, MUST NOT propagate to Provider/Peer/RS.
//  2. Meta-based Gao-Rexford check: if source role is Provider/Peer/RS, suppress to Provider/Peer/RS.
func OTCEgressFilter(src, dest filterapi.PeerFilterInfo, payload []byte, meta map[string]any, mods *filterapi.ModAccumulator) bool {
	// RFC 9234 Section 5: OTC MUST NOT be applied to other address families by default.
	if !isPayloadUnicast(payload) {
		return true
	}

	srcCfg, _ := getFilterConfig(src.Address.String())
	destCfg, destCapRole := getFilterConfig(dest.Address.String())

	// destRemoteRole gates the RFC 9234 Section 5 MUSTs and falls back to the
	// config complement when the peer sent no Role capability (resolvePeerRole).
	// destCapRole stays capability-only for the operator export set below, where
	// "unknown" is a documented target rather than a missing answer.
	destRemoteRole := resolvePeerRole(destCapRole, destCfg)

	// RFC 9234 Section 5 egress rule 2 (unconditional, wire-bytes):
	// "If a route already contains the OTC Attribute, it MUST NOT be
	// propagated to Providers, Peers, or RSes."
	// This check does not depend on source peer configuration.
	if checkOTCEgress(destRemoteRole, extractAttrsFromPayload(payload)) {
		recordDrop(dropOTCPresent, dest.Address, destRemoteRole)
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
		srcRole := resolveSrcRole(meta, srcCfg)
		if srcRole == roleCustomer || srcRole == rolePeer || srcRole == roleRSClient {
			recordDrop(dropSourceRole, dest.Address, destRemoteRole)
			logger().Debug("OTC egress suppress (src-role)",
				"src", src.Address, "src-role", srcRole, "dest", dest.Address, "dest-role", destRemoteRole)
			return false
		}
	}

	// Export role filtering: check if destination role is in the allowed set.
	// Uses pre-computed resolvedExport (resolved at config time, not per-UPDATE).
	//
	// A source with no role config gets no export filtering, but MUST still fall
	// through to the stamping rule below. This used to be an early
	// `if srcCfg == nil { return true }`, which also skipped the RFC MUST --
	// and the RFC does not condition that rule on the source at all. It made
	// every route from a config-less source (iBGP, an RR client, a locally
	// originated or API-injected route) reach a Customer WITHOUT OTC, so the
	// customer could leak it upward with nothing for a compliant neighbor to
	// catch, which is the entire purpose of the attribute.
	if srcCfg != nil && len(srcCfg.resolvedExport) > 0 {
		// Capability-only on purpose: "unknown" is an operator-selected export
		// target for peers whose role we do not know (config.go), not an
		// unanswered question. That covers a peer that announced no role AND a
		// peer whose OPEN was never recorded -- Thomas ruled the second one in on
		// 2026-08-03, see the suppression block below. See resolvePeerRole's
		// scope note.
		destRole := destCapRole
		if destRole == "" {
			destRole = roleUnknown
		}
		if !slices.Contains(srcCfg.resolvedExport, destRole) {
			// Operator policy, not an RFC gate -- but just as invisible when it
			// fires, and a mistyped export set is exactly how a peer's routes
			// disappear. Counted under its own reason so it is distinguishable
			// from the RFC suppressions above.
			//
			// OWNER RULING, 2026-08-03 (R6-1 / Q-1 in
			// plan/spec-fixit-stored-route-relay-hardening.md). Thomas decided
			// that a destination whose role was NEVER RECORDED still matches an
			// explicit `export { unknown }`: "KEEP MATCHING. Pin it as intended."
			// `unknown` is the operator's own word for "this peer's role is not
			// known to us", and an unrecorded peer is in exactly that state, so
			// honoring the token literally changes no working config. The
			// accepted cost is stated in the spec: during a validate-open RPC
			// failure or a plugin respawn, ze advertises to a peer whose role is
			// genuinely unknown.
			//
			// So roleUnknown above is a TOTAL answer over the destination-role
			// state, the membership test always evaluates a defined input, and
			// reaching this branch is a policy DECISION -- which is what
			// forwardUpdateCore counts it as, and what the stored-route relay
			// reports as a handled route.
			// TestExportSetUnrecordedStillMatchesExplicitUnknown
			// (role_recorded_test.go) is the test of that decision; do not
			// re-open it without a fresh owner ruling.
			//
			// The recorded/unrecorded split below is therefore a DIAGNOSIS for
			// the operator, not a second decision. A recorded empty role means
			// this peer's OPEN declared none. An UNRECORDED role means no OPEN
			// was ever seen for this peer, which broadcastValidateOpen
			// (internal/component/bgp/server/validate.go) reaches whenever it
			// skips the plugin and lets the session establish anyway. Both
			// suppress here, identically, and always have. What the second one
			// now does is SAY SO (ai/rules/evidence.md): its own counter, and
			// recordDrop's first-occurrence WARN. It used to be reported as an
			// export-set decision against role "unknown", which sent an operator
			// to check a policy when the thing to check was validate-open.
			//
			// The recorded lookup is taken HERE, on the cold suppression path,
			// rather than beside the config read above: the accepting path never
			// needs it and must not pay a second lock (ai/rules/performance.md).
			recorded := remoteRoleRecorded(dest.Address.String())
			reason := dropExportSet
			if !recorded {
				reason = dropRoleUnrecorded
			}
			recordDrop(reason, dest.Address, destRole)
			logger().Debug("OTC egress suppress (export set)",
				"src", src.Address, "dest", dest.Address, "dest-role", destRole,
				"role-recorded", recorded,
				"export", srcCfg.resolvedExport)
			return false // Destination role not in export set.
		}
	}

	// RFC 9234 Section 5: "If a route is to be advertised to a Customer, a Peer,
	// or an RS-Client [...] and the OTC Attribute is not present, then [...]
	// an OTC Attribute MUST be added with a value equal to the AS number of the local AS."
	//
	// "is to be advertised" is the first clause of the rule, and it is a
	// condition, not scene-setting. Without payloadAdvertisesNLRI this block
	// queued an OTC mod for a pure withdrawal, for an MP_UNREACH-only UPDATE
	// and for an End-of-RIB marker, and the reactor's unconsumed-ops pass wrote
	// the 7 bytes onto the wire (forward_build.go:242-259 calls the handler
	// with src=nil, so nothing downstream declines to fabricate the attribute).
	// See payloadAdvertisesNLRI for why that output is interop-fatal rather
	// than merely useless.
	if mods != nil && payloadAdvertisesNLRI(payload) &&
		(destRemoteRole == roleCustomer || destRemoteRole == rolePeer || destRemoteRole == roleRSClient) {
		attrs := extractAttrsFromPayload(payload)
		_, hasOTC, _ := findOTC(attrs)
		if !hasOTC {
			// RFC 9234 R008: stamp OTC with our local AS for this session. The
			// reactor hands the effective per-peer local AS in dest.LocalAS
			// (peer_forward_facts.go); it is never re-parsed from raw config here.
			localASN := dest.LocalAS
			if localASN > 0 {
				var asnBuf [otcAttrLen]byte
				binary.BigEndian.PutUint32(asnBuf[:], localASN)
				mods.Op(otcAttrCode, filterapi.AttrModSet, asnBuf[:]) // value bytes only (4-byte ASN)
				logger().Debug("OTC egress stamp mod",
					"src", src.Address, "dest", dest.Address, "dest-role", destRemoteRole, "otc-asn", localASN)
			} else {
				// Fail closed and say so (ai/rules/evidence.md): a peer
				// with no local AS cannot be established (config parsing rejects
				// it), so a zero here signals a wiring gap, not a valid answer.
				logger().Warn("OTC egress stamp skipped: no local AS for destination peer",
					"src", src.Address, "dest", dest.Address, "dest-role", destRemoteRole)
			}
		}
	}

	return true
}

// otcAttrModHandler is the AttrModHandler for OTC (type 35).
//
// It PLANS the attribute rather than writing it: the plan carries the exact
// output size, so the buffer the rebuild acquires already has room and the old
// "buffer overflow, skipping stamp" branch cannot occur. That branch was
// fail-open — it silently emitted a route missing the marker RFC 9234 requires.
//
// RFC 9234 Section 5: "Once the OTC Attribute has been set, it MUST be preserved unchanged."
// If the source already has OTC it is kept unchanged and the set op is ignored.
func otcAttrModHandler(p *filterapi.AttrPlan) {
	_, suppress := filterapi.LastSetOrSuppress(p.Ops())

	// OTC already present in source: preserve unchanged.
	//
	// A Suppress operation is REFUSED here rather than honored, and that is a
	// deliberate asymmetry with every other handler. RFC 9234 Section 5:
	// "Once the OTC Attribute has been set, it MUST be preserved unchanged."
	// Removing the attribute is a change, so honoring the suppression would
	// break a gated MUST (RFC9234-5-6).
	//
	// But it is refused OUT LOUD. Reading Set alone, as this handler used to,
	// discarded a Suppress operation in exactly the silence that let the same
	// blind spot in the community handler ship a fail-open: the operation was
	// consumed, the attribute re-emitted, and nothing said so. A guard that
	// neither denies nor speaks does not exist (ai/rules/evidence.md).
	//
	// No production producer emits Suppress for code 35 today, so this branch is
	// latent. It exists so the producer that eventually does gets a line naming
	// the refusal instead of rediscovering the silence.
	if p.Source() != nil {
		if suppress {
			logger().Warn("OTC suppression refused: RFC 9234 Section 5 requires a set OTC attribute to be preserved unchanged",
				"attribute-code", otcAttrCode)
		}
		p.KeepAll()
		return
	}

	// OTC absent. A Suppress means the attribute must not be created, so it
	// overrides any Set that came before it -- the ordinary last-wins rule,
	// which the RFC preservation clause above does not reach because there is
	// nothing yet to preserve.
	if suppress {
		p.Drop()
		return
	}

	// OTC absent: create from the first set op's value bytes.
	for i, op := range p.Ops() {
		if op.Action != filterapi.AttrModSet || len(op.Buf) != otcAttrLen {
			continue
		}
		p.Op(i)
		p.Emit(otcAttrFlags, otcAttrCode)
		return
	}
	p.Drop()
}
