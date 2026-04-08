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

// Conflict represents a merge conflict in a config commit.
type Conflict struct {
	Path       string
	MyValue    string
	OtherUser  string
	OtherValue string
}

// CommitResult holds the outcome of a config commit.
type CommitResult struct {
	Conflicts []Conflict
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
type SessionModelFactory func(username string) tea.Model

// LoginWarningsFunc returns login warnings for the SSH banner.
type LoginWarningsFunc func() []LoginWarning
