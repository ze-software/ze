package cos

import (
	"fmt"
	"net"
	"slices"
	"sort"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	coreCos "github.com/ze-software/ze/internal/core/cos"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/plugins/cos/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

var dynamicHandler *cosHandler

const configRootClassOfService = "class-of-service"

func init() {
	coreCos.RegisterResolver(resolveCoSForUnit)
	coreCos.RegisterProfileParser(parseProfiles)

	reg := registry.Registration{
		Name:        Name,
		Description: "802.1p class-of-service profile definitions",
		Features:    "yang",
		YANG:        yang.ZeCosConfYANG,
		ConfigRoots: []string{configRootClassOfService},
		// StartAfter, never a load: cos listens for the RADIUS CoS change
		// l2tp-auth-radius emits, and applies its static profiles without it
		// (./le plugin coupling check).
		StartAfter:              []string{"l2tp-auth-radius"},
		InProcessConfigVerifier: verifyCoSConfig,
		RunEngine:               runPlugin,
		Commands:                commandDecls(),
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			BindMetrics(reg)
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			updateFn := func(ifaceName string, ingress, egress map[uint32]uint32) error {
				b := iface.GetBackend()
				if b == nil {
					return fmt.Errorf("cos: no iface backend loaded")
				}
				device, err := iface.ResolveDevice(ifaceName)
				if err != nil {
					return err
				}
				return b.UpdateVLANQoSMap(device, ingress, egress)
			}
			var previous *stoppedCosHandler
			if dynamicHandler != nil {
				previous = dynamicHandler.stopOwners()
			}
			dynamicHandler = newCosHandlerFromStopped(eb, updateFn, publishDaemonCoSIssue, previous)
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
		panic("BUG: " + Name + ": registration failed: " + err.Error())
	}
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: wireCleanupCommand, Handler: forwardCleanupCommand, PluginCommand: cleanupCommand},
	)
}

func showProfiles() any {
	all := coreCos.All()
	names := make([]string, 0, len(all))
	for name := range all {
		names = append(names, name)
	}
	slices.Sort(names)

	type mapEntry struct {
		From uint32 `json:"from"`
		To   uint32 `json:"to"`
	}
	type profileView struct {
		Name    string     `json:"name"`
		Ingress []mapEntry `json:"ingress,omitempty"`
		Egress  []mapEntry `json:"egress,omitempty"`
	}

	result := make([]profileView, 0, len(names))
	for _, name := range names {
		p := all[name]
		pv := profileView{Name: name}
		for from, to := range p.IngressMap {
			pv.Ingress = append(pv.Ingress, mapEntry{From: from, To: to})
		}
		for from, to := range p.EgressMap {
			pv.Egress = append(pv.Egress, mapEntry{From: from, To: to})
		}
		sort.Slice(pv.Ingress, func(i, j int) bool { return pv.Ingress[i].From < pv.Ingress[j].From })
		sort.Slice(pv.Egress, func(i, j int) bool { return pv.Egress[i].From < pv.Egress[j].From })
		result = append(result, pv)
	}
	return result
}

// verifyCoSConfig is the in-process verifier: the same parse the config
// transaction's verify runs, and like it, it writes no state.
func verifyCoSConfig(sections []sdk.ConfigSection) error {
	_, _, err := parseCoSSections(sections)
	return err
}

// newCoSConfigTx returns the cos config transaction: verify parses a
// candidate profile set and installs nothing, apply installs it into the
// shared registry in one replace, rollback installs the set it replaced.
func newCoSConfigTx() *sdk.ConfigTx[cosProfiles] {
	return sdk.NewConfigTx(cosProfiles{}, parseCoSSections, installCoSProfiles)
}

// warnIfExternal logs a warning when cos is not running in-process.
// ConfigureEventBus (this file's init(), the only place that sets
// dynamicHandler) is wired exclusively through
// plugin.GetInternalPluginRunner, which is never used for an external
// plugin -- so an external cos never has ConfigureEventBus called at all,
// dynamicHandler stays nil for the plugin's entire lifetime, and every
// EventBus-triggered VLAN QoS map push (updateFn's iface.GetBackend().
// UpdateVLANQoSMap) silently never fires, with no error anywhere.
//
// Unlike as112 (which refuses to start entirely when external -- see
// sdk.Plugin.IsInternal's doc comment -- because its whole purpose depends
// on the same-process call succeeding), cos still provides real value
// external: static profile config (the config transaction, newCoSConfigTx)
// and `show class-of-service` both work unaffected. So this warns rather
// than refuses.
func warnIfExternal(isInternal bool) {
	if isInternal {
		return
	}
	logger().Warn("cos: running as an external plugin process -- dynamic per-interface QoS map updates require in-process wiring (ConfigureEventBus is never invoked for external plugins) and will not apply; configure cos to run internal for dynamic updates. Static profile config and 'show class-of-service' are unaffected.")
}

func runPlugin(conn net.Conn) int {
	logger().Debug("cos plugin starting")

	p := sdk.NewWithConn(Name, conn)
	warnIfExternal(p.IsInternal())
	defer func() {
		if dynamicHandler != nil {
			dynamicHandler.stop()
		}
		if err := p.Close(); err != nil {
			logger().Warn("cos: close failed", "error", err)
		}
	}()

	newCoSConfigTx().Register(p)

	p.OnExecuteCommand(executeCommand)

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, &sdk.Registration{
		WantsConfig:  []string{configRootClassOfService},
		VerifyBudget: 1,
		Commands:     commandDecls(),
	}); err != nil {
		logger().Error("cos plugin failed", "error", err)
		return 1
	}
	return 0
}

// commandDecls names the commands this plugin serves and states what each
// answer holds (pkg/plugin/rpc/types.go, CommandDecl).
//
// It has two readers and MUST stay one function. init() puts it on the
// registry.Registration, which anything linking the composition root reads
// without starting an engine, and the runner sends it in the Stage 1
// registration message, which a running daemon reads.
func commandDecls() []sdk.CommandDecl {
	return slices.Concat([]sdk.CommandDecl{
		{
			Name:      "show class-of-service",
			ShortHelp: "Show each class-of-service profile with its ingress and egress marking maps.",
		},
	}, sessionCommandDecls(), cleanupCommandDecls())
}

// executeCommand admits only commands this plugin declares. The session owner
// route uses the same handler for DirectBridge and socket command callbacks.
func executeCommand(_, command string, args []string, peer string, _ []sdk.PipeOperation) (string, any, error) {
	handlers := map[string]func([]string, string) (string, any, error){
		"show class-of-service": func([]string, string) (string, any, error) {
			return rpc.StatusDone, showProfiles(), nil
		},
	}
	registerSessionCommand(handlers)
	registerCleanupCommand(handlers)
	handler, found := handlers[command]
	if !found {
		return rpc.StatusError, "", opererr.New("unknown class-of-service command: "+command,
			"This plugin serves the class-of-service commands declared by the running binary.",
			"ze show class-of-service")
	}
	return handler(args, peer)
}

const (
	cleanupCommand     = "request class-of-service cleanup"
	wireCleanupCommand = "ze-cos:cleanup"
)

func forwardCleanupCommand(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	if len(validated.Tokens()) > 0 {
		return plugin.OperatorErrorResponse(opererr.New(
			cleanupCommand+" takes no arguments",
			"No extra tokens are accepted by the class-of-service cleanup command.",
			"ze "+cleanupCommand)), nil
	}
	d := ctx.Dispatcher()
	if d == nil {
		return plugin.OperatorErrorResponse(opererr.New(
			"class-of-service cleanup cannot run without an active command dispatcher",
			"The request cannot be forwarded to the running class-of-service plugin.",
			"ze show plugin list")), nil
	}
	return d.ForwardToPlugin(ctx, cleanupCommand, nil, ctx.PeerSelector())
}

type sharedCoSCleanupResult struct {
	Attempted int `json:"attempted"`
	Recovered int `json:"recovered"`
}

func cleanupCommandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{{Name: cleanupCommand, Shape: "doc",
		ShortHelp: "Retry failed class-of-service cleanup using each original access owner."}}
}
