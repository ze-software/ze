// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: bypass.go -- the IKE control-plane bypass, the FIRST entry of the database
// Related: spd_policy.go -- the operator's entries, searched between the two
// RFC: rfc/short/rfc4301.md -- Section 5, the disposition of an unmatched packet

package engine

import (
	"fmt"
	"log/slog"
	"net"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// unmatchedDirections are the three halves of the database the catch-all joins.
//
// A security gateway forwards as well as terminates, and the kernel checks a
// forwarded packet against its fwd policies rather than its in policies, so a
// catch-all in and out alone would leave transit traffic with no entry to meet.
// The IKE bypass omits fwd for the opposite reason (ikeBypassPolicies): IKE is never
// forwarded.
var unmatchedDirections = [...]dataplane.SADir{
	dataplane.SADirIn, dataplane.SADirOut, dataplane.SADirFwd,
}

// unmatchedPolicies builds the catch-all entry of the SPD in one address family.
//
// RFC 4301 Section 5: "If no policy is found in the SPD that matches a packet (for
// either inbound or outbound traffic), the packet MUST be discarded." The Linux
// kernel does the opposite when no policy matches: it passes the packet in the
// clear. So the discard is not a kernel default Ze can rely on; it is an entry Ze
// installs, ranked last (dataplane.PriorityUnmatched) so that every operator entry
// and every Child SA policy is searched before it.
//
// The selector is the wildcard of the family, any protocol, any port. It is
// family-correct for the reason anyNetFor gives: one selector carries one family.
//
// The disposition is the operator's choice (the `unmatched` leaf, ipsec/config.go
// parseUnmatched). A bypass catch-all reproduces what the kernel did with no entry,
// and it is still an ENTRY: the packet met a policy, so Section 5 has nothing left to
// say about it. PROTECT and any other action is refused rather than installed,
// because a catch-all that hands every unmatched packet to a transform names no
// transform to hand it to.
func unmatchedPolicies(family net.IP, action dataplane.SPAction) ([]dataplane.SPParams, error) {
	if action != dataplane.SPActionDiscard && action != dataplane.SPActionBypass {
		return nil, fmt.Errorf("ike: unmatched disposition %d is neither discard (%d) nor bypass (%d)",
			action, dataplane.SPActionDiscard, dataplane.SPActionBypass)
	}
	anyNet := anyNetFor(family)
	out := make([]dataplane.SPParams, 0, len(unmatchedDirections))
	for _, dir := range unmatchedDirections {
		out = append(out, dataplane.SPParams{
			Src:      anyNet,
			Dst:      anyNet,
			Dir:      dir,
			Action:   action,
			Priority: dataplane.PriorityUnmatched,
			SrcPort:  dataplane.AnyPortMatch(),
			DstPort:  dataplane.AnyPortMatch(),
		})
	}
	return out, nil
}

// installUnmatched installs the catch-all for every address family.
//
// It runs on every apply, after the operator's entries (apply.go), and the backend
// upserts a template-free policy (xfrmBackend.InstallPolicy), so a changed
// disposition replaces the entry under the same selector rather than adding a second.
//
// An install failure is logged and never fatal, and it is loud for a DISCARD: that
// failure fails OPEN, because the kernel then passes what the operator asked to
// stop, and nothing else reports it. A platform with no XFRM is tolerated the way
// installIKEBypass tolerates it.
func installUnmatched(dp dataplane.Dataplane, action dataplane.SPAction, log *slog.Logger) {
	if dp == nil {
		return
	}
	for _, family := range ikeBypassFamilies {
		policies, err := unmatchedPolicies(family, action)
		if err != nil {
			log.Error("ike: could not build the SPD catch-all entry", "error", err)
			return
		}
		for _, p := range policies {
			err := dp.InstallPolicy(p)
			if err == nil {
				continue
			}
			if isXFRMUnsupported(err) {
				log.Debug("ike: unmatched policies unavailable on this platform", "error", err)
				return
			}
			log.Warn("ike: could not install the SPD catch-all entry; traffic no entry matches crosses the IPsec boundary unchanged",
				"action", spdActionName(action), "dir", p.Dir, "error", err)
		}
	}
}

// removeUnmatched releases the catch-all for every address family, on every exit
// of the engine, for the reason removeIKEBypass gives: a DISCARD that outlives the
// process keeps dropping traffic for a daemon that is no longer running.
//
// The disposition does not matter to a removal, because the kernel identifies a
// policy by its selector alone, so the bypass form is built and its selector used.
func removeUnmatched(dp dataplane.Dataplane, log *slog.Logger) {
	if dp == nil {
		return
	}
	for _, family := range ikeBypassFamilies {
		policies, err := unmatchedPolicies(family, dataplane.SPActionBypass)
		if err != nil {
			log.Error("ike: could not build the SPD catch-all entry for removal", "error", err)
			return
		}
		for _, p := range policies {
			if err := dp.RemovePolicyParams(p); err != nil {
				log.Debug("ike: remove SPD catch-all entry", "dir", p.Dir, "error", err)
			}
		}
	}
}
