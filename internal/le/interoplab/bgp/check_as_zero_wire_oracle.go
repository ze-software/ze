// Design: docs/architecture/testing/interop.md -- AS-zero raw-write wire proof.
// Related: check_as_zero_wire.go -- the actual operator command and native RIB.
// FRR: https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_attr.c
package bgp

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/capability"
)

const (
	asZeroRawScenario = "bgp-as-zero-raw-frr"
	asZeroWireCapture = "/tmp/as-zero-wire"
	asZeroFencePrefix = "10.76.255.0/24"
)

// asZeroWireInput is a fixture command pair, not a decoded route or a verdict.
type asZeroWireInput struct {
	code       byte
	prefix     string
	clean      []byte
	zero       []byte
	path       string
	aggregator uint32
}

func asZeroWireInputs(nextHop netip.Addr) []asZeroWireInput {
	inputs := make([]asZeroWireInput, 0, 4)
	for ordinal, code := range []byte{2, 17, 7, 18} {
		path := "65001 65020"
		if code == 17 {
			path = "65001 70000"
		}
		aggregator := uint32(0)
		if code == 7 {
			aggregator = 65020
		}
		if code == 18 {
			aggregator = 70000
		}
		inputs = append(inputs, asZeroWireInput{
			code: code, prefix: fmt.Sprintf("10.76.%d.0/24", ordinal),
			clean:      asZeroWireBody(code, byte(ordinal), false, nextHop),
			zero:       asZeroWireBody(code, byte(ordinal), true, nextHop),
			path:       path,
			aggregator: aggregator,
		})
	}
	return inputs
}

// asZeroWireBody builds independent, fixed IPv4 /24 fixture bytes. Code zero
// means the final clean fence, with no optional attribute. The AS_PATH's first
// AS is always Ze's 65001, so even the bad path passes FRR's first-AS check.
// RFC 7607 Section 2: "A BGP speaker MUST NOT originate or propagate a route
// with an AS number of zero in the AS_PATH, AS4_PATH, AGGREGATOR, or
// AS4_AGGREGATOR attributes."
// UPDATE body: [0:2] withdrawn length | [2:4] attribute length | attributes |
// one NLRI [prefix length=24, 10, 76, ordinal]. AS_PATH and AGGREGATOR use the
// configured OLD width; compatibility attributes retain four-octet ASN fields.
func asZeroWireBody(code, ordinal byte, zero bool, nextHop netip.Addr) []byte {
	path := []byte{2, 2, 0xfd, 0xe9, 0xfd, 0xfc} // 65001 65020, two octets each
	value := []byte{0, 1, 0x11, 0x70}            // non-mappable 70000
	if code == 17 {
		copy(path[4:], []byte{0x5b, 0xa0})
	} // AS_TRANS
	if zero {
		value = []byte{0, 0, 0, 0}
		if code == 2 {
			copy(path[4:], value[:2])
		}
	}
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, byte(len(path))}
	attrs = append(attrs, path...)
	attrs = append(attrs, 0x40, 3, 4)
	hop := nextHop.As4()
	attrs = append(attrs, hop[:]...)
	// These are the three optional attributes in this fixture, not a registry.
	switch code {
	case 17:
		attrs = append(attrs, 0xc0, code, 10, 2, 2, 0, 0, 0xfd, 0xe9)
		attrs = append(attrs, value...)
	case 7:
		asn := []byte{0xfd, 0xfc}
		if zero {
			asn = []byte{0, 0}
		}
		attrs = append(attrs, 0xc0, code, 6)
		attrs = append(attrs, asn...)
		attrs = append(attrs, 192, 0, 2, 9)
	case 18:
		attrs = append(attrs, 0xc0, 7, 6, 0x5b, 0xa0, 192, 0, 2, 9)
		attrs = append(attrs, 0xc0, code, 8)
		attrs = append(attrs, value...)
		attrs = append(attrs, 192, 0, 2, 9)
	}
	body := []byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))}
	body = append(body, attrs...)
	return append(body, 24, 10, 76, ordinal)
}

func asZeroWireWithdrawal(ordinal byte) []byte {
	return []byte{0, 4, 24, 10, 76, ordinal, 0, 0}
}

// judgeASZeroWire judges a declared interval ending at the complete final
// UPDATE, never an absence window. The expected announcements are exact raw
// input bytes; the replacements MUST be withdrawals of those same subjects.
// Bytes beyond the fence lie outside this interval. Empty/truncated captures,
// changed OPENs and an AS-zero leak rejected later by FRR cannot pass. Native
// UPDATE contents are not the egress obligation: FRR can export received eBGP
// bestpaths back to this peer. Their framing and attribute envelopes are checked
// while OPEN, KEEPALIVE and session-generation checks witness connection health.
func judgeASZeroWire(text string, ze, native netip.Addr, fromNative bool) (bool, error) {
	if len(text) > 2*1024*1024 {
		return false, errors.New("AS-zero capture exceeds its bounded fixture size")
	}
	expected := make([][]byte, 0, 9)
	for _, input := range asZeroWireInputs(ze) {
		expected = append(expected, input.clean)
	}
	for ordinal := range 4 {
		expected = append(expected, asZeroWireWithdrawal(byte(ordinal)))
	}
	expected = append(expected, asZeroWireBody(0, 255, false, ze))
	decoder := json.NewDecoder(strings.NewReader(text))
	opened, keepalives, updates := false, 0, 0
	for ordinal := range 256 {
		var row extendedRelayFrame
		if err := decoder.Decode(&row); err != nil {
			if errors.Is(err, io.EOF) {
				if fromNative {
					return opened && keepalives > 0, nil
				}
				return false, nil
			}
			return false, err
		}
		if ordinal == 255 {
			return false, errors.New("AS-zero capture exhausted its frame bound")
		}
		if row.Delivered != "" {
			return false, errors.New("AS-zero relay changed a frame")
		}
		if row.Complete != nil {
			return false, errors.New("unexpected relay completion marker")
		}
		frame, err := hex.DecodeString(row.Original)
		if err != nil {
			return false, err
		}
		if len(frame) < bgpHeaderLength {
			return false, errors.New("short AS-zero capture frame")
		}
		size, err := validateSpeakerMessageHeader(frame[:bgpHeaderLength])
		if err != nil {
			return false, err
		}
		if size != len(frame) {
			return false, errors.New("AS-zero capture length differs from its header")
		}
		if frame[18] == bgpOpen {
			if opened {
				return false, errors.New("AS-zero capture changed session generation")
			}
			if err := requireASZeroWireOpen(frame, ze, native, fromNative); err != nil {
				return false, err
			}
			opened = true
			continue
		}
		if !opened {
			return false, errors.New("AS-zero capture lacks its original OPEN")
		}
		switch frame[18] {
		case bgpKeepalive:
			keepalives++
		case bgpUpdate:
			if keepalives == 0 {
				return false, errors.New("AS-zero UPDATE precedes handshake witness")
			}
			body := frame[bgpHeaderLength:]
			if fromNative {
				if _, err := parsedEmptyMPUpdate(body); err != nil {
					return false, fmt.Errorf("malformed native AS-zero UPDATE: %w", err)
				}
				continue
			}
			if bytes.Equal(body, []byte{0, 0, 0, 0}) {
				if updates != 0 {
					return false, errors.New("unexpected EOR inside AS-zero command sequence")
				}
				continue
			}
			if !bytes.Equal(body, expected[updates]) {
				return false, fmt.Errorf("AS-zero wire step %d differs: got %x, want %x", updates+1, body, expected[updates])
			}
			updates++
			if updates == len(expected) {
				return true, nil
			}
		default:
			// This is an open wire type; this scenario requests no other traffic.
			return false, fmt.Errorf("unexpected AS-zero capture type %d", frame[18])
		}
	}
	return false, errors.New("AS-zero capture has no complete bounded interval")
}

func requireASZeroWireOpen(frame []byte, ze, native netip.Addr, fromNative bool) error {
	if err := validateSpeakerOpen(frame[bgpHeaderLength:]); err != nil {
		return err
	}
	asn, address := uint16(65001), ze
	if fromNative {
		asn, address = 65002, native
	}
	router := address.As4()
	if binary.BigEndian.Uint16(frame[20:22]) != asn {
		return errors.New("AS-zero capture names another speaker AS")
	}
	if !bytes.Equal(frame[24:28], router[:]) {
		return errors.New("AS-zero capture names another speaker identifier")
	}
	caps, err := capability.ParseFromOptionalParams(frame[29:], false)
	if err != nil {
		return err
	}
	asn4 := 0
	for _, entry := range caps {
		switch advertised := entry.(type) {
		case *capability.ASN4:
			if advertised.ASN != uint32(asn) {
				return errors.New("inconsistent AS-zero session ASN4 capability")
			}
			asn4++
		case *capability.AddPath:
			if !fromNative {
				return errors.New("Ze offered ADD-PATH in the unframed AS-zero fixture")
			}
		}
	}
	wantASN4 := 0
	if fromNative {
		wantASN4 = 1
	}
	if asn4 != wantASN4 {
		return errors.New("AS-zero capture differs from its configured OLD-to-NEW capability boundary")
	}
	return nil
}

// requireASZeroFRRPath reads the native decoder, not an echo of raw input.
// FRR 10.7.1 bgp_route.c route_vty_out_detail renders peerId, aspath.string,
// aggregatorAs and aggregatorId. An expected aggregator of zero means this
// fixture input has no aggregator obligation; zero is never a valid twin ASN.
func requireASZeroFRRPath(text, prefix, neighbor, asPath string, aggregator uint32) error {
	var route struct {
		Prefix string `json:"prefix"`
		Paths  []struct {
			Valid bool `json:"valid"`
			Peer  struct {
				ID string `json:"peerId"`
			} `json:"peer"`
			ASPath struct {
				Value string `json:"string"`
			} `json:"aspath"`
			Aggregator   *uint32    `json:"aggregatorAs"`
			AggregatorID netip.Addr `json:"aggregatorId"`
		} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(text), &route); err != nil {
		return err
	}
	if route.Prefix != prefix {
		return errors.New("FRR omitted the exact AS-zero control prefix")
	}
	if len(route.Paths) != 1 {
		return errors.New("FRR AS-zero control does not have exactly one native path")
	}
	path := route.Paths[0]
	if !path.Valid {
		return errors.New("FRR rejected the valid AS-zero control path")
	}
	if path.Peer.ID != neighbor {
		return errors.New("FRR AS-zero control came from another neighbor")
	}
	if path.ASPath.Value != asPath {
		return errors.New("FRR decoded another AS_PATH for the valid control")
	}
	if aggregator != 0 {
		if path.Aggregator == nil {
			return errors.New("FRR omitted the valid twin's decoded aggregator")
		}
		if *path.Aggregator != aggregator {
			return errors.New("FRR did not decode the valid twin's aggregator ASN")
		}
		if path.AggregatorID != netip.MustParseAddr("192.0.2.9") {
			return errors.New("FRR did not decode the valid twin's aggregator identifier")
		}
	}
	return nil
}
