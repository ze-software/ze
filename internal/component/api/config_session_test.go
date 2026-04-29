package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeEditor implements ConfigEditor for testing.
type fakeEditor struct {
	values    map[string]string
	saved     bool
	discarded bool
}

func newFakeEditor() *fakeEditor {
	return &fakeEditor{values: make(map[string]string)}
}

func (e *fakeEditor) SetValue(path []string, key, value string) error {
	e.values[strings.Join(path, ".")+"."+key] = value
	return nil
}

func (e *fakeEditor) DeleteByPath(fullPath []string) error {
	delete(e.values, strings.Join(fullPath, "."))
	return nil
}

func (e *fakeEditor) Diff() string {
	if len(e.values) == 0 {
		return ""
	}
	var b strings.Builder
	for k, v := range e.values {
		fmt.Fprintf(&b, "+%s = %s\n", k, v)
	}
	return b.String()
}

func (e *fakeEditor) Save() error {
	e.saved = true
	return nil
}

func (e *fakeEditor) Discard() error {
	e.discarded = true
	e.values = make(map[string]string)
	return nil
}

func (e *fakeEditor) WorkingContent() string {
	return "# config\n"
}

func fakeEditorFactory() ConfigEditorFactory {
	return func() (ConfigEditor, error) {
		return newFakeEditor(), nil
	}
}

// VALIDATES: AC-5 -- ConfigEnter + Set + Commit lifecycle.
// PREVENTS: config session lifecycle broken.
func TestEngineConfigSession(t *testing.T) {
	mgr := NewConfigSessionManager(fakeEditorFactory())

	// Enter session.
	id, err := mgr.Enter("admin")
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// Set a value.
	err = mgr.Set("admin", id, "bgp.router-id", "10.0.0.1")
	require.NoError(t, err)

	// Diff shows changes.
	diff, err := mgr.Diff("admin", id)
	require.NoError(t, err)
	assert.NotEmpty(t, diff)

	// Commit applies changes.
	err = mgr.Commit("admin", id)
	require.NoError(t, err)

	// Session is gone after commit.
	_, err = mgr.Diff("admin", id)
	assert.Error(t, err)
}

// TestConfigSessionCommitHook verifies API commits apply the saved config to
// runtime before reporting success.
//
// VALIDATES: Commit calls the configured runtime apply hook after saving.
// PREVENTS: REST/gRPC config commit returning success for file-only writes.
func TestConfigSessionCommitHook(t *testing.T) {
	mgr := NewConfigSessionManager(fakeEditorFactory())
	called := false
	mgr.SetCommitHook(func() error {
		called = true
		return nil
	})

	id, err := mgr.Enter("admin")
	require.NoError(t, err)
	require.NoError(t, mgr.Set("admin", id, "bgp.router-id", "10.0.0.1"))

	require.NoError(t, mgr.Commit("admin", id))
	assert.True(t, called, "commit hook should be called")

	_, err = mgr.Diff("admin", id)
	assert.Error(t, err, "session should be removed after successful hook")
}

// TestConfigSessionCommitHookFailureKeepsSession verifies runtime apply errors
// are visible to the client and leave the session available for retry.
//
// VALIDATES: Commit returns hook errors and does not delete the session.
// PREVENTS: Failed runtime apply being hidden behind a successful commit response.
func TestConfigSessionCommitHookFailureKeepsSession(t *testing.T) {
	mgr := NewConfigSessionManager(fakeEditorFactory())
	mgr.SetCommitHook(func() error { return fmt.Errorf("reload failed") })

	id, err := mgr.Enter("admin")
	require.NoError(t, err)
	require.NoError(t, mgr.Set("admin", id, "bgp.router-id", "10.0.0.1"))

	err = mgr.Commit("admin", id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "runtime reload failed")
	assert.Contains(t, err.Error(), "reload failed")

	_, err = mgr.Diff("admin", id)
	assert.NoError(t, err, "session should remain after failed hook")
}

// VALIDATES: AC-5 -- ConfigDiscard throws away changes.
// PREVENTS: discard leaving stale session state.
func TestEngineConfigDiscard(t *testing.T) {
	mgr := NewConfigSessionManager(fakeEditorFactory())

	id, err := mgr.Enter("admin")
	require.NoError(t, err)

	err = mgr.Set("admin", id, "bgp.router-id", "10.0.0.1")
	require.NoError(t, err)

	err = mgr.Discard("admin", id)
	require.NoError(t, err)

	// Session is gone after discard.
	_, err = mgr.Diff("admin", id)
	assert.Error(t, err)
}

// VALIDATES: unknown session ID returns error.
// PREVENTS: operations on nonexistent sessions.
func TestConfigSessionNotFound(t *testing.T) {
	mgr := NewConfigSessionManager(fakeEditorFactory())

	err := mgr.Set("admin", "nonexistent", "bgp.router-id", "10.0.0.1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// VALIDATES: multiple concurrent sessions are independent.
// PREVENTS: session cross-contamination.
func TestConfigSessionIndependence(t *testing.T) {
	mgr := NewConfigSessionManager(fakeEditorFactory())

	id1, err := mgr.Enter("alice")
	require.NoError(t, err)

	id2, err := mgr.Enter("bob")
	require.NoError(t, err)

	assert.NotEqual(t, id1, id2)

	// Set different values in each (using correct owner).
	require.NoError(t, mgr.Set("alice", id1, "bgp.router-id", "1.1.1.1"))
	require.NoError(t, mgr.Set("bob", id2, "bgp.router-id", "2.2.2.2"))

	// Commit one, other still exists.
	require.NoError(t, mgr.Commit("alice", id1))

	diff, err := mgr.Diff("bob", id2)
	require.NoError(t, err)
	assert.NotEmpty(t, diff)
}

// VALIDATES: session owned by one user cannot be accessed by another.
// PREVENTS: session hijacking.
func TestConfigSessionOwnership(t *testing.T) {
	mgr := NewConfigSessionManager(fakeEditorFactory())

	id, err := mgr.Enter("alice")
	require.NoError(t, err)

	// Bob tries to hijack alice's session.
	err = mgr.Set("bob", id, "bgp.router-id", "9.9.9.9")
	assert.ErrorIs(t, err, ErrSessionForbidden)

	_, err = mgr.Diff("bob", id)
	assert.ErrorIs(t, err, ErrSessionForbidden)

	err = mgr.Commit("bob", id)
	assert.ErrorIs(t, err, ErrSessionForbidden)

	err = mgr.Discard("bob", id)
	assert.ErrorIs(t, err, ErrSessionForbidden)

	// Alice can still use her session.
	require.NoError(t, mgr.Set("alice", id, "bgp.router-id", "1.1.1.1"))
}

// VALIDATES: path too short returns error.
// PREVENTS: index out of bounds on short paths.
func TestConfigSessionSetShortPath(t *testing.T) {
	mgr := NewConfigSessionManager(fakeEditorFactory())

	id, err := mgr.Enter("admin")
	require.NoError(t, err)

	err = mgr.Set("admin", id, "single", "value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path too short")
}

// VALIDATES: factory error propagated from Enter.
// PREVENTS: silent failure on editor creation.
func TestConfigSessionFactoryError(t *testing.T) {
	failFactory := func() (ConfigEditor, error) {
		return nil, fmt.Errorf("cannot open config")
	}
	mgr := NewConfigSessionManager(failFactory)

	_, err := mgr.Enter("admin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot open config")
}

// VALIDATES: splitPath handles degenerate dot patterns.
// PREVENTS: empty segments from leading/trailing/consecutive dots.
func TestSplitPathEdgeCases(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"single", []string{"single"}},
		{"a.b", []string{"a", "b"}},
		{"a.b.c", []string{"a", "b", "c"}},
		{".leading", []string{"leading"}},
		{"trailing.", []string{"trailing"}},
		{"a..b", []string{"a", "b"}},
		{"..a..b..", []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitPath(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// VALIDATES: CleanExpired removes stale sessions.
// PREVENTS: session leak on transport disconnect.
func TestConfigSessionCleanExpired(t *testing.T) {
	mgr := NewConfigSessionManager(fakeEditorFactory())
	mgr.timeout = 0 // Expire immediately.

	id, err := mgr.Enter("admin")
	require.NoError(t, err)
	require.NoError(t, mgr.Set("admin", id, "bgp.router-id", "10.0.0.1"))

	cleaned := mgr.CleanExpired()
	assert.Equal(t, 1, cleaned)

	// Session is gone.
	_, err = mgr.Diff("admin", id)
	assert.Error(t, err)
}
