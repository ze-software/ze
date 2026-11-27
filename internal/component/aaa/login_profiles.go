// Design: docs/architecture/core-design.md -- AAA login-resolved profiles
// Related: types.go -- AuthResult.Profiles, the value recorded here
// Related: ../authz/authz.go -- Store.Authorize consumes this as a fallback
//
// Why this exists: a backend decides which authz profiles a user holds at
// AUTHENTICATION time. The local backend reads them from
// system.authentication.user[*].profile, but TACACS+ derives them from the
// server's priv-lvl reply via the tacacs-profile mapping -- a value that exists
// nowhere in config keyed by username.
//
// Authorization runs later, on a different call, and receives only the username.
// Without somewhere to put the login-time answer, authz.Store could only look up
// its static config assignments, find nothing for a TACACS+ user, and fall
// through to the built-in admin profile: the priv-lvl mapping was logged at login
// and then ignored, so every TACACS+ user was authorized as admin.
//
// Only profile NAMES are recorded, never a resolved Profile. Authorization looks
// each name up in the live store, so a reload that changes what "read-only" means
// takes effect on the next command rather than being pinned at login.

package aaa

import (
	"slices"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/config/loginservice"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const (
	// aaaMetricsHookName is the key registry.InjectPluginMetrics stores this
	// component's metrics callback under.
	aaaMetricsHookName = "aaa-login-service"

	// unknownLoginServiceLabel is the Prometheus service label used when a
	// refused authentication request omitted AuthRequest.Service.
	unknownLoginServiceLabel = "unknown"
)

// loginServiceSetTable holds an immutable snapshot of accepted login-service
// sets, mapping each configured set name to its allowed login service names.
type loginServiceSetTable struct {
	sets map[string][]string
}

// aaaMetrics holds Prometheus counters published by internal/component/aaa.
type aaaMetrics struct {
	loginServiceRefusedTotal metrics.CounterVec
}

// aaaLogger is the lazy subsystem logger for AAA login-service gate decisions.
var aaaLogger = slogutil.LazyLogger("aaa")

// acceptedLoginSets holds the currently published accepted login-service sets
// generation. nil means no generation has been published or the generation was
// retired, so every restricted login fails closed.
var acceptedLoginSets atomic.Pointer[loginServiceSetTable]

// aaaMetricsPtr holds the active Prometheus metric set for AAA. It is nil until
// a metrics registry is installed by telemetry startup.
var aaaMetricsPtr atomic.Pointer[aaaMetrics]

// registerAAAMetrics registers the deferred metrics callback with the plugin
// registry so ze_aaa_login_service_refused_total is published once a Prometheus
// registry exists.
func registerAAAMetrics() {
	registry.InjectPluginMetrics(aaaMetricsHookName, bindAAAMetrics)
}

// bindAAAMetrics publishes the AAA login-service counters into reg.
func bindAAAMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	aaaMetricsPtr.Store(&aaaMetrics{
		loginServiceRefusedTotal: reg.CounterVec(
			"ze_aaa_login_service_refused_total",
			"Login attempts refused by system/authorization/login-service, by target login service.",
			[]string{"service"},
		),
	})
}

// countLoginServiceRefused increments ze_aaa_login_service_refused_total for
// service. No-op while no metrics registry is bound.
func countLoginServiceRefused(service string) {
	m := aaaMetricsPtr.Load()
	if m == nil {
		return
	}
	label := service
	if label == "" {
		label = unknownLoginServiceLabel
	}
	m.loginServiceRefusedTotal.With(label).Inc()
}

// acceptedLocalUserGenerations maps each accepted local user name to the
// per-user generations (UserCredential.UserGeneration) its records carry. A
// name can repeat in the accepted list, hence a slice. nil means no local
// identity is accepted, so every recovery grant is refused.
var acceptedLocalUserGenerations atomic.Pointer[map[string][]uint64]

// SetAcceptedLocalUsers publishes the per-user generations of the accepted
// local credentials, against which local recovery grants are valid. The hub
// calls it each time it accepts a local identity, and with nil when it retires
// one, which revokes every grant. Remote login-resolved profiles are not
// generation-bound.
func SetAcceptedLocalUsers(users []UserCredential) {
	if users == nil {
		acceptedLocalUserGenerations.Store(nil)
		return
	}
	generations := make(map[string][]uint64, len(users))
	for _, user := range users {
		generations[user.Name] = append(generations[user.Name], user.UserGeneration)
	}
	acceptedLocalUserGenerations.Store(&generations)
}

// acceptedLocalUserHolds reports whether the accepted local identity still
// declares username at generation, the per-user generation that granted a
// recovery session. It is the check the web session store makes for a
// locally anchored cookie (web.SessionStore.localUserSessionStatus): a change
// to another user leaves it true, and a change to this user's own hash,
// profiles or keys, or this user's removal, makes it false.
//
// Generation numbers are never shared between two user records, so a grant
// presented under another username matches nothing and is refused.
func acceptedLocalUserHolds(username string, generation uint64) bool {
	// refuse-by-default: guards acceptedGenerationAuthorizer.Authorize -- a
	// result no local record stamped carries generation 0 and grants nothing.
	if generation == 0 {
		return false
	}
	generations := acceptedLocalUserGenerations.Load()
	// refuse-by-default: guards acceptedGenerationAuthorizer.Authorize -- no
	// accepted identity (before boot publishes, after shutdown) grants nothing.
	if generations == nil {
		return false
	}
	return slices.Contains((*generations)[username], generation)
}

// acceptedGenerationAuthorizer is the break-glass recovery grant one local
// login holds. It is valid only while the accepted local identity declares the
// session's user at the per-user generation that authenticated it.
type acceptedGenerationAuthorizer struct {
	generation uint64
	next       Authorizer
}

func (a acceptedGenerationAuthorizer) Authorize(username, remoteAddr, command string, isReadOnly bool) bool {
	if !acceptedLocalUserHolds(username, a.generation) {
		return false
	}
	if a.next == nil {
		return true
	}
	return a.next.Authorize(username, remoteAddr, command, isReadOnly)
}

func (a acceptedGenerationAuthorizer) AuthorizeCommandArgs(
	username, remoteAddr, command string,
	args []string,
	peer string,
	isReadOnly bool,
) bool {
	if !acceptedLocalUserHolds(username, a.generation) {
		return false
	}
	if a.next == nil {
		return true
	}
	if typed, ok := a.next.(CommandArgsAuthorizer); ok {
		return typed.AuthorizeCommandArgs(username, remoteAddr, command, args, peer, isReadOnly)
	}
	return a.next.Authorize(username, remoteAddr, CanonicalCommand(command, args, peer), isReadOnly)
}

// AuthorizeLocalScope implements LocalScopeAuthorizer. No positive scope
// survives an authorization generation change of the authenticated local user,
// and an unsupported underlying authorizer fails closed.
func (a acceptedGenerationAuthorizer) AuthorizeLocalScope(
	username, command string,
	args []string,
	isReadOnly bool,
) bool {
	if !acceptedLocalUserHolds(username, a.generation) {
		return false
	}
	if a.next == nil {
		return true
	}
	allowed, supported := AuthorizeLocalScope(a.next, username, command, args, isReadOnly)
	if !supported {
		return false
	}
	return allowed
}

// AuthorizeExplicit implements ExplicitAuthorizer. Explicit-grant commands fail
// closed whenever the authenticated local user's generation has retired, when
// no underlying authorizer is installed (a.next == nil), or when a.next does
// not implement ExplicitAuthorizer.
func (a acceptedGenerationAuthorizer) AuthorizeExplicit(
	username, remoteAddr, canonicalPath string,
	tokens []string,
	isReadOnly bool,
) bool {
	if !acceptedLocalUserHolds(username, a.generation) {
		return false
	}
	if a.next == nil {
		return false
	}
	allowed, supported := AuthorizeExplicit(a.next, username, remoteAddr, canonicalPath, tokens, isReadOnly)
	if !supported {
		return false
	}
	return allowed
}

// resultAuthorizingAuthenticator rejects reserved identities and binds
// authorization to each successful authentication result.
type resultAuthorizingAuthenticator struct {
	next       Authenticator
	authorizer Authorizer
}

// WithProfileAuthorizer applies the common authentication choke point and
// binds authorizer to each successful result's resolved profiles.
func WithProfileAuthorizer(next Authenticator, authorizer Authorizer) Authenticator {
	return resultAuthorizingAuthenticator{next: next, authorizer: authorizer}
}

// AuthorizerForResult returns the authorization view for one successful
// authentication result. Local users retain live assignment behavior across
// reloads. Remote profiles are bound to the result. A recovery profile is valid
// only while its own user's accepted generation (result.UserGeneration) remains
// current: a change to any other local user leaves it valid.
func AuthorizerForResult(authorizer Authorizer, result AuthResult) Authorizer {
	if result.Source == SourceLocal {
		if !slices.Contains(result.Profiles, ReservedRecoveryProfile) {
			return authorizer
		}
		return acceptedGenerationAuthorizer{
			generation: result.UserGeneration,
			next:       BindProfiles(authorizer, []string{ReservedRecoveryProfile}),
		}
	}

	profiles := make([]string, 0, len(result.Profiles))
	for _, profile := range result.Profiles {
		if profile != ReservedRecoveryProfile {
			profiles = append(profiles, profile)
		}
	}
	return BindProfiles(authorizer, profiles)
}

func (p resultAuthorizingAuthenticator) Authenticate(request AuthRequest) (AuthResult, error) {
	// Fail closed: no externally-supplied username may bear the reserved prefix.
	// Such a username is a bug or an attempt to spoof a server-injected trusted
	// identity that authz.Store.Authorize permits, or a reserved recovery
	// profile. This wrapper is the one authentication choke point every surface
	// (ssh, web, api) passes through, so rejecting here stops any backend,
	// including a hostile remote TACACS+/RADIUS server, from ever making such a
	// username Authenticated. Server-injected identities never pass through
	// authentication, so they are unaffected.
	if IsReservedName(request.Username) {
		return AuthResult{}, ErrAuthRejected
	}
	result, err := p.next.Authenticate(request)
	if err == nil && result.Authenticated {
		if gateErr := CheckLoginService(request.Username, &result, request.Service); gateErr != nil {
			return AuthResult{}, gateErr
		}
		result.Authorizer = AuthorizerForResult(p.authorizer, result)
	}
	return result, err
}

// SetAcceptedLoginSets publishes an immutable defensive copy of the accepted
// login-service sets generation against which CheckLoginService and
// CheckLoginSet decide login admission. An empty non-nil map publishes an
// accepted generation with zero defined sets (distinct from nil, which retires
// the generation so every restricted login fails closed).
//
// Safe for concurrent use.
func SetAcceptedLoginSets(sets map[string][]string) {
	if sets == nil {
		acceptedLoginSets.Store(nil)
		return
	}
	copied := make(map[string][]string, len(sets))
	for name, services := range sets {
		if services == nil {
			copied[name] = []string{}
			continue
		}
		copied[name] = slices.Clone(services)
	}
	acceptedLoginSets.Store(&loginServiceSetTable{sets: copied})
}

// CheckLoginService decides whether an authenticated result for username may
// use the named login service. Returns nil to admit or ErrAuthRejected to
// refuse.
//
// Safe for concurrent use.
func CheckLoginService(username string, result *AuthResult, service string) error {
	if result == nil {
		return refuseLoginService(username, "", service, "missing authentication result")
	}
	if !result.Authenticated {
		return refuseLoginService(username, result.LoginSet, service, "unauthenticated result")
	}
	// OD-2 option (a): the surviving stored recovery user's local result is
	// exempt before reading any login set. Remote results are never exempt even
	// if they carry ReservedRecoveryProfile.
	if result.Source == SourceLocal {
		if slices.Contains(result.Profiles, ReservedRecoveryProfile) {
			return nil
		}
	}
	// OD-3 option (a): an account naming no login-service set is unrestricted.
	if result.LoginSet == "" {
		return nil
	}
	return CheckLoginSet(username, result.LoginSet, service)
}

// CheckLoginSet decides whether username holding loginSet may use the named
// login service against the currently published accepted login-service sets.
// Returns nil to admit or ErrAuthRejected to refuse.
//
// Safe for concurrent use.
func CheckLoginSet(username, loginSet, service string) error {
	// OD-3 option (a): an absent login-service set imposes no door restriction.
	if loginSet == "" {
		return nil
	}
	if service == "" {
		return refuseLoginService(username, loginSet, service, "login service not specified")
	}
	if !loginservice.Registered(service) {
		return refuseLoginService(username, loginSet, service, "login service not registered")
	}
	table := acceptedLoginSets.Load()
	if table == nil {
		return refuseLoginService(username, loginSet, service, "accepted login-service sets unavailable")
	}
	allowed, ok := table.sets[loginSet]
	if !ok {
		return refuseLoginService(username, loginSet, service, "login-service set not found")
	}
	if !slices.Contains(allowed, service) {
		return refuseLoginService(username, loginSet, service, "service not in login-service set")
	}
	return nil
}

// refuseLoginService records one structured Info log entry and increments the
// refusal Prometheus counter before returning terminal ErrAuthRejected.
func refuseLoginService(username, loginSet, service, reason string) error {
	aaaLogger().Info("login refused by login-service gate",
		"account", username,
		"login_service_set", loginSet,
		"service", service,
		"reason", reason,
	)
	countLoginServiceRefused(service)
	return ErrAuthRejected
}
