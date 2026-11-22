// Design: docs/architecture/update-building.md -- actual EOR completion before BoRR.
// Related: session_wire.go -- accepted complete frames, under Session.writeMu.
// RFC 7313 Section 4 -- see rfc/short/rfc7313.md.
package reactor

import (
	"bytes"
	"encoding/binary"
	"errors"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
)

var errBoRRBeforeEOR = errors.New("bgp: cannot send BoRR because EOR has not reached this session's wire for this family when Graceful Restart is supported")

// checkBoRRAllowedHeld enforces RFC 7313 Section 4: "For a BGP speaker that
// supports the BGP Graceful Restart, it MUST NOT send a BoRR for an <AFI, SAFI>
// to a neighbor before it sends the EoR for the <AFI, SAFI> to the neighbor."
// Caller MUST hold s.writeMu.
func (s *Session) checkBoRRAllowedHeld(fam family.Family) error {
	if !s.localGR {
		return nil
	}
	if s.wireWriter == nil {
		return errBoRRBeforeEOR
	}
	if !s.wireWriter.sentEOR[fam] {
		return errBoRRBeforeEOR
	}
	return nil
}

// checkRawBoRRAllowedHeld enforces checkBoRRAllowedHeld atomically under
// s.writeMu when a complete wire ROUTE-REFRESH BoRR frame is written through
// SendRawMessage, closing any TOCTOU across session replacement or queue drain.
// Caller MUST hold s.writeMu.
func (s *Session) checkRawBoRRAllowedHeld(frame []byte) error {
	if len(frame) != message.HeaderLen+routeRefreshBodyLen {
		return nil
	}
	if frame[18] != byte(msgtype.TypeROUTEREFRESH) {
		return nil
	}
	body := frame[message.HeaderLen:]
	if message.RouteRefreshSubtype(body[routeRefreshSubtypeOffset]) != message.RouteRefreshBoRR {
		return nil
	}
	fam := family.Family{
		AFI:  family.AFI(binary.BigEndian.Uint16(body[0:2])),
		SAFI: family.SAFI(body[3]),
	}
	return s.checkBoRRAllowedHeld(fam)
}

// normalizeReceivedRouteRefresh enforces RFC 7313 Section 4 / RFC 2918 Section 3:
// "The procedures described in this section are applicable only when a BGP
// speaker has received the 'Enhanced Route Refresh Capability' from a peer."
// When the peer did not advertise capability 70, octet 2 of a 4-octet
// ROUTE-REFRESH body is RFC 2918's Reserved field ("ignored by the receiver"),
// not an RFC 7313 Message Subtype, so downstream consumers must observe subtype 0.
func (s *Session) normalizeReceivedRouteRefresh(body []byte) []byte {
	if s == nil {
		return body
	}
	s.mu.RLock()
	neg := s.negotiated
	s.mu.RUnlock()
	if neg == nil {
		return body
	}
	if neg.PeerAdvertised(capability.CodeEnhancedRouteRefresh) {
		return body
	}
	if len(body) != routeRefreshBodyLen {
		return body
	}
	if body[routeRefreshSubtypeOffset] == 0 {
		return body
	}
	out := make([]byte, routeRefreshBodyLen)
	copy(out, body)
	out[routeRefreshSubtypeOffset] = 0
	return out
}

// checkBoRRAllowed acquires s.writeMu and enforces the RFC 7313 Section 4
// Graceful Restart End-of-RIB gate before a BoRR may be sent for fam.
func (s *Session) checkBoRRAllowed(fam family.Family) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.checkBoRRAllowedHeld(fam)
}

// recordReceivedEOR records that the peer sent End-of-RIB for fam during the
// current negotiation epoch (RFC 7313 Section 4).
func (s *Session) recordReceivedEOR(fam family.Family) {
	if s.recvEORFor != s.negotiated {
		s.recvEOR = nil
		s.recvEORFor = s.negotiated
	}
	if s.recvEOR == nil {
		s.recvEOR = make(map[family.Family]bool, 2)
	}
	s.recvEOR[fam] = true
}

// routeRefreshBoRRBeforeEOR enforces RFC 7313 Section 4: "A BGP speaker that
// has received the Graceful Restart Capability from its neighbor MUST ignore
// any BoRRs for an <AFI, SAFI> from the neighbor before the speaker receives
// the EoR for the given <AFI, SAFI> from the neighbor. The BGP speaker SHOULD
// log an error of the condition for further analysis."
func (s *Session) routeRefreshBoRRBeforeEOR(body []byte) bool {
	if s.negotiated == nil {
		return false
	}
	if !s.negotiated.PeerAdvertised(capability.CodeEnhancedRouteRefresh) {
		return false
	}
	if !s.negotiated.PeerAdvertised(capability.CodeGracefulRestart) {
		return false
	}
	if len(body) < routeRefreshBodyLen {
		return false
	}
	if message.RouteRefreshSubtype(body[routeRefreshSubtypeOffset]) != message.RouteRefreshBoRR {
		return false
	}
	fam := family.Family{
		AFI:  family.AFI(binary.BigEndian.Uint16(body[0:2])),
		SAFI: family.SAFI(body[3]),
	}
	if s.recvEORFor == s.negotiated {
		if s.recvEOR[fam] {
			return false
		}
	}
	sessionLogger().Error("ignoring BoRR received before End-of-RIB from graceful-restart peer",
		"code", codeDoctorBGPBoRRBeforeEOR,
		"peer", s.settings.Address,
		"family", fam.String(),
	)
	return true
}

// complete observes exactly one accepted transport frame, never staged bytes.
// sentEOR belongs to this connection epoch and is bounded by its Session's
// negotiated family list, which already includes classic IPv4 without MP.
// It MUST NOT be copied into a replacement observedBGPWriter.
func (w *observedBGPWriter) complete(frame []byte) {
	// RFC 7313 Section 4. These are the complete RFC4724 EOR lengths:
	// BGP header[0:19], IPv4 empty UPDATE[19:23], or MP_UNREACH carrying
	// AFI[26:28]/SAFI[28] (extended attr length shifts that pair by one).
	switch len(frame) {
	case 23, 29, 30:
		w.completeEOR(frame)
	}
	if w.session.onWireMessage != nil {
		w.session.onWireMessage(frame, bgpctx.ContextID(w.context.Load()), true, w.transport)
	}
}

func (w *observedBGPWriter) completeEOR(frame []byte) {
	if frame[18] != byte(msgtype.TypeUPDATE) {
		return
	}
	body := frame[19:]
	var fam family.Family
	switch len(body) {
	case 4:
		if !bytes.Equal(body, []byte{0, 0, 0, 0}) {
			return
		}
		fam = family.IPv4Unicast
	case 10:
		// RFC 4760 Section 4: "This is an optional non-transitive attribute that can be used for the purpose of withdrawing multiple unfeasible routes from service."
		// Sole MP_UNREACH, optional/non-transitive/non-partial, normal length.
		if !bytes.Equal(body[:7], []byte{0, 0, 0, 6, 0x80, 15, 3}) {
			return
		}
		fam = family.Family{AFI: family.AFI(binary.BigEndian.Uint16(body[7:9])), SAFI: family.SAFI(body[9])}
	case 11:
		// The extended attribute-length encoding is legal even for length 3.
		if !bytes.Equal(body[:8], []byte{0, 0, 0, 7, 0x90, 15, 0, 3}) {
			return
		}
		fam = family.Family{AFI: family.AFI(binary.BigEndian.Uint16(body[8:10])), SAFI: family.SAFI(body[10])}
	default:
		return
	}
	// RFC 7606 Section 3(c) makes conflicting Optional/Transitive flags
	// malformed. Raw diagnostics may send them, but cannot create EOR proof.
	if !w.eorFamily(fam) {
		return
	}
	if w.sentEOR == nil {
		w.sentEOR = make(map[family.Family]bool)
	}
	w.sentEOR[fam] = true
}

func (w *observedBGPWriter) eorFamily(fam family.Family) bool {
	// Encoding IDs group compatible wire formats, not negotiated families.
	// Session.negotiated is published under writeMu, which this observer holds.
	if w.session.negotiated == nil {
		return false
	}
	return slices.Contains(w.session.negotiated.Families(), fam)
}
