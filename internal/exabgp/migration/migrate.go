// Design: docs/architecture/core-design.md -- ExaBGP migration orchestration
// Detail: env.go -- ExaBGP env file migration
// Detail: migrate_bus.go -- bus endpoint emission and capability flags for ExaBGP processes
// Detail: migrate_routes.go -- route conversion to Ze update blocks
// Detail: migrate_family.go -- family and nexthop syntax conversion
// Detail: migrate_serialize.go -- config tree serialization
// Detail: migrate_defaults.go -- ExaBGP defaults the migration states

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

// configFalse represents the string value "false" used in config trees.
const configFalse = "false"

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
//   - neighbor -> bgp { group <group> { peer <name> { ... } } }
//   - process -> bus { endpoint <name> { external { run "ze exabgp plugin ..."; } } }
//   - api { processes [...] } -> endpoint receive/send bus filters and --form / capability flags
//   - capability { route-refresh; } -> capability { route-refresh enable; }
//   - template { neighbor X { } } + inherit X -> expanded peer
//   - Every migrated peer carries explicit import and export accept policies
//   - If GR, route-refresh, or API processes: declare bus > endpoint bgp-rib { internal bgp-rib; }
func MigrateFromExaBGP(tree *config.Tree) (*MigrateResult, error) {
	if tree == nil {
		return nil, ErrNilTree
	}

	result := &MigrateResult{
		Tree: config.NewTree(),
	}

	// Collect templates for inheritance expansion.
	templates := collectTemplates(tree)

	// Check if we need the RIB endpoint.
	if needsRIBPlugin(tree) {
		result.RIBInjected = true
		ensureInternalEndpoint(result.Tree, "bgp-rib")
	}

	// Migrate ExaBGP process definitions -> external bus endpoint metadata.
	processes, endpointNames := migrateProcesses(tree, result)
	apiVersion4 := isExaBGPAPIVersion4(tree)

	// Migrate neighbors -> peers (with template expansion) and emit script endpoints.
	if err := migrateNeighbors(tree, result, processes, endpointNames, apiVersion4, templates); err != nil {
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
// A template counts as a neighbor, because its blocks are expanded into the
// neighbors that inherit it.
func needsRIBPlugin(tree *config.Tree) bool {
	if tree == nil {
		return false
	}

	for _, neighborTree := range tree.GetList("neighbor") {
		if neighborNeedsRIB(neighborTree) {
			return true
		}
	}
	for _, neighborTree := range collectTemplates(tree) {
		if neighborNeedsRIB(neighborTree) {
			return true
		}
	}

	return false
}

// neighborNeedsRIB reports whether one ExaBGP neighbor relies on state that
// only ze's RIB plugin keeps.
func neighborNeedsRIB(neighborTree *config.Tree) bool {
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

	// A neighbor fed by a process owes the RIB whatever the process asks to
	// receive. ExaBGP keeps every route a process announced (adj-rib-out
	// defaults to true: "keep the routes sent, to send them again on a route
	// refresh or a new session", src/exabgp/configuration/grammar/tree/neighbor.py)
	// and ze keeps them only in the RIB plugin, so without it a reconnect
	// would lose them (qa/old/4.2 api-teardown). It also covers a process
	// asking for received UPDATEs. A neighbor can carry several api blocks,
	// named or anonymous (exabgp.yang, list api).
	for _, api := range neighborTree.GetList("api") {
		if len(extractProcessList(api, "processes")) > 0 {
			return true
		}
		if len(extractProcessList(api, processesMatchField)) > 0 {
			return true
		}
	}

	// ZeBGP-style process bindings (ExaBGP tree format) feed a process too.
	return len(neighborTree.GetList("process")) > 0
}

// migrateProcesses extracts ExaBGP process definitions so each becomes an
// external bus endpoint running `ze exabgp plugin` (E-1).
//
// Every leaf of the ExaBGP process block that changes what ze does is carried:
// `run` (the program and all its arguments), `respawn`, `encoder` and `on-exit`.
//
// It returns the ordered migrated processes and a map from each ExaBGP process
// name to the sanitized bus endpoint name chosen by endpointProcessNames.
func migrateProcesses(tree *config.Tree, result *MigrateResult) (processes []migratedProcess, endpointNames map[string]string) {
	processes = make([]migratedProcess, 0, 2)
	carried := make([]string, 0, 2)
	for _, entry := range tree.GetListOrdered("process") {
		if len(entry.Value.GetSlice("run")) > 0 {
			carried = append(carried, entry.Key)
		}
	}
	endpointNames = endpointProcessNames(carried)
	for _, entry := range tree.GetListOrdered("process") {
		// ExaBGP runs the program with every word after it as its argv, with no
		// shell; the endpoint `run` leaf is a shell command line, so the argv
		// is joined with bridge.JoinCommandLine to preserve every argument.
		argv := entry.Value.GetSlice("run")
		if len(argv) == 0 {
			continue
		}
		runCmd := bridge.JoinCommandLine(argv)
		result.Processes = append(result.Processes, ExternalProcess{
			Name:   entry.Key,
			RunCmd: runCmd,
		})
		encoder, err := exabgpEncoder(entry.Value)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("process %s: %v", entry.Key, err))
		}
		endpointName := endpointNames[entry.Key]
		if endpointName != entry.Key {
			result.Warnings = append(result.Warnings, renamedProcessWarning(entry.Key, endpointName))
		}
		processes = append(processes, migratedProcess{
			OrigName:     entry.Key,
			EndpointName: endpointName,
			RunCmd:       runCmd,
			Respawn:      exabgpRespawn(entry.Value),
			Encoder:      encoder,
			OnExit:       exabgpOnExit(entry.Value),
		})
	}
	return processes, endpointNames
}

// exabgpOnExit reads a process block's `on-exit` leaf as written. ExaBGP
// accepts `keep` and `withdraw`; any other word is carried as it is, so the ze
// config refuses it by name at load rather than the migration dropping it.
func exabgpOnExit(process *config.Tree) string {
	value, ok := process.Get("on-exit")
	if !ok {
		return ""
	}
	return strings.ToLower(strings.Trim(value, `"';`))
}

// exabgpEncoder reads a process block's `encoder` leaf.
//
// An absent leaf answers EncoderUnspecified, and the endpoint's `run` line
// omits `--encoder`, leaving the `ze exabgp plugin` wrapper's default (`text`)
// in place.
//
// A word that names neither format is reported as a warning and `--encoder` is
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

// migrateNeighbors converts ExaBGP neighbors to ZeBGP peers inside groups.
// Peers are grouped by their ExaBGP template name. Peers without a template
// go into a "default" group.
func migrateNeighbors(
	tree *config.Tree,
	result *MigrateResult,
	processes []migratedProcess,
	endpointNames map[string]string,
	apiVersion4 bool,
	templates map[string]*config.Tree,
) error {
	// Track which group each peer belongs to.
	groups := make(map[string]*config.Tree) // group name -> group tree

	endpoints := newScriptEndpointSet(processes)

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
		expandedTree, err := expandInheritance(neighborTree, templates)
		if err != nil {
			return err
		}

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

		// A watchdog-controlled route needs the bgp-watchdog endpoint loaded.
		if peerHasWatchdogRoute(peer) {
			ensureInternalEndpoint(result.Tree, watchdogPluginName)
		}

		// Collect ExaBGP api / process bindings for bus endpoint filters and
		// virtual session capability options.
		bound, err := collectProcessBindings(expandedTree, endpointNames, result.Processes)
		if err != nil {
			return fmt.Errorf("neighbor %s: %w", addr, err)
		}
		var pathBuf textbuf.Buffer
		peerPath := pathBuf.Str("/bgp/group/").Str(groupName).Str("/peer/").Str(peerName).String()
		if err := endpoints.record(peerPath, addr, extractNeighborCaps(expandedTree), bound); err != nil {
			return err
		}

		// Any route an API process announces can name a watchdog
		// (`announce route ... watchdog <name>`, ExaBGP c43f90f79), and the
		// translator hands such a route to bgp-watchdog, which sends it.
		if len(bound) > 0 {
			ensureInternalEndpoint(result.Tree, watchdogPluginName)
		}

		// RFC 8955 Section 4 keeps FlowSpec MP_REACH next-hop length at 0 by
		// default. Preserve ExaBGP's draft-simpson 0x0800 next-hop behavior on
		// migrated peers that carry a Simpson static route or serve an API
		// process on a FlowSpec family.
		if peerNeedsFlowSpecRedirectNextHop(peer, len(bound) > 0) {
			peerSession(peer).Set(zeFlowRedirectNextHopLeaf, configTrue)
		}

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

	endpoints.write(result.Tree, apiVersion4)
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
// omits its endpoint bus filters to keep the registered wiring.
type processGrant struct {
	Name     string
	Events   []string
	Selected bool
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
func expandInheritance(neighbor *config.Tree, templates map[string]*config.Tree) (*config.Tree, error) {
	// Check for inherit field.
	inheritName, hasInherit := neighbor.Get("inherit")
	if !hasInherit {
		return neighbor, nil
	}

	// Look up template.
	tmpl, found := templates[inheritName]
	if !found {
		// Template not found - return original (warning could be added).
		return neighbor, nil
	}

	// Create merged tree: template first, then neighbor overrides.
	merged := tmpl.Clone()

	// Merge simple values (neighbor overrides template).
	// These are the known leaf fields in ExaBGP neighbor config.
	leafFields := []string{
		"description", "router-id", "local-address", "local-link-local", "local-as", "peer-as",
		"hold-time", "passive", "listen", "connect", "ttl-security",
		"md5-password", "md5-base64", "group-updates", "manual-eor", "shutdown",
		"as-set", "tunnel-encapsulation",
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
			if err := merged.MergeContainer(key, c.Clone(), nil); err != nil {
				return nil, fmt.Errorf("inherit %s: %w", inheritName, err)
			}
		}
	}

	// Merge list entries (append neighbor's to template's).
	// For static routes, we want template routes + neighbor routes.
	// Both trees are ExaBGP input, which keeps its own `process` list
	// (exabgp.yang); the rename applies to ze-native output alone.
	// The api entries of the template and of the neighbor are both kept, which
	// is what ExaBGP does with them: ParseAPI.flatten unions the processes and
	// the per-direction flags of every api block a neighbor holds
	// (src/exabgp/configuration/neighbor/api.py). Duplicate peer references for
	// the same process are merged in scriptEndpointSet.record.
	listFields := []string{"process", "static", "api"}
	for _, key := range listFields {
		for _, entry := range neighbor.GetListOrdered(key) {
			merged.AddListEntry(key, entry.Key, entry.Value.Clone())
		}
	}

	return merged, nil
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
//   - manual-eor -> behavior > manual-eor
//   - local-link-local -> session > link-local <addr>
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

	// ExaBGP "hold-time" -> Ze "timer > receive-hold-time". The leaf is written
	// for every neighbor, stated or not, because the two defaults differ:
	// exabgpHoldTime answers ExaBGP's 180 where ze's own leaf would answer 90.
	timerContainer := config.NewTree()
	timerContainer.Set("receive-hold-time", exabgpHoldTime(src))
	dst.SetContainer("timer", timerContainer)

	// ExaBGP "local-link-local" -> Ze session > link-local <addr>.
	//
	// The address is the whole of it. A connection > link-local boolean was
	// written here too until nothing was found that read it, and a leaf that
	// changes no behavior is worse in a migrated config than in a schema: the
	// operator reads it back as a setting that took effect.
	if v, ok := src.Get("local-link-local"); ok {
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

	// Two ExaBGP neighbor leaves keep their name inside ze's behavior
	// container (ze-bgp-conf.yang, peer > behavior).
	//
	// ExaBGP's `auto-flush` is accepted (exabgp.yang) and written nowhere,
	// because ExaBGP itself ignores it. Its only reader was the line in
	// peer.py that commit d337e1519 ("always check for new routes in
	// outgoing RIB", 2017, first in 4.0.4) deleted:
	//   self._have_routes = self.neighbor.flush if update is None else update
	// Since then 4.2.25 (configuration/neighbor/__init__.py), 5.0.1
	// (bgp/neighbor.py) and f9bfb853b (bgp/neighbor/neighbor.py) only parse,
	// compare and print it, and every route goes out at once. Writing nothing
	// gives a migrated peer the same effect on the remote router.
	for _, field := range []string{"group-updates", "manual-eor"} {
		v, ok := src.Get(field)
		if !ok {
			continue
		}
		dst.GetOrCreateContainer("behavior").Set(field, v)
	}

	// ExaBGP `shutdown true` starts the neighbor disabled until the API's
	// `neighbor <ip> enable` (ExaBGP peer.py, issue #1013). ze's peer shutdown
	// container holds the peer the same way; `shutdown false` is the default and
	// writes nothing.
	if v, ok := src.Get("shutdown"); ok && v == configTrue {
		dst.GetOrCreateContainer("shutdown")
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
			localContainer.Set("accept", configTrue)
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
		remoteContainer.Set("connect", configFalse)
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

// keepExaBGPSoftwareVersionFraming gives a migrated software-version
// capability the framing ExaBGP sends: a length octet before the version
// string, which the software-version encoding leaf calls legacy. Ze's default
// is the draft's bare string, so a migrated peer left on the default would
// change what goes on the wire, and an FRR neighbor that accepted ExaBGP's
// OPEN would refuse Ze's.
func keepExaBGPSoftwareVersionFraming(dstCap *config.Tree) {
	if container := dstCap.GetContainer("software-version"); container != nil {
		container.Set("encoding", "legacy")
		return
	}
	mode, ok := dstCap.GetFlex("software-version")
	if !ok {
		return
	}
	dstCap.Delete("software-version")
	container := config.NewTree()
	container.Set("mode", mode)
	container.Set("encoding", "legacy")
	dstCap.SetContainer("software-version", container)
}

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
			if v == "disable" || v == configFalse {
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
				// Copy the container, stating ExaBGP's Restart Time when it
				// names none (exabgpGracefulRestartTime).
				dstCap.SetContainer(field, migratedCapabilityContainer(src, field, container))
				hasCapabilities = true
				continue
			}
			// Check for value form (e.g., graceful-restart 120;).
			if v, ok := srcCap.GetFlex(field); ok {
				dstCap.Set(field, migratedCapabilityValue(src, field, v))
				hasCapabilities = true
			}
		}

		keepExaBGPSoftwareVersionFraming(dstCap)
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
// Ze unified: capability { add-path { direction send/receive; family { ipv4/unicast { limit { receive 10; } } } } }.
// ExaBGP's per-family limit is the receive request its OPEN advertises
// (PATHS-LIMIT), and ExaBGP has no send-side limit, so only `receive` is
// written. A family named without a limit gets no limit container.
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
					limit := config.NewTree()
					limit.Set("receive", p)
					entry.SetContainer("limit", limit)
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
// error of a flow route whose scope block ze cannot express, and of a route
// attribute ExaBGP itself refuses. The flow routes ExaBGP drops with a warning
// are left out with a warning in result.
func copyContainers(src, dst *config.Tree, result *MigrateResult) error {
	// Copy and convert family block.
	// ExaBGP: "ipv4 unicast" -> ZeBGP list entries: key="ipv4/unicast".
	// Families go into session > family.
	convertFamilyToList(src.GetContainer("family"), dst)

	// Convert announce block to update blocks.
	if announce := src.GetContainer("announce"); announce != nil {
		if err := convertAnnounceToUpdate(announce, dst); err != nil {
			return err
		}
	}

	// Convert static block to update blocks.
	if static := src.GetContainer("static"); static != nil {
		if err := convertStaticToUpdate(static, dst); err != nil {
			return err
		}
	}

	// Convert flow block to update blocks.
	if flow := src.GetContainer("flow"); flow != nil {
		if err := convertFlowToUpdate(flow, dst, result); err != nil {
			return err
		}
	}

	// Convert neighbor-level l2vpn block to update blocks.
	// ExaBGP has l2vpn { vpls ... } at the neighbor level for VPLS routes.
	if l2vpn := src.GetContainer("l2vpn"); l2vpn != nil {
		if err := convertL2VPNToUpdate(l2vpn, dst); err != nil {
			return err
		}
	}

	// RFC 8950: nexthop block is now moved into capability block by migrateCapability.
	return nil
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

// collectProcessBindings reads ExaBGP `api` blocks and ze-native `process`
// blocks on one neighbor and returns the process grants they declare.
//
// declared is every ExaBGP process the config runs, which is what a
// processes-match pattern selects from.
func collectProcessBindings(src *config.Tree, endpointNames map[string]string, declared []ExternalProcess) ([]processGrant, error) {
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
			if _, ok := endpointNames[name]; !ok {
				continue // No endpoint created for this process -- skip binding.
			}
			bound = append(bound, processGrant{Name: name, Events: events, Selected: true})
		}
	}

	// Then, handle ZeBGP-style process blocks (ordered for deterministic output).
	for _, entry := range src.GetListOrdered("process") {
		key := entry.Key
		procTree := entry.Value

		// Check if this is old-style (has "processes" field) or new-style (named).
		processNames := extractProcessList(procTree, "processes")

		if len(processNames) > 0 {
			for _, name := range processNames {
				if _, ok := endpointNames[name]; !ok {
					continue // No endpoint created -- skip binding.
				}
				bound = append(bound, processGrant{Name: name})
			}
		} else if key != config.KeyDefault {
			if _, ok := endpointNames[key]; !ok {
				continue // No endpoint created -- skip binding.
			}
			bound = append(bound, processGrant{Name: key})
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

	reactor.Set("update-groups", configFalse)
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
	if err := copyContainers(neighborTree, peer, result); err != nil {
		return nil, err
	}

	// RFC 8950 Section 4: ask for the Extended Next Hop Encoding the migrated
	// IPv4 routes with an IPv6 next hop need, as ExaBGP does at load.
	if err := addExtendedNextHopEntries(neighborTree, peer, peerName, result); err != nil {
		return nil, err
	}

	// ExaBGP 6.0 receive policy: write ze's leaf, or refuse a value ze cannot reproduce.
	if err := applyReceivePolicy(neighborTree, peer); err != nil {
		return nil, err
	}

	// Preserve ExaBGP's exchange intent on this peer only. RFC 8212 requires
	// explicit import/export policy for EBGP; IBGP is stated too so a later
	// AS change does not silently change the migrated neighbor's intent.
	filter := config.NewTree()
	for _, direction := range []string{"import", "export"} {
		policy := config.NewTree()
		policy.Set("final", "accept")
		filter.SetContainer(direction, policy)
	}
	peer.SetContainer("filter", filter)
	result.Warnings = append(result.Warnings, fmt.Sprintf(
		"peer %s: added explicit import and export final accept policies to preserve ExaBGP route-exchange intent under RFC 8212", peerName))

	// Check for unsupported features.
	checkUnsupported(neighborTree, result)

	return peer, nil
}
