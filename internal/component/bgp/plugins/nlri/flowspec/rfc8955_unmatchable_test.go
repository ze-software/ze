// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec NLRI plugin
// RFC: rfc/short/rfc8955.md — component combinations (Section 4.2)

package flowspec

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/nlri/flowspec"
)

// TestFlowSpecUnmatchableRefused proves a flow route that matches an ICMP type
// or code together with a port is refused on both origination paths: the config
// route parser and the text encoder that `update text` reaches.
//
// VALIDATES: each ICMP component beside each port component is refused with
// flowspec.ErrFlowSpecUnmatchable, and ICMP or a port alone is still accepted.
// PREVENTS: Ze propagating a route that matches no packet, which RFC 8955
// Section 4.2 says SHOULD NOT be propagated by BGP.
func TestFlowSpecUnmatchableRefused(t *testing.T) {
	icmp := [][]string{
		{flowspec.KwICMPType, "=8"},
		{flowspec.KwICMPCode, "=0"},
	}
	ports := [][]string{
		{flowspec.KwPort, "=80"},
		{flowspec.KwDestPort, "=80"},
		{flowspec.KwSourcePort, "=80"},
	}
	base := []string{flowspec.KwDestinationIPv4, "192.0.2.0/24"}

	for _, i := range icmp {
		for _, p := range ports {
			args := append(append(append([]string{}, base...), i...), p...)
			t.Run(i[0]+"+"+p[0], func(t *testing.T) {
				if _, err := parseConfigRoute(registry.ConfigRouteRequest{Content: args}); !errors.Is(err, flowspec.ErrFlowSpecUnmatchable) {
					t.Errorf("config route %v: err = %v, want ErrFlowSpecUnmatchable", args, err)
				}
				if _, err := EncodeFlowSpecComponents(IPv4FlowSpec, args); !errors.Is(err, flowspec.ErrFlowSpecUnmatchable) {
					t.Errorf("text encode %v: err = %v, want ErrFlowSpecUnmatchable", args, err)
				}
			})
		}
	}

	for _, alone := range append(icmp, ports...) {
		args := append(append([]string{}, base...), alone...)
		if _, err := parseConfigRoute(registry.ConfigRouteRequest{Content: args}); err != nil {
			t.Errorf("config route %v refused: %v", args, err)
		}
		if _, err := EncodeFlowSpecComponents(IPv4FlowSpec, args); err != nil {
			t.Errorf("text encode %v refused: %v", args, err)
		}
	}
}
