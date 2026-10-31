// Design: docs/guide/redistribution.md -- migrating the retired import leaves
// Related: migrate.go -- the transformation registered here

package redistribute

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/migration"
)

func init() {
	const replacement = "a policy match instance (match NAME { family [ ... ]; tag [ N ]; }) and name it in filters [ NAME ]; final accept;"
	config.MustRegisterRetiredLeaf(importPath, retiredFamilyLeaf, &config.ValueOrArrayNode{Type: config.TypeString}, replacement)
	config.MustRegisterRetiredLeaf(importPath, retiredTagLeaf, config.Leaf(config.TypeUint32), replacement)
	migration.MustRegisterTransformation(migration.Transformation{
		Name:        importLeavesTransformationName,
		Description: "Rewrite redistribute import family and tag into a policy match filter named in filters, with final accept",
		Detect:      hasImportToMigrate,
		Apply:       migrateImportLeaves,
	})
}
