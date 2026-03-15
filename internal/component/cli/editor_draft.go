// Design: docs/architecture/config/yang-config-design.md — write-through draft protocol
// Overview: editor.go — config editor (calls write-through from SetValue/DeleteValue)
// Related: editor_session.go — session identity for concurrent editing

package cli

import (
	"fmt"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/migration"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

// ConflictType identifies whether a conflict is a live disagreement or stale previous.
type ConflictType int

const (
	// ConflictLive means another active session has a pending change at the same path with a different value.
	ConflictLive ConflictType = iota
	// ConflictStale means the committed value changed since this session's edit.
	ConflictStale
)

// Conflict describes a single conflict detected during commit.
type Conflict struct {
	Path          string       // YANG path (space-separated).
	Type          ConflictType // Live disagreement or stale previous.
	MyValue       string       // Value this session wants to set.
	OtherValue    string       // Conflicting value (committed or other session).
	OtherUser     string       // Who made the conflicting change.
	PreviousValue string       // What config.conf had when this session edited.
}

// CommitResult holds the outcome of a CommitSession attempt.
type CommitResult struct {
	Conflicts        []Conflict // Non-empty if commit was blocked.
	Applied          int        // Number of changes applied (0 if conflicts).
	MigrationWarning string     // Non-empty if tree structure migration failed (format conversion still applied).
}

// writeThroughSet implements the write-through protocol for set commands.
// Steps: lock -> read draft -> parse -> apply set -> record metadata -> serialize -> write draft -> unlock.
func (e *Editor) writeThroughSet(path []string, key, value string) error {
	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return fmt.Errorf("write-through lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths

	// Read draft (or config.conf if no draft exists).
	draftPath := DraftPath(e.originalPath)
	tree, meta, err := e.readDraftOrConfig(guard, draftPath)
	if err != nil {
		return fmt.Errorf("write-through read: %w", err)
	}

	// Apply the set to the draft tree.
	target, err := e.walkOrCreateIn(tree, path)
	if err != nil {
		return err
	}
	target.Set(key, value)

	// Build the YANG path for metadata recording.
	metaPath := append(path, key) //nolint:gocritic // intentional new slice

	// Read committed value for Previous field (re-read under lock
	// to capture external commits; reuses getValueAtPath for navigation).
	previous := ""
	if committedTree := e.readCommittedTree(guard); committedTree != nil {
		previous = getValueAtPath(committedTree, e.schema, metaPath)
	}

	// Record metadata at the leaf.
	now := time.Now()
	entry := config.MetaEntry{
		User:     e.session.UserAtOrigin(),
		Time:     now,
		Session:  e.session.ID,
		Previous: previous,
		Value:    value,
	}
	metaTarget := walkOrCreateMeta(meta, e.schema, path)
	metaTarget.SetEntry(key, entry)

	// Serialize and write draft through the guard.
	output := config.SerializeSetWithMeta(tree, meta, e.schema)
	if err := guard.WriteFile(draftPath, []byte(output), 0o600); err != nil {
		return fmt.Errorf("write-through write: %w", err)
	}

	// Update in-memory state.
	e.tree = tree
	e.meta = meta
	e.dirty.Store(true)
	return nil
}

// writeThroughDelete implements the write-through protocol for delete commands.
// Steps: lock -> read draft -> parse -> apply delete -> serialize -> write draft -> unlock.
func (e *Editor) writeThroughDelete(path []string, key string) error {
	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return fmt.Errorf("write-through lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths

	// Read draft (or config.conf if no draft exists).
	draftPath := DraftPath(e.originalPath)
	tree, meta, err := e.readDraftOrConfig(guard, draftPath)
	if err != nil {
		return fmt.Errorf("write-through read: %w", err)
	}

	// Apply the delete to the draft tree.
	target := walkPath(tree, e.schema, path)
	if target == nil {
		return fmt.Errorf("path not found")
	}
	target.Delete(key)

	// Read committed value for Previous field (re-read under lock
	// to capture external commits, matching writeThroughSet behavior).
	metaPath := append(path, key) //nolint:gocritic // intentional new slice
	previous := ""
	if committedTree := e.readCommittedTree(guard); committedTree != nil {
		previous = getValueAtPath(committedTree, e.schema, metaPath)
	}

	// Record metadata for the delete. The serializer emits "delete" lines
	// for metadata entries without corresponding tree values, so Previous
	// survives the serialize->parse round-trip.
	now := time.Now()
	entry := config.MetaEntry{
		User:     e.session.UserAtOrigin(),
		Time:     now,
		Session:  e.session.ID,
		Previous: previous,
	}
	metaTarget := walkOrCreateMeta(meta, e.schema, path)
	metaTarget.SetEntry(key, entry)

	// Serialize and write draft through the guard.
	output := config.SerializeSetWithMeta(tree, meta, e.schema)
	if err := guard.WriteFile(draftPath, []byte(output), 0o600); err != nil {
		return fmt.Errorf("write-through write: %w", err)
	}

	// Update in-memory state.
	e.tree = tree
	e.meta = meta
	e.dirty.Store(true)
	return nil
}

// readDraftOrConfig reads and parses the draft file, falling back to config.conf.
// Returns the parsed tree and metadata. Uses guard for I/O (called within locked sections).
func (e *Editor) readDraftOrConfig(guard storage.WriteGuard, draftPath string) (*config.Tree, *config.MetaTree, error) {
	parser := config.NewSetParser(e.schema)

	data, err := guard.ReadFile(draftPath)
	if err == nil {
		tree, meta, parseErr := parser.ParseWithMeta(string(data))
		if parseErr != nil {
			return nil, nil, fmt.Errorf("parse draft: %w", parseErr)
		}
		return tree, meta, nil
	}

	// No draft exists: clone the in-memory tree (already parsed from any format)
	// and start with empty metadata.
	return e.tree.Clone(), config.NewMetaTree(), nil
}

// readCommittedTree reads and parses config.conf under lock.
// Re-reads each time to capture external commits between write-through calls.
// Returns nil if the file cannot be read or parsed.
func (e *Editor) readCommittedTree(guard storage.WriteGuard) *config.Tree {
	data, err := guard.ReadFile(e.originalPath)
	if err != nil {
		return nil
	}
	tree, _, err := parseConfigWithFormat(string(data), e.schema)
	if err != nil {
		return nil
	}
	return tree
}

// walkOrCreateIn is like walkOrCreate but operates on an arbitrary tree (not e.tree).
func (e *Editor) walkOrCreateIn(tree *config.Tree, path []string) (*config.Tree, error) {
	if tree == nil || e.schema == nil {
		return nil, fmt.Errorf("tree or schema not available")
	}
	if len(path) == 0 {
		return tree, nil
	}

	currentTree := tree
	var currentSchema schemaGetter = e.schema

	i := 0
	for i < len(path) {
		name := path[i]
		schemaNode := currentSchema.Get(name)
		if schemaNode == nil {
			return nil, fmt.Errorf("unknown path element: %s", name)
		}

		if container, ok := schemaNode.(*config.ContainerNode); ok {
			child := currentTree.GetOrCreateContainer(name)
			currentTree = child
			currentSchema = container
			i++
			continue
		}

		if listNode, ok := schemaNode.(*config.ListNode); ok {
			// Anonymous vs keyed: anonymous if no next element
			// or next element is a schema child of the list.
			var key string
			var step int
			if i+1 >= len(path) || listNode.Get(path[i+1]) != nil {
				key = config.KeyDefault
				step = 1
			} else {
				key = path[i+1]
				step = 2
			}
			entries := currentTree.GetList(name)
			if entries == nil {
				entries = make(map[string]*config.Tree)
			}
			entry := entries[key]
			if entry == nil {
				entry = config.NewTree()
				currentTree.AddListEntry(name, key, entry)
			}
			currentTree = entry
			currentSchema = listNode
			i += step
			continue
		}

		if flexNode, ok := schemaNode.(*config.FlexNode); ok {
			child := currentTree.GetOrCreateContainer(name)
			currentTree = child
			currentSchema = flexNode
			i++
			continue
		}

		if ilNode, ok := schemaNode.(*config.InlineListNode); ok {
			// Inline lists use the same key navigation as regular lists.
			var key string
			var step int
			if i+1 >= len(path) || ilNode.Get(path[i+1]) != nil {
				key = config.KeyDefault
				step = 1
			} else {
				key = path[i+1]
				step = 2
			}
			entries := currentTree.GetList(name)
			if entries == nil {
				entries = make(map[string]*config.Tree)
			}
			entry := entries[key]
			if entry == nil {
				entry = config.NewTree()
				currentTree.AddListEntry(name, key, entry)
			}
			currentTree = entry
			currentSchema = ilNode
			i += step
			continue
		}

		return nil, fmt.Errorf("unexpected schema node type at %s", name)
	}

	return currentTree, nil
}

// walkOrCreateMeta walks the MetaTree path using schema to distinguish containers
// from list entries. List names map to containers; list keys map to list entries.
// This matches the setparser pattern (GetOrCreateContainer for list name,
// GetOrCreateListEntry for key) and the serializer's metaListEntry reader.
func walkOrCreateMeta(meta *config.MetaTree, schema *config.Schema, path []string) *config.MetaTree {
	current := meta
	var currentSchema schemaGetter = schema

	i := 0
	for i < len(path) {
		name := path[i]
		schemaNode := currentSchema.Get(name)
		if schemaNode == nil {
			// Unknown path element: treat as container (best-effort fallback).
			current = current.GetOrCreateContainer(name)
			i++
			continue
		}

		switch n := schemaNode.(type) {
		case *config.ContainerNode:
			current = current.GetOrCreateContainer(name)
			currentSchema = n
			i++
		case *config.ListNode:
			// List name -> container, then key -> list entry.
			// Anonymous list detection: if the next path element is a schema
			// child of the list, treat as anonymous (no key in path).
			// Anonymous lists use KeyDefault to match walkOrCreateIn.
			listContainer := current.GetOrCreateContainer(name)
			if i+1 < len(path) && n.Get(path[i+1]) == nil {
				current = listContainer.GetOrCreateListEntry(path[i+1])
				currentSchema = n
				i += 2
			} else {
				current = listContainer.GetOrCreateListEntry(config.KeyDefault)
				currentSchema = n
				i++
			}
		case *config.FlexNode:
			current = current.GetOrCreateContainer(name)
			currentSchema = n
			i++
		case *config.InlineListNode:
			// Anonymous inline lists use KeyDefault to match walkOrCreateIn.
			listContainer := current.GetOrCreateContainer(name)
			if i+1 < len(path) && n.Get(path[i+1]) == nil {
				current = listContainer.GetOrCreateListEntry(path[i+1])
				currentSchema = n
				i += 2
			} else {
				current = listContainer.GetOrCreateListEntry(config.KeyDefault)
				currentSchema = n
				i++
			}
		case *config.LeafNode, *config.FreeformNode,
			*config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode:
			// Leaf-like nodes should not appear mid-path. Use container as
			// best-effort so we don't lose metadata.
			current = current.GetOrCreateContainer(name)
			i++
		}
	}
	return current
}

// walkMetaReadOnly navigates MetaTree using schema for read-only access.
// Returns nil if any segment is missing (does not create nodes).
func walkMetaReadOnly(meta *config.MetaTree, schema *config.Schema, path []string) *config.MetaTree {
	current := meta
	var currentSchema schemaGetter = schema

	i := 0
	for i < len(path) {
		name := path[i]
		schemaNode := currentSchema.Get(name)
		if schemaNode == nil {
			return nil
		}

		switch n := schemaNode.(type) {
		case *config.ContainerNode:
			child := current.GetContainer(name)
			if child == nil {
				return nil
			}
			current = child
			currentSchema = n
			i++
		case *config.ListNode:
			listContainer := current.GetContainer(name)
			if listContainer == nil {
				return nil
			}
			if i+1 < len(path) && n.Get(path[i+1]) == nil {
				entry := listContainer.GetListEntry(path[i+1])
				if entry == nil {
					return nil
				}
				current = entry
				currentSchema = n
				i += 2
			} else {
				// Anonymous list: use KeyDefault to match walkOrCreateMeta.
				entry := listContainer.GetListEntry(config.KeyDefault)
				if entry == nil {
					return nil
				}
				current = entry
				currentSchema = n
				i++
			}
		case *config.FlexNode:
			child := current.GetContainer(name)
			if child == nil {
				return nil
			}
			current = child
			currentSchema = n
			i++
		case *config.InlineListNode:
			listContainer := current.GetContainer(name)
			if listContainer == nil {
				return nil
			}
			if i+1 < len(path) && n.Get(path[i+1]) == nil {
				entry := listContainer.GetListEntry(path[i+1])
				if entry == nil {
					return nil
				}
				current = entry
				currentSchema = n
				i += 2
			} else {
				// Anonymous inline list: use KeyDefault to match walkOrCreateMeta.
				entry := listContainer.GetListEntry(config.KeyDefault)
				if entry == nil {
					return nil
				}
				current = entry
				currentSchema = n
				i++
			}
		case *config.LeafNode, *config.FreeformNode,
			*config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode:
			return nil // Leaf-like nodes cannot be navigated into.
		}
	}
	return current
}

// walkPath walks an arbitrary tree using schema to navigate lists.
func walkPath(tree *config.Tree, schema *config.Schema, path []string) *config.Tree {
	if len(path) == 0 {
		return tree
	}

	current := tree
	var currentSchema schemaGetter = schema

	i := 0
	for i < len(path) {
		name := path[i]
		schemaNode := currentSchema.Get(name)
		if schemaNode == nil {
			return nil
		}

		if container, ok := schemaNode.(*config.ContainerNode); ok {
			child := current.GetContainer(name)
			if child == nil {
				return nil
			}
			current = child
			currentSchema = container
			i++
			continue
		}

		if listNode, ok := schemaNode.(*config.ListNode); ok {
			// Anonymous vs keyed: anonymous if no next element
			// or next element is a schema child of the list.
			var key string
			var step int
			if i+1 >= len(path) || listNode.Get(path[i+1]) != nil {
				key = config.KeyDefault
				step = 1
			} else {
				key = path[i+1]
				step = 2
			}
			entries := current.GetList(name)
			if entries == nil {
				return nil
			}
			entry := entries[key]
			if entry == nil {
				return nil
			}
			current = entry
			currentSchema = listNode
			i += step
			continue
		}

		if flexNode, ok := schemaNode.(*config.FlexNode); ok {
			child := current.GetContainer(name)
			if child == nil {
				return nil
			}
			current = child
			currentSchema = flexNode
			i++
			continue
		}

		if ilNode, ok := schemaNode.(*config.InlineListNode); ok {
			var key string
			var step int
			if i+1 >= len(path) || ilNode.Get(path[i+1]) != nil {
				key = config.KeyDefault
				step = 1
			} else {
				key = path[i+1]
				step = 2
			}
			entries := current.GetList(name)
			if entries == nil {
				return nil
			}
			entry := entries[key]
			if entry == nil {
				return nil
			}
			current = entry
			currentSchema = ilNode
			i += step
			continue
		}

		return nil
	}

	return current
}

// CommitSession commits the current session's changes to config.conf.
// Returns a CommitResult with conflicts (if any) or the number of applied changes.
//
//nolint:cyclop // commit protocol has inherently many steps
func (e *Editor) CommitSession() (*CommitResult, error) {
	if e.session == nil {
		return nil, fmt.Errorf("no session set")
	}

	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return nil, fmt.Errorf("commit lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths

	// Read and parse config.conf (committed), detecting format to handle
	// both hierarchical and set+meta formats (config.conf becomes set+meta
	// after the first session commit).
	committedData, err := guard.ReadFile(e.originalPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	committedContent := string(committedData)
	committedTree, existingMeta, err := parseConfigWithFormat(committedContent, e.schema)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Apply tree structure migrations if the committed config is hierarchical
	// (e.g., neighbor->peer renaming). This runs once on the first session commit
	// of a legacy config; subsequent commits read set+meta and skip this.
	var migrationWarning string
	if config.DetectFormat(committedContent) == config.FormatHierarchical {
		if migration.NeedsMigration(committedTree) {
			result, migrateErr := migration.Migrate(committedTree)
			if migrateErr == nil {
				committedTree = result.Tree
			} else {
				// Format conversion still happens (set+meta output),
				// but tree transforms were skipped. Surface the warning.
				migrationWarning = fmt.Sprintf("tree migration skipped: %v", migrateErr)
			}
		}
	}

	// Read and parse config.draft.
	draftPath := DraftPath(e.originalPath)
	draftData, err := guard.ReadFile(draftPath)
	if err != nil {
		return nil, fmt.Errorf("read draft: %w", err)
	}
	setParser := config.NewSetParser(e.schema)
	draftTree, draftMeta, err := setParser.ParseWithMeta(string(draftData))
	if err != nil {
		return nil, fmt.Errorf("parse draft: %w", err)
	}

	// Find my changes.
	myEntries := draftMeta.SessionEntries(e.session.ID)
	if len(myEntries) == 0 {
		return &CommitResult{Applied: 0}, nil
	}

	// Check for conflicts.
	var conflicts []Conflict
	for _, se := range myEntries {
		pathParts := strings.Fields(se.Path)

		// Use session metadata for this session's intent, not the draft tree.
		// The draft tree holds the merged state of all sessions (last writer wins),
		// which may not reflect this session's actual intent (e.g., if another
		// session set a value after this session deleted it).
		myValue := se.Entry.Value

		// Check stale Previous: compare config.conf current value with recorded Previous.
		// Three checks:
		// 1. Convergent agreement: if committed already matches my intent, no conflict
		//    (handles concurrent delete: both sessions deleted the same value).
		// 2. Previous != "" and committed changed: the value I edited was modified by another commit.
		// 3. Previous == "" and committed != "": I added a new value, but someone else also
		//    added and committed a value at the same path -- stale conflict.
		committedValue := getValueAtPath(committedTree, e.schema, pathParts)
		isStale := myValue != committedValue &&
			((se.Entry.Previous != "" && committedValue != se.Entry.Previous) ||
				(se.Entry.Previous == "" && committedValue != ""))
		if isStale {
			conflicts = append(conflicts, Conflict{
				Path:          se.Path,
				Type:          ConflictStale,
				MyValue:       myValue,
				OtherValue:    committedValue,
				PreviousValue: se.Entry.Previous,
			})
		}

		// Check live disagreement: another session has a pending change at same path.
		conflicts = append(conflicts, checkLiveConflicts(draftMeta, e.session.ID, se.Path, pathParts, myValue, e.schema)...)
	}

	if len(conflicts) > 0 {
		return &CommitResult{Conflicts: conflicts}, nil
	}

	// No conflicts: apply my changes to committed tree.
	// Use session metadata (Entry.Value) for each change, not the draft tree.
	// Empty Value with a session ID means delete.
	applied := 0
	for _, se := range myEntries {
		pathParts := strings.Fields(se.Path)
		value := se.Entry.Value
		if len(pathParts) > 0 {
			leafName := pathParts[len(pathParts)-1]
			parentPath := pathParts[:len(pathParts)-1]
			target, walkErr := e.walkOrCreateIn(committedTree, parentPath)
			if walkErr != nil {
				continue
			}
			if value != "" {
				target.Set(leafName, value)
			} else {
				target.Delete(leafName)
			}
			applied++
		}
	}

	// Create backup before overwriting config.conf. Use freshly-read data
	// (not cached originalContent) in case another session committed since
	// this editor was created.
	if err := e.createBackup(string(committedData), guard); err != nil {
		return nil, fmt.Errorf("backup: %w", err)
	}

	// Write committed tree to config.conf (with user/time metadata, no session).
	now := time.Now()
	commitMeta := buildCommitMeta(existingMeta, draftMeta, myEntries, e.session.UserAtOrigin(), now, e.schema)
	committedOutput := config.SerializeSetWithMeta(committedTree, commitMeta, e.schema)
	if err := guard.WriteFile(e.originalPath, []byte(committedOutput), 0o600); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	// Remove my session from draft.
	draftMeta.RemoveSession(e.session.ID)

	// Check if other sessions remain.
	remainingSessions := draftMeta.AllSessions()
	if len(remainingSessions) == 0 {
		// No more pending changes: delete draft.
		guard.Remove(draftPath) //nolint:errcheck // Best effort
	} else {
		// Regenerate draft without my entries.
		draftOutput := config.SerializeSetWithMeta(draftTree, draftMeta, e.schema)
		if err := guard.WriteFile(draftPath, []byte(draftOutput), 0o600); err != nil {
			return nil, fmt.Errorf("write draft: %w", err)
		}
	}

	// Update in-memory state: originalContent always tracks config.conf.
	e.originalContent = committedOutput
	if len(remainingSessions) == 0 {
		// No pending changes: show committed state.
		e.tree = committedTree
		e.meta = commitMeta
	} else {
		// Other sessions have pending changes: show draft state so
		// show/blame/changes commands are consistent (tree and meta
		// both reflect the draft, not a mix of committed + draft).
		e.tree = draftTree
		e.meta = draftMeta
	}
	e.dirty.Store(false)

	return &CommitResult{Applied: applied, MigrationWarning: migrationWarning}, nil
}

// DiscardSessionPath discards this session's changes at the given path.
// If path is nil, discards all changes for this session.
func (e *Editor) DiscardSessionPath(path []string) error {
	if e.session == nil {
		return fmt.Errorf("no session set")
	}

	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return fmt.Errorf("discard lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths

	draftPath := DraftPath(e.originalPath)
	draftData, err := guard.ReadFile(draftPath)
	if err != nil {
		return fmt.Errorf("read draft: %w", err)
	}
	setParser := config.NewSetParser(e.schema)
	draftTree, draftMeta, err := setParser.ParseWithMeta(string(draftData))
	if err != nil {
		return fmt.Errorf("parse draft: %w", err)
	}

	// Read committed config under lock (not cached originalContent) to capture
	// external commits since the editor was created. Spec: "restore committed
	// value (from config.conf)".
	committedData, readErr := guard.ReadFile(e.originalPath)
	if readErr != nil {
		return fmt.Errorf("read config: %w", readErr)
	}
	committedTree, _, err := parseConfigWithFormat(string(committedData), e.schema)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	// Find my entries to discard.
	myEntries := draftMeta.SessionEntries(e.session.ID)
	pathPrefix := strings.Join(path, " ")

	for _, se := range myEntries {
		// Filter by path prefix (empty prefix = discard all).
		// Use prefix + space boundary to avoid "bgp peer" matching "bgp peer-group".
		if pathPrefix != "" && se.Path != pathPrefix && !strings.HasPrefix(se.Path, pathPrefix+" ") {
			continue
		}

		pathParts := strings.Fields(se.Path)
		if len(pathParts) == 0 {
			continue
		}
		leafName := pathParts[len(pathParts)-1]
		parentPath := pathParts[:len(pathParts)-1]

		// Remove this session's metadata entry (preserves other sessions' entries).
		metaTarget := walkOrCreateMeta(draftMeta, e.schema, parentPath)
		metaTarget.RemoveSessionEntry(leafName, e.session.ID)

		// If another session still has a pending value at this leaf,
		// restore that session's value (not the committed value).
		target, walkErr := e.walkOrCreateIn(draftTree, parentPath)
		if walkErr != nil {
			continue
		}
		remaining := metaTarget.GetAllEntries(leafName)
		if len(remaining) > 0 && remaining[len(remaining)-1].Value != "" {
			target.Set(leafName, remaining[len(remaining)-1].Value)
		} else {
			// No other session has a pending value: restore committed value.
			committedValue := getValueAtPath(committedTree, e.schema, pathParts)
			if committedValue != "" {
				target.Set(leafName, committedValue)
			} else {
				target.Delete(leafName)
			}
		}
	}

	// Remove my session from draft metadata.
	if pathPrefix == "" {
		draftMeta.RemoveSession(e.session.ID)
	}

	// Check if other sessions remain.
	remainingSessions := draftMeta.AllSessions()
	if len(remainingSessions) == 0 {
		guard.Remove(draftPath) //nolint:errcheck // Best effort
	} else {
		draftOutput := config.SerializeSetWithMeta(draftTree, draftMeta, e.schema)
		if err := guard.WriteFile(draftPath, []byte(draftOutput), 0o600); err != nil {
			return fmt.Errorf("write draft: %w", err)
		}
	}

	// Update in-memory state. Refresh originalContent from disk to capture
	// external commits since this editor was created (prevents stale backup
	// content in subsequent operations).
	e.tree = draftTree
	e.meta = draftMeta
	e.originalContent = string(committedData)
	// Mark dirty only if this session still has pending entries (partial discard
	// keeps other entries alive; discard-all clears everything).
	e.dirty.Store(len(draftMeta.SessionEntries(e.session.ID)) > 0)
	return nil
}

// DisconnectSession removes all entries for another session from the draft.
// Restores committed values where the disconnected session was the only editor.
func (e *Editor) DisconnectSession(sessionID string) error {
	if e.session == nil {
		return fmt.Errorf("no session set")
	}

	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return fmt.Errorf("disconnect lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths

	draftPath := DraftPath(e.originalPath)
	draftData, err := guard.ReadFile(draftPath)
	if err != nil {
		return fmt.Errorf("read draft: %w", err)
	}
	setParser := config.NewSetParser(e.schema)
	draftTree, draftMeta, err := setParser.ParseWithMeta(string(draftData))
	if err != nil {
		return fmt.Errorf("parse draft: %w", err)
	}

	// Parse committed config for restoring values.
	committedData, readErr := guard.ReadFile(e.originalPath)
	if readErr != nil {
		return fmt.Errorf("read config: %w", readErr)
	}
	committedTree, _, parseErr := parseConfigWithFormat(string(committedData), e.schema)
	if parseErr != nil {
		return fmt.Errorf("parse config: %w", parseErr)
	}

	// Restore values for the disconnected session's entries.
	// Remove metadata first, then check if another session has pending values.
	entries := draftMeta.SessionEntries(sessionID)
	for _, se := range entries {
		pathParts := strings.Fields(se.Path)
		if len(pathParts) == 0 {
			continue
		}
		leafName := pathParts[len(pathParts)-1]
		parentPath := pathParts[:len(pathParts)-1]

		// Remove this session's metadata entry first (preserves other sessions').
		metaTarget := walkOrCreateMeta(draftMeta, e.schema, parentPath)
		metaTarget.RemoveSessionEntry(leafName, sessionID)

		// If another session still has a pending value, use that instead.
		target, walkErr := e.walkOrCreateIn(draftTree, parentPath)
		if walkErr != nil {
			continue
		}
		remaining := metaTarget.GetAllEntries(leafName)
		if len(remaining) > 0 && remaining[len(remaining)-1].Value != "" {
			target.Set(leafName, remaining[len(remaining)-1].Value)
		} else {
			// No other session has a pending value: restore committed value.
			committedValue := getValueAtPath(committedTree, e.schema, pathParts)
			if committedValue != "" {
				target.Set(leafName, committedValue)
			} else {
				target.Delete(leafName)
			}
		}
	}

	draftMeta.RemoveSession(sessionID)

	// Check if any sessions remain.
	remainingSessions := draftMeta.AllSessions()
	if len(remainingSessions) == 0 {
		guard.Remove(draftPath) //nolint:errcheck // Best effort
	} else {
		draftOutput := config.SerializeSetWithMeta(draftTree, draftMeta, e.schema)
		if err := guard.WriteFile(draftPath, []byte(draftOutput), 0o600); err != nil {
			return fmt.Errorf("write draft: %w", err)
		}
	}

	// Update in-memory state. Refresh originalContent from disk to capture
	// external commits since this editor was created (matching DiscardSessionPath).
	e.tree = draftTree
	e.meta = draftMeta
	e.originalContent = string(committedData)
	return nil
}

// getValueAtPath retrieves a leaf value from a tree at the given YANG path.
func getValueAtPath(tree *config.Tree, schema *config.Schema, pathParts []string) string {
	if len(pathParts) == 0 {
		return ""
	}
	leafName := pathParts[len(pathParts)-1]
	parentPath := pathParts[:len(pathParts)-1]
	target := walkPath(tree, schema, parentPath)
	if target == nil {
		return ""
	}
	val, _ := target.Get(leafName)
	return val
}

// checkLiveConflicts checks if other sessions have pending changes at the same YANG path.
// myValue is this session's Entry.Value (empty string = delete intent).
// Other sessions' values are also read from Entry.Value, not the draft tree.
func checkLiveConflicts(meta *config.MetaTree, mySessionID, yangPath string, pathParts []string, myValue string, schema *config.Schema) []Conflict {
	if len(pathParts) == 0 {
		return nil
	}
	leafName := pathParts[len(pathParts)-1]
	parentPath := pathParts[:len(pathParts)-1]

	// Walk MetaTree to the leaf's parent using schema-aware navigation
	// so list entries are found in .lists (not .containers).
	metaTarget := walkMetaReadOnly(meta, schema, parentPath)
	if metaTarget == nil {
		return nil
	}

	entries := metaTarget.GetAllEntries(leafName)
	if len(entries) == 0 {
		return nil
	}

	// Check all entries for other-session disagreements.
	// Both myValue and otherValue come from Entry.Value (session intent).
	// Empty Value with non-empty Session = delete intent.
	var conflicts []Conflict
	for _, entry := range entries {
		if entry.Session != mySessionID && entry.Session != "" {
			otherValue := entry.Value
			if otherValue != myValue {
				conflicts = append(conflicts, Conflict{
					Path:       yangPath,
					Type:       ConflictLive,
					MyValue:    myValue,
					OtherValue: otherValue,
					OtherUser:  entry.User,
				})
			}
		}
	}

	return conflicts
}

// parseConfigWithFormat reads config content using format auto-detection.
// Returns the tree and any existing metadata (nil for hierarchical format).
func parseConfigWithFormat(content string, schema *config.Schema) (*config.Tree, *config.MetaTree, error) {
	format := config.DetectFormat(content)

	switch format {
	case config.FormatSetMeta:
		return config.NewSetParser(schema).ParseWithMeta(content)
	case config.FormatSet:
		tree, err := config.NewSetParser(schema).Parse(content)
		return tree, config.NewMetaTree(), err
	case config.FormatHierarchical:
		tree, err := config.NewParser(schema).Parse(content)
		return tree, config.NewMetaTree(), err
	}

	tree, err := config.NewParser(schema).Parse(content)
	return tree, config.NewMetaTree(), err
}

// buildCommitMeta creates metadata for the committed config.conf.
// Starts from existing committed metadata (preserving prior commit annotations),
// overlays with the committer's entries, and copies non-session metadata from draft.
func buildCommitMeta(existingMeta, draftMeta *config.MetaTree, myEntries []config.SessionEntry, user string, commitTime time.Time, schema *config.Schema) *config.MetaTree {
	// Start from existing committed metadata to preserve prior annotations.
	commitMeta := existingMeta
	if commitMeta == nil {
		commitMeta = config.NewMetaTree()
	}

	// For each committed entry, record user and time (no session, no previous).
	// This overwrites any prior metadata for leaves this session changed.
	// For deletes (Value=""), remove any existing metadata instead of creating
	// a tombstone -- deleted leaves don't need metadata in the committed config.
	for _, se := range myEntries {
		pathParts := strings.Fields(se.Path)
		if len(pathParts) == 0 {
			continue
		}
		leafName := pathParts[len(pathParts)-1]
		parentPath := pathParts[:len(pathParts)-1]
		target := walkOrCreateMeta(commitMeta, schema, parentPath)
		if se.Entry.Value == "" {
			// Delete: remove metadata rather than creating a tombstone.
			target.RemoveSessionEntry(leafName, "")
		} else {
			target.SetEntry(leafName, config.MetaEntry{
				User: user,
				Time: commitTime,
			})
		}
	}

	// Copy metadata from draft for non-session entries (hand-written lines).
	copyNonSessionMeta(commitMeta, draftMeta)

	return commitMeta
}

// copyNonSessionMeta copies entries without session IDs from src to dst.
// Uses AllEntries to iterate all entries per leaf (not just the last),
// preserving any hand-written metadata that might have multiple entries.
func copyNonSessionMeta(dst, src *config.MetaTree) {
	if src == nil {
		return
	}

	for name, entries := range src.AllEntries() {
		for _, entry := range entries {
			if entry.Session == "" {
				if _, exists := dst.GetEntry(name); !exists {
					dst.SetEntry(name, entry)
				}
			}
		}
	}

	for name, child := range src.Containers() {
		dstChild := dst.GetOrCreateContainer(name)
		copyNonSessionMeta(dstChild, child)
	}

	for key, child := range src.Lists() {
		dstChild := dst.GetOrCreateListEntry(key)
		copyNonSessionMeta(dstChild, child)
	}
}
