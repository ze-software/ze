// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec NLRI plugin
// RFC: rfc/short/rfc8955.md — component combinations (Section 4.2)

package flowspec

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/nlri/flowspec"
)

// TestFlowSpecUnmatchableByValueRefused proves the origination paths refuse the
// combinations whose contradiction is in the values, or in a pair other than
// ICMP and a port: the config route parser and the update text encoder.
//
// VALIDATES: a protocol value no transport component beside it can match, ICMP
// beside TCP flags, a fragment match admitting only non-first fragments beside
// a port, a port no destination-port or source-port beside it admits, and a
// component list whose own terms admit no value are refused with flowspec.ErrFlowSpecUnmatchable; the same
// components beside a protocol or fragment they can match are accepted.
// PREVENTS: Ze propagating a route that matches no packet, which RFC 8955
// Section 4.2 says SHOULD NOT be propagated by BGP.
//
// RFC requirement: RFC8955-4.2-3 negative -- protocol =17 beside tcp-flags, protocol =47 beside destination-port, protocol =6 beside icmp-type, icmp-type beside tcp-flags, fragment is-fragment beside destination-port, port =80 beside destination-port =443 and source-port =443, destination-port >100 AND <50, and tcp-flags SYN AND NOT SYN are refused with ErrFlowSpecUnmatchable by both the config route parser and the update text encoder (§4.2).
// RFC requirement: RFC8955-4.2-3 positive -- protocol =17 beside destination-port, protocol =6 beside tcp-flags, protocol =1 beside icmp-type, fragment first-fragment beside destination-port, port =80 beside destination-port =80 and source-port =443, destination-port >100 AND <500, and tcp-flags SYN AND NOT ACK are accepted by both (§4.2).
func TestFlowSpecUnmatchableByValueRefused(t *testing.T) {
	base := []string{flowspec.KwDestinationIPv4, "192.0.2.0/24"}
	// text holds the update text spelling where it differs from the config
	// spelling in args: the text grammar joins bitmask terms with a leading "&"
	// as their own token. Numeric terms share the config spelling.
	for _, tc := range []struct {
		args        []string
		unmatchable bool
		text        []string
	}{
		{[]string{flowspec.KwProtocol, "=17", flowspec.KwTCPFlags, "syn"}, true, nil},
		{[]string{flowspec.KwProtocol, "=47", flowspec.KwDestPort, "=80"}, true, nil},
		{[]string{flowspec.KwProtocol, "=6", flowspec.KwICMPType, "=8"}, true, nil},
		{[]string{flowspec.KwICMPType, "=8", flowspec.KwTCPFlags, "syn"}, true, nil},
		{[]string{flowspec.KwFragment, "is-fragment", flowspec.KwDestPort, "=80"}, true, nil},
		{[]string{flowspec.KwProtocol, "=17", flowspec.KwDestPort, "=53"}, false, nil},
		{[]string{flowspec.KwProtocol, "=6", flowspec.KwTCPFlags, "syn"}, false, nil},
		{[]string{flowspec.KwProtocol, "=1", flowspec.KwICMPType, "=8"}, false, nil},
		{[]string{flowspec.KwFragment, "first-fragment", flowspec.KwDestPort, "=80"}, false, nil},
		{[]string{flowspec.KwPort, "=80", flowspec.KwDestPort, "=443", flowspec.KwSourcePort, "=443"}, true, nil},
		{[]string{flowspec.KwPort, "=80", flowspec.KwDestPort, "=80", flowspec.KwSourcePort, "=443"}, false, nil},
		{[]string{flowspec.KwDestPort, ">100&<50"}, true, nil},
		{[]string{flowspec.KwDestPort, ">100&<500"}, false, nil},
		{[]string{flowspec.KwTCPFlags, "syn&!=syn"}, true, []string{flowspec.KwTCPFlags, "=syn", "&!=syn"}},
		{[]string{flowspec.KwTCPFlags, "syn&!=ack"}, false, []string{flowspec.KwTCPFlags, "=syn", "&!=ack"}},
	} {
		args := append(append([]string{}, base...), tc.args...)
		_, err := parseConfigRoute(registry.ConfigRouteRequest{Content: args})
		if errors.Is(err, flowspec.ErrFlowSpecUnmatchable) != tc.unmatchable {
			t.Errorf("config route %v: err = %v, want unmatchable %v", args, err, tc.unmatchable)
		}
		if !tc.unmatchable && err != nil {
			t.Errorf("config route %v refused: %v", args, err)
		}
		if tc.text != nil {
			args = append(append([]string{}, base...), tc.text...)
		}
		_, err = EncodeFlowSpecComponents(IPv4FlowSpec, args)
		if errors.Is(err, flowspec.ErrFlowSpecUnmatchable) != tc.unmatchable {
			t.Errorf("text encode %v: err = %v, want unmatchable %v", args, err, tc.unmatchable)
		}
		if !tc.unmatchable && err != nil {
			t.Errorf("text encode %v refused: %v", args, err)
		}
	}
}
