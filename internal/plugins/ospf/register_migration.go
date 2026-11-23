// Design: docs/guide/redistribution.md -- migrating the retired ospf redistribute leaves
// Related: migrate.go -- the transformation registered here

package ospf

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/migration"
)

func init() {
	const replacement = "a policy modify instance (modify NAME { match { protocol [ SOURCE ]; } set { metric N; metric-type type-1; tag N; } }) named in ospf filter export { filters [ NAME ]; final accept; }"
	config.MustRegisterRetiredLeaf(redistributePath, retiredMetricLeaf, config.Leaf(config.TypeUint32), replacement)
	config.MustRegisterRetiredLeaf(redistributePath, retiredMetricTypeLeaf, config.Leaf(config.TypeString), replacement)
	config.MustRegisterRetiredLeaf(redistributePath, retiredTagLeaf, config.Leaf(config.TypeUint32), replacement)
	migration.MustRegisterTransformation(migration.Transformation{
		Name:        redistributeLeavesTransformationName,
		Description: "Rewrite ospf redistribute metric, metric-type and tag into policy modify filters named in ospf filter export",
		Detect:      hasRedistributeToMigrate,
		Apply:       migrateRedistributeLeaves,
	})
}
