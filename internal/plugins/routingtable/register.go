// Design: plan/learned/710-gap-2-static-route-enhancements.md -- routing-table plugin registration

package routingtable

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
	rtyang "github.com/ze-software/ze/internal/plugins/routingtable/yang"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const pluginName = "routing-table"

func init() {
	reg := registry.Registration{
		Name:                    pluginName,
		Description:             "Named routing table registry: maps names to kernel table IDs",
		Features:                "yang",
		YANG:                    rtyang.ZeRoutingTableConfYANG,
		ConfigRoots:             []string{pluginName},
		InProcessConfigVerifier: verifyRoutingTableConfig,
		RunEngine:               runRoutingTablePlugin,
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
		fmt.Fprintf(os.Stderr, "routing-table: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func verifyRoutingTableConfig(sections []sdk.ConfigSection) error {
	for _, section := range sections {
		if section.Root != pluginName {
			continue
		}
		if _, err := parseRoutingTableConfig(section.Data); err != nil {
			return err
		}
	}
	return nil
}

func runRoutingTablePlugin(conn net.Conn) int {
	logger().Debug("routing-table plugin starting")

	p := sdk.NewWithConn(pluginName, conn)
	defer func() { _ = p.Close() }()

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != pluginName {
				continue
			}
			tables, err := parseRoutingTableConfig(section.Data)
			if err != nil {
				return err
			}
			SetRegistry(New(tables))
			logger().Info("routing-table registry loaded", "count", len(tables))
		}
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{pluginName},
	})
	if err != nil {
		logger().Error("routing-table plugin failed", "error", err)
		return 1
	}

	return 0
}
