// Design: docs/architecture/config/syntax.md -- a block that moved to a top-level root
// Related: retired.go -- UnknownKeywordHint, the hint a keyword with no new home carries
// Related: yang_schema.go -- schemaFromResolved, which marks each old position
// Related: migration/migrate.go -- the transformation that rewrites the old position

package config

import (
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// relocation is a container that moved from `<parent> { <root> { ... } }` to the
// top-level `<root> { ... }`. The schema holds only the new position, so the old
// one parses as an unknown field: the refusal names the new root and the
// `ze config migrate` command, and that command parses the file with the old
// position grafted back (MigrationSchema) and moves the block (ApplyRelocations).
// The block keeps its name: only its parent changes.
type relocation struct {
	parent string
	root   string
}

var (
	relocationsMu sync.RWMutex
	relocations   []relocation

	errRelocationConflict = errors.New("the same entry is configured at both positions")
)

// MustRegisterRelocatedContainer records that the block `root`, once a child of
// the top-level container `parent`, is now the top-level root `root`. The
// package that owns the new root calls it from init(). A second registration of
// the same pair is a Ze defect and panics.
func MustRegisterRelocatedContainer(parent, root string) {
	relocationsMu.Lock()
	defer relocationsMu.Unlock()
	for _, r := range relocations {
		if r.parent == parent && r.root == root {
			panic("BUG: config: relocation " + parent + "/" + root + " registered twice")
		}
	}
	relocations = append(relocations, relocation{parent: parent, root: root})
}

// registeredRelocations answers a copy of the registered relocations.
func registeredRelocations() []relocation {
	relocationsMu.RLock()
	defer relocationsMu.RUnlock()
	return slices.Clone(relocations)
}

// markRelocations records each relocation on its old parent's schema node, so
// the parsers refuse the old position with the relocation hint instead of the
// generic closest-key one. A parent the build does not carry has no old
// position to refuse.
func markRelocations(schema *Schema) {
	for _, r := range registeredRelocations() {
		parent, ok := schema.Get(r.parent).(*ContainerNode)
		if !ok {
			continue
		}
		if parent.relocated == nil {
			parent.relocated = make(map[string]string)
		}
		parent.relocated[r.root] = r.parent
	}
}

// relocatedHint answers the text an unknown-field refusal carries when name is
// a block that moved out of this container to a top-level root, or "" when it
// did not.
func (n *ContainerNode) relocatedHint(name string) string {
	parent, moved := n.relocated[name]
	if !moved {
		return ""
	}
	var b textbuf.Buffer
	return b.Str("\n  ").Str(name).Str(" is no longer inside ").Str(parent).
		Str(": it is now the top-level ").Str(name).Str(" { ... } block, shared by every protocol\n").
		Str("  fix: ze config migrate -o <new-file> <file>\n").
		Str("  this writes ").Str(parent).Str(" { ").Str(name).Str(" { ... } } as ").Str(name).
		Str(" { ... }; then load <new-file>").String()
}

// MigrationSchema answers a schema that also accepts each relocated block at its
// old position, for `ze config migrate` to parse a file written before the move.
// It is built fresh, never the cached YANGSchema, because grafting the old
// position into the shared schema would make every load accept it.
func MigrationSchema() (*Schema, error) {
	schema, err := YANGSchemaWithPlugins(nil)
	if err != nil {
		return nil, err
	}
	for _, r := range registeredRelocations() {
		parent, isContainer := schema.Get(r.parent).(*ContainerNode)
		if !isContainer {
			continue
		}
		moved := schema.Get(r.root)
		if moved == nil {
			continue
		}
		parent.children[r.root] = moved
		parent.order = append(parent.order, r.root)
		delete(parent.relocated, r.root)
	}
	graftRetiredLeaves(schema)
	return schema, nil
}

// HasRelocatedContainers answers whether tree holds a block at a position it
// moved from.
func HasRelocatedContainers(tree *Tree) bool {
	for _, r := range registeredRelocations() {
		parent := tree.GetContainer(r.parent)
		if parent != nil && parent.GetContainer(r.root) != nil {
			return true
		}
	}
	return false
}

// ApplyRelocations moves every block found at a position it moved from to its
// top-level root, merging it into a root block the file already holds. A list
// entry configured under both is refused: the two may differ, and choosing one
// would drop the other in silence. The tree is modified in place; on error the
// caller MUST discard it.
func ApplyRelocations(tree *Tree) (*Tree, error) {
	schema, err := YANGSchema()
	if err != nil {
		return nil, err
	}
	for _, r := range registeredRelocations() {
		parent := tree.GetContainer(r.parent)
		if parent == nil {
			continue
		}
		old := parent.GetContainer(r.root)
		if old == nil {
			continue
		}
		if err := refuseRelocationConflict(r, tree.GetContainer(r.root), old); err != nil {
			return nil, err
		}
		parent.RemoveContainer(r.root)
		var rootSchema ChildProvider
		if node, ok := schema.Get(r.root).(*ContainerNode); ok {
			rootSchema = node
		}
		if err := tree.MergeContainer(r.root, old, rootSchema); err != nil {
			return nil, fmt.Errorf("move %s %s to the top-level %s: %w", r.parent, r.root, r.root, err)
		}
	}
	return tree, nil
}

// refuseRelocationConflict refuses a move when current, the root block already
// at the new position, holds a list entry old holds too.
func refuseRelocationConflict(r relocation, current, old *Tree) error {
	if current == nil {
		return nil
	}
	for _, list := range old.ListNames() {
		existing := current.GetList(list)
		for _, key := range old.ListKeys(list) {
			if _, both := existing[key]; both {
				return fmt.Errorf("%s %s %s %s and %s %s %s: %w; keep one and run ze config migrate again",
					r.parent, r.root, list, key, r.root, list, key, errRelocationConflict)
			}
		}
	}
	return nil
}
