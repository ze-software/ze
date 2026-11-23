// Design: docs/architecture/config/syntax.md -- a leaf a schema change retired
// Related: relocated.go -- MigrationSchema, which grafts the retired leaves back
// Related: parser.go -- hierarchical parse, unknown-field refusal
// Related: parser_list.go -- list entry parse, unknown-field refusal
// Related: setparser.go -- set-format parse, unknown-field refusal

package config

import (
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// retiredLeaf is a leaf the schema no longer holds at path, whose meaning a
// registered `ze config migrate` transformation rewrites into the current shape.
// The current schema refuses it, naming replacement and the migrate command;
// MigrationSchema accepts it with node, so the migration can read the old value.
type retiredLeaf struct {
	path        []string
	name        string
	node        Node
	replacement string
}

var (
	retiredLeavesMu sync.RWMutex
	retiredLeaves   []retiredLeaf
)

// MustRegisterRetiredLeaf records that the leaf name under the schema node at
// path (schema names only, no list keys: "redistribute", "destination",
// "import") was retired. node is the leaf's old shape, which MigrationSchema
// parses; replacement says in a phrase what the operator writes now. The package
// that owns the transformation rewriting the leaf calls it from init(). A second
// registration of the same leaf is a Ze defect and panics.
func MustRegisterRetiredLeaf(path []string, name string, node Node, replacement string) {
	retiredLeavesMu.Lock()
	defer retiredLeavesMu.Unlock()
	for _, r := range retiredLeaves {
		if r.name == name && slices.Equal(r.path, path) {
			panic("BUG: config: retired leaf " + strings.Join(path, " ") + " " + name + " registered twice")
		}
	}
	retiredLeaves = append(retiredLeaves, retiredLeaf{path: slices.Clone(path), name: name, node: node, replacement: replacement})
}

// registeredRetiredLeaves answers a copy of the registered retired leaves.
func registeredRetiredLeaves() []retiredLeaf {
	retiredLeavesMu.RLock()
	defer retiredLeavesMu.RUnlock()
	return slices.Clone(retiredLeaves)
}

// markRetiredLeaves records each retired leaf on the schema node that held it,
// so the parsers refuse it with the migration hint instead of the closest-key
// one. A parent the build does not carry has no leaf to refuse.
func markRetiredLeaves(schema *Schema) {
	for _, r := range registeredRetiredLeaves() {
		switch parent := schema.LookupTokenPath(r.path).(type) {
		case *ContainerNode:
			parent.retired = withRetired(parent.retired, r)
		case *ListNode:
			parent.retired = withRetired(parent.retired, r)
		}
	}
}

// withRetired answers marks with r's hint added under its name.
func withRetired(marks map[string]string, r retiredLeaf) map[string]string {
	if marks == nil {
		marks = make(map[string]string)
	}
	marks[r.name] = retiredLeafHint(r)
	return marks
}

// retiredLeafHint is the text an unknown-field refusal carries for r.
func retiredLeafHint(r retiredLeaf) string {
	var b textbuf.Buffer
	return b.Str("\n  ").Str(r.name).Str(" is no longer a leaf of ").Str(strings.Join(r.path, " ")).
		Str(": write ").Str(r.replacement).Str(" instead\n").
		Str("  fix: ze config migrate -o <new-file> <file>\n").
		Str("  this rewrites the old leaf into the current shape; then load <new-file>").String()
}

// graftRetiredLeaves puts each retired leaf back under its old parent, for
// MigrationSchema alone. The cached YANGSchema never carries them.
func graftRetiredLeaves(schema *Schema) {
	for _, r := range registeredRetiredLeaves() {
		switch parent := schema.LookupTokenPath(r.path).(type) {
		case *ContainerNode:
			parent.children[r.name] = r.node
			parent.order = append(parent.order, r.name)
			delete(parent.retired, r.name)
		case *ListNode:
			parent.children[r.name] = r.node
			parent.order = append(parent.order, r.name)
			delete(parent.retired, r.name)
		}
	}
}

// retiredHint answers the refusal text for name when it is a retired leaf of
// this container, or "" when it is not.
func (n *ContainerNode) retiredHint(name string) string { return n.retired[name] }

// retiredHint answers the refusal text for name when it is a retired leaf of
// this list's entries, or "" when it is not.
func (n *ListNode) retiredHint(name string) string { return n.retired[name] }
