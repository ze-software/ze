// Design: docs/architecture/web-interface.md -- Per-user editor management
// Related: handler_config.go -- Config view and edit handlers
// Related: cli.go -- CLI bar command dispatch using Editor
// Related: auth.go -- Session authentication

package web

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

// userSession holds the per-user editor state for web-based config editing.
// Each authenticated user gets an independent Editor instance with its own
// working tree and change tracking.
type userSession struct {
	editor       *cli.Editor
	mu           sync.Mutex // Serializes Editor method calls for this user.
	lastActivity time.Time
}

// EditorManager manages per-user Editor instances for the web interface.
// Each user gets an independent Editor with its own working tree and session.
// The manager handles creation, eviction of idle sessions, and serialized
// access to each user's Editor.
//
// NOT safe for use without initialization via NewEditorManager.
type EditorManager struct {
	mu          sync.RWMutex
	sessions    map[string]*userSession // Keyed by username.
	store       storage.Storage
	configPath  string
	schema      *config.Schema
	maxSessions int
	idleTimeout time.Duration
}

// NewEditorManager creates an EditorManager for the given storage backend and config path.
// Default limits: 50 concurrent sessions, 1 hour idle timeout.
func NewEditorManager(store storage.Storage, configPath string, schema *config.Schema) *EditorManager {
	return &EditorManager{
		sessions:    make(map[string]*userSession),
		store:       store,
		configPath:  configPath,
		schema:      schema,
		maxSessions: 50,
		idleTimeout: time.Hour,
	}
}

// GetOrCreate returns the existing userSession for the given username, or creates
// a new one backed by a fresh Editor and EditSession. When the session count exceeds
// maxSessions, idle sessions older than idleTimeout are evicted.
func (m *EditorManager) GetOrCreate(username string) (*userSession, error) {
	// Fast path: session already exists. Uses write lock because
	// lastActivity update is a write that races under RLock.
	m.mu.Lock()
	if us, ok := m.sessions[username]; ok {
		us.lastActivity = time.Now()
		m.mu.Unlock()
		return us, nil
	}

	// Slow path: create new session (already holding write lock).
	defer m.mu.Unlock()

	// Evict idle sessions if over capacity.
	if len(m.sessions) >= m.maxSessions {
		m.evictInactive()
	}

	// Recheck after eviction: still at capacity means no idle sessions were freed.
	if len(m.sessions) >= m.maxSessions {
		return nil, fmt.Errorf("maximum concurrent editor sessions reached (%d)", m.maxSessions)
	}

	ed, err := cli.NewEditorWithStorage(m.store, m.configPath)
	if err != nil {
		return nil, fmt.Errorf("editor create for %s: %w", username, err)
	}

	session := cli.NewEditSession(username, "web")
	ed.SetSession(session)

	us := &userSession{
		editor:       ed,
		lastActivity: time.Now(),
	}
	m.sessions[username] = us

	return us, nil
}

// CreateEntry creates an empty list entry at the given path in the user's working tree.
func (m *EditorManager) CreateEntry(username string, path []string) error {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return err
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	return us.editor.CreateEntry(path)
}

// SetValue sets a leaf value at the given path in the user's working tree.
func (m *EditorManager) SetValue(username string, path []string, key, value string) error {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return err
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	return us.editor.SetValue(path, key, value)
}

// DeleteValue removes a leaf value at the given path in the user's working tree.
func (m *EditorManager) DeleteValue(username string, path []string, key string) error {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return err
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	return us.editor.DeleteValue(path, key)
}

// Commit applies the user's pending changes to the configuration file.
// Returns a CommitResult describing conflicts or the number of applied changes.
func (m *EditorManager) Commit(username string) (*cli.CommitResult, error) {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return nil, err
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	return us.editor.CommitSession()
}

// Discard reverts the user's working tree to the original state and removes
// the session from the manager.
func (m *EditorManager) Discard(username string) error {
	m.mu.RLock()
	us, ok := m.sessions[username]
	m.mu.RUnlock()

	if !ok {
		return nil // No session to discard.
	}

	us.mu.Lock()
	err := us.editor.Discard()
	us.mu.Unlock()

	if err != nil {
		return fmt.Errorf("discard for %s: %w", username, err)
	}

	m.mu.Lock()
	if current, ok := m.sessions[username]; ok && current == us {
		delete(m.sessions, username)
	}
	m.mu.Unlock()

	return nil
}

// Diff returns a textual diff of the user's pending changes.
// For session-based editing, formats the change entries as a readable diff.
// Returns an empty string if no session exists or no changes are pending.
func (m *EditorManager) Diff(username string) (string, error) {
	m.mu.RLock()
	us, ok := m.sessions[username]
	m.mu.RUnlock()

	if !ok {
		return "", nil
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	// Try text diff first (non-session mode).
	if d := us.editor.Diff(); d != "" {
		return d, nil
	}

	// Session mode: build diff from tracked changes.
	sid := us.editor.SessionID()
	if sid == "" {
		return "", nil
	}

	entries := us.editor.SessionChanges(sid)
	if len(entries) == 0 {
		return "", nil
	}

	var b strings.Builder
	for _, e := range entries {
		if e.Entry.Previous != "" {
			fmt.Fprintf(&b, "- %s = %s\n+ %s = %s\n", e.Path, e.Entry.Previous, e.Path, e.Entry.Value)
		} else {
			fmt.Fprintf(&b, "+ %s = %s\n", e.Path, e.Entry.Value)
		}
	}
	return b.String(), nil
}

// ChangeCount returns the number of pending changes for the user's session.
// Returns 0 if no session exists.
func (m *EditorManager) ChangeCount(username string) int {
	m.mu.RLock()
	us, ok := m.sessions[username]
	m.mu.RUnlock()

	if !ok {
		return 0
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	sid := us.editor.SessionID()
	if sid == "" {
		return 0
	}

	return len(us.editor.SessionChanges(sid))
}

// Tree returns the user's working configuration tree for rendering.
// Returns nil if no session exists.
func (m *EditorManager) Tree(username string) *config.Tree {
	m.mu.RLock()
	us, ok := m.sessions[username]
	m.mu.RUnlock()

	if !ok {
		return nil
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	return us.editor.Tree()
}

// ContentAtPath returns the serialized config content at the given context path
// for the user's working tree. Returns an empty string if no session exists.
func (m *EditorManager) ContentAtPath(username string, path []string) string {
	m.mu.RLock()
	us, ok := m.sessions[username]
	m.mu.RUnlock()

	if !ok {
		return ""
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	return us.editor.ContentAtPath(path)
}

// ActiveSessions returns a summary of active web editing sessions.
// Each entry is formatted as "user@web%timestamp - N pending changes".
func (m *EditorManager) ActiveSessions() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]string, 0, len(m.sessions))
	for _, us := range m.sessions {
		us.mu.Lock()
		sid := us.editor.SessionID()
		count := 0
		if sid != "" {
			count = len(us.editor.SessionChanges(sid))
		}
		us.mu.Unlock()

		changeWord := "changes"
		if count == 1 {
			changeWord = "change"
		}
		result = append(result, fmt.Sprintf("%s - %d pending %s", sid, count, changeWord))
	}
	return result
}

// evictInactive removes sessions with lastActivity older than idleTimeout.
// Caller MUST hold m.mu in write mode.
func (m *EditorManager) evictInactive() {
	cutoff := time.Now().Add(-m.idleTimeout)
	for name, us := range m.sessions {
		if us.lastActivity.Before(cutoff) {
			us.mu.Lock()
			if discardErr := us.editor.Discard(); discardErr != nil {
				serverLogger.Debug("evict discard failed", "user", name, "error", discardErr)
			}
			us.mu.Unlock()
			delete(m.sessions, name)
		}
	}
}
