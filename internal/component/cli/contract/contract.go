// Design: docs/architecture/core-design.md -- component boundaries (cli/contract)
//
// Package contract defines interfaces and types for the cli component's
// consumers (ssh, web) without depending on cli's concrete implementation.
// cli implements these interfaces; hub injects concrete instances at startup.
//
// This package has zero imports from internal/component/* to ensure it is
// a true leaf dependency that any component can import safely.
package contract

import (
	"context"

	tea "charm.land/bubbletea/v2"
)

// LoginWarning holds a warning message for the SSH login banner.
type LoginWarning struct {
	Message string
	Command string
}

// EditSession identifies a concurrent editing session.
type EditSession struct {
	User      string
	Origin    string
	ID        string
	StartTime string
}

// Completion represents a single completion suggestion.
type Completion struct {
	Text        string
	Description string
	Type        string
}

// ConflictType distinguishes live vs stale conflicts.
type ConflictType int

const (
	// ConflictLive means another session has a pending change at the same path.
	ConflictLive ConflictType = iota
	// ConflictStale means the committed value changed since this session started.
	ConflictStale
)

// Conflict represents a merge conflict in a config commit.
type Conflict struct {
	Path          string
	Type          ConflictType
	MyValue       string
	OtherUser     string
	OtherValue    string
	PreviousValue string
}

// CommitResult holds the outcome of a config commit.
type CommitResult struct {
	Conflicts        []Conflict
	Applied          int
	MigrationWarning string
}

// MonitorSession represents an active streaming monitor.
type MonitorSession struct {
	EventChan  <-chan string
	Cancel     context.CancelFunc
	FormatFunc func(string) string
}

// MonitorFactory creates monitor sessions for streaming event display.
type MonitorFactory func(ctx context.Context, args []string) (*MonitorSession, error)

// DashboardFactory creates a dashboard poller function.
type DashboardFactory func() (func() (string, error), error)

// SessionModelFactory creates a bubbletea Model for an SSH session.
// The returned model handles editor, command mode, monitor, and dashboard.
type SessionModelFactory func(username, remoteAddr string) tea.Model

// LoginWarningsFunc returns login warnings for the SSH banner.
type LoginWarningsFunc func() []LoginWarning

// SessionChange represents a single tracked change in an editing session.
type SessionChange struct {
	Path     string
	Previous string
	Value    string
}

// PendingChangeKind identifies the operator-visible change type.
type PendingChangeKind string

const (
	PendingChangeSet    PendingChangeKind = "set"
	PendingChangeDelete PendingChangeKind = "delete"
	PendingChangeRename PendingChangeKind = "rename"
)

// PendingChange is the unified pending-change view used by web diff/count UI.
type PendingChange struct {
	Kind     PendingChangeKind
	Path     string
	Previous string
	Value    string
	OldPath  string
	NewPath  string
}

// Editor provides config editing operations for the web UI.
// Implemented by cli.Editor; consumed by web's EditorManager.
type Editor interface {
	SetSession(s EditSession)
	SessionID() string
	CreateEntry(path []string) error
	SetValue(path []string, key, value string) error
	DeleteValue(path []string, key string) error
	RenameListEntry(parentPath []string, listName, oldKey, newKey string) error
	CommitSession() (*CommitResult, error)
	Discard() error
	Diff() string
	// Tree returns the parsed config tree (concrete *config.Tree).
	// Returned as any to avoid contract importing config.
	Tree() any
	ContentAtPath(path []string) string
	SessionChanges(sessionID string) []SessionChange
	PendingChanges(sessionID string) []PendingChange
}

// EditorFactory creates a new Editor backed by storage.
// store and configPath are the backing storage and config file path.
type EditorFactory func(store any, configPath string) (Editor, error)

// EditSessionFactory creates a new EditSession for the given user and origin.
type EditSessionFactory func(username, origin string) EditSession

// Completer provides config path completion for the web CLI bar.
type Completer interface {
	// SetTree sets the config tree for data-aware completion (any = *config.Tree).
	SetTree(tree any)
	Complete(input string, contextPath []string) []Completion
}
