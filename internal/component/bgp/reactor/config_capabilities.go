// Design: docs/architecture/core-design.md — BGP capability parsing from config tree
// Overview: config.go — peer config parsing

package reactor

import (
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
// Accepts enable/disable/require/refuse plus backwards-compat true/false.
// Empty string or unrecognized values default to enable (lenient parsing).
func parseCapMode(s string) capMode {
	switch strings.ToLower(s) {
	case "", valTrue, valEnable:
		return capModeEnable
	case valFalse, valDisable:
		return capModeDisable
	case valRequire:
		return capModeRequire
	case valRefuse:
		return capModeRefuse
	}
	return capModeEnable
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
	}
}

// parseCapabilitiesFromTree parses capability configuration from the tree.
func parseCapabilitiesFromTree(tree map[string]any, ps *PeerSettings) {
	capMap, ok := mapMap(tree, "capability")
	if !ok {
		// ASN4 is enabled by default (RFC 6793).
		return
	}

	// ASN4 — enabled by default (RFC 6793), supports all four modes.
	asn4Mode := capModeEnable
	if v, ok := mapString(capMap, "asn4"); ok {
		asn4Mode = parseCapMode(v)
	}
	ps.DisableASN4 = !asn4Mode.advertise()
	applyCapMode(asn4Mode, capability.CodeASN4, ps)

	// RFC 8654: Extended Message Support (opt-in, absent = disabled).
	if v := flexString(capMap, "extended-message"); v != "" {
		extMsgMode := parseCapMode(v)
		if extMsgMode.advertise() {
			ps.Capabilities = append(ps.Capabilities, &capability.ExtendedMessage{})
		}
		applyCapMode(extMsgMode, capability.CodeExtendedMessage, ps)
	}

	// RFC 2918/7313: Route Refresh (opt-in, absent = disabled).
	// Enforcement targets basic route-refresh (code 2) only.
	// Enhanced route-refresh (code 70) is a separate capability not independently configurable.
	if v := flexString(capMap, "route-refresh"); v != "" {
		rrMode := parseCapMode(v)
		if rrMode.advertise() {
			ps.Capabilities = append(ps.Capabilities, &capability.RouteRefresh{}, &capability.EnhancedRouteRefresh{})
		}
		applyCapMode(rrMode, capability.CodeRouteRefresh, ps)
	}

	// Graceful restart — block capability with optional mode key.
	if grMap, ok := mapMap(capMap, "graceful-restart"); ok {
		grMode := capModeEnable
		if v, ok := mapString(grMap, "mode"); ok {
			grMode = parseCapMode(v)
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
		nhMode := parseExtendedNextHopFromTree(nhMap, ps)
		applyCapMode(nhMode, capability.CodeExtendedNextHop, ps)
	}

	// ADD-PATH — global and per-family.
	parseAddPathFromTree(capMap, tree, ps)

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
}

// extractNextHopEntry extracts nhAFI name and mode from a nexthop entry value.
// Handles both string format ("ipv6 require") and list entry map ({"nhafi": "ipv6", "mode": "require"}).
func extractNextHopEntry(rawVal any) (string, capMode) {
	if vs, ok := rawVal.(string); ok {
		tokens := strings.Fields(vs)
		if len(tokens) == 0 {
			return "", capModeEnable
		}
		mode := capModeEnable
		if len(tokens) > 1 {
			mode = parseCapMode(tokens[1])
		}
		return tokens[0], mode
	}
	if m, ok := rawVal.(map[string]any); ok {
		nh, nhOK := mapString(m, "nhafi")
		if !nhOK {
			return "", capModeEnable
		}
		mode := capModeEnable
		if modeStr, ok := mapString(m, "mode"); ok {
			mode = parseCapMode(modeStr)
		}
		return nh, mode
	}
	return "", capModeEnable
}

// parseExtendedNextHopFromTree parses RFC 8950 extended next-hop families.
// Map key is the NLRI family (e.g., "ipv4/unicast"), value is "nhAFI [mode]":
//
//	"ipv4/unicast" → "ipv6"          (enable, default)
//	"ipv4/unicast" → "ipv6 require"  (require mode)
//
// Returns the most restrictive mode seen across all entries (require > refuse > enable).
func parseExtendedNextHopFromTree(nhMap map[string]any, ps *PeerSettings) capMode {
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
		nhAFIName, entryMode := extractNextHopEntry(rawVal)
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
	return mode
}

// parseAddPathFromTree parses ADD-PATH + PATHS-LIMIT from the unified capability add-path block.
// RFC 7911 + draft-abraitis-idr-addpath-paths-limit.
// Unified config: add-path { direction send; family ipv4/unicast { direction send/receive; limit 10; } }.
func parseAddPathFromTree(capMap, _ map[string]any, ps *PeerSettings) {
	apBlock, ok := mapMap(capMap, "add-path")
	if !ok {
		return
	}

	// Default direction and limit from container level.
	defaultDir := parseAddPathDirection(flexString(apBlock, "direction"))
	var defaultLimit uint16
	if limitStr := flexString(apBlock, "limit"); limitStr != "" {
		defaultLimit = parseUint16(limitStr)
	}
	addPathMode := capModeEnable

	// Per-family overrides from nested family list.
	type familyEntry struct {
		fam  family.Family
		dir  capability.AddPathMode
		mode capMode
	}
	var perFamily []familyEntry
	var pathsLimitEntries []capability.PathsLimitEntry
	perFamilyHasLimit := make(map[family.Family]bool)

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
					entry.mode = parseCapMode(modeStr)
					if entry.mode == capModeRequire {
						addPathMode = capModeRequire
					} else if entry.mode == capModeRefuse && addPathMode != capModeRequire {
						addPathMode = capModeRefuse
					}
				}
				if limitStr, ok := mapString(m, "limit"); ok {
					if limit := parseUint16(limitStr); limit > 0 {
						pathsLimitEntries = append(pathsLimitEntries, capability.PathsLimitEntry{
							AFI:   fam.AFI,
							SAFI:  fam.SAFI,
							Limit: limit,
						})
						perFamilyHasLimit[fam] = true
					}
				}
			}

			switch entry.mode {
			case capModeRequire:
				ps.RequiredAddPathFamilies = append(ps.RequiredAddPathFamilies, fam)
			case capModeRefuse:
				ps.RefusedAddPathFamilies = append(ps.RefusedAddPathFamilies, fam)
			case capModeEnable, capModeDisable:
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
		return
	}

	if !addPathMode.advertise() {
		return
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

	// Apply default limit to AddPath families without a per-family limit.
	if defaultLimit > 0 {
		for _, apf := range addPath.Families {
			f := family.Family{AFI: apf.AFI, SAFI: apf.SAFI}
			if !perFamilyHasLimit[f] {
				pathsLimitEntries = append(pathsLimitEntries, capability.PathsLimitEntry{
					AFI: apf.AFI, SAFI: apf.SAFI, Limit: defaultLimit,
				})
			}
		}
	}

	// Emit PATHS-LIMIT capability if any family has a limit.
	if len(pathsLimitEntries) > 0 {
		ps.Capabilities = append(ps.Capabilities, &capability.PathsLimit{Entries: pathsLimitEntries})
	}
}

// parseAddPathDirection converts a direction string to AddPathMode.
func parseAddPathDirection(dir string) capability.AddPathMode {
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

// parseUint16 parses a string to uint16, returning 0 on failure.
func parseUint16(s string) uint16 {
	n, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return 0
	}
	return uint16(n)
}
