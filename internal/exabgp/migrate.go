package exabgp

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/config"
)

// ErrNilTree is returned when a nil tree is passed.
var ErrNilTree = errors.New("nil tree")

// familyIPv4Unicast is the default family for IPv4 routes.
const familyIPv4Unicast = "ipv4/unicast"

// familyIPv6Unicast is used for IPv6 routes when family detection is needed.
const familyIPv6Unicast = "ipv6/unicast"

// configTrue represents the string value "true" used in config trees.
const configTrue = "true"

// ExternalProcess describes an ExaBGP process that must be handled
// by the exabgp wrapper (not convertible to a Ze plugin because
// ExaBGP uses stdout text API, Ze uses YANG RPC over socket pairs).
type ExternalProcess struct {
	Name   string // Original process name (e.g., "service-watchdog")
	RunCmd string // Run command (e.g., "./run/watchdog.run")
}

// MigrateResult holds the outcome of ExaBGP→ZeBGP migration.
type MigrateResult struct {
	Tree        *config.Tree      // Transformed tree
	RIBInjected bool              // True if RIB plugin was auto-injected
	Warnings    []string          // Non-fatal issues found
	Processes   []ExternalProcess // ExaBGP processes (handled by wrapper, not Ze)
}

// MigrateFromExaBGP converts an ExaBGP config tree to ZeBGP format.
//
// Transformations applied:
//   - neighbor → peer
//   - process → plugin (wrapped with ze exabgp plugin bridge)
//   - process { processes [...] } → process NAME { ... } inside peer
//   - capability { route-refresh; } → capability { route-refresh enable; }
//   - template { neighbor X { } } + inherit X → expanded peer
//   - If GR or route-refresh: inject RIB plugin
func MigrateFromExaBGP(tree *config.Tree) (*MigrateResult, error) {
	if tree == nil {
		return nil, ErrNilTree
	}

	result := &MigrateResult{
		Tree: config.NewTree(),
	}

	// Collect templates for inheritance expansion.
	templates := collectTemplates(tree)

	// Check if we need to inject RIB plugin
	needsRIB := NeedsRIBPlugin(tree)
	if needsRIB {
		result.RIBInjected = true
		injectRIBPlugin(result.Tree)
	}

	// Migrate processes → plugins (wrapped with bridge)
	processMap := migrateProcesses(tree, result)

	// Migrate neighbors → peers (with template expansion)
	migrateNeighbors(tree, result, processMap, needsRIB, templates)

	// Copy other top-level items (excluding templates - they're expanded)
	copyOtherItems(tree, result)

	return result, nil
}

// collectTemplates extracts template definitions for inheritance expansion.
// Returns map of template name → neighbor tree.
func collectTemplates(tree *config.Tree) map[string]*config.Tree {
	templates := make(map[string]*config.Tree)

	tmpl := tree.GetContainer("template")
	if tmpl == nil {
		return templates
	}

	// Templates contain neighbor definitions.
	for _, entry := range tmpl.GetListOrdered("neighbor") {
		templates[entry.Key] = entry.Value
	}

	return templates
}

// NeedsRIBPlugin checks if the config requires a RIB plugin.
// ZeBGP delegates RIB to plugins, so features requiring state storage need one.
func NeedsRIBPlugin(tree *config.Tree) bool {
	if tree == nil {
		return false
	}

	// Check neighbors for GR, route-refresh, or receive { update }.
	for _, neighborTree := range tree.GetList("neighbor") {
		// Check capabilities.
		if cap := neighborTree.GetContainer("capability"); cap != nil {
			// graceful-restart requires RIB for state storage.
			// Can be: graceful-restart; or graceful-restart 120; or graceful-restart { ... }.
			if cap.GetContainer("graceful-restart") != nil {
				return true
			}
			if _, ok := cap.GetFlex("graceful-restart"); ok {
				return true
			}
			// route-refresh requires RIB for refresh response.
			if _, ok := cap.GetFlex("route-refresh"); ok {
				return true
			}
		}

		// Check ExaBGP api block with receive { update; }.
		if api := neighborTree.GetContainer("api"); api != nil {
			if recv := api.GetContainer("receive"); recv != nil {
				if _, ok := recv.GetFlex("update"); ok {
					return true
				}
			}
		}

		// Check ZeBGP-style process bindings with receive { update; }.
		for _, procTree := range neighborTree.GetList("process") {
			if recv := procTree.GetContainer("receive"); recv != nil {
				if _, ok := recv.GetFlex("update"); ok {
					return true
				}
			}
		}
	}

	return false
}

// injectRIBPlugin adds the RIB plugin to the tree.
func injectRIBPlugin(tree *config.Tree) {
	ribPlugin := config.NewTree()
	ribPlugin.Set("run", `"ze plugin bgp-rib"`)
	tree.AddListEntry("plugin", "rib", ribPlugin)
}

// migrateProcesses collects ExaBGP process definitions for the wrapper to handle.
// ExaBGP processes cannot run as Ze plugins because the protocols are incompatible
// (ExaBGP uses stdout text API, Ze uses YANG RPC over socket pairs).
// Returns an empty map — no process bindings are created since there are no plugins.
func migrateProcesses(tree *config.Tree, result *MigrateResult) map[string]string {
	for _, entry := range tree.GetListOrdered("process") {
		processTree := entry.Value
		if runCmd, ok := processTree.Get("run"); ok {
			runCmd = strings.Trim(runCmd, `"'`)
			result.Processes = append(result.Processes, ExternalProcess{
				Name:   entry.Key,
				RunCmd: runCmd,
			})
		}
	}

	// Return empty map: no plugins created, so no process bindings should reference them.
	// Ze validates that process bindings reference defined plugins — undefined refs are fatal.
	return make(map[string]string)
}

// migrateNeighbors converts ExaBGP neighbors to ZeBGP peers.
func migrateNeighbors(tree *config.Tree, result *MigrateResult, processMap map[string]string, needsRIB bool, templates map[string]*config.Tree) {
	// Use ordered iteration for deterministic output.
	for _, entry := range tree.GetListOrdered("neighbor") {
		addr := entry.Key
		neighborTree := entry.Value

		// Check for template inheritance and expand if found.
		expandedTree := expandInheritance(neighborTree, templates)

		// Convert neighbor to peer.
		peer := migrateSingleNeighbor(expandedTree, result)

		// If RIB was injected, bind it to this peer.
		if needsRIB {
			bindRIBProcess(peer, expandedTree)
		}

		// Migrate process bindings (old: process { processes [...] } → new: process NAME { ... }).
		migrateProcessBindings(expandedTree, peer, processMap)

		result.Tree.AddListEntry("peer", addr, peer)
	}
}

// expandInheritance merges template properties into neighbor if inherit is specified.
// Template properties are applied first, then neighbor properties override.
func expandInheritance(neighbor *config.Tree, templates map[string]*config.Tree) *config.Tree {
	// Check for inherit field.
	inheritName, hasInherit := neighbor.Get("inherit")
	if !hasInherit {
		return neighbor
	}

	// Look up template.
	tmpl, found := templates[inheritName]
	if !found {
		// Template not found - return original (warning could be added).
		return neighbor
	}

	// Create merged tree: template first, then neighbor overrides.
	merged := tmpl.Clone()

	// Merge simple values (neighbor overrides template).
	// These are the known leaf fields in ExaBGP neighbor config.
	leafFields := []string{
		"description", "router-id", "local-address", "local-link-local", "local-as", "peer-as",
		"hold-time", "passive", "listen", "connect", "ttl-security",
		"md5-password", "md5-base64", "group-updates", "auto-flush",
	}
	for _, key := range leafFields {
		if v, ok := neighbor.Get(key); ok {
			// ExaBGP "local-link-local" → Ze "link-local"
			outKey := key
			if key == "local-link-local" {
				outKey = "link-local"
			}
			merged.Set(outKey, v)
		}
	}

	// Merge containers (neighbor overrides template, except static/announce which merge).
	// These are the known container fields in ExaBGP neighbor config.
	containerFields := []string{
		"capability", "family", "nexthop", "api",
	}
	for _, key := range containerFields {
		if c := neighbor.GetContainer(key); c != nil {
			merged.SetContainer(key, c.Clone())
		}
	}

	// Merge static/announce containers (template + neighbor routes).
	// Multiple static blocks become merged routes.
	mergeContainerFields := []string{"static", "announce"}
	for _, key := range mergeContainerFields {
		if c := neighbor.GetContainer(key); c != nil {
			merged.MergeContainer(key, c.Clone())
		}
	}

	// Merge list entries (append neighbor's to template's).
	// For static routes, we want template routes + neighbor routes.
	listFields := []string{"process", "static"}
	for _, key := range listFields {
		for _, entry := range neighbor.GetListOrdered(key) {
			merged.AddListEntry(key, entry.Key, entry.Value.Clone())
		}
	}

	return merged
}

// copySimpleFields copies simple leaf values from neighbor to peer.
func copySimpleFields(src, dst *config.Tree) {
	fields := []string{
		"description", "router-id", "local-address", "local-link-local", "local-as", "peer-as",
		"hold-time", "passive", "listen", "connect", "ttl-security",
		"md5-password", "md5-base64", "group-updates", "auto-flush",
	}

	for _, field := range fields {
		if v, ok := src.Get(field); ok {
			// ExaBGP "local-link-local" → Ze "link-local"
			outField := field
			if field == "local-link-local" {
				outField = "link-local"
			}
			dst.Set(outField, v)
		}
	}
}

// migrateCapability converts ExaBGP capability syntax to ZeBGP.
// ExaBGP: capability { route-refresh; graceful-restart 120; }.
// ZeBGP: capability { route-refresh enable; graceful-restart 120; }.
//
// RFC 8950: Infers nexthop capability from nexthop { } block presence.
func migrateCapability(src, dst *config.Tree) {
	srcCap := src.GetContainer("capability")
	dstCap := config.NewTree()
	hasCapabilities := false

	if srcCap != nil {
		// Fields that need "enable" suffix (Flex type in schema).
		enableFields := []string{"route-refresh", "multi-session", "operational", "aigp", "extended-message", "link-local-nexthop"}
		for _, field := range enableFields {
			if _, ok := srcCap.GetFlex(field); ok {
				dstCap.Set(field, "enable")
				hasCapabilities = true
			}
		}

		// asn4 preserves disable value (ExaBGP allows "asn4 disable;").
		if v, ok := srcCap.GetFlex("asn4"); ok {
			if v == "disable" || v == "false" {
				dstCap.Set("asn4", "disable")
			} else {
				dstCap.Set("asn4", "enable")
			}
			hasCapabilities = true
		}

		// Fields that keep their values (Flex type in schema).
		valueFields := []string{"graceful-restart", "add-path", "software-version"}
		for _, field := range valueFields {
			// Check for container form first (e.g., graceful-restart { restart-time 120; }).
			if container := srcCap.GetContainer(field); container != nil {
				// Copy the container as-is.
				dstCap.SetContainer(field, container.Clone())
				hasCapabilities = true
				continue
			}
			// Check for value form (e.g., graceful-restart 120;).
			if v, ok := srcCap.GetFlex(field); ok {
				if v == "" || v == configTrue {
					// ExaBGP allows bare "graceful-restart;" which parser stores as "true".
					// ZeBGP uses "enable" for boolean capabilities.
					dstCap.Set(field, "enable")
				} else {
					dstCap.Set(field, v)
				}
				hasCapabilities = true
			}
		}
	}

	// RFC 8950: Move nexthop block into capability.
	// ExaBGP: nexthop { ipv4 unicast ipv6; } at neighbor level
	// ZeBGP: capability { nexthop { ipv4/unicast ipv6; } }
	if nexthop := src.GetContainer("nexthop"); nexthop != nil {
		dstCap.SetContainer("nexthop", convertNexthopBlock(nexthop))
		hasCapabilities = true
	}

	// ExaBGP always includes extended-message (RFC 8654) in OPEN.
	// Ensure it's present in migrated config even if not explicitly configured.
	if _, ok := dstCap.Get("extended-message"); !ok {
		dstCap.Set("extended-message", "enable")
		hasCapabilities = true
	}

	// Convert host-name/domain-name from peer level to capability hostname block.
	// ExaBGP: host-name foo; domain-name bar; (at neighbor level)
	// ZeBGP: capability { hostname { host foo; domain bar; } }
	migrateHostnameToCapability(src, dstCap, &hasCapabilities)

	if hasCapabilities {
		dst.SetContainer("capability", dstCap)
	}
}

// migrateHostnameToCapability converts peer-level host-name/domain-name
// to capability { hostname { host ...; domain ...; } } format.
func migrateHostnameToCapability(src, dstCap *config.Tree, hasCapabilities *bool) {
	hostName, hasHost := src.Get("host-name")
	domainName, hasDomain := src.Get("domain-name")

	if !hasHost && !hasDomain {
		return
	}

	hostnameBlock := config.NewTree()
	if hasHost {
		hostnameBlock.Set("host", hostName)
	}
	if hasDomain {
		hostnameBlock.Set("domain", domainName)
	}
	dstCap.SetContainer("hostname", hostnameBlock)
	*hasCapabilities = true
}

// copyContainers copies container blocks from neighbor to peer.
func copyContainers(src, dst *config.Tree) {
	// Copy and convert family block.
	// ExaBGP: "ipv4 unicast" → ZeBGP: "ipv4/unicast".
	if family := src.GetContainer("family"); family != nil {
		dst.SetContainer("family", convertFamilyBlock(family))
	}

	// Convert announce block to update blocks.
	if announce := src.GetContainer("announce"); announce != nil {
		convertAnnounceToUpdate(announce, dst)
	}

	// Convert static block to update blocks.
	if static := src.GetContainer("static"); static != nil {
		convertStaticToUpdate(static, dst)
	}

	// Convert flow block to update blocks.
	if flow := src.GetContainer("flow"); flow != nil {
		convertFlowToUpdate(flow, dst)
	}

	// Convert neighbor-level l2vpn block to update blocks.
	// ExaBGP has l2vpn { vpls ... } at the neighbor level for VPLS routes.
	if l2vpn := src.GetContainer("l2vpn"); l2vpn != nil {
		convertL2VPNToUpdate(l2vpn, dst)
	}

	// RFC 8950: nexthop block is now moved into capability block by migrateCapability.
}

// convertAnnounceToUpdate converts ExaBGP announce blocks to Ze update blocks.
// ExaBGP: announce { ipv4 { unicast PREFIX next-hop NH; } }.
// Ze: update { attribute { next-hop NH; } nlri { ipv4/unicast PREFIX; } }.
func convertAnnounceToUpdate(announce, dst *config.Tree) {
	// Process each AFI (ipv4, ipv6, l2vpn)
	for _, afi := range []string{"ipv4", "ipv6", "l2vpn"} {
		afiBlock := announce.GetContainer(afi)
		if afiBlock == nil {
			continue
		}

		// Process each SAFI (unicast, multicast, nlri-mpls, mpls-vpn, flow, vpls, evpn)
		safis := []string{"unicast", "multicast", "nlri-mpls", "mpls-vpn", "flow", "vpls", "evpn"}
		for _, safi := range safis {
			// With ze:syntax "inline-list", routes are stored as list entries, not container values.
			// Each list entry has: Key=prefix, Value=Tree containing attributes.
			routeList := afiBlock.GetListOrdered(safi)
			if len(routeList) == 0 {
				continue
			}

			// Convert family name
			family := afi + "/" + safi

			// Process each route entry
			for _, routeEntry := range routeList {
				prefix := routeEntry.Key
				attrTree := routeEntry.Value

				update := config.NewTree()

				// Build attribute block from route's attribute tree
				attrBlock := config.NewTree()

				// Default origin to igp if not specified
				if origin, ok := attrTree.Get("origin"); ok {
					attrBlock.Set("origin", origin)
				} else {
					attrBlock.Set("origin", "igp")
				}

				// Copy common attributes from the route's tree
				attrFields := []string{"next-hop", "local-preference", "med", "as-path", "community",
					"extended-community", "large-community", "aggregator", "originator-id", "cluster-list",
					"rd", "label", "labels", "path-information"}
				for _, field := range attrFields {
					if v, ok := attrTree.Get(field); ok {
						attrBlock.Set(field, v)
					}
				}

				update.SetContainer("attribute", attrBlock)

				// Build nlri block
				nlriBlock := config.NewTree()
				nlriBlock.Set(family, prefix)
				update.SetContainer("nlri", nlriBlock)

				// Add update to dst as list entry
				dst.AddListEntry("update", "", update)
			}
		}

		// Handle flex SAFIs (mcast-vpn, mup, vpls) which store values via AppendValue.
		// These use ze:syntax "flex" in the ExaBGP YANG schema, so GetListOrdered returns nothing.
		flexSafis := []string{"mcast-vpn", "mup", "vpls"}
		for _, safi := range flexSafis {
			values := afiBlock.GetMultiValues(safi)
			if len(values) == 0 {
				continue
			}
			convertFlexToUpdate(afi, safi, values, dst)
		}
	}

	// Handle announce { route PREFIX ... } (generic route syntax)
	// This uses inline-list as well, so use GetListOrdered.
	routeList := announce.GetListOrdered("route")
	for _, routeEntry := range routeList {
		convertRouteToUpdate(routeEntry.Key, routeEntry.Value, dst)
	}
}

// convertStaticToUpdate converts ExaBGP static blocks to Ze update blocks.
// ExaBGP: static { route PREFIX next-hop NH; }.
// Ze: update { attribute { next-hop NH; } nlri { ipv4/unicast PREFIX; } }.
func convertStaticToUpdate(static, dst *config.Tree) {
	// With ze:syntax "inline-list", routes are stored as list entries.
	routeList := static.GetListOrdered("route")
	for _, routeEntry := range routeList {
		convertRouteToUpdate(routeEntry.Key, routeEntry.Value, dst)
	}
}

// convertFlowToUpdate converts ExaBGP flow blocks to Ze update blocks.
// ExaBGP: flow { route NAME { rd RD; next-hop NH; match { criteria; } then { actions; } } }
// Ze: update { attribute { extended-community ...; } nlri { ipv4/flow criterion value ...; } }.
func convertFlowToUpdate(flow, dst *config.Tree) {
	for _, entry := range flow.GetListOrdered("route") {
		route := entry.Value

		rd, _ := route.Get("rd")
		nextHop, _ := route.Get("next-hop")

		// Parse match criteria and detect IPv6.
		isIPv6 := false
		var nlriCriteria []string
		if match := route.GetContainer("match"); match != nil {
			for _, key := range match.Values() {
				val, _ := match.Get(key)
				criterion, value := parseFlowMatchEntry(key, val)
				if criterion == "" {
					continue
				}
				// Detect IPv6 by address content.
				if (criterion == "source" || criterion == "destination") && strings.Contains(value, ":") {
					isIPv6 = true
				}
				// Add AFI suffix to source/destination.
				nlriCriteria = append(nlriCriteria, flowCriterionWithValues(criterion, value, isIPv6))
			}
		}

		// Parse then block → attributes.
		var extComms []string
		var community string
		var rawAttr string
		if then := route.GetContainer("then"); then != nil {
			for _, key := range then.Values() {
				val, _ := then.Get(key)
				action, value := parseFlowMatchEntry(key, val)
				switch action {
				case "discard":
					extComms = append(extComms, "rate-limit:0")
				case "rate-limit":
					extComms = append(extComms, "rate-limit:"+value)
				case "redirect":
					// redirect <IP> = set next-hop + redirect-to-nexthop-draft
					// redirect <AS:VAL> = redirect extended community
					if net.ParseIP(value) != nil {
						nextHop = value
						extComms = append(extComms, "redirect-to-nexthop-draft")
					} else {
						extComms = append(extComms, "redirect:"+value)
					}
				case "redirect-to-nexthop":
					extComms = append(extComms, "redirect-to-nexthop-draft")
				case "copy":
					// copy <IP> = set next-hop + copy-to-nexthop
					if net.ParseIP(value) != nil {
						nextHop = value
					}
					extComms = append(extComms, "copy-to-nexthop")
				case "mark":
					extComms = append(extComms, "mark "+value)
				case "action":
					extComms = append(extComms, "action "+value)
				case "community":
					community = strings.Trim(value, "[] ")
				case "extended-community":
					// Inline extended communities: "[ origin:... origin:... ]"
					inner := strings.Trim(value, "[] ")
					extComms = append(extComms, strings.Fields(inner)...)
				case "redirect-to-nexthop-ietf":
					// ExaBGP name → Ze canonical name (RFC 7674)
					ip := net.ParseIP(value)
					if ip != nil && ip.To4() != nil {
						// IPv4: extended-community
						extComms = append(extComms, "redirect-to-nexthop "+value)
					} else if ip != nil {
						// IPv6: raw attribute (type 25=IPv6 ExtComm, flags 0xC0)
						// Format: sub-type 0x000c + IPv6 (16 bytes) + local-admin 0x0000
						ipHex := hex.EncodeToString(ip.To16())
						rawAttr = "[0x19 0xc0 0x000c" + ipHex + "0000]"
					}
				case "attribute":
					rawAttr = value
				}
			}
		}

		// Determine family.
		family := "ipv4/flow"
		if isIPv6 {
			family = "ipv6/flow"
		}
		if rd != "" {
			if isIPv6 {
				family = "ipv6/flow-vpn"
			} else {
				family = "ipv4/flow-vpn"
			}
		}

		// Build NLRI line: <family> [rd <rd>] <criteria...>
		var nlriLine strings.Builder
		nlriLine.WriteString(family)
		if rd != "" {
			nlriLine.WriteString(" rd " + rd)
		}
		for _, c := range nlriCriteria {
			nlriLine.WriteString(" " + c)
		}

		// Build update block.
		update := config.NewTree()

		attrBlock := config.NewTree()
		if nextHop != "" {
			attrBlock.Set("next-hop", nextHop)
		}
		if len(extComms) > 0 {
			attrBlock.Set("extended-community", "["+strings.Join(extComms, " ")+"]")
		}
		if community != "" {
			attrBlock.Set("community", "["+community+"]")
		}
		if rawAttr != "" {
			attrBlock.Set("attribute", rawAttr)
		}
		update.SetContainer("attribute", attrBlock)

		nlriBlock := config.NewTree()
		nlriBlock.Set(nlriLine.String(), "")
		update.SetContainer("nlri", nlriBlock)

		dst.AddListEntry("update", "", update)
	}
}

// convertL2VPNToUpdate converts neighbor-level l2vpn blocks to Ze update blocks.
// Handles both inline VPLS routes (flex multi-values) and named VPLS blocks (list entries).
func convertL2VPNToUpdate(l2vpn, dst *config.Tree) {
	// Handle inline VPLS routes (stored via AppendValue by the flex parser).
	vplsValues := l2vpn.GetMultiValues("vpls")
	if len(vplsValues) > 0 {
		convertFlexToUpdate("l2vpn", "vpls", vplsValues, dst)
	}

	// Handle named VPLS routes (stored via AddListEntry by the flex parser).
	for _, entry := range l2vpn.GetListOrdered("vpls") {
		convertNamedVPLSToUpdate(entry.Value, dst)
	}
}

// convertNamedVPLSToUpdate converts a named VPLS block (e.g., vpls site5 { ... })
// to a Ze update block. The block's Tree has parsed fields like endpoint, base, rd, etc.
func convertNamedVPLSToUpdate(vpls, dst *config.Tree) {
	update := config.NewTree()

	attrBlock := config.NewTree()

	// Extract path attributes from the VPLS block.
	// Simple values (single word) are stored as-is.
	simpleFields := []string{"next-hop", "origin", "local-preference", "med", "originator-id"}
	for _, field := range simpleFields {
		if v, ok := vpls.Get(field); ok {
			attrBlock.Set(field, v)
		}
	}
	// Array fields: value-or-array strips brackets, so re-wrap multi-word values.
	arrayFields := []string{"as-path", "community", "extended-community", "large-community", "cluster-list"}
	for _, field := range arrayFields {
		if v, ok := vpls.Get(field); ok {
			if strings.Contains(v, " ") && !strings.HasPrefix(v, "[") {
				v = "[" + v + "]"
			}
			attrBlock.Set(field, v)
		}
	}
	if _, ok := attrBlock.Get("origin"); !ok {
		attrBlock.Set("origin", "igp")
	}
	update.SetContainer("attribute", attrBlock)

	// Build NLRI line from VPLS-specific fields.
	// Format: "l2vpn/vpls rd X endpoint Y base Z offset A size B"
	nlriFields := []string{"rd", "endpoint", "ve-id", "base", "offset", "ve-block-offset", "size", "ve-block-size"}
	var nlriParts []string
	for _, field := range nlriFields {
		if v, ok := vpls.Get(field); ok {
			nlriParts = append(nlriParts, field, v)
		}
	}

	nlriBlock := config.NewTree()
	nlriBlock.Set("l2vpn/vpls "+strings.Join(nlriParts, " "), "")
	update.SetContainer("nlri", nlriBlock)

	dst.AddListEntry("update", "", update)
}

// parseFlowMatchEntry splits a freeform entry into criterion and value.
// Freeform stores "keyword value" as key→"true", or "keyword" as key→"[ values ]".
func parseFlowMatchEntry(key, val string) (string, string) {
	if val == "true" || val == "" {
		parts := strings.SplitN(key, " ", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
		return key, ""
	}
	return key, val
}

// flowCriterionWithValues formats a FlowSpec criterion for the NLRI line.
// Adds -ipv4/-ipv6 suffix to source/destination, normalizes operators.
func flowCriterionWithValues(criterion, value string, isIPv6 bool) string {
	switch criterion {
	case "source":
		if isIPv6 {
			return "source-ipv6 " + value
		}
		return "source-ipv4 " + value
	case "destination":
		if isIPv6 {
			return "destination-ipv6 " + value
		}
		return "destination-ipv4 " + value
	default: // pass through other criteria unchanged
		if value == "" {
			return criterion
		}
		// Unwrap single-element bracket lists: "[ !=0 ]" → "!=0"
		if strings.HasPrefix(value, "[ ") && strings.HasSuffix(value, " ]") {
			inner := strings.Fields(value[2 : len(value)-2])
			if len(inner) == 1 {
				return criterion + " " + inner[0]
			}
		}
		return criterion + " " + value
	}
}

// convertRouteToUpdate converts a single ExaBGP route to a Ze update block.
// prefix is the NLRI prefix (e.g., "10.0.0.0/24").
// attrTree contains the route attributes from ExaBGP.
// dst is the peer block where the update will be added.
//
// Family detection: rd present → mpls-vpn, label present (no rd) → nlri-mpls, else → unicast.
// RD and label go inline in the NLRI (config loader expects them there, not in attribute block).
func convertRouteToUpdate(prefix string, attrTree, dst *config.Tree) {
	update := config.NewTree()

	attrBlock := config.NewTree()
	attrBlock.Set("origin", "igp")

	// Path attributes that go in the attribute block.
	// Note: rd and label are NOT here — they go inline in the NLRI line.
	attrFields := []string{"next-hop", "local-preference", "med", "as-path", "community",
		"extended-community", "large-community", "aggregator", "originator-id", "cluster-list",
		"path-information", "labels", "split"}
	for _, field := range attrFields {
		if v, ok := attrTree.Get(field); ok {
			attrBlock.Set(field, v)
		}
	}

	// BGP Prefix-SID (RFC 8669) — inline-list parser stores bracketed values via Set().
	if v, ok := attrTree.Get("bgp-prefix-sid"); ok {
		attrBlock.Set("bgp-prefix-sid", v)
	}

	// Handle atomic-aggregate (flag attribute stored as "true")
	if v, ok := attrTree.Get("atomic-aggregate"); ok && v == configTrue {
		attrBlock.Set("atomic-aggregate", configTrue)
	}

	// Handle raw attribute bytes: "[0x20 0xc0 0x...]" → "0x20 0xc0 0x..."
	if v, ok := attrTree.Get("attribute"); ok {
		// Strip brackets from array syntax
		v = strings.TrimPrefix(v, "[")
		v = strings.TrimSuffix(v, "]")
		attrBlock.Set("attribute", v)
	}

	update.SetContainer("attribute", attrBlock)

	// Detect family from prefix format and route attributes.
	isIPv6 := strings.Contains(prefix, ":")
	rdVal, hasRD := attrTree.Get("rd")
	labelVal, hasLabel := attrTree.Get("label")

	family := detectRouteFamily(isIPv6, hasRD, hasLabel)

	// Build NLRI value with inline rd/label (config loader parses these from NLRI line).
	nlriValue := prefix
	if hasLabel {
		nlriValue = "label " + labelVal + " " + nlriValue
	}
	if hasRD {
		nlriValue = "rd " + rdVal + " " + nlriValue
	}

	nlriBlock := config.NewTree()
	nlriBlock.Set(family, nlriValue)
	update.SetContainer("nlri", nlriBlock)

	// Handle watchdog attribute.
	// ExaBGP: "route PREFIX ... watchdog NAME withdraw;" stores watchdog=NAME, withdraw=true.
	// Ze: "update { watchdog { name NAME; } ... }"
	// Note: we omit "withdraw true" because in ExaBGP the watchdog process announces
	// before session establishment, so the route is effectively announced at session start.
	// The exabgp wrapper handles the process lifecycle via the API socket.
	if wdName, ok := attrTree.Get("watchdog"); ok {
		wdBlock := config.NewTree()
		wdBlock.Set("name", wdName)
		update.SetContainer("watchdog", wdBlock)
	}

	dst.AddListEntry("update", "", update)
}

// detectRouteFamily determines the BGP address family from route characteristics.
// rd present → mpls-vpn, label present (no rd) → nlri-mpls, else → unicast.
func detectRouteFamily(isIPv6, hasRD, hasLabel bool) string {
	if hasRD && isIPv6 {
		return "ipv6/mpls-vpn"
	}
	if hasRD {
		return "ipv4/mpls-vpn"
	}
	if hasLabel && isIPv6 {
		return "ipv6/mpls"
	}
	if hasLabel {
		return "ipv4/mpls"
	}
	if isIPv6 {
		return familyIPv6Unicast
	}
	return familyIPv4Unicast
}

// convertFamilyBlock converts ExaBGP family syntax to ZeBGP.
// ExaBGP: "ipv4 unicast;" → ZeBGP: "ipv4/unicast;".
func convertFamilyBlock(src *config.Tree) *config.Tree {
	dst := config.NewTree()

	// Get keys and sort for deterministic output.
	keys := src.Values()
	sort.Strings(keys)

	for _, key := range keys {
		// Convert "ipv4 unicast" → "ipv4/unicast".
		converted := convertFamilySyntax(key)
		// Family entries are flags (no value), stored as "true" by Freeform parser.
		// Output as empty value to get "ipv4/unicast;" not "ipv4/unicast true;".
		dst.Set(converted, "")
	}

	return dst
}

// convertFamilySyntax converts ExaBGP family format to ZeBGP.
// Examples: "ipv4 unicast" → "ipv4/unicast", "ipv6 multicast" → "ipv6/multicast".
func convertFamilySyntax(family string) string {
	// Common ExaBGP family formats.
	replacements := map[string]string{
		"ipv4 unicast":   "ipv4/unicast",
		"ipv4 multicast": "ipv4/multicast",
		"ipv4 nlri-mpls": "ipv4/nlri-mpls",
		"ipv4 flowspec":  "ipv4/flow",
		"ipv6 unicast":   "ipv6/unicast",
		"ipv6 multicast": "ipv6/multicast",
		"ipv6 nlri-mpls": "ipv6/nlri-mpls",
		"ipv6 flowspec":  "ipv6/flow",
		"l2vpn vpls":     "l2vpn/vpls",
		"l2vpn evpn":     "l2vpn/evpn",
	}

	if converted, ok := replacements[strings.ToLower(family)]; ok {
		return converted
	}

	// Fallback: replace first space with slash.
	return strings.Replace(family, " ", "/", 1)
}

// convertNexthopBlock converts ExaBGP nexthop syntax to ZeBGP.
// ExaBGP: "ipv4 unicast ipv6;" → ZeBGP: "ipv4/unicast ipv6;".
// The nexthop block maps (AFI, SAFI) → NextHop-AFI.
func convertNexthopBlock(src *config.Tree) *config.Tree {
	dst := config.NewTree()

	// Get keys and sort for deterministic output.
	keys := src.Values()
	sort.Strings(keys)

	for _, key := range keys {
		// ExaBGP stores "ipv4 unicast ipv6" as key, value "true".
		// Convert to ZeBGP format: "ipv4/unicast ipv6".
		converted := convertNexthopSyntax(key)
		dst.Set(converted, "")
	}

	return dst
}

// convertNexthopSyntax converts ExaBGP nexthop format to ZeBGP.
// ExaBGP: "ipv4 unicast ipv6" → ZeBGP: "ipv4/unicast ipv6".
// Format: "<afi> <safi> <nhafi>" → "<afi>/<safi> <nhafi>".
func convertNexthopSyntax(nexthop string) string {
	parts := strings.Fields(nexthop)
	if len(parts) != 3 {
		// Unknown format, return as-is.
		return nexthop
	}

	// parts[0] = afi (ipv4/ipv6)
	// parts[1] = safi (unicast/mpls-vpn/etc)
	// parts[2] = nexthop-afi (ipv4/ipv6)

	// Normalize SAFI names to ZeBGP conventions.
	// ZeBGP's parseNexthopFamilies expects "mpls-label" for SAFI 4.
	safi := normalizeSAFI(parts[1])

	return parts[0] + "/" + safi + " " + parts[2]
}

// normalizeSAFI converts ExaBGP SAFI names to ZeBGP conventions.
// ExaBGP uses "nlri-mpls" and "labeled-unicast" for SAFI 4.
// ZeBGP's nexthop parser expects "mpls-label".
func normalizeSAFI(safi string) string {
	switch strings.ToLower(safi) {
	case "nlri-mpls", "labeled-unicast":
		return "mpls-label"
	default:
		return safi
	}
}

// bindRIBProcess binds the RIB plugin to a peer.
func bindRIBProcess(peer, src *config.Tree) {
	ribProcess := config.NewTree()

	// Send block.
	sendBlock := config.NewTree()
	sendBlock.Set("update", "")
	sendBlock.Set("state", "")

	// Add refresh if route-refresh is enabled.
	if cap := src.GetContainer("capability"); cap != nil {
		if _, ok := cap.GetFlex("route-refresh"); ok {
			sendBlock.Set("refresh", "")
		}
	}
	ribProcess.SetContainer("send", sendBlock)

	// Receive block.
	recvBlock := config.NewTree()
	recvBlock.Set("update", "")
	ribProcess.SetContainer("receive", recvBlock)

	peer.AddListEntry("process", "rib", ribProcess)
}

// migrateProcessBindings converts ExaBGP api block and process blocks to ZeBGP named bindings.
// ExaBGP syntax: api { processes [ foo bar ]; }.
// ZeBGP syntax: process foo-compat { send { update; state; } }.
func migrateProcessBindings(src, dst *config.Tree, processMap map[string]string) {
	// First, handle ExaBGP-style api block.
	if api := src.GetContainer("api"); api != nil {
		processNames := extractProcessNames(api)
		for _, name := range processNames {
			newName, ok := processMap[name]
			if !ok {
				continue // No plugin created for this process — skip binding.
			}
			addProcessBinding(dst, newName)
		}
	}

	// Then, handle ZeBGP-style process blocks (ordered for deterministic output).
	for _, entry := range src.GetListOrdered("process") {
		key := entry.Key
		procTree := entry.Value

		// Check if this is old-style (has "processes" field) or new-style (named).
		processNames := extractProcessNames(procTree)

		if len(processNames) > 0 {
			// Old-style: convert to named bindings.
			for _, name := range processNames {
				newName, ok := processMap[name]
				if !ok {
					continue // No plugin created — skip binding.
				}
				addProcessBinding(dst, newName)
			}
		} else if key != config.KeyDefault {
			// New-style named binding - copy with name mapping.
			newName, ok := processMap[key]
			if !ok {
				continue // No plugin created — skip binding.
			}
			dst.AddListEntry("process", newName, procTree.Clone())
		}
	}
}

// extractProcessNames extracts process names from a block with "processes" field.
func extractProcessNames(tree *config.Tree) []string {
	// Try multi-value first.
	processNames := tree.GetMultiValues("processes")
	if len(processNames) > 0 {
		return processNames
	}

	// Try single value.
	if plist, ok := tree.Get("processes"); ok {
		// Parse process list: "[ name1 name2 ]" or "[ name1, name2 ]".
		plist = strings.Trim(plist, "[]")
		plist = strings.ReplaceAll(plist, ",", " ")
		return strings.Fields(plist)
	}

	return nil
}

// addProcessBinding adds a process binding with default send block.
func addProcessBinding(dst *config.Tree, name string) {
	proc := config.NewTree()

	// Send block.
	sendBlock := config.NewTree()
	sendBlock.Set("update", "")
	sendBlock.Set("state", "")
	proc.SetContainer("send", sendBlock)

	dst.AddListEntry("process", name, proc)
}

// checkUnsupported adds warnings for features that need manual migration.
func checkUnsupported(_ *config.Tree, _ *MigrateResult) {
	// L2VPN/VPLS: handled by convertL2VPNToUpdate.
	// Flow blocks: handled by convertFlowToUpdate.
}

// copyOtherItems copies non-neighbor, non-process items.
// Templates are NOT copied - they are expanded via inheritance.
func copyOtherItems(src *config.Tree, result *MigrateResult) {
	// Templates are expanded via inherit, not copied.
	// Other top-level items could be copied here if needed.
}

// migrateSingleNeighbor converts a single neighbor tree to peer format.
// Used for both top-level neighbors and template neighbors.
func migrateSingleNeighbor(neighborTree *config.Tree, result *MigrateResult) *config.Tree {
	peer := config.NewTree()

	// Copy simple fields.
	copySimpleFields(neighborTree, peer)

	// Migrate capability block.
	migrateCapability(neighborTree, peer)

	// Copy other containers (family, etc.).
	copyContainers(neighborTree, peer)

	// Check for unsupported features.
	checkUnsupported(neighborTree, result)

	return peer
}

// SerializeTree serializes a config tree to ZeBGP config format.
func SerializeTree(tree *config.Tree) string {
	if tree == nil {
		return ""
	}

	var buf strings.Builder
	serializeTreeIndent(tree, &buf, "", true)
	return buf.String()
}

func serializeTreeIndent(tree *config.Tree, buf *strings.Builder, indent string, isRoot bool) {
	// Write simple values (sorted for deterministic output).
	keys := tree.Values()
	sort.Strings(keys)
	for _, key := range keys {
		v, _ := tree.Get(key)
		switch {
		case v == "":
			buf.WriteString(indent + key + ";\n")
		case strings.Contains(v, " ") && !strings.HasPrefix(v, "[") && !strings.HasPrefix(v, "\""):
			buf.WriteString(indent + key + " \"" + v + "\";\n")
		default:
			buf.WriteString(indent + key + " " + v + ";\n")
		}
	}

	// Write plugin blocks - new syntax: plugin { external NAME { ... } }
	pluginList := tree.GetListOrdered("plugin")
	if len(pluginList) > 0 {
		_, _ = fmt.Fprintf(buf, "%splugin {\n", indent)
		for _, entry := range pluginList {
			_, _ = fmt.Fprintf(buf, "%s\texternal %s {\n", indent, entry.Key)
			serializeTreeIndent(entry.Value, buf, indent+"\t\t", false)
			_, _ = fmt.Fprintf(buf, "%s\t}\n", indent)
		}
		_, _ = fmt.Fprintf(buf, "%s}\n", indent)
	}

	// Write peer blocks - wrap in bgp {} at root level.
	peerList := tree.GetListOrdered("peer")
	if len(peerList) > 0 {
		if isRoot {
			_, _ = fmt.Fprintf(buf, "%sbgp {\n", indent)
			indent += "\t"
		}
		for _, entry := range peerList {
			_, _ = fmt.Fprintf(buf, "%speer %s {\n", indent, entry.Key)
			serializeTreeIndent(entry.Value, buf, indent+"\t", false)
			_, _ = fmt.Fprintf(buf, "%s}\n", indent)
		}
		if isRoot {
			indent = indent[:len(indent)-1]
			_, _ = fmt.Fprintf(buf, "%s}\n", indent)
		}
	}

	// Write nested containers.
	if cap := tree.GetContainer("capability"); cap != nil {
		_, _ = fmt.Fprintf(buf, "%scapability {\n", indent)
		serializeTreeIndent(cap, buf, indent+"\t", false)
		_, _ = fmt.Fprintf(buf, "%s}\n", indent)
	}

	if family := tree.GetContainer("family"); family != nil {
		_, _ = fmt.Fprintf(buf, "%sfamily {\n", indent)
		serializeTreeIndent(family, buf, indent+"\t", false)
		_, _ = fmt.Fprintf(buf, "%s}\n", indent)
	}

	// Write hostname block (FQDN capability).
	if hostname := tree.GetContainer("hostname"); hostname != nil {
		buf.WriteString(indent)
		buf.WriteString("hostname {\n")
		serializeTreeIndent(hostname, buf, indent+"\t", false)
		buf.WriteString(indent)
		buf.WriteString("}\n")
	}

	// Write nexthop block (RFC 8950).
	if nexthop := tree.GetContainer("nexthop"); nexthop != nil {
		_, _ = fmt.Fprintf(buf, "%snexthop {\n", indent)
		serializeTreeIndent(nexthop, buf, indent+"\t", false)
		_, _ = fmt.Fprintf(buf, "%s}\n", indent)
	}

	// Write attribute block (used in update blocks).
	if attr := tree.GetContainer("attribute"); attr != nil {
		_, _ = fmt.Fprintf(buf, "%sattribute {\n", indent)
		serializeTreeIndent(attr, buf, indent+"\t", false)
		_, _ = fmt.Fprintf(buf, "%s}\n", indent)
	}

	// Write nlri block (used in update blocks).
	if nlri := tree.GetContainer("nlri"); nlri != nil {
		_, _ = fmt.Fprintf(buf, "%snlri {\n", indent)
		serializeTreeIndent(nlri, buf, indent+"\t", false)
		_, _ = fmt.Fprintf(buf, "%s}\n", indent)
	}

	// Watchdog block (routes controlled via "bgp watchdog announce/withdraw").
	if wdog := tree.GetContainer("watchdog"); wdog != nil {
		buf.WriteString(indent + "watchdog {\n")
		serializeTreeIndent(wdog, buf, indent+"\t", false)
		buf.WriteString(indent + "}\n")
	}

	// Write process bindings.
	for _, entry := range tree.GetListOrdered("process") {
		_, _ = fmt.Fprintf(buf, "%sprocess %s {\n", indent, entry.Key)
		serializeTreeIndent(entry.Value, buf, indent+"\t", false)
		_, _ = fmt.Fprintf(buf, "%s}\n", indent)
	}

	// Write send/receive blocks.
	if send := tree.GetContainer("send"); send != nil {
		_, _ = fmt.Fprintf(buf, "%ssend {\n", indent)
		serializeTreeIndent(send, buf, indent+"\t", false)
		_, _ = fmt.Fprintf(buf, "%s}\n", indent)
	}
	if recv := tree.GetContainer("receive"); recv != nil {
		_, _ = fmt.Fprintf(buf, "%sreceive {\n", indent)
		serializeTreeIndent(recv, buf, indent+"\t", false)
		_, _ = fmt.Fprintf(buf, "%s}\n", indent)
	}

	// Write update blocks (converted from announce/static/flow).
	for _, entry := range tree.GetListOrdered("update") {
		_, _ = fmt.Fprintf(buf, "%supdate {\n", indent)
		serializeTreeIndent(entry.Value, buf, indent+"\t", false)
		_, _ = fmt.Fprintf(buf, "%s}\n", indent)
	}

	// Templates are expanded via inherit, not serialized.
}

// flexAttrKeywords lists path attribute keywords that go in the attribute block
// when converting flex container entries (mcast-vpn, mup) to update blocks.
// Everything else is treated as NLRI fields.
var flexAttrKeywords = map[string]bool{
	"next-hop":            true,
	"origin":              true,
	"local-preference":    true,
	"med":                 true,
	"extended-community":  true,
	"large-community":     true,
	"community":           true,
	"originator-id":       true,
	"cluster-list":        true,
	"as-path":             true,
	"bgp-prefix-sid-srv6": true,
}

// convertFlexToUpdate converts flex container entries to update blocks.
// Flex SAFIs (mcast-vpn, mup) store entire route lines as strings via AppendValue().
// Each string contains both NLRI fields and path attributes intermixed.
// This function separates them into attribute { } and nlri { } blocks.
func convertFlexToUpdate(afi, safi string, values []string, dst *config.Tree) {
	family := afi + "/" + safi

	for _, value := range values {
		attrs, nlriParts := splitFlexAttrs(value)

		update := config.NewTree()

		// Build attribute block
		attrBlock := config.NewTree()
		if _, ok := attrs["origin"]; !ok {
			attrBlock.Set("origin", "igp")
		}
		for k, v := range attrs {
			attrBlock.Set(k, v)
		}
		update.SetContainer("attribute", attrBlock)

		// Build nlri block
		nlriBlock := config.NewTree()
		nlriBlock.Set(family+" "+strings.Join(nlriParts, " "), "")
		update.SetContainer("nlri", nlriBlock)

		dst.AddListEntry("update", "", update)
	}
}

// splitFlexAttrs separates a flex value string into path attributes and NLRI fields.
// Returns a map of attribute key→value and a slice of NLRI tokens.
// Attribute values with brackets [..] or parens (..) have the delimiters stripped.
func splitFlexAttrs(value string) (map[string]string, []string) {
	tokens := tokenizeFlexValue(value)
	attrs := make(map[string]string)
	var nlriParts []string

	i := 0
	for i < len(tokens) {
		token := tokens[i]
		// Skip backslash continuations — artifacts from multiline ExaBGP config parsing
		if token == `\` {
			i++
			continue
		}
		if flexAttrKeywords[token] {
			// Path attribute — next token is its value
			i++
			if i < len(tokens) {
				attrValue := tokens[i]
				// Bracket values [val1 val2]: keep brackets for multi-value (Ze value-or-array syntax),
				// strip for single-value (no spaces inside brackets).
				if strings.HasPrefix(attrValue, "[") && strings.HasSuffix(attrValue, "]") {
					inner := attrValue[1 : len(attrValue)-1]
					if !strings.Contains(inner, " ") {
						// Single value inside brackets — strip them
						attrValue = inner
					}
					// Multi-value: keep brackets as-is for Ze "[ val1 val2 ]" syntax
				}
				// Paren values (val1 val2): ExaBGP grouping — strip parens, content is the value.
				// Flex syntax handles "key value key value" natively without outer brackets.
				if strings.HasPrefix(attrValue, "(") && strings.HasSuffix(attrValue, ")") {
					attrValue = attrValue[1 : len(attrValue)-1]
				}
				attrs[token] = strings.TrimSpace(attrValue)
			}
			i++
		} else {
			nlriParts = append(nlriParts, token)
			i++
		}
	}
	return attrs, nlriParts
}

// tokenizeFlexValue splits a flex value string into tokens, grouping bracketed [...]
// and parenthesized (...) sequences as single atomic tokens.
// The flex parser (parseFlex value mode) joins brackets/parens with their content,
// so "[target:10:10 mup:10:10]" may split into ["[target:10:10", "mup:10:10]"]
// after strings.Fields. This function regroups them.
func tokenizeFlexValue(s string) []string {
	fields := strings.Fields(s)
	var tokens []string

	i := 0
	for i < len(fields) {
		f := fields[i]

		switch {
		case strings.HasPrefix(f, "["):
			// Collect until matching ]
			var parts []string
			for i < len(fields) {
				parts = append(parts, fields[i])
				if strings.HasSuffix(fields[i], "]") {
					i++
					break
				}
				i++
			}
			tokens = append(tokens, strings.Join(parts, " "))
		case strings.HasPrefix(f, "("):
			// Collect until paren depth returns to 0
			var parts []string
			depth := 0
			for i < len(fields) {
				part := fields[i]
				parts = append(parts, part)
				depth += strings.Count(part, "(") - strings.Count(part, ")")
				i++
				if depth <= 0 {
					break
				}
			}
			tokens = append(tokens, strings.Join(parts, " "))
		default: // plain token — no grouping
			tokens = append(tokens, f)
			i++
		}
	}
	return tokens
}
