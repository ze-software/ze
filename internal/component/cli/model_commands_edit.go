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

	// The address parse runs over every token before the value is split
	// off: `... fe80::b interface zbl1` ends at the address of an entry of a
	// multi-leaf-key list, so it creates that entry rather than setting the
	// key leaf `interface` on entry `fe80::b`, and a key leaf given twice is
	// refused before anything changes.
	entry, endsAtEntry, err := config.WalkListEntryPath(m.editor.schema, fullPath)
	if err != nil {
		return commandResult{}, fmt.Errorf("set failed: %w", err)
	}
	if endsAtEntry && entry.End == len(fullPath) && len(entry.Address.Pairs) > 0 {
		return m.createListEntry(entry, fullPath[len(entry.ParentPath)+1:])
	}

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
		if err := m.editor.ensureListEntry(listParent, listName, []string{value}); err != nil {
			return commandResult{}, fmt.Errorf("set failed: %w", err)
		}
	} else {
		// Validate the full token path (with list keys) against schema.
		if err := m.completer.validateTokenPath(path); err != nil {
			return commandResult{}, err
		}
		// Validate value against YANG type before applying. A refusal names
		// what it refused, so the message carries the value the operator typed.
		if err := m.completer.ValidateValueAtPath(path, value); err != nil {
			return commandResult{}, errors.New(m.editor.DisplayMessageAtPath(path, err.Error(), value))
		}
		if err := m.editor.SetValue(containerPath, key, value); err != nil {
			return commandResult{}, fmt.Errorf("set failed: %w", err)
		}
	}

	// Update completer with mutated tree
	m.refreshCompleter()

	var tb textbuf.Buffer
	if isListKey {
		// A list key names an entry rather than holding a value, so no schema
		// leaf can mark it as a secret.
		tb.Str("created ").Str(containerPath[len(containerPath)-1]).Byte(' ').Str(value)
	} else {
		displayPath := append(append([]string{}, containerPath...), key)
		// The status line echoes what the operator typed. A credential typed
		// here would sit in the scrollback of a shared terminal.
		tb.Str("set ").Join(displayPath, " ").Byte(' ').Str(m.editor.DisplayValueAtPath(path, value))
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

// createListEntry creates the entry a `set` path names by its full address
// (a multi-leaf key's first value and pairs) and reports it.
func (m *Model) createListEntry(entry config.ListEntryTarget, address []string) (commandResult, error) {
	if err := m.editor.ensureListEntry(entry.ParentPath, entry.ListName, address); err != nil {
		return commandResult{}, fmt.Errorf("set failed: %w", err)
	}
	m.refreshCompleter()
	var tb textbuf.Buffer
	tb.Str("created ").Str(entry.ListName).Byte(' ').Join(address, " ")
	if conflicts := m.editor.detectConflicts(); len(conflicts) > 0 {
		tb.Str(" (conflict with ").Str(conflicts[0].OtherUser).Str(" on ").Str(conflicts[0].Path).Byte(')')
	}
	return commandResult{
		statusMessage: tb.String(),
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
// (activate=true). The schema check and the operator's context are the
// model's; the dispatch and the wording are Editor.ApplyActivation, which the
// web terminal runs too.
func (m *Model) runActivation(args []string, activate bool) (commandResult, error) {
	verb := "deactivate"
	if activate {
		verb = "activate"
	}
	if len(args) < 1 {
		return commandResult{}, fmt.Errorf("usage: %s <path>", verb)
	}

	fullPath := make([]string, 0, len(m.contextPath)+len(args))
	fullPath = append(fullPath, m.contextPath...)
	fullPath = append(fullPath, args...)

	// A leaf-list value is no schema node, so only the other paths are checked.
	if _, _, isLeafList := m.resolveLeafListValue(fullPath); !isLeafList {
		if err := m.completer.validateTokenPath(fullPath); err != nil {
			return commandResult{}, err
		}
	}
	msg, err := m.editor.ApplyActivation(fullPath, activate)
	if err != nil {
		return commandResult{}, err
	}
	m.refreshCompleter()
	var tb textbuf.Buffer
	tb.Str(msg)
	if conflicts := m.editor.detectConflicts(); len(conflicts) > 0 {
		tb.Str(" (conflict with ").Str(conflicts[0].OtherUser).Str(" on ").Str(conflicts[0].Path).Byte(')')
	}
	return commandResult{
		statusMessage: tb.String(),
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
		return commandResult{}, errors.New(m.editor.DisplayMessageAtPath(fullPath, err.Error(), value))
	}

	if err := m.editor.InsertLeafListValue(containerPath, leafListName, value, position, ref); err != nil {
		return commandResult{}, fmt.Errorf("insert failed: %w", err)
	}

	m.refreshCompleter()
	m.searchCache = ""

	var tb textbuf.Buffer
	tb.Str("Inserted ").Str(m.editor.DisplayValueAtPath(fullPath, value)).Str(" into ").Str(leafListName).Byte(' ').Str(position)
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
	return m.listEntryMove(args, listEntryMoveRename)
}

// cmdCopy clones a list entry under a new key, preserving the source.
// JunOS syntax: copy <list> <old-key> to <new-key>
// Works relative to current context.
func (m *Model) cmdCopy(args []string) (commandResult, error) {
	return m.listEntryMove(args, listEntryMoveCopy)
}

// listEntryMoveVerb is what differs between rename and copy.
type listEntryMoveVerb struct {
	name  string // the command, in a refusal
	done  string // the status line's verb
	usage error
	apply func(e *Editor, parentPath []string, listName, key string, dst []string) error
}

var (
	listEntryMoveRename = listEntryMoveVerb{
		name: "rename", done: "Renamed", usage: errUsageRenamePathOldNameTo,
		apply: (*Editor).RenameListEntry,
	}
	listEntryMoveCopy = listEntryMoveVerb{
		name: "copy", done: "Copied", usage: errUsageCopyPathSourceToDestination,
		apply: (*Editor).CopyListEntry,
	}
)

// listEntryMove runs `rename` or `copy`: <path...> <source address> to
// <destination address>. The source address is read by the codec's one
// target walk, which reports where it ends; `to` must stand there, and the
// tokens after it are the destination address. Reading `to` at the end of
// the source, never by searching for it, keeps a key spelled `to` exact.
func (m *Model) listEntryMove(args []string, verb listEntryMoveVerb) (commandResult, error) {
	fullPath := make([]string, 0, len(m.contextPath)+len(args))
	fullPath = append(fullPath, m.contextPath...)
	fullPath = append(fullPath, args...)

	target, err := m.editor.resolveListTarget(verb.name, fullPath)
	if err != nil {
		return commandResult{}, err
	}
	if target.End+1 >= len(fullPath) {
		return commandResult{}, verb.usage
	}
	if fullPath[target.End] != "to" {
		return commandResult{}, verb.usage
	}
	dst := fullPath[target.End+1:]

	// Validate the destination against the YANG schema (same validation as
	// set paths).
	newPath := make([]string, 0, len(target.ParentPath)+1+len(dst))
	newPath = append(newPath, target.ParentPath...)
	newPath = append(newPath, target.ListName)
	newPath = append(newPath, dst...)
	if err := m.completer.validateTokenPath(newPath); err != nil {
		return commandResult{}, fmt.Errorf("invalid destination: %w", err)
	}

	if err := verb.apply(m.editor, target.ParentPath, target.ListName, target.Address.Key, dst); err != nil {
		return commandResult{}, fmt.Errorf("%s failed: %w", verb.name, err)
	}

	m.refreshCompleter()
	m.searchCache = "" // tree changed, invalidate cached set-view

	var tb textbuf.Buffer
	tb.Str(verb.done).Byte(' ').Str(target.ListName).Byte(' ').
		Join(fullPath[len(target.ParentPath)+1:target.End], " ").Str(" to ").Join(dst, " ")
	if conflicts := m.editor.detectConflicts(); len(conflicts) > 0 {
		tb.Str(" (conflict with ").Str(conflicts[0].OtherUser).Str(" on ").Str(conflicts[0].Path).Byte(')')
	}
	return commandResult{
		statusMessage: tb.String(),
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}
