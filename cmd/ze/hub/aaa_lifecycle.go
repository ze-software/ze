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
func swapAAABundle(b *aaa.Bundle, logger *slog.Logger) {
	aaaBundleBootClaimed.Store(true)
	prev := aaaBundle.Swap(b)
	if prev != nil && prev != b {
		if err := prev.Close(); err != nil && logger != nil {
			logger.Warn("aaa: previous bundle close error on swap", "error", err)
		}
	}
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
	prev := aaaBundle.Swap(nil)
	if prev != nil {
		if err := prev.Close(); err != nil && logger != nil {
			logger.Warn("aaa: bundle close error on shutdown", "error", err)
		}
	}
	acceptedLocalIdentity.Store(nil)
	aaa.SetAcceptedLocalProfileGeneration(0)
	aaaBundleBootClaimed.Store(false)
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

type liveAAABundleAuthorizer struct{}

func (liveAAABundleAuthorizer) Authorize(username, remoteAddr, command string, isReadOnly bool) bool {
	bundle := aaaBundle.Load()
	if bundle == nil {
		// No accepted AAA bundle means startup has not installed authorization
		// yet. Deny until a non-nil bundle explicitly establishes either RBAC
		// policy or the existing no-RBAC allow mode.
		return false
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
		return false
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

type liveAAABundleAccountant struct {
	mu     sync.Mutex
	nextID uint64
	tasks  map[string]liveAAAAccountingTask
}

type liveAAAAccountingTask struct {
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
	task.accountant.CommandStop(task.taskID, username, remoteAddr, command)
}
