// Design: docs/architecture/route-selection.md -- forked next-hop metric delivery
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func (s *Server) opRouteMetrics(ctx context.Context, _ *process.Process, params json.RawMessage) (any, error) {
	var input rpc.RouteMetricsInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, err
	}
	if len(input.Addresses) > rpc.RouteMetricsAddressMax {
		return nil, fmt.Errorf("route-metrics: address count exceeds %d", rpc.RouteMetricsAddressMax)
	}
	loc := locrib.Default()
	if loc == nil {
		return nil, fmt.Errorf("route-metrics: engine has no Loc-RIB")
	}
	// Read before resolving: a change during this batch invalidates it on the
	// next request, including a change to a recursive hop not in Addresses.
	out := &rpc.RouteMetricsOutput{Revision: loc.Revision(), Distances: make([]rpc.RouteMetric, len(input.Addresses))}
	for i, text := range input.Addresses {
		addr, err := netip.ParseAddr(text)
		if err != nil {
			return nil, fmt.Errorf("route-metrics: invalid next hop %q: %w", text, err)
		}
		d := igpcost.Resolve(loc, addr)
		out.Distances[i] = rpc.RouteMetric{Cost: d.Cost, Resolved: d.Resolved, MissingAIGP: d.MissingAIGP}
	}
	return out, nil
}
