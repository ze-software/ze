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
	t.Cleanup(func() { ForgetLoginProfiles("noc") })

	inner := &fakeBackend{result: AuthResult{Authenticated: true, Source: "tacacs", Profiles: []string{"read-only"}}}
	auth := profileRecordingAuthenticator{next: inner}

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
	t.Cleanup(func() { ForgetLoginProfiles("intruder") })

	inner := &fakeBackend{
		result: AuthResult{Authenticated: false, Profiles: []string{"admin"}},
		err:    ErrAuthRejected,
	}
	auth := profileRecordingAuthenticator{next: inner}

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
	t.Cleanup(func() { ForgetLoginProfiles("noc") })

	RecordLoginProfiles("noc", []string{"read-only"})
	RecordLoginProfiles("noc", nil)

	got, ok := LoginProfiles("noc")
	assert.True(t, ok, "an empty record must not erase a real mapping")
	assert.Equal(t, []string{"read-only"}, got)
}

// VALIDATES: the recorded slice is independent of the caller's AuthResult.
// PREVENTS: a backend reusing its slice and mutating a live authorization input.
func TestRecordLoginProfilesCopies(t *testing.T) {
	t.Cleanup(func() { ForgetLoginProfiles("noc") })

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
	t.Cleanup(func() { ForgetLoginProfiles("built") })

	r := NewBackendRegistry()
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
