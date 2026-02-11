package bgp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugin/registry"
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

	// Ze format: {"type": "bgp", "bgp": {"type": "<event>", "peer": {...}, "<event>": {...}}}.
	envelope := makeZeEnvelope(msgType)
	bgp, _ := envelope["bgp"].(map[string]any)

	// Merge event-specific content into bgp.<event> section
	if eventContent, ok := result[msgType].(map[string]any); ok {
		bgp[msgType] = eventContent
	} else {
		// Fallback: use result directly as event content
		bgp[msgType] = result
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

// makeZeEnvelope creates the Ze ze-bgp JSON envelope structure.
// Ze format: {"type": "bgp", "bgp": {"peer": {...}, "message": {..., "type": "<event>"}, "<event>": {...}}}.
// The message type can be determined either from message.type or by checking which key exists (open/update).
func makeZeEnvelope(msgType string) map[string]any {
	return map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"peer": map[string]any{
				"address": "127.0.0.1",
				"asn":     65533,
			},
			"message": map[string]any{
				"id":        0,
				"direction": "received",
				"type":      msgType,
			},
		},
	}
}

// decodeOpenMessage decodes a BGP OPEN message and returns Ze format.
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
	caps := capability.ParseFromOptionalParams(open.OptionalParams)

	// Determine ASN (use ASN4 if available)
	asn := uint32(open.MyAS)
	for _, c := range caps {
		if asn4, ok := c.(*capability.ASN4); ok {
			asn = asn4.ASN
			break
		}
	}

	// Ze format: capabilities as array of objects with code, name, value
	capsArray := make([]map[string]any, 0, len(caps))
	for _, c := range caps {
		capJSON := capabilityToZeJSON(c, plugins)
		capsArray = append(capsArray, capJSON)
	}

	// Ze format: open event content
	openContent := map[string]any{
		"asn":          asn,
		"router-id":    open.RouterID(),
		"hold-time":    open.HoldTime,
		"capabilities": capsArray,
	}

	return map[string]any{"open": openContent}, nil
}

// pluginCapabilityMap maps capability codes to plugin names.
// Populated from plugin registry at init time.
var pluginCapabilityMap = registry.CapabilityMap()

// pluginFamilyMap maps address families to plugin names for CLI decode.
// Populated from plugin registry at init time.
var pluginFamilyMap = registry.FamilyMap()

// capabilityToZeJSON converts a capability to Ze ze-bgp JSON format.
// Ze format: {"code": N, "name": "...", "value": "..."}.
func capabilityToZeJSON(c capability.Capability, plugins []string) map[string]any {
	code := int(c.Code())

	switch cap := c.(type) {
	case *capability.Multiprotocol:
		return map[string]any{"code": code, "name": "multiprotocol", "value": cap.AFI.String() + "/" + cap.SAFI.String()}
	case *capability.ASN4:
		return map[string]any{"code": code, "name": "asn4", "value": fmt.Sprintf("%d", cap.ASN)}
	case *capability.RouteRefresh:
		return map[string]any{"code": code, "name": "route-refresh"}
	case *capability.ExtendedMessage:
		return map[string]any{"code": code, "name": "extended-message"}
	case *capability.AddPath:
		families := make([]string, len(cap.Families))
		for i, f := range cap.Families {
			families[i] = fmt.Sprintf("%s/%s", f.AFI.String(), f.SAFI.String())
		}
		return map[string]any{"code": code, "name": "add-path", "value": families}
	case *capability.GracefulRestart:
		return map[string]any{"code": code, "name": "graceful-restart", "restart-time": cap.RestartTime}
	case *capability.SoftwareVersion:
		return map[string]any{"code": code, "name": "software-version", "value": cap.Version}
	}
	// Unknown capability type - try plugin decode or return raw
	return unknownCapabilityZe(c, plugins)
}

// unknownCapabilityZe returns Ze format JSON for an unrecognized/plugin-required capability.
func unknownCapabilityZe(c capability.Capability, plugins []string) map[string]any {
	code := int(c.Code())
	raw := make([]byte, c.Len())
	c.WriteTo(raw, 0)
	var rawHex string
	if len(raw) >= 2 {
		rawHex = fmt.Sprintf("%X", raw[2:])
	}

	// Check if a plugin can decode this capability
	pluginName, hasPlugin := pluginCapabilityMap[uint8(c.Code())]
	if hasPlugin && hasPluginEnabled(plugins, pluginName) {
		result := invokePluginDecode(pluginName, uint8(c.Code()), rawHex)
		if result != nil {
			result["code"] = code
			return result
		}
	}

	return map[string]any{"code": code, "name": "unknown", "raw": rawHex}
}

// PluginMode represents how a plugin should be invoked.
type PluginMode int

const (
	// ModeFork spawns a subprocess via exec.
	ModeFork PluginMode = iota
	// ModeInternal runs the plugin in a goroutine with pipes.
	ModeInternal
	// ModeDirect calls the decode function synchronously.
	ModeDirect
)

// parsePluginName extracts the plugin name, invocation mode, and optional path from input.
//
// Syntax:
//   - "name" → (name, ModeFork, "") - subprocess
//   - "ze.name" → (name, ModeInternal, "") - goroutine + pipe
//   - "ze-name" → (name, ModeDirect, "", nil) - sync in-process
//   - "/path/to/prog" → ("", ModeFork, path, nil) - execute path directly
//   - "/path/to/prog --arg" → ("", ModeFork, path, ["--arg"]) - path with args
func parsePluginName(input string) (name string, mode PluginMode, path string, args []string) {
	if strings.HasPrefix(input, "ze.") {
		return strings.TrimPrefix(input, "ze."), ModeInternal, "", nil
	}
	if strings.HasPrefix(input, "ze-") {
		return strings.TrimPrefix(input, "ze-"), ModeDirect, "", nil
	}
	if strings.Contains(input, "/") {
		// Path mode: split into binary path and arguments.
		parts := strings.Fields(input)
		if len(parts) == 1 {
			return "", ModeFork, input, nil
		}
		return "", ModeFork, parts[0], parts[1:]
	}
	// Plain name: fork subprocess.
	return input, ModeFork, "", nil
}

// hasPluginEnabled checks if a plugin is in the enabled list.
// Accepts "ze.name", "ze-name", and "name" formats.
func hasPluginEnabled(plugins []string, name string) bool {
	for _, p := range plugins {
		pName, _, _, _ := parsePluginName(p)
		if pName == name || p == name {
			return true
		}
	}
	return false
}

// invokePluginDecodeRequest spawns a plugin in decode mode and sends a decode request.
// Returns decoded JSON map or nil if decoding failed.
// The request format varies: "decode capability <code> <hex>" or "decode nlri <family> <hex>".
func invokePluginDecodeRequest(pluginName, request string) map[string]any {
	// Skip subprocess spawning during tests - os.Args[0] is the test binary
	// which would cause recursive test execution (fork bomb).
	if strings.HasSuffix(os.Args[0], ".test") {
		slog.Debug("plugin subprocess skipped in test environment", "plugin", pluginName)
		return nil
	}

	// Build plugin command - pluginName comes from fixed maps (pluginCapabilityMap, pluginFamilyMap)
	args := []string{"bgp", "plugin", pluginName, "--decode"}

	// Create command with timeout context for subprocess decode operation.
	// 5s allows for process startup chain (sh -> wrapper -> ze bgp plugin --decode).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], args...) //nolint:gosec // pluginName from fixed map

	stdin, err := cmd.StdinPipe()
	if err != nil {
		slog.Debug("plugin stdin pipe failed", "plugin", pluginName, "err", err)
		return nil
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		slog.Debug("plugin stdout pipe failed", "plugin", pluginName, "err", err)
		return nil
	}

	if err := cmd.Start(); err != nil {
		slog.Debug("plugin start failed", "plugin", pluginName, "err", err)
		return nil
	}

	// Send decode request
	if _, err := stdin.Write([]byte(request + "\n")); err != nil {
		slog.Debug("plugin write failed", "plugin", pluginName, "err", err)
	}
	if err := stdin.Close(); err != nil {
		slog.Debug("plugin stdin close failed", "plugin", pluginName, "err", err)
	}

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

// invokePluginNLRIDecode invokes a plugin to decode NLRI.
// Routes by mode based on plugin name syntax:
//   - "name" → Fork subprocess (with in-process retry)
//   - "ze.name" → Internal (goroutine + pipe)
//   - "ze-name" → Direct (sync in-process)
//   - "/path" → Fork external binary
func invokePluginNLRIDecode(pluginName, family, hexData string) any {
	request := fmt.Sprintf("decode nlri %s %s", family, hexData)
	name, mode, path, args := parsePluginName(pluginName)

	switch mode {
	case ModeInternal:
		return invokePluginInternal(name, request)
	case ModeDirect:
		return invokePluginInProcess(name, request)
	case ModeFork:
		if path != "" {
			return invokePluginPath(path, args, request)
		}
		return invokePluginNLRIDecodeRequest(name, request)
	}
	return nil
}

// invokePluginNLRIDecodeRequest spawns a built-in plugin subprocess.
// For plain names (ModeFork without path), retries with in-process on failure.
func invokePluginNLRIDecodeRequest(pluginName, request string) any {
	// Try subprocess first.
	result := invokePluginSubprocess(pluginName, request)
	if result != nil {
		return result
	}

	// Retry: in-process decode (for tests where subprocess can't run).
	return invokePluginInProcess(pluginName, request)
}

// invokePluginPath executes an external plugin binary at the given path.
// User-provided args are passed before the mandatory --decode flag.
func invokePluginPath(path string, userArgs []string, request string) any {
	// 5s allows for process startup chain (sh -> wrapper -> plugin binary --decode).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Build args: user args + --decode
	cmdArgs := append(userArgs, "--decode")           //nolint:gocritic // intentional append to new slice
	cmd := exec.CommandContext(ctx, path, cmdArgs...) //nolint:gosec // path from user input

	stdin, err := cmd.StdinPipe()
	if err != nil {
		slog.Debug("plugin path stdin failed", "path", path, "err", err)
		return nil
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		slog.Debug("plugin path stdout failed", "path", path, "err", err)
		return nil
	}

	if err := cmd.Start(); err != nil {
		slog.Debug("plugin path start failed", "path", path, "err", err)
		return nil
	}

	if _, err := stdin.Write([]byte(request + "\n")); err != nil {
		slog.Debug("plugin path write failed", "path", path, "err", err)
	}
	if err := stdin.Close(); err != nil {
		slog.Debug("plugin path stdin close failed", "path", path, "err", err)
	}

	scanner := bufio.NewScanner(stdout)
	if scanner.Scan() {
		line := scanner.Text()
		if result := parseDecodedJSON(line); result != nil {
			if err := cmd.Wait(); err != nil {
				slog.Debug("plugin path wait failed", "path", path, "err", err)
			}
			return result
		}
	}

	if err := cmd.Wait(); err != nil {
		slog.Debug("plugin path wait failed", "path", path, "err", err)
	}
	return nil
}

// parseDecodedJSON extracts JSON from "decoded json <json>" response.
func parseDecodedJSON(line string) any {
	if !strings.HasPrefix(line, "decoded json ") {
		return nil
	}
	jsonStr := strings.TrimPrefix(line, "decoded json ")
	var arrResult []any
	if err := json.Unmarshal([]byte(jsonStr), &arrResult); err == nil {
		return arrResult
	}
	var mapResult map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &mapResult); err == nil {
		return mapResult
	}
	return nil
}

// invokePluginSubprocess spawns a plugin subprocess for NLRI decode.
func invokePluginSubprocess(pluginName, request string) any {
	// Skip subprocess spawning during tests - os.Args[0] is the test binary
	// which would cause recursive test execution (fork bomb).
	if strings.HasSuffix(os.Args[0], ".test") {
		slog.Debug("plugin subprocess skipped in test environment", "plugin", pluginName)
		return nil // Fall back to in-process via invokePluginNLRIDecodeRequest
	}

	args := []string{"bgp", "plugin", pluginName, "--decode"}

	// 5s allows for process startup chain (sh -> wrapper -> ze bgp plugin --decode).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], args...) //nolint:gosec // pluginName from fixed map

	stdin, err := cmd.StdinPipe()
	if err != nil {
		slog.Debug("plugin stdin pipe failed", "plugin", pluginName, "err", err)
		return nil
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		slog.Debug("plugin stdout pipe failed", "plugin", pluginName, "err", err)
		return nil
	}

	if err := cmd.Start(); err != nil {
		slog.Debug("plugin start failed", "plugin", pluginName, "err", err)
		return nil
	}

	if _, err := stdin.Write([]byte(request + "\n")); err != nil {
		slog.Debug("plugin write failed", "plugin", pluginName, "err", err)
	}
	if err := stdin.Close(); err != nil {
		slog.Debug("plugin stdin close failed", "plugin", pluginName, "err", err)
	}

	scanner := bufio.NewScanner(stdout)
	if scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "decoded json ") {
			jsonStr := strings.TrimPrefix(line, "decoded json ")
			// Try array first (EVPN returns array), then map (FlowSpec returns map)
			var arrResult []any
			if err := json.Unmarshal([]byte(jsonStr), &arrResult); err == nil {
				_ = cmd.Wait()
				return arrResult
			}
			var mapResult map[string]any
			if err := json.Unmarshal([]byte(jsonStr), &mapResult); err == nil {
				_ = cmd.Wait()
				return mapResult
			}
			slog.Debug("plugin json parse failed", "plugin", pluginName, "json", jsonStr)
		}
	}

	_ = cmd.Wait()
	return nil
}

// inProcessDecoders maps plugin names to their in-process decode functions.
// Populated from plugin registry at init time.
var inProcessDecoders = registry.InProcessDecoders()

// invokePluginInProcess runs plugin decode in-process (Direct mode: ze-name).
// Synchronous, blocking - fastest invocation for CLI decode.
func invokePluginInProcess(pluginName, request string) any {
	decoder, ok := inProcessDecoders[pluginName]
	if !ok {
		return nil
	}

	input := bytes.NewBufferString(request + "\n")
	output := &bytes.Buffer{}

	decoder(input, output)

	return parsePluginResponse(output.String())
}

// invokePluginInternal runs decode via goroutine + pipes (Internal mode: ze.name).
// Uses decode-only runners (same as Direct) but with async pipe I/O.
func invokePluginInternal(pluginName, request string) any {
	decoder, ok := inProcessDecoders[pluginName]
	if !ok {
		slog.Debug("internal plugin decoder not available", "plugin", pluginName)
		return nil
	}

	// Create pipes for communication.
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	// Run decoder in goroutine.
	done := make(chan int, 1)
	go func() {
		// Wrap pipes in buffers for decoder interface.
		inBuf := &bytes.Buffer{}
		if _, err := io.Copy(inBuf, inR); err != nil {
			slog.Debug("internal plugin read input failed", "plugin", pluginName, "err", err)
		}
		outBuf := &bytes.Buffer{}
		exitCode := decoder(inBuf, outBuf)
		if _, err := outW.Write(outBuf.Bytes()); err != nil {
			slog.Debug("internal plugin write output failed", "plugin", pluginName, "err", err)
		}
		if err := outW.Close(); err != nil {
			slog.Debug("internal plugin outW close failed", "plugin", pluginName, "err", err)
		}
		done <- exitCode
	}()

	// Send request and close input.
	if _, err := inW.Write([]byte(request + "\n")); err != nil {
		slog.Debug("internal plugin write failed", "plugin", pluginName, "err", err)
		if err := inW.Close(); err != nil {
			slog.Debug("internal plugin inW close failed", "plugin", pluginName, "err", err)
		}
		return nil
	}
	if err := inW.Close(); err != nil {
		slog.Debug("internal plugin inW close failed", "plugin", pluginName, "err", err)
	}

	// Read response.
	var output bytes.Buffer
	if _, err := io.Copy(&output, outR); err != nil {
		slog.Debug("internal plugin read failed", "plugin", pluginName, "err", err)
	}

	<-done // Wait for decoder to finish.

	return parsePluginResponse(output.String())
}

// parsePluginResponse parses the "decoded json ..." response from a plugin.
func parsePluginResponse(output string) any {
	line := strings.TrimSpace(output)
	if strings.HasPrefix(line, "decoded json ") {
		jsonStr := strings.TrimPrefix(line, "decoded json ")
		var arrResult []any
		if err := json.Unmarshal([]byte(jsonStr), &arrResult); err == nil {
			return arrResult
		}
		var mapResult map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &mapResult); err == nil {
			return mapResult
		}
	}
	return nil
}

// validateDecodeFamily validates a family string has valid AFI/SAFI format.
// Families are registered dynamically by plugins, so this validates format only:
// must be non-empty and contain "afi/safi" structure.
func validateDecodeFamily(family string) error {
	if family == "" {
		return fmt.Errorf("empty address family")
	}
	parts := strings.SplitN(family, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid address family format: %s (expected afi/safi)", family)
	}
	return nil
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

// decodeUpdateMessage decodes a BGP UPDATE message and returns Ze format.
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

	// Build Ze format update content
	updateContent := map[string]any{}

	// Parse path attributes - Ze format uses "attr" key
	attrs, mpReach, mpUnreach := parsePathAttributesZe(update.PathAttributes)

	// Extract and remove internal next-hop field (used for NLRI operations)
	nextHop := "0.0.0.0"
	if nh, ok := attrs["_next-hop"].(string); ok {
		nextHop = nh
		delete(attrs, "_next-hop")
	}

	if len(attrs) > 0 {
		updateContent["attr"] = attrs
	}

	// Ze format: family is direct key under update (no "nlri" wrapper)
	// Handle MP_REACH_NLRI (announcements)
	if mpReach != nil {
		family, ops := buildMPReachZe(mpReach)
		if family != "" && len(ops) > 0 {
			updateContent[family] = ops
		}
	}

	// Handle MP_UNREACH_NLRI (withdrawals)
	if mpUnreach != nil {
		family, ops := buildMPUnreachZe(mpUnreach)
		if family != "" && len(ops) > 0 {
			if existing, ok := updateContent[family].([]map[string]any); ok {
				updateContent[family] = append(existing, ops...)
			} else {
				updateContent[family] = ops
			}
		}
	}

	// Handle IPv4 withdrawn routes
	if len(update.WithdrawnRoutes) > 0 {
		prefixes := parseIPv4Prefixes(update.WithdrawnRoutes)
		if len(prefixes) > 0 {
			withdrawOp := map[string]any{"action": "del", "nlri": prefixes}
			if existing, ok := updateContent["ipv4/unicast"].([]map[string]any); ok {
				updateContent["ipv4/unicast"] = append(existing, withdrawOp)
			} else {
				updateContent["ipv4/unicast"] = []map[string]any{withdrawOp}
			}
		}
	}

	// Handle IPv4 NLRI (announcements)
	if len(update.NLRI) > 0 {
		prefixes := parseIPv4Prefixes(update.NLRI)
		if len(prefixes) > 0 {
			announceOp := map[string]any{"next-hop": nextHop, "action": "add", "nlri": prefixes}
			if existing, ok := updateContent["ipv4/unicast"].([]map[string]any); ok {
				updateContent["ipv4/unicast"] = append(existing, announceOp)
			} else {
				updateContent["ipv4/unicast"] = []map[string]any{announceOp}
			}
		}
	}

	return map[string]any{"update": updateContent}, nil
}

// parsePathAttributesZe parses path attributes for Ze format (uses simple AS_PATH array).
func parsePathAttributesZe(data []byte) (attrs map[string]any, mpReach, mpUnreach []byte) {
	attrs = make(map[string]any)
	offset := 0

	for offset < len(data) {
		if offset+2 > len(data) {
			break
		}

		flags := data[offset]
		code := data[offset+1]

		hdrLen := 3
		var valueLen int
		if flags&0x10 != 0 {
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
		case 2: // AS_PATH - Ze format uses simple array
			asPath := parseASPathZe(value)
			if len(asPath) > 0 {
				attrs["as-path"] = asPath
			}
		case 3: // NEXT_HOP
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
				asn := binary.BigEndian.Uint16(value[0:2])
				ip := fmt.Sprintf("%d.%d.%d.%d", value[2], value[3], value[4], value[5])
				attrs["aggregator"] = fmt.Sprintf("%d:%s", asn, ip)
			} else if len(value) == 8 {
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
				clusters = append(clusters, fmt.Sprintf("%d.%d.%d.%d", value[i], value[i+1], value[i+2], value[i+3]))
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
		case 29: // BGP-LS Attribute
			bgplsAttr := parseBGPLSAttribute(value)
			if len(bgplsAttr) > 0 {
				attrs["bgp-ls"] = bgplsAttr
			}
		}

		offset += hdrLen + valueLen
	}

	return attrs, mpReach, mpUnreach
}

// parseASPathZe parses AS_PATH attribute value into Ze format (simple array).
func parseASPathZe(data []byte) []uint32 {
	var result []uint32
	offset := 0

	for offset < len(data) {
		if offset+2 > len(data) {
			break
		}

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

		for i := 0; i < segLen; i++ {
			var asn uint32
			if asnSize == 4 {
				asn = binary.BigEndian.Uint32(data[offset : offset+4])
			} else {
				asn = uint32(binary.BigEndian.Uint16(data[offset : offset+2]))
			}
			result = append(result, asn)
			offset += asnSize
		}
	}

	return result
}

// buildMPReachZe builds Ze format NLRI operations from MP_REACH_NLRI.
func buildMPReachZe(mpReach []byte) (string, []map[string]any) {
	if len(mpReach) < 5 {
		return "", nil
	}

	afi := nlri.AFI(binary.BigEndian.Uint16(mpReach[0:2]))
	safi := nlri.SAFI(mpReach[2])
	nhLen := int(mpReach[3])

	if len(mpReach) < 4+nhLen+1 {
		return "", nil
	}

	nhData := mpReach[4 : 4+nhLen]
	nextHop := parseNextHop(nhData, afi)

	nlriOffset := 4 + nhLen + 1
	if nlriOffset >= len(mpReach) {
		return "", nil
	}

	nlriData := mpReach[nlriOffset:]
	familyKey := formatFamily(afi, safi)

	routes := parseNLRIByFamily(nlriData, afi, safi, false)
	if len(routes) == 0 {
		return "", nil
	}

	// Ze format: array of operations with action/next-hop/nlri
	op := map[string]any{
		"next-hop": nextHop,
		"action":   "add",
		"nlri":     routes,
	}

	return familyKey, []map[string]any{op}
}

// buildMPUnreachZe builds Ze format NLRI operations from MP_UNREACH_NLRI.
func buildMPUnreachZe(mpUnreach []byte) (string, []map[string]any) {
	if len(mpUnreach) < 3 {
		return "", nil
	}

	afi := nlri.AFI(binary.BigEndian.Uint16(mpUnreach[0:2]))
	safi := nlri.SAFI(mpUnreach[2])

	if len(mpUnreach) <= 3 {
		return "", nil
	}

	nlriData := mpUnreach[3:]
	familyKey := formatFamily(afi, safi)

	routes := parseNLRIByFamily(nlriData, afi, safi, true)
	if len(routes) == 0 {
		return "", nil
	}

	// Ze format: withdraw operation
	op := map[string]any{
		"action": "del",
		"nlri":   routes,
	}

	return familyKey, []map[string]any{op}
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
		// EVPN decoding delegated to plugin
		family := nlri.Family{AFI: afi, SAFI: safi}.String()
		hexData := fmt.Sprintf("%X", data)
		result := invokePluginNLRIDecode("evpn", family, hexData)
		if result != nil {
			// Result can be array (multiple NLRIs) or map (single NLRI)
			if arr, ok := result.([]any); ok {
				routes = arr
			} else {
				routes = []any{result}
			}
		} else {
			// Plugin failed or unavailable - return raw bytes
			routes = []any{map[string]any{"parsed": false, "raw": hexData}}
		}
	case safi == nlri.SAFIFlowSpec || safi == nlri.SAFIFlowSpecVPN:
		// FlowSpec decoding delegated to plugin
		family := nlri.Family{AFI: afi, SAFI: safi}.String()
		hexData := fmt.Sprintf("%X", data)
		result := invokePluginNLRIDecode("flowspec", family, hexData)
		if result != nil {
			// Result can be array (multiple NLRIs) or map (single NLRI)
			if arr, ok := result.([]any); ok {
				routes = arr
			} else {
				routes = []any{result}
			}
		} else {
			// Plugin failed or unavailable - return raw bytes
			routes = []any{map[string]any{"parsed": false, "raw": hexData}}
		}
	case afi == nlri.AFIBGPLS:
		// BGP-LS decoding delegated to plugin
		family := nlri.Family{AFI: afi, SAFI: safi}.String()
		hexData := fmt.Sprintf("%X", data)
		result := invokePluginNLRIDecode("bgpls", family, hexData)
		if result != nil {
			if arr, ok := result.([]any); ok {
				routes = arr
			} else {
				routes = []any{result}
			}
		} else {
			routes = []any{map[string]any{"parsed": false, "raw": hexData}}
		}
	case safi == nlri.SAFIVPN:
		// VPN decoding delegated to plugin (RFC 4364, 4659)
		family := nlri.Family{AFI: afi, SAFI: safi}.String()
		hexData := fmt.Sprintf("%X", data)
		result := invokePluginNLRIDecode("vpn", family, hexData)
		if result != nil {
			if arr, ok := result.([]any); ok {
				routes = arr
			} else {
				routes = []any{result}
			}
		} else {
			routes = []any{map[string]any{"parsed": false, "raw": hexData}}
		}
	default: // IPv4/IPv6 unicast/multicast - simple prefix format
		routes = parseGenericNLRI(data, afi)
	}

	return routes
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
// Returns a slice of prefix strings (e.g., ["10.0.0.0/24", "2001::1/128"]).
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

		// Return plain prefix string (consistent with IPv4 unicast format)
		routes = append(routes, prefix)
		offset += byteLen
	}

	return routes
}

// decodeNLRIOnly decodes NLRI without envelope.
// If a matching plugin is enabled, it will be invoked for decoding.
// If outputJSON is false, returns human-readable format.
func decodeNLRIOnly(data []byte, family string, plugins []string, outputJSON bool) (string, error) {
	// Validate family against known AFI/SAFI combinations
	if err := validateDecodeFamily(family); err != nil {
		return "", err
	}

	// Try plugin decode first if plugin is enabled for this family
	pluginName := lookupFamilyPlugin(family, plugins)
	if pluginName != "" {
		hexData := fmt.Sprintf("%X", data)
		result := invokePluginNLRIDecode(pluginName, family, hexData)
		if result != nil {
			if !outputJSON {
				// Handle both array and map results
				if mapResult, ok := result.(map[string]any); ok {
					return formatNLRIHuman(mapResult, family), nil
				}
				if arrResult, ok := result.([]any); ok && len(arrResult) > 0 {
					if firstMap, ok := arrResult[0].(map[string]any); ok {
						return formatNLRIHuman(firstMap, family), nil
					}
				}
			}
			jsonData, err := json.Marshal(result)
			if err != nil {
				return "", fmt.Errorf("json marshal: %w", err)
			}
			return string(jsonData), nil
		}
		// Plugin failed, fall through to built-in decode
	}

	// Plugin failed or unknown family - return raw bytes
	result := map[string]any{
		"parsed": false,
		"raw":    fmt.Sprintf("%X", data),
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
