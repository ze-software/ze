// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RRO collection + ERO/RRO display (AC-9)
// RFC: rfc/short/rfc3209.md
// Related: wire.go -- RROEntry/EROHop and their Encode/Decode primitives
// Related: engine.go -- RESV handlers record the route; register.go shows it
//
// RFC 3209 Section 4.4: as a RESV travels upstream each node prepends its own
// address to the Record Route Object, so the head-end's RSB carries the full
// path the LSP actually took. `show rsvp-te session` reports the configured ERO
// (from the PSB) and this recorded RRO (from the RSB).
package rsvpte

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// prependRRO returns a new RRO with this node's IPv4 address recorded at the
// head, ahead of the route recorded by downstream nodes (RFC 3209 Section 4.4).
// An invalid self address is not recorded. The second result reports whether the
// recorded route was truncated at maxRecordRouteHops -- callers MUST surface that
// (a route longer than the limit means a pathological path or a routing loop, and
// must not be dropped silently).
func prependRRO(self netip.Addr, downstream []rroEntry) (out []rroEntry, truncated bool) {
	out = make([]rroEntry, 0, len(downstream)+1)
	if self.IsValid() {
		out = append(out, rroEntry{Type: RROSubIPv4, Address: self})
	}
	out = append(out, downstream...)
	// Bound the recorded route so a long path (or a routing loop) cannot grow it
	// past what the fixed message buffer can encode. Keep the head (this node and
	// the nearest hops); the far tail is the least useful for the head-end view.
	if len(out) > maxRecordRouteHops {
		out = out[:maxRecordRouteHops]
		truncated = true
	}
	return out, truncated
}

// formatERO renders ERO hops as "prefix strict|loose" strings for display.
func formatERO(hops []eroHop) []string {
	if len(hops) == 0 {
		return nil
	}
	out := make([]string, 0, len(hops))
	var tb textbuf.Buffer
	for _, h := range hops {
		kind := "strict"
		if h.Loose {
			kind = "loose"
		}
		tb.Reset()
		out = append(out, tb.Prefix(h.Address).Byte(' ').Str(kind).String())
	}
	return out
}

// formatRRO renders recorded-route entries as strings for display: node hops as
// their address, label subobjects as "label N".
func formatRRO(entries []rroEntry) []string {
	if len(entries) == 0 {
		return nil
	}
	out := make([]string, 0, len(entries))
	var tb textbuf.Buffer
	for _, e := range entries {
		if e.Type == RROSubLabel {
			tb.Reset()
			out = append(out, tb.Str("label ").Uint(uint64(e.Label)).String())
			continue
		}
		out = append(out, e.Address.String())
	}
	return out
}
