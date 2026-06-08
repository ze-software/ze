// Design: plan/spec-op-2-surface-parity.md -- dedicated per-user change-file structural ops
// Related: meta.go -- leaf-level metadata (MetaTree)
// Related: serialize_set.go -- set-format serialization (tree + meta)
// Related: setparser.go -- set-format parser

package config

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

const (
	// ChangeFileRenameToken identifies a rename structural op line.
	ChangeFileRenameToken = "rename"
	// ChangeFileToToken separates the old and new keys in a rename line.
	ChangeFileToToken = "to"
	// ChangeFileDeleteEntryToken identifies a list-entry delete structural op line.
	ChangeFileDeleteEntryToken = "delete-entry"
	// ChangeFileDeleteContainerToken identifies a container delete structural op line.
	ChangeFileDeleteContainerToken = "delete-container"
)

// StructuralOpType identifies the kind of structural op stored in a change file.
type StructuralOpType string

const (
	// StructuralOpRename renames a single keyed list entry.
	StructuralOpRename StructuralOpType = ChangeFileRenameToken
	// StructuralOpDeleteEntry removes a keyed list entry.
	StructuralOpDeleteEntry StructuralOpType = ChangeFileDeleteEntryToken
	// StructuralOpDeleteContainer removes a container.
	StructuralOpDeleteContainer StructuralOpType = ChangeFileDeleteContainerToken
)

// PendingChangeKind identifies the operator-visible type of a pending change.
type PendingChangeKind string

const (
	PendingChangeSet    PendingChangeKind = "set"
	PendingChangeDelete PendingChangeKind = "delete"
	PendingChangeRename PendingChangeKind = "rename"
)

// PendingChange is the unified pending-change view used by session diff/count code.
// Leaf changes use Path/Previous/Value. Renames use OldPath/NewPath.
type PendingChange struct {
	SessionID string
	Kind      PendingChangeKind
	Path      string
	Previous  string
	Value     string
	OldPath   string
	NewPath   string
}

// StructuralOp records a structural change in a per-user change file.
type StructuralOp struct {
	Type       StructuralOpType
	User       string
	Source     string
	Time       time.Time
	ParentPath string
	ListName   string
	OldKey     string
	NewKey     string
}

// SessionKey returns the stable per-session identifier for the op.
func (op StructuralOp) SessionKey() string {
	entry := MetaEntry{User: op.User, Source: op.Source, Time: op.Time}
	return entry.SessionKey()
}

// SourcePath returns the full YANG path to the original list entry.
func (op StructuralOp) SourcePath() string {
	if op.Type == StructuralOpDeleteContainer {
		return joinChangePath(op.ParentPath, op.ListName)
	}
	return joinChangePath(op.ParentPath, op.ListName, op.OldKey)
}

// DestinationPath returns the full YANG path to the renamed list entry.
func (op StructuralOp) DestinationPath() string {
	if op.Type == StructuralOpDeleteEntry || op.Type == StructuralOpDeleteContainer {
		return op.SourcePath()
	}
	return joinChangePath(op.ParentPath, op.ListName, op.NewKey)
}

// PendingChange converts the structural op into the unified pending-change form.
func (op StructuralOp) PendingChange() PendingChange {
	switch op.Type {
	case StructuralOpDeleteEntry, StructuralOpDeleteContainer:
		return PendingChange{
			SessionID: op.SessionKey(),
			Kind:      PendingChangeDelete,
			Path:      op.SourcePath(),
		}
	default:
		return PendingChange{
			SessionID: op.SessionKey(),
			Kind:      PendingChangeRename,
			Path:      op.DestinationPath(),
			OldPath:   op.SourcePath(),
			NewPath:   op.DestinationPath(),
		}
	}
}

// ConflictPaths returns the paths that should participate in overlap checks.
func (pc PendingChange) ConflictPaths() []string {
	switch pc.Kind {
	case PendingChangeRename:
		return []string{pc.OldPath, pc.NewPath}
	default:
		if pc.Path == "" {
			return nil
		}
		return []string{pc.Path}
	}
}

// Summary returns a concise human-readable form of the pending change.
func (pc PendingChange) Summary() string {
	var tb textbuf.Buffer
	switch pc.Kind {
	case PendingChangeDelete:
		return tb.Str("delete ").Str(pc.Path).String()
	case PendingChangeRename:
		return tb.Str("rename ").Str(pc.OldPath).Str(" to ").Str(pc.NewPath).String()
	default:
		return tb.Str("set ").Str(pc.Path).Byte(' ').Str(pc.Value).String()
	}
}

// PendingChangeFromSessionEntry converts a leaf-level metadata entry into the
// unified pending-change representation.
func PendingChangeFromSessionEntry(se SessionEntry) PendingChange {
	kind := PendingChangeSet
	if se.Entry.Value == "" {
		kind = PendingChangeDelete
	}
	return PendingChange{
		SessionID: se.Entry.SessionKey(),
		Kind:      kind,
		Path:      se.Path,
		Previous:  se.Entry.Previous,
		Value:     se.Entry.Value,
	}
}

// SortPendingChanges orders pending changes for stable diffs and tests.
func SortPendingChanges(changes []PendingChange) {
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].SessionID != changes[j].SessionID {
			return changes[i].SessionID < changes[j].SessionID
		}
		if changes[i].Kind != changes[j].Kind {
			return changes[i].Kind < changes[j].Kind
		}
		if changes[i].OldPath != changes[j].OldPath {
			return changes[i].OldPath < changes[j].OldPath
		}
		if changes[i].NewPath != changes[j].NewPath {
			return changes[i].NewPath < changes[j].NewPath
		}
		return changes[i].Path < changes[j].Path
	})
}

// ParseChangeFile parses a per-user change file into tree, meta, and structural ops.
// Rename directives are validated strictly; malformed rename lines return an error.
func ParseChangeFile(content string, parser *SetParser) (*Tree, *MetaTree, []StructuralOp, error) {
	var (
		ops         []StructuralOp
		configLines []string
	)

	lineNum := 0
	for line := range strings.SplitSeq(content, "\n") {
		lineNum++
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			configLines = append(configLines, line)
			continue
		}

		entry, cmdLine := extractMeta(trimmed)
		if op, err, matched := parseStructuralOp(lineNum, entry, cmdLine); matched {
			if err != nil {
				return nil, nil, nil, err
			}
			ops = append(ops, op)
			continue
		}

		configLines = append(configLines, line)
	}

	tree, meta, err := parser.ParseWithMeta(textbuf.Join(configLines, "\n"))
	if err != nil {
		return nil, nil, nil, err
	}
	return tree, meta, ops, nil
}

// SerializeChangeFile renders tree, meta, and structural ops into a per-user
// change file. Structural op lines are emitted before the set/delete body.
func SerializeChangeFile(tree *Tree, meta *MetaTree, ops []StructuralOp, schema *Schema) string {
	var b textbuf.Buffer
	for i := range ops {
		b.Str(formatStructuralLine(ops[i]))
		b.Byte('\n')
	}
	body := SerializeSetWithMeta(tree, meta, schema)
	if body != "" {
		b.Str(body)
	}
	return b.String()
}

// CoalesceRenameOps collapses same-session rename chains into their effective rename.
func CoalesceRenameOps(ops []StructuralOp) []StructuralOp {
	if len(ops) <= 1 {
		return ops
	}

	result := make([]StructuralOp, 0, len(ops))
	for i := range ops {
		merged := false
		for j := range result {
			prev := &result[j]
			if prev.Type != StructuralOpRename || ops[i].Type != StructuralOpRename {
				continue
			}
			if prev.SessionKey() != ops[i].SessionKey() || prev.ParentPath != ops[i].ParentPath || prev.ListName != ops[i].ListName {
				continue
			}
			if prev.NewKey != ops[i].OldKey {
				continue
			}
			prev.NewKey = ops[i].NewKey
			merged = true
			break
		}
		if !merged {
			result = append(result, ops[i])
		}
	}

	filtered := result[:0]
	for i := range result {
		if result[i].Type == StructuralOpRename && result[i].OldKey == result[i].NewKey {
			continue
		}
		filtered = append(filtered, result[i])
	}
	return filtered
}

func parseRenameLine(lineNum int, entry MetaEntry, cmdLine string) (StructuralOp, error) {
	tokens := strings.Fields(cmdLine)
	if len(tokens) < 5 {
		return StructuralOp{}, fmt.Errorf("line %d: rename requires <parent-path> <list-name> <old-key> to <new-key>", lineNum)
	}
	if tokens[0] != ChangeFileRenameToken {
		return StructuralOp{}, fmt.Errorf("line %d: not a rename line", lineNum)
	}
	if entry.User == "" {
		return StructuralOp{}, fmt.Errorf("line %d: rename requires #user metadata", lineNum)
	}

	toIdx := -1
	for i := 1; i < len(tokens); i++ {
		if tokens[i] == ChangeFileToToken {
			toIdx = i
			break
		}
	}
	if toIdx == -1 || toIdx != len(tokens)-2 {
		return StructuralOp{}, fmt.Errorf("line %d: rename must end with 'to <new-key>'", lineNum)
	}
	if toIdx < 3 {
		return StructuralOp{}, fmt.Errorf("line %d: rename requires list-name and old-key", lineNum)
	}

	oldKey := tokens[toIdx-1]
	listName := tokens[toIdx-2]
	newKey := tokens[toIdx+1]
	parentPath := textbuf.Join(tokens[1:toIdx-2], " ")
	if newKey == "" {
		return StructuralOp{}, fmt.Errorf("line %d: rename requires a new key", lineNum)
	}

	return StructuralOp{
		Type:       StructuralOpRename,
		User:       entry.User,
		Source:     entry.Source,
		Time:       entry.Time,
		ParentPath: parentPath,
		ListName:   listName,
		OldKey:     oldKey,
		NewKey:     newKey,
	}, nil
}

func formatRenameLine(op StructuralOp) string {
	var b textbuf.Buffer
	writeMetaPrefix(&b, MetaEntry{User: op.User, Source: op.Source, Time: op.Time})
	b.Str(ChangeFileRenameToken)
	b.Byte(' ')
	if op.ParentPath != "" {
		b.Str(op.ParentPath)
		b.Byte(' ')
	}
	b.Str(op.ListName)
	b.Byte(' ')
	b.Str(op.OldKey)
	b.Byte(' ')
	b.Str(ChangeFileToToken)
	b.Byte(' ')
	b.Str(op.NewKey)
	return b.String()
}

// parseStructuralOp dispatches structural op parsing by the first token.
// Returns (op, nil, true) on success, (_, err, true) on parse error,
// or (_, nil, false) if the line is not a structural op.
func parseStructuralOp(lineNum int, entry MetaEntry, cmdLine string) (StructuralOp, error, bool) {
	token, _, found := strings.Cut(cmdLine, " ")
	if !found {
		return StructuralOp{}, nil, false
	}
	switch token {
	case ChangeFileRenameToken:
		op, err := parseRenameLine(lineNum, entry, cmdLine)
		return op, err, true
	case ChangeFileDeleteEntryToken:
		op, err := parseDeleteEntryLine(lineNum, entry, cmdLine)
		return op, err, true
	case ChangeFileDeleteContainerToken:
		op, err := parseDeleteContainerLine(lineNum, entry, cmdLine)
		return op, err, true
	}
	return StructuralOp{}, nil, false
}

// formatStructuralLine serializes a structural op into its change-file line.
func formatStructuralLine(op StructuralOp) string {
	switch op.Type {
	case StructuralOpDeleteEntry:
		return formatDeleteEntryLine(op)
	case StructuralOpDeleteContainer:
		return formatDeleteContainerLine(op)
	default:
		return formatRenameLine(op)
	}
}

// parseDeleteEntryLine parses a delete-entry structural op line.
func parseDeleteEntryLine(lineNum int, entry MetaEntry, cmdLine string) (StructuralOp, error) {
	tokens := strings.Fields(cmdLine)
	if len(tokens) < 3 {
		return StructuralOp{}, fmt.Errorf("line %d: delete-entry requires <list-name> <key>", lineNum)
	}
	if entry.User == "" {
		return StructuralOp{}, fmt.Errorf("line %d: delete-entry requires #user metadata", lineNum)
	}
	key := tokens[len(tokens)-1]
	listName := tokens[len(tokens)-2]
	parentPath := textbuf.Join(tokens[1:len(tokens)-2], " ")
	return StructuralOp{
		Type:       StructuralOpDeleteEntry,
		User:       entry.User,
		Source:     entry.Source,
		Time:       entry.Time,
		ParentPath: parentPath,
		ListName:   listName,
		OldKey:     key,
	}, nil
}

// parseDeleteContainerLine parses a delete-container structural op line.
func parseDeleteContainerLine(lineNum int, entry MetaEntry, cmdLine string) (StructuralOp, error) {
	tokens := strings.Fields(cmdLine)
	if len(tokens) < 2 {
		return StructuralOp{}, fmt.Errorf("line %d: delete-container requires <container-name>", lineNum)
	}
	if entry.User == "" {
		return StructuralOp{}, fmt.Errorf("line %d: delete-container requires #user metadata", lineNum)
	}
	containerName := tokens[len(tokens)-1]
	parentPath := textbuf.Join(tokens[1:len(tokens)-1], " ")
	return StructuralOp{
		Type:       StructuralOpDeleteContainer,
		User:       entry.User,
		Source:     entry.Source,
		Time:       entry.Time,
		ParentPath: parentPath,
		ListName:   containerName,
	}, nil
}

func formatDeleteEntryLine(op StructuralOp) string {
	var b textbuf.Buffer
	writeMetaPrefix(&b, MetaEntry{User: op.User, Source: op.Source, Time: op.Time})
	b.Str(ChangeFileDeleteEntryToken)
	b.Byte(' ')
	if op.ParentPath != "" {
		b.Str(op.ParentPath)
		b.Byte(' ')
	}
	b.Str(op.ListName)
	b.Byte(' ')
	b.Str(op.OldKey)
	return b.String()
}

func formatDeleteContainerLine(op StructuralOp) string {
	var b textbuf.Buffer
	writeMetaPrefix(&b, MetaEntry{User: op.User, Source: op.Source, Time: op.Time})
	b.Str(ChangeFileDeleteContainerToken)
	b.Byte(' ')
	if op.ParentPath != "" {
		b.Str(op.ParentPath)
		b.Byte(' ')
	}
	b.Str(op.ListName)
	return b.String()
}

func joinChangePath(parentPath string, elems ...string) string {
	parts := make([]string, 0, len(elems)+1)
	if parentPath != "" {
		parts = append(parts, parentPath)
	}
	for _, elem := range elems {
		if elem != "" {
			parts = append(parts, elem)
		}
	}
	return textbuf.Join(parts, " ")
}
