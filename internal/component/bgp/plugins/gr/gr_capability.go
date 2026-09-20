// Design: docs/guide/graceful-restart.md — the Graceful Restart capability Ze sends
// RFC: rfc/short/rfc4724.md
// RFC: rfc/short/rfc9494.md
// Related: gr.go — plugin event loop, GR capability decode, CLI decode
// Related: gr_llgr.go — the LLGR capability (code 71) Ze sends beside this one
//
// This file builds the RFC 4724 Graceful Restart capability (code 64) that Ze
// puts in every OPEN for a peer configured with a "graceful-restart" container.
// The reactor takes the payload verbatim (Peer.getPluginCapabilities,
// internal/component/bgp/reactor/peer.go), with one edit: restartFlagsFor
// (internal/component/bgp/reactor/peer_gr_flags.go) sets the Restart State
// bit while the restart marker is live, and the per-family Forwarding State
// bit when the forwarding plane kept Ze's routes across that restart.

package gr

import (
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/family"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// grCapCode is the BGP capability code for Graceful Restart.
// RFC 4724 Section 3: "Capability code: 64".
const grCapCode = 64

// grRestartTimeDefault is the Restart Time Ze advertises when the
// "restart-time" leaf is absent. It matches the YANG default.
const grRestartTimeDefault = 120

// grRestartTimeMax is the largest value the 12-bit Restart Time field holds.
// RFC 4724 Section 3: "Restart Time in seconds (12 bits)".
const grRestartTimeMax = 4095

// grFamilyTupleLen is the wire length of one <AFI, SAFI, Flags for address
// family> tuple: 2 octets of AFI, 1 of SAFI, 1 of flags.
const grFamilyTupleLen = 4

// grFamilyMax bounds the tuple list.
// RFC 4724 Section 3: "Capability value: Consists of the "Restart Flags"
// field, "Restart Time" field, and 0 to 63 of the tuples <AFI, SAFI, Flags for
// address family>".
const grFamilyMax = 63

// grForwardStateClear is the "Flags for Address Family" octet this file
// writes: every bit zero, which is the whole octet a cold start sends.
//
// RFC 4724 Section 4.1: "Unless allowed via configuration, the "Forwarding
// State" bit for an address family in the capability can be set only if the
// forwarding state has indeed been preserved for that address family during
// the restart." parseGRCapValue runs when the configuration is loaded, which
// is before any restart is known, so it cannot make that claim HERE. It is
// made later, by the same route the Restart State bit takes: restartFlagsFor
// (internal/component/bgp/reactor/peer_gr_flags.go) sets both on the way to
// the OPEN, from the restart marker and from the forwarding plane's own
// answer. Leaving this octet clear is therefore the starting point rather
// than the final word, and a cold start never reaches that edit.
//
// RFC 4724 Section 3 covers the rest of the octet: "The remaining bits are
// reserved and MUST be set to zero by the sender and ignored by the receiver".
const grForwardStateClear = 0x00

// grFamilyNode is the presence container under "graceful-restart" that names
// the address families the capability carries, and grFamilyNameLeaf is the
// leaf-list inside it.
const (
	grFamilyNode     = "family"
	grFamilyNameLeaf = "name"
)

// parseGRCapValue extracts a GR capability hex value from a capability map's
// "graceful-restart" entry. Returns "" if no GR config is present.
//
// sessionFamilies are the address families the session carries, in the form
// collectPeerFamilies returns. Each one becomes a tuple, unless the "family"
// container narrows the set, because a capability that lists none says the
// opposite of what Ze means by it.
//
// Caller MUST run refuseUncarriedGRFamilies over the same document first. That
// walk is where a "family" container the session cannot carry is refused, and
// it is the reason this function needs no error return: every name it reads
// has already been checked against the peer's own family list.
//
// RFC 4724 Section 3: "When a sender of this capability does not include any
// <AFI, SAFI> in the capability, it means that the sender is not capable of
// preserving its forwarding state during BGP restart, but supports procedures
// for the Receiving Speaker (as defined in Section 4.2 of this document). In
// that case, the value of the "Restart Time" field advertised by the sender is
// irrelevant". Ze runs those Receiving Speaker procedures and lists its
// families as well, so the Restart Time it advertises means something unless
// the operator asks for that signal with an empty "family" container.
func parseGRCapValue(capMap map[string]any, peerAddr string, sessionFamilies []string) string {
	if capMap == nil {
		return ""
	}
	grData, ok := capMap["graceful-restart"].(map[string]any)
	if !ok {
		return ""
	}

	families := grAdvertisedFamilies(grData, sessionFamilies)

	restartTime := uint16(grRestartTimeDefault)
	if rtVal, ok := grData["restart-time"]; ok {
		switch v := rtVal.(type) {
		case float64:
			restartTime = uint16(v)
		case string:
			if parsed, err := strconv.ParseUint(v, 10, 16); err == nil {
				restartTime = uint16(parsed)
			}
		}
	}

	if restartTime > grRestartTimeMax {
		logger().Warn("restart-time exceeds 12-bit max, clamping", "peer", peerAddr, "value", restartTime)
		restartTime = grRestartTimeMax
	}

	// RFC 4724 Section 3: Restart Flags (4 bits) then Restart Time (12 bits).
	// The flags nibble stays zero here; see grForwardStateClear for the two
	// bits Ze does not set in this function.
	value := make([]byte, 0, 2+len(families)*grFamilyTupleLen)
	value = append(value, byte(restartTime>>8), byte(restartTime))

	tuples := 0
	for _, name := range families {
		if tuples == grFamilyMax {
			logger().Warn("graceful-restart family list truncated at the RFC 4724 maximum",
				"peer", peerAddr, "maximum", grFamilyMax, "configured", len(families))
			break
		}
		fam, ok := family.LookupFamily(name)
		if !ok {
			logger().Warn("graceful-restart skips an address family the family index does not know",
				"peer", peerAddr, "family", name)
			continue
		}
		// RFC 4724 Section 3: "The AFI and SAFI, taken in combination,
		// indicate that Graceful Restart is supported for routes that are
		// advertised with the same AFI and SAFI."
		value = append(value,
			byte(fam.AFI>>8), byte(fam.AFI),
			byte(fam.SAFI),
			grForwardStateClear,
		)
		tuples++
	}

	return hex.EncodeToString(value)
}

// grAdvertisedFamilies answers which address families the code-64 capability
// names for this peer.
//
// The "family" container carries presence, so it has three states and they are
// three different answers. A bare leaf-list could tell only two of them apart,
// because "the operator wrote no name" and "the operator wrote nothing" both
// arrive as an empty list.
//
//	absent            every family the session carries
//	present, empty    no family at all
//	present, filled   exactly the families named
//
// RFC 4724 Section 3: "When a sender of this capability does not include any
// <AFI, SAFI> in the capability, it means that the sender is not capable of
// preserving its forwarding state during BGP restart, but supports procedures
// for the Receiving Speaker (as defined in Section 4.2 of this document)." The
// empty container is how an operator asks for that signal, so it stays
// reachable and stays off the default.
//
// The container governs code 64 alone. extractLLGRCapabilities (gr_llgr.go)
// reads the same collectPeerFamilies answer and this function never narrows
// it, because RFC 9494 Section 4.1 says "the conventional GR phase can be
// skipped by omitting all AFIs/SAFIs from the GR Capability, advertising a
// Restart Time of zero, or both". A family in code 71 and not in code 64 is
// that documented request rather than a defect: RFC 9494 Section 4.2 deems its
// "GR Restart Time ... zero", so only the LLGR procedures run for it. Making
// one container narrow both lists would take that request away with nothing
// left to ask for it.
//
// A container that is not a map reads here as absent, and that answer is
// unreachable: the config lowering writes every container as a map
// ((*Tree).toMap, internal/component/config/tree.go), and
// refuseUncarriedGRFamilies refuses any other shape before this runs.
func grAdvertisedFamilies(grData map[string]any, sessionFamilies []string) []string {
	famData, ok := grData[grFamilyNode].(map[string]any)
	if !ok {
		return sessionFamilies
	}
	return configvalue.LeafList(famData[grFamilyNameLeaf])
}

// refuseUncarriedGRSections runs the graceful-restart family check over every
// bgp section of one delivery. It is the whole of what this plugin refuses,
// and both config-verify and Stage 2 call it so that the two answer alike
// (RunGRPlugin, gr.go).
func refuseUncarriedGRSections(sections []sdk.ConfigSection) error {
	for _, section := range sections {
		if section.Root != configRootBGP {
			continue
		}
		if err := refuseUncarriedGRFamilies(section.Data); err != nil {
			return err
		}
	}
	return nil
}

// refuseUncarriedGRFamilies returns the first "graceful-restart family" in the
// document that names an address family the peer does not carry.
//
// Caller MUST run it over a bgp config section before extractGRCapabilities
// reads the same section. It is the guard that lets parseGRCapValue narrow the
// family list with no error return of its own.
//
// The check cannot be declarative and cannot be a config validator. The YANG
// engine implements no "must", and yang.CustomValidator.ValidateFn
// (internal/component/config/validators.go) is handed one path and one value,
// so it can never see the peer's own family list beside the name under test.
// Stage 2 is the first point that holds both.
//
// RFC 4724 Section 3: "The AFI and SAFI, taken in combination, indicate that
// Graceful Restart is supported for routes that are advertised with the same
// AFI and SAFI." A session carries no route of a family it does not negotiate,
// so such a tuple promises the peer that Ze preserves routes that cannot
// exist. The operator is told rather than corrected, because either half of
// the configuration can be the one they meant.
func refuseUncarriedGRFamilies(jsonStr string) error {
	bgpSubtree, ok := configjson.ParseBGPSubtree(jsonStr)
	if !ok {
		return nil
	}

	var refusal error
	configjson.ForEachPeer(bgpSubtree, func(peerAddr string, peerMap, groupMap map[string]any, _ configjson.PeerOrigin) {
		if refusal != nil {
			return
		}
		sessionFamilies := collectPeerFamilies(peerMap, groupMap)
		if err := refusePeerGRFamilies(grCapabilityFor(peerMap, groupMap), peerAddr, sessionFamilies); err != nil {
			refusal = err
		}
	})
	return refusal
}

// grCapabilityFor answers which "graceful-restart" container governs one
// peer: its own when it writes one, and its group's otherwise.
//
// A container on the peer REPLACES the container on the group rather than
// adding to it (ze-graceful-restart.yang), so exactly one of the two decides
// what the peer advertises. This function is the only statement of that
// precedence, and both the guard and the builder take it from here. They read
// it separately until 2026-09-20, and the guard read BOTH containers: a group
// list a peer overrode was refused on that peer's behalf although no OPEN
// would ever carry it, and FatalOnConfigError made the refusal stop ze.
func grCapabilityFor(peerMap, groupMap map[string]any) map[string]any {
	if capMap := configjson.GetCapability(peerMap); capMap["graceful-restart"] != nil {
		return capMap
	}
	if groupMap == nil {
		return nil
	}
	return configjson.GetCapability(groupMap)
}

// refusePeerGRFamilies checks the "graceful-restart family" container of one
// capability map against the families the session carries.
func refusePeerGRFamilies(capMap map[string]any, peerAddr string, sessionFamilies []string) error {
	grData, ok := capMap["graceful-restart"].(map[string]any)
	if !ok {
		return nil
	}
	node, ok := grData[grFamilyNode]
	if !ok {
		return nil
	}
	famData, ok := node.(map[string]any)
	if !ok {
		return fmt.Errorf("peer %s: graceful-restart family arrived as %T, and the config lowering writes a container as a map",
			peerAddr, node)
	}

	for _, name := range configvalue.LeafList(famData[grFamilyNameLeaf]) {
		if slices.Contains(sessionFamilies, name) {
			continue
		}
		return fmt.Errorf(
			"peer %s: graceful-restart family names %q, and the peer carries %s: "+
				"add %q to the peer's family list, or remove it from graceful-restart family",
			peerAddr, name, strings.Join(sessionFamilies, ", "), name)
	}
	return nil
}

// extractGRCapabilities parses bgp config JSON and returns per-peer GR capabilities.
// Handles both standalone peers (bgp.peer) and grouped peers (bgp.group.<name>.peer).
//
// Caller MUST run refuseUncarriedGRFamilies over the same section first, so
// that every "graceful-restart family" name this walk reads is one the peer
// carries.
func extractGRCapabilities(jsonStr string) []sdk.CapabilityDecl {
	bgpSubtree, ok := configjson.ParseBGPSubtree(jsonStr)
	if !ok {
		logger().Warn("invalid JSON in bgp config")
		return nil
	}

	var caps []sdk.CapabilityDecl

	configjson.ForEachPeer(bgpSubtree, func(peerAddr string, peerMap, groupMap map[string]any, origin configjson.PeerOrigin) {
		// One family list serves both capabilities of this peer.
		// RFC 9494 Section 4.2: "If the Graceful Restart Capability that was
		// received does not list all AFIs/SAFIs supported by the session, then
		// the GR Restart Time shall be deemed zero for those AFIs/SAFIs that
		// are not listed." A family Ze lists in the code-71 capability and
		// omits here would carry a Restart Time of zero on arrival, so the two
		// lists come from one reading of the configuration.
		families := collectPeerFamilies(peerMap, groupMap)

		// The peer's own container, or its group's. grCapabilityFor states
		// that precedence once, and refuseUncarriedGRFamilies reads the same
		// answer, so the guard can never refuse a container this walk would
		// not have advertised.
		capValue := parseGRCapValue(grCapabilityFor(peerMap, groupMap), peerAddr, families)
		if capValue == "" {
			return
		}

		caps = append(caps, sdk.CapabilityDecl{
			Code:     grCapCode,
			Encoding: sdk.CapEncodingHex,
			Payload:  capValue,
			Peers:    []string{configjson.CapabilitySelector(peerAddr, origin)},
		})
		logger().Debug("gr capability", "peer", peerAddr)
	})

	return caps
}
