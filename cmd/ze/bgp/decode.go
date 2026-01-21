package bgp

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/netip"
	"os"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/bgp/nlri"
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
	nlriOnly := fs.Bool("nlri", false, "decode as NLRI only")
	family := fs.String("f", "", "address family (e.g., 'ipv4/unicast', 'l2vpn/evpn')")

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
  ze bgp decode -f "l2vpn/evpn" --nlri 02...  # Decode NLRI with family context

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
	case *nlriOnly:
		msgType = msgTypeNLRI
	}

	output, err := decodeHexPacket(payload, msgType, *family)
	if err != nil {
		// Return valid JSON error
		errJSON := map[string]any{
			"error":  err.Error(),
			"parsed": false,
		}
		data, _ := json.Marshal(errJSON)
		fmt.Println(string(data))
		return 1
	}

	fmt.Println(output)
	return 0
}

// decodeHexPacket decodes a hex BGP packet and returns ExaBGP-compatible JSON.
func decodeHexPacket(hexStr, msgType, family string) (string, error) {
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
		return decodeNLRIOnly(data, family)
	}

	// Build JSON output with envelope
	var result map[string]any
	switch msgType {
	case msgTypeOpen:
		result, err = decodeOpenMessage(data, hasHeader)
	case msgTypeUpdate:
		result, err = decodeUpdateMessage(data, family, hasHeader)
	default:
		return "", fmt.Errorf("unsupported message type: %s", msgType)
	}

	if err != nil {
		return "", err
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
func decodeOpenMessage(data []byte, hasHeader bool) (map[string]any, error) {
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
		capsJSON[fmt.Sprintf("%d", c.Code())] = capabilityToJSON(c)
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
func capabilityToJSON(c capability.Capability) map[string]any {
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
			"software": cap.Version,
		}
	case *capability.FQDN:
		return map[string]any{
			"name":     "fqdn",
			"hostname": cap.Hostname,
			"domain":   cap.DomainName,
		}
	default:
		return map[string]any{
			"name": fmt.Sprintf("unknown-%d", c.Code()),
		}
	}
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
	case safi == nlri.SAFIFlowSpec:
		routes = parseFlowSpecRoutes(data, afi)
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

// parseFlowSpecRoutes parses FlowSpec NLRI.
func parseFlowSpecRoutes(data []byte, afi nlri.AFI) []any {
	var routes []any

	// Determine family for FlowSpec
	var family nlri.Family
	if afi == nlri.AFIIPv6 {
		family = nlri.IPv6FlowSpec
	} else {
		family = nlri.IPv4FlowSpec
	}

	// FlowSpec parser consumes one NLRI at a time
	parsed, err := nlri.ParseFlowSpec(family, data)
	if err != nil {
		routes = append(routes, map[string]any{
			"parsed": false,
			"raw":    fmt.Sprintf("%X", data),
		})
		return routes
	}

	route := flowSpecToJSON(parsed)
	routes = append(routes, route)

	return routes
}

// flowSpecToJSON converts a FlowSpec NLRI to ZeBGP JSON format.
// Uses nested arrays: outer=OR, inner=AND. Example: [[">80","<90"],["=100"]] means (>80 AND <90) OR =100.
func flowSpecToJSON(fs *nlri.FlowSpec) map[string]any {
	result := make(map[string]any)

	for _, comp := range fs.Components() {
		key := flowSpecKeyName(comp.Type(), fs.Family())
		values := flowSpecComponentValues(comp)
		if len(values) > 0 {
			result[key] = values
		}
	}

	return result
}

// flowSpecKeyName returns the ExaBGP JSON key for a component type.
func flowSpecKeyName(t nlri.FlowComponentType, family nlri.Family) string {
	isIPv6 := family == nlri.IPv6FlowSpec

	switch t {
	case nlri.FlowDestPrefix:
		if isIPv6 {
			return "destination-ipv6"
		}
		return "destination"
	case nlri.FlowSourcePrefix:
		if isIPv6 {
			return "source-ipv6"
		}
		return "source"
	case nlri.FlowIPProtocol:
		if isIPv6 {
			return "next-header"
		}
		return "protocol"
	case nlri.FlowPort:
		return "port"
	case nlri.FlowDestPort:
		return "destination-port"
	case nlri.FlowSourcePort:
		return "source-port"
	case nlri.FlowICMPType:
		return "icmp-type"
	case nlri.FlowICMPCode:
		return "icmp-code"
	case nlri.FlowTCPFlags:
		return "tcp-flags"
	case nlri.FlowPacketLength:
		return "packet-length"
	case nlri.FlowDSCP:
		return "dscp"
	case nlri.FlowFragment:
		return "fragment"
	case nlri.FlowFlowLabel:
		return "flow-label"
	default:
		return fmt.Sprintf("type-%d", t)
	}
}

// flowSpecComponentValues extracts values from a component as nested arrays.
// Returns [][]string where outer=OR groups, inner=AND groups.
// Example: [[">80","<90"],["=100"]] means (>80 AND <90) OR =100.
func flowSpecComponentValues(comp nlri.FlowComponent) [][]string {
	// Check component type to extract values appropriately
	switch c := comp.(type) {
	case interface{ Prefix() netip.Prefix }:
		// Prefix component (destination/source) - single value in single OR group
		prefix := c.Prefix()
		// Check for offset (IPv6)
		if offseter, ok := comp.(interface{ Offset() uint8 }); ok {
			offset := offseter.Offset()
			return [][]string{{fmt.Sprintf("%s/%d", prefix, offset)}}
		}
		return [][]string{{prefix.String()}}

	case interface{ Matches() []nlri.FlowMatch }:
		// Numeric component - group by AND bit
		matches := c.Matches()

		// Fragment flags need special handling
		if comp.Type() == nlri.FlowFragment {
			return groupFlowMatches(matches, comp.Type())
		}

		// TCP flags need special handling
		if comp.Type() == nlri.FlowTCPFlags {
			return groupTCPFlagsMatches(matches)
		}

		return groupFlowMatches(matches, comp.Type())

	default:
		// Fallback to string representation
		return [][]string{{comp.String()}}
	}
}

// groupFlowMatches groups FlowMatch values by AND bit into nested arrays.
// Returns [][]string where outer=OR groups, inner=AND groups.
// Example: [[">80","<90"],["=100"]] means (>80 AND <90) OR =100.
func groupFlowMatches(matches []nlri.FlowMatch, compType nlri.FlowComponentType) [][]string {
	if len(matches) == 0 {
		return nil
	}

	var result [][]string
	currentGroup := make([]string, 0, 2)

	for i, m := range matches {
		// Format the match value
		formatted := formatFlowMatch(m, compType)

		switch {
		case i == 0:
			// First match always starts a new group
			currentGroup = append(currentGroup, formatted)
		case m.And:
			// AND=true: combine with current group
			currentGroup = append(currentGroup, formatted)
		default:
			// AND=false: finish current group, start new one
			if len(currentGroup) > 0 {
				result = append(result, currentGroup)
			}
			currentGroup = make([]string, 0, 2)
			currentGroup = append(currentGroup, formatted)
		}
	}

	// Don't forget the last group
	if len(currentGroup) > 0 {
		result = append(result, currentGroup)
	}

	return result
}

// formatTCPFlagsFlat formats TCP flags matches as flat string slice for string output.
// Consecutive matches with AND bit form compound expressions like "cwr&!fin&!ece".
func formatTCPFlagsFlat(matches []nlri.FlowMatch) []string {
	var results []string
	current := make([]string, 0, len(matches))

	for _, m := range matches {
		if !m.And && len(current) > 0 {
			results = append(results, strings.Join(current, "&"))
			current = nil
		}
		flagStr := formatSingleTCPFlag(m)
		current = append(current, flagStr)
	}

	if len(current) > 0 {
		results = append(results, strings.Join(current, "&"))
	}

	return results
}

// groupTCPFlagsMatches groups TCP flags matches by AND bit into nested arrays.
// Returns [][]string where outer=OR groups, inner=AND groups.
func groupTCPFlagsMatches(matches []nlri.FlowMatch) [][]string {
	if len(matches) == 0 {
		return nil
	}

	var result [][]string
	currentGroup := make([]string, 0, 2)

	for i, m := range matches {
		// Format the TCP flag match
		formatted := formatSingleTCPFlag(m)

		switch {
		case i == 0:
			// First match always starts a new group
			currentGroup = append(currentGroup, formatted)
		case m.And:
			// AND=true: combine with current group
			currentGroup = append(currentGroup, formatted)
		default:
			// AND=false: finish current group, start new one
			if len(currentGroup) > 0 {
				result = append(result, currentGroup)
			}
			currentGroup = make([]string, 0, 2)
			currentGroup = append(currentGroup, formatted)
		}
	}

	// Don't forget the last group
	if len(currentGroup) > 0 {
		result = append(result, currentGroup)
	}

	return result
}

// formatSingleTCPFlag formats a single TCP flag match.
// For bitmask operations (TCP flags, fragment), FlowOpNot (0x02) means NOT.
// Always prefixes with = (match) or ! (not match) for clarity.
func formatSingleTCPFlag(m nlri.FlowMatch) string {
	// Get flag representation (handles combined flags like fin+push)
	flagStr := tcpFlagsString(m.Value)

	// For bitmask operations: FlowOpNot (0x02) = negation
	if m.Op&nlri.FlowOpNot != 0 {
		return "!" + flagStr
	}
	// Default to match (=) for consistency
	return "=" + flagStr
}

// formatFlowMatch formats a single FlowMatch for JSON output.
func formatFlowMatch(m nlri.FlowMatch, compType nlri.FlowComponentType) string {
	// Get operator prefix
	opStr := flowOpString(m.Op)

	// Special handling for fragment flags - use bitmask operators like TCP flags
	if compType == nlri.FlowFragment {
		return formatSingleFragmentFlag(m)
	}

	// Special handling for protocol/next-header
	if compType == nlri.FlowIPProtocol {
		return opStr + protocolName(m.Value)
	}

	// Special handling for TCP flags
	if compType == nlri.FlowTCPFlags {
		return opStr + tcpFlagsString(m.Value)
	}

	// Default: operator + numeric value
	return fmt.Sprintf("%s%d", opStr, m.Value)
}

// flowOpString returns the operator string prefix.
func flowOpString(op nlri.FlowOperator) string {
	// Mask out non-comparison bits
	cmp := op &^ (nlri.FlowOpEnd | nlri.FlowOpAnd | nlri.FlowOpLenMask)
	switch cmp { //nolint:exhaustive // Only comparison bits after masking
	case nlri.FlowOpEqual:
		return "="
	case nlri.FlowOpGreater:
		return ">"
	case nlri.FlowOpLess:
		return "<"
	case nlri.FlowOpGreater | nlri.FlowOpEqual:
		return ">="
	case nlri.FlowOpLess | nlri.FlowOpEqual:
		return "<="
	case nlri.FlowOpLess | nlri.FlowOpGreater:
		return "!="
	default:
		return ""
	}
}

// protocolName returns protocol name or number.
func protocolName(value uint64) string {
	switch value {
	case 1:
		return "icmp"
	case 6:
		return "tcp"
	case 17:
		return "udp"
	case 58:
		return "icmpv6"
	default:
		return fmt.Sprintf("%d", value)
	}
}

// tcpFlagsString returns TCP flags representation with + separator.
func tcpFlagsString(value uint64) string {
	var flags []string
	if value&0x01 != 0 {
		flags = append(flags, "fin")
	}
	if value&0x02 != 0 {
		flags = append(flags, "syn")
	}
	if value&0x04 != 0 {
		flags = append(flags, "rst")
	}
	if value&0x08 != 0 {
		flags = append(flags, "push")
	}
	if value&0x10 != 0 {
		flags = append(flags, "ack")
	}
	if value&0x20 != 0 {
		flags = append(flags, "urgent")
	}
	if value&0x40 != 0 {
		flags = append(flags, "ece")
	}
	if value&0x80 != 0 {
		flags = append(flags, "cwr")
	}
	if len(flags) == 0 {
		return fmt.Sprintf("0x%x", value)
	}
	return strings.Join(flags, "+")
}

// formatSingleFragmentFlag formats a single fragment flag match.
// For bitmask operations (fragment), FlowOpNot (0x02) means NOT.
// Always prefixes with = (match) or ! (not match) for clarity.
func formatSingleFragmentFlag(m nlri.FlowMatch) string {
	// Get flag representation
	flagStr := formatFragmentFlags(m.Value)

	// For bitmask operations: FlowOpNot (0x02) = negation
	if m.Op&nlri.FlowOpNot != 0 {
		return "!" + flagStr
	}
	// Default to match (=) for consistency
	return "=" + flagStr
}

// fragmentFlagsToArray returns fragment flags as separate array elements.
func fragmentFlagsToArray(value uint64) []string {
	var flags []string
	if value&0x01 != 0 {
		flags = append(flags, "dont-fragment")
	}
	if value&0x02 != 0 {
		flags = append(flags, "is-fragment")
	}
	if value&0x04 != 0 {
		flags = append(flags, "first-fragment")
	}
	if value&0x08 != 0 {
		flags = append(flags, "last-fragment")
	}
	if len(flags) == 0 {
		flags = append(flags, fmt.Sprintf("0x%x", value))
	}
	return flags
}

// formatFragmentFlags returns fragment flag names as single string.
func formatFragmentFlags(value uint64) string {
	flags := fragmentFlagsToArray(value)
	return strings.Join(flags, " ")
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

// decodeNLRIOnly decodes NLRI without envelope (for bgp-ls tests).
func decodeNLRIOnly(data []byte, family string) (string, error) {
	afi, safi := parseFamily(family)

	var result any
	switch {
	case afi == nlri.AFIBGPLS:
		routes := parseBGPLSRoutes(data)
		if len(routes) > 0 {
			result = routes[0] // Return first route for NLRI-only mode
		}
	case afi == nlri.AFIL2VPN && safi == nlri.SAFIEVPN:
		routes := parseEVPNRoutes(data)
		if len(routes) > 0 {
			result = routes[0]
		}
	default:
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
