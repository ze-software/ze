// Design: docs/architecture/config/yang-config-design.md — resolved config schema walk
// Related: claims.go -- Node, and the audit that walks the tree built here
//
// SchemaTree merges the resolved entry trees of every config-schema module into
// one config data tree. Merging is required, not a convenience: several modules
// declare the same top-level container (five contribute children under
// "system"), and an augment-only module declares no top level at all.

package claims

import (
	"errors"
	"slices"

	goyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// configuredModule labels a node that comes from an operator's config rather
// than from a YANG module, so a finding over a live config does not name a
// module that did not produce it.
const configuredModule = "the running config"

// SchemaTree builds the merged config data tree from a resolved loader.
//
// Config-schema modules are the "-conf" modules (Loader.ConfModuleNames). The
// type library, the extensions, and the "-api" modules carry no config surface,
// so they are not walked.
//
// State nodes, RPCs, and notifications are left out: the subject is config an
// operator can write. Choice and case nodes are transparent, because they add a
// schema level that no config path carries.
func SchemaTree(l *configyang.Loader) (*Node, error) {
	if l == nil {
		return nil, errNilLoader
	}

	var tb textbuf.Buffer
	root := &Node{Children: map[string]*Node{}}
	for _, name := range l.ConfModuleNames() {
		entry := l.GetEntry(name)
		if entry == nil {
			return nil, errors.New(tb.Reset().Str("claims: config module ").Str(name).
				Str(" resolved to no entry tree, so its config surface would be checked by nothing").String())
		}
		mergeEntry(root, entry, name)
	}
	return root, nil
}

// FromConfigTree builds a Node tree from one operator config tree.
//
// SchemaTree says what an operator COULD write. This says what one operator
// DID write, which is what the doctor check judges: a root present in a live
// config that this build delivers to nobody.
//
// The walk goes through Tree.ToMap, which is the same shape the daemon delivers
// to a plugin, so a list entry appears as one level below its list name. A claim
// covers everything below it, so the extra level changes no verdict, and it
// keeps a root whose only content is a list from looking empty.
func FromConfigTree(t *config.Tree) *Node {
	root := &Node{Children: map[string]*Node{}}
	if t != nil {
		mergeConfigMap(root, t.ToMap())
	}
	return root
}

func mergeConfigMap(dst *Node, m map[string]any) {
	for name, value := range m {
		node := &Node{
			Path:     config.AppendPath(dst.Path, name),
			Children: map[string]*Node{},
			Modules:  []string{configuredModule},
		}
		dst.Children[name] = node

		if child, ok := value.(map[string]any); ok {
			mergeConfigMap(node, child)
			continue
		}
		node.IsLeaf = true
	}
}

// mergeEntry copies the config data children of entry into dst.
func mergeEntry(dst *Node, entry *goyang.Entry, module string) {
	for name, child := range entry.Dir {
		if skipEntry(child) {
			continue
		}
		if child.IsChoice() || child.IsCase() {
			// Transparent: a choice or a case adds a schema level that the
			// config path does not carry, so its children belong to dst.
			mergeEntry(dst, child, module)
			continue
		}

		node := dst.Children[name]
		if node == nil {
			node = &Node{Path: config.AppendPath(dst.Path, name), Children: map[string]*Node{}}
			dst.Children[name] = node
		}
		addModule(node, module)
		if child.IsLeaf() || child.IsLeafList() {
			node.IsLeaf = true
		}
		mergeEntry(node, child, module)
	}
}

// skipEntry reports whether a schema entry sits outside the config surface.
func skipEntry(e *goyang.Entry) bool {
	switch {
	case e == nil, e.RPC != nil:
		return true
	case e.Kind == goyang.NotificationEntry, e.Kind == goyang.InputEntry, e.Kind == goyang.OutputEntry:
		return true
	case e.ReadOnly():
		return true // config false
	default:
		return false
	}
}

// addModule records a contributing module, keeping Modules a sorted set.
func addModule(n *Node, module string) {
	if slices.Contains(n.Modules, module) {
		return
	}
	n.Modules = append(n.Modules, module)
	for i := len(n.Modules) - 1; i > 0 && n.Modules[i] < n.Modules[i-1]; i-- {
		n.Modules[i], n.Modules[i-1] = n.Modules[i-1], n.Modules[i]
	}
}
