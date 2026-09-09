// Design: docs/architecture/core-design.md -- selection, and the FIB write that follows it
// Overview: sysrib.go -- recomputeBest and cascadeRecompute, the two producers this gates
// Related: register.go -- the configure callbacks that install the permission set
// Related: ../../core/redistevents/registry.go -- ProtocolNames, the vocabulary
//
// Which protocols reach the FIB. `rib { fib-withhold [ bgp isis ] }` names the
// protocols Ze does not program. Selection is untouched: a withheld protocol
// still competes for a prefix on administrative distance and still wins it, and
// the winner stays in the system RIB, on the plugin bus, and available to
// redistribution. Only the write to the kernel, to VPP or to a P4 switch is
// declined.

package sysrib

import (
	"encoding/json"
	"fmt"
	"net/netip"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// fibWithholdLeaf is the leaf-list under `rib` that names the protocols whose
// routes are not written. The name carries the polarity: a list called
// fib-import would not say whether the names in it are the permitted set or the
// excluded one.
const fibWithholdLeaf = "fib-withhold"

// parseFIBImportConfig returns, for every REGISTERED protocol, whether its
// routes are written to the FIB.
//
// The map is COMPLETE over redistevents.ProtocolNames() whether or not the
// operator wrote the leaf, and that is the point of it. The default is then a
// named branch rather than a map miss, so a reader never decides what an absent
// key means, and a protocol nobody named is permitted because the map SAYS so.
// A config that names nothing programs everything, which is what a router
// without this setting does.
//
// A withheld name that nothing registered is not an error here. The config walk
// refuses it first (the `registered-protocol` validator,
// internal/component/config/validators.go), and a name that arrives despite
// that names no protocol, so no route can carry it.
func parseFIBImportConfig(jsonData string) (map[string]bool, error) {
	var tree map[string]any
	if err := json.Unmarshal([]byte(jsonData), &tree); err != nil {
		return nil, fmt.Errorf("unmarshal sysrib config: %w", err)
	}

	withheld := map[string]bool{}
	if section := configvalue.Section(configRootRIB, tree); section != nil {
		for _, name := range configvalue.LeafList(section[fibWithholdLeaf]) {
			withheld[name] = true
		}
	}

	names := redistevents.ProtocolNames()
	permit := make(map[string]bool, len(names))
	for _, name := range names {
		permit[name] = !withheld[name]
	}
	return permit, nil
}

// fibPermits reports whether a permission set writes this protocol's routes to
// the FIB. It is the one reading of the map, so the sweep that compares two
// sets and the read the arbitration takes cannot answer differently.
//
// A protocol the set does not hold is PERMITTED. fibPermitted, the caller that
// speaks for the running configuration, says why.
func fibPermits(permit map[string]bool, protocol string) bool {
	permitted, declared := permit[protocol]
	return !declared || permitted
}

// fibPermitted reports whether this protocol's routes are written to the FIB.
//
// A protocol the permission set does not hold is PERMITTED, and the name is
// spoken once. Two states reach that branch: a route arriving before the
// configure callback has run, and a protocol that registered after it. Reading
// either as "do not program" would blackhole every route the protocol carries,
// which is far worse than programming a route the operator meant to withhold
// and never said so about. This is the direction redistevents.OSInstalled
// documents for the same hazard, decided at the call site rather than in the
// registry (ai/rules/principles.md).
//
// Safe for concurrent use. The caller MUST NOT hold fibMu.
func (s *sysRIB) fibPermitted(protocol string) bool {
	s.fibMu.Lock()
	defer s.fibMu.Unlock()

	if _, declared := s.fibPermit[protocol]; declared {
		return fibPermits(s.fibPermit, protocol)
	}

	if !s.fibSpoken[protocol] {
		s.fibSpoken[protocol] = true
		reason := "the protocol registered after the configuration was read"
		if len(s.fibPermit) == 0 {
			reason = "a route arrived before the FIB import configuration"
		}
		logger().Warn("sysrib: no FIB import declaration for this protocol, programming its routes",
			"protocol", protocol, "reason", reason)
	}
	return true
}

// recordWithheldWinner records a winner whose protocol the operator withholds
// from the FIB, and answers with the change that leaves the forwarding table
// holding nothing for the prefix.
//
// The winner enters the system RIB exactly as any other, because withholding
// decides what is PROGRAMMED and never what is SELECTED: `show rib` reports the
// prefix, the plugin bus carries it, and redistribution can offer it onward.
//
// When Ze HAD programmed the prefix for a previous winner, that entry is now
// stale -- the RIB says this prefix belongs to a protocol Ze does not program,
// while the kernel still forwards on the path it beat -- so it is WITHDRAWN.
// Silence would leave the router forwarding on a route it no longer selects.
//
// REQUIRES: the caller holds s.mu for writing.
func (s *sysRIB) recordWithheldWinner(key prefixKey, prev, winner *protocolRoute, protocols map[string]*protocolRoute) *outgoingChange {
	hadZeRoute := s.programmedByZe(key)
	s.best[key] = winner
	s.lastECMP[key] = s.ecmpCollect(protocols, winner)

	// The next-hop is tracked for a route Ze programs, so the resolver can tell
	// it when the path to that next-hop changes. Nothing is programmed here, so
	// the previous winner's tracking is released and the withheld winner's is
	// never taken. recomputeBest takes it again when a permitted protocol wins
	// the prefix back, whether or not the next-hop changed.
	if prev != nil {
		untrackNextHops(key.prefix, prev)
	}

	if !hadZeRoute {
		return nil
	}
	delete(s.resolvedNH, key)
	logger().Info("sysrib: withdrawing the route Ze programmed, the prefix is held by a withheld protocol now",
		"prefix", key.prefix, "protocol", winner.protocol)
	return &outgoingChange{
		Action: routeaction.Withdraw,
		Prefix: key.prefix,
	}
}

// trackNextHops asks the resolver to re-evaluate this prefix when the path to
// the route's next-hop changes. It is taken for a route Ze MEANS to program,
// and only those: processCascade drives off the tracking table, so an entry for
// a prefix Ze declined would re-evaluate one the kernel does not hold.
//
// Meaning to program is wider than programming. A prefix whose next-hop or
// whose SRv6 SID does not resolve is tracked with nothing installed, which is
// what brings it back when the resolution returns. The two paths that DECLINE
// the prefix release it instead: recordWithheldWinner and
// recordOSInstalledWinner.
//
// The pair MUST balance. Every path that takes a prefix on calls this, and
// every path that gives it up calls untrackNextHops.
func trackNextHops(prefix netip.Prefix, route *protocolRoute) {
	r := getNHResolver()
	if r == nil {
		return
	}
	if route.nextHop.IsValid() {
		r.Track(route.nextHop, prefix)
	}
	if route.srv6SID.IsValid() {
		r.Track(route.srv6SID, prefix)
	}
}

// untrackNextHops releases what trackNextHops took. It MUST be called by every
// path that stops programming a prefix, or the resolver walks the prefix on
// every cascade and `show rib next-hop` reports a dependency Ze does not hold.
func untrackNextHops(prefix netip.Prefix, route *protocolRoute) {
	r := getNHResolver()
	if r == nil {
		return
	}
	if route.nextHop.IsValid() {
		r.Untrack(route.nextHop, prefix)
	}
	if route.srv6SID.IsValid() {
		r.Untrack(route.srv6SID, prefix)
	}
}

// fibChange renders the change that programs a prefix for its current winner.
//
// The action is Add, because fibEntry is the one caller and it answers what the
// FIB owes rather than what Ze already holds. A caller that knows Ze holds an
// install for the prefix overwrites the action with Update, which is the one
// thing neither function can see.
func fibChange(key prefixKey, route *protocolRoute, nextHop netip.Addr, paths []sysribevents.ECMPPath) outgoingChange {
	return outgoingChange{
		Action:    routeaction.Add,
		Prefix:    key.prefix,
		NextHop:   nextHop,
		Interface: route.nextHopInterface,
		Weight:    route.nextHopWeight,
		Protocol:  route.protocol,
		Labels:    route.labels,
		SRv6SID:   route.srv6SID,
		RouteType: route.routeType,
		Metric:    route.metric,
		ECMPPaths: paths,
		Backup:    backupPaths(route),
	}
}

// applyFIBImport installs a new permission set and answers with the changes the
// switch owes, grouped by family: a withdraw for each prefix Ze programs and
// the new set withholds, an add for each prefix it newly permits, and an update
// for each programmed prefix whose multipath group lost or gained a member.
//
// The sweep is what makes the setting take effect NOW. Without it a withheld
// protocol keeps every route it had already programmed until the protocol
// happens to send an update for that prefix, so the operator's change appears
// to do nothing on a converged router, which is the state a converged router is
// usually in.
//
// What decides is the PERMISSION that CHANGED, never the FIB state on its own.
// A prefix Ze does not program is absent for three reasons and this setting is
// only one of them: the OS owns the entry, the next-hop stopped resolving, or
// the operator withholds the protocol. Acting on absence alone would program
// the first two here, and publishFIBImport runs on every configure and every
// rollback, so an edit elsewhere in `rib` would put an unreachable route back.
//
// The group is swept as well as the winner, because a member is programmed
// exactly as the winner is. Withholding a protocol that holds no prefix of its
// own still takes its gateway out of every group it shares.
//
// Caller MUST NOT hold s.mu.
func (s *sysRIB) applyFIBImport(permit map[string]bool) map[family.Family][]outgoingChange {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.fibMu.Lock()
	previous := s.fibPermit
	s.fibPermit = permit
	// A protocol reported as undeclared against the previous set may be
	// declared by this one, and the operator is owed the line if it is not.
	clear(s.fibSpoken)
	s.fibMu.Unlock()

	changesByFamily := make(map[family.Family][]outgoingChange)
	for key, route := range s.best {
		if !s.fibPermitted(route.protocol) {
			// The tracking goes whether or not the prefix is programmed. It is
			// taken for a route Ze MEANS to program, and a prefix waiting on a
			// next-hop or an SRv6 SID that does not resolve holds it with
			// nothing installed (sysrib.go, recomputeBest).
			untrackNextHops(key.prefix, route)
			delete(s.lastECMP, key)
			if !s.programmedByZe(key) {
				continue
			}
			delete(s.resolvedNH, key)
			changesByFamily[key.family] = append(changesByFamily[key.family], outgoingChange{
				Action: routeaction.Withdraw,
				Prefix: key.prefix,
			})
			continue
		}

		if fibPermits(previous, route.protocol) {
			// The winner kept its permission, so the prefix owes a change only
			// where a MEMBER of its group gained or lost one.
			if !s.groupPermissionChanged(key, route, previous) {
				continue
			}
		} else {
			// Newly permitted. The tracking the withhold released comes back
			// BEFORE the state is computed, so the prefix is re-evaluated when
			// the path to its next-hop changes, whether or not it is programmed
			// now. An OS-installed winner takes none, because Ze programs
			// nothing for it whichever way the permission goes.
			if !route.osInstalled {
				trackNextHops(key.prefix, route)
			}
		}

		if change := s.fibStateChange(key, route); change != nil {
			changesByFamily[key.family] = append(changesByFamily[key.family], *change)
		}
	}
	return changesByFamily
}

// groupPermissionChanged reports whether the permission switch added a member
// to the prefix's equal-cost group or took one out of it. It is what the sweep
// acts on for a prefix whose WINNER kept its permission.
//
// Both sides are collected the same way and differ only in the permission set
// they read, so the answer is the permission change and nothing else. Comparing
// against the group Ze last emitted would answer for a next-hop that stopped
// resolving as well, and that one is not this setting's to reverse: every
// unrelated `rib` apply would then rewrite the multipath of a converged router.
//
// REQUIRES: the caller holds s.mu.
func (s *sysRIB) groupPermissionChanged(key prefixKey, winner *protocolRoute, previous map[string]bool) bool {
	protocols := s.routes[key]
	before := s.ecmpGroup(protocols, winner, func(protocol string) bool {
		return fibPermits(previous, protocol)
	})
	after := s.ecmpGroup(protocols, winner, s.fibPermitted)
	return ecmpChanged(before, after)
}

// fibStateChange answers with the change a prefix owes after the permission set
// changed, for a winner the new set permits: an Add for one the FIB does not
// hold, an Update where the entry under it moved, and nothing where the FIB
// already holds what the RIB says.
//
// What the FIB owes is fibEntry's answer, the one the live path emits and the
// cascade and the replay act on. It re-resolves the winner, promotes a member
// when the winner's gateway is unreachable, and filters the group by
// reachability and by permission. Collecting the group here instead would
// compare an unfiltered set against one the cascade wrote: the sweep would put
// a member the resolver dropped back into the kernel multipath, and pair a
// promoted member's address with the winner's device.
//
// The VERDICT is read the way recomputeBest reads it, and NOT the way
// cascadeRecompute reads it. A cascade is reachability NEWS, so a verdict of
// anything but reachable says the prefix lost the path it was programmed over.
// A permission change is no news about a path. It is a fresh install decision,
// and the Loc-RIB is not the router's whole picture of reachability: an OSPF or
// IS-IS next-hop on a link whose connected route no plugin inserted resolves to
// nothing and is on-link all the same, which is why recomputeBest programs it
// (test/ospf/ospf-route-install.ci). So the sweep programs it too.
//
// Taking the cascade's rule here loses the prefix twice over. A permit
// publishes NOTHING and nothing later repairs it, because an identical
// re-announcement is a no-op at recomputeBest and a cascade fires only when a
// covering route changes. And withholding one MEMBER of an equal-cost group
// withdraws a prefix its permitted winner still holds, which blackholes it.
//
// REQUIRES: the caller holds s.mu, and has taken the next-hop tracking for a
// prefix it newly permits.
func (s *sysRIB) fibStateChange(key prefixKey, route *protocolRoute) *outgoingChange {
	// The OS creates the forwarding entry for this prefix, so Ze installs
	// nothing whichever way the permission goes. fibEntry answers for the
	// prefix rather than for the writer, so the test is owed here.
	if route.osInstalled {
		return nil
	}

	programmed := s.programmedByZe(key)

	// RFC 9252 Section 5 is the one rule that forbids the write outright, and
	// fibEntry is where the SID is read. A prefix Ze holds an install for is
	// withdrawn, as recomputeBest withdraws it: the kernel entry encapsulates
	// to a SID no route reaches.
	entry, path := s.fibEntry(key, route)
	if path == fibPathForbidden {
		delete(s.lastECMP, key)
		delete(s.resolvedNH, key)
		if !programmed {
			return nil
		}
		return &outgoingChange{
			Action: routeaction.Withdraw,
			Prefix: key.prefix,
		}
	}

	if programmed && entry.NextHop == s.resolvedNH[key] && !ecmpChanged(s.lastECMP[key], entry.ECMPPaths) {
		return nil
	}

	s.resolvedNH[key] = entry.NextHop
	s.lastECMP[key] = entry.ECMPPaths
	// An Update names an entry to replace and a prefix Ze holds no install for
	// has none, so the verb follows the install rather than the change:
	// recomputeBest says what each one becomes at the kernel writer.
	if programmed {
		entry.Action = routeaction.Update
	}
	return &entry
}

// publishFIBImport installs the permission set and publishes what the switch
// owes. It is the one route from a configure callback to the FIB plugins, so
// the apply path and the rollback path cannot answer differently.
func publishFIBImport(s *sysRIB, permit map[string]bool) {
	for fam, changes := range s.applyFIBImport(permit) {
		if len(changes) > 0 {
			publishChanges(changes, fam)
		}
	}
}
