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
// The local backend always receives liveLocalAuthorizer. A nil live store
// preserves the no-RBAC allow behavior while keeping one stable contribution
// that can follow successful policy reloads.
//
// liveUsers is the running-config view of the same credentials `users` holds.
// The bundle is built once per reactor creation and is NOT rebuilt by a config
// reload, so without it the chain's local backend answers from the boot list
// forever, and it answers FIRST: a deleted user is accepted by the chain before
// any fallback can refuse them. A nil liveUsers leaves the snapshot behavior,
// which is correct only where no reload can reach the process.
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
// liveUsers is the running-config credential source the AAA chain's local
// backend answers from. It is threaded from the hub rather than derived from
// params.ConfigTree because that tree is the one the reactor was BUILT with:
// re-deriving from it would reproduce the snapshot this exists to remove.
func setupInfraHook(recorder audit.Recorder, reloadFn func() error, liveUsers func() ([]aaa.UserCredential, error)) {
	infra.SetHook(func(params infra.HookParams) {
		_ = infraSetup(params, recorder, reloadFn, liveUsers)
	})
}

// infraSetup builds the always-on infra (AAA bundle, authorization, accounting,
// reboot/GR marker) and, when ssh is compiled in, builds + wires the ssh server
// through the seam (ssh_infra.go). Returns the ssh server handle (nil if ssh is
// not configured, failed to start, or compiled out).
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

	// Resolve the shared running-config source once. Its successful snapshot is
	// the common boot input for local AAA and optional SSH. A source error is
	// fail-closed: other registered AAA backends are still built, but the local
	// backend receives no users or live callback, and SSH is not constructed
	// from an unreadable identity source.
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

	// Build the AAA bundle unconditionally. TACACS+ accounting fires on
	// every dispatched command (SSH, MCP, API), so the bundle must exist
	// even when SSH is disabled. Its local authorizer follows the live store
	// across reloads without rebuilding backend workers.
	bundle, buildErr := buildAAABundle(params.ConfigTree, users, aaaLiveUsers, log)
	if buildErr != nil {
		log.Warn("AAA backend build failed", "error", buildErr)
		bundle = nil
	}
	registerAAAAccountingProvider(bundle)
	swapAAABundle(bundle, log)

	// Build the ssh server through the compile-out seam (ssh_infra.go). When
	// ssh is compiled out (ze_ssh off) sshBuild is nil and ssh is skipped; the
	// AAA/authz/accounting work above still runs for MCP/API.
	if hasSSHConfig && usersReady && bundle != nil && sshBuild != nil {
		sshSrv = sshBuild(&sshBuildInputs{
			Config:        sshCfg,
			Users:         users,
			UsersFunc:     aaaLiveUsers,
			Authenticator: bundle.Authenticator,
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
