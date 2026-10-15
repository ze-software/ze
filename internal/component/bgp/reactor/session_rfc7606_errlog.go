// Design: docs/architecture/wire/messages.md -- RFC 7606 error log at the default level.
// RFC: rfc/short/rfc7606.md -- Section 6 error logging.
// RFC: rfc/short/rfc8669.md -- Sections 4, 4.1 and 6 error logging on discard.
// Related: session_validation.go -- applyRFC7606 and rfc7606LateAttributeError, the callers.
// Related: log_window.go -- the rate limit, one window per attribute code.

package reactor

import (
	"time"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// rfc7606ErrorLogInterval bounds how often ONE attribute code from one peer is
// logged. A peer decides how often it sends a malformed UPDATE, so without the
// bound it would decide how fast the operator's log grows.
const rfc7606ErrorLogInterval = time.Second

// rfc7606ErrorLogMessage is the message of the RFC 7606 error record. It covers
// a malformed attribute and one Ze refuses by rule (a Prefix-SID from outside the
// SR domain, a LOCAL_PREF from an external peer); "description" says which.
const rfc7606ErrorLogMessage = "BGP UPDATE from peer failed validation, part of it was dropped"

// rfc7606ErrorNextStep tells the operator what to do with the record.
const rfc7606ErrorNextStep = "read description: ask the peer's operator to correct the UPDATE, " +
	"or change this peer's config if description names a setting; " +
	"set ze.log.bgp.reactor.session=debug to log the whole UPDATE"

// rfc7606ErrorLog holds one log window per attribute code. The code set is
// closed at 256, so a peer cannot grow it. 4 KiB, allocated the first time a
// peer sends a malformed attribute.
type rfc7606ErrorLog struct {
	windows [256]logWindow
}

// logRFC7606Error logs, at Error level, that this peer's UPDATE was malformed
// and what Ze did about it (event: attribute-discard, treat-as-withdraw,
// nlri-syntax). The caller MUST call it once for each RFC 7606 action other
// than session reset, which rfc7606ResetNotification logs at Warn. The Debug
// diagnostics (rfc7606Diagnostics) still carry the NLRI and the whole UPDATE.
//
// RFC 7606 Section 6: "At a minimum, such facilities must include logging an
// error listing the NLRI involved and containing the entire malformed UPDATE
// message when such an attribute is detected."
// RFC 8669 Section 6: "When discarding an attribute, a BGP speaker SHOULD log an
// error for further analysis."
// RFC 8669 Section 4.1: "A BGP speaker SHOULD log an error for further analysis."
//
// Error, not Warn: RFC 8669 Section 4.1 asks for "an error" for an invalid
// attribute and "a warning" for a conflicting one in the same paragraph, so the
// word names a severity. Both levels pass the default WARN filter.
//
// Rate-limited per peer (this Session) and per attribute code: the first record
// of a code logs at once, later ones in the same rfc7606ErrorLogInterval are
// counted, and the next record reports the count as "suppressed". A suppressed
// call allocates nothing. Safe for concurrent use.
func (s *Session) logRFC7606Error(event string, attrCode uint8, description string) {
	limit := s.rfc7606Errors.Load()
	if limit == nil {
		// The loser of a race uses the winner's windows, so no count is lost.
		s.rfc7606Errors.CompareAndSwap(nil, new(rfc7606ErrorLog))
		limit = s.rfc7606Errors.Load()
	}
	emit, suppressed := limit.windows[attrCode].allow(s.clock.Now().UnixNano(), rfc7606ErrorLogInterval)
	if !emit {
		return
	}
	sessionLogger().Error(rfc7606ErrorLogMessage,
		"peer", s.settings.Address,
		"event", event,
		"attr", attrCode,
		"attr-name", attribute.AttributeCode(attrCode).String(),
		"description", description,
		"suppressed", suppressed,
		"next-step", rfc7606ErrorNextStep)
}
