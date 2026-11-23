// Design: docs/architecture/config/syntax.md — config migration
// Detail: registered.go — transformations a feature package registers

package migration

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/config"
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

// relocatedBlocksName names the built-in step that drives the relocated-container
// registry.
const relocatedBlocksName = "relocated-blocks"

// transformations are the built-in steps, which run before every registered one
// (unexported). The only one drives the relocated-container registry: it moves
// each block a package registered with config.MustRegisterRelocatedContainer to
// its top-level root, and converts no format change of its own. The conversion
// of one format change is registered by the package that owns the changed shape
// (MustRegisterTransformation), and is written only once Ze has a stable release.
var transformations = []Transformation{
	{
		Name:        relocatedBlocksName,
		Description: "Move each block registered as relocated to its top-level root",
		Detect:      config.HasRelocatedContainers,
		Apply:       config.ApplyRelocations,
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

	for _, t := range allTransformations() {
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

	for _, t := range allTransformations() {
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
	for _, t := range allTransformations() {
		if t.Detect(tree) {
			return true
		}
	}
	return false
}

// ListTransformations returns all available transformations with their descriptions.
// Returns a copy to prevent external modification.
func ListTransformations() []Transformation {
	return allTransformations()
}
