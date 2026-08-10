// Design: docs/architecture/config/yang-config-design.md — session identity for concurrent editing
// Related: editor.go — config editor (uses EditSession for write-through)
// Related: editor_annotated.go — annotated view and show column preferences

package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var errEmptyUser = errors.New("empty user")

// EditSession represents an editing session identity for concurrent config editing.
// Each editor instance gets a unique session, used to track authorship in the draft file.
type EditSession struct {
	User      string    // User identifier (e.g., "thomas")
	Origin    string    // Origin: "local" for terminal, "ssh" for SSH sessions
	ID        string    // Full session ID matching MetaEntry.SessionKey(): "user@origin%RFC3339time"
	StartTime time.Time // When the session was created
}

// NewEditSession creates a new editing session with the given user and origin.
// The session ID matches MetaEntry.SessionKey() format: "user@origin%RFC3339time".
// The user is sanitized via filepath.Base to prevent path traversal.
// Callers should validate user with ValidateUser at input boundaries.
func NewEditSession(user, origin string) *EditSession {
	safe := sanitizeUser(user)
	now := time.Now()
	var tb textbuf.Buffer
	return &EditSession{
		User:      safe,
		Origin:    origin,
		ID:        tb.Str(safe).Byte('@').Str(origin).Byte('%').Str(now.UTC().Format(time.RFC3339)).String(),
		StartTime: now,
	}
}

// UserAtOrigin returns "user@origin" for metadata prefixes.
func (s *EditSession) UserAtOrigin() string {
	var tb textbuf.Buffer
	return tb.Str(s.User).Byte('@').Str(s.Origin).String()
}

// OrphanedSessions filters a list of session IDs to those belonging to the same
// user and origin as this session but with a different timestamp (i.e., from a
// previous session). Uses "user@origin%" prefix matching -- the % delimiter
// ensures "thomas@local" does not match "thomasmore@local".
func (s *EditSession) OrphanedSessions(allSessions []string) []string {
	var tb textbuf.Buffer
	prefix := tb.Str(s.UserAtOrigin()).Byte('%').String()
	var orphans []string
	for _, sid := range allSessions {
		if strings.HasPrefix(sid, prefix) && sid != s.ID {
			orphans = append(orphans, sid)
		}
	}
	return orphans
}

// DraftPath returns the draft file path for a given config path (appends ".draft").
func DraftPath(configPath string) string {
	var tb textbuf.Buffer
	return tb.Str(configPath).Str(".draft").String()
}

// LockPath returns the lock file path for a given config path (appends ".lock").
func LockPath(configPath string) string {
	var tb textbuf.Buffer
	return tb.Str(configPath).Str(".lock").String()
}

// ChangePath returns the per-user change file path for a given config path and user.
// Uses filepath.Base on the user to strip directory traversal.
func ChangePath(configPath, user string) string {
	var tb textbuf.Buffer
	return tb.Str(configPath).Str(".change.").Str(sanitizeUser(user)).String()
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
		return errEmptyUser
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

// changePrefix returns the filename prefix for scanning all change files.
// Used with store.List(dir) to filter change files from other files.
func changePrefix(configPath string) string {
	var tb textbuf.Buffer
	return tb.Str(filepath.Base(configPath)).Str(".change.").String()
}

// changeUser extracts the username from a change file path.
// Returns empty string if the path is not a valid change file.
func changeUser(configPath, changeFilePath string) string {
	prefix := changePrefix(configPath)
	base := filepath.Base(changeFilePath)
	if !strings.HasPrefix(base, prefix) {
		return ""
	}
	return strings.TrimPrefix(base, prefix)
}
