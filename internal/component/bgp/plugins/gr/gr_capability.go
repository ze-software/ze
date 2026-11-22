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
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/family"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// grConfigName is the name the Graceful Restart capability carries in Ze:
// the YANG container under a peer's `session capability`, and the Name of
// the decoded capability.
const grConfigName = "graceful-restart"

// grCapCode is the BGP capability code for Graceful Restart.
// RFC 4724 Section 3: "Capability code: 64".
const grCapCode = 64

// grCapabilities computes gr's declaration from the bgp root. runGRPlugin
// registers it; the tests drive it through the SDK's CandidateCapabilities.
var grCapabilities = configjson.BGPCapabilityExtractor(func(data string) ([]sdk.CapabilityDecl, error) {
	// RFC 9494: LLGR capability (code 71) declared alongside GR (code 64)
	return append(extractGRCapabilities(data), extractLLGRCapabilities(data)...), nil
})

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

// grShortLivedNode and grLongLivedNode are the two symmetric phase containers
// under "graceful-restart": short-lived governs code 64 (RFC 4724) and
// long-lived governs code 71 (RFC 9494). Both hold a keyed "family" list.
const (
	grShortLivedNode  = "short-lived"
	grLongLivedNode   = "long-lived"
	grFamilyNode      = "family"
	grRestartTimeLeaf = "restart-time"
	grStaleTimeLeaf   = "stale-time"
)

// parseGRCapValue extracts a GR capability hex value from a capability map's
// "graceful-restart" entry. Returns "" if no GR config is present.
//
// sessionFamilies are the address families the session carries, in the form
// collectPeerFamilies returns. When the "short-lived" container is absent,
// code 64 carries Restart Time 0 and zero family tuples, which RFC 4724
// Section 3 reads as a Receiving Speaker preserving nothing of its own and
// RFC 9494 Section 4.1 uses to skip the conventional GR phase when long-lived
// is configured. When "short-lived" is present, every session family becomes a
// tuple unless its "family" list narrows the set. A family the RIB plugin
// cannot store is left out with a warning, and when none is left the
// capability is still sent with no tuple, so Receiving Speaker procedures and
// End-of-RIB still apply.
//
// Caller MUST run refuseUncarriedGRFamilies over the same document first. That
// walk is where an uncarried family under short-lived or long-lived is refused,
// and it is the reason this function needs no error return.
//
// RFC 4724 Section 3: "When a sender of this capability does not include any
// <AFI, SAFI> in the capability, it means that the sender is not capable of
// preserving its forwarding state during BGP restart, but supports procedures
// for the Receiving Speaker (as defined in Section 4.2 of this document). In
// that case, the value of the "Restart Time" field advertised by the sender is
// irrelevant".
func parseGRCapValue(capMap map[string]any, peerAddr string, sessionFamilies []string) string {
	if capMap == nil {
		return ""
	}
	grData, ok := capMap[grConfigName].(map[string]any)
	if !ok {
		return ""
	}

	shortData, hasShort := grData[grShortLivedNode].(map[string]any)
	if !hasShort {
		// RFC 4724 Section 3: "When a sender of this capability does not
		// include any <AFI, SAFI> in the capability, it means that the sender
		// is not capable of preserving its forwarding state during BGP
		// restart, but supports procedures for the Receiving Speaker (as
		// defined in Section 4.2 of this document)."
		// RFC 9494 Section 4.1: "if support for conventional Graceful Restart
		// is not desired for the session, the conventional GR phase can be
		// skipped by omitting all AFIs/SAFIs from the GR Capability,
		// advertising a Restart Time of zero, or both."
		return "0000"
	}

	families := grAdvertisedFamilies(shortData, sessionFamilies)

	restartTime := uint16(grRestartTimeDefault)
	if rtVal, exists := shortData[grRestartTimeLeaf]; exists {
		if parsed, ok := configvalue.Int(rtVal); ok && parsed >= 0 {
			if parsed > grRestartTimeMax {
				logger().Warn("graceful-restart short-lived restart-time exceeds 12-bit max, clamping",
					"code", codeDoctorBGPGRCapability, "peer", peerAddr, "value", parsed)
				restartTime = grRestartTimeMax
			} else {
				restartTime = uint16(parsed)
			}
		}
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
				"code", codeDoctorBGPGRCapability, "peer", peerAddr, "maximum", grFamilyMax, "configured", len(families))
			break
		}
		fam, ok := family.LookupFamily(name)
		if !ok {
			logger().Warn("graceful-restart skips an address family the family index does not know",
				"code", codeDoctorBGPGRCapability, "peer", peerAddr, "family", name)
			continue
		}
		// RFC 4724 Section 4: "A BGP speaker MAY advertise the Graceful
		// Restart Capability for an address family to its peer if it has the
		// ability to preserve its forwarding state for the address family when
		// BGP restarts." What Ze keeps across a restart is the RIB plugin's
		// copy of the routes, which it sends again. bgp-rib is a Dependency of
		// this plugin (register.go), so it is loaded wherever this runs, and it
		// stores a family only through that family's NLRI splitter (rib.go,
		// insertPoolNLRIs). Every compiled-in family has one; a family an
		// external plugin registers at runtime may not, and is left out.
		if !nlrisplit.Supported(fam) {
			logger().Warn("graceful-restart omits an address family the RIB cannot store",
				"code", codeDoctorBGPGRCapability, "peer", peerAddr, "family", name)
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
// names when the short-lived container is present. When its "family" list is
// absent, every family the session carries is advertised; when present, only
// the listed keys (sorted by configvalue.ListEntries) are advertised.
//
// This governs code 64 alone. parseLLGRCapValue (gr_llgr.go) reads the sibling
// "long-lived" container instead, so code 64 and code 71 family memberships
// stay independent (RFC 9494 Section 4.1, Section 4.2).
func grAdvertisedFamilies(shortData map[string]any, sessionFamilies []string) []string {
	raw, exists := shortData[grFamilyNode]
	if !exists {
		return sessionFamilies
	}
	entries := configvalue.ListEntries(raw)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Key)
	}
	return names
}

// refuseUncarriedGRSections runs the graceful-restart family check over every
// bgp section of one delivery. It is the whole of what this plugin refuses,
// and both config-verify and Stage 2 call it so that the two answer alike
// (runGRPlugin, gr.go).
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
	if capMap := configjson.GetCapability(peerMap); capMap[grConfigName] != nil {
		return capMap
	}
	if groupMap == nil {
		return nil
	}
	return configjson.GetCapability(groupMap)
}

// refusePeerGRFamilies checks both the "short-lived family" list and the
// "long-lived family" list of one capability map against the families the
// session carries, and checks each long-lived entry's stale-time.
func refusePeerGRFamilies(capMap map[string]any, peerAddr string, sessionFamilies []string) error {
	grData, ok := capMap[grConfigName].(map[string]any)
	if !ok {
		return nil
	}
	if err := refusePeerPhaseFamilies(grData, grShortLivedNode, peerAddr, sessionFamilies, false); err != nil {
		return err
	}
	return refusePeerPhaseFamilies(grData, grLongLivedNode, peerAddr, sessionFamilies, true)
}

// refusePeerPhaseFamilies checks one phase container ("short-lived" or
// "long-lived") under graceful-restart: every listed family must be carried by
// the session, and when requireStaleTime is true (long-lived), each entry must
// carry a valid stale-time in 0..16777215.
func refusePeerPhaseFamilies(grData map[string]any, phase, peerAddr string, sessionFamilies []string, requireStaleTime bool) error {
	phaseRaw, exists := grData[phase]
	// refuse-by-default: guards refusePeerGRFamilies -- absent phase container means the operator left that phase unconfigured
	if !exists {
		return nil
	}
	phaseData, ok := phaseRaw.(map[string]any)
	if !ok {
		return fmt.Errorf("peer %s: graceful-restart %s arrived as %T, and the config lowering writes a container as a map",
			peerAddr, phase, phaseRaw)
	}
	famRaw, exists := phaseData[grFamilyNode]
	// refuse-by-default: guards refusePeerGRFamilies -- absent family list means default (short-lived) or off (long-lived)
	if !exists {
		return nil
	}
	if _, isMap := famRaw.(map[string]any); !isMap {
		return fmt.Errorf("peer %s: graceful-restart %s family arrived as %T, and the config lowering writes a list as a map",
			peerAddr, phase, famRaw)
	}
	for _, entry := range configvalue.ListEntries(famRaw) {
		if !slices.Contains(sessionFamilies, entry.Key) {
			return fmt.Errorf(
				"peer %s: graceful-restart %s family names %q, and the peer carries %s: "+
					"add %q to the peer's family list, or remove it from graceful-restart %s family",
				peerAddr, phase, entry.Key, strings.Join(sessionFamilies, ", "), entry.Key, phase)
		}
		if !requireStaleTime {
			continue
		}
		llst, valid := configvalue.Int(entry.Fields[grStaleTimeLeaf])
		if !valid {
			return fmt.Errorf(
				"peer %s: graceful-restart %s family %q requires stale-time in 0..%d: "+
					"set stale-time to a value in 0..%d seconds",
				peerAddr, phase, entry.Key, llgrStaleTimeMax, llgrStaleTimeMax)
		}
		if llst < 0 {
			return fmt.Errorf(
				"peer %s: graceful-restart %s family %q stale-time %d is negative: "+
					"set stale-time to a value in 0..%d seconds",
				peerAddr, phase, entry.Key, llst, llgrStaleTimeMax)
		}
		if llst > llgrStaleTimeMax {
			return fmt.Errorf(
				"peer %s: graceful-restart %s family %q stale-time %d exceeds 24-bit max %d: "+
					"set stale-time to a value in 0..%d seconds",
				peerAddr, phase, entry.Key, llst, llgrStaleTimeMax, llgrStaleTimeMax)
		}
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
	// refuse-by-default: guards grCapabilities -- a config section without a bgp subtree declares no GR capabilities
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
