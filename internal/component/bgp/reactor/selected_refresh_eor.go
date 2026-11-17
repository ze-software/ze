// Design: docs/architecture/update-building.md -- actual EOR completion before BoRR.
// Related: session_wire.go -- accepted complete frames, under Session.writeMu.
// RFC 7313 Section 4 -- see rfc/short/rfc7313.md.
package reactor

import (
	"bytes"
	"encoding/binary"
	"slices"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
)

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
