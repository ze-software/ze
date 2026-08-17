// Design: ai/patterns/registration.md -- AAA registry (VFS-like)
// Related: infra_setup.go -- infraSetup installs the bundle on config load
// Related: main.go -- runYANGConfig defers closeAAABundle on exit

package hub

import (
	"log/slog"
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

// publishAcceptedLocalIdentity installs one complete accepted identity
// generation. Callers must finish every fallible reload step before publishing
// a candidate.
func publishAcceptedLocalIdentity(state *acceptedLocalIdentityState) {
	aaa.SetAcceptedLocalProfileGeneration(state.generation)
	acceptedLocalIdentity.Store(state)
}

func newAcceptedLocalIdentity(users []aaa.UserCredential, store *authz.Store, resolveCandidateUsers func() ([]aaa.UserCredential, error), apiToken string) *acceptedLocalIdentityState {
	generation := localIdentityGeneration.Add(1)
	generationUsers := append([]aaa.UserCredential(nil), users...)
	for i := range generationUsers {
		generationUsers[i].LocalGeneration = generation
	}
	generationAuthorizer := acceptedLocalGenerationAuthorizer{store: store}
	return &acceptedLocalIdentityState{
		generation:            generation,
		users:                 generationUsers,
		authorizer:            store,
		resolveCandidateUsers: resolveCandidateUsers,
		apiToken:              apiToken,
		apiAuthentication:     buildAPIAuthentication(generationUsers, apiToken, generationAuthorizer),
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
func stageAPIAuthentication() func() {
	previous := acceptedLocalIdentity.Load()
	if previous == nil {
		return func() {}
	}
	staged := *previous
	staged.apiAuthentication = api.Authentication{Required: true}
	acceptedLocalIdentity.Store(&staged)
	return func() { acceptedLocalIdentity.Store(previous) }
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
