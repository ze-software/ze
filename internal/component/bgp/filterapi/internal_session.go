// Design: docs/architecture/core-design.md -- RFC 7705 Internal BGP AS Migration
// Overview: filterapi.go -- PeerFilterInfo.IsIBGP, the verdict this rule fills
// RFC: rfc/short/rfc7705.md -- AS migration mechanisms

package filterapi

// InternalSession reports whether a session with peerAS is internal, for a
// speaker whose local AS is localAS and whose RFC 7705 migration AS is
// migrationAS (zero when none is configured).
//
// This is the ONE rule. The reactor reaches it through PeerSettings.isIBGPWith
// (session_as_migration.go), and a plugin that classifies a peer from the
// config it was delivered (bgp-role's roleless report) calls it with the ASNs
// that config states, so the two never disagree about one session. It lives
// here because filterapi is the near-leaf both import.
//
// The ordinary rule is one equality: a session is internal when the peer's AS
// is ours. RFC 7705 Section 4.2 adds the second ASN of a migrating router to
// "ours", and requires the session to be internal either way.
//
// Safe for concurrent use: it reads only its arguments.
func InternalSession(peerAS, localAS, migrationAS uint32) bool {
	if peerAS == localAS {
		return true
	}
	if migrationAS == 0 {
		return false
	}
	// RFC 7705 Section 4.2: "In each case, the BGP speaker MUST treat UPDATEs
	// sent and received to this peer as if this was a natively configured iBGP
	// session, as defined by [RFC4271] and [RFC4456]."
	return peerAS == migrationAS
}
