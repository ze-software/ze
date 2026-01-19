// Package exabgp provides compatibility tools for ExaBGP plugins and configs.
package exabgp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Version is the ExaBGP version string used in the JSON envelope.
const Version = "5.0.0"

// Mode/direction string constants.
// Used for both ExaBGP JSON direction field and ADD-PATH CLI mode.
const (
	modeReceive = "receive"
	modeSend    = "send"
	modeBoth    = "both"
)

// ZebgpToExabgpJSON converts a ZeBGP JSON event to ExaBGP JSON format.
//
// ZeBGP format:
//
//	{
//	  "message": {"type": "update", "id": 1, "direction": "received"},
//	  "peer": {"address": "10.0.0.1", "asn": 65001},
//	  "origin": "igp",
//	  "ipv4/unicast": [{"action": "add", "next-hop": "10.0.0.1", "nlri": ["192.168.1.0/24"]}]
//	}
//
// ExaBGP format:
//
//	{
//	  "exabgp": "5.0.0",
//	  "type": "update",
//	  "neighbor": {
//	    "address": {"peer": "10.0.0.1"},
//	    "asn": {"peer": 65001},
//	    "direction": "receive",
//	    "message": {"update": {...}}
//	  }
//	}
func ZebgpToExabgpJSON(zebgp map[string]any) map[string]any {
	msg, _ := zebgp["message"].(map[string]any)
	msgType, _ := msg["type"].(string)
	if msgType == "" {
		msgType = "update"
	}

	peer, _ := zebgp["peer"].(map[string]any)
	peerAddr, _ := peer["address"].(string)
	peerASN, _ := peer["asn"].(float64)

	// Map direction: ZeBGP "received"/"sent" → ExaBGP "receive"/"send"
	direction, _ := msg["direction"].(string)
	switch direction {
	case "received":
		direction = modeReceive
	case "sent":
		direction = modeSend
	case "":
		direction = modeReceive
	}

	// Build ExaBGP envelope
	result := map[string]any{
		"exabgp": Version,
		"time":   float64(time.Now().Unix()),
		"host":   hostname(),
		"pid":    os.Getpid(),
		"ppid":   os.Getppid(),
		"type":   msgType,
	}

	// Build neighbor section
	neighbor := map[string]any{
		"address":   map[string]any{"peer": peerAddr},
		"asn":       map[string]any{"peer": peerASN},
		"direction": direction,
	}

	switch msgType {
	case "state":
		state, _ := zebgp["state"].(string)
		neighbor["state"] = state

	case "update":
		update := convertUpdate(zebgp)
		if len(update) > 0 {
			neighbor["message"] = map[string]any{"update": update}
		}

	case "notification":
		notif, _ := zebgp["notification"].(map[string]any)
		neighbor["notification"] = map[string]any{
			"code":    notif["code"],
			"subcode": notif["subcode"],
			"data":    notif["data"],
		}

	case "negotiated":
		neg, _ := zebgp["negotiated"].(map[string]any)
		result["negotiated"] = convertNegotiated(neg)
	}

	result["neighbor"] = neighbor
	return result
}

// convertNegotiated converts ZeBGP negotiated caps to ExaBGP format.
//
// Key conversions:
//   - Family format: "ipv4/unicast" → "ipv4 unicast".
//   - extended_nexthop map → nexthop list.
func convertNegotiated(zebgp map[string]any) map[string]any {
	if zebgp == nil {
		return map[string]any{}
	}

	result := make(map[string]any)

	// Copy scalar fields directly
	scalarKeys := []string{"message_size", "hold_time", "asn4", "multisession", "operational", "refresh"}
	for _, key := range scalarKeys {
		if v, ok := zebgp[key]; ok {
			result[key] = v
		}
	}

	// Convert families: "ipv4/unicast" → "ipv4 unicast"
	if families, ok := zebgp["families"].([]any); ok {
		result["families"] = convertFamilyList(families)
	}

	// Convert extended_nexthop map to ExaBGP nexthop list format.
	// ZeBGP: {"ipv4/unicast": "ipv6"} → ExaBGP: ["ipv4 unicast ipv6"]
	if extNH, ok := zebgp["extended_nexthop"].(map[string]any); ok {
		result["nexthop"] = convertExtendedNextHop(extNH)
	}

	// Convert add_path
	if addPath, ok := zebgp["add_path"].(map[string]any); ok {
		converted := make(map[string]any)
		if send, ok := addPath["send"].([]any); ok {
			converted["send"] = convertFamilyList(send)
		}
		if recv, ok := addPath["receive"].([]any); ok {
			converted["receive"] = convertFamilyList(recv)
		}
		result["add_path"] = converted
	}

	return result
}

// convertFamilyList converts a list of families from ZeBGP to ExaBGP format.
// Converts "ipv4/unicast" to "ipv4 unicast".
func convertFamilyList(families []any) []string {
	result := make([]string, 0, len(families))
	for _, f := range families {
		if s, ok := f.(string); ok {
			result = append(result, strings.ReplaceAll(s, "/", " "))
		}
	}
	return result
}

// convertExtendedNextHop converts ZeBGP extended_nexthop map to ExaBGP nexthop list.
// Example: {"ipv4/unicast": "ipv6"} becomes ["ipv4 unicast ipv6"].
func convertExtendedNextHop(extNH map[string]any) []string {
	result := make([]string, 0, len(extNH))
	for family, nhAFI := range extNH {
		if nhStr, ok := nhAFI.(string); ok {
			// Convert family "/" to " " and append nexthop AFI
			familySpaced := strings.ReplaceAll(family, "/", " ")
			result = append(result, familySpaced+" "+nhStr)
		}
	}
	return result
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

func convertUpdate(zebgp map[string]any) map[string]any {
	update := make(map[string]any)

	// Extract attributes from top-level ZeBGP fields
	attrs := make(map[string]any)
	attrKeys := []string{
		"origin", "as-path", "med", "local-preference", "community",
		"large-community", "extended-community", "aggregator",
		"originator-id", "cluster-list", "atomic-aggregate",
	}
	for _, key := range attrKeys {
		if v, ok := zebgp[key]; ok {
			attrs[key] = v
		}
	}
	if len(attrs) > 0 {
		update["attribute"] = attrs
	}

	// Convert NLRI sections: "ipv4/unicast" → "ipv4 unicast"
	announce := make(map[string]map[string][]any)
	withdraw := make(map[string][]any)

	for key, value := range zebgp {
		if !strings.Contains(key, "/") || key == "as-path" {
			continue
		}

		// Convert family: "ipv4/unicast" → "ipv4 unicast"
		family := strings.ReplaceAll(key, "/", " ")

		entries, ok := value.([]any)
		if !ok {
			continue
		}

		for _, e := range entries {
			entry, ok := e.(map[string]any)
			if !ok {
				continue
			}

			action, _ := entry["action"].(string)
			nlriList, _ := entry["nlri"].([]any)
			nextHop, _ := entry["next-hop"].(string)

			switch action {
			case "add":
				nhKey := nextHop
				if nhKey == "" {
					nhKey = "null"
				}
				if announce[family] == nil {
					announce[family] = make(map[string][]any)
				}

				for _, nlri := range nlriList {
					if s, ok := nlri.(string); ok {
						announce[family][nhKey] = append(announce[family][nhKey], map[string]any{"nlri": s})
					} else {
						announce[family][nhKey] = append(announce[family][nhKey], nlri)
					}
				}
			case "del":
				for _, nlri := range nlriList {
					if s, ok := nlri.(string); ok {
						withdraw[family] = append(withdraw[family], map[string]any{"nlri": s})
					} else {
						withdraw[family] = append(withdraw[family], nlri)
					}
				}
			}
		}
	}

	if len(announce) > 0 {
		update["announce"] = announce
	}
	if len(withdraw) > 0 {
		update["withdraw"] = withdraw
	}

	return update
}

// ExabgpToZebgpCommand converts an ExaBGP text command to ZeBGP format.
//
// ExaBGP: neighbor <ip> announce route <prefix> next-hop <nh> [origin <o>] ...
// ZeBGP:  peer <ip> update text nhop set <nh> origin set <o> nlri ipv4/unicast add <prefix>.
func ExabgpToZebgpCommand(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return ""
	}

	// Parse neighbor command
	neighborRE := regexp.MustCompile(`(?i)^neighbor\s+(\S+)\s+(.+)$`)
	match := neighborRE.FindStringSubmatch(line)
	if match == nil {
		// Not a neighbor command - pass through
		return line
	}

	peerIP := match[1]
	rest := strings.TrimSpace(match[2])
	restLower := strings.ToLower(rest)

	// Handle announce route
	if strings.HasPrefix(restLower, "announce route") {
		return convertAnnounce(peerIP, rest[14:])
	}

	// Handle withdraw route
	if strings.HasPrefix(restLower, "withdraw route") {
		return convertWithdraw(peerIP, rest[14:])
	}

	// Handle announce/withdraw for other families
	if strings.HasPrefix(restLower, "announce") {
		return convertAnnounceFamily(peerIP, rest[8:])
	}

	if strings.HasPrefix(restLower, "withdraw") {
		return convertWithdrawFamily(peerIP, rest[8:])
	}

	// Unknown command - pass through with peer prefix change
	return fmt.Sprintf("peer %s %s", peerIP, rest)
}

func convertAnnounce(peerIP, routeStr string) string {
	routeStr = strings.TrimSpace(routeStr)
	parts := strings.Fields(routeStr)
	if len(parts) == 0 {
		return fmt.Sprintf("peer %s update text nlri ipv4/unicast add", peerIP)
	}

	prefix := parts[0]
	attrs := parts[1:]

	// Parse attributes
	var cmdParts []string
	cmdParts = append(cmdParts, fmt.Sprintf("peer %s update text", peerIP))

	i := 0
	for i < len(attrs) {
		key := strings.ToLower(attrs[i])
		switch key {
		case "next-hop":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("nhop set %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		case "origin":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("origin set %s", strings.ToLower(attrs[i+1])))
				i += 2
			} else {
				i++
			}
		case "as-path":
			if i+1 < len(attrs) {
				asp := attrs[i+1]
				i += 2
				if strings.HasPrefix(asp, "[") {
					// Collect until ]
					aspParts := []string{asp}
					for i < len(attrs) && !strings.Contains(aspParts[len(aspParts)-1], "]") {
						aspParts = append(aspParts, attrs[i])
						i++
					}
					asp = strings.Join(aspParts, " ")
				}
				asp = strings.Trim(asp, "[]")
				asp = strings.TrimSpace(asp)
				if asp != "" {
					cmdParts = append(cmdParts, fmt.Sprintf("as-path set %s", asp))
				}
			} else {
				i++
			}
		case "med":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("med set %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		case "local-preference":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("local-preference set %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		case "community":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("community add %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		case "large-community":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("large-community add %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		default:
			i++
		}
	}

	// Determine family from prefix
	family := "ipv4/unicast"
	if strings.Contains(prefix, ":") {
		family = "ipv6/unicast"
	}
	cmdParts = append(cmdParts, fmt.Sprintf("nlri %s add %s", family, prefix))

	return strings.Join(cmdParts, " ")
}

func convertWithdraw(peerIP, routeStr string) string {
	routeStr = strings.TrimSpace(routeStr)
	parts := strings.Fields(routeStr)
	if len(parts) == 0 {
		return fmt.Sprintf("peer %s update text nlri ipv4/unicast del", peerIP)
	}

	prefix := parts[0]
	family := "ipv4/unicast"
	if strings.Contains(prefix, ":") {
		family = "ipv6/unicast"
	}
	return fmt.Sprintf("peer %s update text nlri %s del %s", peerIP, family, prefix)
}

func convertAnnounceFamily(peerIP, rest string) string {
	rest = strings.TrimSpace(rest)
	familyRE := regexp.MustCompile(`(?i)^(ipv[46])\s+(unicast|multicast|nlri-mpls|flowspec)\s+(.+)$`)
	match := familyRE.FindStringSubmatch(rest)
	if match != nil {
		afi := strings.ToLower(match[1])
		safi := strings.ToLower(match[2])
		routeStr := match[3]
		family := fmt.Sprintf("%s/%s", afi, safi)
		return convertAnnounceWithFamily(peerIP, family, routeStr)
	}

	// Fall back to basic conversion
	return fmt.Sprintf("peer %s announce %s", peerIP, rest)
}

func convertWithdrawFamily(peerIP, rest string) string {
	rest = strings.TrimSpace(rest)
	familyRE := regexp.MustCompile(`(?i)^(ipv[46])\s+(unicast|multicast|nlri-mpls|flowspec)\s+(.+)$`)
	match := familyRE.FindStringSubmatch(rest)
	if match != nil {
		afi := strings.ToLower(match[1])
		safi := strings.ToLower(match[2])
		prefix := strings.Fields(match[3])[0]
		family := fmt.Sprintf("%s/%s", afi, safi)
		return fmt.Sprintf("peer %s update text nlri %s del %s", peerIP, family, prefix)
	}

	return fmt.Sprintf("peer %s withdraw %s", peerIP, rest)
}

func convertAnnounceWithFamily(peerIP, family, routeStr string) string {
	routeStr = strings.TrimSpace(routeStr)
	parts := strings.Fields(routeStr)
	if len(parts) == 0 {
		return fmt.Sprintf("peer %s update text nlri %s add", peerIP, family)
	}

	prefix := parts[0]
	attrs := parts[1:]

	var cmdParts []string
	cmdParts = append(cmdParts, fmt.Sprintf("peer %s update text", peerIP))

	i := 0
	for i < len(attrs) {
		key := strings.ToLower(attrs[i])
		switch key {
		case "next-hop":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("nhop set %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		case "origin":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("origin set %s", strings.ToLower(attrs[i+1])))
				i += 2
			} else {
				i++
			}
		case "label":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("label set %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		case "rd":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("rd set %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		default:
			i++
		}
	}

	cmdParts = append(cmdParts, fmt.Sprintf("nlri %s add %s", family, prefix))
	return strings.Join(cmdParts, " ")
}

// StartupProtocol handles the 5-stage ZeBGP plugin registration protocol.
// This must be completed before the bridge can begin JSON translation.
//
// Stages:
//  1. Declaration (Bridge → ZeBGP): send family, encoding declarations, then "declare done"
//  2. Config (ZeBGP → Bridge): wait for "config done" (discard config lines)
//  3. Capability (Bridge → ZeBGP): send capability lines, then "capability done"
//  4. Registry (ZeBGP → Bridge): wait for "registry done" (discard registry lines)
//  5. Ready (Bridge → ZeBGP): send "ready"
type StartupProtocol struct {
	output  io.Writer
	scanner *bufio.Scanner

	// Families to declare (ZeBGP format: "ipv4/unicast").
	// Converted to ZeBGP plugin protocol format: "ipv4 unicast".
	Families []string

	// RouteRefresh enables route-refresh capability (RFC 2918, code 2).
	RouteRefresh bool

	// AddPathMode sets ADD-PATH capability mode (RFC 7911, code 69).
	// Valid values: "receive", "send", "both", or "" (disabled).
	AddPathMode string
}

// NewStartupProtocol creates a new startup protocol handler.
// The scanner should be reused after startup for JSON event processing
// to avoid losing buffered data.
func NewStartupProtocol(scanner *bufio.Scanner, output io.Writer) *StartupProtocol {
	return &StartupProtocol{
		scanner:  scanner,
		output:   output,
		Families: []string{"ipv4/unicast"}, // Default family
	}
}

// defaultFamily is the fallback when no families are configured.
const defaultFamily = "ipv4/unicast"

// Run executes the full 5-stage startup protocol.
func (sp *StartupProtocol) Run() error {
	// Stage 1: Declaration
	sp.SendDeclarations()

	// Stage 2: Wait for config done
	if err := sp.WaitForConfigDone(); err != nil {
		return fmt.Errorf("stage 2 (config): %w", err)
	}

	// Stage 3: Capability
	sp.SendCapabilityDone()

	// Stage 4: Wait for registry done
	if err := sp.WaitForRegistryDone(); err != nil {
		return fmt.Errorf("stage 4 (registry): %w", err)
	}

	// Stage 5: Ready
	sp.SendReady()

	return nil
}

// SendDeclarations sends Stage 1 declarations.
func (sp *StartupProtocol) SendDeclarations() {
	if sp.output == nil {
		return
	}

	// Use default family if none configured
	families := sp.Families
	if len(families) == 0 {
		families = []string{defaultFamily}
		slog.Debug("no families configured, using default", "family", defaultFamily)
	}

	// Declare families (convert "/" to " " for ZeBGP protocol)
	for _, family := range families {
		// Convert "ipv4/unicast" → "ipv4 unicast"
		zebgpFamily := strings.ReplaceAll(family, "/", " ")
		_, _ = fmt.Fprintf(sp.output, "declare family %s\n", zebgpFamily)
	}

	// Declare encoding
	_, _ = fmt.Fprintln(sp.output, "declare encoding text")

	// Declare receive types - ExaBGP plugins expect negotiated messages
	_, _ = fmt.Fprintln(sp.output, "declare receive negotiated")

	// Done
	_, _ = fmt.Fprintln(sp.output, "declare done")
}

// SendCapabilityDone sends Stage 3 capability lines and done marker.
//
// Capability format: "capability <enc> <code> [payload]".
// - Route-refresh (RFC 2918): code 2, no payload (0-length value per RFC).
// - ADD-PATH (RFC 7911): code 69, payload is hex-encoded AFI/SAFI/mode tuples.
func (sp *StartupProtocol) SendCapabilityDone() {
	if sp.output == nil {
		return
	}

	// Route-refresh capability (RFC 2918, code 2).
	// RFC 2918: "The Capability Length of the Route Refresh Capability is zero."
	if sp.RouteRefresh {
		_, _ = fmt.Fprintln(sp.output, "capability hex 2")
	}

	// ADD-PATH capability (RFC 7911, code 69)
	if sp.AddPathMode != "" {
		payload := sp.encodeAddPath()
		if payload != "" {
			_, _ = fmt.Fprintf(sp.output, "capability hex 69 %s\n", payload)
		}
	}

	_, _ = fmt.Fprintln(sp.output, "capability done")
}

// encodeAddPath encodes ADD-PATH capability payload for configured families.
//
// RFC 7911 Section 4: Each tuple is 4 octets: AFI (2) + SAFI (1) + Send/Receive (1).
// Mode values: 1=receive, 2=send, 3=both.
func (sp *StartupProtocol) encodeAddPath() string {
	mode := sp.addPathModeValue()
	if mode == 0 {
		return ""
	}

	families := sp.Families
	if len(families) == 0 {
		families = []string{defaultFamily}
	}

	var result []byte
	for _, family := range families {
		afi, safi := parseFamilyToAFISAFI(family)
		if afi == 0 {
			slog.Warn("unknown family ignored for ADD-PATH", "family", family)
			continue
		}
		// AFI (2 bytes big-endian) + SAFI (1 byte) + Mode (1 byte)
		result = append(result, byte(afi>>8), byte(afi), byte(safi), mode)
	}

	return fmt.Sprintf("%x", result)
}

// addPathModeValue converts mode string to RFC 7911 value.
func (sp *StartupProtocol) addPathModeValue() byte {
	switch strings.ToLower(sp.AddPathMode) {
	case modeReceive:
		return 1
	case modeSend:
		return 2
	case modeBoth:
		return 3
	default:
		return 0
	}
}

// parseFamilyToAFISAFI converts "ipv4/unicast" to AFI and SAFI values.
// Returns (0, 0) for invalid/unsupported families.
func parseFamilyToAFISAFI(family string) (afi, safi uint16) {
	parts := strings.Split(family, "/")
	if len(parts) != 2 {
		return 0, 0
	}

	afiStr := strings.ToLower(parts[0])
	safiStr := strings.ToLower(parts[1])

	// AFI: ipv4=1, ipv6=2, l2vpn=25
	switch afiStr {
	case "ipv4":
		afi = 1
	case "ipv6":
		afi = 2
	case "l2vpn":
		afi = 25
	default:
		return 0, 0
	}

	// SAFI values per IANA BGP SAFI registry.
	// L2VPN AFI (25) is only valid with EVPN SAFI (70) per RFC 7432.
	switch safiStr {
	case "unicast":
		if afi == 25 {
			return 0, 0 // L2VPN doesn't use unicast.
		}
		safi = 1
	case "multicast":
		if afi == 25 {
			return 0, 0 // L2VPN doesn't use multicast.
		}
		safi = 2
	case "mpls", "nlri-mpls", "labeled-unicast":
		if afi == 25 {
			return 0, 0 // L2VPN doesn't use labeled unicast.
		}
		safi = 4
	case "evpn":
		// RFC 7432: EVPN is only valid with L2VPN AFI.
		if afi != 25 {
			return 0, 0
		}
		safi = 70
	case "vpn", "mpls-vpn":
		if afi == 25 {
			return 0, 0 // L2VPN doesn't use MPLS VPN SAFI.
		}
		safi = 128
	case "flowspec":
		if afi == 25 {
			return 0, 0 // L2VPN doesn't use flowspec.
		}
		safi = 133
	case "flowspec-vpn":
		if afi == 25 {
			return 0, 0 // L2VPN doesn't use flowspec-vpn.
		}
		safi = 134
	default:
		return 0, 0
	}

	return afi, safi
}

// ValidateFamily checks if a family string is supported.
// Uses parseFamilyToAFISAFI as single source of truth.
func ValidateFamily(family string) error {
	afi, safi := parseFamilyToAFISAFI(family)
	if afi == 0 || safi == 0 {
		return fmt.Errorf("unsupported address family: %s", family)
	}
	return nil
}

// SendReady sends Stage 5 ready signal.
func (sp *StartupProtocol) SendReady() {
	if sp.output == nil {
		return
	}
	_, _ = fmt.Fprintln(sp.output, "ready")
}

// WaitForConfigDone waits for Stage 2 "config done" marker.
func (sp *StartupProtocol) WaitForConfigDone() error {
	return sp.waitForLine("config done")
}

// WaitForRegistryDone waits for Stage 4 "registry done" marker.
func (sp *StartupProtocol) WaitForRegistryDone() error {
	return sp.waitForLine("registry done")
}

// waitForLine reads lines until the expected line is found.
func (sp *StartupProtocol) waitForLine(expected string) error {
	if sp.scanner == nil {
		return nil
	}

	for sp.scanner.Scan() {
		line := sp.scanner.Text()
		if line == expected {
			return nil
		}
	}

	if err := sp.scanner.Err(); err != nil {
		return err
	}

	return fmt.Errorf("EOF before %q", expected)
}

// Bridge wraps an ExaBGP plugin process and translates between ZeBGP and ExaBGP formats.
type Bridge struct {
	pluginCmd []string
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	running   bool
	mu        sync.Mutex

	// Families to declare during startup (ZeBGP format: "ipv4/unicast")
	Families []string

	// RouteRefresh enables route-refresh capability (RFC 2918).
	RouteRefresh bool

	// AddPathMode sets ADD-PATH capability mode: "receive", "send", "both", or "" (disabled).
	AddPathMode string
}

// NewBridge creates a new bridge for the given ExaBGP plugin command.
func NewBridge(pluginCmd []string) *Bridge {
	return &Bridge{
		pluginCmd: pluginCmd,
		Families:  []string{"ipv4/unicast"}, // Default family
	}
}

// Start starts the plugin subprocess with the given context.
func (b *Bridge) Start(ctx context.Context) error {
	var err error

	//nolint:gosec // User-provided plugin command is intentional.
	b.cmd = exec.CommandContext(ctx, b.pluginCmd[0], b.pluginCmd[1:]...)

	b.stdin, err = b.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	b.stdout, err = b.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	b.stderr, err = b.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := b.cmd.Start(); err != nil {
		return fmt.Errorf("start plugin: %w", err)
	}

	b.running = true
	return nil
}

// Stop stops the plugin subprocess.
func (b *Bridge) Stop() {
	b.mu.Lock()
	b.running = false
	b.mu.Unlock()

	if b.cmd.Process != nil {
		_ = b.cmd.Process.Kill()
	}
}

// Run runs the bridge, translating between ZeBGP (stdin/stdout) and the plugin.
// It first completes the 5-stage startup protocol, then begins JSON translation.
func (b *Bridge) Run(ctx context.Context) error {
	// Create a single scanner for os.Stdin - used for both startup and JSON events.
	// This prevents data loss from buffered reads.
	stdinScanner := bufio.NewScanner(os.Stdin)

	// Stage 1-5: Complete startup protocol with ZeBGP
	sp := NewStartupProtocol(stdinScanner, os.Stdout)
	sp.Families = b.Families
	sp.RouteRefresh = b.RouteRefresh
	sp.AddPathMode = b.AddPathMode
	if err := sp.Run(); err != nil {
		return fmt.Errorf("startup protocol: %w", err)
	}

	// Stage 6: Running - start plugin and begin JSON translation
	if err := b.Start(ctx); err != nil {
		return err
	}
	defer b.Stop()

	var wg sync.WaitGroup
	wg.Add(3)

	// ZeBGP stdin → plugin stdin (translate ZeBGP JSON → ExaBGP JSON)
	// Uses the SAME scanner that completed startup to avoid losing buffered data.
	go func() {
		defer wg.Done()
		b.zebgpToPluginWithScanner(ctx, stdinScanner, b.stdin)
	}()

	// Plugin stdout → ZeBGP stdout (translate ExaBGP commands → ZeBGP commands)
	go func() {
		defer wg.Done()
		b.pluginToZebgp(ctx, b.stdout, os.Stdout)
	}()

	// Plugin stderr → ZeBGP stderr (pass through)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(os.Stderr, b.stderr)
	}()

	// Wait for plugin to exit
	err := b.cmd.Wait()
	wg.Wait()
	return err
}

func (b *Bridge) zebgpToPluginWithScanner(ctx context.Context, scanner *bufio.Scanner, w io.Writer) {
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			slog.Debug("zebgp→plugin: context cancelled")
			return
		default:
		}

		b.mu.Lock()
		running := b.running
		b.mu.Unlock()
		if !running {
			return
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		var zebgp map[string]any
		if err := json.Unmarshal([]byte(line), &zebgp); err != nil {
			slog.Warn("zebgp→plugin: invalid JSON from ZeBGP",
				"error", err,
				"line", truncate(line, 100))
			continue
		}

		exabgp := ZebgpToExabgpJSON(zebgp)
		out, err := json.Marshal(exabgp)
		if err != nil {
			slog.Warn("zebgp→plugin: failed to marshal ExaBGP JSON",
				"error", err)
			continue
		}

		_, _ = fmt.Fprintln(w, string(out))
	}

	if err := scanner.Err(); err != nil {
		slog.Warn("zebgp→plugin: scanner error", "error", err)
	}
}

func (b *Bridge) pluginToZebgp(ctx context.Context, r io.Reader, w io.Writer) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			slog.Debug("plugin→zebgp: context cancelled")
			return
		default:
		}

		b.mu.Lock()
		running := b.running
		b.mu.Unlock()
		if !running {
			return
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		zebgpCmd := ExabgpToZebgpCommand(line)
		if zebgpCmd != "" {
			_, _ = fmt.Fprintln(w, zebgpCmd)
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Warn("plugin→zebgp: scanner error", "error", err)
	}
}

// truncate returns s truncated to maxLen runes with "..." suffix if needed.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
