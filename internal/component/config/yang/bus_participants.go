// Design: docs/architecture/config/yang-config-design.md — synthesized bus participant schema augmentation
// Related: loader.go — resolveDefault invokes loadBusParticipants before Resolve
// Related: loader_stages.go — StageLoad invokes loadBusParticipants to match resolveDefault
// Related: join.go — resolveJoined invokes loadBusParticipants over compiled and external modules

package yang

import (
	"slices"
	"strings"
	"sync"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	busParticipantsModuleName = "ze-bus-participants"
	busPluginConfModuleName   = "ze-plugin-conf"
	busExtensionsModuleName   = "ze-extensions"
	busContainerName          = "bus"
	confModuleSuffix          = "-conf"
	configFalseValue          = "false"
	maxASTWalkDepth           = 64
)

var (
	busTargetsMu       sync.RWMutex
	busStaticTargets   = map[string]struct{}{}
	busTargetsProvider func() []string
)

// RegisterBusParticipantTarget registers a schema path (for example
// "/alpha/item") to be augmented with a `container bus` carrying
// `uses plug:participant-bus`. Safe for concurrent use.
func RegisterBusParticipantTarget(path string) {
	normalized := normalizeBusTargetPath(path)
	if normalized == "" {
		return
	}
	busTargetsMu.Lock()
	busStaticTargets[normalized] = struct{}{}
	busTargetsMu.Unlock()
}

// SetBusParticipantTargetsProvider installs a dynamic provider supplying
// additional participant schema paths (for example from registered compiled-in
// or external plugin declarations). Passing nil clears the provider. Safe for
// concurrent use.
func SetBusParticipantTargetsProvider(fn func() []string) {
	busTargetsMu.Lock()
	busTargetsProvider = fn
	busTargetsMu.Unlock()
}

// BusParticipantTargets answers the sorted, deduplicated union of statically
// registered and dynamically provided participant schema paths. Safe for
// concurrent use.
func BusParticipantTargets() []string {
	busTargetsMu.RLock()
	seen := make(map[string]struct{}, len(busStaticTargets))
	for target := range busStaticTargets {
		seen[target] = struct{}{}
	}
	provider := busTargetsProvider
	busTargetsMu.RUnlock()

	if provider != nil {
		for _, raw := range provider() {
			normalized := normalizeBusTargetPath(raw)
			if normalized != "" {
				seen[normalized] = struct{}{}
			}
		}
	}

	if len(seen) == 0 {
		return nil
	}
	targets := make([]string, 0, len(seen))
	for target := range seen {
		targets = append(targets, target)
	}
	slices.Sort(targets)
	return targets
}

// ResetBusParticipantTargetsForTest clears the static target set and provider
// for the duration of a test and returns a restore function.
// MUST be paired with the returned restore, typically through t.Cleanup. Not
// safe for concurrent use with other tests that mutate targets.
func ResetBusParticipantTargetsForTest() (restore func()) {
	busTargetsMu.Lock()
	savedTargets := make(map[string]struct{}, len(busStaticTargets))
	for target := range busStaticTargets {
		savedTargets[target] = struct{}{}
	}
	savedProvider := busTargetsProvider
	busStaticTargets = map[string]struct{}{}
	busTargetsProvider = nil
	busTargetsMu.Unlock()

	return func() {
		busTargetsMu.Lock()
		busStaticTargets = savedTargets
		busTargetsProvider = savedProvider
		busTargetsMu.Unlock()
	}
}

// normalizeBusTargetPath returns "/a/b" for a non-empty slash-separated schema
// path (stripping any YANG module prefixes such as "/bgp:bgp/bgp:peer"), or ""
// if the input is blank or contains empty segments.
func normalizeBusTargetPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	for i, part := range parts {
		if part == "" {
			return ""
		}
		if _, local, hasPrefix := strings.Cut(part, ":"); hasPrefix {
			if local == "" {
				return ""
			}
			parts[i] = local
		}
	}
	return "/" + strings.Join(parts, "/")
}

// loadBusParticipants synthesizes the virtual ze-bus-participants module when
// ze-plugin-conf is loaded and at least one registered participant target path
// resolves against the loaded -conf modules. Targets whose owning module is not
// loaded in this build are skipped silently.
func (l *Loader) loadBusParticipants() error {
	if err := l.modules.refuseLoad(); err != nil {
		return err
	}
	if l.modules.Modules.Modules[busPluginConfModuleName] == nil {
		return nil
	}
	if l.modules.Modules.Modules[busExtensionsModuleName] == nil {
		return nil
	}
	targets := BusParticipantTargets()
	if len(targets) == 0 {
		return nil
	}

	index := buildASTConfigIndex(l.modules)
	resolved := make([][]resolvedTargetSeg, 0, len(targets))
	for _, target := range targets {
		chain, ok := index.resolveTarget(target)
		if !ok {
			continue
		}
		resolved = append(resolved, chain)
	}
	if len(resolved) == 0 {
		return nil
	}

	moduleText := renderBusParticipantsModule(resolved)
	return l.AddModuleFromText(busParticipantsModuleName+".yang", moduleText)
}

// resolvedTargetSeg is one segment of a resolved YANG augment target path.
type resolvedTargetSeg struct {
	module string
	name   string
}

// astSchemaNode is one container or list node discovered from parsed YANG ASTs
// before Modules.Process runs.
type astSchemaNode struct {
	module   string
	name     string
	children map[string]*astSchemaNode
}

// astConfigIndex holds the container/list tree rooted at loaded -conf modules,
// including nodes added via uses and top-level augment statements.
type astConfigIndex struct {
	roots []*astSchemaNode
}

// pendingAugment pairs a loaded module with one of its top-level augment blocks.
type pendingAugment struct {
	module  *gyang.Module
	augment *gyang.Augment
	applied bool
}

// buildASTConfigIndex builds a lightweight schema tree from already-parsed
// modules in m so registered participant paths can be mapped to defining
// module names before Modules.Process runs.
func buildASTConfigIndex(m *sourcedModules) *astConfigIndex {
	names := m.moduleNames()
	slices.Sort(names)

	byName := make(map[string]*gyang.Module, len(names))
	for _, name := range names {
		if mod := m.Modules.Modules[name]; mod != nil {
			byName[name] = mod
		}
	}

	idx := &astConfigIndex{}
	for _, name := range names {
		if !strings.HasSuffix(name, confModuleSuffix) {
			continue
		}
		mod := byName[name]
		if mod == nil {
			continue
		}
		topChildren := make(map[string]*astSchemaNode)
		collectASTChildren(mod, mod, byName, mod.Container, mod.List, mod.Uses, topChildren, nil, 0)
		for _, childName := range sortedASTChildNames(topChildren) {
			idx.roots = append(idx.roots, topChildren[childName])
		}
	}

	var pending []pendingAugment
	for _, name := range names {
		if name == busParticipantsModuleName {
			continue
		}
		mod := byName[name]
		if mod == nil {
			continue
		}
		for _, aug := range mod.Augment {
			pending = append(pending, pendingAugment{module: mod, augment: aug})
		}
	}

	// Fixed-point loop bounded by the number of pending augment statements so
	// chained augments (augment targeting a node added by another augment) all
	// resolve in deterministic order.
	for range len(pending) {
		progress := false
		for i := range pending {
			if pending[i].applied {
				continue
			}
			target := idx.findAugmentTarget(pending[i].module, pending[i].augment.Name)
			if target == nil {
				continue
			}
			collectASTChildren(
				pending[i].module,
				pending[i].module,
				byName,
				pending[i].augment.Container,
				pending[i].augment.List,
				pending[i].augment.Uses,
				target.children,
				nil,
				0,
			)
			pending[i].applied = true
			progress = true
		}
		if !progress {
			break
		}
	}

	return idx
}

// collectASTChildren populates dst with container and list nodes from the given
// slices and any referenced groupings. Recursion is bounded by maxASTWalkDepth
// and grouping cycles are prevented by visitedGroupings.
func collectASTChildren(
	ownerMod *gyang.Module,
	lookupMod *gyang.Module,
	byName map[string]*gyang.Module,
	containers []*gyang.Container,
	lists []*gyang.List,
	uses []*gyang.Uses,
	dst map[string]*astSchemaNode,
	visitedGroupings map[string]bool,
	depth int,
) {
	if depth >= maxASTWalkDepth {
		return
	}
	for _, c := range containers {
		if c.Config != nil && c.Config.Name == configFalseValue {
			continue
		}
		node := ensureASTChild(dst, ownerMod.Name, c.Name)
		collectASTChildren(ownerMod, lookupMod, byName, c.Container, c.List, c.Uses, node.children, visitedGroupings, depth+1)
	}
	for _, list := range lists {
		if list.Config != nil && list.Config.Name == configFalseValue {
			continue
		}
		node := ensureASTChild(dst, ownerMod.Name, list.Name)
		collectASTChildren(ownerMod, lookupMod, byName, list.Container, list.List, list.Uses, node.children, visitedGroupings, depth+1)
	}
	for _, u := range uses {
		grpMod, grp := resolveASTGrouping(lookupMod, byName, u.Name)
		if grp == nil {
			continue
		}
		visitKey := grpMod.Name + ":" + grp.Name
		if visitedGroupings[visitKey] {
			continue
		}
		nextVisited := make(map[string]bool, len(visitedGroupings)+1)
		for k, v := range visitedGroupings {
			nextVisited[k] = v
		}
		nextVisited[visitKey] = true
		collectASTChildren(ownerMod, grpMod, byName, grp.Container, grp.List, grp.Uses, dst, nextVisited, depth+1)
	}
}

// ensureASTChild returns an existing child named name in dst or creates one
// owned by moduleName.
func ensureASTChild(dst map[string]*astSchemaNode, moduleName, name string) *astSchemaNode {
	if existing, ok := dst[name]; ok {
		return existing
	}
	node := &astSchemaNode{
		module:   moduleName,
		name:     name,
		children: make(map[string]*astSchemaNode),
	}
	dst[name] = node
	return node
}

// resolveASTGrouping looks up a grouping name (unprefixed or prefix:name) from
// mod and its imports.
func resolveASTGrouping(mod *gyang.Module, byName map[string]*gyang.Module, ref string) (*gyang.Module, *gyang.Grouping) {
	targetMod := mod
	localName := ref
	if prefix, name, hasPrefix := strings.Cut(ref, ":"); hasPrefix {
		localName = name
		modName := resolvePrefixToModuleName(mod, prefix)
		if modName == "" {
			return nil, nil
		}
		targetMod = byName[modName]
		if targetMod == nil {
			return nil, nil
		}
	}
	for _, grp := range targetMod.Grouping {
		if grp.Name == localName {
			return targetMod, grp
		}
	}
	return nil, nil
}

// resolvePrefixToModuleName maps a prefix inside mod to the imported (or own)
// module name.
func resolvePrefixToModuleName(mod *gyang.Module, prefix string) string {
	if prefix == "" {
		return mod.Name
	}
	if mod.Prefix != nil && mod.Prefix.Name == prefix {
		return mod.Name
	}
	for _, imp := range mod.Import {
		if imp.Prefix != nil && imp.Prefix.Name == prefix {
			return imp.Name
		}
	}
	return ""
}

// findAugmentTarget locates the schema node addressed by an absolute YANG
// augment path (such as "/pfx:root/pfx:child") parsed in the context of augMod.
func (idx *astConfigIndex) findAugmentTarget(augMod *gyang.Module, augmentPath string) *astSchemaNode {
	if !strings.HasPrefix(augmentPath, "/") {
		return nil
	}
	rawParts := strings.Split(strings.TrimPrefix(augmentPath, "/"), "/")
	if len(rawParts) == 0 {
		return nil
	}

	firstMod, firstName := parseAugmentSegment(augMod, rawParts[0])
	if firstName == "" {
		return nil
	}

	for _, root := range idx.roots {
		if root.name != firstName {
			continue
		}
		if firstMod != "" && root.module != firstMod {
			continue
		}
		current := root
		matched := true
		for _, rawSeg := range rawParts[1:] {
			segMod, segName := parseAugmentSegment(augMod, rawSeg)
			if segName == "" {
				matched = false
				break
			}
			child := current.children[segName]
			if child == nil {
				matched = false
				break
			}
			if segMod != "" && child.module != segMod {
				matched = false
				break
			}
			current = child
		}
		if matched {
			return current
		}
	}
	return nil
}

// parseAugmentSegment splits one augment path segment into (moduleName, localName).
func parseAugmentSegment(augMod *gyang.Module, segment string) (moduleName, localName string) {
	if prefix, name, hasPrefix := strings.Cut(segment, ":"); hasPrefix {
		return resolvePrefixToModuleName(augMod, prefix), name
	}
	return augMod.Name, segment
}

// resolveTarget resolves an unprefixed participant path (such as "/alpha/item")
// against the indexed -conf schema tree and returns each segment paired with
// its defining module name. Returns false when no loaded module defines the
// path or when the target node already holds a child named "bus".
func (idx *astConfigIndex) resolveTarget(target string) ([]resolvedTargetSeg, bool) {
	parts := strings.Split(strings.TrimPrefix(target, "/"), "/")
	if len(parts) == 0 {
		return nil, false
	}

	for _, root := range idx.roots {
		if root.name != parts[0] {
			continue
		}
		chain := make([]resolvedTargetSeg, 0, len(parts))
		chain = append(chain, resolvedTargetSeg{module: root.module, name: root.name})
		current := root
		matched := true
		for _, seg := range parts[1:] {
			child := current.children[seg]
			if child == nil {
				matched = false
				break
			}
			chain = append(chain, resolvedTargetSeg{module: child.module, name: child.name})
			current = child
		}
		if !matched {
			continue
		}
		if _, hasBus := current.children[busContainerName]; hasBus {
			return nil, false
		}
		return chain, true
	}
	return nil, false
}

// sortedASTChildNames returns the sorted keys of children.
func sortedASTChildNames(children map[string]*astSchemaNode) []string {
	names := make([]string, 0, len(children))
	for name := range children {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// renderBusParticipantsModule emits deterministic YANG text for the virtual
// ze-bus-participants module augmenting every resolved target with container bus.
func renderBusParticipantsModule(targets [][]resolvedTargetSeg) string {
	extraSet := make(map[string]struct{})
	for _, chain := range targets {
		for _, seg := range chain {
			if seg.module == busPluginConfModuleName {
				continue
			}
			if seg.module == busExtensionsModuleName {
				continue
			}
			extraSet[seg.module] = struct{}{}
		}
	}
	extraMods := make([]string, 0, len(extraSet))
	for mod := range extraSet {
		extraMods = append(extraMods, mod)
	}
	slices.Sort(extraMods)

	prefixes := make(map[string]string, len(extraMods)+2)
	prefixes[busExtensionsModuleName] = "ze"
	prefixes[busPluginConfModuleName] = "plug"
	for i, mod := range extraMods {
		var pfxBuf textbuf.Buffer
		pfxBuf.Byte('m').Int(int64(i))
		prefixes[mod] = pfxBuf.String()
	}

	var tb textbuf.Buffer
	tb.Str("module ").Str(busParticipantsModuleName).Str(" {\n")
	tb.Str("    namespace \"urn:ze:bus:participants\";\n")
	tb.Str("    prefix buspart;\n\n")
	tb.Str("    import ").Str(busExtensionsModuleName).Str(" { prefix ze; }\n")
	tb.Str("    import ").Str(busPluginConfModuleName).Str(" { prefix plug; }\n")
	for _, mod := range extraMods {
		tb.Str("    import ").Str(mod).Str(" { prefix ").Str(prefixes[mod]).Str("; }\n")
	}
	tb.Str("\n    description \"Synthesized bus block augmentations for registered participant schema nodes.\";\n\n")
	tb.Str("    revision 2026-11-24 {\n")
	tb.Str("        description \"Synthesized from registered participant nodes.\";\n")
	tb.Str("    }\n")

	for _, chain := range targets {
		tb.Str("\n    augment \"")
		for _, seg := range chain {
			tb.Byte('/').Str(prefixes[seg.module]).Byte(':').Str(seg.name)
		}
		tb.Str("\" {\n")
		tb.Str("        container bus {\n")
		tb.Str("            presence \"Configure bus delivery rules for this participant.\";\n")
		tb.Str("            description \"Message bus delivery and filtering rules for this participant.\";\n")
		tb.Str("            ze:help \"Configure message bus delivery and filtering rules for this participant.\";\n")
		tb.Str("            uses plug:participant-bus;\n")
		tb.Str("        }\n")
		tb.Str("    }\n")
	}
	tb.Str("}\n")
	return tb.String()
}
