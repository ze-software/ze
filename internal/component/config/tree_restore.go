// Design: docs/architecture/config/yang-config-design.md -- the config tree the editor edits
// Related: tree.go -- the leaf storage this restores
// Related: tree_comments.go -- the leaf comments this restores

package config

import (
	"maps"
	"slices"
)

// RestoreValue makes leaf name in t what it is in from: its value, its
// leaf-list members and their deactivation, its own deactivation, its stored
// secret, and the comments on its lines. A nil from, or a from that does not
// hold the leaf, removes it from t. A leaf both trees hold keeps its place in
// t's order; a leaf only from holds takes the last place.
//
// It serves the editor's `discard <path>` for one leaf: the pending change to
// that leaf goes, and every other change in t stays (cli Editor.DiscardPath).
// The copy is made field by field rather than through Set, because Set does
// not carry the deactivation, the stored secret or the comments, and a discard
// that left any of them behind would not restore the committed leaf.
//
// Safe for concurrent use. t and from MUST be different trees.
func (t *Tree) RestoreValue(name string, from *Tree) {
	if t == from {
		return
	}
	saved := savedLeafOf(from, name)

	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.values, name)
	delete(t.multiValues, name)
	delete(t.mirrored, name)
	delete(t.inactiveValues, name)
	delete(t.inactiveMembers, name)
	delete(t.storedSecrets, name)
	t.dropLeafCommentsLocked(name)

	if !saved.present {
		t.valuesOrder = slices.DeleteFunc(t.valuesOrder, func(n string) bool { return n == name })
		return
	}
	if saved.hasValue {
		if !slices.Contains(t.valuesOrder, name) {
			t.valuesOrder = append(t.valuesOrder, name)
		}
		t.values[name] = saved.value
	} else {
		t.valuesOrder = slices.DeleteFunc(t.valuesOrder, func(n string) bool { return n == name })
	}
	if saved.members != nil {
		t.multiValues[name] = saved.members
	}
	if saved.hasMirrored {
		t.mirrored[name] = saved.mirrored
	}
	if saved.hasInactive {
		t.inactiveValues[name] = saved.inactive
	}
	if saved.inactiveMembers != nil {
		t.inactiveMembers[name] = saved.inactiveMembers
	}
	if saved.hasSecret {
		t.storedSecrets[name] = saved.secret
	}
	if len(saved.comments) > 0 && t.leafComments == nil {
		t.leafComments = make(map[leafCommentKey]*statementComments, len(saved.comments))
	}
	maps.Copy(t.leafComments, saved.comments)
}

// savedLeaf is a copy of everything one tree holds for one leaf, taken under
// that tree's read lock so RestoreValue never holds two locks at once.
type savedLeaf struct {
	present         bool
	value           string
	hasValue        bool
	members         []string
	mirrored        bool
	hasMirrored     bool
	inactive        bool
	hasInactive     bool
	inactiveMembers map[string]bool
	secret          string
	hasSecret       bool
	comments        map[leafCommentKey]*statementComments
}

// savedLeafOf copies leaf name out of from. A nil from holds no leaf.
func savedLeafOf(from *Tree, name string) savedLeaf {
	var saved savedLeaf
	if from == nil {
		return saved
	}
	from.mu.RLock()
	defer from.mu.RUnlock()

	saved.value, saved.hasValue = from.values[name]
	if members, ok := from.multiValues[name]; ok {
		saved.members = slices.Clone(members)
		if saved.members == nil {
			saved.members = []string{}
		}
	}
	saved.mirrored, saved.hasMirrored = from.mirrored[name]
	saved.inactive, saved.hasInactive = from.inactiveValues[name]
	if members, ok := from.inactiveMembers[name]; ok {
		saved.inactiveMembers = maps.Clone(members)
	}
	saved.secret, saved.hasSecret = from.storedSecrets[name]
	for key, found := range from.leafComments {
		if key.name != name {
			continue
		}
		if saved.comments == nil {
			saved.comments = make(map[leafCommentKey]*statementComments)
		}
		copied := found.clone()
		saved.comments[key] = &copied
	}
	saved.present = saved.hasValue || saved.members != nil
	return saved
}
