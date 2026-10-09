// Design: docs/architecture/testing/interop.md -- non-vacuous RFC 9252 wire proof.
// Related: helper_srv6_identity.go -- SDK receipts and literal expected TLV bytes.
package bgp

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// srv6IdentityAttributes parses this fixture's UPDATE boundary independently of
// the production Prefix-SID parser/rewriter. Values borrow the caller's bytes.
func srv6IdentityAttributes(body []byte) (map[byte][]byte, error) {
	if len(body) < 4 {
		return nil, errors.New("short UPDATE body")
	}
	if binary.BigEndian.Uint16(body[:2]) != 0 {
		return nil, errors.New("unexpected legacy withdrawals")
	}
	if int(binary.BigEndian.Uint16(body[2:4])) != len(body)-4 {
		return nil, errors.New("attribute length mismatch or unexpected legacy NLRI")
	}
	attributes := make(map[byte][]byte)
	for cursor := 4; cursor < len(body); {
		if len(body)-cursor < 3 {
			return nil, errors.New("short attribute header")
		}
		flags, code := body[cursor], body[cursor+1]
		length, header := int(body[cursor+2]), 3
		if flags&0x10 != 0 {
			if len(body)-cursor < 4 {
				return nil, errors.New("short extended attribute header")
			}
			length, header = int(binary.BigEndian.Uint16(body[cursor+2:cursor+4])), 4
		}
		if length > len(body)-cursor-header {
			return nil, errors.New("truncated attribute value")
		}
		if _, duplicate := attributes[code]; duplicate {
			return nil, fmt.Errorf("duplicate attribute %d", code)
		}
		attributes[code] = body[cursor+header : cursor+header+length]
		cursor += header + length
	}
	return attributes, nil
}

func srv6IdentityPrefix(reach []byte, nextHop string) (int, error) {
	if len(reach) != 30 {
		return 0, fmt.Errorf("MP_REACH length %d, want 30", len(reach))
	}
	if !bytes.Equal(reach[:4], []byte{0, 2, 1, 16}) {
		return 0, fmt.Errorf("wrong negotiated carrier %x", reach[:4])
	}
	hop := netip.MustParseAddr(nextHop).As16()
	if !bytes.Equal(reach[4:20], hop[:]) {
		return 0, fmt.Errorf("next hop %x, want %s", reach[4:20], nextHop)
	}
	if !bytes.Equal(reach[20:29], []byte{0, 64, 0x20, 1, 0x0d, 0xb8, 0x92, 0x52, 0}) {
		return 0, fmt.Errorf("wrong Reserved/NLRI %x", reach[20:])
	}
	if reach[29] < 1 {
		return 0, errors.New("unexpected prefix zero")
	}
	if reach[29] > 3 {
		return 0, errors.New("unexpected prefix above three")
	}
	return int(reach[29]), nil
}

// srv6IdentitySemanticError identifies a mismatch only after a complete capture
// passes every non-semantic fence. Its empty value is an empty diagnostic.
type srv6IdentitySemanticError string

func (err srv6IdentitySemanticError) Error() string { return string(err) }

// requireSRv6IdentityWire requires a real subject, a separate attribute-free
// control, and a later FIFO fence on the original session. Absence alone never
// passes. OPEN-only relays preserve every UPDATE exactly as Ze emitted it.
func requireSRv6IdentityWire(capture string, changed bool) error {
	hop := srv6IdentityA
	wantSID := srv6IdentityLabel + srv6IdentityServices + srv6IdentityOther
	if changed {
		hop = srv6IdentityB
		wantSID = srv6IdentityLabel + srv6IdentityOther
	}
	var seen [4]int
	var semanticErr error
	opens, keepalives := 0, 0
	for line := range strings.SplitSeq(strings.TrimSpace(capture), "\n") {
		var row extendedRelayFrame
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return err
		}
		frame, err := hex.DecodeString(row.Original)
		if err != nil {
			return err
		}
		if len(frame) < 19 {
			return errors.New("short captured BGP frame")
		}
		if int(binary.BigEndian.Uint16(frame[16:18])) != len(frame) {
			return errors.New("captured BGP length mismatch")
		}
		for _, octet := range frame[:16] {
			if octet != 0xff {
				return errors.New("captured BGP marker mismatch")
			}
		}
		// The wire type is open input; reject all non-fixture message types.
		switch frame[18] {
		case 1:
			opens++
		case 4:
			keepalives++
		case 2:
			if row.Delivered != "" {
				return errors.New("relay altered an UPDATE")
			}
			attrs, err := srv6IdentityAttributes(frame[19:])
			if err != nil {
				return err
			}
			if len(attrs) == 0 {
				continue
			}
			if len(attrs) == 1 && bytes.Equal(attrs[15], []byte{0, 2, 1}) {
				continue
			}
			prefix, err := srv6IdentityPrefix(attrs[14], hop)
			if err != nil {
				return err
			}
			if _, found := attrs[15]; found {
				return errors.New("unexpected MP_UNREACH alongside subject")
			}
			if !bytes.Equal(attrs[1], []byte{0}) {
				return errors.New("subject ORIGIN changed")
			}
			if !bytes.Equal(attrs[2], []byte{2, 2, 0, 0, 0xfd, 0xe9, 0, 0, 0xfd, 0xec}) {
				return fmt.Errorf("AS_PATH %x, want 65001 65004", attrs[2])
			}
			if prefix == 1 {
				if hex.EncodeToString(attrs[40]) != wantSID {
					// A mismatching subject alone proves nothing about the rest
					// of the stream. Structural/control failures take precedence.
					semanticErr = fmt.Errorf("RFC9252 selective semantics changed=%t: Prefix-SID=%x want=%s", changed, attrs[40], wantSID)
				}
			} else if _, found := attrs[40]; found {
				return fmt.Errorf("attribute-free control %d acquired Prefix-SID", prefix)
			}
			seen[prefix]++
			if prefix == 3 {
				if seen[1] != 1 {
					return errors.New("FIFO fence arrived without exactly one subject")
				}
				if seen[2] != 1 {
					return errors.New("FIFO fence arrived without exactly one distinct control")
				}
			}
		default:
			return fmt.Errorf("unexpected BGP message %d (including NOTIFICATION)", frame[18])
		}
	}
	if opens != 1 {
		return fmt.Errorf("captured %d OPENs, want original session only", opens)
	}
	if keepalives == 0 {
		return errors.New("capture has no session KEEPALIVE")
	}
	for prefix := 1; prefix <= 3; prefix++ {
		if seen[prefix] != 1 {
			return fmt.Errorf("prefix %d captured %d times, want one", prefix, seen[prefix])
		}
	}
	if semanticErr != nil {
		return srv6IdentitySemanticError(semanticErr.Error())
	}
	return nil
}

func requireSRv6IdentityPolicy(output, peer string) error {
	var receipt srv6IdentityPolicyReceipt
	if err := json.Unmarshal([]byte(output), &receipt); err != nil {
		return err
	}
	if receipt.Attempts != 4 {
		return fmt.Errorf("SDK callback attempts=%d (five means five or more), want three announcements and one EOR", receipt.Attempts)
	}
	if receipt.Failure != "" {
		return fmt.Errorf("SDK callback failure retained: %s", receipt.Failure)
	}
	if receipt.Rejected != (srv6IdentityRejected{}) {
		return errors.New("SDK callback rejected-input evidence exists without failure text")
	}
	if len(receipt.Calls) != 4 {
		return fmt.Errorf("SDK callback receipts=%d, want three announcements and one EOR", len(receipt.Calls))
	}
	for index, call := range receipt.Calls {
		if call.Peer != peer {
			return fmt.Errorf("policy ran for %s, want only %s", call.Peer, peer)
		}
		before, err := hex.DecodeString(call.Input)
		if err != nil {
			return err
		}
		after, err := hex.DecodeString(call.Output)
		if err != nil {
			return err
		}
		if index == 3 {
			// Literal oracle independent of the helper's EOR discriminator:
			// zero legacy withdrawals, six attribute octets, MP_UNREACH v6/u.
			if call.Input != "00000006800f03000201" || !bytes.Equal(before, after) {
				return errors.New("fourth SDK callback is not the exact unchanged IPv6 EOR")
			}
			if call.Action != sdk.FilterAccept {
				return errors.New("SDK policy did not accept IPv6 EOR unchanged")
			}
			continue
		}
		if call.Action != sdk.FilterModify {
			return errors.New("SDK policy did not modify announcement next hop")
		}
		input, err := srv6IdentityAttributes(before)
		if err != nil {
			return err
		}
		result, err := srv6IdentityAttributes(after)
		if err != nil {
			return err
		}
		prefix, err := srv6IdentityPrefix(input[14], srv6IdentityA)
		if err != nil {
			return err
		}
		outPrefix, err := srv6IdentityPrefix(result[14], srv6IdentityB)
		if err != nil {
			return err
		}
		if prefix != outPrefix {
			return errors.New("SDK policy changed NLRI")
		}
		if prefix != index+1 {
			return errors.New("SDK policy announcement population is duplicated or out of order")
		}
		if len(input) != len(result) {
			return errors.New("SDK policy changed attribute inventory")
		}
		for code, value := range input {
			if code == 14 {
				continue
			}
			if !bytes.Equal(value, result[code]) {
				return fmt.Errorf("SDK policy modified non-next-hop attribute %d", code)
			}
		}
		if prefix == 1 {
			want := srv6IdentityDirtyLabel + srv6IdentityServices + srv6IdentityOther
			if hex.EncodeToString(input[40]) != want {
				return fmt.Errorf("public policy did not receive complete original Prefix-SID: %x", input[40])
			}
		} else if _, present := input[40]; present {
			return errors.New("SDK policy control unexpectedly carries Prefix-SID")
		}
	}
	return nil
}
