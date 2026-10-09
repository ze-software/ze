// Design: docs/architecture/api/architecture.md -- egress filter for originated routes
// Related: reactor_api_forward.go -- forwardUpdateCore runs export filters for FORWARDED routes
// Related: session_write.go -- writeUpdate / SendAnnounce call this gate for originated routes
//
// Forwarded (reflected) routes run their export filter chain in forwardUpdateCore
// before the forward pool writes them via writeRawUpdateBody / writeUpdatePreFiltered.
// The bgp-adj-rib-in replay takes that same rail. It calls RelayStoredRoute
// (reactor_api_relay.go), which reconstructs the received wire and gives it to
// forwardUpdateCore, so the replay is filtered there and never reaches this gate.
// Every OTHER outbound route -- API/plugin injection, redistribute, configured
// update{} blocks, static routes -- is written by the session via
// writeUpdate or SendAnnounce, which historically bypassed export filters entirely.
// exportFilterForBody is the single egress gate those session write paths call so a
// peer's export filter applies uniformly to ALL outbound routes, not just reflected
// ones. EORs and the already-filtered forwarded path are excluded by the callers:
// the forward pool must NOT re-enter this gate, or every export filter is applied
// twice -- once by forwardUpdateCore on the original wire and once here on the final
// EBGP-prepended wire (see writeUpdatePreFiltered in session_write.go).

package reactor

import (
	"encoding/binary"
	"log/slog"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// exportFilterForBody runs the destination peer's export filter chain on the wire
// body of an outbound (non-forwarded, non-EOR) route UPDATE, by delegating to the
// SAME chain body forwardUpdateCore uses (runEgressPolicyChainASN4). It then
// normalizes the effective ordinary IPv6 next hop, even with no active filters.
// Returns suppress=true to drop the route for this peer, or override != nil to
// write a rewritten body instead. Unchanged bodies are neither allocated nor copied.
//
// It must not re-implement the chain: this function used to "mirror"
// forwardUpdateCore and honored only Reject and raw overrides, so every
// FilterModify TEXT delta (which is what remove-private-as, as-path prepend and
// the other text filters return) was silently discarded and the route went out
// unfiltered, leaking a private ASN a configured remove-private-as policy had
// been told to strip.
//
// Called from writeUpdate/SendAnnounce while the session writeMu is held, so the
// (synchronous, in-process) filter RPC runs under that lock. This is acceptable:
// the forward pool skips export-filtered peers (forward_rs.go FastPathSkipped), so
// it never contends for this peer's writeMu; the only serialized writes are this
// peer's own keepalives/routes, which writeMu serializes regardless.
func (r *Reactor) exportFilterForBody(peer *Peer, body []byte) (suppress bool, override []byte) {
	facts := peer.forwardFacts()
	if facts == nil {
		// No established forwarding snapshot: retain the existing admission.
		return false, nil
	}
	// Inactive refs skip policy, not Section 3 normalization. Testing raw chain
	// length would wrongly fail closed for a policy the operator switched off.
	if hasActiveFilter(facts.exportFilters) {
		// An active export policy needs its engine. Preserve the same fail-closed
		// decision as policyFilterFunc and default-originate.
		if r.api == nil {
			slog.Warn("export filter: no API server -- fail-closed", "peer", facts.addrStr)
			return true, nil
		}
		// The body is already encoded in THIS peer's SEND context. Passing 0
		// loses ASN4 attributes from filter text and silently defeats matches.
		wireUpdate := wireu.NewWireUpdate(body, facts.sendCtxID)
		res := r.runEgressPolicyChainASN4(facts.exportFilters, facts.addrStr, facts.peerAS, facts.localAS, !facts.isEBGP, wireUpdate, facts.sendASN4)
		if !res.accept {
			return true, nil
		}
		if res.wireOverride != nil {
			override = res.wireOverride.Payload()
			body = override
		}
	}

	// RFC 2545 Section 3: the common-subnet condition judges the effective
	// post-policy field, including when the destination has no active policy.
	normalized, fail := r.normalizeOrdinaryNextHop(peer, facts, body)
	r.recordModifyFailureAddr(fail, modifySiteExportChain, facts.addr)
	if fail.failed() {
		return true, nil
	}
	if normalized != nil {
		// The nil-pool rebuild owns this result; no second copy is needed.
		return false, normalized
	}
	if override == nil {
		return false, nil
	}
	// A raw policy override may alias the session's writeBuf. Only copy when
	// normalization did not already rebuild it into independently owned bytes.
	out := make([]byte, len(override))
	copy(out, override)
	return false, out
}

// normalizeOrdinaryNextHop applies RFC 2545 Section 3 to a valid effective plain
// IPv6 field. Invalid roles/widths, mapped and standalone link-local forms remain
// untouched for writeUpdateGated's existing final admission. VPN and independent
// layouts are not plain fields and retain their own rules.
//
// RFC 2545 Section 3: "The link-local address shall be included in the Next Hop
// field if and only if the BGP speaker shares a common subnet with the entity
// identified by the global IPv6 address carried in the Network Address of Next
// Hop field and the peer the route is being advertised to."
// "In all other cases a BGP speaker shall advertise to its peer in the
// Network Address field only the global IPv6 address of the next hop
// (the value of the Length of Network Address of Next Hop field shall
// be set to 16)."
//
// MP_REACH value offsets (RFC 4760 Section 3; RFC 2545 Section 3):
//
//	[0:2] AFI | [2] SAFI | [3] NH length | [4:20] Global
//	          | [20:36] optional Link-Local | [4+length] Reserved
//	          | [5+length:] NLRI
//
// RFC 8950 Section 3 extends this plain 16/32-octet layout to IPv4 SAFI 1/2/4:
// "This field is to be constructed as per Section 3 of [RFC2545]."
// MPNextHopProfile marks those forms IPv6Roles as well as ExtendedIPv6; the
// rewrite preserves their IPv6 field form, so final capability admission still
// refuses a recipient that did not negotiate it.
//
// A nil result with no failure means identity, as in buildModifiedPayload. The
// caller MUST suppress on failure and MUST keep the original body on identity.
func (r *Reactor) normalizeOrdinaryNextHop(peer *Peer, facts *peerForwardFacts, body []byte) ([]byte, modifyFailure) {
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		return nil, modifyFailureNone
	}
	_, _, value, found := attribute.AttrFind(sections.Attrs(body), attribute.AttrMPReachNLRI)
	if !found || len(value) < 5 {
		return nil, modifyFailureNone
	}
	octets := int(value[3])
	// Do not let the rewrite handler drop an attribute missing its reserved
	// octet, thereby hiding a malformed field from the ordinary writer.
	if 5+octets > len(value) {
		return nil, modifyFailureNone
	}
	profile := attribute.MPNextHopProfile(attribute.AFI(binary.BigEndian.Uint16(value)), attribute.SAFI(value[2]))
	if !profile.IPv6Roles || (octets != 16 && octets != 32) {
		return nil, modifyFailureNone
	}
	field := value[4 : 4+octets]
	global, suppliedLL := nextHopAddr(field)
	// RFC 2545 Section 3; RFC 8950 Section 3; RFC 9830 Section 2.1:
	// validate both original slots before trimming could conceal an invalid one.
	if message.ValidateMPNextHop(profile, octets, global, suppliedLL) != nil {
		return nil, modifyFailureNone
	}
	if !global.Is6() || !global.IsGlobalUnicast() {
		// Mapped IPv4 and capability-77 standalone LL keep their exact forms.
		return nil, modifyFailureNone
	}
	if global == facts.addr.Unmap() || suppliedLL == facts.addr.Unmap() {
		// In particular, stripping a peer-owned LL must not hide the address
		// from the final RFC 4271 Section 5.1.3 refusal.
		return nil, modifyFailureNone
	}

	scope := peer.llScope.Load()
	var ll netip.Addr
	if octets == 32 {
		// RFC 2545 Section 3: one common subnet, not independent memberships.
		if scope.sharesNextHopSubnet(global) {
			return nil, modifyFailureNone
		}
	} else {
		owners := facts.nextHopOwners()
		// LinkLocal is restart-scoped, not hot-swappable (Peer.Settings,
		// hotSwappableSettings). A reload constructs another Peer rather than
		// mutating this address, so no p.mu lock or duplicate snapshot is needed.
		// RFC 2545 Section 3: append only this next-hop entity's own LL, and
		// only when the same common-subnet predicate permits it.
		ll = scope.linkLocalNextHop(peer.settings.LinkLocal, global, owners.classify(global))
		if !ll.IsValid() {
			return nil, modifyFailureNone
		}
	}

	// Only a real size change reaches the allocator. The existing MP_REACH
	// handler preserves AFI/SAFI, reserved octet and NLRI while recalculating
	// attribute/section lengths, including the extended-length boundary.
	var mods filterapi.ModAccumulator
	if ll.IsValid() {
		var pair [32]byte
		copy(pair[:16], field)
		address := ll.As16()
		copy(pair[16:], address[:])
		mods.OpCopy(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, pair[:])
	} else {
		mods.Op(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, field[:16])
	}
	// RFC 2545 Section 3; RFC 8654 Section 4. This is a logical ordinary body,
	// not a complete wire message: writeOrdinaryUpdateBody MUST split it after
	// final admission. Forward/raw rebuilds keep their complete-message bound.
	// Growth is one link-local address plus a possible extended length octet.
	const nextHopGrowthMax = 16 + 1
	out, _, fail := buildModifiedPayloadWithLimit(body, &mods, r.attrModHandlers, nil, nil, maxUpdateBody+nextHopGrowthMax)
	return out, fail
}
