package aaa

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubBackend is a test Backend that records what it was asked to build.
type stubBackend struct {
	name     string
	priority int
	contrib  Contribution
	err      error
	built    bool
}

func (s *stubBackend) Name() string  { return s.name }
func (s *stubBackend) Priority() int { return s.priority }
func (s *stubBackend) Build(_ BuildParams) (Contribution, error) {
	s.built = true
	return s.contrib, s.err
}

// VALIDATES: AC-10 — duplicate registration is rejected.
// PREVENTS: two backends claiming the same name and silently overwriting.
func TestRegisterDuplicateName(t *testing.T) {
	r := NewBackendRegistry()
	require.NoError(t, r.Register(&stubBackend{name: "local"}))
	err := r.Register(&stubBackend{name: "local"})
	assert.Error(t, err, "duplicate name must be rejected")
	assert.Contains(t, err.Error(), "local")
}

// VALIDATES: empty backend name is rejected at Register.
// PREVENTS: anonymous backends.
func TestRegisterEmptyName(t *testing.T) {
	r := NewBackendRegistry()
	err := r.Register(&stubBackend{name: ""})
	assert.Error(t, err)
}

// VALIDATES: Registry freezes after Build and refuses later Register calls.
// PREVENTS: late registration bypassing the composed chain.
func TestRegisterFrozenAfterBuild(t *testing.T) {
	r := NewBackendRegistry()
	require.NoError(t, r.Register(&stubBackend{name: "first", contrib: Contribution{Authenticator: &fakeBackend{}}}))

	_, err := r.Build(BuildParams{})
	require.NoError(t, err)

	err = r.Register(&stubBackend{name: "late"})
	assert.Error(t, err, "register after build must be rejected")
	assert.Contains(t, err.Error(), "frozen")
}

// VALIDATES: priority ordering determines chain order.
// PREVENTS: alphabetical ordering putting "local" before "tacacs".
func TestRegistryPriorityOrder(t *testing.T) {
	r := NewBackendRegistry()
	// Register in reverse order; priority must win.
	require.NoError(t, r.Register(&stubBackend{name: "local", priority: 200, contrib: Contribution{Authenticator: &fakeBackend{}}}))
	require.NoError(t, r.Register(&stubBackend{name: "tacacs", priority: 100, contrib: Contribution{Authenticator: &fakeBackend{}}}))

	ordered := r.orderedBackends()
	require.Len(t, ordered, 2)
	assert.Equal(t, "tacacs", ordered[0].Name(), "lower priority comes first")
	assert.Equal(t, "local", ordered[1].Name())
}
