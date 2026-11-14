// Design: docs/guide/route-policy.md -- show rib candidates, the hidden paths and the chain that hid each
// Related: register.go -- dispatches the command and declares it
// Related: route_view_worker_operations.go -- routeViewClient.Candidates, which reads the selector with candidatesPrefix
// Related: route_view_candidates.go -- readCandidates, the daemon scan that renders each prefix with candidateEntryOf
// Related: sysrib.go -- showRIB, the selected routes this command complements

package sysrib

import (
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// candidatesSelectorPrefix is the keyword before the prefix value in
// `show rib candidates prefix <prefix>` (ai/rules/cli.md: a typed selector
// keyword before any free-form value).
const candidatesSelectorPrefix = "prefix"

// candidatePath is one path a protocol offered for a prefix: selected,
// standing by, or hidden by an import chain.
type candidatePath struct {
	Source    string     `json:"source"`
	Instance  uint32     `json:"instance"`
	NextHop   netip.Addr `json:"next-hop,omitzero"`
	Interface string     `json:"interface,omitempty"`
	Distance  uint8      `json:"distance"`
	Metric    uint32     `json:"metric"`
	// Tag is present only when the producer states one (HasTag): zero is a
	// real tag, so the absent case is a nil pointer, not 0.
	Tag       *uint32 `json:"tag,omitempty"`
	Level     uint8   `json:"level,omitempty"`
	RouteType string  `json:"route-type,omitempty"`
	Selected  bool    `json:"selected"`
	Hidden    bool    `json:"hidden"`
	// HiddenBy is "source" or "central"; Chain is the attachment whose chain
	// hid the path ("isis import", "rib import"). Both are omitted for a path
	// no chain hid.
	HiddenBy string `json:"hidden-by,omitempty"`
	Chain    string `json:"chain,omitempty"`
}

// candidateEntry is one prefix and every path offered for it.
type candidateEntry struct {
	Prefix netip.Prefix    `json:"prefix"`
	Family family.Family   `json:"family"`
	Paths  []candidatePath `json:"paths"`
}

// candidatesPrefix reads the optional `prefix <prefix>` selector of
// `show rib candidates [prefix <prefix>]`, before routeViewClient.Candidates
// sends anything to the daemon. No argument lists every prefix; anything else
// is refused by name.
func candidatesPrefix(args []string) (netip.Prefix, bool, error) {
	if len(args) == 0 {
		return netip.Prefix{}, false, nil
	}
	if args[0] != candidatesSelectorPrefix {
		var tb textbuf.Buffer
		tb.Str("show rib candidates: unknown argument ").Str(args[0]).
			Str("; the only selector is `prefix <prefix>`, as in `show rib candidates prefix 10.0.0.0/24`")
		return netip.Prefix{}, false, errors.New(tb.String())
	}
	if len(args) != 2 {
		return netip.Prefix{}, false, errors.New("show rib candidates: `prefix` takes exactly one prefix, as in `show rib candidates prefix 10.0.0.0/24`")
	}
	p, err := netip.ParsePrefix(args[1])
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("show rib candidates: ").Str(args[1]).
			Str(" is not a prefix; write it as address/length, as in `show rib candidates prefix 10.0.0.0/24`")
		return netip.Prefix{}, false, errors.New(tb.String())
	}
	return p.Masked(), true, nil
}

// candidateEntryOf renders one PathGroup: every Loc-RIB path for the prefix,
// hidden ones included. A hidden path is stored in the Loc-RIB but never
// selected, so `show rib` cannot show it. The chain is read from the path
// (HiddenBy, written by the gate when it hid the path), so the answer is the
// verdict the RIB acted on, not a fresh evaluation that a reload in flight
// could change. readCandidates calls it for each prefix it scans.
func candidateEntryOf(fam family.Family, prefix netip.Prefix, g *locrib.PathGroup) candidateEntry {
	e := candidateEntry{Prefix: prefix, Family: fam, Paths: make([]candidatePath, 0, len(g.Paths))}
	for i := range g.Paths {
		e.Paths = append(e.Paths, candidatePathOf(&g.Paths[i], i == g.Best))
	}
	return e
}

// candidatePathOf renders one path.
func candidatePathOf(p *locrib.Path, selected bool) candidatePath {
	c := candidatePath{
		Source:    redistevents.ProtocolName(p.Source),
		Instance:  p.Instance,
		NextHop:   p.NextHop,
		Interface: p.Interface,
		Distance:  p.AdminDistance,
		Metric:    p.Metric,
		Level:     uint8(p.Level),
		Selected:  selected,
		Hidden:    p.Hidden,
	}
	if p.HasTag {
		tag := p.Tag
		c.Tag = &tag
	}
	if name, ok := policy.RouteTypeName(p.OSPFRouteType); ok {
		c.RouteType = name
	}
	if p.HiddenBy != ribpolicy.ChainRoleUnspecified {
		c.HiddenBy = p.HiddenBy.String()
		if chain, ok := policy.ImportAttachment(p.Source, p.HiddenBy); ok {
			c.Chain = chain
		}
	}
	return c
}
