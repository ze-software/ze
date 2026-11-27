package loginservice

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoginServiceRegistryRejectsConflictingRegistration verifies that
// registering a login service name twice with a conflicting owner or
// description returns ErrConflict, while an identical repeat is idempotent,
// and Names/Registered/Lookup reflect the registered entries cleanly.
//
// VALIDATES: AC-1 -- conflicting re-registration returns ErrConflict; identical
// repeat is idempotent; Names returns sorted names; Registered and Lookup match.
// PREVENTS: Silent overwrite when two components claim the same login service name.
func TestLoginServiceRegistryRejectsConflictingRegistration(t *testing.T) {
	snap := Snapshot()
	t.Cleanup(func() { Restore(snap) })
	Restore(nil)

	sshEntry := Entry{
		Name:        "ssh",
		Owner:       "internal/component/ssh",
		Description: "SSH and ze cli login",
	}
	webEntry := Entry{
		Name:        "web",
		Owner:       "internal/component/web",
		Description: "Web UI login",
	}
	restEntry := Entry{
		Name:        "rest",
		Owner:       "internal/component/api/rest",
		Description: "REST API login",
	}

	require.NoError(t, Register(webEntry))
	require.NoError(t, Register(sshEntry))
	require.NoError(t, Register(restEntry))

	// Identical repeat registration is accepted idempotently.
	require.NoError(t, Register(sshEntry))

	// Conflicting owner is rejected with ErrConflict.
	err := Register(Entry{
		Name:        "ssh",
		Owner:       "internal/component/other",
		Description: "SSH and ze cli login",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrConflict)

	// Conflicting description for same owner is also rejected with ErrConflict.
	err = Register(Entry{
		Name:        "ssh",
		Owner:       "internal/component/ssh",
		Description: "different description",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrConflict)

	// Empty name or owner is rejected with ErrInvalidEntry.
	assert.ErrorIs(t, Register(Entry{Owner: "internal/component/ssh"}), ErrInvalidEntry)
	assert.ErrorIs(t, Register(Entry{Name: "grpc"}), ErrInvalidEntry)

	// Lookup and Registered reflect the surviving original registration.
	got, ok := Lookup("ssh")
	require.True(t, ok)
	assert.Equal(t, sshEntry, got)
	assert.True(t, Registered("ssh"))
	assert.True(t, Registered("web"))
	assert.True(t, Registered("rest"))
	assert.False(t, Registered("unknown"))

	_, ok = Lookup("unknown")
	assert.False(t, ok)

	// Names returns all registered names in sorted order.
	names := Names()
	assert.Equal(t, []string{"rest", "ssh", "web"}, names)
	assert.True(t, slices.IsSorted(names))

	// MustRegister panics with BUG: on conflict.
	assert.PanicsWithValue(t,
		`BUG: loginservice: loginservice: registration conflict: "ssh" already registered by "internal/component/ssh", got "internal/component/conflict"`,
		func() {
			MustRegister(Entry{
				Name:        "ssh",
				Owner:       "internal/component/conflict",
				Description: "conflict",
			})
		},
	)
}
