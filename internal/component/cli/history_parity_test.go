// Design: docs/guide/config-editor.md -- safe shared command recall.
package cli

import (
	"errors"
	"io/fs"
	"strings"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config/storage"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

type unavailableHistoryStore struct {
	storage.Storage
	failure error
	writes  atomic.Int64
}

// TestHistoryModeSnapshotsAreBoundedAndDetached checks the browser-facing
// shared API without a second history owner or a second persistence format.
func TestHistoryModeSnapshotsAreBoundedAndDetached(t *testing.T) {
	store := storage.CreateMemory()
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.WriteFile("meta/history/max", []byte("2"), 0))
	h := NewHistory(store, "parity")
	t.Cleanup(h.Close)
	for _, line := range []string{"show one", "show one", "show two", "show three"} {
		_, err := h.Record(ModeConfig, nil, line)
		require.NoError(t, err)
	}
	_, err := h.Record(ModeOperational, nil, "show operational")
	require.NoError(t, err)
	config, err := h.Snapshot(ModeConfig)
	require.NoError(t, err)
	assert.Equal(t, []string{"show two", "show three"}, config)
	config[0] = "caller changed its copy"
	config, err = h.Snapshot(ModeConfig)
	require.NoError(t, err)
	assert.Equal(t, []string{"show two", "show three"}, config)
	h.Close()
	fresh := NewHistory(store, "parity")
	t.Cleanup(fresh.Close)
	config, err = fresh.Snapshot(ModeConfig)
	require.NoError(t, err)
	assert.Equal(t, []string{"show two", "show three"}, config)
	operational, err := fresh.Snapshot(ModeOperational)
	require.NoError(t, err)
	assert.Equal(t, []string{"show operational"}, operational)
	other := NewHistory(store, "another-user")
	t.Cleanup(other.Close)
	entries, err := other.Snapshot(ModeConfig)
	require.NoError(t, err)
	assert.Empty(t, entries)
	_, err = fresh.Snapshot(EditorMode(99))
	require.Error(t, err)
	_, err = fresh.Record(EditorMode(99), nil, "show must-not-land")
	require.Error(t, err)
	config, err = fresh.Snapshot(ModeConfig)
	require.NoError(t, err)
	assert.Equal(t, []string{"show two", "show three"}, config)
}

// TestHistoryRelativeSetReplaysItsOriginalTarget proves a nonsecret value
// survives reopening and executes at its original address from another context.
func TestHistoryRelativeSetReplaysItsOriginalTarget(t *testing.T) {
	ed, path := newLeafListSessionEditor(t, validBGPConfig)
	original := []string{"bgp", "group", "original"}
	other := []string{"bgp", "group", "elsewhere"}
	require.NoError(t, ed.SetValue(other, "description", "keep-other"))
	h := NewHistory(ed.store, "parity")
	t.Cleanup(h.Close)
	recorded, err := h.Record(ModeConfig, original, `set description "public ? label"`)
	require.NoError(t, err)
	require.Len(t, recorded, 1)
	h.Close()
	fresh := NewHistory(ed.store, "parity")
	t.Cleanup(fresh.Close)
	recalled, err := fresh.Snapshot(ModeConfig)
	require.NoError(t, err)
	require.Equal(t, recorded, recalled)
	_, err = ed.ExecuteConfigCommand(contract.ConfigCommandRequest{Line: recalled[0], Path: other})
	require.NoError(t, err)
	entry := walkPath(ed.Tree(), ed.schema, original)
	require.NotNil(t, entry)
	value, found := entry.Get("description")
	require.True(t, found)
	assert.Equal(t, "public ? label", value)
	value, found = walkPath(ed.Tree(), ed.schema, other).Get("description")
	require.True(t, found)
	assert.Equal(t, "keep-other", value)
	committed, err := ed.store.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(committed), "public ? label")
}

// TestHistoryLoadDistinguishesMissingAndFailure protects the strict source
// read independently of the UI's decision to keep diagnostic commands usable.
func TestHistoryLoadDistinguishesMissingAndFailure(t *testing.T) {
	store := storage.CreateMemory()
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	h := NewHistory(store, "parity")
	t.Cleanup(h.Close)
	entries, err := h.Load(ModeConfig.String())
	require.NoError(t, err)
	assert.Empty(t, entries)
	failure := errors.New("saved history unavailable")
	broken := NewHistory(&unavailableHistoryStore{Storage: store, failure: failure}, "parity")
	t.Cleanup(broken.Close)
	entries, err = broken.Load(ModeConfig.String())
	require.ErrorIs(t, err, failure)
	assert.Empty(t, entries)
}

// TestHistoryMasksSchemaMarkedValues proves masking follows the schema rather
// than merely recognizing familiar password argument names.
func TestHistoryMasksSchemaMarkedValues(t *testing.T) {
	const module = `module ze-history-probe-conf {
 yang-version 1.1;
 namespace "urn:ze:history-probe";
 prefix probe;
 import ze-extensions { prefix ze; }
 container history-probe {
  leaf material { type string; ze:sensitive; }
  leaf public { type string; }
 }
}`
	t.Cleanup(configyang.RegisterModuleForTest("ze-history-probe-conf.yang", module))
	for _, line := range []string{
		"set material private-material-value",
		"top set history-probe material private-material-value",
		"run set history-probe material private-material-value",
	} {
		t.Run(line, func(t *testing.T) {
			store := storage.CreateMemory()
			t.Cleanup(func() { require.NoError(t, store.Close()) })
			h := NewHistory(store, "parity")
			t.Cleanup(h.Close)
			entries, err := h.Record(ModeConfig, []string{"history-probe"}, line)
			require.NoError(t, err)
			assert.NotContains(t, strings.Join(entries, "\n"), "private-material-value")
			h.Close()
			stored, err := store.ReadFile("meta/history/parity/config")
			require.NoError(t, err)
			assert.NotContains(t, string(stored), "private-material-value")
			fresh := NewHistory(store, "parity")
			t.Cleanup(fresh.Close)
			entries, err = fresh.Snapshot(ModeConfig)
			require.NoError(t, err)
			assert.NotContains(t, strings.Join(entries, "\n"), "private-material-value")
		})
	}
}

func (s *unavailableHistoryStore) ReadFile(key string) ([]byte, error) {
	if strings.HasPrefix(key, "meta/history/parity/") {
		return nil, s.failure
	}
	return s.Storage.ReadFile(key)
}

func (s *unavailableHistoryStore) WriteFile(key string, data []byte, perm fs.FileMode) error {
	if strings.HasPrefix(key, "meta/history/parity/") {
		s.writes.Add(1)
	}
	return s.Storage.WriteFile(key, data, perm)
}

// TestHistoryUnavailableRemainsVisibleWithoutReplacingUnreadData keeps the
// command path usable while a failed history read disables publication.
func TestHistoryUnavailableRemainsVisibleWithoutReplacingUnreadData(t *testing.T) {
	m := newTestModel(t)
	t.Cleanup(func() { require.NoError(t, m.editor.Close()) })
	store := storage.CreateMemory()
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	const key = "meta/history/parity/config"
	const original = "show previously-saved"
	require.NoError(t, store.WriteFile(key, []byte(original), 0))
	rw := &unavailableHistoryStore{Storage: store, failure: errors.New("history read refused")}
	m.SetHistory(NewHistory(rw, "parity"))
	defer func() { require.NoError(t, m.Close()) }()
	assert.Contains(t, m.feedbackLine(), "history")
	assert.Contains(t, m.ViewportContent(), "fix:")
	called := false
	m.SetCommandExecutor(func(string) (CommandOutput, error) {
		called = true
		return CommandOutput{Text: "diagnostic answer"}, nil
	})
	m.SetInput("run show version")
	command := applyHistoryProgressMessage(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, command, "a history failure must not disable diagnostics")
	applyHistoryProgressMessage(t, m, command())
	assert.True(t, called)
	assert.Contains(t, m.feedbackLine(), "history")
	m.history.Append("unread data must not be replaced")
	m.history.Save(ModeConfig)
	require.NoError(t, m.Close())
	assert.Zero(t, rw.writes.Load())
	actual, err := store.ReadFile(key)
	require.NoError(t, err)
	assert.Equal(t, original, string(actual))
}

// TestModelHistoryMasksContextualAssignments exercises the real Enter path,
// including relative addresses and top/run, before reading persisted recall.
func TestModelHistoryMasksContextualAssignments(t *testing.T) {
	for _, line := range []string{
		"set password private-parity-secret",
		"top set bgp peer test connection md5 password private-parity-secret",
		"run update data key probe content private-parity-secret",
	} {
		t.Run(line, func(t *testing.T) {
			m := newTestModel(t)
			t.Cleanup(func() { require.NoError(t, m.editor.Close()) })
			store := storage.CreateMemory()
			t.Cleanup(func() { require.NoError(t, store.Close()) })
			m.SetHistory(NewHistory(store, "parity"))
			defer func() { require.NoError(t, m.Close()) }()
			m.contextPath = []string{"bgp", "peer", "test", "connection", "md5"}
			m.SetInput(line)
			applyHistoryProgressMessage(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
			require.NotEmpty(t, m.history.Entries())
			assert.NotContains(t, strings.Join(m.history.Entries(), "\n"), "private-parity-secret")
			require.NoError(t, m.Close())
			data, err := store.ReadFile("meta/history/parity/config")
			require.NoError(t, err)
			assert.NotContains(t, string(data), "private-parity-secret")
			assert.Contains(t, string(data), "<redacted>")
		})
	}
}

// TestModelHistoryMasksLegacyRecords protects recall even when an older
// session wrote raw credentials without retaining its relative editor path.
func TestModelHistoryMasksLegacyRecords(t *testing.T) {
	m := newTestModel(t)
	t.Cleanup(func() { require.NoError(t, m.editor.Close()) })
	store := storage.CreateMemory()
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	const key = "meta/history/parity/config"
	legacy := []string{
		"top set bgp peer test connection md5 password legacy-secret",
		"set password legacy-secret",
		"run update data key probe content legacy-secret",
		"show bgp",
	}
	require.NoError(t, store.WriteFile(key, []byte(strings.Join(legacy, "\n")), 0))
	m.SetHistory(NewHistory(store, "parity"))
	defer func() { require.NoError(t, m.Close()) }()
	assert.NotContains(t, strings.Join(m.history.Entries(), "\n"), "legacy-secret")
	assert.Contains(t, m.history.Entries(), "show bgp", "ordinary recall must remain useful")
	m.SetInput("show")
	applyHistoryProgressMessage(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NoError(t, m.Close())
	data, err := store.ReadFile(key)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "legacy-secret", "publishing the next snapshot must not republish old secrets")
}
