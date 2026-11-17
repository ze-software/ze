// Design: docs/architecture/web-interface.md -- browser recall ownership and retirement.
package web

import (
	"errors"
	"io/fs"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

type browserHistoryStore struct {
	storage.Storage
	readFailure atomic.Bool
	entered     chan struct{}
	release     chan struct{}
	first       sync.Once
}

func (s *browserHistoryStore) ReadFile(key string) ([]byte, error) {
	if s.readFailure.Load() {
		if strings.HasPrefix(key, zefs.KeyHistory.Prefix()) {
			return nil, errors.New("history read unavailable")
		}
	}
	return s.Storage.ReadFile(key)
}

func (s *browserHistoryStore) WriteFile(key string, data []byte, mode fs.FileMode) error {
	if s.entered != nil {
		if strings.HasPrefix(key, zefs.KeyHistory.Prefix()) {
			s.first.Do(func() { close(s.entered); <-s.release })
		}
	}
	return s.Storage.WriteFile(key, data, mode)
}

// TestBrowserHistoryReceiptsSurviveInterleavedContinuation proves request
// receipts, rather than consecutive dedup, keep a load's second POST out once.
func TestBrowserHistoryReceiptsSurviveInterleavedContinuation(t *testing.T) {
	mgr := newTestEditorManager(t)
	require.NoError(t, mgr.RecordHistory("alice", cli.ModeConfig, nil, "load terminal relative merge", "load-one"))
	require.NoError(t, mgr.RecordHistory("alice", cli.ModeConfig, nil, "show", "show-two"))
	require.NoError(t, mgr.RecordHistory("alice", cli.ModeConfig, nil, "load terminal relative merge", "load-one"))
	entries, err := mgr.HistorySnapshot("alice", cli.ModeConfig)
	require.NoError(t, err)
	require.Equal(t, []string{"load terminal relative merge", "show"}, entries)
}

// TestBrowserHistoryOwnerIsIndependentOfEditorDiscard holds the history owner
// while actual editor discard and another admitted history record run together.
func TestBrowserHistoryOwnerIsIndependentOfEditorDiscard(t *testing.T) {
	mgr, _, _, _ := setupCLITerminalYANGTest(t)
	require.NoError(t, mgr.RecordHistory("alice", cli.ModeConfig, nil, "show", "one"))
	_, err := mgr.GetOrCreate("alice")
	require.NoError(t, err)
	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "5.6.7.8"))
	changePath := cli.ChangePath(mgr.configPath, "alice")
	require.True(t, mgr.store.Exists(changePath), "discard must start with durable candidate intent")
	answers := make(chan error, 2)
	go func() { answers <- mgr.Discard("alice") }()
	go func() { answers <- mgr.RecordHistory("alice", cli.ModeConfig, nil, "compare", "two") }()
	require.NoError(t, <-answers)
	require.NoError(t, <-answers)
	require.False(t, mgr.store.Exists(changePath), "discard must remove durable candidate intent")
	entries, err := mgr.HistorySnapshot("alice", cli.ModeConfig)
	require.NoError(t, err)
	require.Equal(t, []string{"show", "compare"}, entries)
}

// TestBrowserHistoryReadFailurePreservesAndReopensStoredRecall seeds actual
// durable history, refuses its read, and proves retry reopens without overwriting.
func TestBrowserHistoryReadFailurePreservesAndReopensStoredRecall(t *testing.T) {
	mgr := newTestEditorManager(t)
	key := zefs.KeyHistory.Prefix() + "alice/config"
	require.NoError(t, mgr.store.WriteFile(key, []byte("show saved"), 0o600))
	store := &browserHistoryStore{Storage: mgr.store}
	store.readFailure.Store(true)
	mgr.store = store
	require.Error(t, mgr.RecordHistory("alice", cli.ModeConfig, nil, "show refused", "one"))
	_, err := mgr.HistorySnapshot("alice", cli.ModeConfig)
	require.ErrorContains(t, err, "history read unavailable")
	store.readFailure.Store(false)
	entries, err := mgr.HistorySnapshot("alice", cli.ModeConfig)
	require.NoError(t, err)
	require.Equal(t, []string{"show saved"}, entries)
	data, err := store.ReadFile(key)
	require.NoError(t, err)
	require.Equal(t, "show saved", string(data))
}

// TestBrowserHistoryRetirementKeepsItsSlotUntilWriterDrains gates a real
// publication, then tries same-user reopen and another user at capacity.
func TestBrowserHistoryRetirementKeepsItsSlotUntilWriterDrains(t *testing.T) {
	mgr := newTestEditorManager(t)
	mgr.maxSessions = 1
	synctest.Test(t, func(t *testing.T) {
		store := &browserHistoryStore{Storage: mgr.store, entered: make(chan struct{}), release: make(chan struct{})}
		mgr.store = store
		require.NoError(t, mgr.RecordHistory("alice", cli.ModeOperational, nil, "show first", "one"))
		<-store.entered
		mgr.historyMu.Lock()
		mgr.histories["alice"].activity = time.Now().Add(-2 * time.Hour)
		mgr.historyMu.Unlock()
		second := make(chan error, 1)
		go func() { second <- mgr.RecordHistory("bob", cli.ModeOperational, nil, "show second", "two") }()
		synctest.Wait()
		_, err := mgr.HistorySnapshot("alice", cli.ModeOperational)
		require.ErrorContains(t, err, "still draining")
		require.ErrorContains(t, mgr.RecordHistory("charlie", cli.ModeOperational, nil, "show third", "three"), "capacity")
		mgr.historyMu.Lock()
		require.Len(t, mgr.histories, 1)
		mgr.historyMu.Unlock()
		close(store.release)
		require.NoError(t, <-second)
		require.NoError(t, mgr.Close())
		data, err := store.ReadFile(zefs.KeyHistory.Prefix() + "alice/operational")
		require.NoError(t, err)
		require.Equal(t, "show first", string(data))
	})
}

// TestBrowserHistoryBoundsAndModeUserIsolation exercises the shared max key,
// detached projections, receipt bound and reopening through the borrowed store.
func TestBrowserHistoryBoundsAndModeUserIsolation(t *testing.T) {
	mgr := newTestEditorManager(t)
	require.NoError(t, mgr.store.WriteFile(zefs.KeyHistoryMax.Pattern, []byte("2"), 0o600))
	for _, input := range []string{"show one", "show two", "show three"} {
		require.NoError(t, mgr.RecordHistory("alice", cli.ModeOperational, nil, input, input[5:]))
	}
	require.NoError(t, mgr.RecordHistory("alice", cli.ModeConfig, nil, "compare", "config"))
	require.NoError(t, mgr.RecordHistory("bob", cli.ModeOperational, nil, "show bob", "bob"))
	entries, err := mgr.HistorySnapshot("alice", cli.ModeOperational)
	require.NoError(t, err)
	require.Equal(t, []string{"show two", "show three"}, entries)
	entries[0] = "caller changed its copy"
	entries, err = mgr.HistorySnapshot("alice", cli.ModeOperational)
	require.NoError(t, err)
	require.Equal(t, "show two", entries[0])
	mgr.historyMu.Lock()
	require.Len(t, mgr.histories["alice"].receipts, 2)
	mgr.historyMu.Unlock()
	require.NoError(t, mgr.Close())
	reopened := NewEditorManager(mgr.store, mgr.configPath, mgr.schema, testEditorFactory(), testEditSessionFactory())
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
	entries, err = reopened.HistorySnapshot("alice", cli.ModeConfig)
	require.NoError(t, err)
	require.Equal(t, []string{"compare"}, entries)
	entries, err = reopened.HistorySnapshot("bob", cli.ModeOperational)
	require.NoError(t, err)
	require.Equal(t, []string{"show bob"}, entries)
}
