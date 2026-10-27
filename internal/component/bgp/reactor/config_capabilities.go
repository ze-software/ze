// Design: docs/architecture/core-design.md — BGP capability parsing from config tree
// Overview: config.go — peer config parsing

package reactor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// capMode represents the negotiation mode for a non-family capability.
// Four modes: enable (advertise, no enforcement), disable (don't advertise),
// require (advertise + reject if peer lacks), refuse (don't advertise + reject if peer has).
type capMode int

const (
	capModeEnable capMode = iota
	capModeDisable
	capModeRequire
	capModeRefuse
)

// parseCapMode parses a capability mode string.
// Accepts the four modes of the capability-mode enumeration (ze-bgp-conf.yang)
// and nothing else. Any other word is an error: the schema refuses one before
// the tree reaches here, so a word this switch does not name is a builder that
// bypassed the schema, and answering enable for it would advertise a capability
// the operator never asked for (ai/rules/principles.md).
func parseCapMode(s string) (capMode, error) {
	switch strings.ToLower(s) {
	case valEnable:
		return capModeEnable, nil
	case valDisable:
		return capModeDisable, nil
	case valRequire:
		return capModeRequire, nil
	case valRefuse:
		return capModeRefuse, nil
	}
	return capModeEnable, fmt.Errorf("capability mode %q: expected one of enable, disable, require, refuse", s)
}

// capModeAdvertise reports whether the mode means the capability should be advertised.
func (m capMode) advertise() bool { return m == capModeEnable || m == capModeRequire }

// applyCapMode records require/refuse enforcement for a capability code.
func applyCapMode(mode capMode, code capability.Code, ps *PeerSettings) {
	switch mode {
	case capModeRequire:
		ps.RequiredCapabilities = append(ps.RequiredCapabilities, code)
	case capModeRefuse:
		ps.RefusedCapabilities = append(ps.RefusedCapabilities, code)
	case capModeEnable, capModeDisable:
		// No enforcement action needed.
	default:
		panic("BUG: invalid capability mode")
	}
}

// capModeLeaf reads the capability-mode leaf name from capMap. It answers
// set=false when the leaf is absent, which means the capability is not
// configured. A present value that is not one of the four mode words is an
// error naming the capability, whatever its shape: until 2026-10-11 these
// capabilities were presence containers read through flexString, which
// answered "" for a container and so dropped the capability with no error.
func capModeLeaf(capMap map[string]any, name string) (mode capMode, set bool, err error) {
	raw, present := capMap[name]
	if !present {
		return capModeDisable, false, nil
	}
	word, isWord := raw.(string)
	if !isWord {
		return capModeDisable, false, fmt.Errorf("%s: capability mode must be one of enable, disable, require, refuse, got a %T", name, raw)
	}
	mode, err = parseCapMode(word)
	if err != nil {
		return capModeDisable, false, fmt.Errorf("%s: %w", name, err)
	}
	return mode, true, nil
}

// parseCapabilitiesFromTree parses capability configuration from the tree.
// It returns an error naming the capability whose mode word is not one the
// four-mode vocabulary names.
func parseCapabilitiesFromTree(tree map[string]any, ps *PeerSettings) error {
	capMap, ok := mapMap(tree, "capability")
	if !ok {
		// ASN4 is enabled by default (RFC 6793).
		return nil
	}

	// ASN4 — enabled by default (RFC 6793), supports all four modes.
	asn4Mode := capModeEnable
	if v, ok := mapString(capMap, "asn4"); ok {
		mode, err := parseCapMode(v)
		if err != nil {
			return fmt.Errorf("asn4: %w", err)
		}
		asn4Mode = mode
	}
	ps.DisableASN4 = !asn4Mode.advertise()
	applyCapMode(asn4Mode, capability.CodeASN4, ps)

	// RFC 8654: Extended Message Support (opt-in, absent = disabled).
	extMsgMode, extMsgSet, err := capModeLeaf(capMap, "extended-message")
	if err != nil {
		return err
	}
	if extMsgSet {
		if extMsgMode.advertise() {
			ps.Capabilities = append(ps.Capabilities, &capability.ExtendedMessage{})
		}
		applyCapMode(extMsgMode, capability.CodeExtendedMessage, ps)
	}

	// RFC 2918/7313: Route Refresh (opt-in, absent = disabled).
	// Enforcement targets basic route-refresh (code 2) only.
	// Enhanced route-refresh (code 70) is a separate capability not independently configurable.
	rrMode, rrSet, err := capModeLeaf(capMap, "route-refresh")
	if err != nil {
		return err
	}
	if rrSet {
		if rrMode.advertise() {
			ps.Capabilities = append(ps.Capabilities, &capability.RouteRefresh{}, &capability.EnhancedRouteRefresh{})
		}
		applyCapMode(rrMode, capability.CodeRouteRefresh, ps)
	}

	// Graceful restart — block capability with optional mode key.
	if grMap, ok := mapMap(capMap, "graceful-restart"); ok {
		grMode := capModeEnable
		if v, ok := mapString(grMap, "mode"); ok {
			mode, err := parseCapMode(v)
			if err != nil {
				return fmt.Errorf("graceful-restart: %w", err)
			}
			grMode = mode
		}
		applyCapMode(grMode, capability.CodeGracefulRestart, ps)

		if ps.RawCapabilityConfig == nil {
			ps.RawCapabilityConfig = make(map[string]map[string]string)
		}
		ps.RawCapabilityConfig["graceful-restart"] = make(map[string]string)
		if v, ok := mapString(grMap, "restart-time"); ok {
			ps.RawCapabilityConfig["graceful-restart"]["restart-time"] = v
		}
	}

	// RFC 8950: Extended Next Hop — mode is inline on each family line.
	// e.g., "ipv4/unicast ipv6 require;" — last token is a mode if it matches a mode keyword.
	if nhMap, ok := mapMap(capMap, "nexthop"); ok {
		nhMode, err := parseExtendedNextHopFromTree(nhMap, ps)
		if err != nil {
			return fmt.Errorf("nexthop: %w", err)
		}
		applyCapMode(nhMode, capability.CodeExtendedNextHop, ps)
	}

	// ADD-PATH — global and per-family.
	if err := parseAddPathFromTree(capMap, tree, ps); err != nil {
		return fmt.Errorf("add-path: %w", err)
	}

	// Hostname — populate RawCapabilityConfig for plugin delivery.
	if hnMap, ok := mapMap(capMap, "hostname"); ok {
		if ps.RawCapabilityConfig == nil {
			ps.RawCapabilityConfig = make(map[string]map[string]string)
		}
		if ps.RawCapabilityConfig["hostname"] == nil {
			ps.RawCapabilityConfig["hostname"] = make(map[string]string)
		}
		if v, ok := mapString(hnMap, "host"); ok {
			ps.RawCapabilityConfig["hostname"]["host"] = v
		}
		if v, ok := mapString(hnMap, "domain"); ok {
			ps.RawCapabilityConfig["hostname"]["domain"] = v
		}
	}

	// Also check top-level host-name/domain-name (plugin YANG augmented fields).
	if v, ok := mapString(tree, "host-name"); ok {
		if ps.RawCapabilityConfig == nil {
			ps.RawCapabilityConfig = make(map[string]map[string]string)
		}
		if ps.RawCapabilityConfig["hostname"] == nil {
			ps.RawCapabilityConfig["hostname"] = make(map[string]string)
		}
		ps.RawCapabilityConfig["hostname"]["host"] = v
	}
	if v, ok := mapString(tree, "domain-name"); ok {
		if ps.RawCapabilityConfig == nil {
			ps.RawCapabilityConfig = make(map[string]map[string]string)
		}
		if ps.RawCapabilityConfig["hostname"] == nil {
			ps.RawCapabilityConfig["hostname"] = make(map[string]string)
		}
		ps.RawCapabilityConfig["hostname"]["domain"] = v
	}

	// Capability config JSON for plugin delivery.
	ps.CapabilityConfigJSON = mapToJSON(capMap)
	return nil
}

// extractNextHopEntry extracts nhAFI name and mode from a nexthop entry value.
// Handles both string format ("ipv6 require") and list entry map ({"nhafi": "ipv6", "mode": "require"}).
// An entry with no next-hop AFI answers an empty name; a mode word outside the
// vocabulary is an error.
func extractNextHopEntry(rawVal any) (string, capMode, error) {
	if vs, ok := rawVal.(string); ok {
		tokens := strings.Fields(vs)
		if len(tokens) == 0 {
			return "", capModeEnable, nil
		}
		if len(tokens) == 1 {
			return tokens[0], capModeEnable, nil
		}
		mode, err := parseCapMode(tokens[1])
		return tokens[0], mode, err
	}
	if m, ok := rawVal.(map[string]any); ok {
		nh, nhOK := mapString(m, "nhafi")
		if !nhOK {
			return "", capModeEnable, nil
		}
		modeStr, ok := mapString(m, "mode")
		if !ok {
			return nh, capModeEnable, nil
		}
		mode, err := parseCapMode(modeStr)
		return nh, mode, err
	}
	return "", capModeEnable, nil
}

// parseExtendedNextHopFromTree parses RFC 8950 extended next-hop families.
// Map key is the NLRI family (e.g., "ipv4/unicast"), value is "nhAFI [mode]":
//
//	"ipv4/unicast" → "ipv6"          (enable, default)
//	"ipv4/unicast" → "ipv6 require"  (require mode)
//
// Returns the most restrictive mode seen across all entries (require > refuse > enable),
// or an error naming the family whose mode word is outside the vocabulary.
func parseExtendedNextHopFromTree(nhMap map[string]any, ps *PeerSettings) (capMode, error) {
	afiMap := map[string]uint16{"ipv4": 1, "ipv6": 2}
	safiMap := map[string]uint8{
		"unicast": 1, "multicast": 2, "mpls-vpn": 128, "mpls-label": 4,
	}

	var families []capability.ExtendedNextHopFamily
	mode := capModeEnable

	for familyKey, rawVal := range nhMap {
		// Parse family key: "ipv4/unicast" → afi="ipv4", safi="unicast"
		parts := strings.SplitN(familyKey, "/", 2)
		if len(parts) != 2 {
			continue
		}
		nlriAFI, afiOK := afiMap[parts[0]]
		nlriSAFI, safiOK := safiMap[parts[1]]
		if !afiOK || !safiOK {
			continue
		}

		// Parse value: string "ipv6 [require]" or list entry map {nhafi, mode}.
		nhAFIName, entryMode, err := extractNextHopEntry(rawVal)
		if err != nil {
			return capModeEnable, fmt.Errorf("%s: %w", familyKey, err)
		}
		if nhAFIName == "" {
			continue
		}

		nhAFI, ok := afiMap[nhAFIName]
		if !ok {
			continue
		}

		// Only include family if mode advertises (enable/require).
		if entryMode.advertise() {
			families = append(families, capability.ExtendedNextHopFamily{
				NLRIAFI:    capability.AFI(nlriAFI),
				NLRISAFI:   capability.SAFI(nlriSAFI),
				NextHopAFI: capability.AFI(nhAFI),
			})
		}
		// Precedence: require > refuse > enable/disable.
		if entryMode == capModeRequire {
			mode = capModeRequire
		} else if entryMode == capModeRefuse && mode != capModeRequire {
			mode = capModeRefuse
		}
	}

	if len(families) > 0 {
		ps.Capabilities = append(ps.Capabilities, &capability.ExtendedNextHop{
			Families: families,
		})
	}
	return mode, nil
}

// parseAddPathFromTree parses ADD-PATH + PATHS-LIMIT from the unified capability add-path block.
// RFC 7911 + draft-abraitis-idr-addpath-paths-limit.
// Unified config: add-path { direction send; limit { receive 10; } family ipv4/unicast { direction send/receive; limit { receive 3; } } }.
func parseAddPathFromTree(capMap, _ map[string]any, ps *PeerSettings) error {
	apBlock, ok := mapMap(capMap, "add-path")
	if !ok {
		return nil
	}

	// Default direction and receive limit from container level.
	defaultDir := parseAddPathDirection(flexString(apBlock, "direction"))
	defaultLimit, _, err := addPathReceiveLimit(apBlock)
	if err != nil {
		return err
	}
	addPathMode := capModeEnable

	// Per-family overrides from nested family list.
	type familyEntry struct {
		fam  family.Family
		dir  capability.AddPathMode
		mode capMode
	}
	var perFamily []familyEntry
	perFamilyLimits := make(map[family.Family]uint16)

	if familyMap, ok := mapMap(apBlock, "family"); ok {
		for key, val := range familyMap {
			fam, famOK := family.LookupFamily(key)
			if !famOK {
				continue
			}

			entry := familyEntry{fam: fam, dir: defaultDir, mode: capModeEnable}

			if m, ok := val.(map[string]any); ok {
				if dirStr, ok := mapString(m, "direction"); ok {
					entry.dir = parseAddPathDirection(dirStr)
				}
				if modeStr, ok := mapString(m, "mode"); ok {
					mode, err := parseCapMode(modeStr)
					if err != nil {
						return fmt.Errorf("family %s: %w", key, err)
					}
					entry.mode = mode
					if entry.mode == capModeRequire {
						addPathMode = capModeRequire
					} else if entry.mode == capModeRefuse && addPathMode != capModeRequire {
						addPathMode = capModeRefuse
					}
				}
				limit, configured, err := addPathReceiveLimit(m)
				if err != nil {
					return fmt.Errorf("family %s: %w", key, err)
				}
				if configured {
					perFamilyLimits[fam] = limit
				}
			}

			switch entry.mode {
			case capModeRequire:
				ps.RequiredAddPathFamilies = append(ps.RequiredAddPathFamilies, fam)
			case capModeRefuse:
				ps.RefusedAddPathFamilies = append(ps.RefusedAddPathFamilies, fam)
			case capModeEnable, capModeDisable:
			default:
				panic("BUG: invalid ADD-PATH capability mode")
			}

			if entry.dir != capability.AddPathNone {
				perFamily = append(perFamily, entry)
			}
		}
	}

	hasDefault := defaultDir != capability.AddPathNone

	// Apply enforcement.
	if hasDefault || len(perFamily) > 0 {
		applyCapMode(addPathMode, capability.CodeAddPath, ps)
	}

	if !hasDefault && len(perFamily) == 0 {
		return nil
	}

	if !addPathMode.advertise() {
		return nil
	}

	addPath := &capability.AddPath{
		Families: make([]capability.AddPathFamily, 0),
	}

	// Apply default direction to all multiprotocol families.
	if hasDefault {
		overridden := make(map[family.Family]bool)
		for _, pf := range perFamily {
			overridden[pf.fam] = true
		}
		for _, cap := range ps.Capabilities {
			if mp, ok := cap.(*capability.Multiprotocol); ok {
				f := family.Family{AFI: mp.AFI, SAFI: mp.SAFI}
				if !overridden[f] {
					addPath.Families = append(addPath.Families, capability.AddPathFamily{
						AFI: mp.AFI, SAFI: mp.SAFI, Mode: defaultDir,
					})
				}
			}
		}
	}

	// Add per-family overrides.
	for _, pf := range perFamily {
		if pf.mode.advertise() {
			addPath.Families = append(addPath.Families, capability.AddPathFamily{
				AFI: pf.fam.AFI, SAFI: pf.fam.SAFI, Mode: pf.dir,
			})
		}
	}

	if len(addPath.Families) > 0 {
		ps.Capabilities = append(ps.Capabilities, addPath)
	}

	// draft-abraitis-idr-addpath-paths-limit-04 Section 3: "An AFI/SAFI tuple MUST be
	// ignored if the same tuple was not received in the ADD-PATH capability."
	// The tuples are derived from addPath.Families, the same slice that reaches the wire,
	// so the two sets cannot disagree. A family the mode filter above dropped therefore
	// contributes no tuple, and Ze never sends a limit a conformant peer must discard.
	var pathsLimitEntries []capability.PathsLimitEntry
	for _, apf := range addPath.Families {
		// Section 3: the Paths Limit field "indicates the maximum paths limit the
		// receiver wants to receive from its peer." A family whose ADD-PATH direction
		// excludes receive asks the peer for no paths, so it carries no tuple.
		if apf.Mode&capability.AddPathReceive == 0 {
			continue
		}
		f := family.Family{AFI: apf.AFI, SAFI: apf.SAFI}
		limit, hasOverride := perFamilyLimits[f]
		if !hasOverride {
			limit = defaultLimit
		}

		// Section 3: "If the received Paths Limit is zero (0), the tuple SHOULD be ignored."
		// addPathReceiveLimit refuses zero, so a zero here is an absent container
		// default rather than an operator asking for a limit of none.
		if limit > 0 {
			pathsLimitEntries = append(pathsLimitEntries, capability.PathsLimitEntry{
				AFI: apf.AFI, SAFI: apf.SAFI, Limit: limit,
			})
		}
	}

	// draft-abraitis-idr-addpath-paths-limit-04 Section 3: "The PATHS-LIMIT capability
	// MUST be ignored if the ADD-PATH capability is not present."
	// An empty addPath.Families appends no ADD-PATH capability above, and it leaves this
	// slice empty, so PATHS-LIMIT is never advertised on its own.
	if len(pathsLimitEntries) > 0 {
		ps.Capabilities = append(ps.Capabilities, &capability.PathsLimit{Entries: pathsLimitEntries})
	}
	return nil
}

// addPathReceiveLimit reads `limit { receive N; }` from an add-path container or
// family entry. configured is false when no receive value is set there. The
// YANG range 1..65535 refuses zero at load; this refuses it again, together with
// a non-numeric value and a `limit` that is not a container (the deleted scalar
// spelling), so no tree reaching the reactor can request an unbounded tuple.
func addPathReceiveLimit(block map[string]any) (limit uint16, configured bool, err error) {
	value, present := block["limit"]
	if !present {
		return 0, false, nil
	}
	limitBlock, isContainer := value.(map[string]any)
	if !isContainer {
		return 0, false, fmt.Errorf("add-path limit is a container: write limit { receive VALUE; }")
	}
	receiveValue, present := limitBlock["receive"]
	if !present {
		return 0, false, nil
	}
	receive, isString := receiveValue.(string)
	if !isString {
		return 0, false, fmt.Errorf("add-path limit receive %v: want a number from 1 through 65535", receiveValue)
	}
	count, err := strconv.ParseUint(receive, 10, 16)
	if err != nil {
		return 0, false, fmt.Errorf("add-path limit receive %q: %w", receive, err)
	}
	if count == 0 {
		return 0, false, fmt.Errorf("add-path limit receive 0: use a value from 1 through 65535")
	}
	return uint16(count), true, nil
}

// parseAddPathDirection converts a direction string to AddPathMode.
func parseAddPathDirection(dir string) capability.AddPathMode {
	// enumeration: gated by TestAddPathDirectionsMatchTheYANGModel
	switch dir {
	case "send":
		return capability.AddPathSend
	case "receive":
		return capability.AddPathReceive
	case "send/receive", "receive/send":
		return capability.AddPathBoth
	}
	return capability.AddPathNone
}
