// Design: docs/guide/config-editor.md -- discard all, discard <path>
// Related: editor_commit.go -- DiscardSessionPath, the session form of the same command
// Related: editor_commands.go -- Discard, which drops every pending change

package cli

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// DiscardPath drops the pending changes under fullPath and keeps every change
// outside it: the element at fullPath becomes what the committed config holds,
// or goes when the committed config does not hold it. In an editing session it
// is DiscardSessionPath, which drops this session's recorded changes instead.
//
// The editor without a session holds no record of its changes, only the
// working tree beside the committed text, so the committed element is read
// from a parse of that text and copied over the working one. The model once
// answered `discard <path>` with Discard, which dropped every change and
// ignored the path.
func (e *Editor) DiscardPath(fullPath []string) error {
	if len(fullPath) == 0 {
		return errEmptyPath
	}
	if e.session != nil {
		return e.DiscardSessionPath(fullPath)
	}
	if e.schema == nil {
		return errSchemaNotAvailable
	}
	path, err := config.CanonicalEditPath(e.schema, fullPath)
	if err != nil {
		return fmt.Errorf("discard address: %w", err)
	}
	committed, err := config.NewParser(e.schema).Parse(e.originalContent)
	if err != nil {
		return fmt.Errorf("discard '%s': the committed config does not parse, so its content cannot be read back: %w", textbuf.Join(path, " "), err)
	}

	entry, isEntry, err := config.WalkListEntryPath(e.schema, path)
	if err != nil {
		return err
	}
	if isEntry && entry.End == len(path) {
		return e.restoreElement(committed, entry.ParentPath, func(working, original *config.Tree) {
			restoreListEntry(working, original, entry.ListName, entry.Address.Key)
		})
	}

	if len(path) >= 2 {
		memberParent := path[:len(path)-2]
		leafName := path[len(path)-2]
		member := path[len(path)-1]
		isLeafList := e.isValueOrArrayLeaf(memberParent, leafName)
		if !isLeafList {
			isLeafList = e.bracketLeafList(memberParent, leafName) != nil
		}
		if isLeafList {
			return e.restoreElement(committed, memberParent, func(working, original *config.Tree) {
				restoreMember(working, original, leafName, member)
			})
		}
	}

	name := path[len(path)-1]
	parentPath := path[:len(path)-1]
	parentSchema := e.walkSchema(parentPath)
	if parentSchema == nil {
		return fmt.Errorf("discard '%s': the config schema holds no such path", textbuf.Join(path, " "))
	}
	switch parentSchema.Get(name).(type) {
	case *config.LeafNode, *config.FreeformNode,
		*config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode:
		return e.restoreElement(committed, parentPath, func(working, original *config.Tree) {
			working.RestoreValue(name, original)
		})
	case *config.ContainerNode, *config.FlexNode:
		return e.restoreElement(committed, parentPath, func(working, original *config.Tree) {
			restoreContainer(working, original, name)
		})
	case *config.ListNode, *config.InlineListNode:
		return e.restoreElement(committed, parentPath, func(working, original *config.Tree) {
			restoreList(working, original, name)
		})
	}
	return fmt.Errorf("discard '%s': the config schema holds no such path", textbuf.Join(path, " "))
}

// restoreElement runs restore on the working and the committed tree that hold
// the element, at parentPath. original is nil when the committed config does
// not hold parentPath. The working parent is created only when the committed
// config has something to put back; otherwise a missing working parent means
// there is nothing pending to drop.
func (e *Editor) restoreElement(committed *config.Tree, parentPath []string, restore func(working, original *config.Tree)) error {
	original := walkPath(committed, e.schema, parentPath)
	working := walkPath(e.tree, e.schema, parentPath)
	if working == nil {
		// refuse-by-default: guards cmdDiscardPath -- a path neither tree holds has no pending change, so its discard is already complete
		if original == nil {
			return nil
		}
		created, err := walkOrCreateTree(e.tree, e.schema, parentPath)
		if err != nil {
			return err
		}
		working = created
	}
	restore(working, original)
	// Dirty reports the changes that remain, as the session form does: a
	// discard of the only pending change leaves nothing to save.
	e.dirty.Store(config.Serialize(e.tree, e.schema) != config.Serialize(committed, e.schema))
	return nil
}

// restoreContainer puts the committed container name back under working, or
// removes it when original holds none.
func restoreContainer(working, original *config.Tree, name string) {
	working.DeleteContainer(name)
	if original == nil {
		return
	}
	if child := original.GetContainer(name); child != nil {
		working.SetContainer(name, child.Clone())
	}
}

// restoreList puts every committed entry of list name back under working, in
// the committed order, or removes the list when original holds none.
func restoreList(working, original *config.Tree, name string) {
	working.DeleteList(name)
	if original == nil {
		return
	}
	for _, item := range original.GetListOrdered(name) {
		working.AddListEntry(name, item.Key, item.Value.Clone())
	}
}

// restoreListEntry puts the committed entry key of list name back under
// working at its committed place, or removes it when original holds none.
func restoreListEntry(working, original *config.Tree, name, key string) {
	working.RemoveListEntry(name, key)
	if original == nil {
		return
	}
	entry, ok := original.GetList(name)[key]
	if !ok {
		return
	}
	working.AddListEntry(name, key, entry.Clone())
	working.RepositionListEntries(name, []string{key}, original)
}

// restoreMember makes one leaf-list member what the committed config holds:
// present with its committed deactivation, or absent. The other members, and
// their pending changes, stay.
func restoreMember(working, original *config.Tree, leafName, member string) {
	present, inactive := false, false
	if original != nil {
		present, inactive = original.MultiValueMemberState(leafName, member)
	}
	states := working.GetMultiValuesState(leafName)
	kept := make([]config.MemberState, 0, len(states)+1)
	found := false
	for _, state := range states {
		if state.Value != member {
			kept = append(kept, state)
			continue
		}
		found = true
		if present {
			kept = append(kept, config.MemberState{Value: member, Inactive: inactive})
		}
	}
	if !found && present {
		kept = append(kept, config.MemberState{Value: member, Inactive: inactive})
	}
	if len(kept) == 0 {
		working.RestoreValue(leafName, original)
		return
	}
	working.SetMemberStates(leafName, kept)
}
