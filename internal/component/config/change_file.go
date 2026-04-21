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
)

const (
	// ChangeFileRenameToken identifies a rename structural op line.
	ChangeFileRenameToken = "rename"
	// ChangeFileToToken separates the old and new keys in a rename line.
	ChangeFileToToken = "to"
)

// StructuralOpType identifies the kind of structural op stored in a change file.
type StructuralOpType string

const (
	// StructuralOpRename renames a single keyed list entry.
	StructuralOpRename StructuralOpType = ChangeFileRenameToken
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
	return joinChangePath(op.ParentPath, op.ListName, op.OldKey)
}

// DestinationPath returns the full YANG path to the renamed list entry.
func (op StructuralOp) DestinationPath() string {
	return joinChangePath(op.ParentPath, op.ListName, op.NewKey)
}

// PendingChange converts the structural op into the unified pending-change form.
func (op StructuralOp) PendingChange() PendingChange {
	return PendingChange{
		SessionID: op.SessionKey(),
		Kind:      PendingChangeRename,
		Path:      op.DestinationPath(),
		OldPath:   op.SourcePath(),
		NewPath:   op.DestinationPath(),
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
	switch pc.Kind {
	case PendingChangeDelete:
		return "delete " + pc.Path
	case PendingChangeRename:
		return "rename " + pc.OldPath + " to " + pc.NewPath
	default:
		return "set " + pc.Path + " " + pc.Value
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

	for lineNum, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			configLines = append(configLines, line)
			continue
		}

		entry, cmdLine := extractMeta(trimmed)
		if strings.HasPrefix(cmdLine, ChangeFileRenameToken+" ") {
			op, err := parseRenameLine(lineNum+1, entry, cmdLine)
			if err != nil {
				return nil, nil, nil, err
			}
			ops = append(ops, op)
			continue
		}

		configLines = append(configLines, line)
	}

	tree, meta, err := parser.ParseWithMeta(strings.Join(configLines, "\n"))
	if err != nil {
		return nil, nil, nil, err
	}
	return tree, meta, ops, nil
}

// SerializeChangeFile renders tree, meta, and structural ops into a per-user
// change file. Rename lines are emitted before the set/delete body.
func SerializeChangeFile(tree *Tree, meta *MetaTree, ops []StructuralOp, schema *Schema) string {
	var b strings.Builder
	for i := range ops {
		b.WriteString(formatRenameLine(ops[i]))
		b.WriteByte('\n')
	}
	body := SerializeSetWithMeta(tree, meta, schema)
	if body != "" {
		b.WriteString(body)
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
		if result[i].OldKey == result[i].NewKey {
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
	parentPath := strings.Join(tokens[1:toIdx-2], " ")
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
	var b strings.Builder
	writeMetaPrefix(&b, MetaEntry{User: op.User, Source: op.Source, Time: op.Time})
	b.WriteString(ChangeFileRenameToken)
	b.WriteByte(' ')
	if op.ParentPath != "" {
		b.WriteString(op.ParentPath)
		b.WriteByte(' ')
	}
	b.WriteString(op.ListName)
	b.WriteByte(' ')
	b.WriteString(op.OldKey)
	b.WriteByte(' ')
	b.WriteString(ChangeFileToToken)
	b.WriteByte(' ')
	b.WriteString(op.NewKey)
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
	return strings.Join(parts, " ")
}
