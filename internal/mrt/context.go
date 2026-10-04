// Design: docs/architecture/mrt.md — file-local directional OPEN evidence.
package mrt

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// ErrContextUnavailable means the record cannot determine each family's NLRI mode.
var ErrContextUnavailable = errors.New("mrt: ADD-PATH decoding context unavailable or ambiguous; both directional OPENs are required")

// SessionContexts owns the OPEN evidence of one MRT stream. It is not concurrent.
// Callers MUST create a fresh value for each independently opened file/stream;
// ObserveMessage and ObserveState MUST see every record before filtering it.
// At most 65536 live endpoint identities are retained; overflow is refused.
type SessionContexts struct {
	peers map[sessionKey]*sessionEvidence
}

type sessionKey struct {
	peer, local netip.Addr
	iface       uint16
}

type sessionEvidence struct {
	opens      [2][]capability.Capability
	seen       [2]bool
	asn        [2]uint32
	negotiated *capability.Negotiated
	used       bool
}

func contextKey(h *BGP4MPHeader) (sessionKey, error) {
	peer, ok := netip.AddrFromSlice(h.PeerIP)
	if !ok {
		return sessionKey{}, ErrBadAFI
	}
	local, ok := netip.AddrFromSlice(h.LocalIP)
	if !ok {
		return sessionKey{}, ErrBadAFI
	}
	return sessionKey{peer: peer.Unmap(), local: local.Unmap(), iface: h.IfIndex}, nil
}

// ObserveState invalidates a terminated epoch, not the recorder's post-OPEN
// Idle-to-Established notification. RFC 6396 Section 4.4.1 records old/new FSM states.
func (c *SessionContexts) ObserveState(record *StateChangeRecord) error {
	key, err := contextKey(&record.BGP4MPHeader)
	if err != nil {
		return err
	}
	if record.NewState <= FSMActive {
		delete(c.peers, key)
	} else if record.OldState == FSMEstablished && record.NewState != FSMEstablished {
		delete(c.peers, key)
	}
	return nil
}

// ObserveMessage attaches immutable negotiation evidence without copying bytes.
// Callers MUST invoke it in stream order, including messages a filter will omit.
// RFC 7911 Section 5: "that BGP speaker MUST advertise the ADD-PATH Capability
// with the Send/Receive field set to either 2 or 3, and MUST receive from its peer
// the ADD-PATH Capability with the Send/Receive field set to either 1 or 3, for
// the corresponding <AFI, SAFI>.".
func (c *SessionContexts) ObserveMessage(subtype uint16, record *MessageRecord) error {
	key, err := contextKey(&record.BGP4MPHeader)
	if err != nil {
		return err
	}
	wire := &record.BGPMessage
	if len(wire.Bytes) < 19 {
		return errShortMessage
	}
	sent := subtype == BGP4MPMessageLocal || subtype == BGP4MPMessageAS4Local || subtype == BGP4MPMessageLocalAP || subtype == BGP4MPMessageAS4LocalAP
	direction := 0
	if sent {
		direction = 1
	}
	evidence := c.peers[key]
	switch wire.Bytes[18] {
	case 1:
		// A repeated OPEN or an OPEN after session traffic starts a new epoch.
		if evidence != nil && (evidence.used || evidence.seen[direction]) {
			delete(c.peers, key)
			evidence = nil
		}
		caps, asn, err := openEvidence(wire.Bytes)
		if err != nil {
			delete(c.peers, key)
			return fmt.Errorf("MRT OPEN evidence: %w", err)
		}
		if evidence == nil {
			if len(c.peers) >= 65536 {
				return errors.New("mrt: session context limit (65536) exceeded")
			}
			if c.peers == nil {
				c.peers = make(map[sessionKey]*sessionEvidence)
			}
			evidence = &sessionEvidence{}
			c.peers[key] = evidence
		}
		evidence.opens[direction] = caps
		evidence.asn[direction] = asn
		evidence.seen[direction] = true
		if evidence.seen[0] && evidence.seen[1] {
			evidence.negotiated = capability.Negotiate(evidence.opens[1], evidence.opens[0], capability.PeerIdentity{})
		}
	case 3:
		delete(c.peers, key)
	default:
		if evidence == nil {
			return nil
		}
		if evidence.negotiated == nil {
			// Traffic cannot fill the missing half of a handshake.
			evidence.used = true
			return nil
		}
		if wire.Bytes[18] != 2 {
			// Dynamic peers may still have header ASN zero on pre-established
			// KEEPALIVEs. Only UPDATEs consume the negotiated NLRI context.
			evidence.used = true
			return nil
		}
		if !openASNMatches(evidence.asn[0], record.PeerAS, IsAS4Subtype(subtype)) {
			delete(c.peers, key)
			return nil
		}
		if !openASNMatches(evidence.asn[1], record.LocalAS, IsAS4Subtype(subtype)) {
			delete(c.peers, key)
			return nil
		}
		evidence.used = true
		wire.negotiated = evidence.negotiated
		wire.sent = sent
	}
	return nil
}

func openASNMatches(actual, header uint32, as4 bool) bool {
	if actual == header {
		return true
	}
	return !as4 && actual > 65535 && header == 23456
}

// openEvidence reads capability framing from the actual OPEN, including RFC 9072.
// RFC 5492 Section 4: "The Capabilities Optional Parameter contains one or more
// triples <Capability Code, Capability Length, Capability Value>".
func openEvidence(wire []byte) ([]capability.Capability, uint32, error) {
	if len(wire) < 29 {
		return nil, 0, errShortMessage
	}
	if int(binary.BigEndian.Uint16(wire[16:18])) != len(wire) {
		return nil, 0, errShortMessage
	}
	for _, b := range wire[:16] {
		if b != 255 {
			return nil, 0, errBadMarker
		}
	}
	body := wire[19:]
	if body[0] != 4 {
		return nil, 0, errors.New("mrt: unsupported OPEN version")
	}
	offset, length := 10, int(body[9])
	extended := body[9] != 0 && len(body) > 10 && body[10] == 255
	if extended {
		if len(body) < 13 {
			return nil, 0, errShortMessage
		}
		offset, length = 13, int(binary.BigEndian.Uint16(body[11:13]))
	}
	if offset+length != len(body) {
		return nil, 0, errShortMessage
	}
	caps, err := capability.ParseFromOptionalParams(body[offset:], extended)
	if err != nil {
		return nil, 0, err
	}
	asn := uint32(binary.BigEndian.Uint16(body[1:3]))
	for _, cap := range caps {
		if as4, ok := cap.(*capability.ASN4); ok {
			asn = as4.ASN
		}
	}
	return caps, asn, nil
}

// AddPathFor answers from both actual OPENs when available, otherwise from the
// subtype after the caller has validated that the UPDATE is unambiguous.
func (m BGPMessage) AddPathFor(afi uint16, safi uint8) bool {
	if m.negotiated == nil {
		return m.AddPath
	}
	mode := m.negotiated.AddPathMode(family.Family{AFI: family.AFI(afi), SAFI: family.SAFI(safi)})
	if m.sent {
		return mode&capability.AddPathSend != 0
	}
	return mode&capability.AddPathReceive != 0
}

// updateContext refuses to infer different families' modes from one subtype.
// RFC 8050 Section 3: "These enhancements continue to encapsulate the entire
// BGP message in the BGP message field.".
func (m BGPMessage) updateContext(withdrawn []byte, attrs []PathAttribute, nlri []byte) error {
	if m.negotiated != nil {
		return nil
	}
	if !m.AddPath {
		return nil
	}
	var first family.Family
	if len(withdrawn)+len(nlri) != 0 {
		first = capability.FamilyImplicit
	}
	for _, attr := range attrs {
		if attr.Code != AttrMPReachNLRI && attr.Code != AttrMPUnreachNLRI {
			continue
		}
		value := attr.Value
		if len(value) < 3 {
			return ErrShortData
		}
		start := 3
		if attr.Code == AttrMPReachNLRI {
			if len(value) < 5 {
				return ErrShortData
			}
			start = 5 + int(value[3])
		}
		if start > len(value) {
			return ErrShortData
		}
		// An empty MP field still supplies subtype-selection evidence at the
		// recorder. Ignoring it here can invent IDs in another family's NLRI.
		current := family.Family{AFI: family.AFI(binary.BigEndian.Uint16(value[:2])), SAFI: family.SAFI(value[2])}
		if first == (family.Family{}) {
			first = current
		} else if first != current {
			return ErrContextUnavailable
		}
	}
	return nil
}
