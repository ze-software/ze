// Design: docs/architecture/core-design.md -- Policy Filter Chain, built-in next-hop steps and intent folding
// RFC: rfc/short/rfc4271.md -- NEXT_HOP default rewrite semantics (Section 5.1.3)
// RFC: rfc/short/rfc2545.md -- IPv6 global next-hop validation (Section 3)
// Related: sets_attributes.go -- Sets.NextHop, Sets.NextHopMode, Sets.NextHopIntent
// Related: chain.go -- compileChain, SameDecisions

package policychain

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Built-in next-hop chain reference names compiled directly into Chain
// (zero RPC, export-only).
const (
	BuiltinNextHopPrefix    = "next-hop"
	BuiltinNextHopAuto      = "next-hop:auto"
	BuiltinNextHopSelf      = "next-hop:self"
	BuiltinNextHopUnchanged = "next-hop:unchanged"

	nextHopKeywordAuto      = "auto"
	nextHopKeywordSelf      = "self"
	nextHopKeywordUnchanged = "unchanged"

	attachmentBGPImport = "bgp import"
	builtinNextHopType  = "next-hop"
)

// NextHopMode names the next-hop intent a policy chain resolved for a route.
// The zero value means no chain member selected a next-hop outcome.
type NextHopMode uint8

const (
	NextHopModeUnspecified NextHopMode = iota
	// NextHopModeAuto applies the RFC 4271 Section 5.1.3 default: rewrite to
	// self on ordinary eBGP, preserve on iBGP and route-server clients.
	NextHopModeAuto
	// NextHopModeSelf rewrites the next hop to this speaker's session address.
	NextHopModeSelf
	// NextHopModeUnchanged preserves the received next hop (subject to post-policy RFC gates).
	NextHopModeUnchanged
	// NextHopModeExplicit rewrites the next hop to NextHopIntent.Addr.
	NextHopModeExplicit
)

// NextHopIntent is the folded next-hop decision a policy chain resolved.
// When Mode is NextHopModeExplicit, Addr holds the validated IPv4 or global
// IPv6 address (unmapped, zone-free). AutoDefault is true only when the intent
// came from the auto-appended trailing next-hop:auto step (D-3, D-5), which
// acts as fill-if-unset and applies to forwarded routes only.
type NextHopIntent struct {
	Mode        NextHopMode
	Addr        netip.Addr
	AutoDefault bool
}

// String returns the human-readable keyword for m. It is for display only;
// never compare against it.
func (m NextHopMode) String() string {
	switch m {
	case NextHopModeUnspecified:
		return "unspecified"
	case NextHopModeAuto:
		return nextHopKeywordAuto
	case NextHopModeSelf:
		return nextHopKeywordSelf
	case NextHopModeUnchanged:
		return nextHopKeywordUnchanged
	case NextHopModeExplicit:
		return "explicit"
	}
	return "unknown"
}

// BuiltinNextHopMode parses a built-in next-hop chain reference name into its
// NextHopMode. It returns (NextHopModeUnspecified, false) for any other name.
func BuiltinNextHopMode(ref string) (NextHopMode, bool) {
	switch ref {
	case BuiltinNextHopAuto:
		return NextHopModeAuto, true
	case BuiltinNextHopSelf:
		return NextHopModeSelf, true
	case BuiltinNextHopUnchanged:
		return NextHopModeUnchanged, true
	}
	return NextHopModeUnspecified, false
}

// nextHopModeKeyword parses a FilterSet value keyword ("auto", "self",
// "unchanged") into its NextHopMode.
func nextHopModeKeyword(text string) (NextHopMode, bool) {
	switch text {
	case nextHopKeywordAuto:
		return NextHopModeAuto, true
	case nextHopKeywordSelf:
		return NextHopModeSelf, true
	case nextHopKeywordUnchanged:
		return NextHopModeUnchanged, true
	}
	return NextHopModeUnspecified, false
}

// validateExplicitNextHop verifies addr is a routable unicast IPv4 or global
// IPv6 address suitable for NEXT_HOP (RFC 4271 Section 5.1.3) or the global
// slot of MP_REACH_NLRI (RFC 2545 Section 3).
func validateExplicitNextHop(addr netip.Addr) error {
	if !addr.IsValid() {
		return errors.New("next-hop address is invalid")
	}
	if addr.Zone() != "" {
		return fmt.Errorf("next-hop %s carries an interface zone; write a zone-free IPv4 or global IPv6 address", addr)
	}
	if addr.Is4In6() {
		return fmt.Errorf("next-hop %s is an IPv4-mapped IPv6 address; write a plain IPv4 or global IPv6 address", addr)
	}
	if addr.IsUnspecified() {
		return fmt.Errorf("next-hop %s is the unspecified address; write a routable IPv4 or global IPv6 address", addr)
	}
	if addr.IsMulticast() {
		return fmt.Errorf("next-hop %s is a multicast address; write a unicast IPv4 or global IPv6 address", addr)
	}
	if err := attribute.ValidateGlobalNextHop(addr); err != nil {
		return err
	}
	if addr.Is6() && !addr.IsGlobalUnicast() {
		return fmt.Errorf("next-hop %s is not a global unicast IPv6 address (RFC 2545 Section 3); write a global IPv6 or unicast IPv4 address", addr)
	}
	return nil
}

// foldNextHopIntent folds later onto current: explicit sets use last-write-wins,
// while an auto-appended trailing next-hop:auto (later.AutoDefault) fills only
// when current is still unset (D-3).
func foldNextHopIntent(current, later NextHopIntent) NextHopIntent {
	if later.Mode == NextHopModeUnspecified {
		return current
	}
	if later.AutoDefault {
		if current.Mode != NextHopModeUnspecified {
			return current
		}
	}
	return later
}

// writeExplicitMPNextHop encodes an explicit IPv4 or global IPv6 next hop for
// fam's MP_REACH_NLRI field using the shared attribute.MPNextHopProfile
// contract and records it on acc without heap allocation.
func writeExplicitMPNextHop(acc *filterapi.ModAccumulator, fam family.Family, addr netip.Addr) {
	if !fam.NeedsNextHop() {
		return
	}
	profile := attribute.MPNextHopProfile(attribute.AFI(fam.AFI), attribute.SAFI(fam.SAFI))
	mapV4 := false
	if addr.Is4() {
		if profile.MappedIPv4 {
			if len(profile.Lengths) != 0 {
				mapV4 = !slices.Contains(profile.Lengths, profile.NextHopOctets(addr))
			}
		}
	}
	selected := profile.SelectAddress(addr, mapV4)
	size := profile.NextHopLen(selected, netip.Addr{})
	if !profile.AdmitsNextHopOctets(size) {
		return
	}
	// Maximum single-address MP next hop: 8-octet RD + 16-octet IPv6 = 24 octets.
	var wire [24]byte
	n := profile.WriteNextHops(wire[:], 0, selected, netip.Addr{})
	acc.OpCopy(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, wire[:n])
}

// isImportAttachment reports whether attachment names an import chain, where
// egress-only next-hop modes (auto, self, unchanged) are refused (D-4).
func isImportAttachment(attachment string) bool {
	if attachment == attachmentBGPImport {
		return true
	}
	return strings.HasSuffix(attachment, " import")
}

// compileBuiltinNextHop compiles a built-in next-hop:* engine step into an
// in-process zero-RPC chain member. Built-in next-hop steps are export-only.
func compileBuiltinNextHop(attachment string, m Member, mode NextHopMode) (member, error) {
	if isImportAttachment(attachment) {
		return member{}, fmt.Errorf("%s: built-in %q is valid on export chains only; remove it from import or use a policy modify filter with an explicit IPv4 or global IPv6 address", attachment, m.Name)
	}
	if err := policy.CheckDeclaredFilter(attachment, m.Name, nil, []string{policy.FieldNextHop.String()}); err != nil {
		return member{}, err
	}
	intent := NextHopIntent{Mode: mode, AutoDefault: m.AutoDefault}
	return member{
		name:           m.Name,
		filterType:     builtinNextHopType,
		onError:        rpc.OnErrorReject,
		settable:       setKindNextHop.bit(),
		builtinNextHop: intent,
	}, nil
}

// StaticNextHop answers the folded NextHopIntent and true when every member of
// c is a built-in next-hop step (no instance or external filters), so
// peerForwardFacts can keep its zero-allocation static fast path without
// running the chain per route. Returns (zero, false) when the chain is nil,
// empty, or contains any non-builtin member.
func (c *Chain) StaticNextHop() (NextHopIntent, bool) {
	if c == nil {
		return NextHopIntent{}, false
	}
	if len(c.members) == 0 {
		return NextHopIntent{}, false
	}
	var folded NextHopIntent
	for i := range c.members {
		intent := c.members[i].builtinNextHop
		if intent.Mode == NextHopModeUnspecified {
			return NextHopIntent{}, false
		}
		folded = foldNextHopIntent(folded, intent)
	}
	return folded, true
}

// HasAutoDefaultNextHop reports whether c carries an auto-appended trailing
// next-hop:auto step. After a whole-UPDATE raw override ends the chain early,
// the reactor checks this to decide whether trailing auto still applies when
// the override left next-hop bytes unchanged (D-3).
func (c *Chain) HasAutoDefaultNextHop() bool {
	if c == nil {
		return false
	}
	for i := range c.members {
		if c.members[i].builtinNextHop.AutoDefault {
			return true
		}
	}
	return false
}
