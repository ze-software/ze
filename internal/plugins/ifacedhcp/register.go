package ifacedhcp

import (
	"context"
	"fmt"
	"net"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)

	// Register the DHCP client factory so the interface plugin can
	// create clients without importing this package directly.
	iface.SetDHCPClientFactory(newDHCPClientFromFactory)

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

// newDHCPClientFromFactory creates and starts a DHCPClient, returning it as
// a dhcpStopper interface. This bridges the iface package's factory callback
// to the ifacedhcp package's concrete type.
func newDHCPClientFromFactory(ifaceName string, unit int, eb ze.EventBus, v4, v6 bool, hostname, clientID string, pdLength int, duid string) (iface.DHCPStopper, error) {
	cfg := DHCPConfig{
		Hostname: hostname,
		ClientID: clientID,
		PDLength: pdLength,
		DUID:     duid,
	}
	client, err := NewDHCPClient(ifaceName, unit, eb, v4, v6, cfg)
	if err != nil {
		return nil, err
	}
	if err := client.Start(); err != nil {
		return nil, fmt.Errorf("iface dhcp: start: %w", err)
	}
	return client, nil
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
