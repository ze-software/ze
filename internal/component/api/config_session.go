// Design: docs/architecture/api/architecture.md -- API config session manager
// Related: engine.go -- engine that manages config sessions
// Related: types.go -- shared types (CallerIdentity)

package api

import (
	"context"
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

// ConfigCommitHook applies a saved config to the running daemon.
type ConfigCommitHook func() error

// ConfigSession tracks a single config editing session.
type ConfigSession struct {
	ID           string
	Editor       ConfigEditor
	Username     string
	CreatedAt    time.Time
	LastActivity time.Time
}

// DefaultSessionTimeout is the maximum age of an idle config session.
const DefaultSessionTimeout = 30 * time.Minute

// ConfigSessionManager manages config editing sessions for API use.
// Each session wraps a ConfigEditor with a unique ID and tracks ownership.
// Thread-safe for concurrent access.
// Idle sessions are automatically reaped by a background goroutine started
// via RunCleanup. Sessions idle beyond the timeout are discarded.
type ConfigSessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*ConfigSession
	factory  ConfigEditorFactory
	onCommit ConfigCommitHook
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

// SetCommitHook sets the hook called after a session save succeeds and before
// the session is removed. A hook error is returned to the client so API commits
// do not report success while runtime state remains unchanged.
func (m *ConfigSessionManager) SetCommitHook(hook ConfigCommitHook) {
	m.mu.Lock()
	m.onCommit = hook
	m.mu.Unlock()
}

// CleanExpired discards sessions idle beyond the configured timeout.
// Returns the number of sessions cleaned. Safe for concurrent use.
func (m *ConfigSessionManager) CleanExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-m.timeout)
	var cleaned int
	for id, s := range m.sessions {
		if s.LastActivity.Before(cutoff) {
			_ = s.Editor.Discard()
			delete(m.sessions, id)
			cleaned++
		}
	}
	return cleaned
}

// RunCleanup starts a background goroutine that periodically reaps idle
// sessions. Blocks until ctx is canceled. Call as `go m.RunCleanup(ctx)`.
func (m *ConfigSessionManager) RunCleanup(ctx context.Context) {
	ticker := time.NewTicker(m.timeout / 2)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.CleanExpired()
		}
	}
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

	now := time.Now()
	m.mu.Lock()
	m.sessions[id] = &ConfigSession{
		ID:           id,
		Editor:       editor,
		Username:     username,
		CreatedAt:    now,
		LastActivity: now,
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
	m.mu.RLock()
	onCommit := m.onCommit
	m.mu.RUnlock()
	if onCommit != nil {
		if hookErr := onCommit(); hookErr != nil {
			return fmt.Errorf("commit saved to disk but runtime reload failed (config file may diverge from running state): %w", hookErr)
		}
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
	m.mu.Lock()
	session, ok := m.sessions[id]
	if ok {
		session.LastActivity = time.Now()
	}
	m.mu.Unlock()
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
