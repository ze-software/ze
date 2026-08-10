// Design: docs/architecture/core-design.md -- audit component
// Related: store.go -- JSON-lines persistence
// Related: query.go -- audit entry filtering

// Package audit provides Ze's local, append-only structured audit log.
package audit

import (
	"fmt"
	"sync"
	"time"
)

const (
	MinMaxEntries     = 100
	DefaultMaxEntries = 10000
	MaxMaxEntries     = 100000
)

const (
	ActionConfigCommit   = "config-commit"
	ActionConfigDiscard  = "config-discard"
	ActionConfigDownload = "config-download"
	ActionConfigUpload   = "config-upload"
	ActionDaemonReload   = "daemon-reload"
	ActionAuthFail       = "auth-fail"
)

const (
	OutcomeSuccess = "success"
	OutcomeDenied  = "denied"
	OutcomeError   = "error"
)

const (
	API    = "api"
	CLI    = "cli"
	GRPC   = "grpc"
	MCP    = "mcp"
	REST   = "rest"
	SSH    = "ssh"
	System = "system"
	Web    = "web"
)

// Entry is one structured audit record.
type Entry struct {
	Timestamp  time.Time `json:"timestamp"`
	Actor      string    `json:"actor,omitempty"`
	RemoteAddr string    `json:"remote-addr,omitempty"`
	Surface    string    `json:"surface"`
	Action     string    `json:"action"`
	Detail     string    `json:"detail,omitempty"`
	Outcome    string    `json:"outcome"`
}

// Recorder accepts audit entries. Implementations must not block user-facing
// operations on best-effort audit I/O failures.
type Recorder interface {
	Record(Entry) error
}

// Log stores audit entries in memory and optionally mirrors them to disk.
type Log struct {
	mu         sync.Mutex
	path       string
	maxEntries int
	entries    []Entry
}

// validateMaxEntries verifies the configured audit retention bound.
func validateMaxEntries(maxEntries int) error {
	if maxEntries < MinMaxEntries {
		return fmt.Errorf("audit max entries must be >= %d", MinMaxEntries)
	}
	if maxEntries > MaxMaxEntries {
		return fmt.Errorf("audit max entries must be <= %d", MaxMaxEntries)
	}
	return nil
}

// NewMemory creates an in-memory audit log.
func NewMemory(maxEntries int) (*Log, error) {
	if err := validateMaxEntries(maxEntries); err != nil {
		return nil, err
	}
	return &Log{maxEntries: maxEntries}, nil
}

// Record appends one audit record.
func (l *Log) Record(entry Entry) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	} else {
		entry.Timestamp = entry.Timestamp.UTC()
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = append(l.entries, entry)
	if len(l.entries) > l.maxEntries {
		copy(l.entries, l.entries[len(l.entries)-l.maxEntries:])
		l.entries = l.entries[:l.maxEntries]
	}
	if l.path == "" {
		return nil
	}
	return l.persistLocked()
}
