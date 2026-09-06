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
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/exabgp/bridge"
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
//   - process { processes [...] } -> attach process NAME { ... } inside peer
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
	needsRIB := needsRIBPlugin(tree)
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

// needsRIBPlugin checks if the config requires a RIB plugin.
// ZeBGP delegates RIB to plugins, so features requiring state storage need one.
func needsRIBPlugin(tree *config.Tree) bool {
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

		// Check ExaBGP api blocks with receive { update; } (ExaBGP format). A
		// neighbor can carry several api blocks, named or anonymous, and one
		// asking for UPDATE is enough (exabgp.yang, list api).
		for _, api := range neighborTree.GetList("api") {
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
	ribPlugin.Set("use", "bgp-rib")
	tree.AddListEntry("plugin", "bgp-rib", ribPlugin)
}

// bridgePluginName is the registry name of the in-process ExaBGP bridge, and it
// is the plugin every migrated ExaBGP process binds to.
const bridgePluginName = "exabgp-bridge"

// migrateProcesses converts ExaBGP process definitions into the in-process
// ExaBGP bridge.
//
// The bridge is the adapter between the two protocols: the script keeps writing
// ExaBGP API text on its stdout, and the bridge reads each line and answers the
// ze command that sends it (`TranslateLine`, internal/exabgp/bridge). So a
// migrated config starts the operator's script and their announcements reach
// the wire.
//
// This dropped every process until 2026-09-05, on the reasoning that "ExaBGP
// processes cannot run as Ze plugins because the protocols are incompatible".
// The bridge is what makes them compatible and it postdates that comment, so
// the config was losing the half of itself that produces routes: the script was
// collected into result.Processes, printed to stderr as `process:NAME:CMD`, and
// read by nothing.
//
// Every leaf of the ExaBGP process block that changes what ze does is carried:
// `run`, `respawn` and `encoder`. `encoder` was read by nothing until
// 2026-09-06, so a config asking for the text event format was migrated into a
// bridge that answered JSON, with no error and no log line.
func migrateProcesses(tree *config.Tree, result *MigrateResult) map[string]string {
	processMap := make(map[string]string)
	processes := make([]bridgeProcess, 0, 2)
	for _, entry := range tree.GetListOrdered("process") {
		runCmd, ok := entry.Value.Get("run")
		if !ok {
			continue
		}
		runCmd = strings.Trim(runCmd, `"'`)
		result.Processes = append(result.Processes, ExternalProcess{
			Name:   entry.Key,
			RunCmd: runCmd,
		})
		encoder, err := exabgpEncoder(entry.Value)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("process %s: %v", entry.Key, err))
		}
		processes = append(processes, bridgeProcess{
			Name:    entry.Key,
			RunCmd:  runCmd,
			Respawn: exabgpRespawn(entry.Value),
			Encoder: encoder,
		})
		processMap[entry.Key] = bridgePluginName
	}
	if len(processes) == 0 {
		return processMap
	}
	injectBridgePlugin(result.Tree, processes)
	return processMap
}

// bridgeProcess is one ExaBGP process block, in the form the bridge declares.
type bridgeProcess struct {
	Name   string
	RunCmd string
	// Encoder is the format the block asked its script's events be written in.
	Encoder bridge.Encoder
	// Respawn is ExaBGP's own default, true, unless the block says otherwise.
	Respawn bool
}

// exabgpEncoder reads a process block's `encoder` leaf.
//
// An absent leaf answers EncoderUnspecified, and injectBridgePlugin then writes
// no leaf, which leaves the ze default in place. ExaBGP's own leaf default is
// `text` (src/exabgp/configuration/process/__init__.py), and ExaBGP 6 overrides
// it to JSON for every process; ze declares the 6.0.0 envelope, so an ExaBGP
// config that stated nothing migrates to the JSON ze already answered.
//
// A word that names neither format is REPORTED as a warning and the leaf is
// left unwritten, so the migrated config states no encoder rather than an
// invented one.
func exabgpEncoder(process *config.Tree) (bridge.Encoder, error) {
	value, ok := process.Get("encoder")
	if !ok {
		return bridge.EncoderUnspecified, nil
	}
	return bridge.ParseEncoder(strings.ToLower(strings.Trim(value, `"';`)))
}

// exabgpRespawn reads a process block's `respawn` leaf. ExaBGP restarts a
// process that exits unless the block turns it off
// (src/exabgp/configuration/process/__init__.py, `'respawn': True`).
func exabgpRespawn(process *config.Tree) bool {
	value, ok := process.Get("respawn")
	if !ok {
		return true
	}
	return exabgpBoolean(value)
}

// exabgpFalse are the words an ExaBGP config writes to turn a flag OFF. Every
// other word, the bare keyword included, turns it on.
var exabgpFalse = []string{"false", "disable", "no"}

// exabgpBoolean reads one ExaBGP flag value. It is the one place the OFF
// vocabulary is written down, so the respawn leaf and an api block's event
// grants cannot disagree about what `disable` means.
func exabgpBoolean(value string) bool {
	return !slices.Contains(exabgpFalse, strings.ToLower(strings.Trim(value, `"';`)))
}

// injectBridgePlugin declares the bridge and the script it runs:
//
//	exabgp { bridge { process <name> { run "<command>"; respawn <v>; encoder <e> } } }
//	plugin { internal exabgp-bridge { use exabgp-bridge } }
//
// The family leaf is left out on purpose. It refines the ADD-PATH capability
// encoding the bridge negotiates for the script, and the neighbor already
// declares the families this config asks for.
//
// dst is the RESULT tree. Writing to the source tree loses the block: the
// source is read and discarded, and only result.Tree reaches the serializer.
func injectBridgePlugin(dst *config.Tree, processes []bridgeProcess) {
	block := config.NewTree()
	for _, process := range processes {
		entry := config.NewTree()
		entry.Set("run", process.RunCmd)
		// The leaf is written only to turn respawning OFF, because true is
		// both ExaBGP's default and the ze YANG default.
		if !process.Respawn {
			entry.Set("respawn", "disable")
		}
		// The encoder is written only when the ExaBGP block stated one, so an
		// absent leaf stays absent and the ze default decides.
		if process.Encoder != bridge.EncoderUnspecified {
			entry.Set("encoder", process.Encoder.String())
		}
		block.AddListEntry("process", process.Name, entry)
	}
	exabgp := config.NewTree()
	exabgp.SetContainer("bridge", block)
	dst.SetContainer("exabgp", exabgp)

	plugin := config.NewTree()
	plugin.Set("use", bridgePluginName)
	dst.AddListEntry("plugin", bridgePluginName, plugin)
}

// migrateNeighbors converts ExaBGP neighbors to ZeBGP peers inside groups.
// Peers are grouped by their ExaBGP template name. Peers without a template
// go into a "default" group.
func migrateNeighbors(tree *config.Tree, result *MigrateResult, processMap map[string]string, needsRIB bool, templates map[string]*config.Tree) error {
	// Track which group each peer belongs to.
	groups := make(map[string]*config.Tree) // group name -> group tree

	// scriptFeeds is the relation each ExaBGP process block needs and no
	// attachment can carry: the neighbors that named THIS process, and the
	// events each of them granted it. Every process reaches ze as the one
	// exabgp-bridge plugin, so the relation is written back into the process
	// block rather than read off the attachment.
	scriptFeeds := newScriptFeeds()

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
		peer, err := migrateSingleNeighbor(expandedTree, peerName, result)
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

		// A watchdog-controlled route needs bgp-watchdog fed this peer's state.
		if peerHasWatchdogRoute(peer) {
			bindWatchdogProcess(result.Tree, peer)
		}

		// Migrate process bindings (old: process { processes [...] } -> new: attach process NAME { ... }).
		bound, err := migrateProcessBindings(expandedTree, peer, processMap, result.Processes)
		if err != nil {
			return fmt.Errorf("neighbor %s: %w", addr, err)
		}
		scriptFeeds.record(addr, bound)

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
	slices.Sort(sortedGroupNames)
	for _, name := range sortedGroupNames {
		result.Tree.AddListEntry("group", name, groups[name])
	}

	scriptFeeds.write(result.Tree)
	return nil
}

// processGrant is what ONE neighbor's binding says about ONE ExaBGP process:
// the process it names, and the events it grants it.
//
// Selected says the binding STATED an event selection, which an `api` block
// always does and a ze-native `process` block never does. An empty Events with
// Selected true is a neighbor that feeds the script no event, which ExaBGP
// writes as `api { processes [ p ]; }` with no direction block. An empty Events
// with Selected false states nothing, and a script any such binding reaches
// keeps the every-peer-every-event default.
type processGrant struct {
	Name     string
	Events   []string
	Selected bool
}

// scriptFeeds accumulates the (peer, process) -> events relation while the
// neighbors are migrated, and writes it into the bridge root afterwards.
//
// It runs in two steps because the block it writes into was injected before the
// neighbors by migrateProcesses, and the relation is not known until every
// neighbor's api block has been read.
type scriptFeeds struct {
	// byProcess holds each process's peers, in the order the neighbors were
	// migrated, so the written config is deterministic.
	byProcess map[string][]scriptFeed
	// unscoped names the processes some binding stated no selection for. Such a
	// process keeps the every-peer-every-event default rather than a feed list
	// built from the bindings that did state one.
	unscoped map[string]bool
}

// scriptFeed is one neighbor's grant to one process.
type scriptFeed struct {
	Peer   string
	Events []string
}

func newScriptFeeds() *scriptFeeds {
	return &scriptFeeds{
		byProcess: make(map[string][]scriptFeed),
		unscoped:  make(map[string]bool),
	}
}

// record stores what one neighbor's bindings granted.
func (f *scriptFeeds) record(peer string, grants []processGrant) {
	for _, grant := range grants {
		if !grant.Selected {
			f.unscoped[grant.Name] = true
			continue
		}
		f.byProcess[grant.Name] = append(f.byProcess[grant.Name],
			scriptFeed{Peer: peer, Events: grant.Events})
	}
}

// write emits each script's feed blocks into `exabgp { bridge { process <name>
// { feed <peer> { event [ ... ] } } } }`.
//
// A process no binding scoped gets no feed block, which the bridge reads as
// every peer and every event. That is what a bridge whose config carries no
// feed list has always meant, and it is what a hand-written ze config with one
// script means.
func (f *scriptFeeds) write(dst *config.Tree) {
	exabgp := dst.GetContainer("exabgp")
	if exabgp == nil {
		return
	}
	block := exabgp.GetContainer("bridge")
	if block == nil {
		return
	}

	for _, process := range block.GetListOrdered("process") {
		if f.unscoped[process.Key] {
			continue
		}
		for _, feed := range f.byProcess[process.Key] {
			entry := config.NewTree()
			entry.Set("event", bracketList(feed.Events))
			process.Value.AddListEntry("feed", feed.Peer, entry)
		}
	}
}

// bracketList renders a leaf-list the way the ze config file writes one,
// `[ a b c ]`. The migration serializer writes plain values, so a leaf-list
// reaches the file as one already-bracketed string (addProcessBinding writes
// `receive [ * ]` the same way).
func bracketList(values []string) string {
	var tb textbuf.Buffer
	tb.Str("[ ")
	for _, value := range values {
		tb.Str(value).Byte(' ')
	}
	return tb.Byte(']').String()
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
	var b textbuf.Buffer
	return b.Reset().Str("peer-").Int(int64(*counter)).String()
}

// sanitizePeerName converts a description into a valid peer name.
// Replaces spaces and invalid characters with hyphens, collapses runs of hyphens,
// and trims leading/trailing hyphens.
func sanitizePeerName(s string) string {
	var b textbuf.Buffer
	b.Reset().Grow(len(s))
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
		"md5-password", "md5-base64", "group-updates", "auto-flush", "manual-eor",
	}
	for _, key := range leafFields {
		if v, ok := neighbor.Get(key); ok {
			// ExaBGP "local-link-local" -> Ze "link-local"
			outKey := key
			if key == "local-link-local" {
				outKey = "link-local"
			}
			// ExaBGP "passive true" -> Ze connection { remote { connect false } } (handled in copySimpleFields)
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
		"capability", "family", "nexthop",
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
	// Both trees are ExaBGP input, which keeps its own `process` list
	// (exabgp.yang); the rename applies to ze-native output alone.
	// The api entries of the template and of the neighbor are both kept, which
	// is what ExaBGP does with them: ParseAPI.flatten unions the processes and
	// the per-direction flags of every api block a neighbor holds
	// (src/exabgp/configuration/neighbor/api.py). A process named by both is
	// attached once (addProcessBinding).
	listFields := []string{"process", "static", "api"}
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
//   - passive -> connection > remote > connect false, connection > local > accept true
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

	// Three ExaBGP neighbor leaves keep their name inside ze's behavior
	// container (ze-bgp-conf.yang, peer > behavior).
	for _, field := range []string{"group-updates", "manual-eor", "auto-flush"} {
		v, ok := src.Get(field)
		if !ok {
			continue
		}
		dst.GetOrCreateContainer("behavior").Set(field, v)
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

	// Fields that move into connection > local: local-address -> ip, passive -> accept true.
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
			localContainer.Set("accept", "true")
		}
	}

	// Passive also sets connection > remote > connect false.
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
		remoteContainer.Set("connect", "false")
	}
}

// capabilityEnableFields and capabilityValueFields are the ExaBGP capability
// keywords migrateCapability carries across, the first group as `<field>
// enable` and the second keeping the value ExaBGP gave it. They are declared
// here, where the translation happens, and read by untranslatedCapabilities
// (migrate_unimplemented.go), which warns about every key in the block that is
// in neither group. A keyword this function grows a branch for joins a list
// here, and the warning stops naming it.
var (
	capabilityEnableFields = []string{"route-refresh", "extended-message", "link-local-nexthop"}
	capabilityValueFields  = []string{"graceful-restart", "software-version"}
)

// migrateCapability converts ExaBGP capability syntax to ZeBGP.
// ExaBGP: capability { route-refresh; graceful-restart 120; }.
// ZeBGP: session { capability { route-refresh enable; graceful-restart 120; } }.
//
// RFC 8950: Infers nexthop capability from nexthop { } block presence.
//
// It answers nothing: every keyword it does not translate is REPORTED by
// migrateSingleNeighbor rather than refused here (migrate_unimplemented.go), so
// there is no longer an ExaBGP capability block this function can fail on.
func migrateCapability(src, dst *config.Tree) {
	srcCap := src.GetContainer("capability")
	dstCap := config.NewTree()
	hasCapabilities := false

	if srcCap != nil {
		// Fields that need "enable" suffix (Flex type in schema).
		for _, field := range capabilityEnableFields {
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
		for _, field := range capabilityValueFields {
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

	// ADD-PATH: convert ExaBGP capability-level direction + neighbor-level per-family
	// into unified ze add-path block.
	// ExaBGP capability: add-path { send true; receive true; }
	// ExaBGP neighbor: add-path { ipv4 unicast; ipv4 unicast limit 10; }
	// Ze: capability { add-path { direction send/receive; family { ipv4/unicast { limit 10; } } } }
	migrateAddPathToUnified(srcCap, src, dstCap, &hasCapabilities)

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
}

const dirSendReceive = "send/receive"

// migrateAddPathToUnified converts ExaBGP add-path config into the unified ze format.
// ExaBGP has add-path at two levels:
//   - capability: add-path { send true; receive true; } (direction)
//   - neighbor: add-path { ipv4 unicast; ipv4 unicast limit 10; } (per-family + optional limit)
//
// Ze unified: capability { add-path { direction send/receive; family { ipv4/unicast { limit 10; } } } }.
func migrateAddPathToUnified(srcCap, src, dstCap *config.Tree, hasCapabilities *bool) {
	var direction string

	// Extract direction from capability-level add-path.
	if srcCap != nil {
		if container := srcCap.GetContainer("add-path"); container != nil {
			s, _ := container.Get("send")
			r, _ := container.Get("receive")
			switch {
			case s == configTrue && r == configTrue:
				direction = dirSendReceive
			case s == configTrue:
				direction = "send"
			case r == configTrue:
				direction = "receive"
			}
		} else if v, ok := srcCap.GetFlex("add-path"); ok {
			if v == configTrue || v == "enable" || v == "" {
				direction = dirSendReceive
			} else {
				direction = v
			}
		}
	}

	// Extract per-family entries from neighbor-level add-path.
	// ExaBGP freeform parser joins all words into the key: "ipv4 unicast limit 10" -> key, "true" -> value.
	var familyBlock *config.Tree
	if ap := src.GetContainer("add-path"); ap != nil {
		familyBlock = config.NewTree()
		for _, key := range ap.Values() {
			parts := strings.Fields(key)
			if len(parts) < 2 {
				continue
			}
			var tb textbuf.Buffer
			famKey := tb.Str(parts[0]).Byte('/').Str(parts[1]).String()
			entry := config.NewTree()
			for _, p := range parts[2:] {
				if n := parseLimit(p); n > 0 {
					entry.Set("limit", p)
				}
			}
			familyBlock.SetContainer(famKey, entry)
		}
	}

	if direction == "" && familyBlock == nil {
		return
	}

	// If no capability-level direction, default to send/receive.
	if direction == "" {
		direction = "send/receive"
	}

	addPathBlock := config.NewTree()
	addPathBlock.Set("direction", direction)
	if familyBlock != nil {
		addPathBlock.SetContainer("family", familyBlock)
	}
	dstCap.SetContainer("add-path", addPathBlock)
	*hasCapabilities = true
}

// parseLimit extracts a numeric limit value from a string token.
func parseLimit(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
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

// copyContainers copies container blocks from neighbor to peer. It reports the
// error of a flow route whose scope block ze cannot express.
func copyContainers(src, dst *config.Tree) error {
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
		if err := convertFlowToUpdate(flow, dst); err != nil {
			return err
		}
	}

	// Convert neighbor-level l2vpn block to update blocks.
	// ExaBGP has l2vpn { vpls ... } at the neighbor level for VPLS routes.
	if l2vpn := src.GetContainer("l2vpn"); l2vpn != nil {
		convertL2VPNToUpdate(l2vpn, dst)
	}

	// RFC 8950: nexthop block is now moved into capability block by migrateCapability.
	return nil
}

// bindRIBProcess binds the RIB plugin to a peer.
func bindRIBProcess(peer, src *config.Tree) {
	ribProcess := config.NewTree()

	// Send flags: what bgp-rib ORIGINATES toward the peer. UPDATEs always, and
	// ROUTE-REFRESH when the peer negotiated the capability: answering a refresh
	// request, bgp-rib dispatches the RFC 7313 BoRR and EoRR markers, which are
	// ROUTE-REFRESH messages on the wire (rib.go, dispatchPeerAction).
	sendFlags := "[ update"
	if cap := src.GetContainer("capability"); cap != nil {
		if _, ok := cap.GetFlex("route-refresh"); ok {
			sendFlags += " refresh"
		}
	}
	sendFlags += " ]"
	ribProcess.Set("send", sendFlags)

	// Receive flags: what bgp-rib DECLARES it consumes (rib.go
	// SetStartupSubscriptions -- update in both directions, state, refresh).
	// Delivery is the overlap of the declaration and this grant
	// (pluginserver.Server.PeerScopedProcs), so a narrower grant here silently
	// costs the plugin the peer-up replay that `state` drives.
	ribProcess.Set("receive", "[ update state refresh ]")

	attachedProcesses(peer).AddListEntry("process", "bgp-rib", ribProcess)
}

// watchdogPluginName is the plugin that owns watchdog-controlled routes.
const watchdogPluginName = "bgp-watchdog"

// peerHasWatchdogRoute reports whether any update this peer carries is
// watchdog-controlled (migrate_routes.go writes the block).
func peerHasWatchdogRoute(peer *config.Tree) bool {
	for _, entry := range peer.GetListOrdered("update") {
		if entry.Value.GetContainer("watchdog") != nil {
			return true
		}
	}
	return false
}

// bindWatchdogProcess attaches bgp-watchdog to a peer whose migrated config
// carries a watchdog-controlled route, and declares the plugin once.
//
// The plugin announces and withdraws those routes on the operator's command,
// and it can only do that for a session it has been told about: it defers an
// announce for a peer it has not seen come up (watchdog/server.go
// handleStateUp). Delivery is per peer and per event type, so without this the
// peer feeds it nothing, `request bgp watchdog announce` reports success, and
// no UPDATE reaches the wire.
func bindWatchdogProcess(tree, peer *config.Tree) {
	attached := attachedProcesses(peer)
	if attached.GetList("process")[watchdogPluginName] == nil {
		wdProcess := config.NewTree()
		wdProcess.Set("receive", "[ state ]")
		// The send half is what makes the migrated config WORK. bgp-watchdog
		// originates the peer's watchdog routes through update-route, and ze
		// refuses a process the peer does not attach with `send [ update ]`
		// (reactor/send_permission.go). Emitting the receive half alone
		// converted a working ExaBGP config into a silent one.
		wdProcess.Set("send", "[ update ]")
		attached.AddListEntry("process", watchdogPluginName, wdProcess)
	}
	if tree.GetList("plugin")[watchdogPluginName] != nil {
		return
	}
	wdPlugin := config.NewTree()
	wdPlugin.Set("use", watchdogPluginName)
	tree.AddListEntry("plugin", watchdogPluginName, wdPlugin)
}

// attachedProcesses returns the attach container of a ze-native peer tree,
// creating it. Ze-native output writes `attach process <name> { ... }`, so
// every binding this package emits lives one level down from the peer.
func attachedProcesses(peer *config.Tree) *config.Tree {
	return peer.GetOrCreateContainer("attach")
}

// migrateProcessBindings converts ExaBGP api block and process blocks to ze
// named attachments, and answers the ExaBGP process names this neighbor named.
// ExaBGP syntax: api { processes [ foo bar ]; } or api { processes-match [ ^foo ]; }.
// Ze syntax: attach process foo-compat { send [ update ] }.
//
// declared is every ExaBGP process the config runs, which is what a
// processes-match pattern selects from.
//
// The grants it answers are what the caller needs to write the RELATION back:
// every ExaBGP process reaches ze as the one `exabgp-bridge` plugin, so the
// attachment alone cannot say which SCRIPT this neighbor feeds, nor which
// events it feeds it. The bridge needs both, because a script is fed by the
// neighbors that named it, with the events each of them granted, and sends an
// unaddressed command to those neighbors alone (bridgerun.ScriptFeed).
func migrateProcessBindings(src, dst *config.Tree, processMap map[string]string, declared []ExternalProcess) ([]processGrant, error) {
	var bound []processGrant

	// First, handle ExaBGP-style api blocks. A neighbor can name several, and
	// ExaBGP unions their process lists rather than letting the last one win
	// (src/exabgp/configuration/neighbor/api.py, ParseAPI.flatten), so every
	// entry contributes its processes here.
	for _, entry := range src.GetListOrdered("api") {
		names := extractProcessList(entry.Value, "processes")
		patterns := extractProcessList(entry.Value, processesMatchField)

		// ExaBGP refuses a neighbor that carries both lists rather than
		// choosing between them (src/exabgp/configuration/configuration.py,
		// validate: "processes and processes-match are mutually exclusive").
		if len(names) > 0 && len(patterns) > 0 {
			return nil, fmt.Errorf("api %s: processes and processes-match are mutually exclusive", entry.Key)
		}
		if len(patterns) > 0 {
			matched, err := matchProcessNames(entry.Key, patterns, declared)
			if err != nil {
				return nil, err
			}
			names = matched
		}

		events := apiBlockEvents(entry.Value)
		for _, name := range names {
			newName, ok := processMap[name]
			if !ok {
				continue // No plugin created for this process -- skip binding.
			}
			bound = append(bound, processGrant{Name: name, Events: events, Selected: true})
			addProcessBinding(dst, newName)
		}
	}

	// Then, handle ZeBGP-style process blocks (ordered for deterministic output).
	for _, entry := range src.GetListOrdered("process") {
		key := entry.Key
		procTree := entry.Value

		// Check if this is old-style (has "processes" field) or new-style (named).
		processNames := extractProcessList(procTree, "processes")

		if len(processNames) > 0 {
			// Old-style: convert to named bindings.
			for _, name := range processNames {
				newName, ok := processMap[name]
				if !ok {
					continue // No plugin created -- skip binding.
				}
				bound = append(bound, processGrant{Name: name})
				addProcessBinding(dst, newName)
			}
		} else if key != config.KeyDefault {
			// New-style named binding - copy with name mapping.
			newName, ok := processMap[key]
			if !ok {
				continue // No plugin created -- skip binding.
			}
			bound = append(bound, processGrant{Name: key})
			attachedProcesses(dst).AddListEntry("process", newName, procTree.Clone())
		}
	}

	return bound, nil
}

// extractProcessList reads one bracketed list of an api block: the literal
// names of "processes", or the regular expressions of "processes-match".
func extractProcessList(tree *config.Tree, field string) []string {
	// Try multi-value first.
	processNames := tree.GetMultiValues(field)
	if len(processNames) > 0 {
		return processNames
	}

	// Try single value.
	if plist, ok := tree.Get(field); ok {
		// Parse process list: "[ name1 name2 ]" or "[ name1, name2 ]".
		plist = strings.Trim(plist, "[]")
		plist = strings.ReplaceAll(plist, ",", " ")
		return strings.Fields(plist)
	}

	return nil
}

// addProcessBinding attaches one process.
//
// The bridge gets every event and every send type. An ExaBGP script with
// `encoder json` is written against the full event stream, so a narrower
// receive list would drop the messages it reports on, and it announces and
// withdraws, so a narrower send list would refuse the routes it originates.
func addProcessBinding(dst *config.Tree, name string) {
	// Two api blocks can name the same process. AddListEntry keeps both under
	// generated keys, so the peer would attach the plugin twice.
	if attachedProcesses(dst).GetList("process")[name] != nil {
		return
	}

	proc := config.NewTree()
	if name == bridgePluginName {
		proc.Set("receive", "[ * ]")
		proc.Set("send", "[ * ]")
	} else {
		proc.Set("send", "[ update ]")
	}
	attachedProcesses(dst).AddListEntry("process", name, proc)
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
// Used for both top-level neighbors and template neighbors. peerName is the
// name the emitted peer carries, and the warnings this function writes name it
// so an operator with several neighbors knows which one they are about.
func migrateSingleNeighbor(neighborTree *config.Tree, peerName string, result *MigrateResult) (*config.Tree, error) {
	peer := config.NewTree()

	// Copy simple fields.
	copySimpleFields(neighborTree, peer)

	// Migrate capability block.
	migrateCapability(neighborTree, peer)

	// Say what the capability block asked for and the migration left out.
	if warning := untranslatedCapabilityWarning(neighborTree.GetContainer("capability"), peerName); warning != "" {
		result.Warnings = append(result.Warnings, warning)
	}

	// Copy other containers (family, etc.).
	if err := copyContainers(neighborTree, peer); err != nil {
		return nil, err
	}

	// Check for unsupported features.
	checkUnsupported(neighborTree, result)

	return peer, nil
}
