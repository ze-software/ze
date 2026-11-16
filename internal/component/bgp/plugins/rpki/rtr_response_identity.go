// Design: docs/architecture/plugin/rib-storage-design.md -- RTR response identity
// RFC: rfc/short/rfc8210.md -- Sections 5.1, 5.5, 5.8, and 7
// RFC: rfc/short/draft-ietf-sidrops-8210bis.md -- Sections 5.12, 7, and 12
package rpki

import "errors"

var (
	errRtrSessionIDMismatch    = errors.New("rtr: cache response session ID does not match its query or response")
	errRtrUnexpectedVersion    = errors.New("rtr: cache changed the protocol version during a session")
	errRtrMissingCacheResponse = errors.New("rtr: cache sent payload or End of Data before Cache Response")
)

// rtrResponseIdentity is the identity accepted in a Cache Response. A nil
// response means that no Cache Response has begun the current query's payloads.
// RFC 8210 Section 5.8: "The Session ID and Protocol Version MUST be the same as that of the
// corresponding Cache Response which began the (possibly null) sequence of payload PDUs."
type rtrResponseIdentity struct {
	version   uint8
	sessionID uint16
}

// checkStreamSequence requires an opening Cache Response on the live transport,
// without changing the isolated PDU handler used by non-stream callers.
// RFC 8210 Section 5.8: "The Session ID and Protocol Version MUST be the same as that of the
// corresponding Cache Response which began the (possibly null) sequence of payload PDUs."
// Sections 8.1 and 8.2 place Cache Response before the sequence of payload PDUs.
func (s *RTRSession) checkStreamSequence(hdr rTRHeader, buf []byte) error {
	// RFC 8210 Section 7: version refusal belongs to handlePDU, which the
	// stream calls immediately after this check, so Unsupported Protocol
	// Version takes precedence over a missing Cache Response.
	if hdr.Version < rtrVersionMin {
		return nil
	}
	if hdr.Version > rtrVersionMax {
		return nil
	}
	// refuse-by-default: guards RTRSession.handlePDU -- nil delegates non-payload and unknown PDUs to its control handling or refusal.
	switch hdr.Type {
	case pduIPv4Prefix, pduIPv6Prefix, pduASPA, pduRouterKey, pduEndOfData:
		s.mu.Lock()
		started := s.response != nil
		version := s.version
		s.mu.Unlock()
		if started {
			return nil
		}
		if hdr.Type == pduASPA {
			// Draft-ietf-sidrops-8210bis Section 5.12: "If it does not, an
			// Error Report PDU with Error Code 9 ("ASPA Provider List Error") MUST
			// be returned by the router."
			// Preserve the provider-list diagnostic by pure validation on this
			// error path. No valid early ASPA reaches the staging handler.
			if version >= 2 {
				if _, _, err := parseASPAPDU(buf); err != nil {
					return err
				}
			}
		}
		return errRtrMissingCacheResponse
	}
	return nil
}

// checkResponseIdentity validates the identity before any payload is staged or
// published. Only Session ID-bearing PDUs compare bytes 2-3; payload PDUs use
// those bytes for flags or reserved fields instead.
//
// RFC 8210 Section 5.1: "the party which detects the mismatch MUST immediately terminate the
// session with an Error Report PDU with code 0 ("Corrupt Data"), and the router MUST flush all
// data learned from that cache."
// RFC 8210 Section 5.5: "In response to a Reset Query, the new value of the Session ID tells
// the router the instance of the cache session for future confirmation."
//
// The common RTR header (RFC 8210 Section 5) is:
//
//	byte     0        1          2-3             4-7
//	     +---------+------+----------------+------------+
//	     | Version | Type | Session ID (*) |   Length   |
//	     +---------+------+----------------+------------+
//
// (*) Cache Response, End of Data, and Serial Notify only.
func (s *RTRSession) checkResponseIdentity(hdr rTRHeader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.response != nil {
		// RFC 8210 Section 7: "If either party receives a PDU for a different Protocol Version
		// once the above negotiation completes, that party MUST drop the session; unless the
		// PDU containing the unexpected Protocol Version was itself an Error Report PDU, the
		// party dropping the session SHOULD send an Error Report with an error code of 8
		// ("Unexpected Protocol Version")."
		if hdr.Version != s.response.version {
			return errRtrUnexpectedVersion
		}
	}
	if s.response == nil {
		// RFC 8210 Section 7; draft-ietf-sidrops-8210bis Section 7.
		// A remembered serial does not negotiate this new transport. Let
		// handlePDU refuse an unknown initial version with Error Code 4.
		if hdr.Version < rtrVersionMin {
			return nil
		}
		if hdr.Version > rtrVersionMax {
			return nil
		}
	}

	mismatch := false
	if hdr.Type == pduCacheResp {
		if s.serial != 0 {
			// RFC 8210 Section 5.1: "routers MUST treat sessions with different Protocol Version
			// fields as separate sessions even if they do happen to have the same Session ID."
			if hdr.Version != s.version {
				return errRtrUnexpectedVersion
			}
			mismatch = hdr.SessionID != s.sessionID
		}
	}
	if s.response != nil {
		switch hdr.Type {
		case pduCacheResp, pduEndOfData, pduSerialNotify:
			// RFC 8210 Sections 5.1 and 5.8.
			mismatch = hdr.SessionID != s.response.sessionID
		}
	}
	if mismatch {
		return errRtrSessionIDMismatch
	}
	return nil
}

// rtrResponseErrorCode classifies protocol failures independently from reply
// eligibility. A received Error Report can require fatal cleanup without a reply.
// RFC 8210 Section 12: "Errors which are considered fatal MUST cause the session to be dropped."
func rtrResponseErrorCode(hdr rTRHeader, err error) (uint16, bool) {
	switch {
	case errors.Is(err, errRtrMissingCacheResponse):
		// RFC 8210 Section 12: "0: Corrupt Data (fatal):  The receiver believes the received PDU to
		// be corrupt in a manner not specified by another error code."
		return 0, true
	case errors.Is(err, errRtrSessionIDMismatch):
		// RFC 8210 Section 5.1: "immediately terminate the session with an Error Report PDU with
		// code 0 ("Corrupt Data"), and the router MUST flush all data learned from that cache."
		return 0, true
	case errors.Is(err, errRtrUnexpectedVersion):
		// RFC 8210 Section 7: "the party dropping the session SHOULD send an Error Report with an
		// error code of 8 ("Unexpected Protocol Version")."
		return 8, true
	case hdr.Type == pduErrorRpt:
		// RFC 8210 Section 12; draft-ietf-sidrops-8210bis Section 12.
		return hdr.SessionID, true
	case errors.Is(err, errRtrUnsupportedProtocol):
		// RFC 8210 Section 7: "it MUST either downgrade to a known version or terminate the
		// connection, with an Error Report PDU unless the received PDU is itself an Error
		// Report PDU."
		return errUnsupportedVersion, true
	case hdr.Type == pduASPA:
		// 8210bis Section 5.12: an invalid provider list requires Error Report 9;
		// other malformed ASPA PDUs carry Corrupt Data (0).
		if errors.Is(err, errASPAProviderList) {
			return 9, true
		}
		return 0, true
	}
	return 0, false
}

// revokeResponseData revokes a fatal cache's current generation before any
// Error Report write, using the trusted session version rather than the version
// in the failed PDU. The caller MUST close the transport and then run the
// returned notification, when non-nil, even when the report cannot be written.
// RFC 8210 Section 5.1: "the router MUST flush all data learned from that cache."
// Draft-ietf-sidrops-8210bis Section 12: "Errors which are considered fatal MUST cause the
// session to be dropped, and the router MUST flush all data learned from that cache."
func (s *RTRSession) revokeResponseData(err error, code uint16) func() {
	s.mu.Lock()
	if !errors.Is(err, errRtrSessionIDMismatch) {
		if s.version < 2 {
			s.mu.Unlock()
			return nil
		}
		// Draft-ietf-sidrops-8210bis Section 12 classifies these as non-fatal:
		// "2: No Data Available (non-fatal)", "4: Unsupported Protocol Version
		// (non-fatal)", and "12: Cache Restart (non-fatal)".
		switch code {
		case errNoDataAvail, errUnsupportedVersion, 12:
			s.mu.Unlock()
			return nil
		}
	}
	s.serial = 0
	s.synced = false
	s.fullSync = true
	s.response = nil
	s.pendingVRPs = nil
	s.pendingDels = nil
	s.pendingASPAs = nil
	s.pendingASPADels = nil
	lease, generation := s.dataLeaseLocked(), s.dataGeneration
	s.mu.Unlock()
	// RFC 8210 Section 5.1; draft-ietf-sidrops-8210bis Section 12.
	return lease.invalidate(generation)
}
