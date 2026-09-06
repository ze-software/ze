// Design: docs/architecture/exabgp-bridge.md -- internal exabgp bridge runner
//
// runInternalBridge is the RunEngine entry point for the internal exabgp-bridge
// plugin. It mirrors the external SDK-mode runner (internal/plugins/exabgp
// main_sdk.go runSDKMode) but sources the script commands from the
// exabgp.bridge config root delivered via the SDK OnConfigure (Stage 2)
// callback, since a RunEngine(conn net.Conn) runner never sees the
// process-manager `run` line.
//
// The scripts themselves are run by a bridgerun.Fleet, which both runners share.

package bridgeplugin

import (
	"context"
	"log/slog"
	"net"
	"sync"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/exabgp/bridge"
	"github.com/ze-software/ze/internal/plugins/exabgp/bridgerun"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// familyDecls builds the Stage-1 family declarations. Config is not available
// at Stage-1 registration (it arrives at Stage-2 OnConfigure), so the runner
// declares the CLI-default family here; the `family` leaf refines the ADD-PATH
// capability encoding at Stage-3 (see runInternalBridge).
func familyDecls(families []string) []sdk.FamilyDecl {
	var decls []sdk.FamilyDecl
	for _, f := range families {
		fam, ok := family.LookupFamily(f)
		if !ok {
			continue
		}
		decls = append(decls, sdk.FamilyDecl{Name: f, Mode: "both", AFI: uint16(fam.AFI), SAFI: uint8(fam.SAFI)})
	}
	if len(decls) == 0 {
		if fam, ok := family.LookupFamily(defaultFamily); ok {
			decls = append(decls, sdk.FamilyDecl{Name: defaultFamily, Mode: "both", AFI: uint16(fam.AFI), SAFI: uint8(fam.SAFI)})
		}
	}
	return decls
}

// capabilityDecls builds the Stage-3 capability declarations from config.
func capabilityDecls(cfg bridgeConfig) []sdk.CapabilityDecl {
	var caps []sdk.CapabilityDecl
	if cfg.RouteRefresh {
		// RFC 2918: route-refresh capability, code 2, zero-length value.
		caps = append(caps, sdk.CapabilityDecl{Code: 2})
	}
	if cfg.AddPath != "" && cfg.AddPath != addPathNone {
		if hex := bridge.EncodeAddPathHex(cfg.Families, cfg.AddPath); hex != "" {
			caps = append(caps, sdk.CapabilityDecl{Code: 69, Encoding: sdk.CapEncodingHex, Payload: hex})
		}
	}
	return caps
}

// bridgeRunner holds the state shared between the OnConfigure (config capture)
// callback, the OnAllPluginsReady (start) callback, the OnEvent (write)
// callback, and the shutdown path.
type bridgeRunner struct {
	log *slog.Logger

	mu sync.Mutex
	// config is the committed bridge config, captured at OnConfigure and read
	// by the start callback. The two run in different startup stages, so the
	// script commands cannot be passed between them on the stack.
	config  bridgeConfig
	started bool
	// fleet runs every configured script. It is nil until OnAllPluginsReady.
	fleet *bridgerun.Fleet
}

// scripts answers the running fleet, or nil while none is running.
func (r *bridgeRunner) scripts() *bridgerun.Fleet {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fleet
}

// runInternalBridge runs the ExaBGP bridge in-process. It spawns every script
// the config declares as a subprocess and translates between ze JSON events and
// the external text/JSON commands, exactly like the external SDK-mode runner
// but with the script commands sourced from config. Returns the plugin exit
// code.
func runInternalBridge(conn net.Conn) int {
	log := logger()
	r := &bridgeRunner{log: log}

	p := sdk.NewWithConn("exabgp-bridge", conn)
	defer func() { _ = p.Close() }()

	ctx, cancel := sdk.SignalContext()
	defer cancel()

	// Subscribe to all events so the bridge can translate them for the scripts.
	// Config-independent, so set before Run (Stage 5 reads it after this).
	p.SetStartupSubscriptions([]string{"*"}, nil, "")

	// ze events -> external JSON on every subprocess stdin. Registered before Run
	// so the bridge (Stage 5) captures it; no-ops until the subprocesses start.
	p.OnEvent(r.onEvent)

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, s := range sections {
			if s.Root != configRoot {
				continue
			}
			cfg, err := parseConfig(s.Data)
			if err != nil {
				return err
			}
			if !cfg.Present {
				continue
			}
			if err := r.applyConfig(p, cfg); err != nil {
				return err
			}
			return nil
		}
		return nil
	})

	// The scripts start here rather than at OnConfigure, because a script's first
	// line is a command and a command dispatched before the registries are frozen
	// aborts the startup barrier. startWhenReady says what that cost.
	p.OnAllPluginsReady(func() error { return r.startWhenReady(ctx, p) })

	runErr := p.Run(ctx, bridgeRegistration())

	// Shutdown: cancel the subprocess context, close each stdin (EOF), wait.
	cancel()
	r.stop()

	if runErr != nil && ctx.Err() == nil {
		log.Error("exabgp-bridge internal plugin failed", "error", runErr)
		return 1
	}
	log.Info("exabgp-bridge internal plugin stopped")
	return 0
}

// applyConfig declares the capabilities the config asks for and remembers the
// script commands. It does NOT start the scripts: see startWhenReady.
//
// Reloads do not restart the subprocesses: a script owns its own lifecycle, and
// family/capability changes need a plugin restart (engine reconcile), matching
// the external mode.
func (r *bridgeRunner) applyConfig(p *sdk.Plugin, cfg bridgeConfig) error {
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return nil
	}
	r.config = cfg
	r.mu.Unlock()

	if len(cfg.Scripts) == 0 {
		return errRunRequired
	}

	if caps := capabilityDecls(cfg); len(caps) > 0 {
		p.SetCapabilities(caps)
	}
	return nil
}

// startWhenReady starts every configured script once the engine has loaded
// every plugin in every startup phase and frozen the dispatcher registry.
//
// A script's very first line is a command, and the bridge dispatches it the
// moment it is read. Starting the scripts at OnConfigure therefore put a
// dispatch-command frame on the wire during the bridge's own 5-stage handshake:
// the engine was at stage 5 waiting for `ready`, read the dispatch instead, and
// aborted the startup barrier, which took the bgp plugin and the daemon with it
// (`stage 5: expected ready, got ze-plugin-engine:dispatch-command`).
//
// A script line can also address ANOTHER plugin -- `announce watchdog <name>`
// reaches bgp-watchdog -- so even a handshake-safe start is too early at
// OnStarted, where a target in a later startup phase has not registered its
// command yet. OnAllPluginsReady is the one point where both are settled
// (pkg/plugin/sdk/sdk_callbacks.go, OnAllPluginsReady).
func (r *bridgeRunner) startWhenReady(ctx context.Context, p *sdk.Plugin) error {
	r.mu.Lock()
	cfg := r.config
	started := r.started
	r.mu.Unlock()
	if started || len(cfg.Scripts) == 0 {
		return nil
	}

	fleet := bridgerun.New(r.log, cfg.Families, cfg.Scripts)
	if err := fleet.Start(ctx, p); err != nil {
		// A script that did start is stopped here rather than left running
		// behind a failed startup.
		fleet.Stop()
		return err
	}

	r.mu.Lock()
	r.fleet = fleet
	r.started = true
	r.mu.Unlock()

	r.log.Info("exabgp-bridge started", "scripts", fleet.Count(), "families", cfg.Families)
	return nil
}

// onEvent hands one ze JSON event to every running script. No-op until the
// scripts start.
func (r *bridgeRunner) onEvent(event string) error {
	if fleet := r.scripts(); fleet != nil {
		fleet.Broadcast(event)
	}
	return nil
}

// stop closes every script's stdin (EOF) and waits for the scripts and their
// goroutines. The caller MUST cancel the run context before calling it.
func (r *bridgeRunner) stop() {
	if fleet := r.scripts(); fleet != nil {
		fleet.Stop()
	}
}

// bridgeRegistration is the Stage-1 declaration the bridge makes about itself.
//
// The failure policy is the part an operator's configuration is checked against.
// ExaBGP restarts a process that exits unless the block turns it off
// (src/exabgp/configuration/process/__init__.py, 'respawn': True), and the
// bridge is what ze runs those scripts under, so it declares that it may be
// started again. A migrated ExaBGP configuration that wrote `respawn true` is
// then asking for something ze can give, and its scripts come back after a
// crash the way their author expected.
func bridgeRegistration() sdk.Registration {
	return sdk.Registration{
		Families:      familyDecls([]string{defaultFamily}),
		WantsConfig:   []string{configRoot},
		FailurePolicy: sdk.FailureRestart,
	}
}
