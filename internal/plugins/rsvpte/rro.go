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
// An invalid self address is not recorded. The second result reports that the
// route grew past maxRecordRouteHops, the most one message buffer encodes, and
// was dropped whole. RFC 3209 Section 4.4.3: "If the newly added subobject
// causes the RRO to be too big to fit in a Path (or Resv) message, the RRO
// object SHALL be dropped from the message and message processing continues as
// normal." Callers MUST surface the drop. A recorded label counts toward the
// same bound and is added only with this node's address.
func prependRRO(self netip.Addr, downstream []rroEntry, recordLabel uint32) (out []rroEntry, dropped bool) {
	entries := len(downstream)
	if self.IsValid() {
		entries++
		if recordLabel != 0 {
			entries++
		}
	}
	if entries > maxRecordRouteHops {
		return nil, true
	}
	out = make([]rroEntry, 0, entries)
	if self.IsValid() {
		out = append(out, rroEntry{Type: RROSubIPv4, Address: self})
		if recordLabel != 0 {
			out = append(out, rroEntry{Type: RROSubLabel, Label: recordLabel})
		}
	}
	out = append(out, downstream...)
	return out, false
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
