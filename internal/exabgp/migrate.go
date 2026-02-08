package exabgp

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/config"
)

// ErrNilTree is returned when a nil tree is passed.
var ErrNilTree = errors.New("nil tree")

// familyIPv6Unicast is used for IPv6 routes when family detection is needed.
const familyIPv6Unicast = "ipv6/unicast"

// configTrue represents the string value "true" used in config trees.
const configTrue = "true"

// MigrateResult holds the outcome of ExaBGP→ZeBGP migration.
type MigrateResult struct {
	Tree        *config.Tree // Transformed tree
	RIBInjected bool         // True if RIB plugin was auto-injected
	Warnings    []string     // Non-fatal issues found
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
	ribPlugin.Set("run", `"ze bgp plugin rib"`)
	tree.AddListEntry("plugin", "rib", ribPlugin)
}

// migrateProcesses converts ExaBGP processes to ZeBGP plugins.
// Returns a map of old process name → new plugin name.
func migrateProcesses(tree *config.Tree, result *MigrateResult) map[string]string {
	processMap := make(map[string]string)

	// Use ordered iteration for deterministic output.
	for _, entry := range tree.GetListOrdered("process") {
		name := entry.Key
		processTree := entry.Value
		newName := name + "-compat"
		processMap[name] = newName

		plugin := config.NewTree()

		// Wrap run command with bridge.
		if runCmd, ok := processTree.Get("run"); ok {
			// Strip quotes if present.
			runCmd = strings.Trim(runCmd, `"'`)
			wrappedCmd := fmt.Sprintf(`"ze exabgp plugin %s"`, runCmd)
			plugin.Set("run", wrappedCmd)
		}

		// Copy encoder.
		if enc, ok := processTree.Get("encoder"); ok {
			plugin.Set("encoder", enc)
		}

		result.Tree.AddListEntry("plugin", newName, plugin)
	}

	return processMap
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
		"description", "router-id", "local-address", "local-as", "peer-as",
		"hold-time", "passive", "listen", "connect", "ttl-security",
		"md5-password", "md5-base64", "group-updates", "auto-flush",
	}
	for _, key := range leafFields {
		if v, ok := neighbor.Get(key); ok {
			merged.Set(key, v)
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
		"description", "router-id", "local-address", "local-as", "peer-as",
		"hold-time", "passive", "listen", "connect", "ttl-security",
		"md5-password", "md5-base64", "group-updates", "auto-flush",
	}

	for _, field := range fields {
		if v, ok := src.Get(field); ok {
			dst.Set(field, v)
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
		enableFields := []string{"route-refresh", "asn4", "multi-session", "operational", "aigp", "extended-message", "link-local-nexthop"}
		for _, field := range enableFields {
			if _, ok := srcCap.GetFlex(field); ok {
				dstCap.Set(field, "enable")
				hasCapabilities = true
			}
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

// convertRouteToUpdate converts a single ExaBGP route to a Ze update block.
// prefix is the NLRI prefix (e.g., "10.0.0.0/24").
// attrTree contains the route attributes from ExaBGP.
// dst is the peer block where the update will be added.
func convertRouteToUpdate(prefix string, attrTree, dst *config.Tree) {
	update := config.NewTree()

	attrBlock := config.NewTree()
	attrBlock.Set("origin", "igp")

	attrFields := []string{"next-hop", "local-preference", "med", "as-path", "community",
		"extended-community", "large-community", "aggregator", "originator-id", "cluster-list",
		"path-information", "rd", "label", "labels"}
	for _, field := range attrFields {
		if v, ok := attrTree.Get(field); ok {
			attrBlock.Set(field, v)
		}
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

	nlriBlock := config.NewTree()
	// Determine family from prefix format.
	family := defaultFamily
	if strings.Contains(prefix, ":") {
		family = familyIPv6Unicast
	}
	nlriBlock.Set(family, prefix)
	update.SetContainer("nlri", nlriBlock)

	dst.AddListEntry("update", "", update)
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
				newName = name + "-compat"
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
					newName = name + "-compat"
				}
				addProcessBinding(dst, newName)
			}
		} else if key != config.KeyDefault {
			// New-style named binding - copy with name mapping.
			newName, ok := processMap[key]
			if !ok {
				newName = key + "-compat"
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
func checkUnsupported(src *config.Tree, result *MigrateResult) {
	// L2VPN/VPLS may need manual attention
	if src.GetContainer("l2vpn") != nil {
		result.Warnings = append(result.Warnings, "l2vpn block found - may need manual review")
	}

	// Flow rules may need manual attention
	if src.GetContainer("flow") != nil {
		result.Warnings = append(result.Warnings, "flow block found - may need manual review")
	}
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
		if v == "" {
			_, _ = fmt.Fprintf(buf, "%s%s;\n", indent, key)
		} else {
			// Quote values containing spaces unless already quoted
			if strings.Contains(v, " ") && !strings.HasPrefix(v, `"`) && !strings.HasPrefix(v, `[`) {
				v = `"` + v + `"`
			}
			_, _ = fmt.Fprintf(buf, "%s%s %s;\n", indent, key, v)
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

	// Write update blocks (converted from announce/static).
	for _, entry := range tree.GetListOrdered("update") {
		_, _ = fmt.Fprintf(buf, "%supdate {\n", indent)
		serializeTreeIndent(entry.Value, buf, indent+"\t", false)
		_, _ = fmt.Fprintf(buf, "%s}\n", indent)
	}

	// Templates are expanded via inherit, not serialized.
}
