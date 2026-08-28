package cos

import (
	"fmt"
	"net"
	"os"
	"sort"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	coreCos "github.com/ze-software/ze/internal/core/cos"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/plugins/cos/yang"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

var dynamicHandler *cosHandler

const configRootClassOfService = "class-of-service"

func init() {
	coreCos.RegisterResolver(resolveCoSForUnit)

	reg := registry.Registration{
		Name:                    Name,
		Description:             "802.1p class-of-service profile definitions",
		Features:                "yang",
		YANG:                    yang.ZeCosConfYANG,
		ConfigRoots:             []string{configRootClassOfService},
		InProcessConfigVerifier: verifyCoSConfig,
		RunEngine:               runPlugin,
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
				return b.UpdateVLANQoSMap(ifaceName, ingress, egress)
			}
			dynamicHandler = newCosHandler(eb, updateFn, nil)
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
		fmt.Fprintf(os.Stderr, "%s: registration failed: %v\n", Name, err)
		os.Exit(1)
	}
}

func showProfiles() any {
	all := coreCos.All()
	names := make([]string, 0, len(all))
	for name := range all {
		names = append(names, name)
	}
	sort.Strings(names)

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

func verifyCoSConfig(sections []sdk.ConfigSection) error {
	coreCos.Clear()
	for _, sec := range sections {
		if sec.Root != configRootClassOfService {
			continue
		}
		if err := parseAndRegisterProfiles(sec.Data); err != nil {
			return err
		}
	}
	return nil
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
// external: static profile config (OnConfigure/parseAndRegisterProfiles)
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
			dynamicHandler = nil
		}
		if err := p.Close(); err != nil {
			logger().Warn("cos: close failed", "error", err)
		}
	}()

	p.OnConfigVerify(verifyCoSConfig)

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, sec := range sections {
			if sec.Root != configRootClassOfService {
				continue
			}
			if err := parseAndRegisterProfiles(sec.Data); err != nil {
				return err
			}
		}
		return nil
	})

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, any, error) {
		if command == "show class-of-service" {
			return "done", showProfiles(), nil
		}
		return "error", "", fmt.Errorf("unknown command: %s", command)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRootClassOfService},
		VerifyBudget: 1,
		Commands: []sdk.CommandDecl{
			{Name: "show class-of-service"},
		},
	}); err != nil {
		logger().Error("cos plugin failed", "error", err)
		return 1
	}
	return 0
}
