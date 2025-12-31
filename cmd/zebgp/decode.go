package main

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/exa-networks/zebgp/pkg/bgp/capability"
	"github.com/exa-networks/zebgp/pkg/bgp/message"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
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
	family := fs.String("f", "", "address family (e.g., 'ipv4 unicast', 'l2vpn evpn')")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: zebgp decode [options] <hex-payload>

Decode BGP message from hexadecimal and output ExaBGP-compatible JSON.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  zebgp decode --open FFFF...       # Decode OPEN message
  zebgp decode --update FFFF...     # Decode UPDATE message
  zebgp decode -f "l2vpn evpn" --nlri 02...  # Decode NLRI with family context

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
				withdraw["ipv4 unicast"] = prefixes
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
			announce["ipv4 unicast"] = map[string]any{
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
			if asPath := parseASPath(value); len(asPath) > 0 {
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
		case 14: // MP_REACH_NLRI
			mpReach = value
		case 15: // MP_UNREACH_NLRI
			mpUnreach = value
		}

		offset += hdrLen + valueLen
	}

	return attrs, mpReach, mpUnreach
}

// parseASPath parses AS_PATH attribute value.
func parseASPath(data []byte) []any {
	var result []any
	offset := 0

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

		if segType == 1 { // AS_SET
			result = append(result, asns)
		} else { // AS_SEQUENCE
			for _, asn := range asns {
				result = append(result, asn)
			}
		}
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
	// Match ExaBGP format
	switch {
	case afi == nlri.AFIL2VPN && safi == nlri.SAFIEVPN:
		return "l2vpn evpn"
	case afi == nlri.AFIIPv4 && safi == nlri.SAFIFlowSpec:
		return "ipv4 flow"
	case afi == nlri.AFIIPv6 && safi == nlri.SAFIFlowSpec:
		return "ipv6 flow"
	case afi == nlri.AFIBGPLS:
		return "bgp-ls bgp-ls"
	default:
		return fmt.Sprintf("%s %s", strings.ToLower(afi.String()), strings.ToLower(safi.String()))
	}
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

// flowSpecToJSON converts a FlowSpec NLRI to JSON.
func flowSpecToJSON(fs *nlri.FlowSpec) map[string]any {
	result := map[string]any{
		"parsed": true,
		"string": fs.String(),
	}

	// Add individual components using their string representation
	for _, comp := range fs.Components() {
		key := comp.Type().String()
		result[key] = comp.String()
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

// bgplsToJSON converts a BGP-LS NLRI to JSON.
func bgplsToJSON(n nlri.BGPLSNLRI) map[string]any {
	return map[string]any{
		"parsed": true,
		"raw":    fmt.Sprintf("%X", n.Bytes()),
		"string": n.String(),
	}
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

// parseFamily parses a family string like "ipv4 unicast" into AFI/SAFI.
func parseFamily(family string) (nlri.AFI, nlri.SAFI) {
	parts := strings.Fields(strings.ToLower(family))
	if len(parts) < 2 {
		return 0, 0
	}

	var afi nlri.AFI
	var safi nlri.SAFI

	switch parts[0] {
	case "ipv4":
		afi = nlri.AFIIPv4
	case "ipv6":
		afi = nlri.AFIIPv6
	case "l2vpn":
		afi = nlri.AFIL2VPN
	case "bgp-ls":
		afi = nlri.AFIBGPLS
	}

	switch parts[1] {
	case "unicast":
		safi = nlri.SAFIUnicast
	case "multicast":
		safi = nlri.SAFIMulticast
	case "evpn":
		safi = nlri.SAFIEVPN
	case "flowspec", "flow":
		safi = nlri.SAFIFlowSpec
	case "vpn":
		safi = nlri.SAFIVPN
	case "bgp-ls":
		safi = nlri.SAFIBGPLinkState
	}

	return afi, safi
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
