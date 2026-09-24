// Design: docs/architecture/wire/nlri-flowspec.md -- numeric operator precedence
package flowspecfirewall

import (
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec"
	"github.com/ze-software/ze/internal/component/firewall"
)

// numericPortRanges lowers OR-of-AND comparisons to disjoint port intervals.
// Bounds come from the 16-bit packet field, independent of operand wire width.
func numericPortRanges(comp flowspec.FlowComponent) ([]firewall.PortRange, error) {
	list, ok := comp.(interface{ Matches() []flowspec.FlowMatch })
	if !ok || len(list.Matches()) == 0 {
		return nil, fmt.Errorf("%w: %s carries no numeric expression", errUnreadableValue, comp.Type())
	}
	var result, group []firewall.PortRange
	for i, match := range list.Matches() {
		if i == 0 || !match.And {
			result = append(result, group...)
			group = []firewall.PortRange{{Lo: 0, Hi: 65535}}
		}
		var allowed []firewall.PortRange
		if match.Op&flowspec.FlowOpLess != 0 && match.Value > 0 {
			allowed = append(allowed, firewall.PortRange{Lo: 0, Hi: uint16(min(match.Value-1, 65535))})
		}
		if match.Op&flowspec.FlowOpEqual != 0 && match.Value <= 65535 {
			allowed = append(allowed, firewall.PortRange{Lo: uint16(match.Value), Hi: uint16(match.Value)})
		}
		if match.Op&flowspec.FlowOpGreater != 0 && match.Value < 65535 {
			allowed = append(allowed, firewall.PortRange{Lo: uint16(match.Value + 1), Hi: 65535})
		}
		var intersection []firewall.PortRange
		for _, a := range group {
			for _, b := range allowed {
				lo, hi := max(a.Lo, b.Lo), min(a.Hi, b.Hi)
				if lo <= hi {
					intersection = append(intersection, firewall.PortRange{Lo: lo, Hi: hi})
				}
			}
		}
		group = intersection
	}
	result = append(result, group...)
	slices.SortFunc(result, func(a, b firewall.PortRange) int { return int(a.Lo) - int(b.Lo) })
	out := result[:0]
	for _, r := range result {
		if len(out) > 0 && uint32(r.Lo) <= uint32(out[len(out)-1].Hi)+1 {
			out[len(out)-1].Hi = max(out[len(out)-1].Hi, r.Hi)
		} else {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: port expression matches no packet", errUnsupportedComponent)
	}
	return out, nil
}
