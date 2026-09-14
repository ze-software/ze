// Design: docs/architecture/wire/capabilities.md — capability negotiation
// RFC: rfc/short/rfc5492.md — per-session capability state

package capability

// SessionCaps holds session-level capabilities (not encoding-related).
// Owned by Negotiated only (not shared with EncodingContexts).
// Immutable after session creation.
//
// Note: ExtendedMessage moved to EncodingCaps because it affects wire encoding
// (max message size: 4096 vs 65535).
type SessionCaps struct {
	// RFC 2918: Route Refresh Capability for BGP-4.
	RouteRefresh bool

	// RFC 7313: Enhanced Route Refresh Capability for BGP.
	EnhancedRouteRefresh bool

	// draft-ietf-idr-bgp-bfd-strict-mode Section 3, attribute 20
	// (BfdStrictNegotiated): TRUE when both speakers advertised the BFD
	// Strict-Mode capability.
	BFDStrictMode bool

	// draft-ietf-idr-linklocal-capability Section 2: TRUE when both speakers
	// advertised the Link-Local Next Hop capability, which is the condition
	// every procedure of Sections 3 to 6 is scoped to.
	LinkLocalNextHop bool

	// RFC 4271 Section 4.2: Negotiated Hold Time (minimum of local and peer).
	HoldTime uint16

	// RFC 4724: Graceful Restart Mechanism for BGP.
	GracefulRestart *GracefulRestart

	// RFC 5492 Section 3: Capabilities that were not negotiated.
	// Tracked for logging/reporting purposes.
	Mismatches []Mismatch
}
