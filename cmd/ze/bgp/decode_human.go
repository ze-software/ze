// Design: docs/architecture/core-design.md — BGP CLI commands
// Related: decode.go — top-level decode dispatch calls human formatters
// Related: decode_mp.go — decodeNLRIOnly calls formatNLRIHuman

package bgp

import (
	"fmt"
	"strings"
)

// =============================================================================
// Human-Readable Formatters
// =============================================================================

// formatOpenHuman formats OPEN message data as human-readable text.
// Works with Ze format: {"open": {...}}.
func formatOpenHuman(result map[string]any) string {
	var sb strings.Builder
	sb.WriteString("BGP OPEN Message\n")

	// Ze format: openSection is directly in result["open"]
	openSection, ok := result["open"].(map[string]any)
	if !ok {
		return sb.String()
	}

	// Version (Ze format doesn't include version in decode, use 4)
	sb.WriteString("  Version:     4\n")

	// ASN
	if asn, ok := openSection["asn"]; ok {
		fmt.Fprintf(&sb, "  ASN:         %v\n", formatNumber(asn))
	}

	// Hold Time (Ze format uses "hold-time")
	if ht, ok := openSection["hold-time"]; ok {
		fmt.Fprintf(&sb, "  Hold Time:   %v seconds\n", formatNumber(ht))
	}

	// Router ID (Ze format uses "router-id")
	if rid, ok := openSection["router-id"]; ok {
		fmt.Fprintf(&sb, "  Router ID:   %v\n", rid)
	}

	// Capabilities (Ze format uses array)
	if caps, ok := openSection["capabilities"].([]map[string]any); ok && len(caps) > 0 {
		sb.WriteString("  Capabilities:\n")
		for _, capMap := range caps {
			formatCapabilityHuman(&sb, capMap)
		}
	} else if caps, ok := openSection["capabilities"].([]any); ok && len(caps) > 0 {
		sb.WriteString("  Capabilities:\n")
		for _, cap := range caps {
			if capMap, ok := cap.(map[string]any); ok {
				formatCapabilityHuman(&sb, capMap)
			}
		}
	}

	return sb.String()
}

// formatCapabilityHuman formats a single capability for human output.
// Works with Ze format: {"code": N, "name": "...", "value": "..."}.
func formatCapabilityHuman(sb *strings.Builder, cap map[string]any) {
	name, _ := cap["name"].(string)
	if name == "" || name == "unknown" {
		if code, ok := cap["code"]; ok {
			name = fmt.Sprintf("code=%v", formatNumber(code))
		} else {
			name = "unknown"
		}
	}

	fmt.Fprintf(sb, "    %-20s ", name)

	// Ze format uses "value" for capability data
	if value, ok := cap["value"]; ok {
		switch v := value.(type) {
		case string:
			sb.WriteString(v)
		case []string:
			sb.WriteString(strings.Join(v, ", "))
		case []any:
			fams := make([]string, 0, len(v))
			for _, f := range v {
				fams = append(fams, fmt.Sprintf("%v", f))
			}
			sb.WriteString(strings.Join(fams, ", "))
		}
	} else if name == "graceful-restart" {
		// Ze format uses "restart-time"
		if rt, ok := cap["restart-time"]; ok {
			fmt.Fprintf(sb, "%v seconds", formatNumber(rt))
		}
	}

	// Unknown capabilities (name starts with "code=") show raw hex data
	if raw, ok := cap["raw"].(string); ok && raw != "" {
		sb.WriteString(raw)
	}

	sb.WriteString("\n")
}

// formatUpdateHuman formats UPDATE message data as human-readable text.
// Works with Ze format: {"update": {...}}.
func formatUpdateHuman(result map[string]any) string {
	var sb strings.Builder
	sb.WriteString("BGP UPDATE Message\n")

	// Ze format: update is directly in result["update"]
	update, ok := result["update"].(map[string]any)
	if !ok {
		return sb.String()
	}

	// Attributes (Ze format uses "attr")
	if attrs, ok := update["attr"].(map[string]any); ok && len(attrs) > 0 {
		sb.WriteString("  Attributes:\n")
		formatAttributesHuman(&sb, attrs)
	}

	// Announced routes
	if announce, ok := update["announce"].(map[string]any); ok && len(announce) > 0 {
		for family, data := range announce {
			fmt.Fprintf(&sb, "  Announced (%s):\n", family)
			formatNLRIListHuman(&sb, data)
		}
	}

	// Withdrawn routes
	if withdraw, ok := update["withdraw"].(map[string]any); ok && len(withdraw) > 0 {
		for family, data := range withdraw {
			fmt.Fprintf(&sb, "  Withdrawn (%s):\n", family)
			formatWithdrawnHuman(&sb, data)
		}
	}

	return sb.String()
}

// formatAttributesHuman formats path attributes for human output.
func formatAttributesHuman(sb *strings.Builder, attrs map[string]any) {
	// Origin
	if origin, ok := attrs["origin"].(string); ok {
		fmt.Fprintf(sb, "    %-20s %s\n", "origin", origin)
	}

	// AS-Path
	if asPath, ok := attrs["as-path"].(map[string]any); ok {
		fmt.Fprintf(sb, "    %-20s ", "as-path")
		formatASPathHuman(sb, asPath)
		sb.WriteString("\n")
	}

	// Next-Hop (if present as attribute)
	if nh, ok := attrs["next-hop"].(string); ok {
		fmt.Fprintf(sb, "    %-20s %s\n", "next-hop", nh)
	}

	// Local Preference
	if lp, ok := attrs["local-preference"]; ok {
		fmt.Fprintf(sb, "    %-20s %v\n", "local-preference", formatNumber(lp))
	}

	// MED
	if med, ok := attrs["med"]; ok {
		fmt.Fprintf(sb, "    %-20s %v\n", "med", formatNumber(med))
	}

	// Communities
	if comms, ok := attrs["community"].([]any); ok {
		fmt.Fprintf(sb, "    %-20s %v\n", "community", comms)
	}

	// Extended Communities
	if extComms, ok := attrs["extended-community"].([]any); ok {
		fmt.Fprintf(sb, "    %-20s ", "extended-community")
		for i, ec := range extComms {
			if i > 0 {
				sb.WriteString(" ")
			}
			if ecMap, ok := ec.(map[string]any); ok {
				if s, ok := ecMap["string"].(string); ok {
					sb.WriteString(s)
				}
			}
		}
		sb.WriteString("\n")
	}
}

// formatASPathHuman formats AS_PATH for human output.
func formatASPathHuman(sb *strings.Builder, asPath map[string]any) {
	// AS_PATH is keyed by segment index ("0", "1", etc.)
	var asns []string
	for i := 0; ; i++ {
		seg, ok := asPath[fmt.Sprintf("%d", i)].(map[string]any)
		if !ok {
			break
		}
		if values, ok := seg["value"].([]any); ok {
			for _, v := range values {
				asns = append(asns, formatNumber(v))
			}
		}
	}
	sb.WriteString(strings.Join(asns, " "))
}

// formatNLRIListHuman formats NLRI list for human output (announced routes).
func formatNLRIListHuman(sb *strings.Builder, data any) {
	// data is map[nexthop][]nlri
	if nhMap, ok := data.(map[string]any); ok {
		for nh, nlris := range nhMap {
			fmt.Fprintf(sb, "    next-hop: %s\n", nh)
			if nlriList, ok := nlris.([]any); ok {
				for _, n := range nlriList {
					if nMap, ok := n.(map[string]any); ok {
						if prefix, ok := nMap["nlri"].(string); ok {
							fmt.Fprintf(sb, "      %s\n", prefix)
						}
					}
				}
			}
		}
	}
}

// formatWithdrawnHuman formats withdrawn routes for human output.
func formatWithdrawnHuman(sb *strings.Builder, data any) {
	switch v := data.(type) {
	case []string:
		for _, prefix := range v {
			fmt.Fprintf(sb, "    %s\n", prefix)
		}
	case []any:
		for _, item := range v {
			fmt.Fprintf(sb, "    %v\n", item)
		}
	}
}

// formatNLRIHuman formats NLRI data as human-readable text.
func formatNLRIHuman(result map[string]any, family string) string {
	var sb strings.Builder

	// Determine NLRI type from family or content
	nlriType := "NLRI"
	switch {
	case strings.Contains(family, "bgp-ls"):
		nlriType = "BGP-LS NLRI"
	case strings.Contains(family, "flow"):
		nlriType = "FlowSpec NLRI"
	case strings.Contains(family, "evpn"):
		nlriType = "EVPN NLRI"
	}

	fmt.Fprintf(&sb, "%s (%s):\n", nlriType, family)

	// Format based on content
	for key, value := range result {
		formatNLRIFieldHuman(&sb, key, value, "  ")
	}

	return sb.String()
}

// formatNLRIFieldHuman formats a single NLRI field for human output.
func formatNLRIFieldHuman(sb *strings.Builder, key string, value any, indent string) {
	switch v := value.(type) {
	case map[string]any:
		fmt.Fprintf(sb, "%s%s:\n", indent, key)
		for k, val := range v {
			formatNLRIFieldHuman(sb, k, val, indent+"  ")
		}
	case []any:
		fmt.Fprintf(sb, "%s%-20s ", indent, key)
		for i, item := range v {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(sb, "%v", item)
		}
		sb.WriteString("\n")
	default:
		fmt.Fprintf(sb, "%s%-20s %v\n", indent, key, value)
	}
}

// formatNumber formats numeric values, handling float64 from JSON unmarshaling.
func formatNumber(v any) string {
	if n, ok := v.(float64); ok {
		if n == float64(int64(n)) {
			return fmt.Sprintf("%d", int64(n))
		}
		return fmt.Sprintf("%v", n)
	}
	return fmt.Sprintf("%v", v)
}
