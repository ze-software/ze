// Design: docs/architecture/hub-architecture.md -- the load order: declared plugins run and declare before the full parse.
// Design: docs/architecture/api/process-protocol.md -- Stage 1 read early, answered in the plugin's startup tier.
// Related: startup_driver.go -- runStartupHandshake answers the request kept here.
// Related: ../manager/manager.go -- StartDeclared, FailDeclared and the SpawnMore that takes a held plugin.
// Related: ../ipc/rpc.go -- PluginConn.HoldStartupRequest keeps the request unanswered.

package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// EarlyDeclaration is what one declared external plugin told the daemon
// before the full config was parsed. HoldDeclaredPlugins builds it.
type EarlyDeclaration struct {
	// Plugin is the plugin's name, as the config or the command line declares it.
	Plugin string
	// Schemas are the YANG modules the plugin's Stage 1 declaration carries,
	// in the order it lists them. They are empty when the declaration carries
	// none, and when Err is set.
	Schemas []rpc.SchemaDecl
	// Err says why the daemon holds no declaration from this plugin: it could
	// not be started, it sent nothing within its bound, its connection ended,
	// or what it sent is not a declaration. It names the plugin, the reason
	// and the command that fixes it. nil means the plugin declared.
	Err error
}

// DeclaredSpawner starts the declared external plugins before the plugin
// server exists. The plugin manager implements it.
type DeclaredSpawner interface {
	// StartDeclared forks each config and answers the error of each one that
	// did not start, by name. environment holds the settings of the config's
	// `environment` block, for the environment of each fork alone.
	StartDeclared(ctx context.Context, configs []plugin.PluginConfig, environment []string) (map[string]error, error)
	// FailDeclared stops a held plugin and keeps cause as the reason.
	FailDeclared(ctx context.Context, name string, cause error)
	// GetProcessManager answers the *process.ProcessManager holding them.
	GetProcessManager() any
}

// HoldDeclaredPlugins runs the external plugins the config declares and reads
// the Stage 1 declaration of each, before the daemon can parse the full
// config: the declaration carries the plugin's YANG module, and the config is
// parsed against the modules joined from them. It answers one EarlyDeclaration
// for each config, in order.
//
// Every plugin that declared is left held at the barrier: its Stage 1 request
// is kept unanswered (PluginConn.HoldStartupRequest), so the plugin waits for
// its answer while the daemon joins the modules and parses the config. The
// plugin's startup tier then reads that same request, runs every Stage 1 check
// and answers it, so a refusal still reaches the plugin on the request it
// sent. The hold is a plugin waiting for an answer and nothing else: no tier
// coordinator exists yet, so no stage timer measures it.
//
// A plugin that does not declare is an outcome, never the end of the boot: its
// Err is kept for the refusal of a config section still unknown at the full
// parse. A plugin that connected and stays silent is bounded by its stage
// timeout (the plugin's own `timeout`, else ze.plugin.stage.timeout), counted
// from the moment every declared plugin was started; it is then stopped, and
// the startup phase that names it is refused with the same reason.
//
// environment holds the settings of the config's `environment` block, as
// "key=value" entries (config.EnvironmentEntries). The daemon applies the
// block to itself only after the full parse, so the plugins started here are
// given it for their own environment, under the daemon's: a plugin started
// before the parse reads the settings one started after it reads.
//
// With no config it starts nothing, waits for nothing and never calls spawner.
// ctx bounds every read: when it ends, the plugins not yet read answer its
// error. Not safe for concurrent use; boot calls it once.
func HoldDeclaredPlugins(ctx context.Context, spawner DeclaredSpawner, configs []plugin.PluginConfig, environment []string) []EarlyDeclaration {
	if len(configs) == 0 {
		return nil
	}

	declarations := make([]EarlyDeclaration, len(configs))
	failures, err := spawner.StartDeclared(ctx, configs, environment)
	if err != nil {
		for i := range configs {
			declarations[i] = EarlyDeclaration{Plugin: configs[i].Name, Err: notStartedRefusal(configs[i], err)}
		}
		return declarations
	}

	pm, ok := spawner.GetProcessManager().(*process.ProcessManager)
	if !ok {
		panic("BUG: the spawner started the declared plugins and holds no process manager")
	}
	if pm == nil {
		panic("BUG: the spawner started the declared plugins and answers a nil process manager")
	}

	// One start time for every plugin: each is read in turn, and a plugin's
	// declaration waits in its connection while an earlier one is read, so
	// the whole step is bounded by the longest stage timeout, not their sum.
	started := time.Now()
	for i := range configs {
		cfg := configs[i]
		declarations[i].Plugin = cfg.Name
		if failure := failures[cfg.Name]; failure != nil {
			declarations[i].Err = notStartedRefusal(cfg, failure)
			continue
		}
		proc := pm.GetProcess(cfg.Name)
		if proc == nil {
			panic("BUG: declared plugin " + cfg.Name + " started and is not in the process manager")
		}
		schemas, held, readErr := readEarlyDeclaration(ctx, proc, started.Add(earlyDeclarationBound(cfg)))
		if readErr != nil {
			declarations[i].Err = readErr
			if !held {
				spawner.FailDeclared(ctx, cfg.Name, readErr)
			}
			continue
		}
		declarations[i].Schemas = schemas
	}
	return declarations
}

// earlyDeclarationBound is how long a started plugin may stay silent before
// its Stage 1 declaration: the bound the startup tiers give a stage with no
// progress (startupStageStall), read with no coordinator.
func earlyDeclarationBound(cfg plugin.PluginConfig) time.Duration {
	if cfg.StageTimeout != 0 {
		return cfg.StageTimeout
	}
	return stageTimeoutFromEnv()
}

// earlyDeclarationFloor is the least time one read waits, whatever is left of
// the plugin's bound: long enough to take a declaration that already arrived,
// and too short to be a second bound.
const earlyDeclarationFloor = 100 * time.Millisecond

// readEarlyDeclaration reads and keeps the Stage 1 request of one started
// plugin and answers the modules it declares. held says whether the plugin is
// still held with a request for its tier to answer: true for a request that is
// not a readable declaration, which the tier refuses on the wire with its own
// text, and false when nothing was read, where the caller stops the plugin.
func readEarlyDeclaration(ctx context.Context, proc *process.Process, deadline time.Time) (schemas []rpc.SchemaDecl, held bool, err error) {
	cfg := proc.Config()
	if initErr := proc.InitConns(); initErr != nil {
		return nil, false, notDeclaredRefusal(cfg, "its connection could not be set up: "+initErr.Error())
	}
	conn := proc.Conn()
	if conn == nil {
		return nil, false, notDeclaredRefusal(cfg, "it has no connection to the daemon")
	}

	// A plugin read after a slow one can reach here with its bound already
	// spent and its declaration waiting in the connection. A read whose
	// context is already done could answer either, so every read gets a
	// moment in which only a waiting declaration can win.
	if floor := time.Now().Add(earlyDeclarationFloor); deadline.Before(floor) {
		deadline = floor
	}
	readCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	request, readErr := conn.HoldStartupRequest(readCtx)
	if readErr != nil {
		if ctx.Err() != nil {
			return nil, false, notDeclaredRefusal(cfg, "the daemon stopped waiting: "+ctx.Err().Error())
		}
		if errors.Is(readErr, context.DeadlineExceeded) {
			return nil, false, silentRefusal(cfg)
		}
		return nil, false, notDeclaredRefusal(cfg, "its connection ended before it sent one: "+readErr.Error())
	}
	if request.Method != rpc.MethodDeclareRegistration {
		return nil, true, unreadableDeclarationRefusal(cfg,
			"its first request was not "+rpc.MethodDeclareRegistration,
			fmt.Errorf("first request %q", request.Method))
	}
	input, decodeErr := rpc.DecodeDeclaration(cfg.Name, request.Params)
	if decodeErr != nil {
		if _, refused := errors.AsType[*opererr.Error](decodeErr); refused {
			// A declaration that carries a key ze does not know: the decoder
			// names the plugin, the key and the fix, and the tier refuses it
			// on the wire with the same text (runStartupHandshake).
			return nil, true, decodeErr
		}
		return nil, true, unreadableDeclarationRefusal(cfg, "its declaration is not valid JSON", decodeErr)
	}
	return input.Schemas, true, nil
}

// unreadableDeclarationRefusal is the outcome of a plugin whose first request
// the daemon read and cannot take as a declaration. What the plugin sent is
// text from outside Ze, so it travels as the cause and never inside the
// operator's sentence.
func unreadableDeclarationRefusal(cfg plugin.PluginConfig, reason string, cause error) error {
	return opererr.Wrap(cause,
		"external plugin \""+cfg.Name+"\" sent no declaration: "+reason,
		earlyDeclarationWhy,
		conformanceFix(cfg))
}

// earlyDeclarationWhy is the consequence every refusal of this file states.
const earlyDeclarationWhy = "ze reads each declared plugin's declaration before it parses the config, because the declaration carries " +
	"the plugin's config schema; without it a config section only that plugin's schema defines is refused as an unknown keyword, " +
	"and the plugin does not run"

// conformanceFix is the command that shows an author what a plugin's
// declaration lacks.
func conformanceFix(cfg plugin.PluginConfig) string {
	return "ze show plugin conformance run \"" + cfg.Run + "\""
}

// notStartedRefusal is the outcome of a declared plugin the daemon could not
// fork or that never connected back.
func notStartedRefusal(cfg plugin.PluginConfig, cause error) error {
	return opererr.Wrap(cause,
		"external plugin \""+cfg.Name+"\" could not be started, so it declared nothing",
		earlyDeclarationWhy,
		conformanceFix(cfg))
}

// notDeclaredRefusal is the outcome of a started plugin from which the daemon
// read no declaration, for the stated reason.
func notDeclaredRefusal(cfg plugin.PluginConfig, reason string) error {
	return opererr.New(
		"external plugin \""+cfg.Name+"\" sent no declaration: "+reason,
		earlyDeclarationWhy,
		conformanceFix(cfg))
}

// silentRefusal is the outcome of a plugin that connected and sent nothing
// within its bound. It names the timeout as well, because a plugin that is
// only slow to start is fixed by a longer one.
func silentRefusal(cfg plugin.PluginConfig) error {
	return opererr.New(
		"external plugin \""+cfg.Name+"\" connected and sent no declaration within "+earlyDeclarationBound(cfg).String()+", so it was stopped",
		earlyDeclarationWhy+"; a plugin that is only slow to start needs a longer ze.plugin.stage.timeout (environment ZE_PLUGIN_STAGE_TIMEOUT)",
		conformanceFix(cfg))
}
