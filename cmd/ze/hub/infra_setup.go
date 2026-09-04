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
// liveUsers is the accepted view of the same credentials `users` holds. Boot
// builds the bundle once and the infrastructure hook reentering does not
// rebuild it, so its local backend must dereference the accepted generation on
// each login. A nil liveUsers leaves snapshot behavior for callers with no
// reload lifecycle.
//
// A config reload DOES rebuild it, through this same function
// (main_reload.go). That covers the remote backends, which hold the address,
// the secret and the timeout they were constructed with. It does not replace
// liveUsers: the reload publishes the accepted credentials before it swaps the
// chain, and a reload carrying no parsed tree publishes them and rebuilds
// nothing.
//
// The returned bundle OWNS live resources: Build has already opened the RADIUS
// client socket and started the TACACS+ accounting worker. A caller that does
// not install it MUST call Close on it. A caller that installs it hands that
// obligation on, to swapAAABundle at boot and to acceptReloadedAAA plus
// closeRetiredAAABundle on a reload; each closes the bundle it replaces.
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
			// Keep the chain that composed. See the same call in main.go: a
			// dropped backend must not take the local one with it.
			log.Error("AAA backend build failed, that backend cannot authenticate", "error", buildErr)
		}
		registerAAAAccountingProvider(bundle)
		swapAAABundle(bundle, log)
	}

	// Build the ssh server through the compile-out seam (ssh_infra.go). When
	// ssh is compiled out (ze_ssh off) sshBuild is nil and ssh is skipped; the
	// AAA/authz/accounting work above still runs for MCP/API.
	//
	// A NIL bundle means NOTHING composed: no backend built, so no account of
	// any kind can authenticate, and a listener that authenticates nobody is a
	// port rather than a service. ssh is not started.
	//
	// That state is rare and says what it means. A backend that will not build
	// is DROPPED and the chain composes without it (aaa.Build), so a keyless
	// TACACS+ server leaves the local backend in place and an operator logs in
	// against it. The chain's own rule then decides which backend answers: a
	// reject stops it, and any other error tries the next one.
	if hasSSHConfig && usersReady && bundle != nil && sshBuild != nil {
		sshSrv = sshBuild(&sshBuildInputs{
			Config:    sshCfg,
			Users:     users,
			UsersFunc: aaaLiveUsers,
			// Both are LIVE indirections over the atomic bundle slot, never
			// the fields of the bundle built above. ssh is started once and
			// reads its authenticator once, so a captured chain would keep
			// authenticating against the RADIUS or TACACS+ server the boot
			// tree named after a reload replaced it (aaa_lifecycle.go).
			//
			// No fallback here. The CHAIN is the fallback: local sits at
			// priority 200 behind TACACS+ and RADIUS, and a backend that will
			// not build is dropped rather than taking the chain with it.
			Authenticator: liveAAABundleAuthenticator{},
			Authorizer:    liveAAABundleAuthorizer{},
			Recorder:      recorder,
			EphemeralFile: ephemeralFile,
			Params:        params,
			ReloadFn:      reloadFn,
			Log:           log,
		})
	}

	// The post-start wiring is registered on every daemon, whatever boot
	// produced. It is what installs authorization on the shared dispatcher, and
	// a dispatcher with no authorizer allows every command, so the case this
	// used to skip -- no bundle, no profiles, no ssh -- was the one case that
	// most needed the deny.
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

		// LIVE adapters over the atomic bundle slot, never the boot
		// bundle's own values. SetPostStartFunc runs once and no reload
		// re-enters it, so a captured authorizer keeps deciding against
		// the TACACS+ server the BOOT tree named, and a captured
		// accountant is the one swapAAABundle's Close has already
		// stopped: its enqueue then drops every record and returns no
		// error. Both are the dispatcher's FALLBACK, used whenever a
		// request carries no per-session authorizer of its own
		// (Dispatcher.isAuthorized, internal/component/plugin/server/command.go).
		//
		// The authorizer is installed UNCONDITIONALLY, which is what the
		// no-BGP path already does (installNoBGPAAADispatch, main.go).
		// The dispatcher authorizes every command when its authorizer is
		// nil, so a boot whose AAA build failed and that configured no
		// authorization profiles used to leave every dispatched command
		// allowed. liveAAABundleAuthorizer denies while the slot is
		// empty, which is the same answer the no-BGP path gives, and it
		// delegates to bundle.Authorizer once a bundle is installed --
		// including liveLocalAuthorizer, which the local backend always
		// contributes (internal/component/authz/register.go, localBackend).
		d.SetAuthorizer(liveAAABundleAuthorizer{})
		log.Info("authorization configured", "source", "live aaa bundle")

		// The accounting hook is installed unconditionally for the same
		// reason: a reload that ADDS TACACS+ accounting must reach the
		// dispatcher, and boot is not the last word on what the bundle
		// holds. A live accountant over an absent bundle answers with an
		// empty task id and costs one call. The log still reports what
		// boot found, because that is what it observed.
		d.SetAccountingHook(newLiveAAABundleAccountant())
		if bundle != nil && bundle.Accountant != nil {
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
	return sshSrv
}
