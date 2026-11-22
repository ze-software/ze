// Design: docs/architecture/plugin/rib-storage-design.md -- RTR Router Key PDU parsing and duplicate detection
// RFC: rfc/short/rfc8210.md -- Section 5.10 (Router Key PDU and duplicate detection over {SKI, ASN, Subject Public Key})
// Related: rtr_pdu.go -- PDU constants and error codes
// Related: rtr_session.go -- RTRSession.handlePDU, applyEndOfData
// Related: rtr_response_identity.go -- rtrResponseErrorCode, revokeResponseData

package rpki

import (
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
)

var (
	errRtrDuplicateRouterKey       = errors.New("rtr: duplicate router key announcement received")
	errRtrUnknownRouterKeyWithdraw = errors.New("rtr: withdrawal of unknown router key")
)

// routerKeyRecord identifies a unique RFC 8210 Section 5.10 Router Key tuple:
// {Subject Key Identifier (20 bytes), AS Number (4 bytes), Subject Public Key Info (DER bytes)}.
//
// RFC 8210 Section 5.10: "Also note that it is possible, albeit very unlikely,
// for multiple distinct Subject Public Key values to hash to the same SKI. For
// this reason, implementations MUST compare Subject Public Key values as well
// as SKIs when detecting duplicate PDUs."
//
// Go map-key equality on this struct compares all 20 SKI bytes, the 32-bit ASN,
// and every byte of SPKI, so two distinct Subject Public Keys sharing an SKI
// never collide as duplicates.
type routerKeyRecord struct {
	SKI  [20]byte
	ASN  uint32
	SPKI string
}

// parseRouterKeyPDU parses an RFC 8210 Section 5.10 Router Key PDU (Type 9):
//
//	Offset  0        1        2        3
//	        Version  Type=9   Flags    Zero
//	        Length (4 octets, offset 4)
//	        Subject Key Identifier (20 octets, offset 8..27)
//	        AS Number (4 octets, offset 28..31)
//	        Subject Public Key Info (variable, offset 32..Length-1)
//
// Reserved flag bits (1..7) and byte 3 are ignored on receipt per RFC 8210
// Sections 5 and 5.1.
func parseRouterKeyPDU(buf []byte) (routerKeyRecord, bool, error) {
	if len(buf) < pduRouterKeyMinLen {
		return routerKeyRecord{}, false, fmt.Errorf("rtr: Router Key PDU too short: %d < %d", len(buf), pduRouterKeyMinLen)
	}
	if uint64(binary.BigEndian.Uint32(buf[4:8])) != uint64(len(buf)) {
		return routerKeyRecord{}, false, fmt.Errorf("rtr: Router Key PDU length does not match framing")
	}
	var rec routerKeyRecord
	copy(rec.SKI[:], buf[8:28])
	rec.ASN = binary.BigEndian.Uint32(buf[28:32])
	rec.SPKI = string(buf[32:])
	announce := buf[2]&1 != 0
	return rec, announce, nil
}

// applyStagedRouterKey stages one Router Key announcement or withdrawal
// against this session's in-flight key set and enforces RFC 8210 Section 5.10
// duplicate detection comparing Subject Public Key values as well as SKIs.
// Uses defer s.mu.Unlock() so a contained panic inside readLoop never leaves
// s.mu held when syncOnce closes the session.
//
// RFC 8210 Section 5.10:
//   - "Should the router client receive a Router Key PDU with a {SKI, ASN,
//     Subject Public Key} identical to one it already has active, it SHOULD
//     raise a Duplicate Announcement Received error."
//   - "For this reason, implementations MUST compare Subject Public Key values
//     as well as SKIs when detecting duplicate PDUs."
func (s *RTRSession) applyStagedRouterKey(rec routerKeyRecord, announce bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stagedRouterKeys == nil {
		if s.fullSync || len(s.activeRouterKeys) == 0 {
			s.stagedRouterKeys = make(map[routerKeyRecord]struct{})
		} else {
			s.stagedRouterKeys = maps.Clone(s.activeRouterKeys)
		}
	}
	if announce {
		if _, dup := s.stagedRouterKeys[rec]; dup {
			return errRtrDuplicateRouterKey
		}
		s.stagedRouterKeys[rec] = struct{}{}
		return nil
	}
	if _, exists := s.stagedRouterKeys[rec]; !exists {
		return errRtrUnknownRouterKeyWithdraw
	}
	delete(s.stagedRouterKeys, rec)
	return nil
}

// commitRouterKeysLocked publishes staged Router Key updates at End of Data.
// Caller MUST hold s.mu.
func (s *RTRSession) commitRouterKeysLocked(fullSync bool) {
	if s.stagedRouterKeys != nil {
		s.activeRouterKeys = s.stagedRouterKeys
		s.stagedRouterKeys = nil
		return
	}
	if fullSync {
		s.activeRouterKeys = nil
	}
}

// clearRouterKeysLocked discards all active and staged Router Keys for this
// session (on fatal error, session reset, or data revocation).
// Caller MUST hold s.mu.
func (s *RTRSession) clearRouterKeysLocked() {
	s.activeRouterKeys = nil
	s.stagedRouterKeys = nil
}
