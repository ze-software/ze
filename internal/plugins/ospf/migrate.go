// Design: docs/guide/redistribution.md -- migrating the retired ospf redistribute leaves
// Related: register_migration.go -- registers the transformation and the retired leaves
// Related: export_chain.go -- compiles the ospf filter export chain this writes

package ospf

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/policy"
)

// The `ospf redistribute <source>` leaves the export chain replaced. `ze config
// migrate` still parses them (config.MustRegisterRetiredLeaf) so
// migrateRedistributeLeaves can rewrite them.
const (
	retiredMetricLeaf     = "metric"
	retiredMetricTypeLeaf = "metric-type"
	retiredTagLeaf        = "tag"
)

// redistributePath is the schema path of an `ospf redistribute` entry, list
// keys left out.
var redistributePath = []string{Namespace, "redistribute"}

var errMigratedModifyExists = errors.New("a policy modify instance with the migrated name already exists")

// redistributeLeavesTransformationName names the transformation in `ze config migrate`.
const redistributeLeavesTransformationName = "ospf-redistribute-export-filter"

// hasRedistributeToMigrate answers whether tree holds an `ospf redistribute`
// entry naming a retired metric, metric-type or tag leaf.
func hasRedistributeToMigrate(tree *config.Tree) bool {
	found := false
	eachRedistribute(tree, func(_ string, entry *config.Tree) {
		if redistributeNeedsMigration(entry) {
			found = true
		}
	})
	return found
}

// redistributeNeedsMigration answers whether one entry names a retired leaf.
func redistributeNeedsMigration(entry *config.Tree) bool {
	for _, leaf := range []string{retiredMetricLeaf, retiredMetricTypeLeaf, retiredTagLeaf} {
		if _, ok := entry.Get(leaf); ok {
			return true
		}
	}
	return false
}

// migrateRedistributeLeaves rewrites every `ospf redistribute <source>` entry's
// retired leaves into policy modify instances that `ospf filter export` names,
// ahead of any filter it already names. The old metric and metric-type applied
// to every route of the source: one modify matching `protocol [ <source> ]`
// sets them. The old tag applied only to a route carrying none (the route's
// own tag won, and an untagged route carries tag 0): a second modify matching `protocol [ <source> ]; tag [ 0 ]`
// sets it, and a tag of 0 needs none. An export chain without a final gets
// final accept, so every route the old entries originated is still
// originated. The tree is modified in place; on error the caller MUST discard
// it.
func migrateRedistributeLeaves(tree *config.Tree) (*config.Tree, error) {
	var written []string
	var failed error
	eachRedistribute(tree, func(source string, entry *config.Tree) {
		if failed != nil {
			return
		}
		var names []string
		names, failed = migrateRedistributeEntry(tree, source, entry)
		written = append(written, names...)
	})
	if failed != nil {
		return nil, failed
	}
	if len(written) == 0 {
		return tree, nil
	}
	export := tree.GetContainer(Namespace).GetOrCreateContainer("filter").GetOrCreateContainer("export")
	export.SetSlice("filters", slices.Concat(written, export.GetMultiValues("filters")))
	if _, hasFinal := export.Get("final"); !hasFinal {
		export.Set("final", "accept")
	}
	return tree, nil
}

// migrateRedistributeEntry rewrites one entry and answers the modify instances
// it wrote; see migrateRedistributeLeaves.
func migrateRedistributeEntry(tree *config.Tree, source string, entry *config.Tree) ([]string, error) {
	metric, hasMetric := entry.Get(retiredMetricLeaf)
	metricType, hasMetricType := entry.Get(retiredMetricTypeLeaf)
	tag, hasTag := entry.Get(retiredTagLeaf)
	entry.Delete(retiredMetricLeaf)
	entry.Delete(retiredMetricTypeLeaf)
	entry.Delete(retiredTagLeaf)

	var names []string
	if hasMetric || hasMetricType {
		set := config.NewTree()
		if hasMetric {
			set.Set(retiredMetricLeaf, metric)
		}
		if hasMetricType {
			set.Set(retiredMetricTypeLeaf, metricType)
		}
		name := "ospf-export-" + source
		if err := addMigratedModify(tree, name, source, false, set); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	if hasTag && tag != "0" {
		set := config.NewTree()
		set.Set(retiredTagLeaf, tag)
		name := "ospf-export-" + source + "-tag"
		if err := addMigratedModify(tree, name, source, true, set); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

// addMigratedModify writes `policy modify <name> { match { protocol [ source ];
// tag [ 0 ] when untagged } set {...} }`, refusing a name already in use.
func addMigratedModify(tree *config.Tree, name, source string, untagged bool, set *config.Tree) error {
	modifies := tree.GetOrCreateContainer(policy.RootName)
	if _, exists := modifies.GetList("modify")[name]; exists {
		return fmt.Errorf("ospf redistribute %s: %w: rename policy modify %s and run ze config migrate again",
			source, errMigratedModifyExists, name)
	}
	modify := config.NewTree()
	match := modify.GetOrCreateContainer("match")
	match.SetSlice("protocol", []string{source})
	if untagged {
		match.SetSlice(retiredTagLeaf, []string{"0"})
	}
	modify.SetContainer("set", set)
	modifies.AddListEntry("modify", name, modify)
	return nil
}

// eachRedistribute calls visit for every `ospf redistribute` entry, in file order.
func eachRedistribute(tree *config.Tree, visit func(source string, entry *config.Tree)) {
	root := tree.GetContainer(Namespace)
	if root == nil {
		return
	}
	for _, entry := range root.GetListOrdered("redistribute") {
		visit(entry.Key, entry.Value)
	}
}
