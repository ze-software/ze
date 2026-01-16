package exabgp

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/zebgp/pkg/config"
)

// ErrNilTree is returned when a nil tree is passed.
var ErrNilTree = errors.New("nil tree")

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
//   - process → plugin (wrapped with zebgp exabgp plugin bridge)
//   - process { processes [...] } → process NAME { ... } inside peer
//   - capability { route-refresh; } → capability { route-refresh enable; }
//   - If GR or route-refresh: inject RIB plugin
func MigrateFromExaBGP(tree *config.Tree) (*MigrateResult, error) {
	if tree == nil {
		return nil, ErrNilTree
	}

	result := &MigrateResult{
		Tree: config.NewTree(),
	}

	// Check if we need to inject RIB plugin
	needsRIB := NeedsRIBPlugin(tree)
	if needsRIB {
		result.RIBInjected = true
		injectRIBPlugin(result.Tree)
	}

	// Migrate processes → plugins (wrapped with bridge)
	processMap := migrateProcesses(tree, result)

	// Migrate neighbors → peers
	migrateNeighbors(tree, result, processMap, needsRIB)

	// Copy other top-level items
	copyOtherItems(tree, result)

	return result, nil
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
	ribPlugin.Set("run", `"zebgp plugin rib"`)
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
			wrappedCmd := fmt.Sprintf(`"zebgp exabgp plugin %s"`, runCmd)
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
func migrateNeighbors(tree *config.Tree, result *MigrateResult, processMap map[string]string, needsRIB bool) {
	// Use ordered iteration for deterministic output.
	for _, entry := range tree.GetListOrdered("neighbor") {
		addr := entry.Key
		neighborTree := entry.Value

		// Convert neighbor to peer.
		peer := migrateSingleNeighbor(neighborTree, result)

		// If RIB was injected, bind it to this peer.
		if needsRIB {
			bindRIBProcess(peer, neighborTree)
		}

		// Migrate process bindings (old: process { processes [...] } → new: process NAME { ... }).
		migrateProcessBindings(neighborTree, peer, processMap)

		result.Tree.AddListEntry("peer", addr, peer)
	}
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
func migrateCapability(src, dst *config.Tree) {
	srcCap := src.GetContainer("capability")
	if srcCap == nil {
		return
	}

	dstCap := config.NewTree()

	// Fields that need "enable" suffix (Flex type in schema).
	enableFields := []string{"route-refresh", "asn4", "multi-session", "operational", "aigp", "extended-message", "nexthop"}
	for _, field := range enableFields {
		if _, ok := srcCap.GetFlex(field); ok {
			dstCap.Set(field, "enable")
		}
	}

	// Fields that keep their values (Flex type in schema).
	valueFields := []string{"graceful-restart", "add-path", "software-version"}
	for _, field := range valueFields {
		// Check for container form first (e.g., graceful-restart { restart-time 120; }).
		if container := srcCap.GetContainer(field); container != nil {
			// Copy the container as-is.
			dstCap.SetContainer(field, container.Clone())
			continue
		}
		// Check for value form (e.g., graceful-restart 120;).
		if v, ok := srcCap.GetFlex(field); ok {
			if v == "" || v == "true" {
				// ExaBGP allows bare "graceful-restart;" which parser stores as "true".
				// ZeBGP uses "enable" for boolean capabilities.
				dstCap.Set(field, "enable")
			} else {
				dstCap.Set(field, v)
			}
		}
	}

	dst.SetContainer("capability", dstCap)
}

// copyContainers copies container blocks from neighbor to peer.
func copyContainers(src, dst *config.Tree) {
	// Copy and convert family block.
	// ExaBGP: "ipv4 unicast" → ZeBGP: "ipv4/unicast".
	if family := src.GetContainer("family"); family != nil {
		dst.SetContainer("family", convertFamilyBlock(family))
	}

	// Copy announce block (Freeform - strip "true" placeholder values).
	if announce := src.GetContainer("announce"); announce != nil {
		dst.SetContainer("announce", convertFreeformBlock(announce))
	}

	// Copy static block (Freeform - strip "true" placeholder values).
	if static := src.GetContainer("static"); static != nil {
		dst.SetContainer("static", convertFreeformBlock(static))
	}
}

// convertFreeformBlock converts a Freeform block, stripping "true" placeholder values.
// Freeform parser stores entries as key → "true", but output should be just "key;".
func convertFreeformBlock(src *config.Tree) *config.Tree {
	dst := config.NewTree()

	// Get keys and sort for deterministic output.
	keys := src.Values()
	sort.Strings(keys)

	for _, key := range keys {
		// Freeform entries have "true" as placeholder - output as empty.
		dst.Set(key, "")
	}

	return dst
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
		"ipv4 flowspec":  "ipv4/flowspec",
		"ipv6 unicast":   "ipv6/unicast",
		"ipv6 multicast": "ipv6/multicast",
		"ipv6 nlri-mpls": "ipv6/nlri-mpls",
		"ipv6 flowspec":  "ipv6/flowspec",
		"l2vpn vpls":     "l2vpn/vpls",
		"l2vpn evpn":     "l2vpn/evpn",
	}

	if converted, ok := replacements[strings.ToLower(family)]; ok {
		return converted
	}

	// Fallback: replace first space with slash.
	return strings.Replace(family, " ", "/", 1)
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
func copyOtherItems(src *config.Tree, result *MigrateResult) {
	// Migrate template block if present.
	// Templates contain neighbor definitions that need the same conversions.
	if tmpl := src.GetContainer("template"); tmpl != nil {
		result.Tree.SetContainer("template", migrateTemplate(tmpl, result))
	}
}

// migrateTemplate converts template block, migrating nested neighbors.
func migrateTemplate(src *config.Tree, result *MigrateResult) *config.Tree {
	dst := config.NewTree()

	// Migrate neighbor templates → peer templates.
	for _, entry := range src.GetListOrdered("neighbor") {
		name := entry.Key
		neighborTree := entry.Value

		peer := migrateSingleNeighbor(neighborTree, result)
		dst.AddListEntry("peer", name, peer)
	}

	return dst
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
	serializeTreeIndent(tree, &buf, "")
	return buf.String()
}

func serializeTreeIndent(tree *config.Tree, buf *strings.Builder, indent string) {
	// Write simple values (sorted for deterministic output).
	keys := tree.Values()
	sort.Strings(keys)
	for _, key := range keys {
		v, _ := tree.Get(key)
		if v == "" {
			_, _ = fmt.Fprintf(buf, "%s%s;\n", indent, key)
		} else {
			_, _ = fmt.Fprintf(buf, "%s%s %s;\n", indent, key, v)
		}
	}

	// Write plugin blocks.
	for _, entry := range tree.GetListOrdered("plugin") {
		_, _ = fmt.Fprintf(buf, "%splugin %s {\n", indent, entry.Key)
		serializeTreeIndent(entry.Value, buf, indent+"\t")
		_, _ = fmt.Fprintf(buf, "%s}\n", indent)
	}

	// Write peer blocks.
	for _, entry := range tree.GetListOrdered("peer") {
		_, _ = fmt.Fprintf(buf, "%speer %s {\n", indent, entry.Key)
		serializeTreeIndent(entry.Value, buf, indent+"\t")
		_, _ = fmt.Fprintf(buf, "%s}\n", indent)
	}

	// Write nested containers.
	if cap := tree.GetContainer("capability"); cap != nil {
		_, _ = fmt.Fprintf(buf, "%scapability {\n", indent)
		serializeTreeIndent(cap, buf, indent+"\t")
		_, _ = fmt.Fprintf(buf, "%s}\n", indent)
	}

	if family := tree.GetContainer("family"); family != nil {
		_, _ = fmt.Fprintf(buf, "%sfamily {\n", indent)
		serializeTreeIndent(family, buf, indent+"\t")
		_, _ = fmt.Fprintf(buf, "%s}\n", indent)
	}

	// Write process bindings.
	for _, entry := range tree.GetListOrdered("process") {
		_, _ = fmt.Fprintf(buf, "%sprocess %s {\n", indent, entry.Key)
		serializeTreeIndent(entry.Value, buf, indent+"\t")
		_, _ = fmt.Fprintf(buf, "%s}\n", indent)
	}

	// Write send/receive blocks.
	if send := tree.GetContainer("send"); send != nil {
		_, _ = fmt.Fprintf(buf, "%ssend {\n", indent)
		serializeTreeIndent(send, buf, indent+"\t")
		_, _ = fmt.Fprintf(buf, "%s}\n", indent)
	}
	if recv := tree.GetContainer("receive"); recv != nil {
		_, _ = fmt.Fprintf(buf, "%sreceive {\n", indent)
		serializeTreeIndent(recv, buf, indent+"\t")
		_, _ = fmt.Fprintf(buf, "%s}\n", indent)
	}

	// Write static block.
	if static := tree.GetContainer("static"); static != nil {
		_, _ = fmt.Fprintf(buf, "%sstatic {\n", indent)
		serializeTreeIndent(static, buf, indent+"\t")
		_, _ = fmt.Fprintf(buf, "%s}\n", indent)
	}

	// Write template block.
	if tmpl := tree.GetContainer("template"); tmpl != nil {
		_, _ = fmt.Fprintf(buf, "%stemplate {\n", indent)
		serializeTreeIndent(tmpl, buf, indent+"\t")
		_, _ = fmt.Fprintf(buf, "%s}\n", indent)
	}

	// Write announce block.
	if announce := tree.GetContainer("announce"); announce != nil {
		_, _ = fmt.Fprintf(buf, "%sannounce {\n", indent)
		serializeTreeIndent(announce, buf, indent+"\t")
		_, _ = fmt.Fprintf(buf, "%s}\n", indent)
	}
}
