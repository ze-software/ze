// Design: docs/architecture/l2tp/subscriber-session-model.md -- canonical subscriber identity
// Related: session.go -- subscriber snapshot and access namespace

package subscriber

import "github.com/ze-software/ze/internal/core/textbuf"

// SessionID names a subscriber within its access namespace. Transports MUST
// supply their full identifier before any narrowing to the PPP driver's key.
// The result is opaque to PPP and is the same key used by Session.ID.
func SessionID(access AccessType, transportID uint64, sessionID uint16) string {
	var buf textbuf.Buffer
	return buf.Str(string(access)).Byte('-').Uint(transportID).Byte('-').Uint16(sessionID).String()
}
