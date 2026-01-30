package bgp

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/nlri"
)

// Message type constants.
const (
	msgTypeOpen   = "open"
	msgTypeUpdate = "update"
	msgTypeNLRI   = "nlri"
)

// cmdDecode handles the 'decode' subcommand.
// Decodes BGP messages from hex and outputs ExaBGP-compatible JSON.
func cmdDecode(args []string) int {
	fs := flag.NewFlagSet("decode", flag.ExitOnError)

	openMsg := fs.Bool("open", false, "decode as OPEN message")
	updateMsg := fs.Bool("update", false, "decode as UPDATE message")
	nlriFamily := fs.String("nlri", "", "decode as NLRI with family (e.g., 'ipv4/flow')")
	family := fs.String("f", "", "address family for UPDATE (e.g., 'ipv4/unicast', 'l2vpn/evpn')")
	outputJSON := fs.Bool("json", false, "output JSON instead of human-readable format")
	var plugins pluginFlags
	fs.Var(&plugins, "plugin", "plugin for capability/NLRI decoding (e.g., ze.hostname, flowspec)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze bgp decode [options] <hex-payload>

Decode BGP message from hexadecimal and output ExaBGP-compatible JSON.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  ze bgp decode --open FFFF...       # Decode OPEN message
  ze bgp decode --update FFFF...     # Decode UPDATE message
  ze bgp decode --plugin ze.hostname --open FFFF...  # Decode with hostname plugin
  ze bgp decode --nlri l2vpn/evpn 02...  # Decode NLRI with family
  ze bgp decode --plugin flowspec --nlri ipv4/flow 07...  # Decode NLRI via plugin

The hex payload can include colons or spaces which will be stripped.
`)
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing hex payload\n")
		fs.Usage()
		return 1
	}

	payload := fs.Arg(0)

	// Determine message type from flags
	var msgType string
	switch {
	case *openMsg:
		msgType = msgTypeOpen
	case *updateMsg:
		msgType = msgTypeUpdate
	case *nlriFamily != "":
		msgType = msgTypeNLRI
	}

	// Use nlriFamily for NLRI mode, fall back to -f flag
	familyStr := *family
	if *nlriFamily != "" {
		familyStr = *nlriFamily
	}

	output, err := decodeHexPacket(payload, msgType, familyStr, plugins, *outputJSON)
	if err != nil {
		if *outputJSON {
			// Return valid JSON error
			errJSON := map[string]any{
				"error":  err.Error(),
				"parsed": false,
			}
			data, _ := json.Marshal(errJSON)
			fmt.Println(string(data))
		} else {
			// Human-readable error
			fmt.Println("Error:", err.Error())
		}
		return 1
	}

	fmt.Println(output)
	return 0
}

// decodeHexPacket decodes a hex BGP packet and returns formatted output.
// If outputJSON is true, returns JSON; otherwise returns human-readable format.
func decodeHexPacket(hexStr, msgType, family string, plugins []string, outputJSON bool) (string, error) {
	// Normalize hex input - remove colons, spaces, uppercase
	hexStr = strings.ReplaceAll(hexStr, ":", "")
	hexStr = strings.ReplaceAll(hexStr, " ", "")
	hexStr = strings.ToUpper(hexStr)

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return "", fmt.Errorf("invalid hex: %w", err)
	}

	// Detect format: if FF*16 marker present, it's a full message
	// Otherwise assume UPDATE body
	hasHeader := hasValidMarker(data)

	if msgType == "" {
		if hasHeader {
			msgType = detectMessageType(data)
		} else {
			msgType = msgTypeUpdate // Default to UPDATE body
		}
	}

	// For NLRI-only mode, don't wrap in envelope
	if msgType == msgTypeNLRI {
		return decodeNLRIOnly(data, family, plugins, outputJSON)
	}

	// Build output based on message type
	var result map[string]any
	switch msgType {
	case msgTypeOpen:
		result, err = decodeOpenMessage(data, hasHeader, plugins)
	case msgTypeUpdate:
		result, err = decodeUpdateMessage(data, family, hasHeader)
	default: // Unsupported message type
		return "", fmt.Errorf("unsupported message type: %s", msgType)
	}

	if err != nil {
		return "", err
	}

	// Human-readable output
	if !outputJSON {
		switch msgType {
		case msgTypeOpen:
			return formatOpenHuman(result), nil
		case msgTypeUpdate:
			return formatUpdateHuman(result), nil
		}
	}

	// Merge result into envelope, preserving neighbor section fields
	envelope := makeEnvelope(msgType)
	if neighborResult, ok := result["neighbor"].(map[string]any); ok {
		// Merge into existing neighbor section (preserving address, asn, direction)
		if neighborEnv, ok := envelope["neighbor"].(map[string]any); ok {
			for k, v := range neighborResult {
				neighborEnv[k] = v
			}
		}
	} else {
		// Non-neighbor keys go directly
		for k, v := range result {
			envelope[k] = v
		}
	}

	jsonData, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("json marshal: %w", err)
	}

	return string(jsonData), nil
}

// detectMessageType reads the BGP message type from the header.
func detectMessageType(data []byte) string {
	if len(data) < message.HeaderLen {
		return msgTypeUpdate
	}
	switch data[18] {
	case 1:
		return msgTypeOpen
	case 2:
		return msgTypeUpdate
	default:
		return msgTypeUpdate
	}
}

// makeEnvelope creates the ExaBGP-compatible envelope structure.
func makeEnvelope(msgType string) map[string]any {
	hostname, _ := os.Hostname()
	return map[string]any{
		"exabgp":  "5.0.0",
		"time":    float64(time.Now().UnixNano()) / 1e9,
		"host":    hostname,
		"pid":     os.Getpid(),
		"ppid":    os.Getppid(),
		"counter": 1,
		"type":    msgType,
		"neighbor": map[string]any{
			"address": map[string]any{
				"local": "127.0.0.1",
				"peer":  "127.0.0.1",
			},
			"asn": map[string]any{
				"local": 65533,
				"peer":  65533,
			},
			"direction": "in",
		},
	}
}

// decodeOpenMessage decodes a BGP OPEN message.
func decodeOpenMessage(data []byte, hasHeader bool, plugins []string) (map[string]any, error) {
	body := data
	if hasHeader {
		if len(data) < message.HeaderLen {
			return nil, fmt.Errorf("data too short for header")
		}
		body = data[message.HeaderLen:]
	}

	open, err := message.UnpackOpen(body)
	if err != nil {
		return nil, fmt.Errorf("unpack open: %w", err)
	}

	// Parse capabilities
	caps := parseCapabilities(open.OptionalParams)

	// Determine ASN (use ASN4 if available)
	asn := uint32(open.MyAS)
	for _, c := range caps {
		if asn4, ok := c.(*capability.ASN4); ok {
			asn = asn4.ASN
			break
		}
	}

	// Build capabilities JSON - keyed by capability code
	capsJSON := make(map[string]any)
	for _, c := range caps {
		capsJSON[fmt.Sprintf("%d", c.Code())] = capabilityToJSON(c, plugins)
	}

	neighbor := map[string]any{
		"open": map[string]any{
			"version":      open.Version,
			"asn":          asn,
			"hold_time":    open.HoldTime,
			"router_id":    open.RouterID(),
			"capabilities": capsJSON,
		},
	}

	return map[string]any{"neighbor": neighbor}, nil
}

// parseCapabilities parses optional parameters for capabilities.
func parseCapabilities(optParams []byte) []capability.Capability {
	var caps []capability.Capability
	offset := 0

	for offset < len(optParams) {
		if offset+2 > len(optParams) {
			break
		}

		paramType := optParams[offset]
		paramLen := int(optParams[offset+1])
		offset += 2

		if offset+paramLen > len(optParams) {
			break
		}

		// Capability parameter (type 2)
		if paramType == 2 {
			parsed, err := capability.Parse(optParams[offset : offset+paramLen])
			if err == nil {
				caps = append(caps, parsed...)
			}
		}
		offset += paramLen
	}

	return caps
}

// capabilityToJSON converts a capability to its JSON representation.
// Plugin-provided capabilities are decoded only when the plugin is specified.
func capabilityToJSON(c capability.Capability, plugins []string) map[string]any {
	switch cap := c.(type) {
	case *capability.Multiprotocol:
		return map[string]any{
			"name":     "multiprotocol",
			"families": []string{cap.AFI.String() + "/" + cap.SAFI.String()},
		}
	case *capability.ASN4:
		return map[string]any{
			"name": "asn4",
			"asn4": cap.ASN,
		}
	case *capability.RouteRefresh:
		return map[string]any{
			"name": "route-refresh",
		}
	case *capability.ExtendedMessage:
		return map[string]any{
			"name": "extended-message",
		}
	case *capability.AddPath:
		families := make([]string, len(cap.Families))
		for i, f := range cap.Families {
			families[i] = fmt.Sprintf("%s/%s", f.AFI.String(), f.SAFI.String())
		}
		return map[string]any{
			"name":     "add-path",
			"families": families,
		}
	case *capability.GracefulRestart:
		return map[string]any{
			"name":         "graceful-restart",
			"restart_time": cap.RestartTime,
		}
	case *capability.SoftwareVersion:
		return map[string]any{
			"name":     "software-version",
			"software": cap.Version,
		}
	default:
		return unknownCapability(c, plugins)
	}
}

// pluginCapabilityMap maps capability codes to plugin names.
// Plugins that can decode specific capabilities register here.
var pluginCapabilityMap = map[uint8]string{
	73: "hostname", // FQDN capability
}

// pluginFamilyMap maps address families to plugin names for CLI decode.
// Used by standalone `ze bgp decode` command which has no runtime registry.
// When engine is running, PluginRegistry.LookupFamily is used instead.
var pluginFamilyMap = map[string]string{
	"ipv4/flow":     "flowspec",
	"ipv6/flow":     "flowspec",
	"ipv4/flow-vpn": "flowspec",
	"ipv6/flow-vpn": "flowspec",
}

// unknownCapability returns JSON for an unrecognized/plugin-required capability.
// If a plugin is specified that can decode this capability, it will be invoked.
func unknownCapability(c capability.Capability, plugins []string) map[string]any {
	raw := c.Pack()
	// Pack() returns full TLV (code + length + value), extract just the value
	var rawHex string
	if len(raw) >= 2 {
		rawHex = fmt.Sprintf("%X", raw[2:])
	}

	// Check if a plugin can decode this capability
	pluginName, hasPlugin := pluginCapabilityMap[uint8(c.Code())]
	if hasPlugin && hasPluginEnabled(plugins, pluginName) {
		// Try plugin decode
		result := invokePluginDecode(pluginName, uint8(c.Code()), rawHex)
		if result != nil {
			return result
		}
	}

	// Fallback: return unknown with raw data
	return map[string]any{
		"name": "unknown",
		"code": int(c.Code()),
		"raw":  rawHex,
	}
}

// hasPluginEnabled checks if a plugin is in the enabled list.
// Accepts both "ze.name" and "name" formats.
func hasPluginEnabled(plugins []string, name string) bool {
	for _, p := range plugins {
		if p == name || p == "ze."+name {
			return true
		}
	}
	return false
}

// invokePluginDecodeRequest spawns a plugin in decode mode and sends a decode request.
// Returns decoded JSON map or nil if decoding failed.
// The request format varies: "decode capability <code> <hex>" or "decode nlri <family> <hex>".
func invokePluginDecodeRequest(pluginName, request string) map[string]any {
	// Build plugin command - pluginName comes from fixed maps (pluginCapabilityMap, pluginFamilyMap)
	args := []string{"bgp", "plugin", pluginName, "--decode"}

	// Create command with timeout context (short timeout for decode operation)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], args...) //nolint:gosec // pluginName from fixed map

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil
	}

	if err := cmd.Start(); err != nil {
		return nil
	}

	// Send decode request
	_, _ = stdin.Write([]byte(request + "\n"))
	_ = stdin.Close()

	// Read response
	scanner := bufio.NewScanner(stdout)
	var result map[string]any
	if scanner.Scan() {
		line := scanner.Text()
		// Parse: "decoded json <json>" or "decoded unknown"
		if strings.HasPrefix(line, "decoded json ") {
			jsonStr := strings.TrimPrefix(line, "decoded json ")
			if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
				_ = cmd.Wait()
				return result
			}
		}
	}

	_ = cmd.Wait()
	return nil
}

// invokePluginDecode spawns a plugin in decode mode and requests capability decoding.
// Returns decoded JSON map or nil if decoding failed.
func invokePluginDecode(pluginName string, code uint8, hexData string) map[string]any {
	request := fmt.Sprintf("decode capability %d %s", code, hexData)
	return invokePluginDecodeRequest(pluginName, request)
}

// invokePluginNLRIDecode spawns a plugin in decode mode and requests NLRI decoding.
// Returns decoded JSON map or nil if decoding failed.
func invokePluginNLRIDecode(pluginName, family, hexData string) map[string]any {
	request := fmt.Sprintf("decode nlri %s %s", family, hexData)
	return invokePluginDecodeRequest(pluginName, request)
}

// lookupFamilyPlugin returns the plugin name for a family.
// For families in pluginFamilyMap, the plugin is auto-invoked without requiring --plugin flag.
// Family string is normalized to lowercase for lookup.
func lookupFamilyPlugin(family string, _ []string) string {
	if pluginName, ok := pluginFamilyMap[strings.ToLower(family)]; ok {
		return pluginName
	}
	return ""
}

// decodeUpdateMessage decodes a BGP UPDATE message.
func decodeUpdateMessage(data []byte, _ string, hasHeader bool) (map[string]any, error) {
	body := data
	if hasHeader {
		if len(data) < message.HeaderLen {
			return nil, fmt.Errorf("data too short for header")
		}
		body = data[message.HeaderLen:]
	}

	update, err := message.UnpackUpdate(body)
	if err != nil {
		return nil, fmt.Errorf("unpack update: %w", err)
	}

	// Build message section
	updateContent := make(map[string]any)

	// Parse path attributes
	attrs, mpReach, mpUnreach := parsePathAttributes(update.PathAttributes)

	// Extract and remove internal next-hop field (used for announce section key)
	nextHop := "0.0.0.0"
	if nh, ok := attrs["_next-hop"].(string); ok {
		nextHop = nh
		delete(attrs, "_next-hop")
	}
	_ = nextHop // Used below for NLRI

	if len(attrs) > 0 {
		updateContent["attribute"] = attrs
	}

	// Handle MP_REACH_NLRI (announcements)
	if mpReach != nil {
		announceSection := buildMPReachSection(mpReach)
		if len(announceSection) > 0 {
			updateContent["announce"] = announceSection
		}
	}

	// Handle MP_UNREACH_NLRI (withdrawals)
	if mpUnreach != nil {
		withdrawSection := buildMPUnreachSection(mpUnreach)
		if len(withdrawSection) > 0 {
			updateContent["withdraw"] = withdrawSection
		}
	}

	// Handle IPv4 withdrawn routes
	if len(update.WithdrawnRoutes) > 0 {
		prefixes := parseIPv4Prefixes(update.WithdrawnRoutes)
		if len(prefixes) > 0 {
			if updateContent["withdraw"] == nil {
				updateContent["withdraw"] = make(map[string]any)
			}
			if withdraw, ok := updateContent["withdraw"].(map[string]any); ok {
				withdraw["ipv4/unicast"] = prefixes
			}
		}
	}

	// Handle IPv4 NLRI (announcements)
	if len(update.NLRI) > 0 {
		prefixes := parseIPv4Prefixes(update.NLRI)
		nlriList := make([]map[string]any, len(prefixes))
		for i, p := range prefixes {
			nlriList[i] = map[string]any{"nlri": p}
		}
		if updateContent["announce"] == nil {
			updateContent["announce"] = make(map[string]any)
		}
		if announce, ok := updateContent["announce"].(map[string]any); ok {
			announce["ipv4/unicast"] = map[string]any{
				nextHop: nlriList,
			}
		}
	}

	neighbor := map[string]any{
		"message": map[string]any{
			"update": updateContent,
		},
	}

	return map[string]any{"neighbor": neighbor}, nil
}

// parsePathAttributes parses path attributes and extracts MP_REACH/MP_UNREACH.
func parsePathAttributes(data []byte) (attrs map[string]any, mpReach, mpUnreach []byte) {
	attrs = make(map[string]any)
	offset := 0

	for offset < len(data) {
		if offset+2 > len(data) {
			break
		}

		flags := data[offset]
		code := data[offset+1]

		// Determine header length and value length
		hdrLen := 3
		var valueLen int
		if flags&0x10 != 0 { // Extended length
			if offset+4 > len(data) {
				break
			}
			valueLen = int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
			hdrLen = 4
		} else {
			if offset+3 > len(data) {
				break
			}
			valueLen = int(data[offset+2])
		}

		if offset+hdrLen+valueLen > len(data) {
			break
		}

		value := data[offset+hdrLen : offset+hdrLen+valueLen]

		switch code {
		case 1: // ORIGIN
			if len(value) >= 1 {
				origins := []string{"igp", "egp", "incomplete"}
				if int(value[0]) < len(origins) {
					attrs["origin"] = origins[value[0]]
				}
			}
		case 2: // AS_PATH
			asPath := parseASPath(value)
			if len(asPath) > 0 {
				attrs["as-path"] = asPath
			}
		case 3: // NEXT_HOP - stored separately, not in attributes for ExaBGP format
			if len(value) == 4 {
				attrs["_next-hop"] = fmt.Sprintf("%d.%d.%d.%d", value[0], value[1], value[2], value[3])
			}
		case 4: // MED
			if len(value) == 4 {
				attrs["med"] = binary.BigEndian.Uint32(value)
			}
		case 5: // LOCAL_PREF
			if len(value) == 4 {
				attrs["local-preference"] = binary.BigEndian.Uint32(value)
			}
		case 6: // ATOMIC_AGGREGATE
			attrs["atomic-aggregate"] = true
		case 7: // AGGREGATOR
			if len(value) == 6 {
				// 2-byte ASN + 4-byte IP
				asn := binary.BigEndian.Uint16(value[0:2])
				ip := fmt.Sprintf("%d.%d.%d.%d", value[2], value[3], value[4], value[5])
				attrs["aggregator"] = fmt.Sprintf("%d:%s", asn, ip)
			} else if len(value) == 8 {
				// 4-byte ASN + 4-byte IP
				asn := binary.BigEndian.Uint32(value[0:4])
				ip := fmt.Sprintf("%d.%d.%d.%d", value[4], value[5], value[6], value[7])
				attrs["aggregator"] = fmt.Sprintf("%d:%s", asn, ip)
			}
		case 9: // ORIGINATOR_ID
			if len(value) == 4 {
				attrs["originator-id"] = fmt.Sprintf("%d.%d.%d.%d", value[0], value[1], value[2], value[3])
			}
		case 10: // CLUSTER_LIST
			var clusters []string
			for i := 0; i+4 <= len(value); i += 4 {
				clusters = append(clusters, fmt.Sprintf("%d.%d.%d.%d",
					value[i], value[i+1], value[i+2], value[i+3]))
			}
			if len(clusters) > 0 {
				attrs["cluster-list"] = clusters
			}
		case 16: // EXTENDED_COMMUNITIES
			extComms := parseExtendedCommunities(value)
			if len(extComms) > 0 {
				attrs["extended-community"] = extComms
			}
		case 14: // MP_REACH_NLRI
			mpReach = value
		case 15: // MP_UNREACH_NLRI
			mpUnreach = value
		case 29: // BGP-LS Attribute (RFC 7752 Section 3.3)
			bgplsAttr := parseBGPLSAttribute(value)
			if len(bgplsAttr) > 0 {
				attrs["bgp-ls"] = bgplsAttr
			}
		}

		offset += hdrLen + valueLen
	}

	return attrs, mpReach, mpUnreach
}

// parseExtendedCommunities parses extended communities (type 16).
// Each extended community is 8 bytes.
func parseExtendedCommunities(data []byte) []map[string]any {
	var comms []map[string]any

	for len(data) >= 8 {
		// Read 8-byte extended community
		value := binary.BigEndian.Uint64(data[:8])
		typeHigh := data[0]
		typeLow := data[1]

		comm := map[string]any{
			"value": value,
		}

		// Parse based on type
		switch {
		case typeHigh == 0x80 && typeLow == 0x06:
			// Traffic-rate (FlowSpec)
			rate := binary.BigEndian.Uint32(data[4:8])
			comm["string"] = fmt.Sprintf("rate-limit:%d", rate)
		case typeHigh == 0x80 && typeLow == 0x07:
			// Traffic-action (FlowSpec)
			comm["string"] = "traffic-action"
		case typeHigh == 0x80 && typeLow == 0x08:
			// Redirect (FlowSpec)
			asn := binary.BigEndian.Uint16(data[2:4])
			localAdmin := binary.BigEndian.Uint32(data[4:8])
			comm["string"] = fmt.Sprintf("redirect:%d:%d", asn, localAdmin)
		case typeHigh == 0x80 && typeLow == 0x09:
			// Traffic-marking (FlowSpec)
			dscp := data[7]
			comm["string"] = fmt.Sprintf("mark:%d", dscp)
		case typeHigh == 0x00 && typeLow == 0x02:
			// Route Target
			asn := binary.BigEndian.Uint16(data[2:4])
			localAdmin := binary.BigEndian.Uint32(data[4:8])
			comm["string"] = fmt.Sprintf("target:%d:%d", asn, localAdmin)
		case typeHigh == 0x00 && typeLow == 0x03:
			// Route Origin
			asn := binary.BigEndian.Uint16(data[2:4])
			localAdmin := binary.BigEndian.Uint32(data[4:8])
			comm["string"] = fmt.Sprintf("origin:%d:%d", asn, localAdmin)
		default:
			// Generic format
			comm["string"] = fmt.Sprintf("0x%02x%02x:%x", typeHigh, typeLow, data[2:8])
		}

		comms = append(comms, comm)
		data = data[8:]
	}

	return comms
}

// parseASPath parses AS_PATH attribute value into ExaBGP object format.
// ExaBGP format: {"0": {"element": "as-sequence", "value": [asn1, asn2, ...]}, ...}.
func parseASPath(data []byte) map[string]any {
	result := make(map[string]any)
	offset := 0
	segIndex := 0

	for offset < len(data) {
		if offset+2 > len(data) {
			break
		}

		segType := data[offset]
		segLen := int(data[offset+1])
		offset += 2

		// Try 4-byte ASNs first, then 2-byte
		asnSize := 4
		if offset+segLen*4 > len(data) {
			asnSize = 2
		}
		if offset+segLen*asnSize > len(data) {
			break
		}

		var asns []uint32
		for i := 0; i < segLen; i++ {
			var asn uint32
			if asnSize == 4 {
				asn = binary.BigEndian.Uint32(data[offset : offset+4])
			} else {
				asn = uint32(binary.BigEndian.Uint16(data[offset : offset+2]))
			}
			asns = append(asns, asn)
			offset += asnSize
		}

		// Determine segment type name
		var elemType string
		switch segType {
		case 1:
			elemType = "as-set"
		case 2:
			elemType = "as-sequence"
		default:
			elemType = fmt.Sprintf("as-type-%d", segType)
		}

		result[fmt.Sprintf("%d", segIndex)] = map[string]any{
			"element": elemType,
			"value":   asns,
		}
		segIndex++
	}

	return result
}

// parseIPv4Prefixes parses IPv4 NLRI prefixes.
func parseIPv4Prefixes(data []byte) []string {
	var prefixes []string
	offset := 0

	for offset < len(data) {
		if offset >= len(data) {
			break
		}

		prefixLen := int(data[offset])
		offset++

		byteLen := (prefixLen + 7) / 8
		if offset+byteLen > len(data) {
			break
		}

		prefixBytes := make([]byte, 4)
		copy(prefixBytes, data[offset:offset+byteLen])

		prefix := fmt.Sprintf("%d.%d.%d.%d/%d",
			prefixBytes[0], prefixBytes[1], prefixBytes[2], prefixBytes[3], prefixLen)
		prefixes = append(prefixes, prefix)

		offset += byteLen
	}

	return prefixes
}

// buildMPReachSection builds the announce section from MP_REACH_NLRI.
func buildMPReachSection(mpReach []byte) map[string]any {
	if len(mpReach) < 5 {
		return nil
	}

	afi := nlri.AFI(binary.BigEndian.Uint16(mpReach[0:2]))
	safi := nlri.SAFI(mpReach[2])
	nhLen := int(mpReach[3])

	if len(mpReach) < 4+nhLen+1 {
		return nil
	}

	// Parse next-hop
	nhData := mpReach[4 : 4+nhLen]
	nextHop := parseNextHop(nhData, afi)

	// Skip reserved byte
	nlriOffset := 4 + nhLen + 1
	if nlriOffset >= len(mpReach) {
		return nil
	}

	nlriData := mpReach[nlriOffset:]
	familyKey := formatFamily(afi, safi)

	// Parse NLRI based on family
	routes := parseNLRIByFamily(nlriData, afi, safi, false)

	if len(routes) == 0 {
		return nil
	}

	return map[string]any{
		familyKey: map[string]any{
			nextHop: routes,
		},
	}
}

// buildMPUnreachSection builds the withdraw section from MP_UNREACH_NLRI.
func buildMPUnreachSection(mpUnreach []byte) map[string]any {
	if len(mpUnreach) < 3 {
		return nil
	}

	afi := nlri.AFI(binary.BigEndian.Uint16(mpUnreach[0:2]))
	safi := nlri.SAFI(mpUnreach[2])

	if len(mpUnreach) <= 3 {
		return nil
	}

	nlriData := mpUnreach[3:]
	familyKey := formatFamily(afi, safi)

	// Parse NLRI based on family
	routes := parseNLRIByFamily(nlriData, afi, safi, true)

	if len(routes) == 0 {
		return nil
	}

	return map[string]any{
		familyKey: routes,
	}
}

// parseNextHop parses the next-hop from MP_REACH_NLRI.
func parseNextHop(data []byte, _ nlri.AFI) string {
	switch {
	case len(data) == 4:
		return fmt.Sprintf("%d.%d.%d.%d", data[0], data[1], data[2], data[3])
	case len(data) == 16:
		addr := netip.AddrFrom16([16]byte(data))
		return addr.String()
	case len(data) == 32: // IPv6 with link-local
		addr := netip.AddrFrom16([16]byte(data[:16]))
		return addr.String()
	case len(data) == 0:
		return "no-nexthop"
	default:
		return fmt.Sprintf("%x", data)
	}
}

// formatFamily returns the family string for JSON output.
func formatFamily(afi nlri.AFI, safi nlri.SAFI) string {
	// Use afi/safi format
	return nlri.Family{AFI: afi, SAFI: safi}.String()
}

// parseNLRIByFamily parses NLRI based on address family.
func parseNLRIByFamily(data []byte, afi nlri.AFI, safi nlri.SAFI, _ bool) []any {
	var routes []any

	switch {
	case afi == nlri.AFIL2VPN && safi == nlri.SAFIEVPN:
		routes = parseEVPNRoutes(data)
	case safi == nlri.SAFIFlowSpec || safi == nlri.SAFIFlowSpecVPN:
		// FlowSpec decoding delegated to plugin
		family := nlri.Family{AFI: afi, SAFI: safi}.String()
		hexData := fmt.Sprintf("%X", data)
		result := invokePluginNLRIDecode("flowspec", family, hexData)
		if result != nil {
			routes = []any{result}
		} else {
			// Plugin failed or unavailable - return raw bytes
			routes = []any{map[string]any{"parsed": false, "raw": hexData}}
		}
	case afi == nlri.AFIBGPLS:
		routes = parseBGPLSRoutes(data)
	default:
		// Generic prefix parsing
		routes = parseGenericNLRI(data, afi)
	}

	return routes
}

// parseEVPNRoutes parses EVPN NLRI with lenient label handling.
// The standard nlri.ParseEVPN fails on labels without bottom-of-stack bit,
// so we implement custom parsing here for decode command.
func parseEVPNRoutes(data []byte) []any {
	var routes []any
	remaining := data

	for len(remaining) >= 2 {
		routeType := remaining[0]
		routeLen := int(remaining[1])

		if len(remaining) < 2+routeLen {
			routes = append(routes, map[string]any{
				"code":   int(routeType),
				"parsed": false,
				"raw":    fmt.Sprintf("%X", remaining),
			})
			break
		}

		routeData := remaining[:2+routeLen]
		routeBody := remaining[2 : 2+routeLen]

		// Parse based on route type with lenient handling
		route := parseEVPNRouteLenient(int(routeType), routeBody, routeData)
		routes = append(routes, route)

		remaining = remaining[2+routeLen:]
	}

	return routes
}

// parseEVPNRouteLenient parses EVPN routes with lenient error handling.
func parseEVPNRouteLenient(routeType int, body, fullData []byte) map[string]any {
	result := map[string]any{
		"code": routeType,
		"raw":  fmt.Sprintf("%X", fullData),
	}

	// Minimum sizes: RD(8) + varies by type
	switch routeType {
	case 2: // MAC/IP Advertisement - most common
		if len(body) < 30 { // RD(8)+ESI(10)+ETag(4)+MACLen(1)+MAC(6)+IPLen(1)
			result["parsed"] = false
			return result
		}
		return parseEVPNType2Lenient(body, result)

	case 1, 3, 4, 5:
		// Try standard parser first, fall back to unparsed
		parsed, _, err := nlri.ParseEVPN(fullData, false)
		if err == nil {
			return evpnToJSON(parsed)
		}
		result["parsed"] = false
		return result

	default:
		result["parsed"] = false
		return result
	}
}

// parseEVPNType2Lenient parses EVPN Type 2 (MAC/IP) with lenient label handling.
func parseEVPNType2Lenient(data []byte, result map[string]any) map[string]any {
	offset := 0

	// RD (8 bytes)
	if offset+8 > len(data) {
		result["parsed"] = false
		return result
	}
	rd, err := nlri.ParseRouteDistinguisher(data[offset : offset+8])
	if err != nil {
		result["parsed"] = false
		return result
	}
	result["rd"] = rd.String()
	offset += 8

	// ESI (10 bytes)
	if offset+10 > len(data) {
		result["parsed"] = false
		return result
	}
	var esi nlri.ESI
	copy(esi[:], data[offset:offset+10])
	result["esi"] = formatESI(esi)
	offset += 10

	// Ethernet Tag (4 bytes)
	if offset+4 > len(data) {
		result["parsed"] = false
		return result
	}
	result["ethernet-tag"] = binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	// MAC Length (1 byte)
	if offset+1 > len(data) {
		result["parsed"] = false
		return result
	}
	macLen := data[offset]
	offset++
	if macLen != 48 {
		result["parsed"] = false
		return result
	}

	// MAC (6 bytes)
	if offset+6 > len(data) {
		result["parsed"] = false
		return result
	}
	var mac [6]byte
	copy(mac[:], data[offset:offset+6])
	result["mac"] = formatMAC(mac)
	offset += 6

	// IP Length (1 byte)
	if offset+1 > len(data) {
		result["parsed"] = false
		return result
	}
	ipLen := data[offset]
	offset++

	// IP (optional)
	switch ipLen {
	case 0:
		// No IP
	case 32:
		if offset+4 > len(data) {
			result["parsed"] = false
			return result
		}
		ip := netip.AddrFrom4([4]byte(data[offset : offset+4]))
		result["ip"] = ip.String()
		offset += 4
	case 128:
		if offset+16 > len(data) {
			result["parsed"] = false
			return result
		}
		ip := netip.AddrFrom16([16]byte(data[offset : offset+16]))
		result["ip"] = ip.String()
		offset += 16
	default:
		result["parsed"] = false
		return result
	}

	// Labels - lenient parsing (just read available 3-byte chunks)
	if offset < len(data) {
		labels := parseLabelStackLenient(data[offset:])
		result["label"] = labels
	} else {
		result["label"] = [][]int{{0}}
	}

	result["parsed"] = true
	result["name"] = "MAC/IP advertisement"
	return result
}

// parseLabelStackLenient parses labels without requiring bottom-of-stack bit.
func parseLabelStackLenient(data []byte) [][]int {
	var labels [][]int
	for len(data) >= 3 {
		labelVal := int(data[0])<<12 | int(data[1])<<4 | int(data[2]>>4)
		labels = append(labels, []int{labelVal})
		data = data[3:]
	}
	if len(labels) == 0 {
		return [][]int{{0}}
	}
	return labels
}

// evpnToJSON converts an EVPN NLRI to ExaBGP JSON format.
func evpnToJSON(n nlri.NLRI) map[string]any {
	evpn, ok := n.(nlri.EVPN)
	if !ok {
		return map[string]any{
			"parsed": false,
			"raw":    fmt.Sprintf("%X", n.Bytes()),
		}
	}

	// Check if this is a generic (unparsed) EVPN route
	if _, isGeneric := n.(*nlri.EVPNGeneric); isGeneric {
		return map[string]any{
			"code":   int(evpn.RouteType()),
			"parsed": false,
			"raw":    fmt.Sprintf("%X", n.Bytes()),
		}
	}

	result := map[string]any{
		"code":   int(evpn.RouteType()),
		"parsed": true,
		"raw":    fmt.Sprintf("%X", n.Bytes()),
		"name":   evpn.RouteType().String(),
		"rd":     evpn.RD().String(),
	}

	switch r := n.(type) {
	case *nlri.EVPNType1:
		result["esi"] = formatESI(r.ESI())
		result["ethernet-tag"] = r.EthernetTag()
		result["label"] = formatLabels(r.Labels())

	case *nlri.EVPNType2:
		result["esi"] = formatESI(r.ESI())
		result["ethernet-tag"] = r.EthernetTag()
		result["mac"] = formatMAC(r.MAC())
		if r.IP().IsValid() {
			result["ip"] = r.IP().String()
		}
		result["label"] = formatLabels(r.Labels())

	case *nlri.EVPNType3:
		result["ethernet-tag"] = r.EthernetTag()
		result["originator"] = r.OriginatorIP().String()

	case *nlri.EVPNType4:
		result["esi"] = formatESI(r.ESI())
		result["originator"] = r.OriginatorIP().String()

	case *nlri.EVPNType5:
		result["esi"] = formatESI(r.ESI())
		result["ethernet-tag"] = r.EthernetTag()
		result["prefix"] = r.Prefix().String()
		if r.Gateway().IsValid() && !r.Gateway().IsUnspecified() {
			result["gateway"] = r.Gateway().String()
		}
		result["label"] = formatLabels(r.Labels())
	}

	return result
}

// formatESI formats an ESI for JSON output.
func formatESI(esi nlri.ESI) string {
	if esi.IsZero() {
		return "-"
	}
	return esi.String()
}

// formatMAC formats a MAC address for JSON output.
func formatMAC(mac [6]byte) string {
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
		mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
}

// formatLabels formats MPLS labels for JSON output.
func formatLabels(labels []uint32) [][]int {
	if len(labels) == 0 {
		return [][]int{{0}}
	}
	result := make([][]int, len(labels))
	for i, l := range labels {
		result[i] = []int{int(l)}
	}
	return result
}

// parseBGPLSRoutes parses BGP-LS NLRI.
func parseBGPLSRoutes(data []byte) []any {
	var routes []any

	// BGP-LS parser parses one NLRI at a time
	parsed, err := nlri.ParseBGPLS(data)
	if err != nil {
		routes = append(routes, map[string]any{
			"parsed": false,
			"raw":    fmt.Sprintf("%X", data),
		})
		return routes
	}

	route := bgplsToJSON(parsed)
	routes = append(routes, route)

	return routes
}

// bgplsToJSON converts a BGP-LS NLRI to ExaBGP-compatible JSON format.
// RFC 7752 defines the NLRI structure; ExaBGP uses specific field names.
func bgplsToJSON(n nlri.BGPLSNLRI) map[string]any {
	result := map[string]any{
		"ls-nlri-type":        bgplsNLRITypeString(uint16(n.NLRIType())),
		"l3-routing-topology": n.Identifier(),
		"protocol-id":         int(n.ProtocolID()),
	}

	// Type-specific fields based on NLRI type
	switch v := n.(type) {
	case *nlri.BGPLSNode:
		// Parse TLVs from wire bytes to include only TLVs actually present
		result["node-descriptors"] = parseBGPLSNodeTLVs(v.Bytes())

	case *nlri.BGPLSLink:
		// Parse TLVs from wire bytes to extract all fields
		localDescs, remoteDescs, linkInfo := parseBGPLSLinkTLVs(v.Bytes())
		result["local-node-descriptors"] = localDescs
		result["remote-node-descriptors"] = remoteDescs
		result["interface-addresses"] = linkInfo.ifAddrs
		result["neighbor-addresses"] = linkInfo.neighAddrs
		result["multi-topology-ids"] = linkInfo.mtIDs
		result["link-identifiers"] = linkInfo.linkIDs

	case *nlri.BGPLSPrefix:
		// Parse TLVs from wire bytes to include only TLVs actually present
		nodeDescs, prefixInfo := parseBGPLSPrefixTLVs(v.Bytes(), v.NLRIType())
		result["node-descriptors"] = nodeDescs
		if prefixInfo.prefix != "" {
			// RFC 7752 Section 3.2.3.2: TLV 265 contains prefix-length + prefix-bytes
			// Both fields include /prefix-length since that's what the TLV contains.
			// ExaBGP historically showed ip-reachability-tlv without /prefix but this
			// is being corrected to match the actual TLV content.
			result["ip-reachability-tlv"] = prefixInfo.prefix
			result["ip-reach-prefix"] = prefixInfo.prefix
		}
		result["multi-topology-ids"] = prefixInfo.mtIDs

	case *nlri.BGPLSSRv6SID:
		// Parse TLVs from wire bytes to include only TLVs actually present
		result["node-descriptors"] = parseBGPLSNodeTLVs(v.Bytes())
		if len(v.SRv6SID.SRv6SID) > 0 {
			result["srv6-sid"] = formatIPv6Compressed(v.SRv6SID.SRv6SID)
		}
	}

	return result
}

// bgplsNLRITypeString returns the ExaBGP-style NLRI type string.
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
	default:
		return fmt.Sprintf("bgpls-type-%d", nlriType)
	}
}

// formatNodeDescriptors converts a NodeDescriptor to ExaBGP array format.
// Each sub-TLV becomes a separate object in the array per ExaBGP convention.
// Note: Only includes fields that have non-zero values (TLVs actually present).
func formatNodeDescriptors(nd *nlri.NodeDescriptor) []any {
	var descs []any

	// RFC 7752 Section 3.2.1.4 - Autonomous System (TLV 512)
	if nd.ASN != 0 {
		descs = append(descs, map[string]any{"autonomous-system": nd.ASN})
	}

	// RFC 7752 Section 3.2.1.4 - BGP-LS Identifier (TLV 513)
	// Only add if non-zero (TLV was present)
	if nd.BGPLSIdentifier != 0 {
		descs = append(descs, map[string]any{"bgp-ls-identifier": fmt.Sprintf("%d", nd.BGPLSIdentifier)})
	}

	// RFC 7752 Section 3.2.1.4 - OSPF Area-ID (TLV 514)
	// Only add if non-zero (TLV was present)
	if nd.OSPFAreaID != 0 {
		descs = append(descs, map[string]any{
			"ospf-area-id": fmt.Sprintf("%d.%d.%d.%d",
				(nd.OSPFAreaID>>24)&0xFF,
				(nd.OSPFAreaID>>16)&0xFF,
				(nd.OSPFAreaID>>8)&0xFF,
				nd.OSPFAreaID&0xFF),
		})
	}

	// RFC 7752 Section 3.2.1.4 - IGP Router-ID (TLV 515)
	if len(nd.IGPRouterID) > 0 {
		descs = append(descs, map[string]any{"router-id": formatRouterID(nd.IGPRouterID)})
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
		// OSPF Router-ID: dotted decimal
		return fmt.Sprintf("%d.%d.%d.%d", id[0], id[1], id[2], id[3])
	case 6:
		// IS-IS System ID: hex digits (ExaBGP shows as decimal string)
		return fmt.Sprintf("%02x%02x%02x%02x%02x%02x", id[0], id[1], id[2], id[3], id[4], id[5])
	case 7:
		// IS-IS pseudonode: System ID + PSN
		return fmt.Sprintf("%02x%02x%02x%02x%02x%02x%02x", id[0], id[1], id[2], id[3], id[4], id[5], id[6])
	case 8:
		// OSPF pseudonode: Router-ID + interface address
		routerID := fmt.Sprintf("%d.%d.%d.%d", id[0], id[1], id[2], id[3])
		ifAddr := fmt.Sprintf("%d.%d.%d.%d", id[4], id[5], id[6], id[7])
		return routerID + "," + ifAddr
	default:
		// Unknown format: hex
		return fmt.Sprintf("%X", id)
	}
}

// formatIPReachability formats the IP Reachability Information TLV.
// RFC 7752 Section 3.2.3.2 defines the format: prefix-length + prefix-bytes.
func formatIPReachability(data []byte, nlriType nlri.BGPLSNLRIType) string {
	if len(data) < 1 {
		return ""
	}

	prefixLen := int(data[0])
	byteLen := (prefixLen + 7) / 8

	if len(data) < 1+byteLen {
		return ""
	}

	prefixBytes := data[1 : 1+byteLen]

	if nlriType == nlri.BGPLSPrefixV6NLRI {
		// IPv6 prefix
		addr := make([]byte, 16)
		copy(addr, prefixBytes)
		return formatIPv6Compressed(addr) + "/" + fmt.Sprintf("%d", prefixLen)
	}

	// IPv4 prefix
	addr := make([]byte, 4)
	copy(addr, prefixBytes)
	return fmt.Sprintf("%d.%d.%d.%d/%d", addr[0], addr[1], addr[2], addr[3], prefixLen)
}

// prefixDescriptorInfo holds parsed prefix descriptor information.
type prefixDescriptorInfo struct {
	prefix string
	mtIDs  []any
}

// parseBGPLSPrefixTLVs parses a Prefix NLRI wire format to extract all fields.
// RFC 7752 Section 3.2.3 defines the Prefix NLRI format.
func parseBGPLSPrefixTLVs(data []byte, nlriType nlri.BGPLSNLRIType) (nodeDescs []any, info prefixDescriptorInfo) {
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

		offset += 4 + tlvLen
	}

	return nodeDescs, info
}

// parseBGPLSNodeTLVs parses a Node NLRI wire format to extract node descriptors.
// RFC 7752 Section 3.2.1 defines the Node NLRI format:
// - NLRI Type (2) + Length (2) + Protocol-ID (1) + Identifier (8)
// - Local Node Descriptors TLV (256).
func parseBGPLSNodeTLVs(data []byte) []any {
	// Minimum: Type(2) + Len(2) + ProtoID(1) + Identifier(8) = 13 bytes
	if len(data) < 13 {
		return []any{}
	}

	// Skip NLRI header: Type(2) + Length(2) = 4 bytes
	// Then Protocol-ID(1) + Identifier(8) = 9 bytes
	// Total: 13 bytes before TLVs
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
// RFC 7752 Section 3.2.2 defines the Link NLRI format:
// - NLRI Type (2) + Length (2) + Protocol-ID (1) + Identifier (8)
// - Local Node Descriptors TLV (256)
// - Remote Node Descriptors TLV (257)
// - Link Descriptor TLVs (258-263).
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

	// Skip NLRI header: Type(2) + Length(2) = 4 bytes
	// Then Protocol-ID(1) + Identifier(8) = 9 bytes
	// Total: 13 bytes before TLVs
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
			// 2 bytes per MT-ID, high 4 bits reserved
			for i := 0; i+2 <= len(value); i += 2 {
				mtID := binary.BigEndian.Uint16(value[i:i+2]) & 0x0FFF
				info.mtIDs = append(info.mtIDs, int(mtID))
			}
		}

		offset += 4 + tlvLen
	}

	return localDescs, remoteDescs, info
}

// parseNodeDescriptorSubTLVs parses node descriptor sub-TLVs into ExaBGP format.
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
			default:
				descs = append(descs, map[string]any{"router-id": routerID})
			}
		}

		offset += 4 + tlvLen
	}

	return descs
}

// parseBGPLSAttribute parses BGP-LS attribute (type 29) TLVs.
// RFC 7752 Section 3.3 defines the attribute format and TLV types.
func parseBGPLSAttribute(data []byte) map[string]any {
	result := make(map[string]any)
	offset := 0

	for offset+4 <= len(data) {
		tlvType := binary.BigEndian.Uint16(data[offset : offset+2])
		tlvLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))

		if offset+4+tlvLen > len(data) {
			break
		}

		value := data[offset+4 : offset+4+tlvLen]

		switch tlvType {
		// Node Attribute TLVs (RFC 7752 Section 3.3.1)
		case 1024: // Node Flag Bits
			if len(value) >= 1 {
				flags := value[0]
				result["node-flags"] = map[string]any{
					"O":   (flags >> 7) & 1,
					"T":   (flags >> 6) & 1,
					"E":   (flags >> 5) & 1,
					"B":   (flags >> 4) & 1,
					"R":   (flags >> 3) & 1,
					"V":   (flags >> 2) & 1,
					"RSV": flags & 0x03,
				}
			}
		case 1026: // Node Name
			result["node-name"] = string(value)
		case 1027: // IS-IS Area Identifier
			// Output as hex with 0x prefix - ExaBGP accepts both decimal and 0x-prefixed hex
			result["area-id"] = fmt.Sprintf("0x%X", value)
		case 1028: // IPv4 Router-ID Local
			if len(value) == 4 {
				// Append to local-router-ids array
				addr := fmt.Sprintf("%d.%d.%d.%d", value[0], value[1], value[2], value[3])
				if existing, ok := result["local-router-ids"].([]string); ok {
					result["local-router-ids"] = append(existing, addr)
				} else {
					result["local-router-ids"] = []string{addr}
				}
			}
		case 1029: // IPv6 Router-ID Local
			if len(value) == 16 {
				addr := formatIPv6Compressed(value)
				if existing, ok := result["local-router-ids"].([]string); ok {
					result["local-router-ids"] = append(existing, addr)
				} else {
					result["local-router-ids"] = []string{addr}
				}
			}

		// Link Attribute TLVs (RFC 7752 Section 3.3.2)
		case 1030: // IPv4 Router-ID Remote
			if len(value) == 4 {
				addr := fmt.Sprintf("%d.%d.%d.%d", value[0], value[1], value[2], value[3])
				if existing, ok := result["remote-router-ids"].([]string); ok {
					result["remote-router-ids"] = append(existing, addr)
				} else {
					result["remote-router-ids"] = []string{addr}
				}
			}
		case 1031: // IPv6 Router-ID Remote
			if len(value) == 16 {
				addr := formatIPv6Compressed(value)
				if existing, ok := result["remote-router-ids"].([]string); ok {
					result["remote-router-ids"] = append(existing, addr)
				} else {
					result["remote-router-ids"] = []string{addr}
				}
			}
		case 1088: // Administrative Group (color)
			if len(value) >= 4 {
				result["admin-group-mask"] = binary.BigEndian.Uint32(value)
			}
		case 1089: // Maximum Link Bandwidth
			if len(value) >= 4 {
				result["maximum-link-bandwidth"] = float64(math.Float32frombits(binary.BigEndian.Uint32(value)))
			}
		case 1090: // Max. Reservable Link Bandwidth
			if len(value) >= 4 {
				result["maximum-reservable-link-bandwidth"] = float64(math.Float32frombits(binary.BigEndian.Uint32(value)))
			}
		case 1091: // Unreserved Bandwidth (8 values)
			if len(value) >= 32 {
				bws := make([]float64, 8)
				for i := 0; i < 8; i++ {
					bws[i] = float64(math.Float32frombits(binary.BigEndian.Uint32(value[i*4:])))
				}
				result["unreserved-bandwidth"] = bws
			}
		case 1092: // TE Default Metric
			if len(value) >= 4 {
				result["te-metric"] = binary.BigEndian.Uint32(value)
			}
		case 1095: // IGP Metric
			switch len(value) {
			case 1:
				result["igp-metric"] = int(value[0] & 0x3F) // IS-IS small metric (6 bits)
			case 2:
				result["igp-metric"] = int(binary.BigEndian.Uint16(value)) // OSPF metric
			case 3:
				result["igp-metric"] = int(value[0])<<16 | int(value[1])<<8 | int(value[2]) // IS-IS wide
			default:
				if len(value) >= 4 {
					result["igp-metric"] = int(binary.BigEndian.Uint32(value))
				}
			}

		// Prefix Attribute TLVs (RFC 7752 Section 3.3.3)
		case 1155: // Prefix Metric
			if len(value) >= 4 {
				result["prefix-metric"] = binary.BigEndian.Uint32(value)
			}
		case 1170: // SR Prefix Attribute Flags
			if len(value) >= 1 {
				flags := value[0]
				result["sr-prefix-attribute-flags"] = map[string]any{
					"X":   (flags >> 7) & 1,
					"R":   (flags >> 6) & 1,
					"N":   (flags >> 5) & 1,
					"RSV": flags & 0x1F,
				}
			}

		// SRv6 Link Attribute TLVs (RFC 9514 Section 4)
		case 1099: // SR-MPLS Adjacency SID (RFC 9085)
			parseSRMPLSAdjSID(result, "sr-adj", value)

		case 1106: // SRv6 End.X SID
			sids := parseSRv6EndXSID(value, 0)
			appendSRv6SIDs(result, "srv6-endx", sids)

		case 1107: // IS-IS SRv6 LAN End.X SID
			sids := parseSRv6EndXSID(value, 6) // 6-byte IS-IS neighbor ID
			appendSRv6SIDs(result, "srv6-lan-endx-isis", sids)

		case 1108: // OSPFv3 SRv6 LAN End.X SID
			sids := parseSRv6EndXSID(value, 4) // 4-byte OSPFv3 neighbor ID
			appendSRv6SIDs(result, "srv6-lan-endx-ospf", sids)

		default:
			// Generic TLV - store as hex
			result[fmt.Sprintf("generic-lsid-%d", tlvType)] = []string{fmt.Sprintf("0x%X", value)}
		}

		offset += 4 + tlvLen
	}

	return result
}

// parseSRv6EndXSID parses SRv6 End.X SID or LAN End.X SID TLVs (RFC 9514 Section 4).
// neighborIDLen is 0 for End.X SID, 6 for IS-IS LAN End.X, 4 for OSPFv3 LAN End.X.
// Returns a slice of parsed SID entries.
func parseSRv6EndXSID(data []byte, neighborIDLen int) []map[string]any {
	var sids []map[string]any

	// Minimum: Behavior(2) + Flags(1) + Algo(1) + Weight(1) + Reserved(1) + NeighborID + SID(16)
	minLen := 6 + neighborIDLen + 16
	offset := 0

	for offset+minLen <= len(data) {
		behavior := binary.BigEndian.Uint16(data[offset : offset+2])
		flags := data[offset+2]
		algorithm := data[offset+3]
		weight := data[offset+4]
		// offset+5 is reserved

		sidOffset := offset + 6 + neighborIDLen
		if sidOffset+16 > len(data) {
			break
		}

		sid := data[sidOffset : sidOffset+16]

		entry := map[string]any{
			"behavior":  int(behavior),
			"algorithm": int(algorithm),
			"weight":    int(weight),
			"flags": map[string]any{
				"B":   int((flags >> 7) & 1),
				"S":   int((flags >> 6) & 1),
				"P":   int((flags >> 5) & 1),
				"RSV": int(flags & 0x1F),
			},
			"sid": formatIPv6Compressed(sid),
		}

		// Add neighbor ID if present (LAN End.X SID)
		if neighborIDLen > 0 {
			neighborID := data[offset+6 : offset+6+neighborIDLen]
			entry["neighbor-id"] = fmt.Sprintf("%X", neighborID)
		}

		// Parse sub-TLVs (SRv6 SID Structure)
		subTLVOffset := sidOffset + 16
		if subTLVOffset+4 <= len(data) {
			subTLVType := binary.BigEndian.Uint16(data[subTLVOffset : subTLVOffset+2])
			subTLVLen := int(binary.BigEndian.Uint16(data[subTLVOffset+2 : subTLVOffset+4]))

			if subTLVType == 1252 && subTLVLen == 4 && subTLVOffset+4+4 <= len(data) {
				// SRv6 SID Structure (RFC 9514 Section 8)
				structData := data[subTLVOffset+4 : subTLVOffset+8]
				entry["srv6-sid-structure"] = map[string]any{
					"loc_block_len": int(structData[0]),
					"loc_node_len":  int(structData[1]),
					"func_len":      int(structData[2]),
					"arg_len":       int(structData[3]),
				}
				offset = subTLVOffset + 4 + subTLVLen
			} else {
				offset = subTLVOffset
			}
		} else {
			offset = subTLVOffset
		}

		sids = append(sids, entry)
	}

	return sids
}

// appendSRv6SIDs appends SRv6 SID entries to the result map under the given key.
func appendSRv6SIDs(result map[string]any, key string, sids []map[string]any) {
	if len(sids) == 0 {
		return
	}
	if existing, ok := result[key].([]map[string]any); ok {
		result[key] = append(existing, sids...)
	} else {
		result[key] = sids
	}
}

// parseSRMPLSAdjSID parses SR-MPLS Adjacency SID TLV 1099 (RFC 9085 Section 2.2.1).
// Format: Flags(1) + Weight(1) + Reserved(2) + SID/Label(variable).
// When V=1 and L=1: 3-byte label. When V=0 and L=0: 4-byte index.
//
//nolint:unparam // key parameter for API consistency with other TLV parsers
func parseSRMPLSAdjSID(result map[string]any, key string, data []byte) {
	if len(data) < 4 {
		return
	}

	flags := data[0]
	weight := int(data[1])
	// data[2:4] is reserved

	// Parse flags: F(7), B(6), V(5), L(4), S(3), P(2), RSV(1), RSV(0)
	flagMap := map[string]any{
		"F":   int((flags >> 7) & 1),
		"B":   int((flags >> 6) & 1),
		"V":   int((flags >> 5) & 1),
		"L":   int((flags >> 4) & 1),
		"S":   int((flags >> 3) & 1),
		"P":   int((flags >> 2) & 1),
		"RSV": int(flags & 0x03),
	}

	vFlag := (flags >> 5) & 1
	lFlag := (flags >> 4) & 1

	sids := make([]int, 0)
	undecoded := make([]string, 0)
	sidData := data[4:]

	// Combine V and L flags: 0b00=index, 0b11=label, others=invalid
	flagCombo := (vFlag << 1) | lFlag
	for len(sidData) > 0 {
		switch flagCombo {
		case 0b11: // V=1, L=1: 3-byte label
			if len(sidData) < 3 {
				undecoded = append(undecoded, fmt.Sprintf("%X", sidData))
				sidData = nil
				continue
			}
			sid := (int(sidData[0]) << 16) | (int(sidData[1]) << 8) | int(sidData[2])
			sids = append(sids, sid)
			sidData = sidData[3:]
		case 0b00: // V=0, L=0: 4-byte index
			if len(sidData) < 4 {
				undecoded = append(undecoded, fmt.Sprintf("%X", sidData))
				sidData = nil
				continue
			}
			sid := int(binary.BigEndian.Uint32(sidData[:4]))
			sids = append(sids, sid)
			sidData = sidData[4:]
		default: // Invalid flag combination
			undecoded = append(undecoded, fmt.Sprintf("%X", sidData))
			sidData = nil
		}
	}

	entry := map[string]any{
		"flags":          flagMap,
		"sids":           sids,
		"weight":         weight,
		"undecoded-sids": undecoded,
	}

	// Accumulate multiple TLV instances into an array (proper JSON, no data loss)
	if existing, ok := result[key].([]map[string]any); ok {
		result[key] = append(existing, entry)
	} else {
		result[key] = []map[string]any{entry}
	}
}

// formatIPv6Compressed formats a 16-byte IPv6 address with zero compression.
func formatIPv6Compressed(addr []byte) string {
	if len(addr) != 16 {
		return fmt.Sprintf("%X", addr)
	}
	// Use netip for proper zero compression
	ip := netip.AddrFrom16([16]byte(addr))
	return ip.String()
}

// parseGenericNLRI parses generic NLRI (IPv4/IPv6 prefixes).
func parseGenericNLRI(data []byte, afi nlri.AFI) []any {
	var routes []any
	offset := 0

	for offset < len(data) {
		prefixLen := int(data[offset])
		offset++

		byteLen := (prefixLen + 7) / 8
		if offset+byteLen > len(data) {
			break
		}

		var prefix string
		if afi == nlri.AFIIPv6 {
			prefixBytes := make([]byte, 16)
			copy(prefixBytes, data[offset:offset+byteLen])
			addr := netip.AddrFrom16([16]byte(prefixBytes))
			prefix = fmt.Sprintf("%s/%d", addr, prefixLen)
		} else {
			prefixBytes := make([]byte, 4)
			copy(prefixBytes, data[offset:offset+byteLen])
			prefix = fmt.Sprintf("%d.%d.%d.%d/%d",
				prefixBytes[0], prefixBytes[1], prefixBytes[2], prefixBytes[3], prefixLen)
		}

		routes = append(routes, map[string]any{"nlri": prefix})
		offset += byteLen
	}

	return routes
}

// decodeNLRIOnly decodes NLRI without envelope.
// If a matching plugin is enabled, it will be invoked for decoding.
// If outputJSON is false, returns human-readable format.
func decodeNLRIOnly(data []byte, family string, plugins []string, outputJSON bool) (string, error) {
	// Try plugin decode first if plugin is enabled for this family
	pluginName := lookupFamilyPlugin(family, plugins)
	if pluginName != "" {
		hexData := fmt.Sprintf("%X", data)
		result := invokePluginNLRIDecode(pluginName, family, hexData)
		if result != nil {
			if !outputJSON {
				return formatNLRIHuman(result, family), nil
			}
			jsonData, err := json.Marshal(result)
			if err != nil {
				return "", fmt.Errorf("json marshal: %w", err)
			}
			return string(jsonData), nil
		}
		// Plugin failed, fall through to built-in decode
	}

	afi, safi := parseFamily(family)

	var result map[string]any
	switch {
	case afi == nlri.AFIBGPLS:
		routes := parseBGPLSRoutes(data)
		if len(routes) > 0 {
			if r, ok := routes[0].(map[string]any); ok {
				result = r
			}
		}
	case afi == nlri.AFIL2VPN && safi == nlri.SAFIEVPN:
		routes := parseEVPNRoutes(data)
		if len(routes) > 0 {
			if r, ok := routes[0].(map[string]any); ok {
				result = r
			}
		}
	default: // Unknown family - return raw bytes
		result = map[string]any{
			"parsed": false,
			"raw":    fmt.Sprintf("%X", data),
		}
	}

	if result == nil {
		result = map[string]any{
			"parsed": false,
			"raw":    fmt.Sprintf("%X", data),
		}
	}

	// Human-readable output
	if !outputJSON {
		return formatNLRIHuman(result, family), nil
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("json marshal: %w", err)
	}

	return string(jsonData), nil
}

// parseFamily parses a family string like "ipv4/unicast" into AFI/SAFI.
func parseFamily(family string) (nlri.AFI, nlri.SAFI) {
	f, ok := nlri.ParseFamily(strings.ToLower(family))
	if !ok {
		return 0, 0
	}
	return f.AFI, f.SAFI
}

// hasValidMarker checks if data has the BGP marker (16 0xFF bytes).
func hasValidMarker(data []byte) bool {
	if len(data) < 16 {
		return false
	}
	for i := 0; i < 16; i++ {
		if data[i] != 0xFF {
			return false
		}
	}
	return true
}

// =============================================================================
// Human-Readable Formatters
// =============================================================================

// formatOpenHuman formats OPEN message data as human-readable text.
func formatOpenHuman(result map[string]any) string {
	var sb strings.Builder
	sb.WriteString("BGP OPEN Message\n")

	neighbor, ok := result["neighbor"].(map[string]any)
	if !ok {
		return sb.String()
	}

	openSection, ok := neighbor["open"].(map[string]any)
	if !ok {
		return sb.String()
	}

	// Version
	if v, ok := openSection["version"]; ok {
		fmt.Fprintf(&sb, "  Version:     %v\n", v)
	}

	// ASN
	if asn, ok := openSection["asn"]; ok {
		fmt.Fprintf(&sb, "  ASN:         %v\n", formatNumber(asn))
	}

	// Hold Time
	if ht, ok := openSection["hold_time"]; ok {
		fmt.Fprintf(&sb, "  Hold Time:   %v seconds\n", formatNumber(ht))
	}

	// Router ID
	if rid, ok := openSection["router_id"]; ok {
		fmt.Fprintf(&sb, "  Router ID:   %v\n", rid)
	}

	// Capabilities
	if caps, ok := openSection["capabilities"].(map[string]any); ok && len(caps) > 0 {
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
func formatCapabilityHuman(sb *strings.Builder, cap map[string]any) {
	name, _ := cap["name"].(string)
	if name == "" {
		name = "unknown"
	}

	fmt.Fprintf(sb, "    %-20s ", name)

	// Format capability-specific values
	switch name {
	case "multiprotocol":
		if families, ok := cap["families"].([]string); ok && len(families) > 0 {
			sb.WriteString(strings.Join(families, ", "))
		} else if families, ok := cap["families"].([]any); ok && len(families) > 0 {
			fams := make([]string, 0, len(families))
			for _, f := range families {
				fams = append(fams, fmt.Sprintf("%v", f))
			}
			sb.WriteString(strings.Join(fams, ", "))
		}
	case "asn4":
		if asn, ok := cap["asn4"]; ok {
			sb.WriteString(formatNumber(asn))
		}
	case "fqdn":
		if hostname, ok := cap["hostname"].(string); ok {
			sb.WriteString(hostname)
			if domain, ok := cap["domain"].(string); ok && domain != "" {
				sb.WriteString("." + domain)
			}
		}
	case "graceful-restart":
		if rt, ok := cap["restart_time"]; ok {
			fmt.Fprintf(sb, "%v seconds", formatNumber(rt))
		}
	case "add-path":
		if families, ok := cap["families"].([]any); ok {
			fams := make([]string, 0, len(families))
			for _, f := range families {
				fams = append(fams, fmt.Sprintf("%v", f))
			}
			sb.WriteString(strings.Join(fams, ", "))
		}
	case "unknown":
		if code, ok := cap["code"]; ok {
			fmt.Fprintf(sb, "code=%v", formatNumber(code))
		}
	case "route-refresh", "extended-message": // No additional value needed
		break
	default: // Handle software version and other capabilities
		if sw, ok := cap["software"].(string); ok {
			sb.WriteString(sw)
		}
	}

	sb.WriteString("\n")
}

// formatUpdateHuman formats UPDATE message data as human-readable text.
func formatUpdateHuman(result map[string]any) string {
	var sb strings.Builder
	sb.WriteString("BGP UPDATE Message\n")

	neighbor, ok := result["neighbor"].(map[string]any)
	if !ok {
		return sb.String()
	}

	message, ok := neighbor["message"].(map[string]any)
	if !ok {
		return sb.String()
	}

	update, ok := message["update"].(map[string]any)
	if !ok {
		return sb.String()
	}

	// Attributes
	if attrs, ok := update["attribute"].(map[string]any); ok && len(attrs) > 0 {
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
				asns = append(asns, fmt.Sprintf("%v", formatNumber(v)))
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
	if prefixes, ok := data.([]string); ok {
		for _, prefix := range prefixes {
			fmt.Fprintf(sb, "    %s\n", prefix)
		}
	} else if items, ok := data.([]any); ok {
		for _, item := range items {
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
	if vMap, ok := value.(map[string]any); ok {
		fmt.Fprintf(sb, "%s%s:\n", indent, key)
		for k, val := range vMap {
			formatNLRIFieldHuman(sb, k, val, indent+"  ")
		}
	} else if vSlice, ok := value.([]any); ok {
		fmt.Fprintf(sb, "%s%-20s ", indent, key)
		for i, item := range vSlice {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(sb, "%v", item)
		}
		sb.WriteString("\n")
	} else {
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
