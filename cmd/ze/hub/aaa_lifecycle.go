// Design: ai/patterns/registration.md -- AAA registry (VFS-like)
// Related: infra_setup.go -- infraSetup installs the bundle on config load
// Related: main.go -- runYANGConfig defers closeAAABundle on exit

package hub

import (
	"log/slog"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
)

// aaaBundle holds the live AAA bundle.
//
// Startup installs one bundle for the active reactor composition. Reload keeps
// that bundle and updates the local authorization store it reads. Replacing a
// bundle closes the previous one so backend workers drain cleanly.
//
// closeAAABundle is wired as a defer at the top of runYANGConfig so the
// currently-installed bundle is Close()d on any exit path (clean shutdown,
// error return, panic recovery).
var (
	aaaBundle           atomic.Pointer[aaa.Bundle]
	liveLocalAuthzStore atomic.Pointer[authz.Store]
)

// swapAAABundle installs the new bundle as the live one and closes the
// previously installed bundle (if any). Safe to call concurrently; safe
// with a nil b (treated as "unregister without replacement").
func swapAAABundle(b *aaa.Bundle, logger *slog.Logger) {
	prev := aaaBundle.Swap(b)
	if prev != nil && prev != b {
		if err := prev.Close(); err != nil && logger != nil {
			logger.Warn("aaa: previous bundle close error on swap", "error", err)
		}
	}
}

// swapLocalAuthzStore installs the authorization policy local AAA decisions
// consult. The store is immutable after extraction, so decisions need only one
// atomic load and do not rebuild configuration on the command path.
func swapLocalAuthzStore(store *authz.Store) {
	liveLocalAuthzStore.Store(store)
}

// closeAAABundle closes the installed bundle and clears the local authorization
// store. Called via defer from runYANGConfig so backend workers drain and no
// policy state survives daemon shutdown.
func closeAAABundle(logger *slog.Logger) {
	prev := aaaBundle.Swap(nil)
	if prev != nil {
		if err := prev.Close(); err != nil && logger != nil {
			logger.Warn("aaa: bundle close error on shutdown", "error", err)
		}
	}
	liveLocalAuthzStore.Store(nil)
}

// liveLocalAuthorizer is the local backend's stable AAA contribution. External
// authorizers keep registry priority over it, while TACACS+ receives the same
// value as its fallback. A nil live store is the existing no-RBAC allow mode.
type liveLocalAuthorizer struct{}

func (liveLocalAuthorizer) Authorize(username, remoteAddr, command string, isReadOnly bool) bool {
	return (authz.StoreAuthorizer{Store: liveLocalAuthzStore.Load()}).
		Authorize(username, remoteAddr, command, isReadOnly)
}

func (liveLocalAuthorizer) AuthorizeCommandArgs(username, remoteAddr, command string, args []string, peer string, isReadOnly bool) bool {
	return (authz.StoreAuthorizer{Store: liveLocalAuthzStore.Load()}).
		AuthorizeCommandArgs(username, remoteAddr, command, args, peer, isReadOnly)
}

type liveAAABundleAuthorizer struct{}

func (liveAAABundleAuthorizer) Authorize(username, remoteAddr, command string, isReadOnly bool) bool {
	bundle := aaaBundle.Load()
	if bundle == nil {
		// Fail open until infra setup swaps in the live AAA bundle. Authenticated
		// callers still need valid credentials; this only affects post-login RBAC.
		return true
	}
	if bundle.Authorizer == nil {
		// No local RBAC configured (no system.authorization profiles).
		return true
	}
	return bundle.Authorizer.Authorize(username, remoteAddr, command, isReadOnly)
}
