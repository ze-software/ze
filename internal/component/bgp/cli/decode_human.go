// Design: docs/architecture/core-design.md — BGP CLI commands
// Overview: decode.go — top-level decode dispatch calls human formatters
// Related: decode_mp.go — decodeNLRIOnly calls formatNLRIHuman

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// =============================================================================
// Human-Readable Formatters
// =============================================================================

// formatOpenHuman formats OPEN message data as human-readable text.
// Works with Ze format: {"open": {...}}.
func formatOpenHuman(result map[string]any) string {
	var sb textbuf.Buffer
	sb.Str("BGP OPEN Message\n")

	// Ze format: openSection is directly in result["open"]
	openSection, ok := result["open"].(map[string]any)
	if !ok {
		return sb.String()
	}

	// Version (Ze format doesn't include version in decode, use 4)
	sb.Str("  Version:     4\n")

	// ASN
	if asn, ok := openSection["asn"]; ok {
		fmt.Fprintf(&sb, "  ASN:         %v\n", formatNumber(asn)) //nolint:errcheck // buffer output
	}

	// Hold Time (Ze format nests under "timer")
	if timer, ok := openSection["timer"].(map[string]any); ok {
		if ht, ok := timer["hold-time"]; ok {
			fmt.Fprintf(&sb, "  Hold Time:   %v seconds\n", formatNumber(ht)) //nolint:errcheck // buffer output
		}
	}

	// Router ID (Ze format uses "router-id")
	if rid, ok := openSection["router-id"]; ok {
		fmt.Fprintf(&sb, "  Router ID:   %v\n", rid) //nolint:errcheck // buffer output
	}

	// Capabilities (Ze format uses array)
	if caps, ok := openSection["capabilities"].([]map[string]any); ok && len(caps) > 0 {
		sb.Str("  Capabilities:\n")
		for _, capMap := range caps {
			formatCapabilityHuman(&sb, capMap)
		}
	} else if caps, ok := openSection["capabilities"].([]any); ok && len(caps) > 0 {
		sb.Str("  Capabilities:\n")
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
func formatCapabilityHuman(sb *textbuf.Buffer, cap map[string]any) {
	name, _ := cap["name"].(string)
	if name == "" || name == capNameUnknown {
		if code, ok := cap["code"]; ok {
			name = fmt.Sprintf("code=%v", formatNumber(code))
		} else {
			name = capNameUnknown
		}
	}

	fmt.Fprintf(sb, "    %-20s ", name) //nolint:errcheck // output

	// Ze format uses "value" for capability data
	if value, ok := cap["value"]; ok {
		switch v := value.(type) {
		case string:
			sb.Str(v)
		case []string:
			sb.Join(v, ", ")
		case []any:
			fams := make([]string, 0, len(v))
			for _, f := range v {
				fams = append(fams, fmt.Sprintf("%v", f))
			}
			sb.Join(fams, ", ")
		}
	} else {
		// Plugin-decoded capabilities may use custom keys (e.g., "version" for software-version).
		// Display any string value that isn't "code" or "name".
		for k, v := range cap {
			if k == "code" || k == "name" || k == jsonKeyRaw {
				continue
			}
			if s, ok := v.(string); ok {
				sb.Str(s)
				break
			}
		}
	}

	// Unknown capabilities (name starts with "code=") show raw hex data
	if raw, ok := cap[jsonKeyRaw].(string); ok && raw != "" {
		sb.Str(raw)
	}

	sb.Str("\n")
}

// formatUpdateHuman formats UPDATE message data as human-readable text.
// Works with Ze format: {"update": {...}}.
func formatUpdateHuman(result map[string]any) string {
	var sb textbuf.Buffer
	sb.Str("BGP UPDATE Message\n")

	// Ze format: update is directly in result["update"]
	update, ok := result["update"].(map[string]any)
	if !ok {
		return sb.String()
	}

	// Attributes (Ze format uses "attr")
	if attrs, ok := update["attr"].(map[string]any); ok && len(attrs) > 0 {
		sb.Str("  Attributes:\n")
		formatAttributesHuman(&sb, attrs)
	}

	// Announced routes
	if announce, ok := update["announce"].(map[string]any); ok && len(announce) > 0 {
		for fam, data := range announce {
			fmt.Fprintf(&sb, "  Announced (%s):\n", fam) //nolint:errcheck // buffer output
			formatNLRIListHuman(&sb, data)
		}
	}

	// Withdrawn routes
	if withdraw, ok := update["withdraw"].(map[string]any); ok && len(withdraw) > 0 {
		for fam, data := range withdraw {
			fmt.Fprintf(&sb, "  Withdrawn (%s):\n", fam) //nolint:errcheck // buffer output
			formatWithdrawnHuman(&sb, data)
		}
	}

	return sb.String()
}

// formatAttributesHuman formats path attributes for human output.
func formatAttributesHuman(sb *textbuf.Buffer, attrs map[string]any) {
	if origin, ok := unwrapAttr(attrs["origin"]).(string); ok {
		writeAttrLine(sb, "origin", origin)
	}

	switch asPath := unwrapAttr(attrs["as-path"]).(type) {
	case []uint32:
		writeAttrLabel(sb, "as-path")
		for i, asn := range asPath {
			if i > 0 {
				sb.WriteByte(' ')
			}
			sb.Str(textbuf.StringUint32(asn))
		}
		sb.WriteByte('\n')
	case []any:
		writeAttrLabel(sb, "as-path")
		for i, v := range asPath {
			if i > 0 {
				sb.WriteByte(' ')
			}
			sb.Str(formatNumber(v))
		}
		sb.WriteByte('\n')
	}

	if nh, ok := unwrapAttr(attrs["next-hop"]).(string); ok {
		writeAttrLine(sb, "next-hop", nh)
	}

	if lp := unwrapAttr(attrs["local-preference"]); lp != nil {
		writeAttrLine(sb, "local-preference", formatNumber(lp))
	}

	if med := unwrapAttr(attrs["med"]); med != nil {
		writeAttrLine(sb, "med", formatNumber(med))
	}

	// The four community attributes, in attribute-code order (8, 16, 25, 32).
	// Each value is what renderAttributeZe (decode_update.go) put in the map,
	// so the types here are the Go types that function returns, never the
	// []any a JSON round trip would give.
	writeCommunityLine(sb, "community", attrs)
	writeExtendedCommunityLine(sb, attrs)
	writeCommunityLine(sb, "ipv6-extended-community", attrs)
	writeCommunityLine(sb, "large-community", attrs)
}

// writeCommunityLine writes one line for a community attribute whose value is a
// list of rendered strings.
func writeCommunityLine(sb *textbuf.Buffer, label string, attrs map[string]any) {
	comms, ok := unwrapAttr(attrs[label]).([]string)
	if !ok {
		return
	}

	writeAttrLabel(sb, label)
	for i, comm := range comms {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.Str(comm)
	}
	sb.WriteByte('\n')
}

// writeExtendedCommunityLine writes the EXTENDED_COMMUNITIES line. It is the
// one community attribute rendered as objects rather than as strings
// (decode_extcomm.go), so it reads the decoded name out of each object.
func writeExtendedCommunityLine(sb *textbuf.Buffer, attrs map[string]any) {
	extComms, ok := unwrapAttr(attrs["extended-community"]).([]map[string]any)
	if !ok {
		return
	}

	writeAttrLabel(sb, "extended-community")
	for i, extComm := range extComms {
		if i > 0 {
			sb.WriteByte(' ')
		}
		if text, ok := extComm["string"].(string); ok {
			sb.Str(text)
		}
	}
	sb.WriteByte('\n')
}

// unwrapAttr extracts the value from a flag-annotated attribute map.
func unwrapAttr(v any) any {
	if m, ok := v.(map[string]any); ok {
		if val, ok := m["value"]; ok {
			return val
		}
	}
	return v
}

const attrLabelWidth = 20

// writeAttrLine writes a padded "    name                 value\n" line.
func writeAttrLine(sb *textbuf.Buffer, name, value string) {
	writeAttrLabel(sb, name)
	sb.Str(value)
	sb.WriteByte('\n')
}

// writeAttrLabel writes "    name" left-padded to attrLabelWidth, followed by a space.
func writeAttrLabel(sb *textbuf.Buffer, name string) {
	sb.Str("    ")
	sb.Str(name)
	for i := len(name); i < attrLabelWidth; i++ {
		sb.WriteByte(' ')
	}
	sb.WriteByte(' ')
}

// formatNLRIListHuman formats NLRI list for human output (announced routes).
func formatNLRIListHuman(sb *textbuf.Buffer, data any) {
	// data is map[nexthop][]nlri
	if nhMap, ok := data.(map[string]any); ok {
		for nh, nlris := range nhMap {
			fmt.Fprintf(sb, "    next-hop: %s\n", nh) //nolint:errcheck // output
			if nlriList, ok := nlris.([]any); ok {
				for _, n := range nlriList {
					if nMap, ok := n.(map[string]any); ok {
						if prefix, ok := nMap[jsonKeyNLRI].(string); ok {
							fmt.Fprintf(sb, "      %s\n", prefix) //nolint:errcheck // output
						}
					}
				}
			}
		}
	}
}

// formatWithdrawnHuman formats withdrawn routes for human output.
func formatWithdrawnHuman(sb *textbuf.Buffer, data any) {
	switch v := data.(type) {
	case []string:
		for _, prefix := range v {
			fmt.Fprintf(sb, "    %s\n", prefix) //nolint:errcheck // output
		}
	case []any:
		for _, item := range v {
			fmt.Fprintf(sb, "    %v\n", item) //nolint:errcheck // output
		}
	}
}

// formatNLRIHuman formats NLRI data as human-readable text.
func formatNLRIHuman(result map[string]any, family string) string {
	var sb textbuf.Buffer

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

	fmt.Fprintf(&sb, "%s (%s):\n", nlriType, family) //nolint:errcheck // buffer output

	// Format based on content
	for key, value := range result {
		formatNLRIFieldHuman(&sb, key, value, "  ")
	}

	return sb.String()
}

// formatNLRIFieldHuman formats a single NLRI field for human output.
func formatNLRIFieldHuman(sb *textbuf.Buffer, key string, value any, indent string) {
	switch v := value.(type) {
	case map[string]any:
		fmt.Fprintf(sb, "%s%s:\n", indent, key) //nolint:errcheck // output
		for k, val := range v {
			formatNLRIFieldHuman(sb, k, val, indent+"  ")
		}
	case []any:
		fmt.Fprintf(sb, "%s%-20s ", indent, key) //nolint:errcheck // output
		for i, item := range v {
			if i > 0 {
				sb.Str(", ")
			}
			fmt.Fprintf(sb, "%v", item) //nolint:errcheck // output
		}
		sb.Str("\n")
	default:
		fmt.Fprintf(sb, "%s%-20s %v\n", indent, key, value) //nolint:errcheck // output
	}
}

// formatNumber formats numeric values, handling float64 from JSON unmarshaling.
func formatNumber(v any) string {
	if n, ok := v.(float64); ok {
		if n == float64(int64(n)) {
			return strconv.Itoa(int(int64(n)))
		}
		return fmt.Sprintf("%v", n)
	}
	return fmt.Sprintf("%v", v)
}
