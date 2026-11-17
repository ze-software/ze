// Design: docs/architecture/container/runtime.md -- plugin registration, the config root and the command proxy
// Related: container.go -- the engine entry and the command this file proxies
// Related: config.go -- the rule set this file names as the static verifier
// Related: register_initrole_linux.go -- the early role a container's init takes
//
// Registration is the plugin's whole coupling to ze: an init() that hands the
// registry a Registration, plus the generated blank imports. A build without
// the ze_container tag links none of this, so the container keyword, the
// command and the init role are all absent from it.
package container

import (
	"os"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	containeryang "github.com/ze-software/ze/internal/plugins/container/yang"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

//nolint:gochecknoinits // registration: the plugin is discovered through the registry, never imported by name
func init() {
	registerContainer()
}

// registerContainer registers the command proxy and the plugin. The os.Exit
// on a registration failure is confined to this registration file.
func registerContainer() {
	registerContainerDiagnosticCodes()

	// Each YANG command node binds a wire method, and its proxy forwards it
	// to this plugin's process, internal or forked.
	pluginserver.RegisterRPCs(append([]pluginserver.RPCRegistration{
		{WireMethod: wireRequestRestart, Handler: forwardRestart, PluginCommand: cmdRequestRestart},
		listCommandRPC(),
	}, imageCommandRPCs()...)...)

	reg := registry.Registration{
		Name:        pluginName,
		Description: "OCI containers run by ze: confined, limited, supervised and removed without a trace",
		Features:    "yang",
		YANG:        containeryang.ZeContainerConfYANG,
		// The plugin owns the container root, so a config that holds it loads
		// the plugin and a config without it does not.
		ConfigRoots: []string{configRoot},
		// The sysctl plugin writes the kernel tunable a container's network
		// namespace needs (tunables.go), so it is running before a start.
		Dependencies: []string{"sysctl"},
		// The network package asks the interface and firewall components for
		// bridges, links and tables when the plugin starts, so the plugin
		// starts after them when they are loaded. StartAfter loads neither: a
		// config with no container network needs neither component.
		StartAfter: []string{"interface", "firewall"},
		RunEngine:  runContainerEngine,
		Commands:   commandDecls(),
		// `ze config validate` never calls a live verify callback. The static
		// verifier is the same rule set the reload's verify and the boot
		// configure run, so what validate accepts is what starts.
		InProcessConfigVerifier: verifyContainerSections,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		var tb textbuf.Buffer
		tb.Str("container: registration failed: ").Str(err.Error()).Byte('\n').StdErr() //nolint:errcheck // the process exits next; stderr is the only channel left
		os.Exit(1)
	}
}

// registerContainerDiagnosticCodes publishes the doctor codes this plugin owns
// (doctor.go for the runtime, doctor_store.go for the image store), so `ze
// show diagnostic <code>` explains them, then declares the store's event
// codes, which need their explanations first. They register from here and
// never from the always-on code table: a build without the plugin then holds
// none of them.
func registerContainerDiagnosticCodes() {
	for _, table := range [][]diagnostic.CodeMeta{containerDiagnosticCodes, storeDiagnosticCodes} {
		for _, meta := range table {
			if err := diagnostic.Register(meta); err != nil {
				var tb textbuf.Buffer
				tb.Str("container: diagnostic code ").Str(meta.Code).Str(" registration failed: ").Str(err.Error()).Byte('\n').StdErr() //nolint:errcheck // the process exits next; stderr is the only channel left
				os.Exit(1)
			}
		}
	}
	if err := registerStoreEventCodes(); err != nil {
		var tb textbuf.Buffer
		tb.Str("container: event code registration failed: ").Str(err.Error()).Byte('\n').StdErr() //nolint:errcheck // the process exits next; stderr is the only channel left
		os.Exit(1)
	}
}

// verifyContainerSections is the side-effect-free verifier the engine calls
// at `ze config validate` and at commit. It is parseContainerSections with
// its config dropped: one rule set, one verdict.
func verifyContainerSections(sections []sdk.ConfigSection) error {
	_, _, err := parseContainerSections(sections)
	return err
}

// forwardRestart proxies the restart command to this plugin's process.
//
// It MUST forward, never dispatch: dispatching the same command string would
// route it straight back to this handler and recurse.
func forwardRestart(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	dispatcher := ctx.Dispatcher()
	if dispatcher == nil {
		return plugin.OperatorErrorResponse(opererr.New(
			"request container restart was not run: the daemon's command dispatcher is not available",
			"the command is answered by the container plugin's process, and nothing can forward it there while the daemon starts or stops",
			"ze show doctor",
		)), nil
	}
	return dispatcher.ForwardToPlugin(ctx, cmdRequestRestart, validated.Tokens(), ctx.PeerSelector())
}
