// Design: docs/architecture/web-interface.md -- bounded authenticated recall owners.
// Related: editor.go -- the editor cache has a separate discard/eviction lifetime.
package web

import (
	"errors"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
)

// historyOwner serializes History's mutable snapshots and a bounded receipt
// window. references, retired and activity belong to EditorManager.historyMu.
// closeOnce joins the lazy History worker, including concurrent manager close.
type historyOwner struct {
	mu         sync.Mutex
	history    *cli.History
	receipts   map[string]struct{}
	order      []string
	references int
	retired    bool
	activity   time.Time
	username   string
	closeOnce  sync.Once
	closeError error
}

// HistorySnapshot returns detached, masked recall for an authenticated owner.
// The caller MUST derive username from trusted authentication, never a query.
// Safe for concurrent use. A failed read or known write failure remains visible.
func (m *EditorManager) HistorySnapshot(username string, mode cli.EditorMode) ([]string, error) {
	owner, err := m.acquireHistory(username)
	if err != nil {
		return nil, historySnapshotFailure(err)
	}
	defer m.releaseHistory(owner)
	owner.mu.Lock()
	defer owner.mu.Unlock()
	entries, err := owner.history.Snapshot(mode)
	if err != nil {
		if validHistoryMode(mode) {
			m.historyMu.Lock()
			owner.retired = true
			m.historyMu.Unlock()
		}
		return nil, historySnapshotFailure(err)
	}
	if err := owner.history.LastWriteError(); err != nil {
		return nil, historySnapshotFailure(err)
	}
	return entries, nil
}

// historySnapshotFailure owns the browser snapshot refusal, including joined
// storage failures. It retains every cause rather than selecting one member
// of a joined error and hiding the other mode's failure.
func historySnapshotFailure(cause error) *opererr.Error {
	return opererr.Wrap(cause, "Browser command recall could not be loaded.",
		"Saved commands cannot be returned; command execution remains available.",
		"Reconnect after the configuration store is available to restore command recall")
}

// RecordHistory records command once within the bounded submission receipt
// window. The caller MUST pass only command text, never uploaded content, and
// MUST use the effective mode returned by cli.HistoryCommandMode. Empty receipt
// uses History's ordinary consecutive dedup. Safe for concurrent use.
func (m *EditorManager) RecordHistory(username string, mode cli.EditorMode, path []string, command, receipt string) error {
	if !validHistoryReceipt(receipt) {
		return historyUnavailable("The command recall submission identifier is invalid.")
	}
	owner, err := m.acquireHistory(username)
	if err != nil {
		return err
	}
	defer m.releaseHistory(owner)
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if receipt != "" {
		if _, found := owner.receipts[receipt]; found {
			return owner.history.LastWriteError()
		}
	}
	if _, err := owner.history.Record(mode, path, command); err != nil {
		return err
	}
	if receipt != "" {
		if len(owner.order) == owner.history.Max() {
			delete(owner.receipts, owner.order[0])
			owner.order = owner.order[1:]
		}
		owner.receipts[receipt] = struct{}{}
		owner.order = append(owner.order, receipt)
	}
	return owner.history.LastWriteError()
}

// acquireHistory pins an owner before releasing the cache lock. An idle owner
// keeps its slot and username tombstone until its writer joins outside that
// lock. Two attempts allow one retirement without an unbounded retry under load.
func (m *EditorManager) acquireHistory(username string) (*historyOwner, error) {
	if username == "" {
		return nil, historyUnavailable("Command recall requires an authenticated user.")
	}
	for range 2 {
		m.historyMu.Lock()
		if m.historyClosed {
			m.historyMu.Unlock()
			return nil, historyUnavailable("The browser command history is closing.")
		}
		if owner, found := m.histories[username]; found {
			if owner.retired {
				m.historyMu.Unlock()
				return nil, historyUnavailable("Your previous command history is still draining.")
			}
			owner.references++
			owner.activity = time.Now()
			m.historyMu.Unlock()
			return owner, nil
		}
		if len(m.histories) < m.maxSessions {
			history, err := cli.NewHistoryWithMask(m.store, username, func(input string, path []string) string {
				return config.DisplayCommandWithSchema(input, path, m.schema)
			})
			if err != nil {
				m.historyMu.Unlock()
				return nil, err
			}
			owner := &historyOwner{history: history, receipts: make(map[string]struct{}), references: 1, activity: time.Now(), username: username}
			m.histories[username] = owner
			m.historyMu.Unlock()
			return owner, nil
		}
		name, retiring := m.retireHistory()
		m.historyMu.Unlock()
		if retiring == nil {
			return nil, historyUnavailable("The browser command-history capacity is in use.")
		}
		err := retiring.close()
		m.historyMu.Lock()
		delete(m.histories, name)
		if err != nil {
			m.historyRetirementError = err
			serverLogger.Warn("browser command history retirement failed", "code", diagnostic.CodeDoctorCLIHistoryWrite, "fix", "ze show doctor", "user", name, "error", err)
		}
		m.historyMu.Unlock()
	}
	return nil, historyUnavailable("The browser command-history capacity changed while reopening it.")
}

// retireHistory MUST hold historyMu. Only an unpinned idle owner can retire.
func (m *EditorManager) retireHistory() (string, *historyOwner) {
	cutoff := time.Now().Add(-m.idleTimeout)
	for name, owner := range m.histories {
		if owner.references != 0 {
			continue
		}
		if owner.retired {
			continue
		}
		if owner.activity.Before(cutoff) {
			owner.retired = true
			return name, owner
		}
	}
	return "", nil
}

func (m *EditorManager) releaseHistory(owner *historyOwner) {
	m.historyMu.Lock()
	owner.references--
	owner.activity = time.Now()
	retire := owner.retired && owner.references == 0
	m.historyMu.Unlock()
	if retire {
		err := owner.close()
		m.historyMu.Lock()
		if m.histories[owner.username] == owner {
			delete(m.histories, owner.username)
		}
		if err != nil {
			m.historyRetirementError = err
			serverLogger.Warn("browser command history recovery close failed", "code", diagnostic.CodeDoctorCLIHistoryWrite, "fix", "ze show doctor", "error", err)
		}
		m.historyMu.Unlock()
	}
}

func validHistoryMode(mode cli.EditorMode) bool {
	switch mode {
	case cli.ModeConfig, cli.ModeOperational:
		return true
	default:
		return false
	}
}

func historyUnavailable(problem string) error {
	return opererr.New(problem, "Command execution remains available, but recall could not be opened.", "Reconnect after the configuration store is available to restore command recall")
}

func (o *historyOwner) close() error {
	o.closeOnce.Do(func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		o.history.Close()
		o.closeError = o.history.LastWriteError()
	})
	return o.closeError
}

// Close seals history admission and joins every accepted publication. It never
// discards durable candidate intent or closes the borrowed store. The service
// MUST join admitted HTTP handlers before Close and MUST wait for Close before
// releasing the store. A stuck store call may keep Close blocked; its service
// owner bounds that wait and reports failure. Safe for concurrent use.
func (m *EditorManager) Close() error {
	m.historyCloseOnce.Do(func() {
		m.historyMu.Lock()
		m.historyClosed = true
		owners := make([]*historyOwner, 0, len(m.histories))
		for _, owner := range m.histories {
			owners = append(owners, owner)
		}
		retirementError := m.historyRetirementError
		m.historyMu.Unlock()
		failures := []error{retirementError}
		for _, owner := range owners {
			failures = append(failures, owner.close())
		}
		m.historyCloseError = errors.Join(failures...)
	})
	return m.historyCloseError
}
