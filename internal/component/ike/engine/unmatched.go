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
	"github.com/ze-software/ze/internal/component/ike/ipsec"
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
var errUnmatchedDiscardNoDataplane = errors.New("ike: `unmatched discard` (written, or the default when the leaf is unset) needs a dataplane to install the SPD catch-all entry, and none is loaded: " +
	"the daemon log names why the dataplane failed to load; fix that, or `set vpn ipsec unmatched bypass` to run without the entry")

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
		return fmt.Errorf("ike: `unmatched discard` (written, or the default when the leaf is unset): the dataplane backend %T does not say whether it can hold the SPD catch-all entry; "+
			"`set vpn ipsec unmatched bypass` to run without the entry", dp)
	}
	if err := capable.CatchAllSupported(); err != nil {
		return fmt.Errorf("ike: `unmatched discard` (written, or the default when the leaf is unset) cannot be enforced: %w; "+
			"`set vpn ipsec unmatched bypass` to run without the entry", err)
	}
	return nil
}

// refuseImplicitDiscard is the commit check that guards the default disposition.
//
// RFC 4301 Section 4.4.1: "Every SPD SHOULD have a nominal, final entry that matches
// anything that is otherwise unmatched, and discards it." So `unmatched` defaults to
// discard, and that entry also drops the router's own traffic wherever it crosses in
// the clear (implicitDiscardCuts names it). The flows Ze would have to exempt are spread
// over every component's configuration and the kernel's routes, so an automatic
// exemption list would be incomplete, and what it missed would be cut with nobody
// told. Instead a commit that would turn the discard on WITHOUT the operator writing
// it is refused, and the refusal names what stays open and the two ways forward.
//
// It refuses only that transition: the leaf absent (cfg.UnmatchedByDefault), the
// result discard, and no discard in force already (discardInForce, set by the last
// apply). A written leaf, either value, always passes, and so does a commit to a box
// already discarding, which the operator has seen. A configuration the daemon STARTS
// with is applied without this check, because the reload pipeline alone verifies;
// warnImplicitDiscardAtStart says so at start instead.
func refuseImplicitDiscard(cfg *ipsec.IPsecConfig, discardInForce bool) error {
	if !cfg.UnmatchedByDefault {
		return nil
	}
	if cfg.Unmatched != dataplane.SPActionDiscard {
		return nil
	}
	if discardInForce {
		return nil
	}
	return fmt.Errorf("this commit turns on the SPD catch-all DISCARD without saying so: "+
		"`vpn ipsec unmatched` is not set and its default is discard (RFC 4301 Section 4.4.1). "+
		"Once applied, every packet this router sends, receives or forwards that no SPD entry names is dropped, "+
		"including its "+implicitDiscardCuts+"; "+
		"only IKE (UDP 500 and 4500), the negotiated tunnels and the %d `vpn ipsec policy` bypass entries stay open. "+
		"Fix: add a `vpn ipsec policy <name>` bypass entry for each flow that must stay in the clear "+
		"("+implicitDiscardICMPExample+"), "+
		"then `set vpn ipsec unmatched discard` to confirm; "+
		"or `set vpn ipsec unmatched bypass` to keep passing that traffic", countBypassEntries(cfg))
}

// implicitDiscardCuts names the router's own traffic the discard catch-all drops
// where it crosses in the clear. It is one string because the commit refusal and the
// start log say the same thing. ICMP is in it because `show mtu`, ping, traceroute
// and the path-MTU errors that size every packet ride it. IPv6 neighbor discovery
// is in it because the catch-all matches every protocol, ICMPv6 included, so an IPv6
// neighbor is never resolved: an unprivileged netns probe with an any-protocol XFRM
// block in and out showed the neighbor FAILED, and REACHABLE once protocol 58 was
// bypassed.
const implicitDiscardCuts = "own BGP, SSH, DNS and NTP traffic, its ICMP (ping, traceroute, " +
	"`show mtu` probes and the path-MTU errors that size its packets) and its IPv6 neighbor discovery, " +
	"outside a tunnel"

// implicitDiscardICMPExample is the bypass that keeps ICMP and IPv6 neighbor
// discovery working under the discard catch-all. Order 3000 ranks it after a
// tunnel's default 2000, so ICMP a tunnel's selectors cover still takes the tunnel.
const implicitDiscardICMPExample = "for ICMP: `set vpn ipsec policy icmp4 action bypass`, " +
	"`set vpn ipsec policy icmp4 protocol 1`, `set vpn ipsec policy icmp4 order 3000`, " +
	"`set vpn ipsec policy icmp4 local prefix 0.0.0.0/0`, `set vpn ipsec policy icmp4 remote prefix 0.0.0.0/0`, " +
	"and the same `icmp6` entry with protocol 58 and prefix ::/0 for ICMPv6 and neighbor discovery"

// warnImplicitDiscardAtStart is the start-time half of refuseImplicitDiscard. The
// configuration the daemon starts with is never verified, so a box booting a file
// whose `vpn ipsec` block leaves `unmatched` unset installs the discard with no
// commit to refuse. A file stamped by a release from before the default became
// discard never gets here: the schema evolution in ipsec/unmatched_evolution.go
// writes `unmatched bypass` into it first. What remains is a file whose stamp
// proves no older release: written by hand, piped on stdin, stamped by a dev build,
// or saved by a release that already defaulted to discard. Each may mean the RFC
// default, and installing bypass for them would make the default dead at every
// boot. So the discard is installed, and the start says it, at Error level, with
// the same facts and commands as the refusal.
func warnImplicitDiscardAtStart(cfg *ipsec.IPsecConfig, log *slog.Logger) {
	if !cfg.UnmatchedByDefault {
		return
	}
	if cfg.Unmatched != dataplane.SPActionDiscard {
		return
	}
	log.Error("ike: the SPD catch-all DISCARD is in force because `vpn ipsec unmatched` is not set "+
		"and its default is discard (RFC 4301 Section 4.4.1): every packet no SPD entry names is dropped, "+
		"including this router's "+implicitDiscardCuts+"; "+
		"fix: add a `vpn ipsec policy <name>` bypass entry for each flow that must stay in the clear "+
		"("+implicitDiscardICMPExample+"), "+
		"then `set vpn ipsec unmatched discard` to confirm, or `set vpn ipsec unmatched bypass` to pass that traffic",
		"bypass-entries", countBypassEntries(cfg))
}

// countBypassEntries counts the operator's `vpn ipsec policy` entries whose action
// is bypass: the flows that stay open under the discard catch-all.
func countBypassEntries(cfg *ipsec.IPsecConfig) int {
	entries := 0
	for name := range cfg.Policies {
		if cfg.Policies[name].Action == dataplane.SPActionBypass {
			entries++
		}
	}
	return entries
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
