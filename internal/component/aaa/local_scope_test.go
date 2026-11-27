// Design: docs/architecture/aaa-tacacs.md -- local device-scope projection contract
// Related: command_args.go -- LocalScopeAuthorizer, AuthorizeLocalScope, FleetDeviceReadTokens
// Related: login_profiles.go -- acceptedGenerationAuthorizer.AuthorizeLocalScope

package aaa

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type localScopeStub struct {
	allow       bool
	calls       int
	legacyCalls int
	lastUser    string
	lastCmd     string
	lastArgs    []string
	lastRO      bool
}

func (s *localScopeStub) Authorize(string, string, string, bool) bool {
	s.legacyCalls++
	return true
}

func (s *localScopeStub) AuthorizeLocalScope(username, command string, args []string, isReadOnly bool) bool {
	s.calls++
	s.lastUser = username
	s.lastCmd = command
	s.lastArgs = append([]string(nil), args...)
	s.lastRO = isReadOnly
	return s.allow
}

func (s *localScopeStub) BindProfiles(_ []string) Authorizer {
	return s
}

type legacyOnlyAuthorizer struct {
	calls int
}

func (l *legacyOnlyAuthorizer) Authorize(string, string, string, bool) bool {
	l.calls++
	return true
}

// VALIDATES: FleetDeviceReadTokens builds canonical show-fleet-device tokens.
// PREVENTS: device-scope projection drifting from the show fleet device name grammar.
func TestFleetDeviceReadTokens(t *testing.T) {
	cmd, args := FleetDeviceReadTokens("edge-01")
	assert.Equal(t, FleetDeviceReadCommand, cmd)
	assert.Equal(t, []string{FleetDeviceNameArg, "edge-01"}, args)
	assert.Equal(t, "show fleet device name edge-01", CanonicalCommand(cmd, args, ""))
}

// VALIDATES: AuthorizeLocalScope reports unsupported and never falls back to Authorize.
// PREVENTS: an unsupported authorizer silently triggering remote per-device authorization.
func TestAuthorizeLocalScopeUnsupportedAndNil(t *testing.T) {
	cmd, args := FleetDeviceReadTokens("edge-01")

	allowed, supported := AuthorizeLocalScope(nil, "alice", cmd, args, true)
	assert.False(t, allowed)
	assert.False(t, supported)

	legacy := &legacyOnlyAuthorizer{}
	allowed, supported = AuthorizeLocalScope(legacy, "alice", cmd, args, true)
	assert.False(t, allowed)
	assert.False(t, supported)
	assert.Equal(t, 0, legacy.calls, "unsupported authorizer must not have Authorize called")

	stub := &localScopeStub{allow: true}
	allowed, supported = AuthorizeLocalScope(stub, "alice", cmd, args, true)
	assert.True(t, allowed)
	assert.True(t, supported)
	assert.Equal(t, 1, stub.calls)
	assert.Equal(t, 0, stub.legacyCalls)
	assert.Equal(t, "alice", stub.lastUser)
	assert.Equal(t, cmd, stub.lastCmd)
	assert.Equal(t, args, stub.lastArgs)
	assert.True(t, stub.lastRO)
}

// VALIDATES: acceptedGenerationAuthorizer enforces user generation on local scope
// and fails closed when its underlying authorizer does not support LocalScopeAuthorizer.
// PREVENTS: a recovery grant retaining positive fleet scope after a generation change.
func TestAcceptedGenerationAuthorizerLocalScope(t *testing.T) {
	t.Cleanup(func() { SetAcceptedLocalUsers(nil) })
	const recoveryUser = "recovery-admin"
	const deviceName = "edge-01"
	cmd, args := FleetDeviceReadTokens(deviceName)

	SetAcceptedLocalUsers([]UserCredential{{Name: recoveryUser, UserGeneration: 7}})

	t.Run("supported delegate revoked on generation change", func(t *testing.T) {
		stub := &localScopeStub{allow: true}
		grant := AuthorizerForResult(stub, AuthResult{
			Authenticated:  true,
			Source:         SourceLocal,
			Profiles:       []string{ReservedRecoveryProfile},
			UserGeneration: 7,
		})
		localScope, ok := grant.(LocalScopeAuthorizer)
		require.True(t, ok)

		assert.True(t, localScope.AuthorizeLocalScope(recoveryUser, cmd, args, true))
		assert.False(t, localScope.AuthorizeLocalScope("other-user", cmd, args, true))
		assert.Equal(t, 1, stub.calls)

		SetAcceptedLocalUsers([]UserCredential{{Name: recoveryUser, UserGeneration: 8}})
		assert.False(t, localScope.AuthorizeLocalScope(recoveryUser, cmd, args, true))
		assert.Equal(t, 1, stub.calls, "expired generation must deny before calling delegate")
	})

	t.Run("unsupported delegate fails closed even while generation holds", func(t *testing.T) {
		SetAcceptedLocalUsers([]UserCredential{{Name: recoveryUser, UserGeneration: 7}})
		legacy := &legacyOnlyAuthorizer{}
		grant := AuthorizerForResult(legacy, AuthResult{
			Authenticated:  true,
			Source:         SourceLocal,
			Profiles:       []string{ReservedRecoveryProfile},
			UserGeneration: 7,
		})
		allowed, supported := AuthorizeLocalScope(grant, recoveryUser, cmd, args, true)
		assert.True(t, supported)
		assert.False(t, allowed)
		assert.Equal(t, 0, legacy.calls)
	})

	t.Run("nil delegate allows only while generation holds", func(t *testing.T) {
		SetAcceptedLocalUsers([]UserCredential{{Name: recoveryUser, UserGeneration: 7}})
		grant := AuthorizerForResult(nil, AuthResult{
			Authenticated:  true,
			Source:         SourceLocal,
			Profiles:       []string{ReservedRecoveryProfile},
			UserGeneration: 7,
		})
		allowed, supported := AuthorizeLocalScope(grant, recoveryUser, cmd, args, true)
		assert.True(t, supported)
		assert.True(t, allowed)

		SetAcceptedLocalUsers(nil)
		allowed, supported = AuthorizeLocalScope(grant, recoveryUser, cmd, args, true)
		assert.True(t, supported)
		assert.False(t, allowed)
	})
}

type explicitStub struct {
	allow       bool
	calls       int
	legacyCalls int
	lastUser    string
	lastAddr    string
	lastPath    string
	lastTokens  []string
	lastRO      bool
}

func (s *explicitStub) Authorize(string, string, string, bool) bool {
	s.legacyCalls++
	return true
}

func (s *explicitStub) AuthorizeExplicit(
	username, remoteAddr, canonicalPath string,
	tokens []string,
	isReadOnly bool,
) bool {
	s.calls++
	s.lastUser = username
	s.lastAddr = remoteAddr
	s.lastPath = canonicalPath
	s.lastTokens = append([]string(nil), tokens...)
	s.lastRO = isReadOnly
	return s.allow
}

func (s *explicitStub) BindProfiles(_ []string) Authorizer {
	return s
}

// VALIDATES: AuthorizeExplicit fails closed on nil authorizer, legacy
// authorizers without ExplicitAuthorizer, and retired or nil-delegate
// acceptedGenerationAuthorizer instances.
// PREVENTS: explicit default-deny commands falling back to permissive
// Authorizer semantics or allowing via a break-glass recovery grant with no
// underlying explicit authorizer.
func TestAuthorizeExplicitNilAndUnsupportedFailClosed(t *testing.T) {
	t.Cleanup(func() { SetAcceptedLocalUsers(nil) })

	const (
		canonicalPath = "show pki certificate name bundle pem"
		remoteAddr    = "198.51.100.10:22"
		recoveryUser  = "recovery-admin"
	)
	tokens := []string{"show", "pki", "certificate", "name", "dev-1", "bundle", "pem", "confirm"}

	t.Run("nil and legacy authorizers fail closed without calling Authorize", func(t *testing.T) {
		allowed, supported := AuthorizeExplicit(nil, "alice", remoteAddr, canonicalPath, tokens, false)
		assert.False(t, allowed)
		assert.False(t, supported)

		legacy := &legacyOnlyAuthorizer{}
		allowed, supported = AuthorizeExplicit(legacy, "alice", remoteAddr, canonicalPath, tokens, false)
		assert.False(t, allowed)
		assert.False(t, supported)
		assert.Equal(t, 0, legacy.calls, "unsupported authorizer must not have Authorize called")
	})

	t.Run("supported authorizer receives exact explicit parameters", func(t *testing.T) {
		stub := &explicitStub{allow: true}
		allowed, supported := AuthorizeExplicit(stub, "exporter", remoteAddr, canonicalPath, tokens, false)
		assert.True(t, allowed)
		assert.True(t, supported)
		assert.Equal(t, 1, stub.calls)
		assert.Equal(t, 0, stub.legacyCalls)
		assert.Equal(t, "exporter", stub.lastUser)
		assert.Equal(t, remoteAddr, stub.lastAddr)
		assert.Equal(t, canonicalPath, stub.lastPath)
		assert.Equal(t, tokens, stub.lastTokens)
		assert.False(t, stub.lastRO)

		stub.allow = false
		allowed, supported = AuthorizeExplicit(stub, "exporter", remoteAddr, canonicalPath, tokens, false)
		assert.False(t, allowed)
		assert.True(t, supported)
		assert.Equal(t, 2, stub.calls)
	})

	t.Run("acceptedGenerationAuthorizer fails closed on nil delegate even while generation holds", func(t *testing.T) {
		SetAcceptedLocalUsers([]UserCredential{{Name: recoveryUser, UserGeneration: 7}})
		grant := AuthorizerForResult(nil, AuthResult{
			Authenticated:  true,
			Source:         SourceLocal,
			Profiles:       []string{ReservedRecoveryProfile},
			UserGeneration: 7,
		})
		allowed, supported := AuthorizeExplicit(grant, recoveryUser, remoteAddr, canonicalPath, tokens, false)
		assert.True(t, supported)
		assert.False(t, allowed, "explicit grant must fail closed when a.next == nil")
	})

	t.Run("acceptedGenerationAuthorizer fails closed on unsupported delegate", func(t *testing.T) {
		SetAcceptedLocalUsers([]UserCredential{{Name: recoveryUser, UserGeneration: 7}})
		legacy := &legacyOnlyAuthorizer{}
		grant := AuthorizerForResult(legacy, AuthResult{
			Authenticated:  true,
			Source:         SourceLocal,
			Profiles:       []string{ReservedRecoveryProfile},
			UserGeneration: 7,
		})
		allowed, supported := AuthorizeExplicit(grant, recoveryUser, remoteAddr, canonicalPath, tokens, false)
		assert.True(t, supported)
		assert.False(t, allowed)
		assert.Equal(t, 0, legacy.calls)
	})

	t.Run("acceptedGenerationAuthorizer revokes on generation change or wrong user", func(t *testing.T) {
		SetAcceptedLocalUsers([]UserCredential{{Name: recoveryUser, UserGeneration: 7}})
		stub := &explicitStub{allow: true}
		grant := AuthorizerForResult(stub, AuthResult{
			Authenticated:  true,
			Source:         SourceLocal,
			Profiles:       []string{ReservedRecoveryProfile},
			UserGeneration: 7,
		})

		allowed, supported := AuthorizeExplicit(grant, recoveryUser, remoteAddr, canonicalPath, tokens, false)
		assert.True(t, supported)
		assert.True(t, allowed)
		assert.Equal(t, 1, stub.calls)

		allowed, supported = AuthorizeExplicit(grant, "other-user", remoteAddr, canonicalPath, tokens, false)
		assert.True(t, supported)
		assert.False(t, allowed)
		assert.Equal(t, 1, stub.calls, "wrong user must deny before calling delegate")

		SetAcceptedLocalUsers([]UserCredential{{Name: recoveryUser, UserGeneration: 8}})
		allowed, supported = AuthorizeExplicit(grant, recoveryUser, remoteAddr, canonicalPath, tokens, false)
		assert.True(t, supported)
		assert.False(t, allowed)
		assert.Equal(t, 1, stub.calls, "retired generation must deny before calling delegate")
	})
}
