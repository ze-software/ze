// Design: docs/architecture/forked-route-install.md -- plugin engine-operation capability gating through the bus send filter.
// Overview: register_bus_send_op_cap_probe.go -- the driver registered from this probe.
// Related: bus_send_fixtures.go -- busRunPlugin, busAssertUndeclaredRefusal.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	busSendOpCapPlugin  = "bus-send-op-cap-probe"
	busSendOpCapPrefix  = "198.51.100.0/24"
	busSendOpCapNextHop = "192.0.2.1"
	busSendOpCapMetric  = 10
	busSendOpCapPeerKey = "127.0.0.2"
)

// busSendOpCapUpdateBody is a valid RFC 4271 Section 4.3 IPv4 unicast UPDATE
// payload (without the 19-byte BGP header): withdrawn=0, path attributes=20B
// (ORIGIN IGP, 4-byte AS_PATH [65001], NEXT_HOP 192.0.2.1), NLRI 203.0.113.0/24.
var busSendOpCapUpdateBody = []byte{
	0x00, 0x00,
	0x00, 0x14,
	0x40, 0x01, 0x01, 0x00,
	0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9,
	0x40, 0x03, 0x04, 0xC0, 0x00, 0x02, 0x01,
	0x18, 0xCB, 0x00, 0x71,
}

// busSendOpCapProbeDriver implements S37 (bus-send-operation-capability.ci).
// Mode "undeclared" (Run 1) declares no sends and asserts route-install,
// route-remove, inject-wire-route, and emit-event acme:alert are all refused.
// Mode "declared" (Run 2) declares system-rib:route-install, bgp:raw, and
// acme:alert and asserts all four succeed, synchronizing between install and
// remove so the caller verifies 198.51.100.0/24 enters and leaves show rib.
// Usage: plugin/bus-send-op-cap-probe [undeclared <done-file> | declared <installed-file> <remove-go-file> <done-file>].
func busSendOpCapProbeDriver(ctx context.Context, args []string) error {
	mode := "undeclared"
	doneFile := "run1.done"
	installedFile := ""
	removeGoFile := ""

	switch len(args) {
	case 0:
		// Default Run 1 mode.
	case 2:
		if args[0] != "undeclared" {
			return fmt.Errorf("bus-send-op-cap-probe: 2-arg form requires mode undeclared, got %q", args[0])
		}
		mode = args[0]
		doneFile = args[1]
	case 4:
		if args[0] != "declared" {
			return fmt.Errorf("bus-send-op-cap-probe: 4-arg form requires mode declared, got %q", args[0])
		}
		mode = args[0]
		installedFile = args[1]
		removeGoFile = args[2]
		doneFile = args[3]
	default:
		return errors.New("usage: plugin/bus-send-op-cap-probe [undeclared <done> | declared <installed> <remove-go> <done>]")
	}

	reg := &sdk.Registration{}
	if mode == "declared" {
		reg.Sends = []sdk.MessageSendDecl{
			{Message: busMsgSysRIBInst},
			{Message: busMsgBGPRaw},
			{Message: busMsgAcmeAlert},
		}
	}

	return busRunPlugin(ctx, busSendOpCapPlugin, reg, nil, func(ctx context.Context, p *sdk.Plugin) error {
		if mode == "undeclared" {
			return busSendOpCapRunUndeclared(ctx, p, doneFile)
		}
		return busSendOpCapRunDeclared(ctx, p, installedFile, removeGoFile, doneFile)
	})
}

func busSendOpCapInstallEntry() rpc.RouteInstallEntry {
	return rpc.RouteInstallEntry{
		Protocol: protocolStatic,
		AFI:      uint16(family.AFIIPv4),
		SAFI:     uint8(family.SAFIUnicast),
		Prefix:   busSendOpCapPrefix,
		NextHop:  busSendOpCapNextHop,
		Metric:   busSendOpCapMetric,
	}
}

func busSendOpCapRemoveEntry() rpc.RouteRemoveEntry {
	return rpc.RouteRemoveEntry{
		Protocol: protocolStatic,
		AFI:      uint16(family.AFIIPv4),
		SAFI:     uint8(family.SAFIUnicast),
		Prefix:   busSendOpCapPrefix,
	}
}

func busSendOpCapRunUndeclared(ctx context.Context, p *sdk.Plugin, doneFile string) error {
	epPath := busEndpointPath(busCurrentEndpointName(busSendOpCapPlugin))

	_, err := p.RouteInstall(ctx, []rpc.RouteInstallEntry{busSendOpCapInstallEntry()})
	if err := busAssertUndeclaredRefusal("route-install", err, epPath, busMsgSysRIBInst); err != nil {
		return err
	}

	_, err = p.RouteRemove(ctx, []rpc.RouteRemoveEntry{busSendOpCapRemoveEntry()})
	if err := busAssertUndeclaredRefusal("route-remove", err, epPath, busMsgSysRIBInst); err != nil {
		return err
	}

	err = p.InjectWireRoute("bmp", busSendOpCapPeerKey, busSendOpCapUpdateBody)
	if err := busAssertUndeclaredRefusal("inject-wire-route", err, epPath, busMsgBGPRaw); err != nil {
		return err
	}

	_, err = p.EmitEvent(ctx, "acme", "alert", "", "", `{"detail":"s37-unauth"}`)
	if err := busAssertUndeclaredRefusal("emit-event acme:alert", err, epPath, busMsgAcmeAlert); err != nil {
		return err
	}

	if err := busWriteAtomicFile(doneFile, "run1-ok\n"); err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stderr, "OK: undeclared route-install, route-remove, inject-wire-route, and emit-event were refused")
	return err
}

func busSendOpCapRunDeclared(ctx context.Context, p *sdk.Plugin, installedFile, removeGoFile, doneFile string) error {
	installed, err := p.RouteInstall(ctx, []rpc.RouteInstallEntry{busSendOpCapInstallEntry()})
	if err != nil {
		return fmt.Errorf("declared route-install failed: %w", err)
	}
	if installed != 1 {
		return fmt.Errorf("declared route-install returned installed=%d, want 1", installed)
	}
	if err := busWriteAtomicFile(installedFile, "installed\n"); err != nil {
		return err
	}
	if err := busPollFile(ctx, removeGoFile); err != nil {
		return fmt.Errorf("wait for remove-go barrier: %w", err)
	}

	removed, err := p.RouteRemove(ctx, []rpc.RouteRemoveEntry{busSendOpCapRemoveEntry()})
	if err != nil {
		return fmt.Errorf("declared route-remove failed: %w", err)
	}
	if removed != 1 {
		return fmt.Errorf("declared route-remove returned removed=%d, want 1", removed)
	}

	if err := p.InjectWireRoute("bmp", busSendOpCapPeerKey, busSendOpCapUpdateBody); err != nil {
		return fmt.Errorf("declared inject-wire-route failed: %w", err)
	}

	delivered, err := p.EmitEvent(ctx, "acme", "alert", "", "", `{"detail":"s37-alert"}`)
	if err != nil {
		return fmt.Errorf("declared emit-event acme:alert failed: %w", err)
	}
	if delivered < 1 {
		return fmt.Errorf("declared emit-event acme:alert delivered=%d, want >= 1", delivered)
	}

	if err := busWriteAtomicFile(doneFile, "run2-ok\n"); err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stderr, "OK: declared route-install, route-remove, inject-wire-route, and emit-event succeeded")
	return err
}
