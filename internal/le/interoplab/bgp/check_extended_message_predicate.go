// Design: docs/architecture/testing/interop.md -- fail-closed foreign wire predicates.
// Related: extended_relay.go -- records frames only after forwarding them.
// FRR JSON: https://github.com/FRRouting/frr/blob/frr-10.3.1/bgpd/bgp_route.c
package bgp

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	extendedBaselinePrefix = "10.86.0.0/24"
	extendedLargePrefix    = "10.86.1.0/24"
	extendedControlPrefix  = "10.86.2.0/24"
	extendedCommunities    = 400
)

// extendedCapture is a summary of one bounded, original-direction transcript.
// An empty transcript is not evidence: parseExtendedCapture requires one OPEN
// and a KEEPALIVE before it answers successfully.
type extendedCapture struct {
	keepalives   int
	updatesLarge int
	largeOctets  int
	control      bool
	notification []byte
}

// parseExtendedCapture proves the original OPEN and its delivered counterpart
// independently. In particular, a relay-added local advertisement cannot satisfy
// Ze's receive permission. It also rejects rewritten non-OPENs and invalid framing.
// RFC 8654 Section 4: "The BGP Extended Message Capability applies to all messages
// except for OPEN and KEEPALIVE messages."
func parseExtendedCapture(text string, originalExtended, deliveredExtended bool) (extendedCapture, error) {
	var result extendedCapture
	if len(text) > 34*1024*1024 {
		return result, errors.New("relay capture exceeds bounded frame budget")
	}
	decoder := json.NewDecoder(strings.NewReader(text))
	opens := 0
	var buffer [65535]byte
	for index := 0; ; index++ {
		var row extendedRelayFrame
		if err := decoder.Decode(&row); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return result, fmt.Errorf("decode relay row: %w", err)
		}
		if index == 256 {
			return result, errors.New("relay capture has more than 256 frames")
		}
		frame, err := hex.DecodeString(row.Original)
		if err != nil {
			return result, err
		}
		// RFC 4271 Section 4.1 and RFC 8654 Section 2: exact wire framing.
		octets, err := readExtendedFrame(bytes.NewReader(frame), buffer[:])
		if err != nil {
			return result, err
		}
		if octets != len(frame) {
			return result, errors.New("capture length differs from BGP header")
		}
		if frame[18] == bgpOpen {
			opens++
			if len(frame) > 4096 {
				return result, errors.New("extended OPEN observed")
			}
			// RFC 8654 Sections 3 and 4: compare exact OPEN-only transformation.
			written, advertised, rewriteErr := rewriteExtendedOpen(buffer[:], frame, deliveredExtended)
			if rewriteErr != nil {
				return result, rewriteErr
			}
			if advertised != originalExtended {
				return result, errors.New("original OPEN capability differs from configured permission")
			}
			delivered, decodeErr := hex.DecodeString(row.Delivered)
			if decodeErr != nil {
				return result, decodeErr
			}
			if !bytes.Equal(buffer[:written], delivered) {
				return result, errors.New("delivered OPEN changed more than capability 6")
			}
			continue
		}
		if opens != 1 {
			return result, errors.New("message outside the single captured OPEN session")
		}
		if row.Delivered != "" {
			return result, errors.New("relay rewrote a non-OPEN message")
		}
		switch frame[18] {
		case bgpKeepalive:
			if len(frame) != bgpHeaderLength {
				return result, errors.New("KEEPALIVE is not 19 octets")
			}
			result.keepalives++
		case bgpNotification:
			if result.notification != nil {
				return result, errors.New("multiple NOTIFICATIONs in one session")
			}
			result.notification = frame
		case bgpUpdate:
			if len(frame) > 4096 {
				result.updatesLarge++
				if extendedFramePrefix(frame, 1) {
					result.largeOctets = len(frame)
				}
			}
			if extendedFramePrefix(frame, 2) {
				result.control = true
			}
		default:
			// This is an open wire type, but this fixture requests no other
			// traffic. An unrecognized type cannot establish test evidence.
			return result, fmt.Errorf("unexpected relay message type %d", frame[18])
		}
	}
	if opens != 1 {
		return result, errors.New("capture does not contain exactly one OPEN")
	}
	if result.keepalives == 0 {
		return result, errors.New("capture contains no KEEPALIVE")
	}
	return result, nil
}

// extendedFramePrefix recognizes the fixture's IPv4 /24 NLRI without using Ze's
// production UPDATE decoder. It never treats an attribute octet as a prefix.
// RFC 4271 Section 4.3: "This variable length field contains a list of IP address
// prefixes." The two uint16 lengths precede withdrawn NLRI and path attributes;
// the ordinary IPv4 announced NLRI is the remaining UPDATE body.
func extendedFramePrefix(frame []byte, subnet byte) bool {
	if len(frame) < 23 {
		return false
	}
	withdrawn := int(binary.BigEndian.Uint16(frame[19:21]))
	attributeOffset := 21 + withdrawn
	if attributeOffset+2 > len(frame) {
		return false
	}
	offset := attributeOffset + 2 + int(binary.BigEndian.Uint16(frame[attributeOffset:]))
	for offset < len(frame) {
		bits := int(frame[offset])
		if bits > 32 {
			return false
		}
		octets := (bits + 7) / 8
		if offset+1+octets > len(frame) {
			return false
		}
		if bits == 24 {
			if bytes.Equal(frame[offset+1:offset+4], []byte{10, 86, subnet}) {
				return true
			}
		}
		offset += 1 + octets
	}
	return false
}

// requireExtendedRejection checks the exact Bad Message Length response, including
// the offending length copied from FRR's actual >4096 UPDATE header.
// RFC 8654 Section 4: "If a BGP message with a length greater than 4,096 octets is
// received by a BGP listener who has not advertised the BGP Extended Message
// Capability, the listener will generate a NOTIFICATION with the Error Subcode
// set to Bad Message Length ([RFC4271], Section 6.1)."
func requireExtendedRejection(frame []byte, offendingOctets int) error {
	if len(frame) != 23 {
		return errors.New("Bad Message Length NOTIFICATION must contain the offending two-octet length")
	}
	if frame[18] != bgpNotification {
		return errors.New("expected NOTIFICATION")
	}
	if frame[19] != 1 {
		return errors.New("expected Message Header Error")
	}
	if frame[20] != 2 {
		return errors.New("expected Bad Message Length")
	}
	if int(binary.BigEndian.Uint16(frame[21:23])) != offendingOctets {
		return errors.New("NOTIFICATION identifies a different offending message length")
	}
	return nil
}

// requireExtendedFRRRoute requires FRR's received path, not its locally originated
// best path. Both coexist deliberately, because one FRR daemon is the producer
// and the consumer on two independent eBGP sessions.
func requireExtendedFRRRoute(output, prefix, neighbor string, large bool) error {
	var route struct {
		Prefix string `json:"prefix"`
		Paths  []struct {
			Valid bool `json:"valid"`
			Peer  struct {
				ID string `json:"peerId"`
			} `json:"peer"`
			ASPath struct {
				String string `json:"string"`
			} `json:"aspath"`
			LargeCommunity struct {
				List []string `json:"list"`
			} `json:"largeCommunity"`
		} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(output), &route); err != nil {
		return err
	}
	if route.Prefix != prefix {
		return errors.New("FRR returned the wrong prefix")
	}
	for index := range route.Paths {
		path := &route.Paths[index]
		if path.Peer.ID != neighbor {
			continue
		}
		if !path.Valid {
			return errors.New("FRR received an invalid path")
		}
		if path.ASPath.String != "65001 65002" {
			return errors.New("FRR did not decode the Ze-forwarded AS_PATH")
		}
		if !large {
			return nil
		}
		if len(path.LargeCommunity.List) != extendedCommunities {
			return errors.New("FRR did not decode all 400 large communities")
		}
		var community textbuf.Buffer
		for ordinal, value := range path.LargeCommunity.List {
			want := community.Reset().Str("65002:8654:").Int(int64(ordinal + 1)).String()
			if value != want {
				return errors.New("FRR large community inventory differs from the originated attribute")
			}
		}
		return nil
	}
	return errors.New("FRR has no path received from the sink session")
}
func extendedFRRSessionClosed(output, neighbor string) (bool, error) {
	var peers map[string]struct {
		State       string `json:"bgpState"`
		Connections uint64 `json:"connectionsEstablished"`
	}
	if err := json.Unmarshal([]byte(output), &peers); err != nil {
		return false, err
	}
	peer, found := peers[neighbor]
	if !found {
		return false, errors.New("FRR omitted the tested neighbor")
	}
	if peer.Connections != 1 {
		return false, errors.New("FRR did not retain the original session generation")
	}
	if peer.State == "" {
		return false, errors.New("FRR omitted session state")
	}
	return peer.State != stateEstablished, nil
}
