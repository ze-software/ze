// Design: docs/architecture/hub-architecture.md -- infrastructure server setup
// Related: main.go -- hub entry point calls infra.SetHook before engine start

package hub

import (
	"context"
	"log/slog"

	"github.com/ze-software/ze/internal/component/config/infra"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
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
// A nil store yields a nil LocalAuthorizer so the local backend does not
// contribute a permissive "allow all" authorizer when the caller explicitly
// has no RBAC configured.
func buildAAABundle(tree *zeconfig.Tree, users []aaa.UserCredential, store *authz.Store, logger *slog.Logger) (*aaa.Bundle, error) {
	var localAuthorizer aaa.Authorizer
	if store != nil {
		localAuthorizer = authz.StoreAuthorizer{Store: store}
	}
	params := aaa.BuildParams{
		Ctx:             context.Background(),
		ConfigTree:      tree,
		Logger:          logger,
		LocalUsers:      users,
		LocalAuthorizer: localAuthorizer,
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
func setupInfraHook(recorder audit.Recorder, reloadFn func() error) {
	infra.SetHook(func(params infra.HookParams) {
		_ = infraSetup(params, recorder, reloadFn)
	})
}

// infraSetup builds the always-on infra (AAA bundle, authorization, accounting,
// reboot/GR marker) and, when ssh is compiled in, builds + wires the ssh server
// through the seam (ssh_infra.go). Returns the ssh server handle (nil if ssh is
// not configured, failed to start, or compiled out).
func infraSetup(params infra.HookParams, recorder audit.Recorder, reloadFn func() error) sshServer {
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

	// Users from zefs + config. Loaded regardless of SSH so the local AAA
	// backend sees them even on API-only or MCP-only deployments where
	// authorization and accounting must still apply.
	var zefsUsers []aaa.UserCredential
	if u, err := loadZefsUsers(); err == nil {
		zefsUsers = u
	}
	users := mergeAuthUsers(zefsUsers, sshCfg.Users)

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
	// even when SSH is disabled. On config reload the previous bundle is
	// closed so backend workers (TACACS+ accounting) drain.
	bundle, buildErr := buildAAABundle(params.ConfigTree, users, params.AuthzStore, log)
	if buildErr != nil {
		log.Warn("AAA backend build failed", "error", buildErr)
		bundle = nil
	}
	registerAAAAccountingProvider(bundle)
	swapAAABundle(bundle, log)

	// Build the ssh server through the compile-out seam (ssh_infra.go). When
	// ssh is compiled out (ze_ssh off) sshBuild is nil and ssh is skipped; the
	// AAA/authz/accounting work above still runs for MCP/API.
	if hasSSHConfig && bundle != nil && sshBuild != nil {
		sshSrv = sshBuild(&sshBuildInputs{
			Config:        sshCfg,
			Users:         users,
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

			// Persist the graceful-restart marker through the always-on seam.
			// The writer is registered by the gated BGP config package; with
			// ze_bgp off it stays nil and this is a no-op -- correct, since a
			// BGP-less daemon has no session for a peer to treat as restarting.
			writeGRMarker := func() {
				apiSrv := params.APIServer()
				if apiSrv == nil {
					return
				}
				infra.WriteGRMarker(apiSrv.AllPluginCapabilities(), params.Store)
			}

			if apiSrv := params.APIServer(); apiSrv != nil {
				apiSrv.SetRebootFunc(func() {
					writeGRMarker()
					rebootRequested.Store(true)
					r.Stop()
				})
			}

			if bundle != nil && bundle.Authorizer != nil {
				d.SetAuthorizer(bundle.Authorizer)
				log.Info("authorization configured", "source", "aaa bundle")
			} else if authzStore != nil {
				d.SetAuthorizer(authz.StoreAuthorizer{Store: authzStore})
				log.Info("authorization profiles loaded")
			}

			if bundle != nil && bundle.Accountant != nil {
				d.SetAccountingHook(bundle.Accountant)
				log.Info("AAA accounting enabled")
			}

			if sshSrv != nil && sshWirePostStart != nil {
				sshWirePostStart(sshSrv, &sshWireInputs{
					Reactor:       r,
					Params:        params,
					WriteGRMarker: writeGRMarker,
				})
				log.Info("SSH command executor wired")
			}
		})
	}
	return sshSrv
}
