// Design: docs/architecture/testing/interop.md -- independent recipient wire evidence.
// Related: extended_relay.go -- bounded capture, UPDATEs never rewritten.
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
)

// parsedEmptyMPOpen reads the actual advertised encoding capabilities.
// RFC 5492 Section 4: "Each capability is encoded as a triple <Capability Code,
// Capability Length, Capability Value>." OPEN offsets: [28] parameter length,
// [29:] parameter TLVs; capability TLVs occur inside parameter type 2.
func parsedEmptyMPOpen(frame []byte, asn4 bool) error {
	if len(frame) < 29 {
		return errors.New("short encoding-context OPEN")
	}
	if frame[18] != bgpOpen {
		return errors.New("encoding-context evidence is not OPEN")
	}
	if int(frame[28])+29 != len(frame) {
		return errors.New("OPEN optional-parameter length mismatch")
	}
	families := map[uint16]bool{}
	gotASN4 := false
	for params := frame[29:]; len(params) > 0; {
		if len(params) < 2 {
			return errors.New("short OPEN parameter")
		}
		length := int(params[1])
		if length+2 > len(params) {
			return errors.New("truncated OPEN parameter")
		}
		if params[0] == 2 {
			for caps := params[2 : 2+length]; len(caps) > 0; {
				if len(caps) < 2 {
					return errors.New("short capability")
				}
				size := int(caps[1])
				if size+2 > len(caps) {
					return errors.New("truncated capability")
				}
				switch caps[0] {
				case 1:
					if size != 4 {
						return errors.New("invalid MP capability length")
					}
					if caps[5] == 1 {
						families[binary.BigEndian.Uint16(caps[2:4])] = true
					}
				case 65:
					if size != 4 {
						return errors.New("invalid ASN4 capability length")
					}
					gotASN4 = true
				case 69:
					return errors.New("ADD-PATH confounds the encoding-context discriminator")
				}
				caps = caps[2+size:]
			}
		}
		params = params[2+length:]
	}
	if gotASN4 != asn4 {
		return fmt.Errorf("OPEN ASN4 = %v, want %v", gotASN4, asn4)
	}
	if !families[1] {
		return errors.New("OPEN does not advertise IPv4 unicast")
	}
	if !families[2] {
		return errors.New("OPEN does not advertise IPv6 unicast")
	}
	return nil
}

// parsedEmptyMPCapture validates the whole bounded stream before extracting any
// UPDATE. Original and delivered OPENs differ only by the existing code-6 relay;
// that difference never changes ASN4 or ADD-PATH.
func parsedEmptyMPCapture(text string, source bool) ([][]byte, error) {
	// RFC 8654 Sections 3-4: source has no relay; recipient uses the existing relay.
	capture, err := parseExtendedCapture(text, false, !source)
	if err != nil {
		return nil, err
	}
	if capture.notification != nil {
		return nil, fmt.Errorf("NOTIFICATION in measured session: %x", capture.notification)
	}
	decoder := json.NewDecoder(strings.NewReader(text))
	var bodies [][]byte
	for {
		var row extendedRelayFrame
		if err := decoder.Decode(&row); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		frame, err := hex.DecodeString(row.Original)
		if err != nil {
			return nil, err
		}
		if frame[18] == bgpOpen {
			// RFC 5492 Section 4 and RFC 6793 Section 3.
			if err := parsedEmptyMPOpen(frame, source); err != nil {
				return nil, err
			}
		}
		if frame[18] == bgpUpdate {
			bodies = append(bodies, frame[19:])
		}
	}
	return bodies, nil
}

// parsedEmptyMPUpdate refuses partial UPDATE sections rather than letting a
// forgiving decoder manufacture an empty field. RFC 4271 Section 4.3: "This
// 2-octets unsigned integer indicates the total length of the Path Attributes
// field in octets." The first uint16 bounds withdrawals; the second bounds attrs.
func parsedEmptyMPUpdate(body []byte) (speakerUpdate, error) {
	if len(body) < 4 {
		return speakerUpdate{}, errors.New("short UPDATE")
	}
	withdrawn := int(binary.BigEndian.Uint16(body))
	if withdrawn+4 > len(body) {
		return speakerUpdate{}, errors.New("truncated withdrawn field")
	}
	attrs := int(binary.BigEndian.Uint16(body[2+withdrawn:]))
	if withdrawn+4+attrs > len(body) {
		return speakerUpdate{}, errors.New("truncated attribute field")
	}
	update := decodeSpeakerUpdate(body)
	consumed := 0
	for _, attr := range update.attributes {
		header := 3
		if attr.flags&0x10 != 0 {
			header = 4
		}
		consumed += header + len(attr.value)
	}
	if consumed != attrs {
		return speakerUpdate{}, errors.New("malformed attribute envelope")
	}
	return update, nil
}

// parsedEmptyMPHistory inspects every UPDATE, including anything following the
// desired withdrawal. A later same-source announcement is the completion fence.
// RFC 4724 Section 2: "For any other address family, it is an UPDATE message
// that contains only the MP_UNREACH_NLRI attribute [BGP-MP] with no withdrawn
// routes for that <AFI, SAFI>." A mixed input is not this standalone marker.
func parsedEmptyMPHistory(bodies [][]byte, source bool, phase int) error {
	fence := byte(100 + phase)
	seed, initialEOR := 0, 0
	stage := byte(0)
	withdrawals, eors := 0, 0
	seenFences := 0
	for _, body := range bodies {
		// RFC 4271 Section 4.3: inspect exact section boundaries.
		update, err := parsedEmptyMPUpdate(body)
		if err != nil {
			return err
		}
		isEOR := len(update.withdrawn) == 0 && len(update.nlri) == 0 && len(update.attributes) == 1 &&
			update.attributes[0].code == mpUnreach && bytes.Equal(update.attributes[0].value, []byte{0, 2, 1})
		if isEOR {
			if stage == 0 {
				initialEOR++
				continue
			}
			eors++
			if stage != 102 {
				return errors.New("fabricated IPv6 EOR in mixed-withdrawal epoch")
			}
			continue
		}
		if len(update.withdrawn) > 0 {
			if stage != 101 {
				return errors.New("withdrawal outside the seeded mixed epoch")
			}
			if !bytes.Equal(update.withdrawn, []byte{24, 198, 51, 100}) {
				return fmt.Errorf("wrong real withdrawal: %x", update.withdrawn)
			}
			if len(update.nlri) != 0 {
				return errors.New("withdrawal unexpectedly announces routes")
			}
			want := []byte{0, 4, 24, 198, 51, 100, 0, 0}
			if source {
				want = []byte{0, 4, 24, 198, 51, 100, 0, 6, 0x80, 15, 3, 0, 2, 1}
			}
			if !bytes.Equal(body, want) {
				return fmt.Errorf("withdrawal body %x, want %x", body, want)
			}
			withdrawals++
			continue
		}
		if len(update.nlri) == 0 {
			if stage == 0 && bytes.Equal(body, []byte{0, 0, 0, 0}) {
				continue // Initial IPv4 EOR is outside the measured epoch.
			}
			return fmt.Errorf("unexpected route-free UPDATE %x", body)
		}
		if len(update.nlri) != 4 {
			return fmt.Errorf("unexpected announcement length %d", len(update.nlri))
		}
		if !bytes.Equal(update.nlri[:3], []byte{24, 198, 51}) {
			return fmt.Errorf("unexpected announcement %x", update.nlri)
		}
		path := []byte{2, 1, 0xfd, 0xec}
		if source {
			path = []byte{2, 1, 0, 0, 0xfd, 0xec}
		}
		paths := 0
		for _, attr := range update.attributes {
			if attr.code == 2 {
				paths++
				if !bytes.Equal(attr.value, path) {
					return fmt.Errorf("AS_PATH %x, want negotiated encoding %x", attr.value, path)
				}
			}
		}
		if paths != 1 {
			return errors.New("announcement lacks exactly one AS_PATH")
		}
		subnet := update.nlri[3]
		if subnet == 100 {
			if stage != 0 {
				return errors.New("seed replayed after measurement began")
			}
			seed++
			continue
		}
		wantFence := byte(101 + seenFences)
		if subnet != wantFence {
			return fmt.Errorf("fence %d, want %d", subnet, wantFence)
		}
		seenFences++
		stage = subnet
	}
	if seed != 1 {
		return fmt.Errorf("seed announcements = %d, want one", seed)
	}
	if initialEOR == 0 {
		return errors.New("missing distinct initial IPv6 EOR control")
	}
	if stage != fence {
		return fmt.Errorf("last recipient fence = %d, want %d", stage, fence)
	}
	wantWithdrawals := 0
	if phase >= 2 {
		wantWithdrawals = 1
	}
	if withdrawals != wantWithdrawals {
		return fmt.Errorf("real withdrawals = %d, want %d", withdrawals, wantWithdrawals)
	}
	wantEORs := 0
	if phase == 3 {
		wantEORs = 1
	}
	if eors != wantEORs {
		return fmt.Errorf("explicit later IPv6 EORs = %d, want %d", eors, wantEORs)
	}
	return nil
}
