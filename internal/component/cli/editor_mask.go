// Design: plan/learned/1181-fixit-bcrypt-hash-credential.md -- mask ze:bcrypt leaves on display
// Related: editor.go -- ContentAtPath / OriginalContentAtPath (unmasked; feed validation)

package cli

import "github.com/ze-software/ze/internal/component/config"

// DisplayContentAtPath mirrors ContentAtPath but masks ze:bcrypt leaf values for
// display. It MUST NOT be used on the validation or persistence path:
// ContentAtPath stays unmasked there so the real hash is validated and written.
// Masking is value-only and line-preserving, so validation line numbers computed
// against the unmasked content still align with this masked view.
func (e *Editor) DisplayContentAtPath(path []string) string {
	if !e.treeValid || e.tree == nil || e.schema == nil {
		return e.workingContent
	}
	masked := config.MaskBcrypt(e.tree, e.schema)
	if len(path) == 0 {
		return config.Serialize(masked, e.schema)
	}
	subtree, schemaNode := e.walkPathWithSchemaFrom(masked, path)
	if subtree == nil || schemaNode == nil {
		return config.Serialize(masked, e.schema)
	}
	return config.SerializeSubtree(subtree, schemaNode)
}

// DisplayOriginalContentAtPath mirrors OriginalContentAtPath but masks ze:bcrypt
// leaf values. Used alongside DisplayContentAtPath so the config-view diff gutter
// compares masked-against-masked (an unchanged hash is not flagged as changed).
func (e *Editor) DisplayOriginalContentAtPath(path []string) string {
	if e.schema == nil {
		if len(path) == 0 {
			return e.originalContent
		}
		return ""
	}
	origTree, _, err := parseConfigWithFormat(e.originalContent, e.schema)
	if err != nil {
		return e.originalContent
	}
	masked := config.MaskBcrypt(origTree, e.schema)
	if len(path) == 0 {
		return config.Serialize(masked, e.schema)
	}
	subtree, schemaNode := e.walkPathWithSchemaFrom(masked, path)
	if subtree == nil || schemaNode == nil {
		return ""
	}
	return config.SerializeSubtree(subtree, schemaNode)
}
