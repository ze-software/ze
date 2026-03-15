// Design: docs/architecture/config/yang-config-design.md — session identity for concurrent editing
// Related: editor.go — config editor (uses EditSession for write-through)

package cli

import (
	"fmt"
	"time"
)

// EditSession represents an editing session identity for concurrent config editing.
// Each editor instance gets a unique session, used to track authorship in the draft file.
type EditSession struct {
	User      string    // User identifier (e.g., "thomas")
	Origin    string    // Origin: "local" for terminal, "ssh" for SSH sessions
	ID        string    // Full session ID: "user@origin:unix-ts"
	StartTime time.Time // When the session was created
}

// NewEditSession creates a new editing session with the given user and origin.
// The session ID is formatted as "user@origin:unix-ts" using the current time.
func NewEditSession(user, origin string) *EditSession {
	now := time.Now()
	return &EditSession{
		User:      user,
		Origin:    origin,
		ID:        fmt.Sprintf("%s@%s:%d", user, origin, now.Unix()),
		StartTime: now,
	}
}

// UserAtOrigin returns "user@origin" for metadata prefixes.
func (s *EditSession) UserAtOrigin() string {
	return fmt.Sprintf("%s@%s", s.User, s.Origin)
}

// DraftPath returns the draft file path for a given config path (appends ".draft").
func DraftPath(configPath string) string {
	return configPath + ".draft"
}

// LockPath returns the lock file path for a given config path (appends ".lock").
func LockPath(configPath string) string {
	return configPath + ".lock"
}
