// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: bypass.go -- the IKE control-plane bypass, the FIRST entry of the database
// Related: spd_policy.go -- the operator's entries, searched between the two
// RFC: rfc/short/rfc4301.md -- Section 5, the disposition of an unmatched packet

package engine

import (
	"errors"
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
// It runs on every apply, ahead of the operator's entries (apply.go), and the backend
// upserts a template-free policy (xfrmBackend.InstallPolicy), so a changed
// disposition replaces the entry under the same selector rather than adding a second.
//
// A DISCARD that is not installed is an error, whatever stopped it: no dataplane
// loaded, a backend that holds no entry bound to no interface, or an install that
// failed. The kernel passes a packet no entry matches, so a missing discard entry
// fails OPEN, and the caller fails the apply rather than report the configuration in
// force. A BYPASS that is not installed is logged and tolerated: with no entry the
// kernel passes the packet, which is what the entry would have done.
func installUnmatched(dp dataplane.Dataplane, action dataplane.SPAction, log *slog.Logger) error {
	discard := action == dataplane.SPActionDiscard
	if dp == nil {
		if discard {
			return errUnmatchedDiscardNoDataplane
		}
		return nil
	}
	for _, family := range ikeBypassFamilies {
		policies, err := unmatchedPolicies(family, action)
		if err != nil {
			return fmt.Errorf("ike: build the SPD catch-all entry: %w", err)
		}
		for _, p := range policies {
			err := dp.InstallPolicy(p)
			if err == nil {
				continue
			}
			// RFC 4301 Section 4.4.1: "Every SPD SHOULD have a nominal, final entry
			// that matches anything that is otherwise unmatched, and discards it."
			// RFC 4301 Section 5: "If no policy is found in the SPD that matches a
			// packet (for either inbound or outbound traffic), the packet MUST be
			// discarded." The operator asked for that entry, so a discard that did not
			// reach the dataplane refuses the configuration.
			if discard {
				return fmt.Errorf("ike: install the SPD catch-all discard entry (dir %d): %w", p.Dir, err)
			}
			if isXFRMUnsupported(err) {
				log.Debug("ike: unmatched policies unavailable on this platform", "error", err)
				return nil
			}
			log.Warn("ike: could not install the SPD catch-all bypass entry; traffic no entry matches crosses the IPsec boundary unchanged",
				"dir", p.Dir, "error", err)
		}
	}
	return nil
}

// errUnmatchedDiscardNoDataplane is the refusal of a discard catch-all when no
// dataplane is loaded: runEngine logs a failed dataplane.Load and carries on, and a
// discard then has nothing to be installed into.
var errUnmatchedDiscardNoDataplane = errors.New("ike: `unmatched discard` needs a dataplane to install the SPD catch-all entry, and none is loaded")

// verifyUnmatchedEnforceable is the config-verify half of installUnmatched: it refuses
// `unmatched discard` when the loaded dataplane says, ahead of any install, that it
// cannot hold the catch-all, so the operator's commit fails rather than the apply.
// A backend that does not implement dataplane.CatchAllInstaller has declared nothing
// and is treated as unable. A bypass is never refused, for the reason installUnmatched
// gives.
func verifyUnmatchedEnforceable(dp dataplane.Dataplane, action dataplane.SPAction) error {
	if action != dataplane.SPActionDiscard {
		return nil
	}
	if dp == nil {
		return errUnmatchedDiscardNoDataplane
	}
	capable, ok := dp.(dataplane.CatchAllInstaller)
	if !ok {
		return fmt.Errorf("ike: `unmatched discard`: the dataplane backend %T does not say whether it can hold the SPD catch-all entry", dp)
	}
	if err := capable.CatchAllSupported(); err != nil {
		return fmt.Errorf("ike: `unmatched discard` cannot be enforced: %w", err)
	}
	return nil
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
