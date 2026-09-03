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
	"time"
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
//
// Description is the one-line summary the menu row shows. LongHelp is the long
// explanation the ? box shows, and it is empty when the node declares none. The
// two texts are declared separately, in the YANG description statement and the
// ze:help extension, and neither is derived from the other.
type Completion struct {
	Text        string
	Description string
	LongHelp    string
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
// When RenderFunc is set, the TUI renders full-screen (alt-screen)
// instead of appending events to the scrollback viewport.
type MonitorSession struct {
	EventChan  <-chan string
	Cancel     context.CancelFunc
	FormatFunc func(string) string
	RenderFunc func(width, height int) string
}

// MonitorFactory creates monitor sessions for streaming event display.
type MonitorFactory func(ctx context.Context, args []string) (*MonitorSession, error)

// DashboardFactory creates a dashboard poller function.
type DashboardFactory func() (func() (string, error), error)

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
	PendingChangeSet        PendingChangeKind = "set"
	PendingChangeDelete     PendingChangeKind = "delete"
	PendingChangeRename     PendingChangeKind = "rename"
	PendingChangeDeactivate PendingChangeKind = "deactivate"
	PendingChangeActivate   PendingChangeKind = "activate"
)

// PendingChange is the unified pending-change view used by web diff/count UI.
// Member is set for leaf-list member operations (one member added, removed,
// deactivated, or activated).
type PendingChange struct {
	Kind     PendingChangeKind
	Path     string
	Previous string
	Value    string
	OldPath  string
	NewPath  string
	Member   string
}

// BackupInfo describes one config backup entry.
type BackupInfo struct {
	Path      string
	Timestamp string
}

// Editor provides config editing operations for the web UI.
// Implemented by cli.Editor; consumed by web's EditorManager.
// Every command the SSH CLI supports must be callable through this interface
// so both CLIs share the same code path.
type Editor interface {
	SetSession(s EditSession)
	SessionID() string
	CreateEntry(path []string) error
	SetValue(path []string, key, value string) error
	DeleteValue(path []string, key string) error
	// DeleteByPath deletes whatever the schema says lives at fullPath: a leaf,
	// a leaf-list member, a container, a whole list, or one list entry. Callers
	// that hold a full path MUST prefer it over DeleteValue, which can only
	// remove a scalar leaf and silently no-ops on everything else.
	DeleteByPath(fullPath []string) error
	RenameListEntry(parentPath []string, listName, oldKey, newKey string) error
	CopyListEntry(parentPath []string, listName, srcKey, dstKey string) error
	DeactivatePath(path []string) error
	ActivatePath(path []string) error
	CommitSession() (*CommitResult, error)
	CommitSessionCandidate(stamp time.Time) (*CommitResult, string, error)
	MarkCommittedContent(content string)
	Discard() error
	DiscardSessionPath(path []string) error
	DisconnectSession(sessionID string) error
	// Diff is the line diff between the committed config and the working
	// config, with every secret leaf masked. It names a secret whose value
	// moved, and it publishes neither value.
	Diff() string
	SaveDraft() error
	SetPreCommitValidate(fn func(candidate string) error)
	ListBackups() ([]BackupInfo, error)
	Rollback(backupPath string) error
	// Tree returns the parsed config tree (concrete *config.Tree).
	// Returned as any to avoid contract importing config.
	Tree() any
	ContentAtPath(path []string) string
	// DisplayContentAtPath is ContentAtPath with every secret leaf masked for
	// display, through config.MaskSecrets. config.LeafHoldsSecret is the one
	// predicate that says which leaf that is. ContentAtPath stays unmasked for
	// validation and persistence.
	DisplayContentAtPath(path []string) string
	OriginalContentAtPath(path []string) string
	Dirty() bool
	SessionChanges(sessionID string) []SessionChange
	PendingChanges(sessionID string) []PendingChange
	ActiveSessions() []string
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
