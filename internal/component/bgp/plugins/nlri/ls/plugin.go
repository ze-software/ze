// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS NLRI plugin
// RFC: rfc/short/rfc7752.md

package ls

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// bgplsLogger is the package-level logger, disabled by default.
var bgplsLogger = slogutil.DiscardLogger()

// SetBGPLSLogger sets the package-level logger.
// Called by cmd/ze/bgp/plugin_bgpls.go with slogutil.PluginLogger().
func SetBGPLSLogger(l *slog.Logger) {
	if l != nil {
		bgplsLogger = l
	}
}

// RunBGPLSPlugin runs the BGP-LS plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func RunBGPLSPlugin(conn net.Conn) int {
	bgplsLogger.Debug("bgpls plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-nlri-ls", conn)
	defer func() { _ = p.Close() }()

	p.OnDecodeNLRI(func(family string, hexStr string) (string, error) {
		if !isValidBGPLSFamily(family) {
			return "", fmt.Errorf("unsupported family: %s", family)
		}

		data, err := hex.DecodeString(hexStr)
		if err != nil {
			return "", fmt.Errorf("invalid hex: %w", err)
		}

		results := decodeBGPLSNLRI(data)
		if len(results) == 0 {
			return "", fmt.Errorf("no valid BGP-LS NLRIs decoded")
		}

		// Single object for single NLRI, array for multiple.
		var jsonBytes []byte
		if len(results) == 1 {
			jsonBytes, err = json.Marshal(results[0])
		} else {
			jsonBytes, err = json.Marshal(results)
		}
		if err != nil {
			return "", fmt.Errorf("JSON encoding failed: %w", err)
		}

		return string(jsonBytes), nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		Families: []sdk.FamilyDecl{
			{Name: "bgp-ls/bgp-ls", Mode: "decode", AFI: 16388, SAFI: 71},
			{Name: "bgp-ls/bgp-ls-vpn", Mode: "decode", AFI: 16388, SAFI: 72},
		},
	})
	if err != nil {
		bgplsLogger.Error("bgpls plugin failed", "error", err)
		return 1
	}

	return 0
}

// Protocol constants.
const (
	cmdDecode       = "decode"
	objTypeNLRI     = "nlri"
	fmtJSON         = "json"
	fmtText         = "text"
	respDecodedUnk  = "decoded unknown"
	respDecodedJSON = "decoded json "
)

// GetBGPLSYANG returns the embedded YANG schema for the bgpls plugin.
// BGP-LS plugin doesn't augment config schema, returns empty.
func GetBGPLSYANG() string {
	return ""
}

// RunBGPLSCLIDecode decodes BGP-LS NLRI from hex string for CLI mode.
// This is for direct CLI invocation: ze plugin bgpls --nlri <hex>
// Output is plain JSON or text (no "decoded json" prefix).
// Errors go to errOut (typically stderr), results go to output (typically stdout).
func RunBGPLSCLIDecode(hexData, family string, textOutput bool, output, errOut io.Writer) int {
	writeErr := func(format string, args ...any) {
		_, e := fmt.Fprintf(errOut, format, args...)
		_ = e // CLI output - pipe failure is unrecoverable
	}
	writeOut := func(s string) {
		_, e := fmt.Fprintln(output, s)
		_ = e // CLI output - pipe failure is unrecoverable
	}

	if !isValidBGPLSFamily(family) {
		writeErr("error: invalid family: %s (expected bgp-ls/bgp-ls or bgp-ls/bgp-ls-vpn)\n", family)
		return 1
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeErr("error: invalid hex: %v\n", err)
		return 1
	}

	results := decodeBGPLSNLRI(data)
	if len(results) == 0 {
		writeErr("error: no valid BGP-LS routes decoded\n")
		return 1
	}

	if textOutput {
		writeOut(formatBGPLSText(results))
		return 0
	}

	// JSON output (default) - single object for single NLRI, array for multiple
	var jsonBytes []byte
	if len(results) == 1 {
		jsonBytes, err = json.MarshalIndent(results[0], "", "  ")
	} else {
		jsonBytes, err = json.MarshalIndent(results, "", "  ")
	}
	if err != nil {
		writeErr("error: JSON encoding failed: %v\n", err)
		return 1
	}
	writeOut(string(jsonBytes))
	return 0
}

// RunBGPLSDecode runs the plugin in decode mode for ze bgp decode (engine protocol).
func RunBGPLSDecode(input io.Reader, output io.Writer) int {
	writeUnknown := func() {
		if _, err := fmt.Fprintln(output, "decoded unknown"); err != nil {
			bgplsLogger.Debug("write error", "err", err)
		}
	}

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 {
			writeUnknown()
			continue
		}

		cmd := parts[0]
		objType := parts[1]

		// Handle format specifier
		format := fmtJSON
		if objType == fmtJSON || objType == fmtText {
			format = objType
			if len(parts) < 4 {
				writeUnknown()
				continue
			}
			objType = parts[2]
			parts = append([]string{cmd, objType}, parts[3:]...)
		}

		if cmd == cmdDecode && objType == objTypeNLRI {
			handleDecodeNLRI(parts, format, output, writeUnknown)
		} else if cmd == cmdDecode {
			writeUnknown()
		}
	}
	return 0
}

// handleDecodeNLRI handles: decode nlri <family> <hex>.
func handleDecodeNLRI(parts []string, format string, output io.Writer, writeUnknown func()) {
	if len(parts) < 4 {
		writeUnknown()
		return
	}

	fam := strings.ToLower(parts[2])
	hexData := parts[3]

	if !isValidBGPLSFamily(fam) {
		writeUnknown()
		return
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeUnknown()
		return
	}

	results := decodeBGPLSNLRI(data)
	if len(results) == 0 {
		writeUnknown()
		return
	}

	if format == fmtText {
		if _, err := fmt.Fprintln(output, "decoded text "+formatBGPLSText(results)); err != nil {
			bgplsLogger.Debug("write error", "err", err)
		}
		return
	}

	// Single object for single NLRI, array for multiple (matches VPN pattern)
	var jsonBytes []byte
	if len(results) == 1 {
		jsonBytes, err = json.Marshal(results[0])
	} else {
		jsonBytes, err = json.Marshal(results)
	}
	if err != nil {
		writeUnknown()
		return
	}
	if _, err := fmt.Fprintln(output, "decoded json "+string(jsonBytes)); err != nil {
		bgplsLogger.Debug("write error", "err", err)
	}
}

// isValidBGPLSFamily checks if family is a BGP-LS family.
func isValidBGPLSFamily(family string) bool {
	return family == "bgp-ls/bgp-ls" || family == "bgp-ls/bgp-ls-vpn"
}

// decodeBGPLSNLRI decodes BGP-LS NLRI wire bytes to array of JSON maps.
// MP_REACH/MP_UNREACH can contain multiple packed NLRIs.
func decodeBGPLSNLRI(data []byte) []map[string]any {
	var results []map[string]any

	// Handle empty/truncated data
	if len(data) < 4 {
		results = append(results, map[string]any{
			"parsed": false,
			"raw":    fmt.Sprintf("%X", data),
		})
		return results
	}

	remaining := data
	for len(remaining) > 0 {
		parsed, rest, err := ParseBGPLSWithRest(remaining)
		if err != nil {
			bgplsLogger.Debug("parse bgpls failed", "err", err)
			// Add remaining as unparsed
			results = append(results, map[string]any{
				"parsed": false,
				"raw":    fmt.Sprintf("%X", remaining),
			})
			break
		}
		results = append(results, bgplsToJSON(parsed, remaining[:len(remaining)-len(rest)]))
		remaining = rest
	}

	return results
}

// bgplsToJSON converts a BGP-LS NLRI to JSON format.
// RFC 7752 defines the NLRI structure.
func bgplsToJSON(n BGPLSNLRI, data []byte) map[string]any {
	result := map[string]any{
		"ls-nlri-type":        bgplsNLRITypeString(uint16(n.NLRIType())),
		"l3-routing-topology": n.Identifier(),
		"protocol-id":         int(n.ProtocolID()),
	}

	// Type-specific fields based on NLRI type - parse from wire bytes
	switch n.NLRIType() {
	case BGPLSNodeNLRI:
		result["node-descriptors"] = parseBGPLSNodeTLVs(data)

	case BGPLSLinkNLRI:
		localDescs, remoteDescs, info := parseBGPLSLinkTLVs(data)
		result["local-node-descriptors"] = localDescs
		result["remote-node-descriptors"] = remoteDescs
		result["interface-addresses"] = info.ifAddrs
		result["neighbor-addresses"] = info.neighAddrs
		result["multi-topology-ids"] = info.mtIDs
		result["link-identifiers"] = info.linkIDs

	case BGPLSPrefixV4NLRI, BGPLSPrefixV6NLRI:
		nodeDescs, prefixInfo := parseBGPLSPrefixTLVs(data, n.NLRIType())
		result["node-descriptors"] = nodeDescs
		if prefixInfo.prefix != "" {
			result["ip-reachability-tlv"] = prefixInfo.prefix
			result["ip-reach-prefix"] = prefixInfo.prefix
		}
		result["multi-topology-ids"] = prefixInfo.mtIDs

	case BGPLSSRv6SIDNLRI:
		result["node-descriptors"] = parseBGPLSNodeTLVs(data)
		if v, ok := n.(*BGPLSSRv6SID); ok && len(v.SRv6SID.SRv6SID) > 0 {
			result["srv6-sid"] = formatIPv6Compressed(v.SRv6SID.SRv6SID)
		}
	}
	// Note: Unknown NLRI types are rejected by ParseBGPLS, so no default case needed.

	return result
}

// bgplsNLRITypeString returns the NLRI type string.
// RFC 7752 Section 3.2, Table 1 defines NLRI types 1-4, RFC 9514 defines type 6.
func bgplsNLRITypeString(nlriType uint16) string {
	switch nlriType {
	case 1:
		return "bgpls-node"
	case 2:
		return "bgpls-link"
	case 3:
		return "bgpls-prefix-v4"
	case 4:
		return "bgpls-prefix-v6"
	case 6:
		return "bgpls-srv6-sid"
	}
	// RFC 7752: Unknown NLRI types are valid and should be labeled generically.
	// This is forward-compatibility, not silent ignore.
	return fmt.Sprintf("bgpls-type-%d", nlriType)
}

// prefixDescriptorInfo holds parsed prefix descriptor information.
type prefixDescriptorInfo struct {
	prefix string
	mtIDs  []any
}

// parseBGPLSPrefixTLVs parses a Prefix NLRI wire format to extract all fields.
// RFC 7752 Section 3.2.3 defines the Prefix NLRI format.
func parseBGPLSPrefixTLVs(data []byte, nlriType BGPLSNLRIType) (nodeDescs []any, info prefixDescriptorInfo) {
	info.mtIDs = []any{}

	// Minimum: Type(2) + Len(2) + ProtoID(1) + Identifier(8) = 13 bytes
	if len(data) < 13 {
		return nodeDescs, info
	}

	offset := 13

	for offset+4 <= len(data) {
		tlvType := binary.BigEndian.Uint16(data[offset : offset+2])
		tlvLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))

		if offset+4+tlvLen > len(data) {
			break
		}

		value := data[offset+4 : offset+4+tlvLen]

		switch tlvType {
		case 256: // Local Node Descriptors
			nodeDescs = parseNodeDescriptorSubTLVs(value)
		case 263: // Multi-Topology ID
			for i := 0; i+2 <= len(value); i += 2 {
				mtID := binary.BigEndian.Uint16(value[i:i+2]) & 0x0FFF
				info.mtIDs = append(info.mtIDs, int(mtID))
			}
		case 265: // IP Reachability Information
			info.prefix = formatIPReachability(value, nlriType)
		}
		// RFC 7752: Unknown TLVs are skipped (forward-compatibility).

		offset += 4 + tlvLen
	}

	return nodeDescs, info
}

// parseBGPLSNodeTLVs parses a Node NLRI wire format to extract node descriptors.
// RFC 7752 Section 3.2.1 defines the Node NLRI format.
func parseBGPLSNodeTLVs(data []byte) []any {
	// Minimum: Type(2) + Len(2) + ProtoID(1) + Identifier(8) = 13 bytes
	if len(data) < 13 {
		return []any{}
	}

	offset := 13

	for offset+4 <= len(data) {
		tlvType := binary.BigEndian.Uint16(data[offset : offset+2])
		tlvLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))

		if offset+4+tlvLen > len(data) {
			break
		}

		value := data[offset+4 : offset+4+tlvLen]

		if tlvType == 256 { // Local Node Descriptors
			return parseNodeDescriptorSubTLVs(value)
		}
		// RFC 7752: Unknown TLVs are skipped (forward-compatibility).

		offset += 4 + tlvLen
	}

	return []any{}
}

// linkDescriptorInfo holds parsed link descriptor information.
type linkDescriptorInfo struct {
	ifAddrs    []any // IPv4/IPv6 interface addresses
	neighAddrs []any // IPv4/IPv6 neighbor addresses
	mtIDs      []any // Multi-topology IDs
	linkIDs    []any // Link local/remote identifiers
}

// parseBGPLSLinkTLVs parses a Link NLRI wire format to extract all TLVs.
// RFC 7752 Section 3.2.2 defines the Link NLRI format.
func parseBGPLSLinkTLVs(data []byte) (localDescs, remoteDescs []any, info linkDescriptorInfo) {
	info = linkDescriptorInfo{
		ifAddrs:    []any{},
		neighAddrs: []any{},
		mtIDs:      []any{},
		linkIDs:    []any{},
	}

	// Minimum: Type(2) + Len(2) + ProtoID(1) + Identifier(8) = 13 bytes
	if len(data) < 13 {
		return localDescs, remoteDescs, info
	}

	offset := 13

	for offset+4 <= len(data) {
		tlvType := binary.BigEndian.Uint16(data[offset : offset+2])
		tlvLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))

		if offset+4+tlvLen > len(data) {
			break
		}

		value := data[offset+4 : offset+4+tlvLen]

		switch tlvType {
		case 256: // Local Node Descriptors
			localDescs = parseNodeDescriptorSubTLVs(value)
		case 257: // Remote Node Descriptors
			remoteDescs = parseNodeDescriptorSubTLVs(value)
		case 258: // Link Local/Remote Identifiers (8 bytes: local(4) + remote(4))
			if len(value) >= 8 {
				localID := binary.BigEndian.Uint32(value[0:4])
				remoteID := binary.BigEndian.Uint32(value[4:8])
				info.linkIDs = append(info.linkIDs, map[string]any{
					"link-local-id":  localID,
					"link-remote-id": remoteID,
				})
			}
		case 259: // IPv4 Interface Address
			if len(value) == 4 {
				info.ifAddrs = append(info.ifAddrs,
					fmt.Sprintf("%d.%d.%d.%d", value[0], value[1], value[2], value[3]))
			}
		case 260: // IPv4 Neighbor Address
			if len(value) == 4 {
				info.neighAddrs = append(info.neighAddrs,
					fmt.Sprintf("%d.%d.%d.%d", value[0], value[1], value[2], value[3]))
			}
		case 261: // IPv6 Interface Address
			if len(value) == 16 {
				info.ifAddrs = append(info.ifAddrs, formatIPv6Compressed(value))
			}
		case 262: // IPv6 Neighbor Address
			if len(value) == 16 {
				info.neighAddrs = append(info.neighAddrs, formatIPv6Compressed(value))
			}
		case 263: // Multi-Topology ID
			for i := 0; i+2 <= len(value); i += 2 {
				mtID := binary.BigEndian.Uint16(value[i:i+2]) & 0x0FFF
				info.mtIDs = append(info.mtIDs, int(mtID))
			}
		}
		// RFC 7752: Unknown TLVs are skipped (forward-compatibility).

		offset += 4 + tlvLen
	}

	return localDescs, remoteDescs, info
}

// parseNodeDescriptorSubTLVs parses node descriptor sub-TLVs into array format.
// RFC 7752 Section 3.2.1.4 defines the sub-TLV types.
func parseNodeDescriptorSubTLVs(data []byte) []any {
	var descs []any
	offset := 0

	for offset+4 <= len(data) {
		tlvType := binary.BigEndian.Uint16(data[offset : offset+2])
		tlvLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))

		if offset+4+tlvLen > len(data) {
			break
		}

		value := data[offset+4 : offset+4+tlvLen]

		switch tlvType {
		case 512: // Autonomous System
			if len(value) >= 4 {
				descs = append(descs, map[string]any{
					"autonomous-system": binary.BigEndian.Uint32(value),
				})
			}
		case 513: // BGP-LS Identifier
			if len(value) >= 4 {
				descs = append(descs, map[string]any{
					"bgp-ls-identifier": fmt.Sprintf("%d", binary.BigEndian.Uint32(value)),
				})
			}
		case 514: // OSPF Area-ID
			if len(value) >= 4 {
				descs = append(descs, map[string]any{
					"ospf-area-id": fmt.Sprintf("%d.%d.%d.%d", value[0], value[1], value[2], value[3]),
				})
			}
		case 515: // IGP Router-ID
			routerID := formatRouterID(value)
			// Check for pseudonode (7-8 bytes) and add designated-router-id
			switch len(value) {
			case 8:
				// OSPF pseudonode: Router-ID + DR interface
				descs = append(descs, map[string]any{
					"router-id":            fmt.Sprintf("%d.%d.%d.%d", value[0], value[1], value[2], value[3]),
					"designated-router-id": fmt.Sprintf("%d.%d.%d.%d", value[4], value[5], value[6], value[7]),
				})
			case 7:
				// IS-IS pseudonode: System-ID + PSN
				descs = append(descs, map[string]any{
					"router-id": routerID,
					"psn":       fmt.Sprintf("%d", value[6]),
				})
			// RFC 7752: 4-byte (OSPF) and 6-byte (IS-IS) are standard lengths.
			case 4, 6:
				descs = append(descs, map[string]any{"router-id": routerID})
			}
			// Unknown Router-ID lengths are silently skipped per RFC 7752 forward-compatibility.
		}
		// RFC 7752: Unknown sub-TLVs are skipped (forward-compatibility).

		offset += 4 + tlvLen
	}

	return descs
}

// formatRouterID formats the IGP Router-ID based on its length.
// - 4 bytes: OSPF Router-ID as dotted decimal
// - 6 bytes: IS-IS System ID as hex
// - 7 bytes: IS-IS pseudonode (6-byte System ID + 1-byte PSN)
// - 8 bytes: OSPF pseudonode (4-byte Router-ID + 4-byte interface).
func formatRouterID(id []byte) string {
	switch len(id) {
	case 4:
		return fmt.Sprintf("%d.%d.%d.%d", id[0], id[1], id[2], id[3])
	case 6:
		return fmt.Sprintf("%02x%02x%02x%02x%02x%02x", id[0], id[1], id[2], id[3], id[4], id[5])
	case 7:
		return fmt.Sprintf("%02x%02x%02x%02x%02x%02x%02x", id[0], id[1], id[2], id[3], id[4], id[5], id[6])
	case 8:
		routerID := fmt.Sprintf("%d.%d.%d.%d", id[0], id[1], id[2], id[3])
		ifAddr := fmt.Sprintf("%d.%d.%d.%d", id[4], id[5], id[6], id[7])
		return routerID + "," + ifAddr
	}
	// Unknown length: return hex (forward-compatibility).
	return fmt.Sprintf("%X", id)
}

// formatIPReachability formats the IP Reachability Information TLV.
// RFC 7752 Section 3.2.3.2 defines the format: prefix-length + prefix-bytes.
func formatIPReachability(data []byte, nlriType BGPLSNLRIType) string {
	if len(data) < 1 {
		return ""
	}

	prefixLen := int(data[0])
	byteLen := (prefixLen + 7) / 8

	if len(data) < 1+byteLen {
		return ""
	}

	prefixBytes := data[1 : 1+byteLen]

	if nlriType == BGPLSPrefixV6NLRI {
		addr := make([]byte, 16)
		copy(addr, prefixBytes)
		return formatIPv6Compressed(addr) + "/" + fmt.Sprintf("%d", prefixLen)
	}

	addr := make([]byte, 4)
	copy(addr, prefixBytes)
	return fmt.Sprintf("%d.%d.%d.%d/%d", addr[0], addr[1], addr[2], addr[3], prefixLen)
}

// formatIPv6Compressed formats a 16-byte IPv6 address with zero compression.
func formatIPv6Compressed(addr []byte) string {
	if len(addr) != 16 {
		return fmt.Sprintf("%X", addr)
	}
	ip := netip.AddrFrom16([16]byte(addr))
	return ip.String()
}

// formatBGPLSText formats BGP-LS NLRI results as human-readable text.
func formatBGPLSText(results []map[string]any) string {
	texts := make([]string, 0, len(results))
	for _, r := range results {
		texts = append(texts, formatBGPLSTextSingle(r))
	}
	return strings.Join(texts, "; ")
}

// formatBGPLSTextSingle formats a single BGP-LS NLRI as human-readable text.
func formatBGPLSTextSingle(result map[string]any) string {
	var parts []string

	if v, ok := result["ls-nlri-type"].(string); ok {
		parts = append(parts, v)
	}
	if v, ok := result["protocol-id"].(int); ok {
		parts = append(parts, fmt.Sprintf("proto=%d", v))
	}
	if v, ok := result["l3-routing-topology"].(uint64); ok {
		parts = append(parts, fmt.Sprintf("id=%d", v))
	}

	if len(parts) == 0 {
		return "(empty)"
	}
	return strings.Join(parts, " ")
}
