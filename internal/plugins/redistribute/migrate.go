// Design: docs/guide/redistribution.md -- import rules and their filter chain
// Related: register_migration.go -- registers the transformation and the retired leaves
// Related: config.go -- compiles the filters [ ... ]; final ...; this writes

package redistribute

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/policy"
)

// The import leaves the filter chain replaced. `ze config migrate` still parses
// them (config.MustRegisterRetiredLeaf) so migrateImportLeaves can rewrite them.
const (
	retiredFamilyLeaf = "family"
	retiredTagLeaf    = "tag"
)

// importPath is the schema path of an import rule, list keys left out.
var importPath = []string{configRoot, "destination", "import"}

var errMigratedFilterExists = errors.New("a policy match instance with the migrated name already exists")

// importLeavesTransformationName names the transformation in `ze config migrate`.
const importLeavesTransformationName = "redistribute-import-filters"

// hasImportToMigrate answers whether tree holds an import rule in the shape
// before the filter chain: one naming a retired family or tag leaf. A rule
// with neither filters nor final is a current shape, which imports every route
// of its source (the final leaf's YANG description, and importChain in the
// config loader), so it is not a migration's to rewrite.
func hasImportToMigrate(tree *config.Tree) bool {
	found := false
	eachImport(tree, func(_, _ string, rule *config.Tree) {
		if importNeedsMigration(rule) {
			found = true
		}
	})
	return found
}

// importNeedsMigration answers whether one import rule names a retired leaf.
func importNeedsMigration(rule *config.Tree) bool {
	if len(rule.GetMultiValues(retiredFamilyLeaf)) > 0 {
		return true
	}
	_, hasTag := rule.Get(retiredTagLeaf)
	return hasTag
}

// migrateImportLeaves rewrites every import rule's retired family and tag leaves
// into one match instance under the top-level policy root, which the rule then
// names first in its filters. The old leaves imported a route only when it met
// both, which is what one match instance with both conditions does, and
// `tag 0` stays `tag [ 0 ]`: an untagged redistributed route carries tag 0
// (redistRoute). A rewritten rule without a final gets final accept, the
// decision the old shape made for a route its leaves let through, because the
// loader refuses filters with no final. A rule naming no retired leaf is left
// as written. The tree is modified in place; on error the caller MUST discard
// it.
func migrateImportLeaves(tree *config.Tree) (*config.Tree, error) {
	var failed error
	eachImport(tree, func(destination, source string, rule *config.Tree) {
		if failed != nil {
			return
		}
		failed = migrateImportRule(tree, destination, source, rule)
	})
	if failed != nil {
		return nil, failed
	}
	return tree, nil
}

// migrateImportRule rewrites one import rule; see migrateImportLeaves.
func migrateImportRule(tree *config.Tree, destination, source string, rule *config.Tree) error {
	// refuse-by-default: guards migrateImportLeaves -- a rule naming no retired leaf is a current shape, left as written
	if !importNeedsMigration(rule) {
		return nil
	}
	families := rule.GetMultiValues(retiredFamilyLeaf)
	tag, hasTag := rule.Get(retiredTagLeaf)
	name := "redistribute-" + destination + "-" + source
	matches := tree.GetOrCreateContainer(policy.RootName)
	if _, exists := matches.GetList("match")[name]; exists {
		return fmt.Errorf("redistribute destination %s import %s: %w: rename policy match %s and run ze config migrate again",
			destination, source, errMigratedFilterExists, name)
	}
	match := config.NewTree()
	if len(families) > 0 {
		match.SetSlice(retiredFamilyLeaf, families)
	}
	if hasTag {
		match.SetSlice(retiredTagLeaf, []string{tag})
	}
	matches.AddListEntry("match", name, match)
	rule.Delete(retiredFamilyLeaf)
	rule.Delete(retiredTagLeaf)
	rule.SetSlice("filters", slices.Concat([]string{name}, rule.GetMultiValues("filters")))
	if _, hasFinal := rule.Get("final"); !hasFinal {
		rule.Set("final", "accept")
	}
	return nil
}

// eachImport calls visit for every import rule under the redistribute root, in
// file order.
func eachImport(tree *config.Tree, visit func(destination, source string, rule *config.Tree)) {
	root := tree.GetContainer(configRoot)
	if root == nil {
		return
	}
	for _, dest := range root.GetListOrdered("destination") {
		for _, imp := range dest.Value.GetListOrdered("import") {
			visit(dest.Key, imp.Key, imp.Value)
		}
	}
}
