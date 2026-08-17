// Design: ai/patterns/registration.md -- AAA registry (VFS-like)
// Overview: aaa.go -- AAA interfaces

package aaa

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"github.com/ze-software/ze/internal/component/config"
)

var (
	errNoAuthenticationBackendsConfigured = errors.New("no authentication backends configured")
	errAllAuthenticationBackendsFailed    = errors.New("all authentication backends failed")
	errAaaBackendHasEmptyName             = errors.New("aaa backend has empty name")
	errNoAuthenticationBackendConfigured  = errors.New("no authentication backend configured")
)

// SSHPublicKey holds a single named SSH public key for key-based authentication.
type SSHPublicKey struct {
	Name string // key identifier (e.g. "jdoe@laptop")
	Type string // algorithm (e.g. "ssh-ed25519")
	Key  string // base64-encoded key data
}

// UserCredential is a configured local user credential. Owned by aaa so the
// local backend factory can consume it without introducing a cycle back to
// authz. The authz package re-exports this as UserConfig for callers.
type UserCredential struct {
	Name       string
	Hash       string
	Profiles   []string
	PublicKeys []SSHPublicKey
	// LocalGeneration identifies the accepted local credential generation that
	// produced this record. It is runtime-only and never loaded from config.
	LocalGeneration uint64
}

// BuildParams is the data passed to every backend factory at Build time.
type BuildParams struct {
	Ctx        context.Context
	ConfigTree *config.Tree
	Logger     *slog.Logger
	LocalUsers []UserCredential // consumed by the local backend

	// LocalUsersFunc returns the local credentials that are valid RIGHT NOW.
	// When set, the local backend prefers it over LocalUsers, so the chain
	// follows the running configuration instead of the one the bundle was built
	// from. It exists because the bundle is NOT rebuilt on a config reload:
	// without it, the chain's local backend keeps authenticating a user the
	// operator has deleted, and it does so BEFORE any later fallback is
	// consulted, which makes it the authoritative stale answer.
	LocalUsersFunc func() ([]UserCredential, error)

	// LocalAuthorizer is the hub-supplied adapter over *authz.Store.
	// Backends that need a local RBAC fallback (tacacs on server error)
	// consume it here. The local backend returns it as its Authorizer
	// contribution so the dispatcher always has an authorizer.
	LocalAuthorizer Authorizer
}

// Contribution is what a Backend contributes to the Bundle. A backend may
// contribute any non-empty subset. Close (optional) is invoked by
// Bundle.Close() for backends with lifecycle state.
type Contribution struct {
	Authenticator Authenticator
	Authorizer    Authorizer
	Accountant    Accountant
	Close         func() error
}

// Bundle is the composed AAA surface. Close MUST be called on shutdown so
// backends with background workers (TACACS+ accounting) drain cleanly.
// Close is idempotent: a second call is a no-op and returns the same error
// (or nil) that the first call returned.
type Bundle struct {
	Authenticator           Authenticator
	Authorizer              Authorizer
	Accountant              Accountant
	localAuthorizerSelected bool
	closers                 []func() error
	closeOnce               sync.Once
	closeErr                error // set by the first Close call, returned by all calls
}

// AuthorizerWithLocalFallback returns the selected external authorizer with
// fallback replaced by local. When the local backend itself was selected, local
// is the complete authorization policy for this request generation.
func (b *Bundle) AuthorizerWithLocalFallback(local Authorizer) Authorizer {
	if b == nil {
		return nil
	}
	if b.Authorizer == nil || b.localAuthorizerSelected {
		return local
	}
	if binder, ok := b.Authorizer.(localFallbackBindingAuthorizer); ok {
		return binder.BindLocalFallback(local)
	}
	return b.Authorizer
}

// backendRegistry holds registered backends. It freezes after the first Build
// call so late registrations cannot bypass the composed chain.
type backendRegistry struct {
	mu       sync.Mutex
	backends []Backend
	names    map[string]struct{}
	frozen   bool
}

// SourceLocal is the AuthResult.Source the built-in bcrypt backend reports, and
// the name that backend registers under. It is declared here, beside the field
// it fills, so a caller that must recognize the local backend compares against
// the registry's own spelling instead of repeating the string.
const SourceLocal = "local"

// AuthResult holds the outcome of an authentication attempt.
type AuthResult struct {
	Authenticated bool
	Profiles      []string // ze authz profile names for this user

	// Source names the backend that produced this result: SourceLocal,
	// "tacacs", "radius", and so on. It is the ANSWER to "who authenticated
	// this user", reported by the backend that did it. A caller that needs the
	// answer reads it here rather than asking a second question of its own,
	// which would be a different question taken at a different instant.
	Source string
	// LocalGeneration is copied from the matched local credential. It makes a
	// recovery authorizer expire after a newer identity generation is accepted,
	// including when authentication and publication overlap.
	LocalGeneration uint64
	// Authorizer is the authorization view for this authentication result.
	// Transports carry it with the session or request so another login that uses
	// the same username cannot change this result's authorization.
	Authorizer Authorizer
}

// GrantedByLocalBackend reports whether the local (configuration plus zefs)
// backend produced this result, and therefore whether the local user list is
// this result's revoker.
//
// An empty Source is NOT local. A backend that names itself is claiming the
// grant; silence is not that claim, and reading silence as "local" would attach
// the local list's revocation to a session some other backend granted.
func (r AuthResult) GrantedByLocalBackend() bool { return r.Source == SourceLocal }

// AuthRequest carries request-scoped authentication input and trusted metadata.
type AuthRequest struct {
	Username   string
	Password   string //nolint:gosec // Transient in-memory auth input passed to backends; never logged or persisted.
	RemoteAddr string
	Service    string

	// Local reports whether the connection originated from a trusted-local
	// transport: a unix-socket peer or a loopback TCP peer, classified from the
	// accepted socket address at the transport entry point (never from
	// client-supplied data such as headers). It gates the bcrypt-hash-as-token
	// credential path in the local authenticator: the ze CLI presents the
	// zefs-stored hash over loopback, which must be accepted, but the same hash
	// presented over any remote transport (non-loopback SSH, all web, all API)
	// must be rejected so a leaked config backup is not itself a credential.
	//
	// Fail-closed by construction: the zero value (false) means remote, so a
	// surface that never sets it (web, API) — or a future caller that forgets
	// to — gets hash-as-token rejected, never accepted.
	Local bool
}

// ChainAuthenticator tries backends in order and distinguishes two failure modes:
//   - Explicit rejection (ErrAuthRejected): stop immediately, do not try next.
//   - Connection error (any other error): try the next backend.
//
// First successful authentication wins.
type ChainAuthenticator struct {
	Backends []Authenticator
}

// Authenticate walks the chain in registration order.
func (c *ChainAuthenticator) Authenticate(request AuthRequest) (AuthResult, error) {
	if len(c.Backends) == 0 {
		return AuthResult{}, errNoAuthenticationBackendsConfigured
	}
	var lastErr error
	for _, backend := range c.Backends {
		result, err := backend.Authenticate(request)
		if err == nil && result.Authenticated {
			return result, nil
		}
		if errors.Is(err, ErrAuthRejected) {
			return result, ErrAuthRejected
		}
		lastErr = err
	}
	if lastErr != nil {
		return AuthResult{}, fmt.Errorf("all authentication backends failed: %w", lastErr)
	}
	return AuthResult{}, errAllAuthenticationBackendsFailed
}

// Close invokes every contributed Close function in registration order.
// Errors are collected and joined. Idempotent: a second call does not
// re-invoke the Close hooks and returns the same error (or nil) that the
// first call returned, so callers using Close() in a retry loop observe
// consistent behavior.
//
// Concurrency: the read of b.closeErr after closeOnce.Do() returns is safe
// because sync.Once.Do establishes happens-before between the function
// body's writes and Do()'s return on every caller. Concurrent Close calls
// each see the same b.closeErr.
func (b *Bundle) Close() error {
	b.closeOnce.Do(func() {
		var errs []error
		for _, c := range b.closers {
			if err := c(); err != nil {
				errs = append(errs, err)
			}
		}
		// Drop the slice so a second call (if closeOnce were somehow
		// bypassed) would not re-run anything.
		b.closers = nil
		if len(errs) > 0 {
			b.closeErr = fmt.Errorf("aaa bundle close: %v", errs)
		}
	})
	return b.closeErr
}

// NewBackendRegistryForTest constructs an empty isolated registry for tests.
// Production code uses the package-level Default registry.
func NewBackendRegistryForTest() *backendRegistry {
	return &backendRegistry{names: make(map[string]struct{})}
}

// Default is the registry used by init()-time self-registrations. Backends
// call aaa.Default.Register(theirBackend).
//
// Tests MUST NOT call Default.Build: once Build runs the registry freezes
// and any subsequent Register (including from other test binaries' init
// paths if they happen to run in the same process) is rejected. Tests that
// need a composed Bundle should construct their own registry via
// NewBackendRegistryForTest and register their fakes onto it.
var Default = &backendRegistry{names: make(map[string]struct{})}

// Register adds a backend to this registry. Duplicate names are rejected.
// Registration after Build is rejected (frozen).
func (r *backendRegistry) Register(b Backend) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return fmt.Errorf("aaa registry is frozen (Build already called); cannot register %q", b.Name())
	}
	name := b.Name()
	if name == "" {
		return errAaaBackendHasEmptyName
	}
	if _, dup := r.names[name]; dup {
		return fmt.Errorf("aaa backend %q already registered", name)
	}
	r.names[name] = struct{}{}
	r.backends = append(r.backends, b)
	return nil
}

// orderedBackends returns backends sorted by Priority (ascending, stable).
func (r *backendRegistry) orderedBackends() []Backend {
	r.mu.Lock()
	defer r.mu.Unlock()
	sorted := make([]Backend, len(r.backends))
	copy(sorted, r.backends)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority() < sorted[j].Priority()
	})
	return sorted
}

// Build composes the Bundle from all registered backends. After Build, the
// registry is frozen and further Register calls are rejected.
func (r *backendRegistry) Build(params BuildParams) (*Bundle, error) {
	backends := r.orderedBackends()

	// Freeze AFTER snapshotting so a late Register during a Build race loses.
	r.mu.Lock()
	r.frozen = true
	r.mu.Unlock()

	bundle := &Bundle{}
	var authChain []Authenticator
	var authorizerOwner, accountantOwner string
	for _, b := range backends {
		contrib, err := b.Build(params)
		if err != nil {
			return nil, fmt.Errorf("aaa backend %q: %w", b.Name(), err)
		}
		if contrib.Authenticator != nil {
			authChain = append(authChain, contrib.Authenticator)
		}
		if contrib.Authorizer != nil {
			if bundle.Authorizer == nil {
				bundle.Authorizer = contrib.Authorizer
				bundle.localAuthorizerSelected = b.Name() == SourceLocal
				authorizerOwner = b.Name()
			} else if params.Logger != nil {
				params.Logger.Info("aaa: dropping duplicate authorizer",
					"from", b.Name(), "kept", authorizerOwner)
			}
		}
		if contrib.Accountant != nil {
			if bundle.Accountant == nil {
				bundle.Accountant = contrib.Accountant
				accountantOwner = b.Name()
			} else if params.Logger != nil {
				params.Logger.Info("aaa: dropping duplicate accountant",
					"from", b.Name(), "kept", accountantOwner)
			}
		}
		if contrib.Close != nil {
			bundle.closers = append(bundle.closers, contrib.Close)
		}
	}
	if len(authChain) == 0 {
		return nil, errNoAuthenticationBackendConfigured
	}
	var authenticator Authenticator
	if len(authChain) == 1 {
		authenticator = authChain[0]
	} else {
		authenticator = &ChainAuthenticator{Backends: authChain}
	}
	// Bind the selected authorizer to each successful authentication result.
	// The transport carries that immutable view through command dispatch.
	bundle.Authenticator = WithProfileAuthorizer(authenticator, bundle.Authorizer)
	return bundle, nil
}
