// Design: docs/architecture/config/change-file-structural-ops.md -- dedicated per-user change-file structural ops
// Related: meta.go -- leaf-level metadata (MetaTree)
// Related: serialize_set.go -- set-format serialization (tree + meta)
// Related: setparser.go -- set-format parser

package config

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// ChangeFileCreateEntryToken identifies a list-entry creation structural op line.
	ChangeFileCreateEntryToken = "create-entry"
	// ChangeFileRenameToken identifies a rename structural op line.
	ChangeFileRenameToken = "rename"
	// ChangeFileToToken separates the old and new keys in a rename line.
	ChangeFileToToken = "to"
	// ChangeFileDeleteEntryToken identifies a list-entry delete structural op line.
	ChangeFileDeleteEntryToken = "delete-entry"
	// ChangeFileDeleteContainerToken identifies a container delete structural op line.
	ChangeFileDeleteContainerToken = "delete-container"
	// ChangeFileInsertMemberToken identifies a leaf-list positional insert op line.
	ChangeFileInsertMemberToken = "insert-member"
	// ChangeFileDeactivateMemberToken identifies a leaf-list member deactivation op line.
	ChangeFileDeactivateMemberToken = "deactivate-member"
	// ChangeFileActivateMemberToken identifies a leaf-list member activation op line.
	ChangeFileActivateMemberToken = "activate-member"
	// ChangeFileDeleteListToken identifies a whole-list delete structural op line.
	ChangeFileDeleteListToken = "delete-list"
	// ChangeFileCopyEntryToken identifies a list-entry copy structural op line.
	ChangeFileCopyEntryToken = "copy-entry"
	// ChangeFileDeactivateLeafToken identifies a leaf deactivation op line.
	ChangeFileDeactivateLeafToken = "deactivate-leaf"
	// ChangeFileActivateLeafToken identifies a leaf activation op line.
	ChangeFileActivateLeafToken = "activate-leaf"
	// ChangeFileDeactivatePathToken identifies a container or list-entry deactivation op line.
	ChangeFileDeactivatePathToken = "deactivate-path"
	// ChangeFileActivatePathToken identifies a container or list-entry activation op line.
	ChangeFileActivatePathToken = "activate-path"
)

// StructuralOpType identifies the kind of structural op stored in a change file.
type StructuralOpType string

const (
	// StructuralOpCreateEntry ensures the entry at ParentPath plus ListName exists.
	// The whole token path is retained so schema-aware replay handles nested and
	// multi-key lists without treating an entry address as a leaf.
	StructuralOpCreateEntry StructuralOpType = ChangeFileCreateEntryToken
	// StructuralOpRename renames a single keyed list entry.
	StructuralOpRename StructuralOpType = ChangeFileRenameToken
	// StructuralOpDeleteEntry removes a keyed list entry.
	StructuralOpDeleteEntry StructuralOpType = ChangeFileDeleteEntryToken
	// StructuralOpDeleteContainer removes a container.
	StructuralOpDeleteContainer StructuralOpType = ChangeFileDeleteContainerToken
	// StructuralOpInsertMember inserts a leaf-list member at a position
	// (first/last/before/after). Recorded as a structural op — not a plain
	// add-member metadata entry — so the position is applied exactly at
	// commit time instead of degrading to append.
	StructuralOpInsertMember StructuralOpType = ChangeFileInsertMemberToken
	// StructuralOpDeactivateMember marks one leaf-list member inactive in place.
	StructuralOpDeactivateMember StructuralOpType = ChangeFileDeactivateMemberToken
	// StructuralOpActivateMember clears a member's inactive marker in place.
	StructuralOpActivateMember StructuralOpType = ChangeFileActivateMemberToken
	// StructuralOpDeleteList removes an entire list (all entries).
	StructuralOpDeleteList StructuralOpType = ChangeFileDeleteListToken
	// StructuralOpCopyEntry clones a keyed list entry under a new key. One op,
	// not synthetic set entries, so the operator's intent stays one change.
	StructuralOpCopyEntry StructuralOpType = ChangeFileCopyEntryToken
	// StructuralOpDeactivateLeaf marks a leaf inactive; ListName is the leaf.
	StructuralOpDeactivateLeaf StructuralOpType = ChangeFileDeactivateLeafToken
	// StructuralOpActivateLeaf clears a leaf's inactive marker; ListName is the leaf.
	StructuralOpActivateLeaf StructuralOpType = ChangeFileActivateLeafToken
	// StructuralOpDeactivatePath marks the container or list entry at
	// ParentPath plus ListName inactive.
	StructuralOpDeactivatePath StructuralOpType = ChangeFileDeactivatePathToken
	// StructuralOpActivatePath clears the inactive marker of the container or
	// list entry at ParentPath plus ListName.
	StructuralOpActivatePath StructuralOpType = ChangeFileActivatePathToken
)

// PendingChangeKind identifies the operator-visible type of a pending change.
type PendingChangeKind string

// enumeration: exempt (these are the pending-change kinds a change file records, not the CLI verb vocabulary)
const (
	PendingChangeSet        PendingChangeKind = "set"
	PendingChangeDelete     PendingChangeKind = "delete"
	PendingChangeRename     PendingChangeKind = "rename"
	PendingChangeDeactivate PendingChangeKind = "deactivate"
	PendingChangeActivate   PendingChangeKind = "activate"
	PendingChangeCopy       PendingChangeKind = "copy"
)

// PendingChange is the unified pending-change view used by session diff/count code.
// Leaf changes use Path/Previous/Value. Renames and copies use OldPath/NewPath.
// Member is set for leaf-list member operations (add when Value non-empty,
// remove when Value empty); members of the same leaf-list are independent
// changes, not contested values.
type PendingChange struct {
	SessionID string
	Kind      PendingChangeKind
	Path      string
	Previous  string
	Value     string
	OldPath   string
	NewPath   string
	Member    string
}

// StructuralOp records a structural change in a per-user change file.
// Field use by type: rename and copy-entry use ListName/OldKey/NewKey;
// create-entry uses ParentPath plus ListName as its full token path;
// delete-entry and delete-container use ListName/OldKey; the leaf and path
// toggles use ListName as the last path element; the leaf-list member ops use
// ListName (leaf name) and NewKey (member), with insert-member also using
// Position and OldKey (the before/after reference member).
type StructuralOp struct {
	Type       StructuralOpType
	User       string
	Source     string
	Time       time.Time
	ParentPath EditTokens
	ListName   string
	OldKey     string
	NewKey     string
	Position   string
}

// SessionKey returns the stable per-session identifier for the op.
func (op StructuralOp) SessionKey() string {
	entry := MetaEntry{User: op.User, Source: op.Source, Time: op.Time}
	return entry.SessionKey()
}

// isMemberOp reports whether the op targets one leaf-list member.
func (op StructuralOp) isMemberOp() bool {
	//exhaustive:ignore // Only classifies leaf-list member operations for path construction.
	switch op.Type {
	case StructuralOpInsertMember, StructuralOpDeactivateMember, StructuralOpActivateMember:
		return true
	case StructuralOpRename, StructuralOpDeleteEntry, StructuralOpDeleteContainer, StructuralOpDeleteList,
		StructuralOpCopyEntry, StructuralOpDeactivateLeaf, StructuralOpActivateLeaf,
		StructuralOpDeactivatePath, StructuralOpActivatePath:
		return false
	}
	return false
}

// isToggleOp reports whether the op deactivates or activates one leaf or path.
func (op StructuralOp) isToggleOp() bool {
	//exhaustive:ignore // Only classifies the leaf and path toggles for path construction.
	switch op.Type {
	case StructuralOpDeactivateLeaf, StructuralOpActivateLeaf, StructuralOpDeactivatePath, StructuralOpActivatePath:
		return true
	}
	return false
}

// SourcePath returns the full YANG path to the original list entry.
func (op StructuralOp) SourcePath(schema *Schema) string {
	key := op.OldKey
	if op.isMemberOp() {
		key = op.NewKey
	}
	return op.targetPath(schema, key)
}

// DestinationPath returns the full YANG path to the renamed list entry.
func (op StructuralOp) DestinationPath(schema *Schema) string {
	if op.Type == StructuralOpRename || op.Type == StructuralOpCopyEntry || op.Type == "" {
		return op.targetPath(schema, op.NewKey)
	}
	return op.SourcePath(schema)
}

func (op StructuralOp) targetPath(schema *Schema, key string) string {
	var b textbuf.Buffer
	writeListAddress(&b, op.ParentPath)
	if len(op.ParentPath) != 0 {
		b.Byte(' ')
	}
	b.Str(quoteIfNeeded(op.ListName))
	if op.Type == StructuralOpCreateEntry || op.Type == StructuralOpDeleteContainer ||
		op.Type == StructuralOpDeleteList || op.isToggleOp() {
		return b.String()
	}
	b.Byte(' ')
	if !op.isMemberOp() {
		// The schema expands stored multi-leaf map keys. An unresolved node
		// still renders its stored key intact; path validation, not rendering,
		// is responsible for refusing unknown ownership addresses.
		var parent ChildProvider = schema
		if len(op.ParentPath) != 0 {
			parent, _ = schema.LookupTokenPath(op.ParentPath).(ChildProvider)
		}
		var node *ListNode
		if parent != nil && schema != nil {
			node, _ = parent.Get(op.ListName).(*ListNode)
		}
		if node != nil && node.HasMultiLeafKey() {
			writeListAddress(&b, ListKeyTokens(node, key))
			return b.String()
		}
	}
	b.Str(quoteIfNeeded(key))
	return b.String()
}

// PendingChange converts the structural op into the unified pending-change form.
func (op StructuralOp) PendingChange(schema *Schema) PendingChange {
	switch op.Type {
	case StructuralOpCreateEntry:
		return PendingChange{
			SessionID: op.SessionKey(),
			Kind:      PendingChangeSet,
			Path:      op.SourcePath(schema),
		}
	case StructuralOpDeleteEntry, StructuralOpDeleteContainer, StructuralOpDeleteList:
		return PendingChange{
			SessionID: op.SessionKey(),
			Kind:      PendingChangeDelete,
			Path:      op.SourcePath(schema),
		}
	case StructuralOpInsertMember:
		return PendingChange{
			SessionID: op.SessionKey(),
			Kind:      PendingChangeSet,
			Path:      joinChangePath(op.ParentPath.String(), op.ListName),
			Value:     op.NewKey,
			Member:    op.NewKey,
		}
	case StructuralOpDeactivateMember:
		// Value stays empty: a deactivation conflicts with a concurrent set
		// or activation of the same member (which carry Value=member).
		return PendingChange{
			SessionID: op.SessionKey(),
			Kind:      PendingChangeDeactivate,
			Path:      joinChangePath(op.ParentPath.String(), op.ListName),
			Member:    op.NewKey,
		}
	case StructuralOpActivateMember:
		return PendingChange{
			SessionID: op.SessionKey(),
			Kind:      PendingChangeActivate,
			Path:      joinChangePath(op.ParentPath.String(), op.ListName),
			Value:     op.NewKey,
			Member:    op.NewKey,
		}
	case StructuralOpCopyEntry:
		return PendingChange{
			SessionID: op.SessionKey(),
			Kind:      PendingChangeCopy,
			Path:      op.DestinationPath(schema),
			OldPath:   op.SourcePath(schema),
			NewPath:   op.DestinationPath(schema),
		}
	case StructuralOpDeactivateLeaf, StructuralOpDeactivatePath:
		return PendingChange{
			SessionID: op.SessionKey(),
			Kind:      PendingChangeDeactivate,
			Path:      op.SourcePath(schema),
		}
	case StructuralOpActivateLeaf, StructuralOpActivatePath:
		return PendingChange{
			SessionID: op.SessionKey(),
			Kind:      PendingChangeActivate,
			Path:      op.SourcePath(schema),
		}
	case "", StructuralOpRename:
	default:
		panic("BUG: invalid structural operation")
	}
	return PendingChange{
		SessionID: op.SessionKey(),
		Kind:      PendingChangeRename,
		Path:      op.DestinationPath(schema),
		OldPath:   op.SourcePath(schema),
		NewPath:   op.DestinationPath(schema),
	}
}

// ConflictPaths returns the paths that should participate in overlap checks.
func (pc PendingChange) ConflictPaths() []string {
	switch pc.Kind {
	case PendingChangeRename, PendingChangeCopy:
		return []string{pc.OldPath, pc.NewPath}
	case "", PendingChangeSet, PendingChangeDelete, PendingChangeDeactivate, PendingChangeActivate:
	default:
		panic("BUG: invalid pending change kind")
	}
	if pc.Path == "" {
		return nil
	}
	return []string{pc.Path}
}

// Summary returns a concise human-readable form of the pending change.
//
// It takes the schema because set-style summaries echo Value, which is what
// the operator typed at Path. A schema that marks that leaf ze:sensitive or
// ze:bcrypt makes the summary a display path for a credential, and the
// adoption prompt of `ze config edit` writes one line of it per change. A nil
// schema masks every value, because it cannot tell a credential from a timer.
func (pc PendingChange) Summary(schema *Schema) string {
	var tb textbuf.Buffer
	switch pc.Kind {
	case PendingChangeDelete:
		tb.Str("delete ").Str(pc.Path)
		if pc.Member != "" {
			tb.Byte(' ').Str(pc.Member)
		}
		return tb.String()
	case PendingChangeRename:
		return tb.Str("rename ").Str(pc.OldPath).Str(" to ").Str(pc.NewPath).String()
	case PendingChangeCopy:
		return tb.Str("copy ").Str(pc.OldPath).Str(" to ").Str(pc.NewPath).String()
	case PendingChangeDeactivate, PendingChangeActivate:
		tb.Str(string(pc.Kind)).Byte(' ').Str(pc.Path)
		if pc.Member == "" {
			// A leaf or path toggle carries no value: the path is the change.
			return tb.String()
		}
		// A member toggle names the member it flips. A deactivation carries
		// no Value, so the set form below would print an empty one.
		path, err := ParseEditPath(pc.Path)
		if err != nil {
			return "invalid pending-change path: " + err.Error()
		}
		member := DisplayValueAtPath(schema, path, pc.Member)
		return tb.Byte(' ').Str(member).String()
	case "", PendingChangeSet:
	default:
		panic("BUG: invalid pending change kind")
	}
	if pc.Value == "" {
		return tb.Str("set ").Str(pc.Path).String()
	}
	path, err := ParseEditPath(pc.Path)
	if err != nil {
		return "invalid pending-change path: " + err.Error()
	}
	value := DisplayValueAtPath(schema, path, pc.Value)
	return tb.Str("set ").Str(pc.Path).Byte(' ').Str(value).String()
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
		Path:      se.Path.String(),
		Previous:  se.Entry.Previous,
		Value:     se.Entry.Value,
		Member:    se.Entry.Member,
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
	materializeMetaPaths(tree, meta, parser.schema)
	return tree, meta, ops, nil
}

// materializeMetaPaths creates in the sparse change tree every container and
// list entry that leads to a metadata entry. A delete line removes a value and
// creates no node, while the serializer reaches metadata only by walking the
// tree, so without these nodes the next write-through would drop every pending
// delete under a path no pending set shares. The empty nodes serialize to
// nothing, as the ones writeThroughDelete creates at the first write do.
//
// The schema is walked beside the meta: a list entry is created through the
// codec's one creator with the key leaf pairs its key names, so an entry
// materialized from a multi-leaf key alone holds `interface zbl1` like one
// any other route created. A nil schema (a name the schema does not know)
// creates the entry with no pairs.
//
// A metadata branch that holds no entry creates no node. The recursion is bounded by the YANG schema
// depth the trees mirror.
func materializeMetaPaths(tree *Tree, meta *MetaTree, schema ChildProvider) {
	for name, child := range meta.Containers() {
		childSchema := schemaChild(schema, name)
		if lists := child.Lists(); len(lists) > 0 {
			listNode, _ := childSchema.(*ListNode) // A name that is not a list creates entries with no pairs.
			for key, entryMeta := range lists {
				if !metaHoldsEntry(entryMeta) {
					continue
				}
				addr := ListAddress{Key: key}
				if listNode != nil {
					addr.First, addr.Pairs = SplitListKey(listNode, key)
				}
				materializeMetaPaths(EnsureListEntry(tree, name, addr), entryMeta, providerOf(listNode))
			}
			continue
		}
		if !metaHoldsEntry(child) {
			continue
		}
		materializeMetaPaths(tree.GetOrCreateContainer(name), child, providerOf(childSchema))
	}
}

// schemaChild returns the schema node name names under schema, nil when the
// schema is nil or does not know it.
func schemaChild(schema ChildProvider, name string) Node {
	if schema == nil {
		return nil
	}
	return schema.Get(name)
}

// providerOf returns node as a ChildProvider, nil when it holds no children
// (or is nil): the walk below it then creates nodes with no pairs.
func providerOf(node Node) ChildProvider {
	provider, ok := node.(ChildProvider)
	if !ok {
		return nil
	}
	if listNode, isList := node.(*ListNode); isList && listNode == nil {
		return nil
	}
	return provider
}

// metaHoldsEntry reports whether meta holds an entry at or below it. The
// recursion is bounded by the YANG schema depth the tree mirrors.
func metaHoldsEntry(meta *MetaTree) bool {
	if len(meta.Entries()) > 0 {
		return true
	}
	for _, child := range meta.Containers() {
		if metaHoldsEntry(child) {
			return true
		}
	}
	for _, child := range meta.Lists() {
		if metaHoldsEntry(child) {
			return true
		}
	}
	return false
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

// CoalesceRenameOps collapses same-session rename chains only when no creation
// program needs their intermediate addresses for saved-materialization cleanup
// and metadata rebasing.
func CoalesceRenameOps(ops []StructuralOp) []StructuralOp {
	if len(ops) <= 1 || slices.ContainsFunc(ops, func(op StructuralOp) bool {
		return op.Type == StructuralOpCreateEntry
	}) {
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
			if prev.SessionKey() != ops[i].SessionKey() || !slices.Equal(prev.ParentPath, ops[i].ParentPath) || prev.ListName != ops[i].ListName {
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

// parseKeyMoveLine parses a rename or copy-entry line, the two ops that take
// one keyed list entry to a new key.
// Form: <token> <parent-path> <list-name> <old-key> to <new-key>, each a
// token split by splitQuotedTokens, so a key holding spaces (a multi-leaf
// address) is one quoted token. `to` is read at its fixed place, second to
// last, so a key spelled `to` reads back exact. A record written raw before
// keys were quoted loads as it did.
func parseKeyMoveLine(lineNum int, entry MetaEntry, cmdLine string, opType StructuralOpType) (StructuralOp, error) {
	token := string(opType)
	tokens, err := splitQuotedTokens(cmdLine)
	if err != nil {
		return StructuralOp{}, fmt.Errorf("line %d: %s: %w", lineNum, token, err)
	}
	if len(tokens) < 5 {
		return StructuralOp{}, fmt.Errorf("line %d: %s requires <parent-path> <list-name> <old-key> to <new-key>", lineNum, token)
	}
	if tokens[0] != token {
		return StructuralOp{}, fmt.Errorf("line %d: not a %s line", lineNum, token)
	}
	if entry.User == "" {
		return StructuralOp{}, fmt.Errorf("line %d: %s requires #user metadata", lineNum, token)
	}

	toIdx := len(tokens) - 2
	if tokens[toIdx] != ChangeFileToToken {
		return StructuralOp{}, fmt.Errorf("line %d: %s must end with 'to <new-key>'", lineNum, token)
	}
	if toIdx < 3 {
		return StructuralOp{}, fmt.Errorf("line %d: %s requires list-name and old-key", lineNum, token)
	}

	oldKey := tokens[toIdx-1]
	listName := tokens[toIdx-2]
	newKey := tokens[toIdx+1]
	parentPath := tokens[1 : toIdx-2]
	if newKey == "" { // An empty key arrives only as a quoted `""`.
		return StructuralOp{}, fmt.Errorf("line %d: %s requires a new key", lineNum, token)
	}

	return StructuralOp{
		Type:       opType,
		User:       entry.User,
		Source:     entry.Source,
		Time:       entry.Time,
		ParentPath: parentPath,
		ListName:   listName,
		OldKey:     oldKey,
		NewKey:     newKey,
	}, nil
}

// formatKeyMoveLine serializes a rename or copy-entry op. The zero Type is a
// rename, as it always was for change files written before the type existed.
func formatKeyMoveLine(op StructuralOp) string {
	var b textbuf.Buffer
	writeMetaPrefix(&b, MetaEntry{User: op.User, Source: op.Source, Time: op.Time})
	if op.Type == StructuralOpCopyEntry {
		b.Str(ChangeFileCopyEntryToken)
	} else {
		b.Str(ChangeFileRenameToken)
	}
	b.Byte(' ')
	// Each token goes through quoteIfNeeded, so parseKeyMoveLine splits the
	// record back exactly (splitQuotedTokens is its inverse).
	for _, parent := range op.ParentPath {
		b.Str(quoteIfNeeded(parent))
		b.Byte(' ')
	}
	b.Str(quoteIfNeeded(op.ListName))
	b.Byte(' ')
	b.Str(quoteIfNeeded(op.OldKey))
	b.Byte(' ')
	b.Str(ChangeFileToToken)
	b.Byte(' ')
	b.Str(quoteIfNeeded(op.NewKey))
	return b.String()
}

// parseToggleLine parses a creation or leaf/path deactivate/activate op line.
// Form: <token> <parent-path...> <name>, where name is the leaf for leaf
// toggles and the last path element for creation and path toggles.
func parseToggleLine(lineNum int, entry MetaEntry, cmdLine string, opType StructuralOpType) (StructuralOp, error) {
	tokens, err := ParseEditPath(cmdLine)
	if err != nil {
		return StructuralOp{}, fmt.Errorf("line %d: %w", lineNum, err)
	}
	if len(tokens) < 2 {
		return StructuralOp{}, fmt.Errorf("line %d: %s requires a path", lineNum, opType)
	}
	if entry.User == "" {
		return StructuralOp{}, fmt.Errorf("line %d: %s requires #user metadata", lineNum, opType)
	}
	parentPath := tokens[1 : len(tokens)-1]
	return StructuralOp{
		Type:       opType,
		User:       entry.User,
		Source:     entry.Source,
		Time:       entry.Time,
		ParentPath: parentPath,
		ListName:   tokens[len(tokens)-1],
	}, nil
}

// formatToggleLine serializes a creation or leaf/path deactivate/activate op.
// The op type is its own change-file token.
func formatToggleLine(op StructuralOp) string {
	var b textbuf.Buffer
	writeMetaPrefix(&b, MetaEntry{User: op.User, Source: op.Source, Time: op.Time})
	b.Str(string(op.Type))
	b.Byte(' ')
	if len(op.ParentPath) != 0 {
		writeListAddress(&b, op.ParentPath)
		b.Byte(' ')
	}
	b.Str(quoteIfNeeded(op.ListName))
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
	case ChangeFileCreateEntryToken:
		op, err := parseToggleLine(lineNum, entry, cmdLine, StructuralOpCreateEntry)
		return op, err, true
	case ChangeFileRenameToken:
		op, err := parseKeyMoveLine(lineNum, entry, cmdLine, StructuralOpRename)
		return op, err, true
	case ChangeFileDeleteEntryToken:
		op, err := parseDeleteEntryLine(lineNum, entry, cmdLine)
		return op, err, true
	case ChangeFileDeleteContainerToken:
		op, err := parseDeleteContainerLine(lineNum, entry, cmdLine)
		return op, err, true
	case ChangeFileDeleteListToken:
		op, err := parseDeleteListLine(lineNum, entry, cmdLine)
		return op, err, true
	case ChangeFileInsertMemberToken:
		op, err := parseInsertMemberLine(lineNum, entry, cmdLine)
		return op, err, true
	case ChangeFileDeactivateMemberToken:
		op, err := parseMemberToggleLine(lineNum, entry, cmdLine, StructuralOpDeactivateMember)
		return op, err, true
	case ChangeFileActivateMemberToken:
		op, err := parseMemberToggleLine(lineNum, entry, cmdLine, StructuralOpActivateMember)
		return op, err, true
	case ChangeFileCopyEntryToken:
		op, err := parseKeyMoveLine(lineNum, entry, cmdLine, StructuralOpCopyEntry)
		return op, err, true
	case ChangeFileDeactivateLeafToken:
		op, err := parseToggleLine(lineNum, entry, cmdLine, StructuralOpDeactivateLeaf)
		return op, err, true
	case ChangeFileActivateLeafToken:
		op, err := parseToggleLine(lineNum, entry, cmdLine, StructuralOpActivateLeaf)
		return op, err, true
	case ChangeFileDeactivatePathToken:
		op, err := parseToggleLine(lineNum, entry, cmdLine, StructuralOpDeactivatePath)
		return op, err, true
	case ChangeFileActivatePathToken:
		op, err := parseToggleLine(lineNum, entry, cmdLine, StructuralOpActivatePath)
		return op, err, true
	}
	return StructuralOp{}, nil, false
}

// formatStructuralLine serializes a structural op into its change-file line.
func formatStructuralLine(op StructuralOp) string {
	switch op.Type {
	case StructuralOpCreateEntry:
		return formatToggleLine(op)
	case StructuralOpDeleteEntry:
		return formatDeleteEntryLine(op)
	case StructuralOpDeleteContainer:
		return formatDeleteContainerLine(op)
	case StructuralOpDeleteList:
		return formatDeleteListLine(op)
	case StructuralOpInsertMember:
		return formatInsertMemberLine(op)
	case StructuralOpDeactivateMember, StructuralOpActivateMember:
		return formatMemberToggleLine(op)
	case StructuralOpDeactivateLeaf, StructuralOpActivateLeaf, StructuralOpDeactivatePath, StructuralOpActivatePath:
		return formatToggleLine(op)
	case "", StructuralOpRename, StructuralOpCopyEntry:
		return formatKeyMoveLine(op)
	default:
		panic("BUG: invalid structural operation")
	}
}

// parseInsertMemberLine parses a positional leaf-list insert op line.
// Form: insert-member <parent-path> <leaf> <member> first|last
// or:   insert-member <parent-path> <leaf> <member> before|after <ref>
func parseInsertMemberLine(lineNum int, entry MetaEntry, cmdLine string) (StructuralOp, error) {
	tokens, err := ParseEditPath(cmdLine)
	if err != nil {
		return StructuralOp{}, fmt.Errorf("line %d: insert-member: %w", lineNum, err)
	}
	if entry.User == "" {
		return StructuralOp{}, fmt.Errorf("line %d: %s requires #user metadata", lineNum, ChangeFileInsertMemberToken)
	}

	op := StructuralOp{
		Type:   StructuralOpInsertMember,
		User:   entry.User,
		Source: entry.Source,
		Time:   entry.Time,
	}
	var rest []string
	last := tokens[len(tokens)-1]
	switch {
	case len(tokens) >= 4 && (last == InsertFirst || last == InsertLast):
		op.Position = last
		rest = tokens[1 : len(tokens)-1]
	case len(tokens) >= 5 && (tokens[len(tokens)-2] == InsertBefore || tokens[len(tokens)-2] == InsertAfter):
		op.Position = tokens[len(tokens)-2]
		op.OldKey = last
		rest = tokens[1 : len(tokens)-2]
	default:
		return StructuralOp{}, fmt.Errorf("line %d: %s requires <parent-path> <leaf> <member> first|last|before <ref>|after <ref>", lineNum, ChangeFileInsertMemberToken)
	}
	if len(rest) < 2 {
		return StructuralOp{}, fmt.Errorf("line %d: %s requires a leaf name and a member", lineNum, ChangeFileInsertMemberToken)
	}
	op.NewKey = rest[len(rest)-1]
	op.ListName = rest[len(rest)-2]
	op.ParentPath = rest[:len(rest)-2]
	return op, nil
}

// parseMemberToggleLine parses a deactivate-member or activate-member op line.
// Form: <token> <parent-path> <leaf> <member>.
func parseMemberToggleLine(lineNum int, entry MetaEntry, cmdLine string, opType StructuralOpType) (StructuralOp, error) {
	tokens, err := ParseEditPath(cmdLine)
	if err != nil {
		return StructuralOp{}, fmt.Errorf("line %d: %s: %w", lineNum, opType, err)
	}
	if len(tokens) < 3 {
		return StructuralOp{}, fmt.Errorf("line %d: %s requires <leaf> <member>", lineNum, tokens[0])
	}
	if entry.User == "" {
		return StructuralOp{}, fmt.Errorf("line %d: %s requires #user metadata", lineNum, tokens[0])
	}
	return StructuralOp{
		Type:       opType,
		User:       entry.User,
		Source:     entry.Source,
		Time:       entry.Time,
		ParentPath: tokens[1 : len(tokens)-2],
		ListName:   tokens[len(tokens)-2],
		NewKey:     tokens[len(tokens)-1],
	}, nil
}

func formatInsertMemberLine(op StructuralOp) string {
	var b textbuf.Buffer
	writeMetaPrefix(&b, MetaEntry{User: op.User, Source: op.Source, Time: op.Time})
	b.Str(ChangeFileInsertMemberToken)
	b.Byte(' ')
	if len(op.ParentPath) != 0 {
		writeListAddress(&b, op.ParentPath)
		b.Byte(' ')
	}
	b.Str(op.ListName)
	b.Byte(' ')
	b.Str(quoteIfNeeded(op.NewKey))
	b.Byte(' ')
	b.Str(op.Position)
	if op.OldKey != "" {
		b.Byte(' ')
		b.Str(quoteIfNeeded(op.OldKey))
	}
	return b.String()
}

func formatMemberToggleLine(op StructuralOp) string {
	var b textbuf.Buffer
	writeMetaPrefix(&b, MetaEntry{User: op.User, Source: op.Source, Time: op.Time})
	if op.Type == StructuralOpActivateMember {
		b.Str(ChangeFileActivateMemberToken)
	} else {
		b.Str(ChangeFileDeactivateMemberToken)
	}
	b.Byte(' ')
	if len(op.ParentPath) != 0 {
		writeListAddress(&b, op.ParentPath)
		b.Byte(' ')
	}
	b.Str(op.ListName)
	b.Byte(' ')
	b.Str(quoteIfNeeded(op.NewKey))
	return b.String()
}

// parseDeleteEntryLine parses a delete-entry structural op line.
func parseDeleteEntryLine(lineNum int, entry MetaEntry, cmdLine string) (StructuralOp, error) {
	tokens, err := ParseEditPath(cmdLine)
	if err != nil {
		return StructuralOp{}, fmt.Errorf("line %d: delete-entry: %w", lineNum, err)
	}
	if len(tokens) < 3 {
		return StructuralOp{}, fmt.Errorf("line %d: delete-entry requires <list-name> <key>", lineNum)
	}
	if entry.User == "" {
		return StructuralOp{}, fmt.Errorf("line %d: delete-entry requires #user metadata", lineNum)
	}
	key := tokens[len(tokens)-1]
	listName := tokens[len(tokens)-2]
	parentPath := tokens[1 : len(tokens)-2]
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
	tokens, err := ParseEditPath(cmdLine)
	if err != nil {
		return StructuralOp{}, fmt.Errorf("line %d: delete-container: %w", lineNum, err)
	}
	if len(tokens) < 2 {
		return StructuralOp{}, fmt.Errorf("line %d: delete-container requires <container-name>", lineNum)
	}
	if entry.User == "" {
		return StructuralOp{}, fmt.Errorf("line %d: delete-container requires #user metadata", lineNum)
	}
	containerName := tokens[len(tokens)-1]
	parentPath := tokens[1 : len(tokens)-1]
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
	if len(op.ParentPath) != 0 {
		writeListAddress(&b, op.ParentPath)
		b.Byte(' ')
	}
	b.Str(op.ListName)
	b.Byte(' ')
	b.Str(quoteIfNeeded(op.OldKey))
	return b.String()
}

func formatDeleteContainerLine(op StructuralOp) string {
	var b textbuf.Buffer
	writeMetaPrefix(&b, MetaEntry{User: op.User, Source: op.Source, Time: op.Time})
	b.Str(ChangeFileDeleteContainerToken)
	b.Byte(' ')
	if len(op.ParentPath) != 0 {
		writeListAddress(&b, op.ParentPath)
		b.Byte(' ')
	}
	b.Str(op.ListName)
	return b.String()
}

func parseDeleteListLine(lineNum int, entry MetaEntry, cmdLine string) (StructuralOp, error) {
	tokens, err := ParseEditPath(cmdLine)
	if err != nil {
		return StructuralOp{}, fmt.Errorf("line %d: delete-list: %w", lineNum, err)
	}
	if len(tokens) < 2 {
		return StructuralOp{}, fmt.Errorf("line %d: delete-list requires <list-name>", lineNum)
	}
	if entry.User == "" {
		return StructuralOp{}, fmt.Errorf("line %d: delete-list requires #user metadata", lineNum)
	}
	listName := tokens[len(tokens)-1]
	parentPath := tokens[1 : len(tokens)-1]
	return StructuralOp{
		Type:       StructuralOpDeleteList,
		User:       entry.User,
		Source:     entry.Source,
		Time:       entry.Time,
		ParentPath: parentPath,
		ListName:   listName,
	}, nil
}

func formatDeleteListLine(op StructuralOp) string {
	var b textbuf.Buffer
	writeMetaPrefix(&b, MetaEntry{User: op.User, Source: op.Source, Time: op.Time})
	b.Str(ChangeFileDeleteListToken)
	b.Byte(' ')
	if len(op.ParentPath) != 0 {
		writeListAddress(&b, op.ParentPath)
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
