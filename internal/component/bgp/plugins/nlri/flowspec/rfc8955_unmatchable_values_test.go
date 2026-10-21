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
// beside TCP flags, and a fragment match admitting only non-first fragments
// beside a port are refused with flowspec.ErrFlowSpecUnmatchable; the same
// components beside a protocol or fragment they can match are accepted.
// PREVENTS: Ze propagating a route that matches no packet, which RFC 8955
// Section 4.2 says SHOULD NOT be propagated by BGP.
//
// RFC requirement: RFC8955-4.2-3 negative -- protocol =17 beside tcp-flags, protocol =47 beside destination-port, protocol =6 beside icmp-type, icmp-type beside tcp-flags, and fragment is-fragment beside destination-port are refused with ErrFlowSpecUnmatchable by both the config route parser and the update text encoder (§4.2).
// RFC requirement: RFC8955-4.2-3 positive -- protocol =17 beside destination-port, protocol =6 beside tcp-flags, protocol =1 beside icmp-type, and fragment first-fragment beside destination-port are accepted by both (§4.2).
func TestFlowSpecUnmatchableByValueRefused(t *testing.T) {
	base := []string{flowspec.KwDestinationIPv4, "192.0.2.0/24"}
	for _, tc := range []struct {
		args        []string
		unmatchable bool
	}{
		{[]string{flowspec.KwProtocol, "=17", flowspec.KwTCPFlags, "syn"}, true},
		{[]string{flowspec.KwProtocol, "=47", flowspec.KwDestPort, "=80"}, true},
		{[]string{flowspec.KwProtocol, "=6", flowspec.KwICMPType, "=8"}, true},
		{[]string{flowspec.KwICMPType, "=8", flowspec.KwTCPFlags, "syn"}, true},
		{[]string{flowspec.KwFragment, "is-fragment", flowspec.KwDestPort, "=80"}, true},
		{[]string{flowspec.KwProtocol, "=17", flowspec.KwDestPort, "=53"}, false},
		{[]string{flowspec.KwProtocol, "=6", flowspec.KwTCPFlags, "syn"}, false},
		{[]string{flowspec.KwProtocol, "=1", flowspec.KwICMPType, "=8"}, false},
		{[]string{flowspec.KwFragment, "first-fragment", flowspec.KwDestPort, "=80"}, false},
	} {
		args := append(append([]string{}, base...), tc.args...)
		_, err := parseConfigRoute(registry.ConfigRouteRequest{Content: args})
		if errors.Is(err, flowspec.ErrFlowSpecUnmatchable) != tc.unmatchable {
			t.Errorf("config route %v: err = %v, want unmatchable %v", args, err, tc.unmatchable)
		}
		if !tc.unmatchable && err != nil {
			t.Errorf("config route %v refused: %v", args, err)
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
