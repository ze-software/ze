// Design: docs/guide/bfd.md -- the operator surface for BFD strict mode
// RFC: rfc/drafts/draft-ietf-idr-bgp-bfd-strict-mode.txt -- Sections 6 and 8
// Overview: peers.go -- the peer pipeline this check reads
// Related: register.go -- the doctor-check registration from init()
//
// The one arrangement BFD strict mode cannot survive: a peer that asks for it
// while no BFD engine is configured to answer.
//
// A strict peer holds its BGP session in OpenSent until the BFD session to that
// neighbor is Up (draft Section 8.5.5), and with no BFD plugin no session ever
// opens, so the peer never establishes and nothing on the peer's own surface
// says why. The runtime half logs at error (reactor/peer_bfd.go,
// startBFDClient); this is the half that answers before the daemon starts.
package bgpconfig

import (
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// codeBFDStrictWithoutEngine is raised when a peer configures BFD strict mode
// and the configuration declares no top-level bfd block for the engine that
// would answer it.
const codeBFDStrictWithoutEngine = "doctor-bgp-bfd-strict-without-engine"

var bfdStrictDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codeBFDStrictWithoutEngine,
		Title: "BFD strict mode is configured with no BFD engine",
		Description: "A BGP peer sets `bfd { strict true; }`, which holds its BGP session out of Established " +
			"until the BFD session to that neighbor reaches Up (draft-ietf-idr-bgp-bfd-strict-mode Section 8.5.5). " +
			"The configuration declares no top-level `bfd { ... }` block, so no BFD engine runs and no session can ever come up: " +
			"the peer stays in OPENSENT for the life of the process. " +
			"Add a top-level `bfd { enabled true; ... }` block, or remove `strict true` from the peer.",
		Examples: []string{"ze doctor --json", "ze explain doctor-bgp-bfd-strict-without-engine"},
	},
}

// bfdStrictDoctorCheck describes the check. register.go registers it.
var bfdStrictDoctorCheck = diagnostic.DoctorCheck{
	Name:         "bgp-bfd-strict-engine",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        746,
	Component:    "bgp",
	Dependencies: []string{"config-tree"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeBFDStrictWithoutEngine},
	Check:        checkBFDStrictHasEngine,
}

// checkBFDStrictHasEngine reports every peer that asks for strict mode in a
// configuration with no BFD engine.
//
// It reads the same tree the peer pipeline reads, so the names it prints are
// the names the operator wrote. An unreadable configuration yields nothing:
// doctor's own peer-validation check already reports a config the engine
// refuses, and naming one failure twice tells the operator nothing new.
func checkBFDStrictHasEngine(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	names := strictPeersWithoutEngine(tree)
	if len(names) == 0 {
		return nil
	}
	var detail textbuf.Buffer
	detail.Str("peers with bfd strict and no bfd engine: ").Str(textbuf.Join(names, ", "))
	return []diagnostic.Diagnostic{{
		Code:     codeBFDStrictWithoutEngine,
		Severity: diagnostic.SeverityError,
		Message:  detail.String(),
	}}
}

// strictPeersWithoutEngine names the peers that configure BFD strict mode where
// the configuration declares no top-level bfd block, in configuration order.
//
// It is the decidable half of the question. Whether the BFD plugin is RUNNING
// is a runtime fact this cannot see, and startBFDClient answers that one; what
// a configuration alone can settle is that no engine was asked for at all.
//
// MUST be called on a CLONE where the caller prunes: it does not prune, because
// its two callers already hold a pruned tree.
func strictPeersWithoutEngine(tree *config.Tree) []string {
	if tree.GetContainer("bfd") != nil {
		return nil
	}
	bgpTree, err := ResolveBGPTree(tree)
	if err != nil {
		return nil
	}
	peers, err := reactor.PeersFromTree(bgpTree)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(peers))
	for _, ps := range peers {
		if ps.BFD == nil || !ps.BFD.Enabled || !ps.BFD.Strict {
			continue
		}
		name := ps.Name
		if name == "" {
			name = ps.Address.String()
		}
		names = append(names, name)
	}
	return names
}
