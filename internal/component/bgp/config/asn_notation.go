// Design: docs/architecture/config/syntax.md — the bgp/as-notation leaf
// Overview: loader_create.go — the two apply paths that call this
// Related: internal/core/bgp/asn/asn.go — Configure, the one writer of the value
package bgpconfig

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/bgp/asn"
)

// applyASNotation records the notation this process writes an AS number in.
//
// One place calls it: the initial reactor build (CreateReactorFromTree,
// loader_create.go). That is the daemon adopting its first configuration, and
// a failure there stops the daemon rather than leaving it running on another.
//
// Nothing on the RELOAD path calls it. A reload is verified before it is
// applied, and the verify phase runs every function the apply phase runs.
// VerifyConfig and ApplyConfigDiff both reach the reload builder through
// loadPeersFullOrTree (../reactor/reactor_api.go). The reload path records the
// notation at SetConfigTree instead, which the coordinator calls last.
//
// The distinction is the point. `ze config validate`, `ze doctor` and the web
// commit path all resolve a CLONE of the tree, so that a candidate cannot
// change a running daemon. A process-wide setting written during resolution
// walks through that clone. An operator whose commit is refused would then be
// left with a process rendering a notation that never took effect. Nothing
// would put it back.
//
// A tree with no bgp block leaves the previous value in place. This function
// runs inside the BGP loader, which is not reached without one.
func applyASNotation(tree *config.Tree) error {
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return nil
	}
	if err := asn.ConfigureFromBGP(bgp.ToMap()); err != nil {
		return fmt.Errorf("bgp/%s: %w", asn.LeafName, err)
	}
	return nil
}
