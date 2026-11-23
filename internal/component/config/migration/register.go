// Design: docs/architecture/config/syntax.md -- migration function registration

package migration

import "github.com/ze-software/ze/internal/component/config"

func init() {
	config.RegisterMigrateFunc(func(tree *config.Tree) (*config.Tree, []string, error) {
		if !NeedsMigration(tree) {
			return tree, nil, nil
		}
		result, err := Migrate(tree)
		if err != nil {
			return nil, nil, err
		}
		return result.Tree, result.Applied, nil
	})
}
