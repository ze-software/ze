// Design: docs/architecture/config/syntax.md — config migration

package migration

import (
	"errors"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/config"
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
	// Phase 3: Structural wrapping (must run after renames)
	{
		Name:        "wrap-bgp-block",
		Description: "Wrap BGP elements in bgp {} block",
		Detect:      hasBGPElementsAtRoot,
		Apply:       wrapBGPBlock,
	},
	{
		Name:        "template->new-format",
		Description: "Convert template.group/match to template.bgp.peer",
		Detect:      hasOldTemplateFormat,
		Apply:       migrateTemplateToNewFormat,
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
	// Check if peer, router-id, local-as, or listen exist at root level
	if len(tree.GetListOrdered("peer")) > 0 {
		return true
	}
	if _, ok := tree.Get("router-id"); ok {
		return true
	}
	if _, ok := tree.Get("local-as"); ok {
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

	// Move local-as
	if val, ok := result.Get("local-as"); ok {
		bgpContainer.Set("local-as", val)
		result.Delete("local-as")
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

// hasOldTemplateFormat returns true if template uses old group/match format.
func hasOldTemplateFormat(tree *config.Tree) bool {
	tmpl := tree.GetContainer("template")
	if tmpl == nil {
		return false
	}
	// Check for group or match lists (old format)
	if len(tmpl.GetListOrdered("group")) > 0 {
		return true
	}
	if len(tmpl.GetListOrdered("match")) > 0 {
		return true
	}
	return false
}

// migrateTemplateToNewFormat converts template.group/match to template.bgp.peer.
func migrateTemplateToNewFormat(tree *config.Tree) (*config.Tree, error) {
	result := tree.Clone()

	tmpl := result.GetContainer("template")
	if tmpl == nil {
		return result, nil
	}

	// Create template.bgp.peer structure
	bgpContainer := tmpl.GetOrCreateContainer("bgp")

	// Migrate group entries to peer with inherit-name
	for _, entry := range tmpl.GetListOrdered("group") {
		// For named groups, create peer * with inherit-name
		peerTree := entry.Value.Clone()
		peerTree.Set("inherit-name", entry.Key)
		bgpContainer.AddListEntry("peer", "*", peerTree)
		tmpl.RemoveListEntry("group", entry.Key)
	}

	// Migrate match entries to peer patterns
	for _, entry := range tmpl.GetListOrdered("match") {
		bgpContainer.AddListEntry("peer", entry.Key, entry.Value)
		tmpl.RemoveListEntry("match", entry.Key)
	}

	return result, nil
}
