// Design: docs/architecture/route-selection.md -- resolution after a proposed install
// Related: igpcost.go -- one bounded recursive resolution algorithm
// RFC: rfc/short/rfc4271.md -- Section 9.1.2.1
package igpcost

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// ResolveProspective resolves addr through the selected routing table after a
// proposed contribution, without inserting it. Nil supplies no reachability.
func ResolveProspective(rib *locrib.RIB, addr netip.Addr, fam family.Family, prefix netip.Prefix, offered locrib.Path) Distance {
	if rib == nil {
		return Distance{}
	}
	if !prefix.IsValid() {
		return Distance{}
	}
	selected, found := rib.PreviewSelected(fam, prefix, offered)
	view := prospectiveReader{rib: rib, fam: fam, prefix: prefix, selected: selected, found: found}
	// RFC 4271 Section 9.1.2: "If the NEXT_HOP attribute of a BGP route depicts
	// an address that is not resolvable, or if it would become unresolvable if
	// the route was installed in the routing table, the BGP route MUST be
	// excluded from the Phase 2 decision function."
	return resolve(nil, &view, addr)
}

// prospectiveReader overlays one elected contribution. A more-specific route
// still wins, and a same-prefix connected/static winner is retained by PreviewSelected.
// All fields are immutable after construction; rib supplies concurrent reads.
type prospectiveReader struct {
	rib      *locrib.RIB
	fam      family.Family
	prefix   netip.Prefix
	selected locrib.Path
	found    bool
}

func (view *prospectiveReader) LPM(fam family.Family, addr netip.Addr) (locrib.Path, netip.Prefix, bool) {
	path, prefix, found := view.rib.LPM(fam, addr)
	if fam != view.fam {
		return path, prefix, found
	}
	if !view.prefix.Contains(addr) {
		return path, prefix, found
	}
	if found {
		if prefix.Bits() > view.prefix.Bits() {
			return path, prefix, true
		}
	}
	if view.found {
		return view.selected, view.prefix, true
	}
	// A rejected replacement can leave no selected contribution at this exact
	// prefix. Find the remaining ancestor, bounded by the address's 32/128 bits.
	for bits := view.prefix.Bits() - 1; bits >= 0; bits-- {
		ancestor := netip.PrefixFrom(addr, bits).Masked()
		if parent, exists := view.rib.Best(fam, ancestor); exists {
			return parent, ancestor, true
		}
	}
	return locrib.Path{}, netip.Prefix{}, false
}
