// Design: docs/architecture/web-interface.md -- the committed editor baseline.
// Related: editor.go -- construction retains known fields for inspection.
// Related: editor_resume.go -- recovery uses the same committed parse policy.

package cli

import "github.com/ze-software/ze/internal/component/config"

// parseEditingConfig preserves the committed-view policy shared by editor
// construction and recovery. A stale set field may be omitted for inspection;
// this does not validate that source for publication. Pending intent uses its
// own parsers, and commit keeps the strict committed-source reader.
func parseEditingConfig(content string, schema *config.Schema) (*config.Tree, *config.MetaTree, error) {
	tree, meta, err := parseConfigWithFormat(content, schema)
	if err == nil {
		return tree, meta, nil
	}
	return parseConfigLenient(content, schema)
}
