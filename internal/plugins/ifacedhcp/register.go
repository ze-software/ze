package ifacedhcp

import (
	"context"
	"fmt"
	"net"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)

	reg := registry.Registration{
		Name:         "iface-dhcp",
		Description:  "DHCP client: DHCPv4/DHCPv6 lease acquisition and renewal",
		Dependencies: []string{"interface"},
		RunEngine:    runDHCPPlugin,
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	reg.ConfigureEngineLogger = func(loggerName string) {
		l := slogutil.Logger(loggerName)
		if l != nil {
			loggerPtr.Store(l)
		}
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "iface-dhcp: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// runDHCPPlugin is the engine-mode entry point for the DHCP plugin.
func runDHCPPlugin(conn net.Conn) int {
	log := loggerPtr.Load()
	log.Debug("iface-dhcp plugin starting")

	p := sdk.NewWithConn("iface-dhcp", conn)
	defer func() { _ = p.Close() }()

	ctx := context.Background()
	if err := p.Run(ctx, sdk.Registration{}); err != nil {
		log.Error("iface-dhcp plugin failed", "error", err)
		return 1
	}

	return 0
}
