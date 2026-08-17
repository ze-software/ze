package aaa

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// VALIDATES: a successful authentication publishes its resolved profiles, so a
//
//	later authorization call (which sees only a username) can find them.
//
// PREVENTS: regression to profiles being resolved at login, logged, and dropped --
//
//	which authorized every TACACS+ user as admin.
func TestProfileRecordingAuthenticatorRecordsOnSuccess(t *testing.T) {
	t.Cleanup(func() { ForgetLoginProfilesForTest("noc") })

	inner := &fakeBackend{result: AuthResult{Authenticated: true, Source: "tacacs", Profiles: []string{"read-only"}}}
	auth := WithProfileRecording(inner)

	result, err := auth.Authenticate(AuthRequest{Username: "noc", Password: "pw"})
	assert.NoError(t, err)
	assert.True(t, result.Authenticated)

	got, ok := LoginProfiles("noc")
	assert.True(t, ok, "a successful authentication must publish its profiles")
	assert.Equal(t, []string{"read-only"}, got)
}

// VALIDATES: a failed authentication publishes nothing.
// PREVENTS: a rejected login granting profiles to that username.
func TestProfileRecordingAuthenticatorIgnoresFailure(t *testing.T) {
	t.Cleanup(func() { ForgetLoginProfilesForTest("intruder") })

	inner := &fakeBackend{
		result: AuthResult{Authenticated: false, Profiles: []string{"admin"}},
		err:    ErrAuthRejected,
	}
	auth := WithProfileRecording(inner)

	_, err := auth.Authenticate(AuthRequest{Username: "intruder", Password: "wrong"})
	assert.Error(t, err)

	_, ok := LoginProfiles("intruder")
	assert.False(t, ok, "a rejected authentication must not publish profiles")
}

// VALIDATES: an authentication that resolves no profiles leaves an earlier
//
//	mapping intact rather than erasing it.
//
// PREVENTS: a profile-less login from a second backend silently widening a user
//
//	to the admin fallthrough in authz.Store.Authorize.
func TestRecordLoginProfilesEmptyDoesNotErase(t *testing.T) {
	t.Cleanup(func() { ForgetLoginProfilesForTest("noc") })

	RecordLoginProfiles("noc", []string{"read-only"})
	RecordLoginProfiles("noc", nil)

	got, ok := LoginProfiles("noc")
	assert.True(t, ok, "an empty record must not erase a real mapping")
	assert.Equal(t, []string{"read-only"}, got)
}

// VALIDATES: the recorded slice is independent of the caller's AuthResult.
// PREVENTS: a backend reusing its slice and mutating a live authorization input.
func TestRecordLoginProfilesCopies(t *testing.T) {
	t.Cleanup(func() { ForgetLoginProfilesForTest("noc") })

	profiles := []string{"read-only"}
	RecordLoginProfiles("noc", profiles)
	profiles[0] = "admin"

	got, ok := LoginProfiles("noc")
	assert.True(t, ok)
	assert.Equal(t, []string{"read-only"}, got, "recorded profiles must not alias the caller's slice")
}

// VALIDATES: Build wraps the composed authenticator, so every surface records.
// PREVENTS: a transport authenticating without publishing profiles because the
//
//	recording lived at one call site instead of the composition point.
func TestBuildWrapsAuthenticatorWithProfileRecording(t *testing.T) {
	t.Cleanup(func() { ForgetLoginProfilesForTest("built") })

	r := NewBackendRegistryForTest()
	err := r.Register(&fakeAuthBackend{
		name:   "probe",
		result: AuthResult{Authenticated: true, Source: "probe", Profiles: []string{"read-only"}},
	})
	assert.NoError(t, err)

	bundle, err := r.Build(BuildParams{})
	assert.NoError(t, err)

	_, err = bundle.Authenticator.Authenticate(AuthRequest{Username: "built", Password: "pw"})
	assert.NoError(t, err)

	got, ok := LoginProfiles("built")
	assert.True(t, ok, "Build must wrap the chain so authentication publishes profiles")
	assert.Equal(t, []string{"read-only"}, got)
}

// TestProfileRecordingAuthenticatorRejectsReservedUsername pins the fail-closed
// ingress guard (spec-fixit-authz-admin-fallthrough review finding 2): an
// externally-supplied username bearing the reserved prefix must be rejected at the
// authentication choke point, before any backend sees it, so it can never become
// Authenticated and reach authz.Store.Authorize, which trusts server-injected
// internal and shared-API identities. Server-injected identities never pass
// through authentication and are unaffected.
//
// VALIDATES: reserved-name usernames are rejected at AAA ingress.
// PREVENTS: a RADIUS/TACACS+ server (or any surface) letting a client spoof a
//
//	reserved internal, shared-API, or recovery name via the username.
func TestProfileRecordingAuthenticatorRejectsReservedUsername(t *testing.T) {
	inner := &fakeBackend{result: AuthResult{Authenticated: true, Source: "fake", Profiles: []string{"admin"}}}
	auth := WithProfileRecording(inner)

	reserved := []string{
		ReservedInternalPrefix + "rpc",
		ReservedInternalPrefix + "plugin:evil",
		ReservedRecoveryProfile,
		ReservedSharedAPIUsername,
	}
	for _, name := range reserved {
		res, err := auth.Authenticate(AuthRequest{Username: name, Password: "x"})
		assert.ErrorIs(t, err, ErrAuthRejected, "reserved username %q must be rejected", name)
		assert.False(t, res.Authenticated, "reserved username %q must not authenticate", name)
		if _, ok := LoginProfiles(name); ok {
			t.Errorf("reserved username %q must not record login profiles", name)
			ForgetLoginProfilesForTest(name)
		}
	}
	assert.False(t, inner.called, "backend must not be consulted for a reserved username")

	// Sanity: a normal username still authenticates through the wrapper.
	t.Cleanup(func() { ForgetLoginProfilesForTest("alice") })
	res, err := auth.Authenticate(AuthRequest{Username: "alice", Password: "x"})
	assert.NoError(t, err)
	assert.True(t, res.Authenticated)
}

// fakeAuthBackend is a Backend contributing only an Authenticator.
type fakeAuthBackend struct {
	name   string
	result AuthResult
}

func (f *fakeAuthBackend) Name() string  { return f.name }
func (f *fakeAuthBackend) Priority() int { return 0 }
func (f *fakeAuthBackend) Build(BuildParams) (Contribution, error) {
	return Contribution{Authenticator: &fakeBackend{result: f.result}}, nil
}
