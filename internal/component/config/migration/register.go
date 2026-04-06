// Design: docs/architecture/config/syntax.md -- migration function registration

package migration

import "codeberg.org/thomas-mangin/ze/internal/component/config"

func init() {
	config.RegisterMigrateFunc(func(tree *config.Tree) ([]string, error) {
		result, err := Migrate(tree)
		if err != nil {
			return nil, err
		}
		return result.Applied, nil
	})
}
