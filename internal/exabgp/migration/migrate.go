// Design: docs/architecture/core-design.md -- ExaBGP migration orchestration
// Detail: env.go -- ExaBGP env file migration
// Detail: migrate_routes.go -- route conversion to Ze update blocks
// Detail: migrate_family.go -- family and nexthop syntax conversion
// Detail: migrate_serialize.go -- config tree serialization

// Package migration converts ExaBGP configuration to Ze format.
package migration

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
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

// MigrateResult holds the outcome of ExaBGP->ZeBGP migration.
type MigrateResult struct {
	Tree        *config.Tree      // Transformed tree
	RIBInjected bool              // True if RIB plugin was auto-injected
	Warnings    []string          // Non-fatal issues found
	Processes   []ExternalProcess // ExaBGP processes (handled by wrapper, not Ze)
}

// MigrateFromExaBGP converts an ExaBGP config tree to ZeBGP format.
//
// Transformations applied:
//   - neighbor -> peer
//   - process -> plugin (wrapped with ze exabgp plugin bridge)
//   - process { processes [...] } -> process NAME { ... } inside peer
//   - capability { route-refresh; } -> capability { route-refresh enable; }
//   - template { neighbor X { } } + inherit X -> expanded peer
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

	// Migrate processes -> plugins (wrapped with bridge)
	processMap := migrateProcesses(tree, result)

	// Migrate neighbors -> peers (with template expansion)
	if err := migrateNeighbors(tree, result, processMap, needsRIB, templates); err != nil {
		return nil, err
	}

	// Copy other top-level items (excluding templates - they're expanded)
	copyOtherItems(tree, result)

	// ExaBGP builds UPDATEs per-peer with no cross-peer sharing.
	// Disable update groups to preserve this behavior in migrated configs.
	injectUpdateGroupsDisabled(result.Tree)

	return result, nil
}

// collectTemplates extracts template definitions for inheritance expansion.
// Returns map of template name -> neighbor tree.
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

	// Check neighbors for GR, route-refresh, or receive [ update ].
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

		// Check ExaBGP api block with receive { update; } (ExaBGP format).
		if api := neighborTree.GetContainer("api"); api != nil {
			if recv := api.GetContainer("receive"); recv != nil {
				if _, ok := recv.GetFlex("update"); ok {
					return true
				}
			}
		}

		// Check ZeBGP-style process bindings with receive [ update ] (ExaBGP tree format).
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
// Returns an empty map -- no process bindings are created since there are no plugins.
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
	// Ze validates that process bindings reference defined plugins -- undefined refs are fatal.
	return make(map[string]string)
}

// migrateNeighbors converts ExaBGP neighbors to ZeBGP peers inside groups.
// Peers are grouped by their ExaBGP template name. Peers without a template
// go into a "default" group.
func migrateNeighbors(tree *config.Tree, result *MigrateResult, processMap map[string]string, needsRIB bool, templates map[string]*config.Tree) error {
	// Track which group each peer belongs to.
	groups := make(map[string]*config.Tree) // group name -> group tree

	// Counter for generating peer names when no description is available.
	peerCounter := 0

	// Use ordered iteration for deterministic output.
	for _, entry := range tree.GetListOrdered("neighbor") {
		addr := entry.Key
		neighborTree := entry.Value

		// Determine which group this peer belongs to.
		groupName := "default"
		if inheritName, hasInherit := neighborTree.Get("inherit"); hasInherit {
			groupName = inheritName
		}

		// Check for template inheritance and expand if found.
		expandedTree := expandInheritance(neighborTree, templates)

		// Derive peer name from description or generate "peer-N".
		peerName := derivePeerName(expandedTree, &peerCounter)

		// Convert neighbor to peer.
		peer, err := migrateSingleNeighbor(expandedTree, result)
		if err != nil {
			return fmt.Errorf("neighbor %s: %w", addr, err)
		}

		// Store the neighbor IP address as connection > remote > ip.
		connContainer := peer.GetContainer("connection")
		if connContainer == nil {
			connContainer = config.NewTree()
			peer.SetContainer("connection", connContainer)
		}
		remoteContainer := connContainer.GetContainer("remote")
		if remoteContainer == nil {
			remoteContainer = config.NewTree()
			connContainer.SetContainer("remote", remoteContainer)
		}
		remoteContainer.Set("ip", addr)

		// If RIB was injected, bind it to this peer.
		if needsRIB {
			bindRIBProcess(peer, expandedTree)
		}

		// Migrate process bindings (old: process { processes [...] } -> new: process NAME { ... }).
		migrateProcessBindings(expandedTree, peer, processMap)

		// Get or create group tree.
		groupTree, ok := groups[groupName]
		if !ok {
			groupTree = config.NewTree()
			groups[groupName] = groupTree
		}

		// Add peer to group using derived name (not IP address).
		groupTree.AddListEntry("peer", peerName, peer)
	}

	// Add all groups to result tree (sorted for deterministic output).
	sortedGroupNames := make([]string, 0, len(groups))
	for name := range groups {
		sortedGroupNames = append(sortedGroupNames, name)
	}
	sort.Strings(sortedGroupNames)
	for _, name := range sortedGroupNames {
		result.Tree.AddListEntry("group", name, groups[name])
	}

	return nil
}

// derivePeerName generates a peer name from the neighbor's description field,
// or falls back to "peer-N" with an incrementing counter.
// The name is sanitized to contain only ASCII alphanumerics, hyphens, and underscores.
func derivePeerName(neighborTree *config.Tree, counter *int) string {
	if desc, ok := neighborTree.Get("description"); ok && desc != "" {
		name := sanitizePeerName(strings.Trim(desc, `"'`))
		if name != "" {
			return name
		}
	}
	*counter++
	return fmt.Sprintf("peer-%d", *counter)
}

// sanitizePeerName converts a description into a valid peer name.
// Replaces spaces and invalid characters with hyphens, collapses runs of hyphens,
// and trims leading/trailing hyphens.
func sanitizePeerName(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevHyphen := false
	for _, ch := range s {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			b.WriteRune(ch)
			prevHyphen = false
		} else if !prevHyphen {
			b.WriteByte('-')
			prevHyphen = true
		}
	}
	return strings.Trim(b.String(), "-")
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
			// ExaBGP "local-link-local" -> Ze "link-local"
			outKey := key
			if key == "local-link-local" {
				outKey = "link-local"
			}
			// ExaBGP "passive true" -> Ze connection { local { connect false } } (handled in copySimpleFields)
			if key == "passive" {
				merged.Set("passive", v)
				continue
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
// Fields that move into new containers:
//   - peer-as -> session > asn > remote
//   - local-as -> session > asn > local
//   - local-address -> connection > local > ip
//   - router-id -> session > router-id
//   - passive -> connection > local > connect false, connection > remote > accept true
//   - ttl-security -> connection > ttl > max
//   - md5-password -> connection > md5 > password
//   - group-updates -> behavior > group-updates
//   - auto-flush -> behavior > auto-flush
//   - local-link-local -> connection > link-local true + session > link-local <addr>
func copySimpleFields(src, dst *config.Tree) {
	// Fields that remain as direct leaves on the peer.
	directFields := []string{
		"description",
	}

	for _, field := range directFields {
		if v, ok := src.Get(field); ok {
			dst.Set(field, v)
		}
	}

	// ExaBGP "hold-time" -> Ze "timer > receive-hold-time"
	if v, ok := src.Get("hold-time"); ok {
		timerContainer := config.NewTree()
		timerContainer.Set("receive-hold-time", v)
		dst.SetContainer("timer", timerContainer)
	}

	// ExaBGP "local-link-local" -> Ze connection > link-local true + session > link-local <addr>
	if v, ok := src.Get("local-link-local"); ok {
		// Set connection > link-local true
		connContainer := dst.GetContainer("connection")
		if connContainer == nil {
			connContainer = config.NewTree()
			dst.SetContainer("connection", connContainer)
		}
		connContainer.Set("link-local", configTrue)

		// Set session > link-local <addr>
		sessionContainer := dst.GetContainer("session")
		if sessionContainer == nil {
			sessionContainer = config.NewTree()
			dst.SetContainer("session", sessionContainer)
		}
		sessionContainer.Set("link-local", v)
	}

	// ExaBGP "router-id" -> Ze session > router-id
	if v, ok := src.Get("router-id"); ok {
		sessionContainer := dst.GetContainer("session")
		if sessionContainer == nil {
			sessionContainer = config.NewTree()
			dst.SetContainer("session", sessionContainer)
		}
		sessionContainer.Set("router-id", v)
	}

	// ExaBGP "ttl-security" -> Ze connection > ttl > max
	if v, ok := src.Get("ttl-security"); ok {
		connContainer := dst.GetContainer("connection")
		if connContainer == nil {
			connContainer = config.NewTree()
			dst.SetContainer("connection", connContainer)
		}
		ttlContainer := config.NewTree()
		ttlContainer.Set("max", v)
		connContainer.SetContainer("ttl", ttlContainer)
	}

	// ExaBGP "md5-password" -> Ze connection > md5 > password
	if v, ok := src.Get("md5-password"); ok {
		connContainer := dst.GetContainer("connection")
		if connContainer == nil {
			connContainer = config.NewTree()
			dst.SetContainer("connection", connContainer)
		}
		md5Container := config.NewTree()
		md5Container.Set("password", v)
		connContainer.SetContainer("md5", md5Container)
	}

	// ExaBGP "group-updates" -> Ze behavior > group-updates
	if v, ok := src.Get("group-updates"); ok {
		behaviorContainer := dst.GetContainer("behavior")
		if behaviorContainer == nil {
			behaviorContainer = config.NewTree()
			dst.SetContainer("behavior", behaviorContainer)
		}
		behaviorContainer.Set("group-updates", v)
	}

	// ExaBGP "auto-flush" -> Ze behavior > auto-flush
	if v, ok := src.Get("auto-flush"); ok {
		behaviorContainer := dst.GetContainer("behavior")
		if behaviorContainer == nil {
			behaviorContainer = config.NewTree()
			dst.SetContainer("behavior", behaviorContainer)
		}
		behaviorContainer.Set("auto-flush", v)
	}

	// Fields that move into session > asn: local-as -> local, peer-as -> remote.
	localAS, hasLocalAS := src.Get("local-as")
	peerAS, hasPeerAS := src.Get("peer-as")
	if hasLocalAS || hasPeerAS {
		sessionContainer := dst.GetContainer("session")
		if sessionContainer == nil {
			sessionContainer = config.NewTree()
			dst.SetContainer("session", sessionContainer)
		}
		asnContainer := sessionContainer.GetContainer("asn")
		if asnContainer == nil {
			asnContainer = config.NewTree()
			sessionContainer.SetContainer("asn", asnContainer)
		}
		if hasLocalAS {
			asnContainer.Set("local", localAS)
		}
		if hasPeerAS {
			asnContainer.Set("remote", peerAS)
		}
	}

	// Fields that move into connection > local: local-address -> ip, passive -> connect false.
	localAddr, hasLocalAddr := src.Get("local-address")
	passive, hasPassive := src.Get("passive")
	isPassive := hasPassive && passive == configTrue
	if hasLocalAddr || isPassive {
		connContainer := dst.GetContainer("connection")
		if connContainer == nil {
			connContainer = config.NewTree()
			dst.SetContainer("connection", connContainer)
		}
		localContainer := connContainer.GetContainer("local")
		if localContainer == nil {
			localContainer = config.NewTree()
			connContainer.SetContainer("local", localContainer)
		}
		if hasLocalAddr {
			localContainer.Set("ip", localAddr)
		}
		if isPassive {
			localContainer.Set("connect", "false")
		}
	}

	// Passive also sets connection > remote > accept true.
	if isPassive {
		connContainer := dst.GetContainer("connection")
		if connContainer == nil {
			connContainer = config.NewTree()
			dst.SetContainer("connection", connContainer)
		}
		remoteContainer := connContainer.GetContainer("remote")
		if remoteContainer == nil {
			remoteContainer = config.NewTree()
			connContainer.SetContainer("remote", remoteContainer)
		}
		remoteContainer.Set("accept", "true")
	}
}

// migrateCapability converts ExaBGP capability syntax to ZeBGP.
// ExaBGP: capability { route-refresh; graceful-restart 120; }.
// ZeBGP: session { capability { route-refresh enable; graceful-restart 120; } }.
//
// RFC 8950: Infers nexthop capability from nexthop { } block presence.
func migrateCapability(src, dst *config.Tree) error {
	srcCap := src.GetContainer("capability")
	dstCap := config.NewTree()
	hasCapabilities := false

	if srcCap != nil {
		// Reject unsupported ExaBGP capabilities (no ze runtime implementation).
		unsupported := []string{"multi-session", "operational", "aigp"}
		for _, field := range unsupported {
			if _, ok := srcCap.GetFlex(field); ok {
				return fmt.Errorf("unsupported capability %q: not implemented in ze", field)
			}
		}

		// Fields that need "enable" suffix (Flex type in schema).
		enableFields := []string{"route-refresh", "extended-message", "link-local-nexthop"}
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
	// ZeBGP: session { capability { nexthop { ipv4/unicast ipv6; } } }
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
	// ZeBGP: session { capability { hostname { host foo; domain bar; } } }
	migrateHostnameToCapability(src, dstCap, &hasCapabilities)

	if hasCapabilities {
		// Capabilities go into session > capability.
		sessionContainer := dst.GetContainer("session")
		if sessionContainer == nil {
			sessionContainer = config.NewTree()
			dst.SetContainer("session", sessionContainer)
		}
		sessionContainer.SetContainer("capability", dstCap)
	}
	return nil
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
	// ExaBGP: "ipv4 unicast" -> ZeBGP list entries: key="ipv4/unicast".
	// Families go into session > family.
	if fam := src.GetContainer("family"); fam != nil {
		convertFamilyToList(fam, dst)
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

// bindRIBProcess binds the RIB plugin to a peer.
func bindRIBProcess(peer, src *config.Tree) {
	ribProcess := config.NewTree()

	// Send flags: update, plus refresh if route-refresh is enabled.
	sendFlags := "[ update"
	if cap := src.GetContainer("capability"); cap != nil {
		if _, ok := cap.GetFlex("route-refresh"); ok {
			sendFlags += " refresh"
		}
	}
	sendFlags += " ]"
	ribProcess.Set("send", sendFlags)

	// Receive flags.
	ribProcess.Set("receive", "[ update ]")

	peer.AddListEntry("process", "rib", ribProcess)
}

// migrateProcessBindings converts ExaBGP api block and process blocks to ZeBGP named bindings.
// ExaBGP syntax: api { processes [ foo bar ]; }.
// ZeBGP syntax: process foo-compat { send [ update ]; }.
func migrateProcessBindings(src, dst *config.Tree, processMap map[string]string) {
	// First, handle ExaBGP-style api block.
	if api := src.GetContainer("api"); api != nil {
		processNames := extractProcessNames(api)
		for _, name := range processNames {
			newName, ok := processMap[name]
			if !ok {
				continue // No plugin created for this process -- skip binding.
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
					continue // No plugin created -- skip binding.
				}
				addProcessBinding(dst, newName)
			}
		} else if key != config.KeyDefault {
			// New-style named binding - copy with name mapping.
			newName, ok := processMap[key]
			if !ok {
				continue // No plugin created -- skip binding.
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

// addProcessBinding adds a process binding with default send flags.
func addProcessBinding(dst *config.Tree, name string) {
	proc := config.NewTree()
	proc.Set("send", "[ update ]")
	dst.AddListEntry("process", name, proc)
}

// checkUnsupported adds warnings for features that need manual migration.
func checkUnsupported(_ *config.Tree, _ *MigrateResult) {
	// L2VPN/VPLS: handled by convertL2VPNToUpdate.
	// Flow blocks: handled by convertFlowToUpdate.
}

// injectUpdateGroupsDisabled adds environment { reactor { update-groups false; } }
// to the output tree. ExaBGP builds UPDATEs per-peer; migrated configs preserve
// this behavior so users see identical output until they opt into update groups.
func injectUpdateGroupsDisabled(tree *config.Tree) {
	env := tree.GetContainer("environment")
	if env == nil {
		env = config.NewTree()
		tree.SetContainer("environment", env)
	}

	reactor := env.GetContainer("reactor")
	if reactor == nil {
		reactor = config.NewTree()
		env.SetContainer("reactor", reactor)
	}

	reactor.Set("update-groups", "false")
}

// copyOtherItems copies non-neighbor, non-process items.
// Templates are NOT copied - they are expanded via inheritance.
func copyOtherItems(src *config.Tree, result *MigrateResult) {
	// Templates are expanded via inherit, not copied.
	// Other top-level items could be copied here if needed.
}

// migrateSingleNeighbor converts a single neighbor tree to peer format.
// Used for both top-level neighbors and template neighbors.
func migrateSingleNeighbor(neighborTree *config.Tree, result *MigrateResult) (*config.Tree, error) {
	peer := config.NewTree()

	// Copy simple fields.
	copySimpleFields(neighborTree, peer)

	// Migrate capability block.
	if err := migrateCapability(neighborTree, peer); err != nil {
		return nil, err
	}

	// Copy other containers (family, etc.).
	copyContainers(neighborTree, peer)

	// Check for unsupported features.
	checkUnsupported(neighborTree, result)

	return peer, nil
}
