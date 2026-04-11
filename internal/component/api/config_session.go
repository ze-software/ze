// Design: docs/architecture/api/architecture.md -- API config session manager
// Related: engine.go -- engine that manages config sessions
// Related: types.go -- shared types (AuthContext)

package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrSessionForbidden is returned when a user tries to access another user's session.
var ErrSessionForbidden = errors.New("session access forbidden")

// ConfigEditor abstracts the config editing operations the engine needs.
// Implemented by the composition root using the real Editor.
type ConfigEditor interface {
	SetValue(path []string, key, value string) error
	DeleteByPath(fullPath []string) error
	Diff() string
	Save() error
	Discard() error
	WorkingContent() string
}

// ConfigEditorFactory creates a new ConfigEditor for a session.
type ConfigEditorFactory func() (ConfigEditor, error)

// ConfigSession tracks a single config editing session.
type ConfigSession struct {
	ID        string
	Editor    ConfigEditor
	Username  string
	CreatedAt time.Time
}

// DefaultSessionTimeout is the maximum age of an idle config session.
const DefaultSessionTimeout = 30 * time.Minute

// ConfigSessionManager manages config editing sessions for API use.
// Each session wraps a ConfigEditor with a unique ID and tracks ownership.
// Thread-safe for concurrent access.
// Sessions that exceed the timeout are discarded by CleanExpired.
type ConfigSessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*ConfigSession
	factory  ConfigEditorFactory
	timeout  time.Duration
}

// NewConfigSessionManager creates a session manager with the default timeout.
func NewConfigSessionManager(factory ConfigEditorFactory) *ConfigSessionManager {
	return &ConfigSessionManager{
		sessions: make(map[string]*ConfigSession),
		factory:  factory,
		timeout:  DefaultSessionTimeout,
	}
}

// CleanExpired discards sessions older than the configured timeout.
// Returns the number of sessions cleaned. Safe for concurrent use.
func (m *ConfigSessionManager) CleanExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-m.timeout)
	var cleaned int
	for id, s := range m.sessions {
		if s.CreatedAt.Before(cutoff) {
			_ = s.Editor.Discard()
			delete(m.sessions, id)
			cleaned++
		}
	}
	return cleaned
}

// Enter creates a new config editing session.
func (m *ConfigSessionManager) Enter(username string) (string, error) {
	editor, err := m.factory()
	if err != nil {
		return "", fmt.Errorf("create editor: %w", err)
	}

	id, idErr := generateSessionID()
	if idErr != nil {
		return "", fmt.Errorf("generate session ID: %w", idErr)
	}

	m.mu.Lock()
	m.sessions[id] = &ConfigSession{
		ID:        id,
		Editor:    editor,
		Username:  username,
		CreatedAt: time.Now(),
	}
	m.mu.Unlock()

	return id, nil
}

// Set modifies a config path in the session's candidate.
// username must match the session owner.
func (m *ConfigSessionManager) Set(username, sessionID, path, value string) error {
	session, err := m.get(username, sessionID)
	if err != nil {
		return err
	}
	parts := splitPath(path)
	if len(parts) < 2 { //nolint:mnd // path needs at least parent + leaf
		return fmt.Errorf("path too short: %q", path)
	}
	return session.Editor.SetValue(parts[:len(parts)-1], parts[len(parts)-1], value)
}

// Delete removes a config path from the session's candidate.
// username must match the session owner.
func (m *ConfigSessionManager) Delete(username, sessionID, path string) error {
	session, err := m.get(username, sessionID)
	if err != nil {
		return err
	}
	return session.Editor.DeleteByPath(splitPath(path))
}

// Diff returns the pending changes for a session.
// username must match the session owner.
func (m *ConfigSessionManager) Diff(username, sessionID string) (string, error) {
	session, err := m.get(username, sessionID)
	if err != nil {
		return "", err
	}
	return session.Editor.Diff(), nil
}

// Commit applies the pending changes.
// username must match the session owner.
func (m *ConfigSessionManager) Commit(username, sessionID string) error {
	session, err := m.get(username, sessionID)
	if err != nil {
		return err
	}
	if saveErr := session.Editor.Save(); saveErr != nil {
		return fmt.Errorf("commit: %w", saveErr)
	}
	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()
	return nil
}

// Discard throws away the session's candidate changes.
// username must match the session owner.
func (m *ConfigSessionManager) Discard(username, sessionID string) error {
	session, err := m.get(username, sessionID)
	if err != nil {
		return err
	}
	if discardErr := session.Editor.Discard(); discardErr != nil {
		return fmt.Errorf("discard: %w", discardErr)
	}
	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()
	return nil
}

// get retrieves a session by ID, verifying the username owns it.
func (m *ConfigSessionManager) get(username, id string) (*ConfigSession, error) {
	m.mu.RLock()
	session, ok := m.sessions[id]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("session %q not found", id)
	}
	if session.Username != username {
		return nil, ErrSessionForbidden
	}
	return session, nil
}

// generateSessionID creates a random 8-byte hex session ID.
func generateSessionID() (string, error) {
	b := make([]byte, 8) //nolint:mnd // 8 bytes = 16 hex chars
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// splitPath splits a dot-separated config path into parts.
func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	var parts []string
	start := 0
	for i := range path {
		if path[i] == '.' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		parts = append(parts, path[start:])
	}
	return parts
}
