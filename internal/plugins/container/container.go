// Design: docs/architecture/container/runtime.md -- the engine entry, the logger and the command surface
// Related: register.go -- the registration that names this entry and the command proxy
// Related: supervise.go -- the supervisor the entry builds and the callbacks reach
// Related: config.go -- the one rule set the config transaction runs
//
// Package container runs OCI containers inside ze: it describes each
// container the config holds, starts it through the vendored runtime library,
// supervises it and removes it. The image store, the container networks and
// the shell are sibling packages of this plugin and reach it through the
// seams of storeseam.go and participant.go.
package container

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// pluginName is the plugin's name in the registry, its log subsystem and its
// config root's owner.
const pluginName = "container"

// The restart command: its path in the command tree, the wire method the
// YANG node binds (yang/ze-container-cmd.yang), and the keyword that types
// its selector. The three strings are the contract between the schema, the
// proxy registration and handleCommand.
const (
	cmdRequestRestart  = "request container restart"
	wireRequestRestart = "ze-container:request-restart"
	selectorKeyword    = "name"
)

// containerPlaceholder is the invocation-form token of a container name in a
// command's declared arguments, and keyState is the JSON key of a state in
// an answer.
const (
	containerPlaceholder = "<container>"
	keyState             = "state"
)

// drainBudgetSeconds is the longest the stop drain may take, declared at
// Stage 1 before any config arrives. It is spent in order: up to 2 seconds to
// end shell sessions, up to 30 seconds for the containers to obey their stop
// signals, which is why stop-timeout is at most 30, and 5 seconds for the
// control group kill and the cleanup.
const drainBudgetSeconds = 37

// loggerPtr holds the plugin's logger. It is an atomic pointer because tests
// run several in-process plugin instances at once.
var loggerPtr atomic.Pointer[slog.Logger]

// logger answers the plugin's logger, which discards every line until the
// engine or the command line set one.
func logger() *slog.Logger {
	if configured := loggerPtr.Load(); configured != nil {
		return configured
	}
	return slogutil.DiscardLogger()
}

// setLogger installs the plugin's logger. A nil logger changes nothing.
func setLogger(configured *slog.Logger) {
	if configured != nil {
		loggerPtr.Store(configured)
	}
}

// runContainerEngine is the plugin's entry point, internal and forked alike.
// Every side effect of the plugin starts here or later: a declaration query
// never calls it.
func runContainerEngine(conn net.Conn) int {
	log := logger()
	p := sdk.NewWithConn(pluginName, conn)
	defer func() {
		if err := p.Close(); err != nil {
			log.Debug("container: closing the plugin connection failed", "error", err)
		}
	}()

	ctx, cancel := sdk.SignalContext()
	defer cancel()

	// The runtime library is nil off Linux, where the supervisor refuses
	// every start with the reason.
	sup := newSupervisor(log, newRuntimeLibrary(log), engineStoreFactory(), participants)
	sup.tunables = newSysctlClaim(p)
	sup.siblings = &registeredSiblings
	defer sup.close()
	// Deferred after close, so it runs first: once the engine connection
	// closed, whatever still runs is killed. After a drain nothing does; when
	// the daemon's stop skipped the drain, this is the stop.
	defer sup.killRunning()

	// The siblings hold the engine client before the sweep makes their ended
	// calls. It refuses every request until the engine started the plugin.
	engine := &startedEngine{engine: p}
	registeredSiblings.handEngine(engine)

	// What a previous run left, a killed daemon's containers among them, is
	// removed before the engine is even asked to start this run: containers
	// are started again, never adopted.
	sup.sweep()

	// One parser serves the boot configure, the reload's verify and the
	// static verifier of the registration, so the three give one verdict.
	// The install applies the siblings' part, then installs the desired set
	// and wakes each container's worker; it never waits for a start.
	installer := newConfigInstaller(ctx, sup, registeredSiblings.configs)
	tx := sdk.NewConfigTx(containerConfig{}, parseContainerSections, installer.install)
	tx.Register(p)

	// A worker's tunable claim and a sibling's apply are calls to the engine,
	// which the engine answers only once the startup handshake ended, so the
	// siblings' part of the boot configure is applied, and its containers
	// start, from here.
	p.OnStarted(func(context.Context) error {
		engine.open()
		return installer.begin()
	})

	// A whole-daemon stop closes the connection with no notice, so the drain
	// is the one call that waits for the containers to stop.
	p.OnDrain(sup.drain)

	p.OnExecuteCommand(func(_ string, command string, args []string, _ string) (string, any, error) {
		return handleCommand(ctx, sup, command, args)
	})

	// The supervisor's records live in this process, internal and forked
	// alike, so show doctor asks it for the runtime checks declared below.
	p.OnDoctorCheck(sup.answerDoctorCheck)

	if err := p.Run(ctx, engineRegistration()); err != nil {
		// The engine reports a plugin that is not running under this code,
		// so the line and the doctor row name the same problem.
		log.Error("container: plugin failed", "error", err, "code", diagnostic.CodeDoctorPluginStopped)
		return 1
	}
	return 0
}

// engineRegistration is the Stage 1 declaration the engine entry sends: the
// config root, the commands, the drain budget, the failure policy and the
// runtime doctor checks. It is a pure value.
func engineRegistration() *sdk.Registration {
	return &sdk.Registration{
		WantsConfig:   []string{configRoot},
		Commands:      commandDecls(),
		DrainBudget:   drainBudgetSeconds,
		FailurePolicy: rpc.FailureRestart,
		DoctorChecks:  runtimeDoctorCheckDecls(),
	}
}

// commandDecls are the commands this plugin answers (SDK Stage 1): the
// restart, then the image commands (image_cmd.go), then each sibling's
// (sibling.go). The registration and the engine entry declare the same list
// from this function. The registration calls it from this package's init,
// before any sibling registered, so the registry's copy holds no sibling
// command: a sibling command reaches the daemon at Stage 1 only.
func commandDecls() []sdk.CommandDecl {
	decls := append([]sdk.CommandDecl{
		{
			Name:      cmdRequestRestart,
			ShortHelp: "Stop the named container, then start it again.",
			// The selector is mandatory, and both catalogs publish these
			// tokens as the invocation form.
			Args: []string{selectorKeyword, containerPlaceholder},
		},
	}, imageCommandDecls()...)
	return append(decls, registeredSiblings.commandDecls()...)
}

// handleCommand answers one command from the engine side. An image command
// is looked up in its table, then a sibling's command in the siblings'; the
// restart is the one command left.
func handleCommand(ctx context.Context, sup *supervisor, command string, args []string) (string, any, error) {
	if handler, ok := imageCommandHandlers[command]; ok {
		return runImageCommand(ctx, sup, command, handler, args)
	}
	if handler, ok := sup.siblings.commandHandler(command); ok {
		answer, err := handler(ctx, args)
		if err != nil {
			return rpc.StatusError, nil, err
		}
		return rpc.StatusDone, answer, nil
	}
	if command != cmdRequestRestart {
		// The declared list and this handler drifted apart: a Ze defect the
		// caller reads as a refusal, never as an answer.
		var tb textbuf.Buffer
		return rpc.StatusError, nil, errors.New(tb.Str("container: unknown command ").Quoted(command).String())
	}
	name := selectorValue(args)
	if name == "" {
		return rpc.StatusError, nil, opererr.New(
			"request container restart needs the name of a container",
			"the command restarts one container, and none was named",
			"ze request container restart name NAME",
		)
	}
	state, err := sup.restart(ctx, name)
	if err != nil {
		return rpc.StatusError, nil, err
	}
	return rpc.StatusDone, map[string]any{"name": name, keyState: state.String()}, nil
}

// selectorValue answers the value after the name keyword, or the one bare
// word a programmatic sender passes after it stripped the keyword. It answers
// empty for a keyword with nothing after it: that is an incomplete command,
// never a container called "name".
func selectorValue(args []string) string {
	return singleValue(args, selectorKeyword)
}
