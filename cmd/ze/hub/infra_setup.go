// Design: docs/architecture/hub-architecture.md -- infrastructure server setup
// Related: main.go -- hub entry point calls infra.SetHook before engine start

package hub

import (
	"context"
	"log/slog"

	"github.com/ze-software/ze/internal/component/config/infra"

	"github.com/ze-software/ze/internal/component/aaa"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/audit"
	coreenv "github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// buildAAABundle composes the AAA bundle through the pluggable backend
// registry. The hub does not import any backend package by name: every
// backend self-registers via init() in its own package (authz, tacacs,
// future RADIUS/LDAP) and Build assembles the live Authenticator chain,
// Authorizer, and Accountant.
//
// The local backend always receives liveLocalAuthorizer and the accepted users
// closure. Both dereference acceptedLocalIdentity, so one generation supplies
// credentials and policy. A nil policy preserves no-RBAC allow behavior.
//
// liveUsers is the accepted view of the same credentials `users` holds. The
// bundle is built once per daemon boot and is not rebuilt when the
// infrastructure hook reenters, so its local backend must dereference the
// accepted generation on each login. A nil liveUsers leaves snapshot behavior
// for callers with no reload lifecycle.
func buildAAABundle(tree *zeconfig.Tree, users []aaa.UserCredential, liveUsers func() ([]aaa.UserCredential, error), logger *slog.Logger) (*aaa.Bundle, error) {
	params := aaa.BuildParams{
		Ctx:             context.Background(),
		ConfigTree:      tree,
		Logger:          logger,
		LocalUsers:      users,
		LocalUsersFunc:  liveUsers,
		LocalAuthorizer: liveLocalAuthorizer{},
	}
	return aaa.Default.Build(params)
}

type accountingDropCounter interface {
	DropCount() uint64
}

func registerAAAAccountingProvider(bundle *aaa.Bundle) {
	var counter accountingDropCounter
	if bundle != nil {
		counter, _ = bundle.Accountant.(accountingDropCounter)
	}
	aaa.RegisterAAAAccountingProvider(func() map[string]any {
		var drops uint64
		if counter != nil {
			drops = counter.DropCount()
		}
		return map[string]any{"dropped-records": drops}
	})
}

// setupInfraHook creates and registers the infrastructure setup hook.
// reloadFn is the daemon-reaching config reload threaded into every SSH
// session editor (commit = apply + propagate); the hub passes a late-bound
// wrapper because the hook is registered before reloadAfterCommit exists.
// liveUsers reads the accepted local identity generation used by the AAA chain
// and optional SSH. It never reads the candidate ConfigProvider.
func setupInfraHook(recorder audit.Recorder, reloadFn func() error, liveUsers func() ([]aaa.UserCredential, error)) {
	infra.SetHook(func(params infra.HookParams) {
		_ = infraSetup(params, recorder, reloadFn, liveUsers)
	})
}

// infraSetup reuses the boot-owned AAA bundle for the always-on authorization,
// accounting, and optional SSH wiring. The first invocation owns the daemon's
// sole build attempt, including a failed attempt, so hook reentry cannot publish
// candidate backends before reload acceptance.
// It returns the optional SSH server handle after wiring that same bundle
// through the compile-out seam.
func infraSetup(params infra.HookParams, recorder audit.Recorder, reloadFn func() error, liveUsers func() ([]aaa.UserCredential, error)) sshServer {
	log := slogutil.Logger("hub.infra")
	r := params.Reactor

	// ssh server handle; built via the seam below (nil when ssh is compiled out).
	var sshSrv sshServer
	sshCfg := params.SSHConfig
	hasSSHConfig := sshCfg.HasConfig

	// Ephemeral mode: config edit starts daemon with ze.ssh.ephemeral.
	ephemeralFile := coreenv.Get("ze.ssh.ephemeral")
	if !hasSSHConfig && ephemeralFile != "" {
		sshCfg = infra.SSHExtractedConfig{
			Listen:    "127.0.0.1:0",
			HasConfig: true,
		}
		hasSSHConfig = true
	}

	// Resolve the accepted generation once. Its snapshot is the common
	// construction input for initial local AAA and optional SSH, while their
	// live callbacks continue to follow later atomic publications. A source
	// error is fail closed: SSH is not constructed, and an initial bundle build
	// receives neither local users nor a callback.
	var users []aaa.UserCredential
	usersReady := true
	aaaLiveUsers := liveUsers
	if liveUsers != nil {
		var liveErr error
		users, liveErr = liveUsers()
		if liveErr != nil {
			log.Warn("live local user source unavailable", "error", liveErr)
			users = nil
			usersReady = false
			aaaLiveUsers = nil
		}
	}
	// Warn on every ze:bcrypt canonical leaf that holds a non-bcrypt value.
	// `ze config validate` already surfaces this, but a daemon reload from a
	// hand-edited file (or a fleet push) bypasses validate -- the warning
	// here ensures operators see the problem in daemon logs.
	//
	// Note: zeconfig.YANGSchema() re-parses the YANG modules on every call
	// (no cache as of 2026-04-15). Cost is sub-millisecond and reload is
	// rare, so the duplicate work is acceptable. If reload latency ever
	// matters, thread a *config.Schema through infra.HookParams so the
	// loader's already-parsed schema is reused here.
	if params.ConfigTree != nil {
		if schema, schemaErr := zeconfig.YANGSchema(); schemaErr == nil {
			for _, msg := range zeconfig.CheckBcryptLeaves(params.ConfigTree, schema) {
				log.Warn("password leaf format invalid", "detail", msg)
			}
		}
	}

	// Build AAA only on the daemon's first boot-owned attempt. A no-BGP boot
	// installs the bundle before this hook can run; BGP auto-load must reuse that
	// exact pointer so established authorization and in-flight accounting never
	// cross a backend close or production bundle swap. A failed initial build
	// also consumes the attempt and keeps later hook reentry fail closed.
	bundle := aaaBundle.Load()
	if bundle == nil && claimAAABundleBoot() {
		var buildErr error
		bundle, buildErr = buildAAABundle(params.ConfigTree, users, aaaLiveUsers, log)
		if buildErr != nil {
			log.Warn("AAA backend build failed", "error", buildErr)
			bundle = nil
		}
		registerAAAAccountingProvider(bundle)
		swapAAABundle(bundle, log)
	}

	// Build the ssh server through the compile-out seam (ssh_infra.go). When
	// ssh is compiled out (ze_ssh off) sshBuild is nil and ssh is skipped; the
	// AAA/authz/accounting work above still runs for MCP/API.
	if hasSSHConfig && usersReady && bundle != nil && sshBuild != nil {
		sshSrv = sshBuild(&sshBuildInputs{
			Config:        sshCfg,
			Users:         users,
			UsersFunc:     aaaLiveUsers,
			Authenticator: bundle.Authenticator,
			Authorizer:    bundle.Authorizer,
			Recorder:      recorder,
			EphemeralFile: ephemeralFile,
			Params:        params,
			ReloadFn:      reloadFn,
			Log:           log,
		})
	}

	authzStore := params.AuthzStore
	needsPostStart := authzStore != nil || sshSrv != nil || bundle != nil
	if needsPostStart {
		r.SetPostStartFunc(func() {
			d := r.Dispatcher()
			if d == nil {
				return
			}

			// The stop a restarting speaker takes: persist the
			// graceful-restart marker through the always-on seam, then drop
			// the sessions in SILENCE. The two halves live together here, and
			// in one place only, because they are one decision -- RFC 4724
			// Section 5 has a peer delete every route of a connection that
			// ends in a NOTIFICATION, and puts no condition on that. A Cease
			// sent beside this marker would therefore foreclose, for every
			// peer, the retention the marker is written to ask for.
			//
			// The marker writer is registered by the gated BGP config package;
			// with ze_bgp off it stays nil and the write is a no-op -- correct,
			// since a BGP-less daemon has no session for a peer to treat as
			// restarting.
			stopForRestart := func() {
				if apiSrv := params.APIServer(); apiSrv != nil {
					infra.WriteGRMarker(apiSrv.AllPluginCapabilities(), params.Store)
				}
				r.StopForRestart()
			}

			if apiSrv := params.APIServer(); apiSrv != nil {
				apiSrv.SetRebootFunc(func() {
					rebootRequested.Store(true)
					stopForRestart()
				})
			}

			if bundle != nil && bundle.Authorizer != nil {
				d.SetAuthorizer(bundle.Authorizer)
				log.Info("authorization configured", "source", "aaa bundle")
			} else if params.AuthzStore != nil {
				d.SetAuthorizer(liveLocalAuthorizer{})
				log.Info("authorization profiles loaded")
			}

			if bundle != nil && bundle.Accountant != nil {
				d.SetAccountingHook(bundle.Accountant)
				log.Info("AAA accounting enabled")
			}

			if sshSrv != nil && sshWirePostStart != nil {
				sshWirePostStart(sshSrv, &sshWireInputs{
					Reactor:        r,
					Params:         params,
					StopForRestart: stopForRestart,
				})
				log.Info("SSH command executor wired")
			}
		})
	}
	return sshSrv
}
