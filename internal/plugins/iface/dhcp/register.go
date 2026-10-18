//go:build linux

package ifacedhcp

import (
	"fmt"
	"net"
	"os"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
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
func newDHCPClientFromFactory(ifaceName, unit string, eb ze.EventBus, v4, v6 bool, hostname, clientID string, pdLength int, duid, resolvConfPath string, hasStaticNameServers bool, routeMetric int) (iface.DHCPStopper, error) {
	cfg := dHCPConfig{
		Hostname:             hostname,
		ClientID:             clientID,
		PDLength:             pdLength,
		DUID:                 duid,
		ResolvConfPath:       resolvConfPath,
		HasStaticNameServers: hasStaticNameServers,
		RouteMetric:          routeMetric,
	}
	client, err := newDHCPClient(ifaceName, unit, eb, v4, v6, cfg, clock.RealClock{})
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

	// The DHCP clients run inside the interface plugin's process, created
	// through iface.SetDHCPClientFactory, and register their leases with its
	// address reconcile by a plain call (iface.RegisterLeasedAddresses). An
	// external iface-dhcp process holds neither the clients nor that registry,
	// so it refuses to start rather than run as a process that does nothing.
	if !p.IsInternal() {
		log.Error("iface-dhcp: refusing to start as an external plugin process -- DHCP clients run inside the interface plugin and register their leases with its address registry (iface.RegisterLeasedAddresses), a same-process call; remove the external iface-dhcp plugin block so it runs internal")
		return 1
	}

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, &sdk.Registration{}); err != nil {
		log.Error("iface-dhcp plugin failed", "error", err)
		return 1
	}

	return 0
}
