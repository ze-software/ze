// Design: docs/guide/config-editor.md — deactivate and activate
// Related: model_commands_edit.go — runActivation, the SSH editor's caller
// Related: editor_commands.go — DeactivateLeaf, DeactivatePath and their activate twins

package cli

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ApplyActivation deactivates (activate false) or activates the node a token
// path names, and returns the status line every editor prints. It is the one
// dispatch the SSH and web editors share: a leaf-list value, a leaf, or a
// container or list entry, told apart by the schema. A node already in the
// requested state answers "<path> already deactivated|active" and no error.
// Not safe for concurrent use: the caller holds the editor.
func (e *Editor) ApplyActivation(fullPath []string, activate bool) (string, error) {
	verb, pastTense, alreadyState := "deactivate", "Deactivated", "deactivated"
	if activate {
		verb, pastTense, alreadyState = "activate", "Activated", "active"
	}
	if len(fullPath) == 0 {
		return "", fmt.Errorf("usage: %s <path>", verb)
	}

	var tb textbuf.Buffer
	if parentPath, leafListName, isLeafList := e.ResolveLeafListValue(fullPath); isLeafList {
		value := fullPath[len(fullPath)-1]
		var err error
		if activate {
			err = e.ActivateLeafListValue(parentPath, leafListName, value)
		} else {
			err = e.DeactivateLeafListValue(parentPath, leafListName, value)
		}
		if err != nil {
			return "", fmt.Errorf("%s failed: %w", verb, err)
		}
		return tb.Str(pastTense).Byte(' ').Str(value).Str(" in ").Str(leafListName).String(), nil
	}

	err := e.applyNodeActivation(fullPath, activate)
	if err != nil {
		if isAlreadyInState(err) {
			return tb.Join(fullPath, " ").Str(" already ").Str(alreadyState).String(), nil
		}
		return "", fmt.Errorf("%s failed: %w", verb, err)
	}
	return tb.Str(pastTense).Byte(' ').Join(fullPath, " ").String(), nil
}

// isAlreadyInState names the four refusals that mean the node already holds
// the state asked for, which the editors report as a status, not an error.
func isAlreadyInState(err error) bool {
	if errors.Is(err, ErrLeafAlreadyInactive) {
		return true
	}
	if errors.Is(err, ErrPathAlreadyInactive) {
		return true
	}
	if errors.Is(err, ErrLeafNotInactive) {
		return true
	}
	return errors.Is(err, ErrPathNotInactive)
}

// applyNodeActivation toggles the leaf, container or list entry fullPath
// names. A path the schema does not know goes to DeactivatePath or
// ActivatePath, which refuse it as "path not found". A positional list entry
// (a list whose children are all leaves, such as capability nexthop) and the
// freeform, flex and inline-list kinds carry no inactive marker, so they are
// refused rather than toggled.
func (e *Editor) applyNodeActivation(fullPath []string, activate bool) error {
	node := e.schema.LookupTokenPath(fullPath)
	if node == nil {
		return e.applyPathActivation(fullPath, activate)
	}
	var tb textbuf.Buffer
	switch node.Kind() {
	case config.NodeLeaf:
		parentPath, leafName := fullPath[:len(fullPath)-1], fullPath[len(fullPath)-1]
		if activate {
			return e.ActivateLeaf(parentPath, leafName)
		}
		return e.DeactivateLeaf(parentPath, leafName)
	case config.NodeList:
		listNode, ok := node.(*config.ListNode)
		if ok && !listNode.HasStructuralChildren() {
			return fmt.Errorf("path %q is a positional list entry; deactivate the parent container instead", tb.Join(fullPath, " ").String())
		}
		return e.applyPathActivation(fullPath, activate)
	case config.NodeContainer:
		return e.applyPathActivation(fullPath, activate)
	case config.NodeFreeform, config.NodeFlex, config.NodeInlineList:
		return fmt.Errorf("path %q resolves to a node type that does not support deactivation", tb.Join(fullPath, " ").String())
	}
	return fmt.Errorf("path %q resolves to an unknown node kind %d", tb.Join(fullPath, " ").String(), node.Kind())
}

// applyPathActivation toggles a container or list entry.
func (e *Editor) applyPathActivation(fullPath []string, activate bool) error {
	if activate {
		return e.ActivatePath(fullPath)
	}
	return e.DeactivatePath(fullPath)
}
