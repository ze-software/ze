// Design: docs/architecture/web-interface.md -- Per-user editor management
// Related: handler_config.go -- Config view and edit handlers
// Related: auth.go -- Session authentication

package web

import (
	"fmt"
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
	delete(m.sessions, username)
	m.mu.Unlock()

	return nil
}

// Diff returns a textual diff of the user's pending changes against the original.
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

	return us.editor.Diff(), nil
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

// evictInactive removes sessions with lastActivity older than idleTimeout.
// Caller MUST hold m.mu in write mode.
func (m *EditorManager) evictInactive() {
	cutoff := time.Now().Add(-m.idleTimeout)
	for name, us := range m.sessions {
		if us.lastActivity.Before(cutoff) {
			delete(m.sessions, name)
		}
	}
}
