// Design: docs/architecture/wire/capabilities.md — capability negotiation

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

	// RFC 4271 Section 4.2: Negotiated Hold Time (minimum of local and peer).
	HoldTime uint16

	// RFC 4724: Graceful Restart Mechanism for BGP.
	GracefulRestart *GracefulRestart

	// RFC 5492 Section 3: Capabilities that were not negotiated.
	// Tracked for logging/reporting purposes.
	Mismatches []Mismatch
}
