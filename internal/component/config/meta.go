// Design: docs/architecture/config/syntax.md — metadata tracking for concurrent config editing
// Related: serialize_set.go — set-format serialization (emits metadata prefixes)
// Related: tree.go — Tree data structure (MetaTree mirrors its navigation)
// Related: serialize_annotated.go — column-aware annotated serialization

package config

import (
	"fmt"
	"maps"
	"sort"
	"sync"
	"time"
)

// MetaEntry records who changed a config leaf and when.
//
// Serialized prefixes: #user @source %time ^previous
//   - # username (e.g., "thomas")
//   - @ connection source (e.g., "local", "192.168.1.5")
//   - % session start time (ISO 8601, same for all edits in a session)
//   - ^ previous value (before this change)
type MetaEntry struct {
	User     string    // Username only (e.g., "thomas"). Serialized as #user.
	Source   string    // Connection origin (e.g., "local", "192.168.1.5"). Serialized as @source.
	Time     time.Time // Session start time. Serialized as %ISO8601. Same for all edits in a session.
	Previous string    // Value from config.conf before this session's change.
	Value    string    // The value this session set (for contested leaves in draft).
}

// SessionKey returns the grouping key for concurrent editing.
// Concatenates user + source + session-start-time to produce a stable key
// that is unique per editing session but shared across all edits within it.
func (e MetaEntry) SessionKey() string {
	key := e.User
	if e.Source != "" {
		key += "@" + e.Source
	}
	if !e.Time.IsZero() {
		key += "%" + e.Time.UTC().Format(time.RFC3339)
	}
	return key
}

// SessionEntry pairs a YANG path with its metadata entry,
// returned by SessionEntries for filtering by session.
type SessionEntry struct {
	Path  string
	Entry MetaEntry
}

// MetaTree mirrors Tree's container/list structure to store per-leaf metadata.
// Navigation uses the same path segments as Tree (containers for YANG containers,
// list entries keyed by their identifier).
// Each leaf can have multiple entries (one per session) for contested leaves.
//
// Safe for concurrent use: mu guards every map below. Each MetaTree owns its
// own mutex; recursion into a child node locks the child separately.
type MetaTree struct {
	mu         sync.RWMutex
	entries    map[string][]MetaEntry
	containers map[string]*MetaTree
	lists      map[string]*MetaTree
}

// NewMetaTree creates an empty metadata tree.
func NewMetaTree() *MetaTree {
	return &MetaTree{
		entries:    make(map[string][]MetaEntry),
		containers: make(map[string]*MetaTree),
		lists:      make(map[string]*MetaTree),
	}
}

// SetEntry stores metadata for a leaf at the given name.
// If an entry from the same session exists, it is replaced.
// Entries from different sessions are preserved (for live conflict detection).
func (mt *MetaTree) SetEntry(name string, entry MetaEntry) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	existing := mt.entries[name]
	var updated []MetaEntry
	replaced := false
	for _, e := range existing {
		if e.SessionKey() == entry.SessionKey() {
			// Replace same-session entry (including sessionless overwrites).
			updated = append(updated, entry)
			replaced = true
		} else {
			updated = append(updated, e)
		}
	}
	if !replaced {
		updated = append(updated, entry)
	}
	mt.entries[name] = updated
}

// GetEntry retrieves metadata for a leaf (returns the last entry).
func (mt *MetaTree) GetEntry(name string) (MetaEntry, bool) {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	entries := mt.entries[name]
	if len(entries) == 0 {
		return MetaEntry{}, false
	}
	return entries[len(entries)-1], true
}

// RemoveEntry removes all metadata entries for a leaf, regardless of session.
func (mt *MetaTree) RemoveEntry(name string) {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	delete(mt.entries, name)
}

// RemoveSessionEntry removes entries for a specific session from a leaf.
// Preserves entries from other sessions. If no entries remain, the key is deleted.
func (mt *MetaTree) RemoveSessionEntry(name, sessionID string) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	entries := mt.entries[name]
	var kept []MetaEntry
	for _, e := range entries {
		if e.SessionKey() != sessionID {
			kept = append(kept, e)
		}
	}
	if len(kept) == 0 {
		delete(mt.entries, name)
	} else {
		mt.entries[name] = kept
	}
}

// GetAllEntries returns all metadata entries for a leaf.
// Multiple entries exist when different sessions have pending changes at the same path.
func (mt *MetaTree) GetAllEntries(name string) []MetaEntry {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	entries := mt.entries[name]
	if len(entries) == 0 {
		return nil
	}
	out := make([]MetaEntry, len(entries))
	copy(out, entries)
	return out
}

// Entries returns the primary (last) entry for each leaf name.
func (mt *MetaTree) Entries() map[string]MetaEntry {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	result := make(map[string]MetaEntry, len(mt.entries))
	for name, entries := range mt.entries {
		if len(entries) > 0 {
			result[name] = entries[len(entries)-1]
		}
	}
	return result
}

// AllEntries returns a snapshot of all entries keyed by leaf name.
// The returned slices are copies; callers may mutate them without affecting the tree.
func (mt *MetaTree) AllEntries() map[string][]MetaEntry {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	result := make(map[string][]MetaEntry, len(mt.entries))
	for name, entries := range mt.entries {
		out := make([]MetaEntry, len(entries))
		copy(out, entries)
		result[name] = out
	}
	return result
}

// Containers returns a snapshot of all container sub-trees.
func (mt *MetaTree) Containers() map[string]*MetaTree {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	out := make(map[string]*MetaTree, len(mt.containers))
	maps.Copy(out, mt.containers)
	return out
}

// Lists returns a snapshot of all list entry sub-trees.
func (mt *MetaTree) Lists() map[string]*MetaTree {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	out := make(map[string]*MetaTree, len(mt.lists))
	maps.Copy(out, mt.lists)
	return out
}

// GetContainer returns the sub-tree for a YANG container, or nil if not found.
// Use this for read-only navigation (e.g., conflict checks) to avoid
// mutating the tree during reads.
func (mt *MetaTree) GetContainer(name string) *MetaTree {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return mt.containers[name]
}

// GetListEntry returns the sub-tree for a YANG list entry, or nil if not found.
// Use this for read-only navigation to avoid creating empty entries.
func (mt *MetaTree) GetListEntry(key string) *MetaTree {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return mt.lists[key]
}

// GetOrCreateContainer returns the sub-tree for a YANG container,
// creating it if it doesn't exist.
func (mt *MetaTree) GetOrCreateContainer(name string) *MetaTree {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	if child, ok := mt.containers[name]; ok {
		return child
	}
	child := NewMetaTree()
	mt.containers[name] = child
	return child
}

// GetOrCreateListEntry returns the sub-tree for a YANG list entry,
// creating it if it doesn't exist. List entries are keyed by their
// identifier (e.g., neighbor address).
func (mt *MetaTree) GetOrCreateListEntry(key string) *MetaTree {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	if child, ok := mt.lists[key]; ok {
		return child
	}
	child := NewMetaTree()
	mt.lists[key] = child
	return child
}

// RenameListEntry renames a list-entry subtree under the named list container.
// Returns an error if the target key already exists.
func (mt *MetaTree) RenameListEntry(listName, oldKey, newKey string) error {
	mt.mu.RLock()
	listContainer := mt.containers[listName]
	mt.mu.RUnlock()
	if listContainer == nil {
		return nil
	}

	listContainer.mu.Lock()
	defer listContainer.mu.Unlock()

	entry, ok := listContainer.lists[oldKey]
	if !ok {
		return nil
	}
	if _, exists := listContainer.lists[newKey]; exists {
		return fmt.Errorf("%s already exists in %s", newKey, listName)
	}
	listContainer.lists[newKey] = entry
	delete(listContainer.lists, oldKey)
	return nil
}

// Clone returns a deep copy of the metadata tree.
func (mt *MetaTree) Clone() *MetaTree {
	if mt == nil {
		return nil
	}

	mt.mu.RLock()
	defer mt.mu.RUnlock()

	clone := NewMetaTree()
	for name, entries := range mt.entries {
		copied := make([]MetaEntry, len(entries))
		copy(copied, entries)
		clone.entries[name] = copied
	}
	for name, child := range mt.containers {
		clone.containers[name] = child.Clone()
	}
	for key, child := range mt.lists {
		clone.lists[key] = child.Clone()
	}
	return clone
}

// SessionEntries collects all entries matching the given session ID,
// walking the entire tree recursively. Each result includes the
// accumulated YANG path to the leaf.
func (mt *MetaTree) SessionEntries(sessionID string) []SessionEntry {
	var result []SessionEntry
	mt.collectSession(sessionID, "", &result)
	return result
}

// collectSession recursively walks the tree, accumulating path prefix.
// Acquires mt.mu.RLock for its own node; child recursion locks the child
// independently.
func (mt *MetaTree) collectSession(sessionID, prefix string, result *[]SessionEntry) {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	for name, entries := range mt.entries {
		for _, entry := range entries {
			if entry.SessionKey() == sessionID {
				path := name
				if prefix != "" {
					path = prefix + " " + name
				}
				*result = append(*result, SessionEntry{Path: path, Entry: entry})
			}
		}
	}

	for name, child := range mt.containers {
		childPrefix := name
		if prefix != "" {
			childPrefix = prefix + " " + name
		}
		child.collectSession(sessionID, childPrefix, result)
	}

	for key, child := range mt.lists {
		childPrefix := key
		if prefix != "" {
			childPrefix = prefix + " " + key
		}
		child.collectSession(sessionID, childPrefix, result)
	}
}

// RemoveSession removes all entries matching the given session ID
// from the entire tree. Preserves entries from other sessions.
func (mt *MetaTree) RemoveSession(sessionID string) {
	mt.mu.Lock()
	for name, entries := range mt.entries {
		var kept []MetaEntry
		for _, e := range entries {
			if e.SessionKey() != sessionID {
				kept = append(kept, e)
			}
		}
		if len(kept) == 0 {
			delete(mt.entries, name)
		} else {
			mt.entries[name] = kept
		}
	}

	// Snapshot children so recursion does not hold mt.mu while calling
	// into the child (the child acquires its own lock).
	containers := make([]*MetaTree, 0, len(mt.containers))
	for _, child := range mt.containers {
		containers = append(containers, child)
	}
	lists := make([]*MetaTree, 0, len(mt.lists))
	for _, child := range mt.lists {
		lists = append(lists, child)
	}
	mt.mu.Unlock()

	for _, child := range containers {
		child.RemoveSession(sessionID)
	}
	for _, child := range lists {
		child.RemoveSession(sessionID)
	}
}

// AllSessions returns all unique session IDs in the tree.
func (mt *MetaTree) AllSessions() []string {
	seen := make(map[string]bool)
	mt.collectSessions(seen)

	sessions := make([]string, 0, len(seen))
	for s := range seen {
		sessions = append(sessions, s)
	}
	sort.Strings(sessions)
	return sessions
}

// collectSessions recursively gathers unique session IDs.
func (mt *MetaTree) collectSessions(seen map[string]bool) {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	for _, entries := range mt.entries {
		for _, entry := range entries {
			if entry.Source == "" {
				continue
			}
			if key := entry.SessionKey(); key != "" {
				seen[key] = true
			}
		}
	}

	for _, child := range mt.containers {
		child.collectSessions(seen)
	}

	for _, child := range mt.lists {
		child.collectSessions(seen)
	}
}

// HasSession returns true if any entry in the tree belongs to the given session.
func (mt *MetaTree) HasSession(sessionID string) bool {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	for _, entries := range mt.entries {
		for _, entry := range entries {
			if entry.SessionKey() == sessionID {
				return true
			}
		}
	}

	for _, child := range mt.containers {
		if child.HasSession(sessionID) {
			return true
		}
	}

	for _, child := range mt.lists {
		if child.HasSession(sessionID) {
			return true
		}
	}

	return false
}
