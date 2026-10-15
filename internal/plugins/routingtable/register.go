// Design: docs/architecture/static-routes.md -- routing-table plugin registration

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
	_, _, err := parseRoutingTableSections(sections)
	return err
}

// parseRoutingTableSections reads the routing-table section of one delivery.
// present is false when the delivery carries no routing-table section.
func parseRoutingTableSections(sections []sdk.ConfigSection) (map[string]uint32, bool, error) {
	for _, section := range sections {
		if section.Root != pluginName {
			continue
		}
		tables, err := ParseConfig(section.Data)
		if err != nil {
			return nil, true, err
		}
		return tables, true, nil
	}
	return nil, false, nil
}

// newRoutingTableConfigTx is the plugin's side of the config transaction: the
// name-to-ID map is its config, and an install publishes it whole as the
// registry, so a reader resolves against the old tables or the new ones and
// never a mix. Before any install no named table exists.
func newRoutingTableConfigTx() *sdk.ConfigTx[map[string]uint32] {
	return sdk.NewConfigTx(map[string]uint32(nil), parseRoutingTableSections, installRoutingTables)
}

// installRoutingTables makes tables the registry every later Resolve reads.
// It cannot fail.
func installRoutingTables(tables map[string]uint32) error {
	SetRegistry(New(tables))
	logger().Info("routing-table registry loaded", "count", len(tables))
	return nil
}

func runRoutingTablePlugin(conn net.Conn) int {
	logger().Debug("routing-table plugin starting")

	p := sdk.NewWithConn(pluginName, conn)
	defer func() { _ = p.Close() }()

	newRoutingTableConfigTx().Register(p)

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
