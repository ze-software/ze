// Design: docs/guide/route-policy.md -- the doctor check for an IGP import filter
// Related: rib.go -- RIBImport.Floods, which marks the sources this check judges
// Related: register.go -- registers the check

package policy

import (
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// igpImportDoctorCheck warns about the transit blackhole an IGP import filter
// can open (spec R-4): the IGP still floods a prefix its import chain hid, so
// neighbors keep sending that traffic here, while this router resolves a
// selected route's next hop through something shorter, or not at all.
var igpImportDoctorCheck = diagnostic.DoctorCheck{
	Name:         "policy-igp-import-hides-nexthop",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2300,
	Component:    PluginName,
	Dependencies: []string{"loc-rib"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorPolicyIGPImportHidesNextHop},
	Check:        checkIGPImportHidesNextHop,
}

// checkIGPImportHidesNextHop reads the process's Loc-RIB. Inside the daemon
// (`show doctor`) that is the live RIB. In `ze doctor`, an operator's own
// process, no RIB exists, so the check has nothing to judge and says nothing:
// the description of the code states that only `show doctor` reports it.
func checkIGPImportHidesNextHop(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	rib := locrib.Default()
	// refuse-by-default: guards locrib.Default -- a process with no Loc-RIB holds no hidden path to report
	if rib == nil {
		return nil
	}
	return igpImportHidesNextHop(rib, floodingAttachments())
}

// hiddenIGPPath is one prefix a flooding source offered and a chain hid.
type hiddenIGPPath struct {
	family family.Family
	prefix netip.Prefix
	chain  string
}

// floodingAttachments maps every source whose RIBImport floods to its import
// attachment name, the chain an operator edits.
func floodingAttachments() map[redistevents.ProtocolID]string {
	out := make(map[redistevents.ProtocolID]string)
	for _, imp := range ribReg.importList() {
		if imp.Floods {
			out[imp.protocolID] = imp.Attachment
		}
	}
	return out
}

// centralAttachment names the central chain's attachment, or "" when none is
// registered.
func centralAttachment() string {
	for _, imp := range ribReg.importList() {
		if imp.Scope == RIBScopeCentral {
			return imp.Attachment
		}
	}
	return ""
}

// igpImportHidesNextHop returns one warning per hidden flooding-source prefix
// that is the most specific cover of a selected route's next hop: the hidden
// prefix is longer than the cover the next hop resolves through now. It is
// a control-plane scan, run only on a doctor request.
func igpImportHidesNextHop(rib *locrib.RIB, flooding map[redistevents.ProtocolID]string) []diagnostic.Diagnostic {
	if len(flooding) == 0 {
		return nil
	}
	hidden := hiddenFloodedPaths(rib, flooding)
	if len(hidden) == 0 {
		return nil
	}
	var out []diagnostic.Diagnostic
	for nextHop, route := range selectedNextHops(rib) {
		h, ok := longestHiddenCover(hidden, nextHop)
		if !ok {
			continue
		}
		_, cover, resolved := rib.LPM(h.family, nextHop)
		if resolved && cover.Bits() >= h.prefix.Bits() {
			continue
		}
		out = append(out, hidesNextHopDiagnostic(h, nextHop, route))
	}
	slices.SortFunc(out, func(a, b diagnostic.Diagnostic) int {
		if a.Message < b.Message {
			return -1
		}
		if a.Message > b.Message {
			return 1
		}
		return 0
	})
	return out
}

// hiddenFloodedPaths lists every path a flooding source offered that a chain
// hid, with the attachment of the chain to relax.
func hiddenFloodedPaths(rib *locrib.RIB, flooding map[redistevents.ProtocolID]string) []hiddenIGPPath {
	central := centralAttachment()
	var hidden []hiddenIGPPath
	for _, fam := range rib.Families() {
		rib.Iterate(fam, func(prefix netip.Prefix, g locrib.PathGroup) bool {
			for i := range g.Paths {
				p := &g.Paths[i]
				if !p.Hidden {
					continue
				}
				chain, floods := flooding[p.Source]
				if !floods {
					continue
				}
				if p.HiddenBy == ribpolicy.ChainRoleCentral {
					chain = central
				}
				hidden = append(hidden, hiddenIGPPath{family: fam, prefix: prefix, chain: chain})
			}
			return true
		})
	}
	return hidden
}

// selectedNextHops maps every next hop a selected path carries to one prefix
// that uses it, so each next hop is judged once however many routes share it.
func selectedNextHops(rib *locrib.RIB) map[netip.Addr]netip.Prefix {
	out := make(map[netip.Addr]netip.Prefix)
	for _, fam := range rib.Families() {
		rib.Iterate(fam, func(prefix netip.Prefix, g locrib.PathGroup) bool {
			if g.Best < 0 {
				return true
			}
			if g.Best >= len(g.Paths) {
				return true
			}
			nextHop := g.Paths[g.Best].NextHop
			if !nextHop.IsValid() {
				return true
			}
			if _, seen := out[nextHop]; !seen {
				out[nextHop] = prefix
			}
			return true
		})
	}
	return out
}

// longestHiddenCover returns the most specific hidden prefix containing addr.
func longestHiddenCover(hidden []hiddenIGPPath, addr netip.Addr) (hiddenIGPPath, bool) {
	var best hiddenIGPPath
	found := false
	for _, h := range hidden {
		if !h.prefix.Contains(addr) {
			continue
		}
		if found && h.prefix.Bits() <= best.prefix.Bits() {
			continue
		}
		best, found = h, true
	}
	return best, found
}

// hidesNextHopDiagnostic names the hidden prefix, the chain that hid it, the
// next hop and a route that depends on it, and the fix.
func hidesNextHopDiagnostic(h hiddenIGPPath, nextHop netip.Addr, route netip.Prefix) diagnostic.Diagnostic {
	var tb textbuf.Buffer
	tb.Str("the ").Str(h.chain).Str(" chain hides ").Str(h.prefix.String()).
		Str(", the most specific route to next hop ").Str(nextHop.String()).
		Str(" of the selected route ").Str(route.String()).
		Str("; the IGP still floods ").Str(h.prefix.String()).
		Str(", so routers downstream send its traffic here and it is blackholed or loops")
	var help textbuf.Buffer
	help.Str("let the ").Str(h.chain).Str(" chain accept ").Str(h.prefix.String()).
		Str(", or stop originating it in the IGP; `show rib candidates prefix ").
		Str(h.prefix.String()).Str("` shows the hidden path")
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorPolicyIGPImportHidesNextHop,
		Severity: diagnostic.SeverityWarning,
		Message:  tb.String(),
		Help:     help.String(),
	}
}
