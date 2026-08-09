// Design: docs/architecture/traffic/fw-7-traffic-vpp.md -- Commit-time rejection matrix

package trafficvpp

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/component/traffic"
)

var (
	// errFilterMarkNotSupportedByBackend names the semantic gap that AC-3's
	// fallback requires. A-3 was validated against real VPP v25.10 (Docker
	// ligato/vpp-base): the classify SET_METADATA action / opaque_index stores
	// an opaque value consumed only by specific downstream graph nodes (ACL,
	// SR-policy), not a general packet mark; there is no VPP feature arc that
	// reads it back into the packet the way Linux SKB fwmark persists. So no
	// faithful mark semantic exists -- rejection is retained. Rationale and
	// evidence: docs/architecture/traffic/followup-vpp-traffic.md (A-3, AC-3).
	errFilterMarkNotSupportedByBackend = errors.New("filter mark: not supported by backend vpp (Linux SKB fwmark has no faithful VPP equivalent; classify SET_METADATA stores opaque graph-node metadata, not a persistent packet mark -- validated on VPP v25.10, see plan/spec-followup-vpp-traffic.md AC-3)")
	// errQdiscPrioNotSupportedByBackend names AC-4's rejection-retained
	// resolution (USER decision 2026-07-10): `qdisc prio` is a priority
	// SCHEDULER in netlink, but VPP exposes no prio scheduler API -- only
	// classify + policer shaping. Mapping prio to a DSCP egress-map would be a
	// silent semantic substitution that exact-or-reject forbids, so prio stays
	// rejected with this actionable error. Rationale:
	// docs/architecture/traffic/followup-vpp-traffic.md (AC-4).
	errQdiscPrioNotSupportedByBackend = errors.New("qdisc prio: not supported by backend vpp (prio is a priority scheduler; VPP has no prio scheduler API, only classify+policer shaping -- use htb/tbf with per-class protocol/dscp filters instead, see plan/spec-followup-vpp-traffic.md AC-4)")
)

// maxProtocol is the largest IP protocol / IPv6 next-header value that fits in
// the single classify byte the protocol filter matches (matches the netlink
// backend's bound).
const maxProtocol = 255

// maxPolicerNameLen is VPP's string[64] limit on policer names. The
// backend uses the format "ze/<iface>/<class>"; if the resulting name
// would exceed this, two classes could truncate to the same name and
// one policer would silently upsert the other. Reject at verify time
// so the operator picks a shorter class or interface name.
const maxPolicerNameLen = 64

// nameSeparator is the character the backend uses to compose policer
// names from interface and class parts. Names containing this separator
// in either part would produce ambiguous policer names (not round-
// trippable back to their components), so the verifier rejects them.
const nameSeparator = "/"

// Verify walks the parsed desired state and rejects qdisc and filter types
// that the VPP backend cannot represent exactly. Registered via
// traffic.RegisterVerifier("vpp", Verify); runs in OnConfigVerify before
// the backend is loaded, so operators see rejections at commit time and
// can edit the config before committing.
//
// Current scope: HTB and TBF qdiscs are accepted. A SINGLE class binds
// one policer to interface output (or, if it carries a steering filter,
// to the ingress policer-classify pipeline). MULTI-class configs are
// accepted only when EVERY class carries a steering filter (protocol or
// dscp): classify then steers each class's traffic to its own policer.
// A multi-class config with an unfiltered class is rejected because that
// class would fall back to the egress policer-output arc and stack IN
// SERIES with the others (effective rate = min(rates)), diverging
// silently from the netlink per-class shaping.
//
// Protocol and dscp filters are ACCEPTED (they steer matching traffic to
// the class policer via the classify pipeline). Mark filters and the prio
// qdisc are rejected: mark has no faithful VPP equivalent, and prio is a
// scheduler VPP does not expose. Rejecting at verify is the
// per-`rules/exact-or-reject.md` posture -- no feature ships that does not
// actually work in VPP. Rationale + evidence:
// docs/architecture/traffic/followup-vpp-traffic.md.
//
// Errors from every bad interface are collected via errors.Join so the
// operator sees all issues in one commit attempt. Interfaces are walked
// in sorted order so the error message is deterministic.
func Verify(desired map[string]traffic.InterfaceQoS) error {
	names := make([]string, 0, len(desired))
	for name := range desired {
		names = append(names, name)
	}
	sort.Strings(names)
	var errs []error
	for _, name := range names {
		iqos := desired[name]
		if err := verifyInterface(name, iqos); err != nil {
			errs = append(errs, fmt.Errorf("interface %q: %w", name, err))
		}
	}
	return errors.Join(errs...)
}

// verifyInterface checks one interface's qdisc and classes. The
// per-interface policer-name length constraint is checked here because it
// depends on both the interface name and every class name.
func verifyInterface(ifaceName string, iqos traffic.InterfaceQoS) error {
	if err := verifyQdiscType(iqos.Qdisc.Type); err != nil {
		return err
	}
	// Reject interface names containing the separator: they would produce
	// ambiguous policer names that cannot be parsed back into their parts.
	if strings.Contains(ifaceName, nameSeparator) {
		return fmt.Errorf("interface name %q must not contain %q (reserved as policer-name separator)", ifaceName, nameSeparator)
	}
	classes := iqos.Qdisc.Classes
	// Zero classes is meaningless (no rate to program) and rejected so
	// operators get a clear error rather than an empty apply.
	if len(classes) == 0 {
		return fmt.Errorf("qdisc %s under backend vpp: at least 1 class required (got 0)", iqos.Qdisc.Type)
	}
	// Multi-class requires EVERY class to carry a steering filter. A single
	// class may be unfiltered (bound to egress policer-output) -- that is the
	// interface-wide rate limit. But two unfiltered classes would both bind to
	// the egress output arc and stack IN SERIES (effective rate = min(rates)),
	// diverging silently from the netlink per-class shaping. With a steering
	// filter each class steers its own traffic to its own policer via the
	// ingress classify pipeline, so per-class shaping is faithful.
	if len(classes) > 1 {
		for _, c := range classes {
			if !classSteers(c) {
				return fmt.Errorf("qdisc %s under backend vpp: multi-class (%d classes) requires every class to carry a steering filter (protocol/dscp); class %q has none",
					iqos.Qdisc.Type, len(classes), c.Name)
			}
		}
	}
	// DefaultClass (if set) must name a configured class. Netlink honors
	// DefaultClass as a routing hint; silently ignoring a dangling reference
	// under vpp would diverge from that behavior.
	if iqos.Qdisc.DefaultClass != "" && !classNamePresent(classes, iqos.Qdisc.DefaultClass) {
		return fmt.Errorf("default-class %q does not name any configured class", iqos.Qdisc.DefaultClass)
	}
	// Reject the SAME steering filter (type+value) on two different classes.
	// Classify sessions are keyed by their match value, so two classes selecting
	// the identical protocol/dscp would collide on one session -- VPP keeps only
	// the last, silently leaving one class's policer dead. Rejecting keeps the
	// backend honest (exact-or-reject) instead of shipping a silent last-wins.
	if err := verifyNoDuplicateSteering(classes); err != nil {
		return err
	}
	for _, cls := range classes {
		if err := verifyClass(ifaceName, iqos.Qdisc.Type, cls); err != nil {
			return err
		}
	}
	return nil
}

// steerKey identifies one steering match (filter type + value) so duplicates
// across classes can be detected.
type steerKey struct {
	kind  traffic.FilterType
	value uint32
}

// verifyNoDuplicateSteering rejects the same (steering-filter type, value)
// appearing on more than one class of an interface: their classify sessions
// would collide on a single match key and VPP would keep only the last,
// silently dropping a class's policing.
func verifyNoDuplicateSteering(classes []traffic.TrafficClass) error {
	seen := make(map[steerKey]string)
	for _, cls := range classes {
		for _, f := range cls.Filters {
			if f.Type != traffic.FilterProtocol && f.Type != traffic.FilterDSCP {
				continue
			}
			k := steerKey{kind: f.Type, value: f.Value}
			if prev, dup := seen[k]; dup && prev != cls.Name {
				return fmt.Errorf("filter %s value %d is used by both class %q and class %q; a steering match may only route to one class",
					f.Type, f.Value, prev, cls.Name)
			}
			seen[k] = cls.Name
		}
	}
	return nil
}

// classNamePresent reports whether any class in the list has the given name.
func classNamePresent(classes []traffic.TrafficClass, name string) bool {
	for _, c := range classes {
		if c.Name == name {
			return true
		}
	}
	return false
}

// verifyClass checks one class's name, policer-name length, rate/ceil, and
// filters. Shared by the single- and multi-class paths.
func verifyClass(ifaceName string, qdiscType traffic.QdiscType, cls traffic.TrafficClass) error {
	if strings.Contains(cls.Name, nameSeparator) {
		return fmt.Errorf("class %q must not contain %q (reserved as policer-name separator)", cls.Name, nameSeparator)
	}
	// Reject class names that would produce a policer name longer than VPP's
	// 64-byte limit. Silent truncation there would let two distinct classes
	// collide on the same name.
	fullName := policerName(ifaceName, cls.Name)
	if len(fullName) > maxPolicerNameLen {
		return fmt.Errorf("class %q: policer name %q exceeds VPP's %d-byte limit; shorten interface or class name",
			cls.Name, fullName, maxPolicerNameLen)
	}
	// Rate must be > 0; Ceil (when set, HTB only) must be >= Rate. These belong
	// in the verifier so the operator sees the error at commit rather than
	// post-apply when policerFromClass would reject the translation.
	if cls.Rate == 0 {
		return fmt.Errorf("class %q: rate must be >= 1, got 0", cls.Name)
	}
	if qdiscType == traffic.QdiscHTB && cls.Ceil != 0 && cls.Ceil < cls.Rate {
		return fmt.Errorf("class %q: ceil (%d) must be >= rate (%d)", cls.Name, cls.Ceil, cls.Rate)
	}
	for _, f := range cls.Filters {
		if err := verifyFilter(f); err != nil {
			return fmt.Errorf("class %q: %w", cls.Name, err)
		}
	}
	return nil
}

// verifyQdiscType rejects qdisc types that have no faithful VPP translation.
// Only HTB and TBF are accepted: both map cleanly to a VPP policer. Prio is
// rejected with a dedicated actionable error (AC-4, USER decision 2026-07-10):
// it is a priority scheduler VPP does not expose, and mapping it to a DSCP
// egress-map would be a silent semantic substitution. HFSC / FQ / SFQ /
// FQ_CoDel / netem have no VPP equivalent; clsact / ingress need a different
// classify pipeline. All are rejected per exact-or-reject.
func verifyQdiscType(q traffic.QdiscType) error {
	if q == traffic.QdiscHTB || q == traffic.QdiscTBF {
		return nil
	}
	if q == traffic.QdiscPrio {
		return errQdiscPrioNotSupportedByBackend
	}
	return fmt.Errorf("qdisc %s: not supported by backend vpp", q)
}

// verifyFilter accepts protocol and dscp filters (both steer matching traffic
// to the class policer via the classify pipeline) and rejects mark filters
// under the vpp backend.
//
//   - Protocol filter: ACCEPTED. The backend builds per-family classify tables
//     whose mask/match vectors match the packet at absolute frame offsets (IPv4
//     protocol byte 23 = Ethernet 14 + IPv4 proto 9; IPv6 next-header byte 20 =
//     Ethernet 14 + IPv6 next-header 6), adds a session per protocol steering to
//     the class policer, and binds the tables to the interface's policer-classify
//     feature so only matching traffic is policed. See classify_linux.go /
//     translate.go, verified against real VPP v25.10.
//   - DSCP filter: ACCEPTED (POLICE-BY-DSCP, USER decision 2026-07-10). Same
//     pipeline as protocol, matching the DiffServ Code Point at its absolute
//     offset (IPv4 TOS byte 15 mask 0xFC; IPv6 traffic-class bytes 14/15 masks
//     0x0F/0xC0) and steering to the class policer. NOT a QoS remark (record/
//     map/mark cannot police DSCP-matched traffic -- see AC-2). Offsets
//     validated on real VPP v25.10.
//   - Mark filter: rejected. Linux SKB fwmark has no faithful VPP equivalent.
//     A-3 (validated on real VPP v25.10): the classify SET_METADATA action
//     stores an opaque value consumed only by specific downstream graph nodes,
//     not a persistent packet mark -- so no faithful semantic exists. Rationale:
//     docs/architecture/traffic/followup-vpp-traffic.md (AC-3).
//
// Rejecting the unimplemented filters at verify keeps the backend honest
// (no half-working features) per `rules/exact-or-reject.md`.
func verifyFilter(f traffic.TrafficFilter) error {
	switch f.Type {
	case traffic.FilterProtocol:
		if f.Value > maxProtocol {
			return fmt.Errorf("filter protocol value %d out of range (0-%d)", f.Value, maxProtocol)
		}
		return nil
	case traffic.FilterDSCP:
		if f.Value > maxDSCPValue {
			return fmt.Errorf("filter dscp value %d out of range (0-%d)", f.Value, maxDSCPValue)
		}
		return nil
	case traffic.FilterMark:
		return errFilterMarkNotSupportedByBackend
	}
	// Fallthrough for an enum value outside the known set. Use the
	// numeric type code directly because FilterType.String() returns
	// "unknown" for out-of-enum values and the operator's original
	// name (from YANG) has already been discarded by the parser.
	// Naming the numeric code helps the maintainer track down which
	// ze model enum value reached here without a matching case.
	return fmt.Errorf("filter type code %d: not recognized by backend vpp (traffic package added a new FilterType without updating trafficvpp.verifyFilter)", uint8(f.Type))
}
