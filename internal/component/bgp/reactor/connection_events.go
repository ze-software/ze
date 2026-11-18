// Design: docs/architecture/behavior/peer-lifecycle.md -- immutable physical Session identity.
package reactor

// connectionIDOf reads only the captured Session. Nil means unrecorded; no
// caller may substitute a current address lookup or initial-replay receipt.
func connectionIDOf(session *Session) uint64 {
	if session == nil {
		return 0
	}
	return session.connectionID
}
