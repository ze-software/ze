// Design: plan/learned/627-fw-7-traffic-vpp.md -- Commit-time rejection matrix

package trafficvpp

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
)

var (
	errFilterDscpNotSupportedByBackend = errors.New("filter dscp: not supported by backend vpp (deferred: VPP QoS record+mark pipeline not yet implemented)")
	errFilterMarkNotSupportedByBackend = errors.New("filter mark: not supported by backend vpp (VPP classifier matches packet-header bytes, not Linux SKB metadata)")
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
// Current scope: HTB and TBF qdiscs are accepted with EXACTLY ONE class
// each (a single policer bound to interface output). Multi-class
// configurations are rejected because without filter support the
// backend would stack every class's policer on VPP's output feature
// arc IN SERIES, producing an effective rate of min(class_rates)
// rather than the per-class shaping the operator asked for. That
// would diverge silently from the netlink backend's behavior for the
// same YANG.
//
// ALL filter types are rejected: DSCP and protocol filters would
// require completing the VPP QoS-record/classify-set-interface
// pipeline that fw-7 does not build. Rejecting at verify is the
// per-`rules/exact-or-reject.md` posture -- no feature ships that
// does not actually work in VPP. See plan/deferrals.md for the
// destination specs that will reintroduce filter and multi-class
// support end-to-end.
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
	// Reject multi-class qdiscs. Without filter support (deferred, see
	// plan/deferrals.md), a multi-class configuration has no way to
	// steer traffic to individual classes. The backend binds every
	// class's policer to VPP's output feature arc, so N policers run
	// IN SERIES on every packet -- effective rate becomes min(rates),
	// not the per-class shaping the operator asked for. Reject the
	// multi-class case until filters bring meaningful classification.
	// Zero classes is also meaningless (no rate to program) and
	// rejected so operators get a clear error rather than an empty
	// apply that programs nothing.
	if len(iqos.Qdisc.Classes) != 1 {
		return fmt.Errorf("qdisc %s under backend vpp: exactly 1 class required (got %d); multi-class shaping needs filters that are deferred (see plan/deferrals.md)",
			iqos.Qdisc.Type, len(iqos.Qdisc.Classes))
	}
	cls := iqos.Qdisc.Classes[0]
	// DefaultClass (if set) must name the single configured class.
	// Netlink honors DefaultClass as a routing hint; silently ignoring
	// a dangling reference under vpp would diverge from that behavior.
	if iqos.Qdisc.DefaultClass != "" && iqos.Qdisc.DefaultClass != cls.Name {
		return fmt.Errorf("default-class %q does not name the configured class %q", iqos.Qdisc.DefaultClass, cls.Name)
	}
	if strings.Contains(cls.Name, nameSeparator) {
		return fmt.Errorf("class %q must not contain %q (reserved as policer-name separator)", cls.Name, nameSeparator)
	}
	// Reject class names that would produce a policer name longer
	// than VPP's 64-byte limit. Silent truncation there would let
	// two distinct classes collide on the same name.
	fullName := "ze/" + ifaceName + "/" + cls.Name
	if len(fullName) > maxPolicerNameLen {
		return fmt.Errorf("class %q: policer name %q exceeds VPP's %d-byte limit; shorten interface or class name",
			cls.Name, fullName, maxPolicerNameLen)
	}
	// Rate must be > 0; Ceil (when set, HTB only) must be >= Rate.
	// These belong in the verifier so the operator sees the error at
	// commit rather than post-apply when policerFromClass would reject
	// the translation.
	//
	// traffic.ValidateRate/ValidateCeil return errors prefixed with
	// "traffic:". The wrapper chain already runs through the
	// traffic-vpp subsystem, so that prefix would appear twice. Rephrase
	// locally instead of wrapping the model's error verbatim.
	if cls.Rate == 0 {
		return fmt.Errorf("class %q: rate must be >= 1, got 0", cls.Name)
	}
	if iqos.Qdisc.Type == traffic.QdiscHTB && cls.Ceil != 0 && cls.Ceil < cls.Rate {
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
// Only HTB and TBF are accepted: both map cleanly to a VPP policer. Prio
// is rejected because the class-index to DSCP-value mapping has no
// operator-facing semantics (deferred). HFSC / FQ / SFQ / FQ_CoDel / netem
// have no VPP equivalent. clsact / ingress need a different classify
// pipeline and are deferred.
func verifyQdiscType(q traffic.QdiscType) error {
	if q == traffic.QdiscHTB || q == traffic.QdiscTBF {
		return nil
	}
	return fmt.Errorf("qdisc %s: not supported by backend vpp", q)
}

// verifyFilter accepts protocol filters and rejects DSCP and mark filters
// under the vpp backend.
//
//   - Protocol filter: ACCEPTED. The backend builds per-family classify
//     tables whose mask/match vectors match the packet at absolute frame
//     offsets (IPv4 protocol byte 23 = Ethernet 14 + IPv4 proto 9; IPv6
//     next-header byte 20 = Ethernet 14 + IPv6 next-header 6), adds a session
//     per protocol steering to the class policer, and binds the tables to the
//     interface's policer-classify feature so only matching traffic is
//     policed. See classify_linux.go / translate.go. The first fw-7 attempt
//     matched at the wrong offset and never attached the table; both failure
//     modes are pinned by golden vectors and the attach wiring test, all
//     verified against real VPP v25.10.
//   - DSCP filter: rejected. `QosEgressMapUpdate` + `QosMarkEnableDisable`
//     need a preceding `QosRecordEnableDisable` on ingress to capture the
//     incoming DSCP; that 3-step pipeline is not built yet.
//   - Mark filter: rejected. VPP's classifier matches packet-header bytes;
//     Linux SKB mark has no equivalent header field to match on.
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
		return errFilterDscpNotSupportedByBackend
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
