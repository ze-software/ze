package config

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/message"
)

// validateProcessCapabilities checks that peers with route-refresh or graceful-restart
// capabilities have a process binding with Send.Update = true.
// These capabilities require a process to resend routes, and without one the engine
// cannot fulfill route-refresh requests or graceful restart route replay.
func validateProcessCapabilities(peers []PeerConfig) error {
	for _, peer := range peers {
		needsProcess := peer.Capabilities.RouteRefresh || peer.Capabilities.GracefulRestart
		if !needsProcess {
			continue
		}

		// Check if any process binding has Send.Update = true
		hasValidProcess := false
		for _, binding := range peer.ProcessBindings {
			if binding.Send.Update {
				hasValidProcess = true
				break
			}
		}

		if hasValidProcess {
			continue
		}

		// Determine which capability requires the process
		capName := "route-refresh"
		if !peer.Capabilities.RouteRefresh {
			capName = "graceful-restart"
		}

		// Build error message
		if len(peer.ProcessBindings) == 0 {
			return fmt.Errorf("peer %s: %s requires process with send { update; }\n  no process bindings configured",
				peer.Address, capName)
		}

		// List configured processes
		var names []string
		for _, binding := range peer.ProcessBindings {
			names = append(names, "process "+binding.PluginName)
		}
		return fmt.Errorf("peer %s: %s requires process with send { update; }\n  configured: %s - none have send { update; }",
			peer.Address, capName, strings.Join(names, ", "))
	}
	return nil
}

// applyTreeSettings applies settings from a tree (match block, template, or peer glob)
// to a PeerConfig. Only explicitly set values are applied.
func applyTreeSettings(nc *PeerConfig, tree *Tree) error {
	// Hold time
	if v, ok := tree.Get("hold-time"); ok {
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return fmt.Errorf("invalid hold-time: %w", err)
		}
		// RFC 4271 Section 4.2: Hold Time MUST be either zero or at least three seconds
		if n >= 1 && n <= 2 {
			return fmt.Errorf("invalid hold-time %d: RFC 4271 requires 0 or >= 3 seconds", n)
		}
		nc.HoldTime = uint16(n)
	}

	// Peer AS
	if v, ok := tree.Get("peer-as"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid peer-as: %w", err)
		}
		nc.PeerAS = uint32(n)
	}

	// Local AS
	if v, ok := tree.Get("local-as"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid local-as: %w", err)
		}
		nc.LocalAS = uint32(n)
	}

	// Description
	if v, ok := tree.Get("description"); ok {
		nc.Description = v
	}

	// Router ID
	if v, ok := tree.Get("router-id"); ok {
		ip, err := netip.ParseAddr(v)
		if err != nil {
			return fmt.Errorf("invalid router-id: %w", err)
		}
		nc.RouterID = ipToUint32(ip)
	}

	// Local address
	if v, ok := tree.Get("local-address"); ok {
		if v == "auto" {
			nc.LocalAddressAuto = true
		} else {
			ip, err := netip.ParseAddr(v)
			if err != nil {
				return fmt.Errorf("invalid local-address: %w", err)
			}
			nc.LocalAddress = ip
		}
	}

	// RFC 2545 Section 3: IPv6 link-local address for MP_REACH next-hop
	if v, ok := tree.Get("link-local"); ok {
		ip, err := netip.ParseAddr(v)
		if err != nil {
			return fmt.Errorf("invalid link-local: %w", err)
		}
		nc.LinkLocal = ip
	}

	// Passive
	if v, ok := tree.Get("passive"); ok {
		nc.Passive = v == configTrue
	}

	// Group updates
	if v, ok := tree.Get("group-updates"); ok {
		nc.GroupUpdates = v == configTrue
		nc.RIBOut.GroupUpdates = v == configTrue
	}

	// RIBOut config
	if ribOut, err := parseRIBOutConfig(tree); err != nil {
		return fmt.Errorf("rib: %w", err)
	} else {
		applyRIBOutParseResult(&nc.RIBOut, ribOut)
	}

	// Capabilities
	if cap := tree.GetContainer("capability"); cap != nil {
		if v, ok := cap.Get("asn4"); ok {
			nc.Capabilities.ASN4 = v == configTrue
		}
		// route-refresh is Flex type, use GetFlex
		if v, ok := cap.GetFlex("route-refresh"); ok {
			nc.Capabilities.RouteRefresh = v == configTrue || v == configEnable
		}
		if gr := cap.GetContainer("graceful-restart"); gr != nil {
			nc.Capabilities.GracefulRestart = true
			if v, ok := gr.Get("restart-time"); ok {
				n, _ := strconv.ParseUint(v, 10, 16)
				nc.Capabilities.RestartTime = uint16(n)
				// Store raw value for plugin delivery
				if nc.RawCapabilityConfig == nil {
					nc.RawCapabilityConfig = make(map[string]map[string]string)
				}
				if nc.RawCapabilityConfig["graceful-restart"] == nil {
					nc.RawCapabilityConfig["graceful-restart"] = make(map[string]string)
				}
				nc.RawCapabilityConfig["graceful-restart"]["restart-time"] = v
			}
		}
		// Handle add-path as value (e.g., "add-path send/receive;")
		if v, ok := cap.GetFlex("add-path"); ok && v != "" {
			switch v {
			case addPathSendReceive, addPathReceiveSend:
				nc.Capabilities.AddPathSend = true
				nc.Capabilities.AddPathReceive = true
			case addPathSend:
				nc.Capabilities.AddPathSend = true
			case addPathReceive:
				nc.Capabilities.AddPathReceive = true
			}
		}
		// Handle add-path as block (e.g., "add-path { send; receive; }")
		if ap := cap.GetContainer("add-path"); ap != nil {
			if v, ok := ap.Get(addPathSend); ok {
				nc.Capabilities.AddPathSend = v == configTrue
			}
			if v, ok := ap.Get(addPathReceive); ok {
				nc.Capabilities.AddPathReceive = v == configTrue
			}
		}
		if v, ok := cap.GetFlex("extended-message"); ok {
			nc.Capabilities.ExtendedMessage = v == configTrue || v == configEnable
		}
		if v, ok := cap.GetFlex("software-version"); ok {
			nc.Capabilities.SoftwareVersion = v == configTrue || v == configEnable
		}
		// RFC 8950: Parse nexthop { ... } block for extended next-hop families.
		// Format: capability { nexthop { ipv4/unicast ipv6; ipv4/mpls-vpn ipv6; } }
		if nhBlock := cap.GetContainer("nexthop"); nhBlock != nil {
			nc.NexthopFamilies = parseNexthopFamilies(nhBlock)
		}
	}

	return nil
}

func parsePeerConfig(addr string, tree *Tree, templates map[string]*Tree, templatePatterns map[string]string, peerGlobs []PeerGlob) (PeerConfig, error) {
	nc := PeerConfig{}

	// Set hold-time default (RFC 4271 Section 10).
	nc.HoldTime = DefaultHoldTime

	// Set default capability values (ASN4 enabled by default per RFC 6793).
	nc.Capabilities.ASN4 = true

	// RIBOut defaults - set early so peer globs can override
	nc.RIBOut = DefaultRIBOutConfig()

	// Address
	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return nc, fmt.Errorf("invalid address: %w", err)
	}
	nc.Address = ip

	// Build precedence chain: [match blocks] -> [inherited templates] -> [peer config]
	// Each layer can override settings from previous layers

	// Layer 1: Apply matching peer globs / template.match blocks (in order)
	// This sets defaults that can be overridden by templates and neighbor config
	// Collect matching trees for reuse when processing API bindings later
	matchingTrees := make([]*Tree, 0, len(peerGlobs))
	for _, glob := range peerGlobs {
		if IPGlobMatch(glob.Pattern, addr) {
			matchingTrees = append(matchingTrees, glob.Tree)
			// Apply all settings from this match block
			if err := applyTreeSettings(&nc, glob.Tree); err != nil {
				return nc, fmt.Errorf("match %s: %w", glob.Pattern, err)
			}
			// Extract routes from match block
			routes, err := extractRoutesFromTree(glob.Tree)
			if err != nil {
				return nc, fmt.Errorf("match %s routes: %w", glob.Pattern, err)
			}
			nc.StaticRoutes = append(nc.StaticRoutes, routes.StaticRoutes...)
			nc.FlowSpecRoutes = append(nc.FlowSpecRoutes, routes.FlowSpecRoutes...)
			nc.VPLSRoutes = append(nc.VPLSRoutes, routes.VPLSRoutes...)
			nc.MVPNRoutes = append(nc.MVPNRoutes, routes.MVPNRoutes...)
			nc.MUPRoutes = append(nc.MUPRoutes, routes.MUPRoutes...)
		}
	}

	// Layer 2: Handle template inheritance (multiple inherit supported)
	// V3 supports multiple inherit statements, applied in order
	inheritedTemplates := make([]*Tree, 0)
	for _, entry := range tree.GetListOrdered("inherit") {
		// inherit is stored as a leaf value, key is the template name
		inheritName := entry.Key
		if t, exists := templates[inheritName]; exists {
			// Validate pattern if template has one
			if pattern, hasPattern := templatePatterns[inheritName]; hasPattern {
				if !IPGlobMatch(pattern, addr) {
					return nc, fmt.Errorf("inherit %q: peer %s does not match template pattern %q", inheritName, addr, pattern)
				}
			}
			inheritedTemplates = append(inheritedTemplates, t)
		} else {
			return nc, fmt.Errorf("inherit %q: template not found", inheritName)
		}
	}
	// Also check single inherit value (ExaBGP compatibility)
	if len(inheritedTemplates) == 0 {
		if inheritName, ok := tree.Get("inherit"); ok {
			if t, exists := templates[inheritName]; exists {
				// Validate pattern if template has one
				if pattern, hasPattern := templatePatterns[inheritName]; hasPattern {
					if !IPGlobMatch(pattern, addr) {
						return nc, fmt.Errorf("inherit %q: peer %s does not match template pattern %q", inheritName, addr, pattern)
					}
				}
				inheritedTemplates = append(inheritedTemplates, t)
			} else {
				return nc, fmt.Errorf("inherit %q: template not found", inheritName)
			}
		}
	}

	// Apply inherited templates in order
	for _, tmpl := range inheritedTemplates {
		if err := applyTreeSettings(&nc, tmpl); err != nil {
			return nc, fmt.Errorf("template: %w", err)
		}
		routes, err := extractRoutesFromTree(tmpl)
		if err != nil {
			return nc, fmt.Errorf("template routes: %w", err)
		}
		nc.StaticRoutes = append(nc.StaticRoutes, routes.StaticRoutes...)
		nc.FlowSpecRoutes = append(nc.FlowSpecRoutes, routes.FlowSpecRoutes...)
		nc.VPLSRoutes = append(nc.VPLSRoutes, routes.VPLSRoutes...)
		nc.MVPNRoutes = append(nc.MVPNRoutes, routes.MVPNRoutes...)
		nc.MUPRoutes = append(nc.MUPRoutes, routes.MUPRoutes...)
	}

	// Get last inherited template for getValue fallback (backward compat)
	var tmpl *Tree
	if len(inheritedTemplates) > 0 {
		tmpl = inheritedTemplates[len(inheritedTemplates)-1]
	}

	// Helper to get value from neighbor tree, falling back to template.
	getValue := func(key string) (string, bool) {
		if v, ok := tree.Get(key); ok {
			return v, true
		}
		if tmpl != nil {
			return tmpl.Get(key)
		}
		return "", false
	}

	// Simple fields (check template fallback for each)
	if v, ok := getValue("description"); ok {
		nc.Description = v
	}

	if v, ok := getValue("router-id"); ok {
		ip, err := netip.ParseAddr(v)
		if err != nil {
			return nc, fmt.Errorf("invalid router-id: %w", err)
		}
		nc.RouterID = ipToUint32(ip)
	}

	if v, ok := getValue("local-address"); ok {
		if v == "auto" {
			nc.LocalAddressAuto = true
		} else {
			ip, err := netip.ParseAddr(v)
			if err != nil {
				return nc, fmt.Errorf("invalid local-address: %w", err)
			}
			nc.LocalAddress = ip
		}
	}

	// RFC 2545 Section 3: IPv6 link-local address for MP_REACH next-hop
	if v, ok := getValue("link-local"); ok {
		ip, err := netip.ParseAddr(v)
		if err != nil {
			return nc, fmt.Errorf("invalid link-local: %w", err)
		}
		nc.LinkLocal = ip
	}

	if v, ok := getValue("local-as"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return nc, fmt.Errorf("invalid local-as: %w", err)
		}
		nc.LocalAS = uint32(n)
	}

	if v, ok := getValue("peer-as"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return nc, fmt.Errorf("invalid peer-as: %w", err)
		}
		nc.PeerAS = uint32(n)
	}

	if v, ok := getValue("hold-time"); ok {
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return nc, fmt.Errorf("invalid hold-time: %w", err)
		}
		// RFC 4271 Section 4.2: Hold Time MUST be either zero or at least three seconds
		if n >= 1 && n <= 2 {
			return nc, fmt.Errorf("invalid hold-time %d: RFC 4271 requires 0 or >= 3 seconds", n)
		}
		nc.HoldTime = uint16(n)
	}

	if v, ok := getValue("passive"); ok {
		nc.Passive = v == configTrue
	}

	// group-updates defaults to true, check template first then neighbor
	nc.GroupUpdates = true
	if tmpl != nil {
		if v, ok := tmpl.Get("group-updates"); ok {
			nc.GroupUpdates = v == configTrue
		}
	}
	if v, ok := tree.Get("group-updates"); ok {
		nc.GroupUpdates = v == configTrue
	}

	// host-name/domain-name are provided by hostname plugin's YANG schema.
	// When plugin is loaded, these fields become valid and are extracted
	// into RawCapabilityConfig for delivery to the plugin during Stage 2.
	if v, ok := tree.Get("host-name"); ok {
		if nc.RawCapabilityConfig == nil {
			nc.RawCapabilityConfig = make(map[string]map[string]string)
		}
		if nc.RawCapabilityConfig["hostname"] == nil {
			nc.RawCapabilityConfig["hostname"] = make(map[string]string)
		}
		nc.RawCapabilityConfig["hostname"]["host"] = v
	}
	if v, ok := tree.Get("domain-name"); ok {
		if nc.RawCapabilityConfig == nil {
			nc.RawCapabilityConfig = make(map[string]map[string]string)
		}
		if nc.RawCapabilityConfig["hostname"] == nil {
			nc.RawCapabilityConfig["hostname"] = make(map[string]string)
		}
		nc.RawCapabilityConfig["hostname"]["domain"] = v
	}

	// Families - FamilyBlock stores "ipv4/unicast" as key with mode as value
	// Also parse ignore-mismatch option from family block
	// Check template first, then override with neighbor values
	familyTree := tree.GetContainer("family")
	if familyTree == nil && tmpl != nil {
		familyTree = tmpl.GetContainer("family")
	}
	if familyTree != nil {
		for _, key := range familyTree.Values() {
			if strings.HasPrefix(key, "ignore-mismatch") {
				// Parse ignore-mismatch option (not a family)
				// Format: "ignore-mismatch [enable|true|disable|false]"
				parts := strings.Fields(key)
				if len(parts) == 2 {
					nc.IgnoreFamilyMismatch = parts[1] == configTrue || parts[1] == configEnable
				} else if len(parts) == 1 {
					// Just "ignore-mismatch" alone means enable
					nc.IgnoreFamilyMismatch = true
				}
			} else {
				// Regular address family - key is "AFI/SAFI", value is mode
				modeStr, _ := familyTree.Get(key)
				mode := ParseFamilyMode(modeStr)

				// Parse AFI and SAFI from key (format: afi/safi)
				parts := strings.SplitN(key, "/", 2)
				if len(parts) == 2 {
					fc := FamilyConfig{
						AFI:  parts[0],
						SAFI: parts[1],
						Mode: mode,
					}
					nc.FamilyConfigs = append(nc.FamilyConfigs, fc)

					// Also populate legacy Families for backward compatibility
					// (only for enabled families)
					if mode != FamilyModeDisable {
						nc.Families = append(nc.Families, key)
					}
				}
			}
		}
	}

	// Capabilities - check template first, then override with neighbor values
	cap := tree.GetContainer("capability")
	if cap == nil && tmpl != nil {
		cap = tmpl.GetContainer("capability")
	}
	if cap != nil {
		if v, ok := cap.Get("asn4"); ok {
			nc.Capabilities.ASN4 = v == configTrue
		}
		if v, ok := cap.Get("route-refresh"); ok {
			nc.Capabilities.RouteRefresh = v == configTrue
		}
		if gr := cap.GetContainer("graceful-restart"); gr != nil {
			nc.Capabilities.GracefulRestart = true
			if v, ok := gr.Get("restart-time"); ok {
				n, _ := strconv.ParseUint(v, 10, 16)
				nc.Capabilities.RestartTime = uint16(n)
				// Store raw value for plugin delivery
				if nc.RawCapabilityConfig == nil {
					nc.RawCapabilityConfig = make(map[string]map[string]string)
				}
				if nc.RawCapabilityConfig["graceful-restart"] == nil {
					nc.RawCapabilityConfig["graceful-restart"] = make(map[string]string)
				}
				nc.RawCapabilityConfig["graceful-restart"]["restart-time"] = v
			}
		}
		// Handle add-path as value (e.g., "add-path send/receive;")
		if v, ok := cap.GetFlex("add-path"); ok && v != "" {
			switch v {
			case addPathSendReceive, addPathReceiveSend:
				nc.Capabilities.AddPathSend = true
				nc.Capabilities.AddPathReceive = true
			case addPathSend:
				nc.Capabilities.AddPathSend = true
			case addPathReceive:
				nc.Capabilities.AddPathReceive = true
			}
		}
		// Handle add-path as block (e.g., "add-path { send; receive; }")
		if ap := cap.GetContainer("add-path"); ap != nil {
			if v, ok := ap.Get(addPathSend); ok {
				nc.Capabilities.AddPathSend = v == configTrue
			}
			if v, ok := ap.Get(addPathReceive); ok {
				nc.Capabilities.AddPathReceive = v == configTrue
			}
		}
		// RFC 8654 Extended Message capability
		if v, ok := cap.GetFlex("extended-message"); ok {
			nc.Capabilities.ExtendedMessage = v == configTrue || v == configEnable
		}
		if v, ok := cap.GetFlex("software-version"); ok {
			nc.Capabilities.SoftwareVersion = v == configTrue || v == configEnable
		}
		// RFC 8950: Parse nexthop { ... } block for extended next-hop families.
		// Format: capability { nexthop { ipv4/unicast ipv6; ipv4/mpls-vpn ipv6; } }
		if nhBlock := cap.GetContainer("nexthop"); nhBlock != nil {
			nc.NexthopFamilies = parseNexthopFamilies(nhBlock)
		}
		// draft-walton-bgp-hostname: Extract hostname from capability block.
		// Format: capability { hostname { host <name>; domain <domain>; } }
		// Populates RawCapabilityConfig for plugin config delivery (pattern-based).
		if hostname := cap.GetContainer("hostname"); hostname != nil {
			if nc.RawCapabilityConfig == nil {
				nc.RawCapabilityConfig = make(map[string]map[string]string)
			}
			if nc.RawCapabilityConfig["hostname"] == nil {
				nc.RawCapabilityConfig["hostname"] = make(map[string]string)
			}
			if v, ok := hostname.Get("host"); ok {
				nc.RawCapabilityConfig["hostname"]["host"] = v
			}
			if v, ok := hostname.Get("domain"); ok {
				nc.RawCapabilityConfig["hostname"]["domain"] = v
			}
		}
		// Convert entire capability container to JSON for plugin delivery.
		// Plugins receive full config as JSON and extract what they need.
		// This solves the "DESIGN ISSUE" from spec-hostname-plugin.md - no per-plugin extraction code needed.
		if capMap := cap.ToMap(); len(capMap) > 0 {
			if jsonBytes, err := json.Marshal(capMap); err == nil {
				nc.CapabilityConfigJSON = string(jsonBytes)
			}
		}
	}

	// Per-family add-path configuration (RFC 7911)
	// Format: add-path { ipv4/unicast send; ipv6/unicast receive; ipv4/multicast send/receive; }
	if addPath := tree.GetContainer("add-path"); addPath != nil {
		for _, key := range addPath.Values() {
			apf := parseAddPathFamily(key)
			if apf.Family != "" {
				nc.AddPathFamilies = append(nc.AddPathFamilies, apf)
			}
		}
	}

	// Extract routes from this neighbor's static and announce blocks (including update blocks)
	routes, err := extractRoutesFromTree(tree)
	if err != nil {
		return nc, err
	}
	nc.StaticRoutes = append(nc.StaticRoutes, routes.StaticRoutes...)
	nc.FlowSpecRoutes = append(nc.FlowSpecRoutes, routes.FlowSpecRoutes...)
	nc.VPLSRoutes = append(nc.VPLSRoutes, routes.VPLSRoutes...)
	nc.MVPNRoutes = append(nc.MVPNRoutes, routes.MVPNRoutes...)
	nc.MUPRoutes = append(nc.MUPRoutes, routes.MUPRoutes...)

	// Extract exotic routes from old ExaBGP syntax (flow, l2vpn, announce blocks)
	nc.MVPNRoutes = append(nc.MVPNRoutes, extractMVPNRoutes(tree)...)
	nc.VPLSRoutes = append(nc.VPLSRoutes, extractVPLSRoutes(tree)...)
	nc.FlowSpecRoutes = append(nc.FlowSpecRoutes, extractFlowSpecRoutes(tree)...)
	nc.MUPRoutes = append(nc.MUPRoutes, extractMUPRoutes(tree)...)

	// Apply template rib.out if present (peer globs already applied above)
	if tmpl != nil {
		if ribOut, err := parseRIBOutConfig(tmpl); err != nil {
			return nc, fmt.Errorf("template rib: %w", err)
		} else {
			applyRIBOutParseResult(&nc.RIBOut, ribOut)
		}
	}

	// Apply neighbor rib.out (overrides template)
	if ribOut, err := parseRIBOutConfig(tree); err != nil {
		return nc, fmt.Errorf("rib: %w", err)
	} else {
		applyRIBOutParseResult(&nc.RIBOut, ribOut)
	}

	// Sync legacy group-updates to RIBOut (if explicit)
	// Legacy neighbor group-updates takes final precedence for backward compat
	if v, ok := tree.Get("group-updates"); ok {
		nc.RIBOut.GroupUpdates = v == configTrue
	} else if tmpl != nil {
		if v, ok := tmpl.Get("group-updates"); ok {
			nc.RIBOut.GroupUpdates = v == configTrue
		}
	}

	// Parse API bindings - supports both old and new syntax:
	// Old: process { processes [ foo bar ]; receive { ... } } (migrated from "api")
	// New: process <plugin-name> { content { encoding json; } receive { update; } }
	//
	// Precedence: match templates → inherited templates → peer config
	// Each layer can override the previous.

	// Layer 1: Match templates (collected earlier in matchingTrees)
	for _, matchTree := range matchingTrees {
		matchBindings, err := parseProcessBindings(matchTree)
		if err != nil {
			return PeerConfig{}, fmt.Errorf("peer %s: %w", addr, err)
		}
		nc.ProcessBindings = mergeProcessBindings(nc.ProcessBindings, matchBindings)
	}

	// Layer 2: Inherited templates (later templates override earlier ones)
	for _, tmpl := range inheritedTemplates {
		tmplBindings, err := parseProcessBindings(tmpl)
		if err != nil {
			return PeerConfig{}, fmt.Errorf("peer %s: %w", addr, err)
		}
		nc.ProcessBindings = mergeProcessBindings(nc.ProcessBindings, tmplBindings)
	}

	// Layer 3: Peer bindings override all templates
	peerBindings, err := parseProcessBindings(tree)
	if err != nil {
		return PeerConfig{}, fmt.Errorf("peer %s: %w", addr, err)
	}
	nc.ProcessBindings = mergeProcessBindings(nc.ProcessBindings, peerBindings)

	return nc, nil
}

// ribOutParseResult holds parsed values with explicit "was set" tracking.
type ribOutParseResult struct {
	GroupUpdates       bool
	GroupUpdatesSet    bool
	AutoCommitDelay    time.Duration
	AutoCommitDelaySet bool
	MaxBatchSize       int
	MaxBatchSizeSet    bool
}

// parseRIBOutConfig extracts RIBOut settings from a tree's rib.out block.
// Returns a parse result that tracks which fields were explicitly set.
func parseRIBOutConfig(tree *Tree) (ribOutParseResult, error) {
	result := ribOutParseResult{}

	rib := tree.GetContainer("rib")
	if rib == nil {
		return result, nil
	}

	ribOut := rib.GetContainer("out")
	if ribOut == nil {
		return result, nil
	}

	if v, ok := ribOut.Get("group-updates"); ok {
		result.GroupUpdates = v == configTrue
		result.GroupUpdatesSet = true
	}
	if v, ok := ribOut.Get("auto-commit-delay"); ok {
		d, err := parseDurationValue(v)
		if err != nil {
			return result, fmt.Errorf("auto-commit-delay: %w", err)
		}
		result.AutoCommitDelay = d
		result.AutoCommitDelaySet = true
	}
	if v, ok := ribOut.Get("max-batch-size"); ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return result, fmt.Errorf("max-batch-size: %w", err)
		}
		result.MaxBatchSize = n
		result.MaxBatchSizeSet = true
	}

	return result, nil
}

// applyRIBOutParseResult applies parsed values to a config, only overriding
// fields that were explicitly set.
func applyRIBOutParseResult(cfg *RIBOutConfig, parsed ribOutParseResult) {
	if parsed.GroupUpdatesSet {
		cfg.GroupUpdates = parsed.GroupUpdates
	}
	if parsed.AutoCommitDelaySet {
		cfg.AutoCommitDelay = parsed.AutoCommitDelay
	}
	if parsed.MaxBatchSizeSet {
		cfg.MaxBatchSize = parsed.MaxBatchSize
	}
}

// parseProcessBindings parses process bindings from a peer tree.
// Supports both old and new syntax:
//   - Old: process { processes [ foo bar ]; } - uses KeyDefault (migrated from "api")
//   - New: process <plugin-name> { content { encoding json; } receive { update; } }
func parseProcessBindings(tree *Tree) ([]PeerProcessBinding, error) {
	var bindings []PeerProcessBinding

	// Schema defines process as List(TypeString, ...) - use GetList
	processList := tree.GetList("process")
	if len(processList) == 0 {
		return nil, nil
	}

	// Sort keys for deterministic order (maps iterate randomly)
	keys := make([]string, 0, len(processList))
	for k := range processList {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		processTree := processList[key]
		if key == KeyDefault {
			// Old syntax: process { processes [ foo bar ]; }
			bindings = append(bindings, parseOldProcessBindings(processTree)...)
		} else {
			// New syntax: process <plugin-name> { content {...} receive {...} send {...} }
			binding, err := parseNewProcessBinding(key, processTree)
			if err != nil {
				return nil, err
			}
			bindings = append(bindings, binding)
		}
	}

	return bindings, nil
}

// parseOldProcessBindings parses the old process { processes [...] } syntax.
// Also handles neighbor-changes flag which maps to receive.State.
func parseOldProcessBindings(processTree *Tree) []PeerProcessBinding {
	var bindings []PeerProcessBinding

	// Check for neighbor-changes flag (maps to receive.State)
	// Flex stores "neighbor-changes;" as GetFlex returning "true" or "enable"
	neighborChanges := false
	if v, ok := processTree.GetFlex("neighbor-changes"); ok {
		neighborChanges = v == configTrue || v == configEnable || v == ""
	}

	// Look for "processes" key with array value like "[ foo bar ]"
	// (old syntax referenced plugin names as "processes")
	if pluginsValue, ok := processTree.Get("processes"); ok {
		// Parse plugin names from "[ foo bar ]" or "foo bar" format
		pluginsValue = strings.Trim(pluginsValue, "[]")
		for _, pluginName := range strings.Fields(pluginsValue) {
			binding := PeerProcessBinding{PluginName: pluginName}
			if neighborChanges {
				binding.Receive.State = true
			}
			bindings = append(bindings, binding)
		}
	}

	return bindings
}

// parseNewProcessBinding parses a single process <plugin-name> { ... } binding.
func parseNewProcessBinding(pluginName string, processTree *Tree) (PeerProcessBinding, error) {
	binding := PeerProcessBinding{PluginName: pluginName}

	// Parse inline run command (defines plugin inline instead of referencing external)
	if v, ok := processTree.Get("run"); ok {
		binding.Run = v
	}

	// Parse content block: content { encoding json; format full; attribute ...; nlri ...; }
	if content := processTree.GetContainer("content"); content != nil {
		if v, ok := content.Get("encoding"); ok {
			binding.Content.Encoding = strings.ToLower(v) // Normalize case
		}
		if v, ok := content.Get("format"); ok {
			binding.Content.Format = strings.ToLower(v) // Normalize case
		}
		if v, ok := content.Get("attribute"); ok {
			filter, err := plugin.ParseAttributeFilter(v)
			if err != nil {
				return PeerProcessBinding{}, fmt.Errorf("process %s: invalid attribute filter: %w", pluginName, err)
			}
			binding.Content.Attributes = &filter
		}
		// Parse nlri entries: nlri ipv4/unicast; nlri ipv6/unicast;
		if nlriEntries := content.GetMultiValues("nlri"); len(nlriEntries) > 0 {
			filter, err := parseNLRIEntries(nlriEntries)
			if err != nil {
				return PeerProcessBinding{}, fmt.Errorf("process %s: invalid nlri filter: %w", pluginName, err)
			}
			binding.Content.NLRI = &filter
		}
	}

	// Parse receive block: receive { update; notification; all; }
	if recv := processTree.GetContainer("receive"); recv != nil {
		binding.Receive = parseReceiveConfig(recv)
	}

	// Parse send block: send { update; refresh; all; }
	if send := processTree.GetContainer("send"); send != nil {
		binding.Send = parseSendConfig(send)
	}

	return binding, nil
}

// parseNLRIEntries parses multiple "nlri <afi> <safi>;" entries into NLRIFilter.
// Each entry is a space-separated string like "ipv4/unicast" or "ipv6/unicast".
// Special values: "all" includes all families, "none" excludes all.
func parseNLRIEntries(entries []string) (plugin.NLRIFilter, error) {
	if len(entries) == 0 {
		return plugin.NewNLRIFilterAll(), nil
	}

	// Check for special keywords
	if len(entries) == 1 {
		entry := strings.TrimSpace(strings.ToLower(entries[0]))
		if entry == "all" {
			return plugin.NewNLRIFilterAll(), nil
		}
		if entry == "none" {
			return plugin.NewNLRIFilterNone(), nil
		}
	}

	// Parse each entry as "<afi> <safi>" and convert to hyphenated form
	families := make(map[string]bool, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(strings.ToLower(entry))
		if entry == "" {
			continue
		}

		// Validate against known families (format: afi/safi)
		canonical, ok := message.FamilyConfigNames[strings.ToLower(entry)]
		if !ok {
			return plugin.NLRIFilter{}, fmt.Errorf("unknown family %q, valid: %s",
				entry, message.ValidFamilyConfigNames())
		}
		families[canonical] = true
	}

	return plugin.NewNLRIFilterSelective(families), nil
}

// parseReceiveConfig parses a Freeform receive block.
// Freeform stores "update;" as key "update" -> value "true".
func parseReceiveConfig(tree *Tree) PeerReceiveConfig {
	cfg := PeerReceiveConfig{}

	// Check for "all" shorthand - sets all flags
	if _, ok := tree.Get("all"); ok {
		cfg.Update = true
		cfg.Open = true
		cfg.Notification = true
		cfg.Keepalive = true
		cfg.Refresh = true
		cfg.State = true
		cfg.Sent = true
		cfg.Negotiated = true
		return cfg
	}

	// Individual flags
	_, cfg.Update = tree.Get("update")
	_, cfg.Open = tree.Get("open")
	_, cfg.Notification = tree.Get("notification")
	_, cfg.Keepalive = tree.Get("keepalive")
	_, cfg.Refresh = tree.Get("refresh")
	_, cfg.State = tree.Get("state")
	_, cfg.Sent = tree.Get("sent")
	_, cfg.Negotiated = tree.Get("negotiated")

	return cfg
}

// parseSendConfig parses a Freeform send block.
func parseSendConfig(tree *Tree) PeerSendConfig {
	cfg := PeerSendConfig{}

	// Check for "all" shorthand
	if _, ok := tree.Get("all"); ok {
		cfg.Update = true
		cfg.Refresh = true
		return cfg
	}

	// Individual flags
	_, cfg.Update = tree.Get("update")
	_, cfg.Refresh = tree.Get("refresh")

	return cfg
}

// mergeProcessBindings merges new bindings into existing bindings.
// Bindings with the same plugin name are replaced (new overrides existing).
// Bindings with different plugin names are appended.
func mergeProcessBindings(existing, new []PeerProcessBinding) []PeerProcessBinding {
	if len(new) == 0 {
		return existing
	}
	if len(existing) == 0 {
		return new
	}

	// Build map of existing bindings by plugin name
	result := make([]PeerProcessBinding, 0, len(existing)+len(new))
	seen := make(map[string]int) // plugin name -> index in result

	// Add existing bindings
	for _, b := range existing {
		seen[b.PluginName] = len(result)
		result = append(result, b)
	}

	// Merge new bindings (replace or append)
	for _, b := range new {
		if idx, exists := seen[b.PluginName]; exists {
			// Replace existing binding
			result[idx] = b
		} else {
			// Append new binding
			seen[b.PluginName] = len(result)
			result = append(result, b)
		}
	}

	return result
}

// parseNexthopFamilies parses the nexthop { ... } block for RFC 8950 extended next-hop.
// Format: nexthop { ipv4/unicast ipv6; ipv4/mpls-vpn ipv6; ipv6/unicast ipv4; }
// Each entry maps (NLRI AFI, NLRI SAFI) -> NextHop AFI.
// Freeform stores ALL words as a single key with value "true".
// So "ipv4/unicast ipv6;" becomes key="ipv4/unicast ipv6", value="true".
func parseNexthopFamilies(tree *Tree) []NexthopFamilyConfig {
	var families []NexthopFamilyConfig

	afiMap := map[string]uint16{
		"ipv4": 1,
		"ipv6": 2,
	}
	safiMap := map[string]uint8{
		"unicast":    1,
		"multicast":  2,
		"mpls-vpn":   128,
		"mpls-label": 4,
	}

	// Iterate over all possible combinations: "<afi>/<safi> <nexthop-afi>"
	// Freeform stores the entire line as the key with value "true"
	for _, nlriAFIName := range []string{"ipv4", "ipv6"} {
		nlriAFI := afiMap[nlriAFIName]
		for _, safiName := range []string{"unicast", "multicast", "mpls-vpn", "mpls-label"} {
			nlriSAFI := safiMap[safiName]
			for _, nhAFIName := range []string{"ipv4", "ipv6"} {
				nhAFI := afiMap[nhAFIName]
				key := nlriAFIName + "/" + safiName + " " + nhAFIName
				if _, ok := tree.Get(key); ok {
					families = append(families, NexthopFamilyConfig{
						NLRIAFI:    nlriAFI,
						NLRISAFI:   nlriSAFI,
						NextHopAFI: nhAFI,
					})
				}
			}
		}
	}

	return families
}
