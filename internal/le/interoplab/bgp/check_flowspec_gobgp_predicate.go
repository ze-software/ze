// Design: docs/architecture/testing/interop.md -- fail-closed FlowSpec observations.
// Related: extended_relay.go -- records complete frames after delivery, no UPDATE edits.
// GoBGP v3.31.0 JSON: https://github.com/osrg/gobgp/blob/v3.31.0/pkg/apiutil/util.go
// Components/actions: https://github.com/osrg/gobgp/blob/v3.31.0/pkg/packet/bgp/bgp.go
package bgp

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

type flowSpecWireState struct {
	frame   int
	present bool
}

// flowSpecWireEvidence reads the original sender frames in the single relay
// session. Every FlowSpec announcement, including baseline rules, must have no
// next hop. Only the tested NLRI advances the fence; the newest event wins.
// RFC 8955 Section 4: "When advertising Flow Specifications, the Length of the
// Next-Hop Network Address MUST be set to 0".
func flowSpecWireEvidence(text, routerID string) (flowSpecWireState, error) {
	var result flowSpecWireState
	if len(text) > 34*1024*1024 {
		return result, errors.New("FlowSpec capture exceeds the relay frame budget")
	}
	id, err := netip.ParseAddr(routerID)
	if err != nil {
		return result, err
	}
	decoder := json.NewDecoder(strings.NewReader(text))
	opens, keepalives := 0, 0
	var buffer [65535]byte
	for index := 1; ; index++ {
		var row extendedRelayFrame
		if err := decoder.Decode(&row); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return result, err
		}
		if index > 256 {
			return result, errors.New("FlowSpec capture exceeds 256 frames")
		}
		frame, err := hex.DecodeString(row.Original)
		if err != nil {
			return result, err
		}
		// RFC 4271 Section 4.1: complete length-framed BGP messages only.
		octets, err := readExtendedFrame(bytes.NewReader(frame), buffer[:])
		if err != nil {
			return result, err
		}
		if octets != len(frame) {
			return result, errors.New("FlowSpec capture has trailing message bytes")
		}
		if frame[18] == bgpOpen {
			opens++
			if opens != 1 {
				return result, errors.New("FlowSpec relay contains a replacement session")
			}
			if len(frame) < 29 {
				return result, errors.New("truncated FlowSpec relay OPEN")
			}
			if !bytes.Equal(frame[24:28], id.AsSlice()) {
				return result, errors.New("FlowSpec relay OPEN has the wrong router ID")
			}
			// RFC 8654 Section 3: this existing relay changes only capability6.
			written, _, err := rewriteExtendedOpen(buffer[:], frame, true)
			if err != nil {
				return result, err
			}
			delivered, err := hex.DecodeString(row.Delivered)
			if err != nil {
				return result, err
			}
			if !bytes.Equal(delivered, buffer[:written]) {
				return result, errors.New("FlowSpec relay changed more than OPEN capability6")
			}
			continue
		}
		if opens != 1 {
			return result, errors.New("FlowSpec capture starts outside an OPEN session")
		}
		if row.Delivered != "" {
			return result, errors.New("FlowSpec relay rewrote a non-OPEN frame")
		}
		switch frame[18] {
		case bgpKeepalive:
			if len(frame) != bgpHeaderLength {
				return result, errors.New("invalid captured KEEPALIVE length")
			}
			keepalives++
		case bgpUpdate:
			// RFC 8955 Sections 4 and 7.1: no NH, exact NLRI and action.
			matched, present, err := flowSpecWireUpdate(frame[bgpHeaderLength:])
			if err != nil {
				return result, err
			}
			if matched {
				result.frame, result.present = index, present
			}
		default:
			// The wire type is open; NOTIFICATION or an unknown frame cannot
			// certify a continuous usable session for this fixture.
			return result, fmt.Errorf("unexpected FlowSpec relay BGP type %d", frame[18])
		}
	}
	if opens != 1 {
		return result, errors.New("FlowSpec capture has no OPEN")
	}
	if keepalives == 0 {
		return result, errors.New("FlowSpec capture has no KEEPALIVE")
	}
	return result, nil
}

// flowSpecWireUpdate never searches attributes for an NLRI byte substring.
// RFC 8955 Section 4: "The NLRI field of the MP_REACH_NLRI and MP_UNREACH_NLRI
// is encoded as one or more 2-tuples of the form <length, NLRI value>".
func flowSpecWireUpdate(body []byte) (bool, bool, error) {
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		return false, false, err
	}
	attrs := sections.Attrs(body)
	iterator := attribute.NewAttrIterator(attrs)
	matched, present, action := false, false, false
	var seen [256]bool
	for code, _, value, ok := iterator.Next(); ok; code, _, value, ok = iterator.Next() {
		if seen[code] {
			return false, false, errors.New("duplicate captured path attribute")
		}
		seen[code] = true
		if code == attribute.AttrExtCommunity {
			action = bytes.Equal(value, []byte{0x80, 6, 0, 0, 0x46, 0x16, 0, 0})
			continue
		}
		if code != attribute.AttrMPReachNLRI {
			if code != attribute.AttrMPUnreachNLRI {
				continue
			}
		}
		if len(value) < 3 {
			return false, false, errors.New("truncated captured MP attribute")
		}
		if !bytes.Equal(value[:3], []byte{0, 1, 133}) {
			continue
		}
		nlri := value[3:]
		if code == attribute.AttrMPReachNLRI {
			if len(value) < 5 {
				return false, false, errors.New("truncated FlowSpec MP_REACH")
			}
			if value[3] != 0 {
				return false, false, fmt.Errorf("outbound FlowSpec NH-length=%d, want exactly 0", value[3])
			}
			nlri = value[5:]
		}
		for len(nlri) > 0 {
			length, header := int(nlri[0]), 1
			if length >= 240 {
				if len(nlri) < 2 {
					return false, false, errors.New("truncated FlowSpec NLRI length")
				}
				length, header = (length&15)*256+int(nlri[1]), 2
			}
			if length == 0 {
				return false, false, errors.New("empty FlowSpec rule")
			}
			if header+length > len(nlri) {
				return false, false, errors.New("truncated FlowSpec rule")
			}
			if bytes.Equal(nlri[header:header+length], []byte{1, 24, 10, 99, 77}) {
				if matched {
					return false, false, errors.New("ambiguous target rule in one UPDATE")
				}
				matched, present = true, code == attribute.AttrMPReachNLRI
			}
			nlri = nlri[header+length:]
		}
	}
	if iterator.Remaining() != 0 {
		return false, false, errors.New("truncated captured path attributes")
	}
	if matched && present && !action {
		return false, false, errors.New("target FlowSpec lost or changed its rate9600 action")
	}
	return matched, present, nil
}

// These raw DTOs follow GoBGP v3.31.0 apiutil.Destination/Path and the packet
// package's MarshalJSON methods. Required zero-valued community fields use
// pointers so a missing field cannot masquerade as an encoded AS0 or rate0.
type flowSpecForeignPath struct {
	NLRI struct {
		Value []struct {
			Type  int             `json:"type"`
			Value json.RawMessage `json:"value"`
		} `json:"value"`
	} `json:"nlri"`
	Best       bool   `json:"best"`
	Stale      bool   `json:"stale"`
	Withdrawal bool   `json:"withdrawal"`
	Neighbor   string `json:"neighbor-ip"`
	Attributes []struct {
		Type  int             `json:"type"`
		Value json.RawMessage `json:"value"`
	} `json:"attrs"`
}

// flowSpecForeignEvidence binds the action to the selected received rule, not
// to a rate string elsewhere in the RIB. The preserved originated discard rule
// supplies positive evidence for every absence query.
// RFC 8955 Section 7: "All Traffic Filtering Actions are specified as transitive
// BGP Extended Communities".
func flowSpecForeignEvidence(output, neighbor string, present bool) error {
	var routes map[string][]flowSpecForeignPath
	if err := json.Unmarshal([]byte(output), &routes); err != nil {
		return err
	}
	const baseline = "[destination: 10.99.0.0/24]"
	const target = "[destination: 10.99.77.0/24]"
	if len(routes[baseline]) != 1 {
		return errors.New("GoBGP omitted the positive baseline rule")
	}
	// RFC 8955 Section 7.1: discard is an explicitly encoded zero rate.
	if err := flowSpecForeignRule(&routes[baseline][0], "10.99.0.0/24", neighbor, 0); err != nil {
		return fmt.Errorf("GoBGP baseline: %w", err)
	}
	// A target under the wrong map key is corrupt evidence, not absence.
	for key, paths := range routes {
		for index := range paths {
			for _, component := range paths[index].NLRI.Value {
				if component.Type != 1 {
					continue
				}
				var value struct {
					Prefix string `json:"prefix"`
				}
				if err := json.Unmarshal(component.Value, &value); err != nil {
					return err
				}
				if value.Prefix == flowSpecReceivedPrefix && key != target {
					return errors.New("GoBGP target NLRI is filed under a different rule")
				}
			}
		}
	}
	paths, found := routes[target]
	if !present {
		if found {
			return errors.New("GoBGP still lists the unauthorized target rule")
		}
		return nil
	}
	if len(paths) != 1 {
		return errors.New("GoBGP does not hold exactly one target path")
	}
	// RFC 8955 Section 7.1: the same path must retain the 9600-byte rate.
	return flowSpecForeignRule(&paths[0], flowSpecReceivedPrefix, neighbor, 9600)
}

// RFC 8955 Section 7.1: "The remaining 4 octets carry the maximum rate
// information in IEEE floating point [IEEE.754.1985] format, units being bytes
// per second." GoBGP must decode that field, not merely retain opaque bytes.
func flowSpecForeignRule(path *flowSpecForeignPath, prefix, neighbor string, rate float64) error {
	if !path.Best {
		return errors.New("foreign FlowSpec path is not selected")
	}
	if path.Stale {
		return errors.New("foreign FlowSpec path is stale")
	}
	if path.Withdrawal {
		return errors.New("foreign FlowSpec path is withdrawn")
	}
	if path.Neighbor != neighbor {
		return errors.New("foreign FlowSpec path came from a different neighbor")
	}
	if len(path.NLRI.Value) != 1 {
		return errors.New("foreign FlowSpec rule has the wrong component count")
	}
	if path.NLRI.Value[0].Type != 1 {
		return errors.New("foreign FlowSpec rule is not a destination prefix")
	}
	var destination struct {
		Prefix string `json:"prefix"`
	}
	if err := json.Unmarshal(path.NLRI.Value[0].Value, &destination); err != nil {
		return err
	}
	if destination.Prefix != prefix {
		return errors.New("foreign FlowSpec destination differs from its RIB key")
	}
	communities := 0
	for _, attr := range path.Attributes {
		if attr.Type != 16 {
			continue
		}
		communities++
		var values []struct {
			Type    int      `json:"type"`
			Subtype int      `json:"subtype"`
			AS      *uint16  `json:"as"`
			Rate    *float64 `json:"rate"`
		}
		if err := json.Unmarshal(attr.Value, &values); err != nil {
			return err
		}
		if len(values) != 1 {
			return errors.New("foreign FlowSpec action count differs")
		}
		value := values[0]
		if value.Type != 128 {
			return errors.New("foreign FlowSpec action is not transitive experimental")
		}
		if value.Subtype != 6 {
			return errors.New("foreign FlowSpec action is not traffic-rate-bytes")
		}
		if value.AS == nil {
			return errors.New("foreign FlowSpec action omitted its AS field")
		}
		if *value.AS != 0 {
			return errors.New("foreign FlowSpec action changed its AS field")
		}
		if value.Rate == nil {
			return errors.New("foreign FlowSpec action omitted its rate field")
		}
		if *value.Rate != rate {
			return errors.New("foreign FlowSpec traffic rate changed")
		}
	}
	if communities != 1 {
		return errors.New("foreign FlowSpec path has no unique extended-community attribute")
	}
	return nil
}

// RFC 8955 Section 6: "The originator of the Flow Specification matches the
// originator of the best-match unicast route for the destination prefix embedded
// in the Flow Specification". The source's cover must reach GoBGP before its
// scripted marker releases the rule; a prefix in an error string is not proof.
func flowSpecForeignCover(output, neighbor string) error {
	var routes map[string][]struct {
		NLRI struct {
			Prefix string `json:"prefix"`
		} `json:"nlri"`
		Best     bool   `json:"best"`
		Stale    bool   `json:"stale"`
		Neighbor string `json:"neighbor-ip"`
	}
	if err := json.Unmarshal([]byte(output), &routes); err != nil {
		return err
	}
	paths := routes[flowSpecReceivedPrefix]
	if len(paths) != 1 {
		return errors.New("GoBGP has no unique covering path")
	}
	if paths[0].NLRI.Prefix != flowSpecReceivedPrefix {
		return errors.New("GoBGP covering route has the wrong NLRI")
	}
	if !paths[0].Best {
		return errors.New("GoBGP covering route is not selected")
	}
	if paths[0].Stale {
		return errors.New("GoBGP covering route is stale")
	}
	if paths[0].Neighbor != neighbor {
		return errors.New("GoBGP covering route came from a different neighbor")
	}
	return nil
}

// flowSpecSourceComplete reads runMessageLoop's send log and completed's
// post-write sentinel. The four frame sizes distinguish cover, nonzero-NH rule,
// cover withdrawal and cover restoration in inject.msg. An extra rule send or
// a truncated log cannot prove replay without source readvertisement.
func flowSpecSourceComplete(logs string) error {
	want := [...]string{
		"sending 47 bytes to peer", "sending 77 bytes to peer",
		"sending 27 bytes to peer", "sending 47 bytes to peer",
	}
	sends := 0
	complete := false
	for line := range strings.SplitSeq(logs, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "sending ") {
			if sends >= len(want) {
				return errors.New("injector sent extra data after its single rule")
			}
			if line != want[sends] {
				return errors.New("injector did not send the cover/rule/withdraw/restore sequence")
			}
			sends++
		}
		if line == "successful" {
			if sends != len(want) {
				return errors.New("injector completed before every prescribed write")
			}
			complete = true
		}
	}
	if !complete {
		return errors.New("injector has not completed the cover-only replay dialog")
	}
	return nil
}
