// Design: plan/learned/1052-ospf-ext-14-debug-introspection.md -- `show ospf [ipv6] neighbor detail`.
// RFC: rfc/short/rfc2328.md (Section 10 NSM state; the O-bit is RFC 5250), rfc/short/rfc5340.md
// (Section A.2 OSPFv3 Options R/V6/E/N; RFC 5838 Section 2.4 AF-bit).
//
// The neighbor deep-dump reads the neighbor table's Detail snapshot (DD seq, list sizes,
// timers, last NSM event) and decodes the Options field per address family: the OSPFv2
// O-bit (opaque-capable) vs the OSPFv3 R/V6/E/N/AF bits. Read-only; additive over the
// summary `... neighbor` view.

package ospf

import (
	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// neighborDetailView is one neighbor's full state plus AF-decoded Options bits.
type neighborDetailView struct {
	ospfneighbor.Detail
	OpaqueCapable bool     `json:"opaque-capable"`
	OptionBits    []string `json:"option-bits"`
}

// neighborDetailSnapshot renders `show ospf neighbor detail` (OSPFv2: O-bit + option bits).
func (e *engine) neighborDetailSnapshot() []any {
	return e.wrapNeighborDetail(false)
}

// v3NeighborDetailSnapshot renders `show ospf ipv6 neighbor detail` (OSPFv3 option bits +
// the advertised Interface ID, already on Detail).
func (e *engine) v3NeighborDetailSnapshot() []any {
	return e.wrapNeighborDetail(true)
}

func (e *engine) wrapNeighborDetail(v6 bool) []any {
	if e.neighbors == nil {
		return []any{}
	}
	details := e.neighbors.DetailSnapshot()
	out := make([]any, 0, len(details))
	for i := range details {
		d := details[i]
		view := neighborDetailView{Detail: d}
		if v6 {
			view.OptionBits = optionsV6Bits(d.Options)
		} else {
			view.OptionBits = optionsV2Bits(d.Options)
			view.OpaqueCapable = types.Options(d.Options).Has(types.OptionO)
		}
		out = append(out, view)
	}
	return out
}

// optionsV2Bits decodes the OSPFv2 Options octet (RFC 2328 Section A.2, plus RFC 5250 O-bit).
func optionsV2Bits(raw uint32) []string {
	o := types.Options(raw)
	var bits []string
	for _, b := range []struct {
		bit  types.Options
		name string
	}{
		{types.OptionE, "E"},
		{types.OptionMC, "MC"},
		{types.OptionNP, "N/P"},
		{types.OptionL, "L"},
		{types.OptionDC, "DC"},
		{types.OptionO, "O"},
		{types.OptionDN, "DN"},
	} {
		if o.Has(b.bit) {
			bits = append(bits, b.name)
		}
	}
	return bits
}

// optionsV6Bits decodes the OSPFv3 Options field (RFC 5340 Section A.2 + RFC 5838 AF-bit).
func optionsV6Bits(raw uint32) []string {
	o := ospfv3types.Options(raw)
	var bits []string
	for _, b := range []struct {
		bit  ospfv3types.Options
		name string
	}{
		{ospfv3types.OptV6, "V6"},
		{ospfv3types.OptE, "E"},
		{ospfv3types.OptN, "N"},
		{ospfv3types.OptR, "R"},
		{ospfv3types.OptAF, "AF"},
	} {
		if o.Has(b.bit) {
			bits = append(bits, b.name)
		}
	}
	return bits
}
