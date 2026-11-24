// Design: docs/architecture/api/architecture.md -- plugin command and operation authorization through the bus send filter.
// Overview: register_bus_send_undeclared_probe.go -- the driver registered from this probe.
// Related: bus_send_fixtures.go -- busRunPlugin, busAssertUndeclaredRefusal.

package fixture

import (
	"context"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	busSendUndeclaredPlugin = "bus-send-undeclared-probe"
	busSendUndeclaredPrefix = "198.51.100.0/24"
	busSendUndeclaredNHop   = "192.0.2.1"
	busSendUndeclaredMetric = 10
)

// busSendUndeclaredProbeDriver implements S20 (bus-send-undeclared-refused.ci).
// It declares ONLY cli:show bgp peer, then dispatches show bgp peer list
// (must succeed), clear bgp rib in * (must be refused as undeclared),
// route-install (must be refused as undeclared), and emit-event of bgp:state
// (must be refused as undeclared). It writes doneMarker once all four checks
// pass.
// Usage: plugin/bus-send-undeclared-probe [<done-marker>].
func busSendUndeclaredProbeDriver(ctx context.Context, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("%s takes at most one argument: <done-marker>", busSendUndeclaredPlugin)
	}
	doneMarker := "probe.done"
	if len(args) == 1 {
		doneMarker = args[0]
	}

	reg := &sdk.Registration{
		Sends: []sdk.MessageSendDecl{{Message: busMsgShowBGPPeer}},
	}
	return busRunPlugin(ctx, busSendUndeclaredPlugin, reg, nil, func(ctx context.Context, p *sdk.Plugin) error {
		epPath := busEndpointPath(busCurrentEndpointName(busSendUndeclaredPlugin))

		if err := busExpectCLISucceeds(ctx, p, busCmdShowBGPPeerList); err != nil {
			return err
		}
		if err := busExpectCLIUndeclared(ctx, p, busCmdClearBGPRibIn, epPath); err != nil {
			return err
		}

		installEntry := rpc.RouteInstallEntry{
			Protocol: protocolStatic,
			AFI:      uint16(family.AFIIPv4),
			SAFI:     uint8(family.SAFIUnicast),
			Prefix:   busSendUndeclaredPrefix,
			NextHop:  busSendUndeclaredNHop,
			Metric:   busSendUndeclaredMetric,
		}
		_, err := p.RouteInstall(ctx, []rpc.RouteInstallEntry{installEntry})
		if err := busAssertUndeclaredRefusal("route-install", err, epPath, busMsgSysRIBInst); err != nil {
			return err
		}

		_, err = p.EmitEvent(ctx, "bgp", "state", "", "", `{"state":"up"}`)
		if err := busAssertUndeclaredRefusal("emit-event bgp:state", err, epPath, busMsgBGPState); err != nil {
			return err
		}

		if err := busWriteAtomicFile(doneMarker, "ok\n"); err != nil {
			return err
		}
		_, err = fmt.Fprintln(os.Stderr, "OK: show bgp peer list succeeded and 3 undeclared sends were refused")
		return err
	})
}
