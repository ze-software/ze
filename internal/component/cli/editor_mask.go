// Design: docs/architecture/ssh/fixit-bcrypt-hash-credential.md -- mask secret leaves on display
// Related: editor.go -- ContentAtPath / OriginalContentAtPath (unmasked; feed validation)

package cli

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// displayTreeOf parses raw config text and answers a tree whose secret leaves
// read as the display placeholder. It answers nil when nothing parses.
//
// The lenient retry is the one newEditor already uses. A config that carries a
// stale field reaches a masked tree, rather than falling back to its own text.
// Before that retry, one unknown field was enough to publish the file.
//
// A config that does not parse AT ALL still falls back to its raw text, in
// DisplayContentAtPath below. The schema resolves no leaf there, so there is
// nothing marked to mask, and the operator needs the offending line to repair
// the file. TestModelContextHighlighting (model_render_test.go) is that
// requirement.
//
// THAT TEXT CAN PUBLISH A SECRET. Editor.WorkingContent (editor.go) serializes
// the tree, commitContent (editor_commands.go) writes it to disk, and no
// serializer re-encodes $9$. A committed config therefore holds every
// ze:sensitive value in the clear. The fallback runs only when the strict parse
// AND the lenient retry both fail, which is the narrowest door left open.
func (e *Editor) displayTreeOf(raw string) *config.Tree {
	if e.schema == nil {
		return nil
	}
	tree, _, err := parseConfigWithFormat(raw, e.schema)
	if err != nil {
		if tree, _, err = parseConfigLenient(raw, e.schema); err != nil {
			return nil
		}
	}
	return config.MaskSecrets(tree, e.schema)
}

// serializeMasked renders an already masked tree at path. wholeOnMiss says what
// a path that does not resolve answers: the whole tree, or nothing.
func (e *Editor) serializeMasked(masked *config.Tree, path []string, wholeOnMiss bool) string {
	if masked == nil || e.schema == nil {
		return ""
	}
	if len(path) == 0 {
		return config.Serialize(masked, e.schema)
	}
	subtree, schemaNode := e.walkPathWithSchemaFrom(masked, path)
	if subtree == nil || schemaNode == nil {
		if wholeOnMiss {
			return config.Serialize(masked, e.schema)
		}
		return ""
	}
	return config.SerializeSubtree(subtree, schemaNode)
}

// displayTree answers the whole working tree with every secret leaf masked, and
// nil when nothing can be masked. It is the tree half of DisplayContentAtPath,
// and both read it, so the text form and the JSON form of `ze config show`
// answer the same configuration.
//
// It FAILS CLOSED. config.MaskSecrets answers its input unchanged when the
// schema is nil. A caller that hands it a nil schema therefore gets the live
// tree, with every secret in it. A tree that parses nowhere answers nil as
// well: a masked tree cannot stand in for text the schema resolves no leaf in.
func (e *Editor) displayTree() *config.Tree {
	if e.schema == nil {
		return nil
	}
	if e.treeValid && e.tree != nil {
		return config.MaskSecrets(e.tree, e.schema)
	}
	return e.displayTreeOf(e.workingContent)
}

// DisplayTreeAtPath answers the tree at path with every secret leaf masked. A
// caller that renders the TREE, rather than the text serializeMasked writes,
// reads this instead of Tree(). `ze config show --json` writes the map, and a
// map built from the working tree carries the value the parser decoded.
//
// An empty path answers the whole tree, which is what the no-path form of that
// command prints. A path that does not resolve answers nil, so the caller can
// report a miss rather than fall back to the whole tree. A configuration that
// parses nowhere answers nil too, and the caller reports THAT rather than the
// empty tree newEditor substitutes.
func (e *Editor) DisplayTreeAtPath(path []string) *config.Tree {
	masked := e.displayTree()
	if masked == nil || len(path) == 0 {
		return masked
	}
	subtree, _ := e.walkPathWithSchemaFrom(masked, path)
	return subtree
}

// secretChangeLine is the one wording every surface uses for a secret whose
// value moved. It names the leaf and publishes neither value. The config view
// writes it (secretChangeLines, model_render.go), the commit diff writes it
// (Editor.Diff, editor.go), and the web writes the same words
// (changedSecretLines, internal/component/web/cli_terminal.go).
func secretChangeLine(leafPath string) string {
	var b textbuf.Buffer
	return b.Str("~ ").Str(leafPath).Byte(' ').Str(config.SecretDataPlaceholder).Str(" (secret changed)").String()
}

// prunedContentAtPath is the display body both filtered views share: clone the
// working tree, drop what the filter removes, mask every secret leaf, serialize.
// The clone is private to this call, so the in-place mask is safe and the
// working tree keeps the real value for validation and persistence.
//
// An unparsed tree answers nothing rather than the raw working text, which is
// the rule displayTreeOf states above.
// unparsed is what an invalid tree answers, and the two views differ. The
// active view falls back to the working text, so a broken config stays
// repairable. The inactive view has nothing to show.
func (e *Editor) prunedContentAtPath(path []string, prune func(*config.Tree, *config.Schema), unparsed string) string {
	if !e.treeValid || e.tree == nil || e.schema == nil {
		return unparsed
	}
	clone := e.tree.Clone()
	prune(clone, e.schema)
	config.MaskSecretsInPlace(clone, e.schema)
	return e.serializeMasked(clone, path, true)
}

// changedSecretsAt names every secret leaf under path whose value differs
// between the committed config and the working tree. It publishes neither value.
//
// The display masks both sides to the same placeholder, so the text diff reads
// them as equal and reports nothing. That is a false statement about a rotated
// credential. This function reports the change, and it reports no value.
func (e *Editor) changedSecretsAt(path []string) []string {
	if !e.treeValid || e.tree == nil {
		return nil
	}
	return e.changedSecretsBetween(e.committedTree(), e.tree, path)
}

// committedTree parses the on-disk config, unmasked. Nil when it cannot parse.
func (e *Editor) committedTree() *config.Tree {
	if e.schema == nil {
		return nil
	}
	tree, _, err := parseConfigWithFormat(e.originalContent, e.schema)
	if err != nil {
		if tree, _, err = parseConfigLenient(e.originalContent, e.schema); err != nil {
			return nil
		}
	}
	return tree
}

// changedSecretsBetween compares two UNMASKED trees under path. Every caller
// holds the real values here: the mask runs later, on the text.
func (e *Editor) changedSecretsBetween(base, work *config.Tree, path []string) []string {
	if base == nil || work == nil || e.schema == nil {
		return nil
	}
	if len(path) == 0 {
		return config.ChangedSecretPaths(base, work, e.schema)
	}
	baseSub, node := e.walkPathWithSchemaFrom(base, path)
	workSub, _ := e.walkPathWithSchemaFrom(work, path)
	if baseSub == nil || workSub == nil || node == nil {
		return nil
	}
	return config.ChangedSecretPathsSubtree(baseSub, workSub, node)
}

// DisplayContentAtPath mirrors ContentAtPath but masks every secret leaf value
// for display. It MUST NOT be used on the validation or persistence path:
// ContentAtPath stays unmasked there so the real value is validated and written.
// Masking is value-only and line-preserving, so validation line numbers computed
// against the unmasked content still align with this masked view.
func (e *Editor) DisplayContentAtPath(path []string) string {
	if masked := e.displayTree(); masked != nil {
		return e.serializeMasked(masked, path, true)
	}
	return e.workingContent
}

// DisplayOriginalContentAtPath mirrors OriginalContentAtPath but masks every
// secret leaf value. Used alongside DisplayContentAtPath so the config-view diff
// gutter compares masked against masked. An unchanged secret is not flagged as
// changed, and changedSecretsAt (editor.go) is what reports one that moved.
func (e *Editor) DisplayOriginalContentAtPath(path []string) string {
	if masked := e.displayTreeOf(e.originalContent); masked != nil {
		return e.serializeMasked(masked, path, false)
	}
	if len(path) == 0 {
		return e.originalContent
	}
	return ""
}
