// Design: docs/architecture/config/yang-config-design.md — session identity for concurrent editing
// Related: editor.go — config editor (uses EditSession for write-through)

package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// EditSession represents an editing session identity for concurrent config editing.
// Each editor instance gets a unique session, used to track authorship in the draft file.
type EditSession struct {
	User      string    // User identifier (e.g., "thomas")
	Origin    string    // Origin: "local" for terminal, "ssh" for SSH sessions
	ID        string    // Full session ID matching MetaEntry.SessionKey(): "user@origin:RFC3339time"
	StartTime time.Time // When the session was created
}

// NewEditSession creates a new editing session with the given user and origin.
// The session ID matches MetaEntry.SessionKey() format: "user@origin:RFC3339time".
// The user is sanitized via filepath.Base to prevent path traversal.
// Callers should validate user with ValidateUser at input boundaries.
func NewEditSession(user, origin string) *EditSession {
	safe := sanitizeUser(user)
	now := time.Now()
	return &EditSession{
		User:      safe,
		Origin:    origin,
		ID:        fmt.Sprintf("%s@%s%%%s", safe, origin, now.UTC().Format(time.RFC3339)),
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

// ChangePath returns the per-user change file path for a given config path and user.
// Uses filepath.Base on the user to strip directory traversal.
func ChangePath(configPath, user string) string {
	return configPath + ".change." + sanitizeUser(user)
}

// sanitizeUser resolves a username to a safe filename component.
// Uses filepath.Base to strip directory traversal (e.g., "../../../etc/passwd" → "passwd").
// Returns "unknown" for empty, ".", or ".." results.
func sanitizeUser(user string) string {
	if user == "" {
		return "unknown"
	}
	base := filepath.Base(user)
	if base == "." || base == ".." || base == "/" {
		return "unknown"
	}
	return base
}

// ValidateUser checks whether a user string is safe for use as a change file identifier.
// Only alphanumeric characters, hyphens, underscores, and dots are allowed.
// Returns an error for empty strings, "..", or any character outside the whitelist.
func ValidateUser(user string) error {
	if user == "" {
		return fmt.Errorf("empty user")
	}
	if user == "." || user == ".." {
		return fmt.Errorf("invalid user: %q", user)
	}
	for _, r := range user {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.'
		if !ok {
			return fmt.Errorf("user contains invalid character %q: %q", string(r), user)
		}
	}
	return nil
}

// ChangePrefix returns the filename prefix for scanning all change files.
// Used with store.List(dir) to filter change files from other files.
func ChangePrefix(configPath string) string {
	return filepath.Base(configPath) + ".change."
}

// ChangeUser extracts the username from a change file path.
// Returns empty string if the path is not a valid change file.
func ChangeUser(configPath, changeFilePath string) string {
	prefix := ChangePrefix(configPath)
	base := filepath.Base(changeFilePath)
	if !strings.HasPrefix(base, prefix) {
		return ""
	}
	return strings.TrimPrefix(base, prefix)
}
