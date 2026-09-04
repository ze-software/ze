// Design: ai/patterns/registration.md -- AAA registry (VFS-like)
// Related: infra_setup.go -- infraSetup installs the bundle on config load
// Related: main.go -- runYANGConfig defers closeAAABundle on exit

package hub

import (
	"log/slog"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/authz"
)

// aaaBundle holds the live AAA bundle. acceptedLocalIdentity holds every local
// credential and authorization decision from one accepted configuration
// generation, including the complete REST/gRPC authentication mode.
//
// Reload builds a replacement while every fallible operation runs. Before a
// listener can move, it publishes a fail-closed API staging view that contains
// no candidate material. Final acceptance replaces that view with the complete
// candidate in one atomic store. A request receives the authorizer captured by
// the same state that authenticated it.
type acceptedLocalIdentityState struct {
	generation            uint64
	users                 []aaa.UserCredential
	authorizer            *authz.Store
	resolveCandidateUsers func() ([]aaa.UserCredential, error)
	apiToken              string
	apiAuthentication     api.Authentication
}

var (
	aaaBundle               atomic.Pointer[aaa.Bundle]
	aaaBundleBootClaimed    atomic.Bool
	acceptedLocalIdentity   atomic.Pointer[acceptedLocalIdentityState]
	localIdentityGeneration atomic.Uint64
)

// claimAAABundleBoot reserves the daemon's single AAA construction attempt.
// A failed build still owns the boot slot so a later infrastructure hook cannot
// publish candidate backends before the surrounding reload is accepted.
func claimAAABundleBoot() bool {
	return aaaBundleBootClaimed.CompareAndSwap(false, true)
}

// swapAAABundle records that boot construction was attempted, installs its
// result, and closes the previously installed bundle (if any). A nil result
// still claims boot ownership so runtime hook reentry cannot retry the build.
//
// Boot is not the only caller. A config reload rebuilds the bundle from the
// reloaded tree and swaps it here at final acceptance (main_reload.go), because
// a RADIUS or TACACS+ client holds the address, the shared secret and the
// timeout it was CONSTRUCTED with and can re-read none of them. The close of
// the replaced bundle is what retires the previous client: both backends name a
// config reload as the swap their Close expects (internal/component/radius,
// internal/component/tacacs).
func swapAAABundle(b *aaa.Bundle, logger *slog.Logger) {
	closeRetiredAAABundle(installAAABundle(b), logger)
}

// installAAABundle records that boot construction was attempted and installs b
// in the atomic slot. It returns the bundle it retired, and the caller MUST pass
// that bundle to closeRetiredAAABundle: until it is closed, the retired chain
// still holds its RADIUS socket open and its TACACS+ accounting worker running.
func installAAABundle(b *aaa.Bundle) *aaa.Bundle {
	aaaBundleBootClaimed.Store(true)
	prev := aaaBundle.Swap(b)
	if prev == b {
		return nil
	}
	return prev
}

// closeRetiredAAABundle closes the bundle installAAABundle retired. It MUST be
// called after installAAABundle, and MUST NOT be called while aaaAcceptance is
// held: Close drains the TACACS+ accounting worker and joins its goroutine, so
// holding the acceptance lock across it would queue every other reload behind a
// network-facing worker. A nil bundle is a no-op.
func closeRetiredAAABundle(prev *aaa.Bundle, logger *slog.Logger) {
	if prev == nil {
		return
	}
	if err := prev.Close(); err != nil && logger != nil {
		logger.Warn("aaa: previous bundle close error on swap", "error", err)
	}
}

// aaaAcceptance serializes the acceptance tail of a config reload: the accepted
// identity publication, the accounting provider registration and the bundle
// install. A caller MUST hold it across all three, and MUST NOT hold it across
// the retired bundle's Close (closeRetiredAAABundle states the same obligation).
//
// Two reload drivers reach that tail on different goroutines: handleSIGHUPReload
// (main_reload.go) and reloadAfterCommitContext (main.go), which any SSH or web
// session editor calls. The plugin server's transaction lock excludes them only
// across Server.reloadConfig (internal/component/plugin/server/reload.go), which
// releases while every acceptance step is still ahead of both.
var aaaAcceptance sync.Mutex

// aaaAcceptedConfigOrder is the read order of the last accepted reload's
// configuration, and aaaAcceptanceRetired records that shutdown has closed the
// slot for good. Both are guarded by aaaAcceptance.
var (
	aaaAcceptedConfigOrder uint64
	aaaAcceptanceRetired   bool
)

// aaaConfigReadOrder numbers the configurations reloads read, in read order.
var aaaConfigReadOrder atomic.Uint64

// nextAAAConfigReadOrder numbers the configuration one reload has just read.
//
// Read order IS recency order: every reload reads the same store, so a tree read
// later can never describe an older configuration than a tree read earlier. That
// is what the acceptance order alone cannot say. The transaction lock releases
// before the tail, so a reload that read FIRST can still reach the tail LAST,
// and a lock over the tail would then let it install a chain built from the
// configuration the operator has already replaced.
func nextAAAConfigReadOrder() uint64 {
	return aaaConfigReadOrder.Add(1)
}

// acceptReloadedAAA publishes one reload's accepted local identity and installs
// the AAA chain that reload built, as one indivisible acceptance. order is the
// number nextAAAConfigReadOrder gave that reload's configuration.
//
// It returns the bundle the install retired, which the caller MUST pass to
// closeRetiredAAABundle. When installed is false the caller MUST close its own
// candidate instead: a candidate that is not installed still owns an open RADIUS
// socket and a started TACACS+ accounting worker.
func acceptReloadedAAA(identity *acceptedLocalIdentityState, bundle *aaa.Bundle, order uint64) (retired *aaa.Bundle, installed bool) {
	aaaAcceptance.Lock()
	defer aaaAcceptance.Unlock()

	if aaaAcceptanceRetired {
		// closeAAABundle has run, so the daemon is shutting down. Installing
		// here would revive a chain nothing is left to close, and leave its
		// TACACS+ accounting worker running past exit.
		return nil, false
	}
	if order <= aaaAcceptedConfigOrder {
		// A reload that read its configuration LATER has already been accepted,
		// so this one describes a superseded configuration. Neither its identity
		// nor its chain may replace what that reload published.
		return nil, false
	}
	aaaAcceptedConfigOrder = order

	if identity != nil {
		publishAcceptedLocalIdentity(identity)
	}
	if bundle == nil {
		return nil, false
	}
	registerAAAAccountingProvider(bundle)
	return installAAABundle(bundle), true
}

// publishLocalIdentity serializes the read-decide-store sequence
// publishAcceptedLocalIdentity runs.
//
// Two reload drivers reach that sequence on different goroutines:
// handleSIGHUPReload (main_reload.go) and the web or SSH commit path through
// reloadAfterCommitContext (main.go). The plugin server's transaction lock
// excludes them only across Server.reloadConfig (plugin/server/reload.go),
// which releases while the reload still has every identity step in front of
// it, so nothing else stops one driver from deciding a generation against a
// state the other is about to replace.
var publishLocalIdentity sync.Mutex

// publishAcceptedLocalIdentity installs one complete accepted identity
// generation. Callers must finish every fallible reload step before publishing
// a candidate.
//
// The generation is decided HERE and not by the caller. Deciding it means
// comparing the candidate against the ACCEPTED credentials, and only an answer
// read at the instant of the store is still true when the store happens. A
// decision taken where the candidate is built let the other driver publish in
// between, and a reused number then LOWERED the accepted generation, reviving
// every session that publication had revoked.
func publishAcceptedLocalIdentity(state *acceptedLocalIdentityState) {
	publishLocalIdentity.Lock()
	defer publishLocalIdentity.Unlock()

	stampLocalIdentityGeneration(state)
	aaa.SetAcceptedLocalProfileGeneration(state.generation)
	acceptedLocalIdentity.Store(state)
}

// stampLocalIdentityGeneration decides the generation this identity publishes
// and writes it into the state and into every credential the state carries.
// The caller MUST hold publishLocalIdentity.
//
// The generation exists for ONE consumer: aaa.acceptedGenerationAuthorizer, the
// break-glass grant a `ze init` bootstrap admin's live session holds
// (internal/component/aaa/login_profiles.go, AuthorizerForResult). That grant is
// valid only while its generation is the accepted one, so advancing the counter
// revokes every session already holding it, silently and mid-request.
//
// So the counter tracks the LOCAL CREDENTIAL SET, not the configuration. A
// candidate whose credentials equal the accepted ones keeps their number.
// Advancing on every reload made an operator's own config commit revoke the
// session that issued it: the commit reloaded, the generation moved, and the
// same request rendered its commit bar read-only and answered every later edit
// with 403.
//
// A credential that differs in any field still advances the counter, which is
// the revocation this grant was written for (a changed password hash, a removed
// admin, a demoted profile list, a revoked public key).
func stampLocalIdentityGeneration(state *acceptedLocalIdentityState) {
	state.generation = localIdentityGenerationFor(state.users)
	for i := range state.users {
		state.users[i].LocalGeneration = state.generation
	}
	state.apiAuthentication = buildAPIAuthentication(
		state.users,
		state.apiToken,
		acceptedLocalGenerationAuthorizer{store: state.authorizer},
	)
}

// localIdentityGenerationFor returns the accepted generation when users carries
// the same credentials as the accepted state, and a fresh number otherwise. The
// caller MUST hold publishLocalIdentity.
func localIdentityGenerationFor(users []aaa.UserCredential) uint64 {
	if accepted := acceptedLocalIdentity.Load(); accepted != nil && sameLocalCredentials(accepted.users, users) {
		return accepted.generation
	}

	return localIdentityGeneration.Add(1)
}

// sameLocalCredentials reports whether two local credential sets carry the same
// authentication and authorization material.
//
// Users are paired by NAME, never by position. Both sides are assembled by
// infra.ExtractAuthUsers and mergeAuthUsers (main_servers.go), which sort and
// then concatenate, so the two orders agree today. Nothing states that as a
// contract, and a comparison that read position would stop revoking on the day
// one of them stopped sorting, with every test still green.
//
// A name that appears twice on either side reports false. The pairing is then
// ambiguous, and revoking is the answer that fails closed.
func sameLocalCredentials(accepted, candidate []aaa.UserCredential) bool {
	if len(accepted) != len(candidate) {
		return false
	}

	unmatched := make(map[string]aaa.UserCredential, len(accepted))
	for _, user := range accepted {
		unmatched[user.Name] = user
	}

	for _, user := range candidate {
		previous, found := unmatched[user.Name]
		if !found || !sameLocalCredential(previous, user) {
			return false
		}
		delete(unmatched, user.Name)
	}

	return len(unmatched) == 0
}

// sameLocalCredential compares the two records of one user. LocalGeneration is
// excluded: it is the stamp this comparison decides, so reading it here would
// compare a record against itself.
//
// Every other field of aaa.UserCredential is read. A field added to that struct
// and not added here would stop revoking a live session on a change nobody
// compared, so TestSameLocalCredentialReadsEveryField pins the field count and
// fails the moment the struct grows.
func sameLocalCredential(accepted, candidate aaa.UserCredential) bool {
	return accepted.Name == candidate.Name &&
		accepted.Hash == candidate.Hash &&
		slices.Equal(accepted.Profiles, candidate.Profiles) &&
		slices.Equal(accepted.PublicKeys, candidate.PublicKeys)
}

// newAcceptedLocalIdentity builds one candidate identity. The result carries no
// generation and no API authentication mode: publishAcceptedLocalIdentity
// decides and stamps both at the instant it installs the state. A candidate a
// later reload step rejects therefore consumes no generation. The caller MUST
// publish the result before any surface reads it.
func newAcceptedLocalIdentity(users []aaa.UserCredential, store *authz.Store, resolveCandidateUsers func() ([]aaa.UserCredential, error), apiToken string) *acceptedLocalIdentityState {
	return &acceptedLocalIdentityState{
		users:                 append([]aaa.UserCredential(nil), users...),
		authorizer:            store,
		resolveCandidateUsers: resolveCandidateUsers,
		apiToken:              apiToken,
	}
}

// liveAcceptedAPIAuthentication returns one immutable accepted API generation.
// A missing state and the explicit staging state both reject every request.
func liveAcceptedAPIAuthentication() api.Authentication {
	state := acceptedLocalIdentity.Load()
	if state == nil {
		return api.Authentication{Required: true}
	}
	return state.apiAuthentication
}

// stageAPIAuthentication hides every accepted and candidate API credential
// while listeners migrate. The returned restore is used only when the reload is
// rejected; successful reload publishes the complete candidate instead.
//
// The restore is a compare-and-swap, not a store: the other reload driver can
// publish a complete identity while this one is staged, and putting the older
// state back would lower the accepted generation and revive the sessions that
// publication revoked. When the staged state is no longer the accepted one,
// somebody newer owns it and this restore has nothing to put back.
func stageAPIAuthentication() func() {
	previous := acceptedLocalIdentity.Load()
	if previous == nil {
		return func() {}
	}
	staged := *previous
	staged.apiAuthentication = api.Authentication{Required: true}
	acceptedLocalIdentity.Store(&staged)
	return func() { acceptedLocalIdentity.CompareAndSwap(&staged, previous) }
}

// liveAcceptedLocalUsers returns credentials from the accepted generation.
// It never reads ConfigProvider, whose roots hold a reload candidate while the
// transaction is still fallible.
func liveAcceptedLocalUsers() ([]aaa.UserCredential, error) {
	state := acceptedLocalIdentity.Load()
	if state == nil {
		return nil, errNoAcceptedLocalIdentity
	}
	return state.users, nil
}

// closeAAABundle closes the installed bundle and clears the accepted local
// identity. Called via defer from runYANGConfig so backend workers drain and no
// credential or policy state survives daemon shutdown.
func closeAAABundle(logger *slog.Logger) {
	// Retire the slot under the acceptance lock, so a reload already inside its
	// tail either installs before this runs or is refused by it. Without that,
	// a swap landing after the close reinstalls a live chain the daemon will
	// never close again (acceptReloadedAAA).
	aaaAcceptance.Lock()
	prev := aaaBundle.Swap(nil)
	acceptedLocalIdentity.Store(nil)
	aaa.SetAcceptedLocalProfileGeneration(0)
	aaaBundleBootClaimed.Store(false)
	aaaAcceptanceRetired = true
	aaaAcceptance.Unlock()

	if prev != nil {
		if err := prev.Close(); err != nil && logger != nil {
			logger.Warn("aaa: bundle close error on shutdown", "error", err)
		}
	}
}

// liveLocalAuthorizer is the local backend's stable AAA contribution. External
// authorizers keep registry priority over it, while TACACS+ receives the same
// value as its fallback. A nil store in the accepted generation is the existing
// no-RBAC allow mode.
type liveLocalAuthorizer struct{}

func (liveLocalAuthorizer) Authorize(username, remoteAddr, command string, isReadOnly bool) bool {
	state := acceptedLocalIdentity.Load()
	var store *authz.Store
	if state != nil {
		store = state.authorizer
	}
	return (authz.StoreAuthorizer{Store: store}).
		Authorize(username, remoteAddr, command, isReadOnly)
}

func (liveLocalAuthorizer) AuthorizeCommandArgs(username, remoteAddr, command string, args []string, peer string, isReadOnly bool) bool {
	state := acceptedLocalIdentity.Load()
	var store *authz.Store
	if state != nil {
		store = state.authorizer
	}
	return (authz.StoreAuthorizer{Store: store}).
		AuthorizeCommandArgs(username, remoteAddr, command, args, peer, isReadOnly)
}

// BindProfiles returns an authorizer that keeps one authentication result's
// profile names while reading the accepted profile definitions on each command.
func (liveLocalAuthorizer) BindProfiles(profiles []string) aaa.Authorizer {
	return liveLocalProfileAuthorizer{profiles: append([]string(nil), profiles...)}
}

type liveLocalProfileAuthorizer struct {
	profiles []string
}

func (a liveLocalProfileAuthorizer) Authorize(username, _, command string, isReadOnly bool) bool {
	state := acceptedLocalIdentity.Load()
	var store *authz.Store
	if state != nil {
		store = state.authorizer
	}
	if store == nil {
		return true
	}
	return store.AuthorizeWithProfiles(username, a.profiles, command, isReadOnly) != authz.Deny
}

func (a liveLocalProfileAuthorizer) AuthorizeCommandArgs(
	username, _ string,
	command string,
	args []string,
	peer string,
	isReadOnly bool,
) bool {
	state := acceptedLocalIdentity.Load()
	var store *authz.Store
	if state != nil {
		store = state.authorizer
	}
	if store == nil {
		return true
	}
	return store.AuthorizeWithProfiles(
		username,
		a.profiles,
		aaa.CanonicalCommand(command, args, peer),
		isReadOnly,
	) != authz.Deny
}

// acceptedLocalGenerationAuthorizer binds API fallback authorization to the
// store from the authentication generation while keeping the selected external
// bundle authorizer live.
type acceptedLocalGenerationAuthorizer struct {
	store         *authz.Store
	profiles      []string
	profilesBound bool
}

func (a acceptedLocalGenerationAuthorizer) Authorize(username, remoteAddr, command string, isReadOnly bool) bool {
	authorizer := a.currentAuthorizer()
	if authorizer == nil {
		return false
	}
	return authorizer.Authorize(username, remoteAddr, command, isReadOnly)
}

func (a acceptedLocalGenerationAuthorizer) AuthorizeCommandArgs(username, remoteAddr, command string, args []string, peer string, isReadOnly bool) bool {
	authorizer := a.currentAuthorizer()
	if authorizer == nil {
		return false
	}
	if typed, ok := authorizer.(aaa.CommandArgsAuthorizer); ok {
		return typed.AuthorizeCommandArgs(username, remoteAddr, command, args, peer, isReadOnly)
	}
	return authorizer.Authorize(
		username,
		remoteAddr,
		aaa.CanonicalCommand(command, args, peer),
		isReadOnly,
	)
}

// BindProfiles keeps the successful result's profiles with its accepted store.
func (a acceptedLocalGenerationAuthorizer) BindProfiles(profiles []string) aaa.Authorizer {
	a.profiles = append([]string(nil), profiles...)
	a.profilesBound = true
	return a
}

func (a acceptedLocalGenerationAuthorizer) currentAuthorizer() aaa.Authorizer {
	bundle := aaaBundle.Load()
	if bundle == nil {
		return nil
	}
	var local aaa.Authorizer = authz.StoreAuthorizer{Store: a.store}
	if a.profilesBound {
		local = acceptedLocalGenerationProfileFallback{
			store:    a.store,
			profiles: a.profiles,
		}
	}
	return bundle.AuthorizerWithLocalFallback(local)
}

type acceptedLocalGenerationProfileFallback struct {
	store    *authz.Store
	profiles []string
}

func (a acceptedLocalGenerationProfileFallback) Authorize(username, _, command string, isReadOnly bool) bool {
	if a.store == nil {
		return true
	}
	return a.store.AuthorizeWithProfiles(username, a.profiles, command, isReadOnly) != authz.Deny
}

func (a acceptedLocalGenerationProfileFallback) AuthorizeCommandArgs(
	username, _ string,
	command string,
	args []string,
	peer string,
	isReadOnly bool,
) bool {
	if a.store == nil {
		return true
	}
	return a.store.AuthorizeWithProfiles(
		username,
		a.profiles,
		aaa.CanonicalCommand(command, args, peer),
		isReadOnly,
	) != authz.Deny
}

// liveAAABundleAuthenticator authenticates against the live AAA bundle's chain
// (RADIUS/TACACS backends plus the local backend) once infra setup installs it.
// It reads the atomic slot on every call, mirroring liveAAABundleAuthorizer, so
// a rebuilt chain takes effect without restarting the surface that holds it
// (AC-2: RADIUS/TACACS admins authenticate on web).
//
// The indirection is what makes a reload reach a surface wired at startup. A
// captured bundle.Authenticator cannot follow a swap: it IS the retired chain,
// so ssh kept authenticating against a shared secret the operator had rotated.
//
// fallback answers when the chain does not authenticate the user, and web wires
// one because a config-file web user can be absent from the chain's local
// backend. It answers from the CURRENT configuration, never from a startup
// snapshot: the chain not knowing a user and the operator having deleted that
// user look identical from here, and only a reader of the running config can
// tell them apart. A nil fallback (ssh) leaves the chain as the only answer.
type liveAAABundleAuthenticator struct {
	fallback aaa.Authenticator
}

func (a liveAAABundleAuthenticator) Authenticate(request aaa.AuthRequest) (aaa.AuthResult, error) {
	if bundle := aaaBundle.Load(); bundle != nil && bundle.Authenticator != nil {
		result, err := bundle.Authenticator.Authenticate(request)
		if err == nil && result.Authenticated {
			return result, nil
		}
		if a.fallback != nil {
			if fres, ferr := a.fallback.Authenticate(request); ferr == nil && fres.Authenticated {
				return fres, nil
			}
		}
		return result, err
	}
	if a.fallback != nil {
		return a.fallback.Authenticate(request)
	}
	return aaa.AuthResult{}, aaa.ErrAuthRejected
}

type liveAAABundleAuthorizer struct{}

func (liveAAABundleAuthorizer) Authorize(username, remoteAddr, command string, isReadOnly bool) bool {
	bundle := aaaBundle.Load()
	if bundle == nil {
		// An AAA chain the daemon could not BUILD falls back to the local RBAC
		// policy, which is the documented behavior (owner ruling, 2026-09-04).
		//
		// This branch used to deny outright, and the denial defeated the
		// failover it sat beside: ssh hands a local account a session, the
		// login resolves its profiles, and then every command is refused. The
		// operator reached a shell that could not edit the config that broke
		// the chain. It reached further than that, because the same dispatch
		// path carries a plugin's own `request shutdown`.
		//
		// The policy consulted here is the accepted local one, which follows
		// the running config. Where the operator declared no
		// system.authorization profile it allows, exactly as an installed
		// bundle with no authorizer does, so this branch widens nothing a
		// built chain would have narrowed.
		return liveLocalAuthorizer{}.Authorize(username, remoteAddr, command, isReadOnly)
	}
	if bundle.Authorizer == nil {
		// No local RBAC configured (no system.authorization profiles).
		return true
	}
	return bundle.Authorizer.Authorize(username, remoteAddr, command, isReadOnly)
}

func (liveAAABundleAuthorizer) AuthorizeCommandArgs(
	username, remoteAddr, command string,
	args []string,
	peer string,
	isReadOnly bool,
) bool {
	bundle := aaaBundle.Load()
	if bundle == nil {
		// The same fallback as Authorize, and for the same reason. The two
		// methods MUST answer alike: a caller that reached one and was refused
		// by the other would see a command allowed by name and denied by its
		// arguments, with no policy behind either answer.
		return liveLocalAuthorizer{}.AuthorizeCommandArgs(username, remoteAddr, command, args, peer, isReadOnly)
	}
	if bundle.Authorizer == nil {
		return true
	}
	if typed, ok := bundle.Authorizer.(aaa.CommandArgsAuthorizer); ok {
		return typed.AuthorizeCommandArgs(username, remoteAddr, command, args, peer, isReadOnly)
	}
	return bundle.Authorizer.Authorize(
		username,
		remoteAddr,
		aaa.CanonicalCommand(command, args, peer),
		isReadOnly,
	)
}

// BindProfiles keeps one authentication result's profile names with the live
// bundle authorizer. Without it, aaa.BindProfiles returns this value unchanged
// and a bound session is authorized by USERNAME instead, which is the lookup
// the binding exists to replace: a TACACS+ priv-lvl answer and a `ze init`
// break-glass recovery grant are both keyed by profile and by nothing else
// (internal/component/aaa/login_profiles.go, AuthorizerForResult).
func (liveAAABundleAuthorizer) BindProfiles(profiles []string) aaa.Authorizer {
	return liveAAABundleProfileAuthorizer{profiles: append([]string(nil), profiles...)}
}

// liveAAABundleProfileAuthorizer is one session's view of the live bundle
// authorizer. It re-reads the atomic slot on every command and re-binds the
// profiles to whatever authorizer the slot now holds.
//
// Which sessions reach it is narrow, and worth stating so nobody reads more
// into it. An ssh PASSWORD login carries result.Authorizer, which the chain
// bound at authentication time, so it does not come through here. An ssh
// PUBLIC-KEY login does: the server binds aaa.AuthorizerForResult over
// Config.Authorizer, which is the value this type answers for
// (internal/component/ssh/ssh.go, the WithPublicKeyAuth handler).
type liveAAABundleProfileAuthorizer struct {
	profiles []string
}

func (a liveAAABundleProfileAuthorizer) Authorize(username, remoteAddr, command string, isReadOnly bool) bool {
	bundle := aaaBundle.Load()
	if bundle == nil {
		// No accepted AAA bundle: startup has not installed authorization yet,
		// or shutdown has retired it. Deny, as liveAAABundleAuthorizer does.
		return false
	}
	if bundle.Authorizer == nil {
		// No local RBAC configured (no system.authorization profiles).
		return true
	}
	return aaa.BindProfiles(bundle.Authorizer, a.profiles).
		Authorize(username, remoteAddr, command, isReadOnly)
}

func (a liveAAABundleProfileAuthorizer) AuthorizeCommandArgs(
	username, remoteAddr, command string,
	args []string,
	peer string,
	isReadOnly bool,
) bool {
	bundle := aaaBundle.Load()
	if bundle == nil {
		// The same fallback as Authorize, and for the same reason. The two
		// methods MUST answer alike: a caller that reached one and was refused
		// by the other would see a command allowed by name and denied by its
		// arguments, with no policy behind either answer.
		return liveLocalAuthorizer{}.AuthorizeCommandArgs(username, remoteAddr, command, args, peer, isReadOnly)
	}
	if bundle.Authorizer == nil {
		return true
	}
	authorizer := aaa.BindProfiles(bundle.Authorizer, a.profiles)
	if typed, ok := authorizer.(aaa.CommandArgsAuthorizer); ok {
		return typed.AuthorizeCommandArgs(username, remoteAddr, command, args, peer, isReadOnly)
	}
	return authorizer.Authorize(
		username,
		remoteAddr,
		aaa.CanonicalCommand(command, args, peer),
		isReadOnly,
	)
}

// liveAAABundleAccountant records commands against the AAA bundle installed at
// the moment of each call. It issues its own task ids, so one command's START
// and STOP stay paired across a reload that replaces the chain between them.
type liveAAABundleAccountant struct {
	mu     sync.Mutex
	nextID uint64
	tasks  map[string]liveAAAAccountingTask
}

// liveAAAAccountingTask is one command in flight: the bundle whose accountant
// took its START, that accountant, and the task id it issued.
type liveAAAAccountingTask struct {
	bundle     *aaa.Bundle
	accountant aaa.Accountant
	taskID     string
}

func newLiveAAABundleAccountant() *liveAAABundleAccountant {
	return &liveAAABundleAccountant{
		tasks: make(map[string]liveAAAAccountingTask),
	}
}

func (a *liveAAABundleAccountant) CommandStart(username, remoteAddr, command string) string {
	bundle := aaaBundle.Load()
	if bundle == nil || bundle.Accountant == nil {
		return ""
	}
	accountant := bundle.Accountant
	taskID := accountant.CommandStart(username, remoteAddr, command)

	a.mu.Lock()
	a.nextID++
	liveTaskID := strconv.FormatUint(a.nextID, 10)
	a.tasks[liveTaskID] = liveAAAAccountingTask{
		bundle:     bundle,
		accountant: accountant,
		taskID:     taskID,
	}
	a.mu.Unlock()
	return liveTaskID
}

func (a *liveAAABundleAccountant) CommandStop(taskID, username, remoteAddr, command string) {
	if taskID == "" {
		return
	}
	a.mu.Lock()
	task, ok := a.tasks[taskID]
	if ok {
		delete(a.tasks, taskID)
	}
	a.mu.Unlock()
	if !ok {
		return
	}
	accountant := task.accountant
	// A reload between START and STOP retires the accountant that took the
	// START: the install closes the bundle it replaces, which stops the TACACS+
	// accounting worker, and a send to a stopped worker drops the record and
	// returns nothing (internal/component/tacacs/accounting.go, enqueue). Send
	// the STOP to the accountant installed NOW instead, carrying the task id the
	// START carried, which is what pairs the two records for the server that
	// reads them.
	if live := aaaBundle.Load(); live != task.bundle && live != nil && live.Accountant != nil {
		accountant = live.Accountant
	}
	accountant.CommandStop(task.taskID, username, remoteAddr, command)
}
