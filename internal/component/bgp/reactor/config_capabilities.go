// Design: docs/architecture/core-design.md — BGP capability parsing from config tree
// Overview: config.go — peer config parsing

package reactor

import (
	"slices"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
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

// extractAddPathEntry extracts family, direction, and mode from an add-path entry.
// Handles both old format (key="ipv4/unicast send require" or key="ipv4/unicast", val="send require")
// and new list format (key="ipv4/unicast", val=map{"direction":"send","mode":"require"}).
func extractAddPathEntry(key string, val any) (familyKey, direction, mode string) {
	// New list entry format: val is map[string]any.
	if m, ok := val.(map[string]any); ok {
		dir, _ := mapString(m, "direction")
		md, _ := mapString(m, "mode")
		return key, dir, md
	}

	// Old format: key may contain space-separated tokens, or val is a string.
	parts := strings.Fields(key)
	if len(parts) < 2 {
		if vs, ok := val.(string); ok {
			parts = append(parts, strings.Fields(vs)...)
		}
	}
	// Check for trailing mode token.
	if len(parts) >= 3 && isCapModeToken(parts[len(parts)-1]) {
		return parts[0], parts[1], parts[len(parts)-1]
	}
	if len(parts) >= 2 {
		return parts[0], parts[1], ""
	}
	return key, "", ""
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

// capModeTokens is the single source of truth for capability mode keywords.
var capModeTokens = []string{valRequire, valRefuse, valEnable, valDisable}

// isCapModeToken reports whether s is a capability mode keyword.
func isCapModeToken(s string) bool {
	return slices.Contains(capModeTokens, strings.ToLower(s))
}

// parseAddPathFromTree parses ADD-PATH capability from the capability map and peer tree.
// RFC 7911: Supports both global add-path mode and per-family overrides.
// An optional trailing mode token (require/refuse/enable/disable) sets enforcement.
func parseAddPathFromTree(capMap, peerTree map[string]any, ps *PeerSettings) {
	var globalSend, globalReceive bool
	addPathMode := capModeEnable

	// Check capability block for add-path (flex: string or map).
	if val, ok := capMap["add-path"]; ok {
		switch v := val.(type) {
		case string:
			// Global mode: "send/receive [require]" — last token may be a mode.
			parts := strings.Fields(v)
			if len(parts) >= 2 && isCapModeToken(parts[len(parts)-1]) {
				addPathMode = parseCapMode(parts[len(parts)-1])
				parts = parts[:len(parts)-1]
			}
			dir := strings.Join(parts, "/")
			switch dir {
			case "send/receive", "receive/send":
				globalSend = true
				globalReceive = true
			case "send":
				globalSend = true
			case "receive":
				globalReceive = true
			}
		case map[string]any:
			// Block mode: add-path { send true; receive true; }
			if s, ok := mapString(v, "send"); ok {
				globalSend = s == valTrue
			}
			if r, ok := mapString(v, "receive"); ok {
				globalReceive = r == valTrue
			}
		}
	}

	// Per-family add-path from peer tree.
	var perFamily []capability.AddPathFamily
	perFamilyHasEnforcement := false // true when any per-family entry sets require/refuse
	if addPathMap, ok := mapMap(peerTree, "add-path"); ok {
		for key, val := range addPathMap {
			familyKey, dirStr, modeStr := extractAddPathEntry(key, val)

			// Apply enforcement mode.
			if modeStr != "" {
				entryMode := parseCapMode(modeStr)
				if entryMode == capModeRequire {
					addPathMode = capModeRequire
					perFamilyHasEnforcement = true
				} else if entryMode == capModeRefuse && addPathMode != capModeRequire {
					addPathMode = capModeRefuse
					perFamilyHasEnforcement = true
				}
			}

			if familyKey == "" || dirStr == "" {
				continue
			}
			family, ok := nlri.ParseFamily(familyKey)
			if !ok {
				continue
			}
			var mode capability.AddPathMode
			switch dirStr {
			case "send":
				mode = capability.AddPathSend
			case "receive":
				mode = capability.AddPathReceive
			case "send/receive", "receive/send":
				mode = capability.AddPathBoth
			}
			if mode != capability.AddPathNone {
				perFamily = append(perFamily, capability.AddPathFamily{
					AFI:  family.AFI,
					SAFI: family.SAFI,
					Mode: mode,
				})
			}
		}
	}

	// Apply add-path enforcement mode.
	if globalSend || globalReceive || perFamilyHasEnforcement {
		applyCapMode(addPathMode, capability.CodeAddPath, ps)
	}

	if !globalSend && !globalReceive && len(perFamily) == 0 {
		return
	}

	// Don't advertise the capability if mode suppresses it (disable/refuse).
	if !addPathMode.advertise() {
		return
	}

	addPath := &capability.AddPath{
		Families: make([]capability.AddPathFamily, 0),
	}

	// Apply global mode to all configured families.
	if globalSend || globalReceive {
		var globalMode capability.AddPathMode
		switch {
		case globalSend && globalReceive:
			globalMode = capability.AddPathBoth
		case globalSend:
			globalMode = capability.AddPathSend
		case globalReceive:
			globalMode = capability.AddPathReceive
		}
		for _, cap := range ps.Capabilities {
			if mp, ok := cap.(*capability.Multiprotocol); ok {
				addPath.Families = append(addPath.Families, capability.AddPathFamily{
					AFI:  mp.AFI,
					SAFI: mp.SAFI,
					Mode: globalMode,
				})
			}
		}
	}

	// Override with per-family settings.
	addPath.Families = append(addPath.Families, perFamily...)

	if len(addPath.Families) > 0 {
		ps.Capabilities = append(ps.Capabilities, addPath)
	}
}
