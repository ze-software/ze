// Design: docs/architecture/config/yang-config-design.md — config tree mutation commands
// Overview: model_commands.go — command dispatch

package cli

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errUsageSetPathValue                      = errors.New("usage: set <path> <value>")
	errUsageDeletePath                        = errors.New("usage: delete <path>")
	errUsageInsertPathValueFirstlastbeforeRef = errors.New("usage: insert <path> <value> first|last|before <ref>|after <ref>")
	errInsertFailedTargetIsNotA               = errors.New("insert failed: target is not a leaf-list")
	errUsageRenamePathOldNameTo               = errors.New("usage: rename <path> <old-name> to <new-name>")
	errUsageCopyPathSourceToDestination       = errors.New("usage: copy <path> <source> to <destination>")
)

func (m *Model) cmdSet(args []string) (commandResult, error) {
	if len(args) < 2 {
		return commandResult{}, errUsageSetPathValue
	}

	// tokenizeCommand already handles quotes, so args are clean tokens.
	// Last token is value, everything before (with context) is the path.
	fullPath := make([]string, 0, len(m.contextPath)+len(args))
	fullPath = append(fullPath, m.contextPath...)
	fullPath = append(fullPath, args...)

	value := fullPath[len(fullPath)-1]
	path := fullPath[:len(fullPath)-1]

	if len(path) < 1 {
		return commandResult{}, errUsageSetPathValue
	}

	key := path[len(path)-1]
	containerPath := path[:len(path)-1]

	// When the path ends at a list's key leaf keyword (e.g., "next-hop address"),
	// the value is the key for a new list entry. Check BEFORE validateTokenPath
	// because the keyword at end-of-path would be rejected as "missing key value".
	isListKey := m.editor.isListKeyLeafPath(path)
	if isListKey {
		listName := containerPath[len(containerPath)-1]
		listParent := containerPath[:len(containerPath)-1]
		if err := m.editor.ensureListEntry(listParent, listName, value); err != nil {
			return commandResult{}, fmt.Errorf("set failed: %w", err)
		}
	} else {
		// Validate the full token path (with list keys) against schema.
		if _, err := m.completer.validateTokenPath(path); err != nil {
			return commandResult{}, err
		}
		// Validate value against YANG type before applying
		if err := m.completer.ValidateValueAtPath(path, value); err != nil {
			return commandResult{}, err
		}
		if err := m.editor.SetValue(containerPath, key, value); err != nil {
			return commandResult{}, fmt.Errorf("set failed: %w", err)
		}
	}

	// Update completer with mutated tree
	m.refreshCompleter()

	var tb textbuf.Buffer
	if isListKey {
		tb.Str("created ").Str(containerPath[len(containerPath)-1]).Byte(' ').Str(value)
	} else {
		displayPath := append(append([]string{}, containerPath...), key)
		tb.Str("set ").Join(displayPath, " ").Byte(' ').Str(value)
	}

	// Detect conflicts with other users' change files after each edit.
	if conflicts := m.editor.detectConflicts(); len(conflicts) > 0 {
		tb.Str(" (conflict with ").Str(conflicts[0].OtherUser).Str(" on ").Str(conflicts[0].Path).Byte(')')
	}
	msg := tb.String()

	return commandResult{
		statusMessage: msg,
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}

func (m *Model) cmdDelete(args []string) (commandResult, error) {
	if len(args) < 1 {
		return commandResult{}, errUsageDeletePath
	}

	// Build full path with context
	fullPath := make([]string, 0, len(m.contextPath)+len(args))
	fullPath = append(fullPath, m.contextPath...)
	fullPath = append(fullPath, args...)

	// Use schema-aware delete to handle leaf values, containers, and list entries.
	if err := m.editor.DeleteByPath(fullPath); err != nil {
		return commandResult{}, fmt.Errorf("delete failed: %w", err)
	}

	// Update completer with mutated tree
	m.refreshCompleter()

	var tb textbuf.Buffer
	tb.Str("Deleted ").Join(fullPath, " ")

	// Detect conflicts with other users' change files after each edit.
	if conflicts := m.editor.detectConflicts(); len(conflicts) > 0 {
		tb.Str(" (conflict with ").Str(conflicts[0].OtherUser).Str(" on ").Str(conflicts[0].Path).Byte(')')
	}
	msg := tb.String()

	return commandResult{
		statusMessage: msg,
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}

// cmdDeactivate marks a config node as inactive.
func (m *Model) cmdDeactivate(args []string) (commandResult, error) {
	return m.runActivation(args, false)
}

// cmdActivate clears the inactive flag from a config node.
func (m *Model) cmdActivate(args []string) (commandResult, error) {
	return m.runActivation(args, true)
}

// runActivation backs both cmdDeactivate (activate=false) and cmdActivate
// (activate=true). The two verbs share path resolution, leaf-list-value
// detection, and idempotent-error mapping; only the editor methods and
// the wording of the status messages differ.
//
//nolint:cyclop // exhaustive node-type dispatch
func (m *Model) runActivation(args []string, activate bool) (commandResult, error) {
	verb := "deactivate"
	pastTense := "Deactivated"
	alreadyState := "deactivated"
	if activate {
		verb = "activate"
		pastTense = "Activated"
		alreadyState = "active"
	}

	if len(args) < 1 {
		return commandResult{}, fmt.Errorf("usage: %s <path>", verb)
	}

	fullPath := make([]string, 0, len(m.contextPath)+len(args))
	fullPath = append(fullPath, m.contextPath...)
	fullPath = append(fullPath, args...)

	// Leaf-list value path.
	if len(fullPath) >= 2 {
		parentPath, leafListName, isLeafList := m.resolveLeafListValue(fullPath)
		if isLeafList {
			value := fullPath[len(fullPath)-1]
			var llErr error
			if activate {
				llErr = m.editor.ActivateLeafListValue(parentPath, leafListName, value)
			} else {
				llErr = m.editor.DeactivateLeafListValue(parentPath, leafListName, value)
			}
			if llErr != nil {
				return commandResult{}, fmt.Errorf("%s failed: %w", verb, llErr)
			}
			m.refreshCompleter()
			var tb textbuf.Buffer
			tb.Str(pastTense).Byte(' ').Str(value).Str(" in ").Str(leafListName)
			if conflicts := m.editor.detectConflicts(); len(conflicts) > 0 {
				tb.Str(" (conflict with ").Str(conflicts[0].OtherUser).Str(" on ").Str(conflicts[0].Path).Byte(')')
			}
			msg := tb.String()
			return commandResult{
				statusMessage: msg,
				configView:    m.configViewAtPath(m.contextPath),
				revalidate:    true,
			}, nil
		}
	}

	// Schema-validated leaf vs container/list-entry dispatch.
	entry, err := m.completer.validateTokenPath(fullPath)
	if err != nil {
		return commandResult{}, err
	}
	var opErr error
	switch {
	case entry != nil && entry.IsLeaf():
		parentPath := fullPath[:len(fullPath)-1]
		leafName := fullPath[len(fullPath)-1]
		if activate {
			opErr = m.editor.ActivateLeaf(parentPath, leafName)
		} else {
			opErr = m.editor.DeactivateLeaf(parentPath, leafName)
		}
	case activate:
		opErr = m.editor.ActivatePath(fullPath)
	default:
		opErr = m.editor.DeactivatePath(fullPath)
	}

	if opErr != nil {
		// Idempotent: already-in-state becomes a status message.
		if errors.Is(opErr, ErrLeafAlreadyInactive) || errors.Is(opErr, ErrPathAlreadyInactive) ||
			errors.Is(opErr, ErrLeafNotInactive) || errors.Is(opErr, ErrPathNotInactive) {
			var tb textbuf.Buffer
			return commandResult{
				statusMessage: tb.Join(fullPath, " ").Str(" already ").Str(alreadyState).String(),
				configView:    m.configViewAtPath(m.contextPath),
			}, nil
		}
		return commandResult{}, fmt.Errorf("%s failed: %w", verb, opErr)
	}

	m.refreshCompleter()
	var tb2 textbuf.Buffer
	tb2.Str(pastTense).Byte(' ').Join(fullPath, " ")
	if conflicts := m.editor.detectConflicts(); len(conflicts) > 0 {
		tb2.Str(" (conflict with ").Str(conflicts[0].OtherUser).Str(" on ").Str(conflicts[0].Path).Byte(')')
	}
	msg := tb2.String()
	return commandResult{
		statusMessage: msg,
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}

// resolveLeafListValue is a thin wrapper around Editor.ResolveLeafListValue.
// Kept for the existing call sites; new code should use the Editor method
// directly.
func (m *Model) resolveLeafListValue(fullPath []string) (parentPath []string, leafListName string, ok bool) {
	if m.editor == nil {
		return nil, "", false
	}
	return m.editor.ResolveLeafListValue(fullPath)
}

// cmdInsert inserts a value into a leaf-list at a specified position.
// Syntax: insert <path> <value> first|last|before <ref>|after <ref>.
// Limitation: values named "first", "last", "before", or "after" are
// ambiguous with position keywords. Quote them if needed.
func (m *Model) cmdInsert(args []string) (commandResult, error) {
	if len(args) < 3 {
		return commandResult{}, errUsageInsertPathValueFirstlastbeforeRef
	}

	// Parse position from the end of args.
	var position, ref string
	var pathAndValue []string

	lastArg := args[len(args)-1]
	if lastArg == config.InsertFirst || lastArg == config.InsertLast {
		position = lastArg
		pathAndValue = args[:len(args)-1]
	} else if len(args) >= 4 {
		secondLast := args[len(args)-2]
		if secondLast == config.InsertBefore || secondLast == config.InsertAfter {
			position = secondLast
			ref = lastArg
			pathAndValue = args[:len(args)-2]
		}
	}

	if position == "" {
		return commandResult{}, errUsageInsertPathValueFirstlastbeforeRef
	}

	if len(pathAndValue) < 2 {
		return commandResult{}, errUsageInsertPathValueFirstlastbeforeRef
	}

	value := pathAndValue[len(pathAndValue)-1]
	pathTokens := pathAndValue[:len(pathAndValue)-1]

	// Build full path to the leaf-list: context + path tokens
	fullPath := make([]string, 0, len(m.contextPath)+len(pathTokens))
	fullPath = append(fullPath, m.contextPath...)
	fullPath = append(fullPath, pathTokens...)

	// Validate the target is a leaf-list using schema-aware path walk.
	// Append a dummy value so resolveLeafListValue sees the leaf-list as second-to-last.
	probePath := make([]string, len(fullPath)+1)
	copy(probePath, fullPath)
	probePath[len(fullPath)] = "__probe__"
	containerPath, leafListName, isLeafList := m.resolveLeafListValue(probePath)
	if !isLeafList {
		return commandResult{}, errInsertFailedTargetIsNotA
	}

	// Validate value against the leaf-list's YANG type before applying,
	// the same gate cmdSet applies to scalar leaves.
	if err := m.completer.ValidateValueAtPath(fullPath, value); err != nil {
		return commandResult{}, err
	}

	if err := m.editor.InsertLeafListValue(containerPath, leafListName, value, position, ref); err != nil {
		return commandResult{}, fmt.Errorf("insert failed: %w", err)
	}

	m.refreshCompleter()
	m.searchCache = ""

	var tb textbuf.Buffer
	tb.Str("Inserted ").Str(value).Str(" into ").Str(leafListName).Byte(' ').Str(position)
	if ref != "" {
		tb.Byte(' ').Str(ref)
	}

	if conflicts := m.editor.detectConflicts(); len(conflicts) > 0 {
		tb.Str(" (conflict with ").Str(conflicts[0].OtherUser).Str(" on ").Str(conflicts[0].Path).Byte(')')
	}
	msg := tb.String()

	return commandResult{
		statusMessage: msg,
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}

// cmdRename renames a list entry key, preserving its subtree and position.
// JunOS syntax: rename <list> <old-key> to <new-key>
// Works relative to current context.
//
//nolint:dupl // shares structure with cmdCopy but different operations (rename vs copy)
func (m *Model) cmdRename(args []string) (commandResult, error) {
	// "to" must be second-to-last: <path...> <old-key> to <new-key>
	// Searching from a fixed position avoids ambiguity when a list key is literally "to".
	if len(args) < 4 {
		return commandResult{}, errUsageRenamePathOldNameTo
	}
	toIdx := len(args) - 2
	if args[toIdx] != "to" {
		return commandResult{}, errUsageRenamePathOldNameTo
	}

	newKey := args[toIdx+1]
	oldTokens := args[:toIdx]

	// Build full path to old entry: context + args before "to"
	fullPath := make([]string, 0, len(m.contextPath)+len(oldTokens))
	fullPath = append(fullPath, m.contextPath...)
	fullPath = append(fullPath, oldTokens...)

	// Identify list name, old key, and parent path using schema
	parentPath, listName, oldKey, err := m.editor.resolveListTarget(fullPath)
	if err != nil {
		return commandResult{}, err
	}

	// Validate new key against YANG schema (same validation as set paths).
	newPath := make([]string, 0, len(parentPath)+2)
	newPath = append(newPath, parentPath...)
	newPath = append(newPath, listName, newKey)
	if _, err := m.completer.validateTokenPath(newPath); err != nil {
		return commandResult{}, fmt.Errorf("invalid new name: %w", err)
	}

	// Perform the rename
	if err := m.editor.RenameListEntry(parentPath, listName, oldKey, newKey); err != nil {
		return commandResult{}, fmt.Errorf("rename failed: %w", err)
	}

	// Update completer with mutated tree
	m.refreshCompleter()
	m.searchCache = "" // tree changed, invalidate cached set-view

	var tb8 textbuf.Buffer
	tb8.Str("Renamed ").Str(listName).Byte(' ').Str(oldKey).Str(" to ").Str(newKey)

	// Detect conflicts with other users' change files after each edit.
	if conflicts := m.editor.detectConflicts(); len(conflicts) > 0 {
		tb8.Str(" (conflict with ").Str(conflicts[0].OtherUser).Str(" on ").Str(conflicts[0].Path).Byte(')')
	}
	msg := tb8.String()

	return commandResult{
		statusMessage: msg,
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}

// cmdCopy clones a list entry under a new key, preserving the source.
// JunOS syntax: copy <list> <old-key> to <new-key>
// Works relative to current context.
//
//nolint:dupl // shares structure with cmdRename but different operations (copy vs rename)
func (m *Model) cmdCopy(args []string) (commandResult, error) {
	// "to" must be second-to-last: <path...> <src-key> to <dst-key>
	if len(args) < 4 {
		return commandResult{}, errUsageCopyPathSourceToDestination
	}
	toIdx := len(args) - 2
	if args[toIdx] != "to" {
		return commandResult{}, errUsageCopyPathSourceToDestination
	}

	dstKey := args[toIdx+1]
	srcTokens := args[:toIdx]

	// Build full path to source entry: context + args before "to"
	fullPath := make([]string, 0, len(m.contextPath)+len(srcTokens))
	fullPath = append(fullPath, m.contextPath...)
	fullPath = append(fullPath, srcTokens...)

	// Identify list name, source key, and parent path using schema
	parentPath, listName, srcKey, err := m.editor.resolveListTarget(fullPath)
	if err != nil {
		return commandResult{}, err
	}

	// Validate destination key against YANG schema.
	newPath := make([]string, 0, len(parentPath)+2)
	newPath = append(newPath, parentPath...)
	newPath = append(newPath, listName, dstKey)
	if _, err := m.completer.validateTokenPath(newPath); err != nil {
		return commandResult{}, fmt.Errorf("invalid destination name: %w", err)
	}

	// Perform the copy
	if err := m.editor.CopyListEntry(parentPath, listName, srcKey, dstKey); err != nil {
		return commandResult{}, fmt.Errorf("copy failed: %w", err)
	}

	// Update completer with mutated tree
	m.refreshCompleter()
	m.searchCache = "" // tree changed, invalidate cached set-view

	var tb9 textbuf.Buffer
	tb9.Str("Copied ").Str(listName).Byte(' ').Str(srcKey).Str(" to ").Str(dstKey)

	if conflicts := m.editor.detectConflicts(); len(conflicts) > 0 {
		tb9.Str(" (conflict with ").Str(conflicts[0].OtherUser).Str(" on ").Str(conflicts[0].Path).Byte(')')
	}
	msg := tb9.String()

	return commandResult{
		statusMessage: msg,
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}
