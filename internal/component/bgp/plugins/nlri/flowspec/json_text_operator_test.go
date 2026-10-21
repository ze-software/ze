// Design: docs/architecture/api/architecture.md — FlowSpec NLRI update text grammar
// RFC: rfc/short/rfc8955.md — numeric operator (Section 4.2.1.1)

package flowspec

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/nlri/flowspec"
)

// TestJSONToTextKeepsNumericOperators proves a FlowSpec rule read as JSON
// reaches the wire with the operators it was written with, for every numeric
// component, protocol and next-header included.
//
// VALIDATES: jsonToTextComponents and normalizeJSONValue keep each value's
// operator, and the update text parser encodes it: protocol `>5&<10` encodes
// gt then AND lt, `!=6` encodes lt|gt, and a named protocol (`!=udp`) encodes
// its number under its own operator. The method is JSON -> text -> wire, then
// a byte comparison of the component octets.
// PREVENTS: `[[">5","<10"]]` coming back as `protocol 5 10`, an equality OR
// that matches a different set of packets.
func TestJSONToTextKeepsNumericOperators(t *testing.T) {
	t.Parallel()
	// Each operator octet is e(0x80) a(0x40) len(0x30) 0 lt(0x04) gt(0x02)
	// eq(0x01), RFC 8955 Section 4.2.1.1. The values fit one octet, so len is 0.
	// [[">5","<10"],["!=20"]] is 0x02 >5, 0x44 AND <10, 0x86 end !=20.
	const rangeOrNot = `[[">5","<10"],["!=20"]]`
	const rangeOrNotWire = "02" + "05" + "44" + "0a" + "86" + "14"
	for _, tc := range []struct {
		name string
		fam  flowspec.Family
		json string
		want string
	}{
		{"protocol range", IPv4FlowSpec, `{"protocol":[[">5","<10"]]}`, "03" + "02" + "05" + "c4" + "0a"},
		{"protocol not equal", IPv4FlowSpec, `{"protocol":[["!=6"]]}`, "03" + "86" + "06"},
		{"protocol named", IPv4FlowSpec, `{"protocol":[["=tcp"],["!=udp"]]}`, "03" + "01" + "06" + "86" + "11"},
		{"protocol named range", IPv4FlowSpec, `{"protocol":[[">=tcp","<=udp"]]}`, "03" + "03" + "06" + "c5" + "11"},
		{"protocol", IPv4FlowSpec, `{"protocol":` + rangeOrNot + `}`, "03" + rangeOrNotWire},
		{"next-header", IPv6FlowSpec, `{"next-header":` + rangeOrNot + `}`, "03" + rangeOrNotWire},
		{"port", IPv4FlowSpec, `{"port":` + rangeOrNot + `}`, "04" + rangeOrNotWire},
		{"destination-port", IPv4FlowSpec, `{"destination-port":` + rangeOrNot + `}`, "05" + rangeOrNotWire},
		{"source-port", IPv4FlowSpec, `{"source-port":` + rangeOrNot + `}`, "06" + rangeOrNotWire},
		{"icmp-type", IPv4FlowSpec, `{"icmp-type":` + rangeOrNot + `}`, "07" + rangeOrNotWire},
		{"icmp-code", IPv4FlowSpec, `{"icmp-code":` + rangeOrNot + `}`, "08" + rangeOrNotWire},
		{"packet-length", IPv4FlowSpec, `{"packet-length":` + rangeOrNot + `}`, "0a" + rangeOrNotWire},
		{"dscp", IPv4FlowSpec, `{"dscp":` + rangeOrNot + `}`, "0b" + rangeOrNotWire},
		// The encoder writes a flow label as four octets (len 0x20), so the
		// operators read 0x22, 0x64 and 0xa6.
		{"flow-label", IPv6FlowSpec, `{"flow-label":` + rangeOrNot + `}`, "0d" + "22" + "00000005" + "64" + "0000000a" + "a6" + "00000014"},
	} {
		got, err := encodeJSONFlowSpecForTest(tc.fam, tc.json)
		if err != nil {
			t.Errorf("%s: %s refused: %v", tc.name, tc.json, err)
			continue
		}
		if hexed := hex.EncodeToString(got); hexed != tc.want {
			t.Errorf("%s: %s encoded %s, want %s", tc.name, tc.json, hexed, tc.want)
		}
	}
}

// encodeJSONFlowSpecForTest runs one JSON rule through the JSON-to-text
// conversion and the update text encoder, and returns the component octets
// after the NLRI length octet.
func encodeJSONFlowSpecForTest(fam flowspec.Family, jsonRule string) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(jsonRule), &m); err != nil {
		return nil, err
	}
	args, err := jsonToTextComponents(m)
	if err != nil {
		return nil, err
	}
	wire, err := EncodeFlowSpecComponents(fam, args)
	if err != nil {
		return nil, err
	}
	if len(wire) < 1 {
		return nil, errFlowspecRequiresAtLeastOneComponent
	}
	return wire[1:], nil
}
