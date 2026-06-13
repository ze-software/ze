package cos

import (
	"fmt"
	"net"
	"os"
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	coreCos "codeberg.org/thomas-mangin/ze/internal/core/cos"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/show"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/plugins/cos/yang"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

var dynamicHandler *cosHandler

func init() {
	coreCos.RegisterResolver(resolveCoSForUnit)

	show.MustRegister("show subscriber detail", "cos", show.Enricher{
		Detail: enrichSubscriberDetail,
	})
	show.MustRegister("show subscriber", "cos", show.Enricher{
		Brief: enrichSubscriberBrief,
	})

	reg := registry.Registration{
		Name:                    Name,
		Description:             "802.1p class-of-service profile definitions",
		Features:                "yang",
		YANG:                    yang.ZeCosConfYANG,
		ConfigRoots:             []string{"class-of-service"},
		InProcessConfigVerifier: verifyCoSConfig,
		RunEngine:               runPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg any) {
			if r, ok := reg.(metrics.Registry); ok {
				BindMetrics(r)
			}
		},
		ConfigureEventBus: func(eb any) {
			if e, ok := eb.(ze.EventBus); ok {
				updateFn := func(ifaceName string, ingress, egress map[uint32]uint32) error {
					b := iface.GetBackend()
					if b == nil {
						return fmt.Errorf("cos: no iface backend loaded")
					}
					return b.UpdateVLANQoSMap(ifaceName, ingress, egress)
				}
				dynamicHandler = newCosHandler(e, updateFn, nil)
			}
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
		if sec.Root != "class-of-service" {
			continue
		}
		if err := parseAndRegisterProfiles(sec.Data); err != nil {
			return err
		}
	}
	return nil
}

func runPlugin(conn net.Conn) int {
	logger().Debug("cos plugin starting")

	p := sdk.NewWithConn(Name, conn)
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
			if sec.Root != "class-of-service" {
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
		WantsConfig:  []string{"class-of-service"},
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
