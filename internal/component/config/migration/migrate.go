// Design: docs/architecture/config/syntax.md — config migration
// Detail: listener.go — listener normalization transformations

package migration

import (
	"errors"
	"fmt"
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// ErrNilTree is returned when a nil tree is passed to a function that requires a tree.
var ErrNilTree = errors.New("nil tree")

// Transformation defines a single migration step.
type Transformation struct {
	Name        string                                   // Semantic name for display
	Description string                                   // Human-readable explanation
	Detect      func(*config.Tree) bool                  // Does this issue exist?
	Apply       func(*config.Tree) (*config.Tree, error) // Fix it
}

// MigrateResult holds the outcome of migration.
type MigrateResult struct {
	Tree    *config.Tree // Transformed tree (only set on full success)
	Applied []string     // Transformations that ran
	Skipped []string     // Transformations not needed
}

// DryRunResult shows what would happen without applying changes.
type DryRunResult struct {
	AlreadyDone  []string // Detect returned false - already migrated
	WouldApply   []string // Detect returned true - would be applied
	WouldSucceed bool     // All transformations would succeed
	FailedAt     string   // Which transformation would fail (if any)
	Error        error    // Why it would fail
}

// transformations in dependency order (unexported).
// Phase 1: Structural renames (must run first - create peer/group blocks)
// Phase 2: Content transforms (operate on blocks created by phase 1).
var transformations = []Transformation{
	// Phase 1: Structural renames
	{
		Name:        "neighbor->peer",
		Description: "Rename 'neighbor' blocks to 'peer'",
		Detect:      hasNeighborAtRoot,
		Apply:       migrateNeighborToPeer,
	},
	{
		Name:        "peer-glob->template.match",
		Description: "Move glob patterns (10.0.0.0/8) to template.match",
		Detect:      hasPeerGlobPattern,
		Apply:       migratePeerGlobToMatch,
	},
	{
		Name:        "template.neighbor->group",
		Description: "Rename template.neighbor to template.group",
		Detect:      hasTemplateNeighbor,
		Apply:       migrateTemplateNeighborToGroup,
	},
	// Phase 2: Content transforms
	{
		Name:        "static->announce",
		Description: "Convert 'static' route blocks to 'announce'",
		Detect:      hasStaticBlocks,
		Apply:       extractStaticRoutes,
	},
	{
		Name:        "api->new-format",
		Description: "Convert old api syntax (processes, format flags) to named blocks",
		Detect:      hasOldAPIBlocks,
		Apply:       migrateAPIBlocks,
	},
	// Phase 2b: Listener normalization (remove ExaBGP legacy leaves)
	{
		Name:        "remove-bgp-listen",
		Description: "Remove ExaBGP legacy bgp listen leaf",
		Detect:      hasBGPListenLeaf,
		Apply:       removeBGPListenLeaf,
	},
	{
		Name:        "remove-tcp-port",
		Description: "Remove environment tcp port (per-peer config)",
		Detect:      hasTCPPortLeaf,
		Apply:       removeTCPPortLeaf,
	},
	{
		Name:        "remove-env-bgp-connect",
		Description: "Remove environment bgp connect (per-peer config)",
		Detect:      hasEnvBGPConnect,
		Apply:       removeEnvBGPConnect,
	},
	{
		Name:        "remove-env-bgp-accept",
		Description: "Remove environment bgp accept (per-peer config)",
		Detect:      hasEnvBGPAccept,
		Apply:       removeEnvBGPAccept,
	},
	{
		Name:        "hub-server-host-to-ip",
		Description: "Rename plugin hub server host to ip",
		Detect:      hasHubServerHost,
		Apply:       renameHubServerHost,
	},
	// Phase 3: Structural wrapping (must run after renames)
	{
		Name:        "wrap-bgp-block",
		Description: "Wrap BGP elements in bgp {} block",
		Detect:      hasBGPElementsAtRoot,
		Apply:       wrapBGPBlock,
	},
	{
		Name:        "template->group",
		Description: "Convert template block to bgp peer-groups and move peers into groups",
		Detect:      hasTemplateBlock,
		Apply:       migrateTemplateToGroups,
	},
}

// Migrate applies all needed transformations.
// Changes are in-memory until ALL succeed; on failure, original unchanged.
func Migrate(tree *config.Tree) (*MigrateResult, error) {
	if tree == nil {
		return nil, ErrNilTree
	}

	working := tree.Clone() // Work on clone - original unchanged until success
	result := &MigrateResult{}

	for _, t := range transformations {
		if t.Detect(working) {
			migrated, err := t.Apply(working)
			if err != nil {
				// Failure: return error, original tree unchanged
				return nil, fmt.Errorf("%s: %w", t.Name, err)
			}
			working = migrated
			result.Applied = append(result.Applied, t.Name)
		} else {
			result.Skipped = append(result.Skipped, t.Name)
		}
	}

	// All succeeded: return transformed tree
	result.Tree = working
	return result, nil
}

// DryRun analyzes what would happen without applying changes.
// Validates transformations would succeed by running on a clone.
func DryRun(tree *config.Tree) (*DryRunResult, error) {
	if tree == nil {
		return nil, ErrNilTree
	}

	result := &DryRunResult{WouldSucceed: true}
	working := tree.Clone()

	for _, t := range transformations {
		if t.Detect(working) {
			result.WouldApply = append(result.WouldApply, t.Name)
			// Run on clone to validate it would succeed
			migrated, err := t.Apply(working)
			if err != nil {
				result.WouldSucceed = false
				result.FailedAt = t.Name
				result.Error = err
				return result, nil //nolint:nilerr // Return analysis in result, not as error
			}
			working = migrated
		} else {
			result.AlreadyDone = append(result.AlreadyDone, t.Name)
		}
	}

	return result, nil
}

// NeedsMigration returns true if any transformation would apply.
func NeedsMigration(tree *config.Tree) bool {
	if tree == nil {
		return false
	}
	for _, t := range transformations {
		if t.Detect(tree) {
			return true
		}
	}
	return false
}

// ListTransformations returns all available transformations with their descriptions.
// Returns a copy to prevent external modification.
func ListTransformations() []Transformation {
	result := make([]Transformation, len(transformations))
	copy(result, transformations)
	return result
}

// --- Apply functions ---

// migrateNeighborToPeer renames neighbor entries to peer.
func migrateNeighborToPeer(tree *config.Tree) (*config.Tree, error) {
	result := tree.Clone()

	neighborEntries := result.GetListOrdered("neighbor")
	for _, entry := range neighborEntries {
		result.RemoveListEntry("neighbor", entry.Key)
		result.AddListEntry("peer", entry.Key, entry.Value)
	}

	return result, nil
}

// migratePeerGlobToMatch moves peer glob patterns to template.match.
func migratePeerGlobToMatch(tree *config.Tree) (*config.Tree, error) {
	result := tree.Clone()

	peerGlobs := collectPeerGlobs(result)
	for _, entry := range peerGlobs {
		result.RemoveListEntry("peer", entry.Key)

		tmpl := result.GetOrCreateContainer("template")
		tmpl.AddListEntry("match", entry.Key, entry.Value)
	}

	return result, nil
}

// migrateTemplateNeighborToGroup renames template.neighbor to template.group.
func migrateTemplateNeighborToGroup(tree *config.Tree) (*config.Tree, error) {
	result := tree.Clone()

	if tmpl := result.GetContainer("template"); tmpl != nil {
		templateNeighbors := tmpl.GetListOrdered("neighbor")
		for _, entry := range templateNeighbors {
			tmpl.RemoveListEntry("neighbor", entry.Key)
			tmpl.AddListEntry("group", entry.Key, entry.Value)
		}
	}

	return result, nil
}

// extractStaticRoutes transforms static blocks to announce blocks.
func extractStaticRoutes(tree *config.Tree) (*config.Tree, error) {
	return ExtractStaticRoutes(tree)
}

// migrateAPIBlocks transforms old api syntax to new named syntax.
func migrateAPIBlocks(tree *config.Tree) (*config.Tree, error) {
	return MigrateAPIBlocks(tree)
}

// collectPeerGlobs returns peer entries that are glob patterns (contain * or /).
func collectPeerGlobs(tree *config.Tree) []struct {
	Key   string
	Value *config.Tree
} {
	var globs []struct {
		Key   string
		Value *config.Tree
	}
	for _, entry := range tree.GetListOrdered("peer") {
		if isGlobPattern(entry.Key) {
			globs = append(globs, entry)
		}
	}
	return globs
}

// hasBGPElementsAtRoot returns true if BGP elements are at root level (not wrapped in bgp {}).
func hasBGPElementsAtRoot(tree *config.Tree) bool {
	// Check if peer, router-id, local-as, local container, or listen exist at root level.
	if len(tree.GetListOrdered("peer")) > 0 {
		return true
	}
	if _, ok := tree.Get("router-id"); ok {
		return true
	}
	if _, ok := tree.Get("local-as"); ok {
		return true
	}
	if tree.GetContainer("local") != nil {
		return true
	}
	if _, ok := tree.Get("listen"); ok {
		return true
	}
	return false
}

// wrapBGPBlock wraps BGP elements in a bgp {} block.
func wrapBGPBlock(tree *config.Tree) (*config.Tree, error) {
	result := tree.Clone()

	// Create bgp container
	bgpContainer := config.NewTree()

	// Move router-id
	if val, ok := result.Get("router-id"); ok {
		bgpContainer.Set("router-id", val)
		result.Delete("router-id")
	}

	// Move local-as into local { as ... } container.
	if val, ok := result.Get("local-as"); ok {
		localContainer := bgpContainer.GetContainer("local")
		if localContainer == nil {
			localContainer = config.NewTree()
		}
		localContainer.Set("as", val)
		bgpContainer.SetContainer("local", localContainer)
		result.Delete("local-as")
	}

	// Move existing local container (if source already uses new format).
	if localContainer := result.GetContainer("local"); localContainer != nil {
		existing := bgpContainer.GetContainer("local")
		if existing == nil {
			bgpContainer.SetContainer("local", localContainer)
		}
		result.RemoveContainer("local")
	}

	// Move listen
	if val, ok := result.Get("listen"); ok {
		bgpContainer.Set("listen", val)
		result.Delete("listen")
	}

	// Move peer entries
	for _, entry := range result.GetListOrdered("peer") {
		bgpContainer.AddListEntry("peer", entry.Key, entry.Value)
		result.RemoveListEntry("peer", entry.Key)
	}

	// Set bgp container if it has content
	if len(bgpContainer.Values()) > 0 || len(bgpContainer.ListKeys("peer")) > 0 {
		result.SetContainer("bgp", bgpContainer)
	}

	return result, nil
}

// migrateTemplateToGroups converts template block to bgp peer-groups.
// It handles both old format (template.group/match) and new format (template.bgp.peer).
// Steps:
//  1. Extract named templates and glob patterns from template block
//  2. Create bgp.group entries from templates
//  3. Move peers with "inherit X" into the appropriate group
//  4. Assign ungrouped peers to a "default" group
//  5. Delete the template block
func migrateTemplateToGroups(tree *config.Tree) (*config.Tree, error) {
	result := tree.Clone()

	tmpl := result.GetContainer("template")
	if tmpl == nil {
		return result, nil
	}

	bgp := result.GetOrCreateContainer("bgp")

	// Collect named templates and glob patterns from template block.
	namedTemplates := make(map[string]*config.Tree) // name -> template tree

	// Old format: template { group <name> { ... } }
	for _, entry := range tmpl.GetListOrdered("group") {
		namedTemplates[entry.Key] = entry.Value.Clone()
	}

	// Old format: template { match <pattern> { ... } }
	for _, entry := range tmpl.GetListOrdered("match") {
		groupName := "match-" + sanitizeGroupName(entry.Key)
		namedTemplates[groupName] = entry.Value.Clone()
	}

	// New format: template { bgp { peer <pattern> { inherit-name <name>; ... } } }
	if bgpTmpl := tmpl.GetContainer("bgp"); bgpTmpl != nil {
		for _, entry := range bgpTmpl.GetListOrdered("peer") {
			peerTree := entry.Value.Clone()
			if inheritName, hasName := peerTree.Get("inherit-name"); hasName {
				peerTree.Delete("inherit-name")
				namedTemplates[inheritName] = peerTree
			} else {
				// Unnamed glob pattern.
				groupName := "match-" + sanitizeGroupName(entry.Key)
				namedTemplates[groupName] = peerTree
			}
		}
	}

	// Create bgp.group entries from templates (sorted for deterministic output).
	sortedNames := make([]string, 0, len(namedTemplates))
	for name := range namedTemplates {
		sortedNames = append(sortedNames, name)
	}
	sort.Strings(sortedNames)
	for _, name := range sortedNames {
		bgp.AddListEntry("group", name, namedTemplates[name])
	}

	// Move peers with "inherit X" into the appropriate group.
	for _, peerEntry := range bgp.GetListOrdered("peer") {
		addr := peerEntry.Key
		peerTree := peerEntry.Value

		inheritName := ""
		// Check single inherit value.
		if v, ok := peerTree.Get("inherit"); ok {
			inheritName = v
		}
		// Check ordered inherit list.
		if inheritName == "" {
			for _, inheritEntry := range peerTree.GetListOrdered("inherit") {
				inheritName = inheritEntry.Key
				break // Use first inherit entry.
			}
		}

		if inheritName == "" {
			continue // Peer without inherit -- will be handled below.
		}

		// Remove inherit directive from peer.
		peerTree.Delete("inherit")
		peerTree.RemoveListEntry("inherit", inheritName)

		// Find or create the group.
		groupTree := findGroupTree(bgp, inheritName)
		if groupTree == nil {
			// Create group for unmatched inherit reference.
			groupTree = config.NewTree()
			bgp.AddListEntry("group", inheritName, groupTree)
		}

		// Move peer into the group.
		groupTree.AddListEntry("peer", addr, peerTree)
		bgp.RemoveListEntry("peer", addr)
	}

	// Move remaining ungrouped peers into a "default" group.
	remainingPeers := bgp.GetListOrdered("peer")
	if len(remainingPeers) > 0 {
		defaultGroup := findGroupTree(bgp, "default")
		if defaultGroup == nil {
			defaultGroup = config.NewTree()
			bgp.AddListEntry("group", "default", defaultGroup)
		}
		for _, peerEntry := range remainingPeers {
			defaultGroup.AddListEntry("peer", peerEntry.Key, peerEntry.Value)
			bgp.RemoveListEntry("peer", peerEntry.Key)
		}
	}

	// Delete the template block.
	result.RemoveContainer("template")

	return result, nil
}

// findGroupTree returns the tree for a named group in bgp, or nil if not found.
func findGroupTree(bgp *config.Tree, name string) *config.Tree {
	for _, entry := range bgp.GetListOrdered("group") {
		if entry.Key == name {
			return entry.Value
		}
	}
	return nil
}

// sanitizeGroupName converts a glob pattern to a valid group name.
// Replaces special characters with hyphens.
func sanitizeGroupName(pattern string) string {
	var b []byte
	for _, ch := range pattern {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
			b = append(b, byte(ch))
		} else {
			b = append(b, '-')
		}
	}
	return string(b)
}
