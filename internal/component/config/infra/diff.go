// Design: docs/architecture/config/syntax.md — config diff command
// Related: bgp.go — the BGP tree resolver the comparison reads the bgp block through

package infra

import (
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/configorder"
	"github.com/ze-software/ze/internal/core/opererr"
)

// bgpRoot is the one root DiffConfigs compares resolved rather than as written.
const bgpRoot = "bgp"

// errNoDiffTree is NewDiffSide's answer to a nil tree: there is nothing to compare.
var errNoDiffTree = errors.New("no parsed configuration to compare")

// DiffSide is one parsed configuration made ready for DiffConfigs. It keeps the
// parsed tree, which holds every root as written and the order of every list,
// beside the resolved bgp block. NewDiffSide is the only constructor, so
// DiffConfigs never compares a side whose bgp block was not resolved.
type DiffSide struct {
	parsed *config.Tree
	// bgp is the resolved bgp block, or nil when the tree holds none.
	bgp map[string]any
}

// NewDiffSide resolves the bgp block of a parsed configuration for DiffConfigs.
// A tree with no bgp block is a side with no BGP values: resolving an absent
// block would ask the engine to validate configuration that is not there.
func NewDiffSide(tree *config.Tree) (*DiffSide, error) {
	if tree == nil {
		return nil, errNoDiffTree
	}
	side := &DiffSide{parsed: tree}
	if tree.GetContainer(bgpRoot) == nil {
		return side, nil
	}
	resolved, err := ResolveBGPTree(tree)
	if err != nil {
		return nil, err
	}
	side.bgp = resolved
	return side, nil
}

// DiffConfigs compares two configurations over the whole tree, with every
// secret masked on both sides. It is the one producer of `ze config diff`,
// `show config diff` and `show config supplied diff`.
//
// The bgp block is compared resolved, at the paths ResolveBGPTree gives it
// (`router-id`, `peer/<name>/...`, with group and bgp-level values inherited
// into each peer), so a value a peer inherits shows on that peer. Every other
// root is compared as written, under its root name (`plugin/hub/client/<name>/host`),
// one entry per leaf: an added list entry or container answers each leaf it
// holds, never one entry carrying the subtree.
// The order of each schema-declared ordered list is compared at its full
// configuration path under the `@<list>` key; the order of a plain list is not
// a change.
//
// The resolved bgp paths carry no root name, so a root whose name is also a key
// of a resolved bgp block would give two values one path. That is refused
// rather than answered with one value hiding the other.
//
// Masking writes the placeholder into the values the sides hold, so each side
// MUST be passed to DiffConfigs once only. A refusal is an operator error.
func DiffConfigs(schema *config.Schema, before, after *DiffSide) (*config.ConfigDiff, error) {
	diff := config.DiffMaps(before.bgp, after.bgp)

	rootsBefore, rootsAfter := rootsAsWritten(before.parsed), rootsAsWritten(after.parsed)
	if err := refuseSharedPath(rootsBefore, before.bgp, after.bgp); err != nil {
		return nil, err
	}
	if err := refuseSharedPath(rootsAfter, before.bgp, after.bgp); err != nil {
		return nil, err
	}
	addRootLeaves(diff, config.DiffMaps(rootsBefore, rootsAfter))

	diffListOrders(before.parsed, after.parsed, schema, diff)
	maskDiffSecrets(diff, config.SecretKeys(schema))
	return diff, nil
}

// DiffChangeCount is the number of changes a diff reports: each added, removed
// and changed entry counts once. An entry of a root compared as written is one
// leaf; an added or removed bgp peer is one entry holding its subtree.
func DiffChangeCount(diff *config.ConfigDiff) int {
	return len(diff.Added) + len(diff.Removed) + len(diff.Changed)
}

// rootsAsWritten lowers every root of a parsed tree except bgp, keyed by root
// name, so a diff of two of them answers paths that start with the root.
func rootsAsWritten(tree *config.Tree) map[string]any {
	roots := tree.ToMap()
	delete(roots, bgpRoot)
	return roots
}

// addRootLeaves adds the diff of the roots compared as written to diff, one
// entry per leaf path. DiffMaps answers an added or removed container or list
// entry as one entry holding the whole subtree, which the text shape would
// print as a Go map. A path whose value turns from a subtree into a leaf, or
// back, answers the leaves of the subtree as removed or added and the leaf
// beside them.
func addRootLeaves(diff, roots *config.ConfigDiff) {
	for path, value := range roots.Added {
		addLeaves(diff.Added, path, value)
	}
	for path, value := range roots.Removed {
		addLeaves(diff.Removed, path, value)
	}
	for path, pair := range roots.Changed {
		_, oldIsSubtree := pair.Old.(map[string]any)
		_, newIsSubtree := pair.New.(map[string]any)
		if !oldIsSubtree && !newIsSubtree {
			diff.Changed[path] = pair
			continue
		}
		addLeaves(diff.Removed, path, pair.Old)
		addLeaves(diff.Added, path, pair.New)
	}
}

// addLeaves writes every leaf of value into entries at its full path. An empty
// container has no leaf, so it stays one entry at its own path: dropping it
// would hide an added or removed presence container. The recursion is bounded
// by the depth of the schema the tree was parsed with.
func addLeaves(entries map[string]any, path string, value any) {
	subtree, ok := value.(map[string]any)
	if !ok || len(subtree) == 0 {
		entries[path] = value
		return
	}
	for key, child := range subtree {
		addLeaves(entries, config.AppendPath(path, key), child)
	}
}

// refuseSharedPath refuses a root whose name is also a top-level key of a
// resolved bgp block, of either side: a removed bgp setting and an added root
// of that name would answer under one path too. No compiled-in module declares
// such a root today; an external plugin's module, whose root its author names,
// can.
func refuseSharedPath(roots map[string]any, bgpSides ...map[string]any) error {
	for name := range roots {
		for _, bgp := range bgpSides {
			if _, shared := bgp[name]; shared {
				return opererr.New(
					fmt.Sprintf("the configuration root %q has the same name as a setting of the bgp block", name),
					"the comparison names bgp settings without their root, so it cannot tell the two apart",
					"ze config show <file> "+name)
			}
		}
	}
	return nil
}

// diffListOrders adds the entry-key sequence of every schema-declared ordered
// list to diff, leaving every value path intact.
func diffListOrders(before, after *config.Tree, schema *config.Schema, diff *config.ConfigDiff) {
	oldOrders, newOrders := make(map[string][]string), make(map[string][]string)
	secretPaths := make(map[string]bool)
	collectDiffListOrders(before, schema, "", oldOrders, secretPaths)
	collectDiffListOrders(after, schema, "", newOrders, secretPaths)
	for path, old := range oldOrders {
		if _, present := newOrders[path]; !present {
			diff.Removed[path] = maskDiffOrder(old, secretPaths[path])
		}
	}
	for path, keys := range newOrders {
		old, present := oldOrders[path]
		if !present {
			diff.Added[path] = maskDiffOrder(keys, secretPaths[path])
			continue
		}
		// Compare real keys first; masking before comparison would hide a move.
		if !slices.Equal(old, keys) {
			diff.Changed[path] = config.DiffPair{
				Old: maskDiffOrder(old, secretPaths[path]),
				New: maskDiffOrder(keys, secretPaths[path]),
			}
		}
	}
}

// collectDiffListOrders walks the parsed tree, including containers and
// entries of plain lists: an ordered list can be nested inside either. The
// recursion is bounded by the depth of the schema, which Ze compiles in.
func collectDiffListOrders(tree *config.Tree, schema config.ChildProvider, prefix string, orders map[string][]string, secrets map[string]bool) {
	for _, name := range schema.Children() {
		node := schema.Get(name)
		path := config.AppendPath(prefix, name)
		entries := tree.GetList(name)
		if list, ok := node.(*config.ListNode); ok && list.UserOrdered && len(entries) > 0 {
			recorded := tree.GetListOrdered(name)
			keys := make([]string, 0, len(recorded))
			for _, entry := range recorded {
				keys = append(keys, entry.Key)
			}
			orderPath := config.AppendPath(prefix, configorder.OrderKey(name))
			orders[orderPath] = config.ListEntryKeys(entries, keys)
			if list.KeyLeaf != nil && (list.KeyLeaf.Sensitive || list.KeyLeaf.Bcrypt) {
				secrets[orderPath] = true
			}
		}
		children, ok := node.(config.ChildProvider)
		if !ok {
			continue
		}
		if sub := tree.GetContainer(name); sub != nil {
			collectDiffListOrders(sub, children, path, orders, secrets)
		}
		for key, entry := range entries {
			collectDiffListOrders(entry, children, config.AppendPath(path, key), orders, secrets)
		}
	}
}

func maskDiffOrder(keys []string, secret bool) []string {
	if !secret {
		return keys
	}
	for i := range keys {
		keys[i] = config.SecretDataPlaceholder
	}
	return keys
}

// maskDiffSecrets replaces every secret value the diff carries with the display
// placeholder, in the text shape and in the JSON shape alike.
//
// The diff runs on the REAL values first, so a rotated credential still reports
// as changed. Masking the two trees before the diff would give both sides the
// same placeholder, and the command would answer that nothing moved. The
// operator learns which leaf changed, and neither value. That is the answer
// Editor.Diff (internal/component/cli/editor.go) and the web commit review
// (changedSecretLines, internal/component/web/cli_terminal.go) already write.
//
// The parser decodes a $9$ value into the tree, so an unmasked diff would print
// in cleartext what the file holds encoded.
func maskDiffSecrets(diff *config.ConfigDiff, secretKeys map[string]bool) {
	if len(secretKeys) == 0 {
		return
	}
	for key, value := range diff.Added {
		diff.Added[key] = maskDiffValue(key, value, secretKeys)
	}
	for key, value := range diff.Removed {
		diff.Removed[key] = maskDiffValue(key, value, secretKeys)
	}
	for key, pair := range diff.Changed {
		diff.Changed[key] = config.DiffPair{
			Old: maskDiffValue(key, pair.Old, secretKeys),
			New: maskDiffValue(key, pair.New, secretKeys),
		}
	}
}

// maskDiffValue masks one diff entry. A whole subtree is walked, because an
// added bgp peer carries its own secret. A value is masked
// when the last segment of its path names a secret leaf.
//
// The name is what identifies the leaf here. ResolveBGPTree flattens group and
// peer inheritance, so a path in the resolved map addresses no schema node.
// `ze config dump` reads a name for the same reason, and one rule serves the
// roots compared as written too.
func maskDiffValue(path string, value any, secretKeys map[string]bool) any {
	if subtree, ok := value.(map[string]any); ok {
		maskDiffSubtree(subtree, secretKeys)
		return subtree
	}
	segments := config.SplitPath(path)
	if secretKeys[segments[len(segments)-1]] {
		return config.SecretDataPlaceholder
	}
	maskDiffComposite(reflect.ValueOf(value), secretKeys)
	return value
}

// maskDiffComposite masks the maps a value carries inside a slice or a struct.
// ResolveBGPTree answers each dynamic group as a DynamicGroupTemplate whose
// Template map holds the values the group gives its peers, an md5 password
// among them, and a diff entry carries the whole list of them. The recursion
// is bounded by the depth of the Go types the resolver answers.
func maskDiffComposite(value reflect.Value, secretKeys map[string]bool) {
	// The kinds below are the ones that can hold a map; every other kind is a
	// scalar the name rule of maskDiffValue already judged.
	//exhaustive:ignore
	switch value.Kind() {
	case reflect.Map:
		if subtree, ok := reflect.TypeAssert[map[string]any](value); ok {
			maskDiffSubtree(subtree, secretKeys)
		}
	case reflect.Slice, reflect.Array:
		for i := range value.Len() {
			maskDiffComposite(value.Index(i), secretKeys)
		}
	case reflect.Struct:
		for i := range value.NumField() {
			if value.Type().Field(i).IsExported() {
				maskDiffComposite(value.Field(i), secretKeys)
			}
		}
	case reflect.Pointer, reflect.Interface:
		if !value.IsNil() {
			maskDiffComposite(value.Elem(), secretKeys)
		}
	}
}

// maskDiffSubtree replaces, in place, every value held under a secret leaf name
// with the placeholder, a leaf-list of secrets included. The recursion is
// bounded by the depth of the schema the tree was parsed with.
func maskDiffSubtree(subtree map[string]any, secretKeys map[string]bool) {
	for key, value := range subtree {
		if nested, ok := value.(map[string]any); ok {
			maskDiffSubtree(nested, secretKeys)
			continue
		}
		if secretKeys[key] {
			subtree[key] = config.SecretDataPlaceholder
		}
	}
}
