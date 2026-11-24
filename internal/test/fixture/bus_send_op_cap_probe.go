// Design: docs/architecture/forked-route-install.md -- plugin route-install and route-remove operation capability gating through the bus send filter.
// Overview: register_bus_send_op_cap_probe.go -- the observer registered from this probe.
// Related: fixture.go -- Observe, ReportFailure.

package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	busSendOpCapPlugin  = "bus-send-op-cap-probe"
	busSendOpCapPrefix  = "198.51.100.0/24"
	busSendOpCapNextHop = "192.0.2.1"
	busSendOpCapMetric  = 10
)

// busSendOpCapProbe connects as an external plugin and invokes RouteInstall
// followed by RouteRemove with a registered protocol name ("static") against a
// live engine Loc-RIB / sysrib without declaring system-rib:route-install. It
// fails when either RPC succeeds (as today's unchecked opRouteInstall and
// opRouteRemove handlers allow) or when either RPC fails for an unrelated
// reason (such as unknown protocol, missing handler, or malformed input).
func busSendOpCapProbe(ctx context.Context, p *sdk.Plugin) error {
	installEntry := rpc.RouteInstallEntry{
		Protocol: protocolStatic,
		AFI:      uint16(family.AFIIPv4),
		SAFI:     uint8(family.SAFIUnicast),
		Prefix:   busSendOpCapPrefix,
		NextHop:  busSendOpCapNextHop,
		Metric:   busSendOpCapMetric,
	}
	installed, err := p.RouteInstall(ctx, []rpc.RouteInstallEntry{installEntry})
	if err == nil {
		return fmt.Errorf("undeclared route-install succeeded (installed=%d): opRouteInstall accepted an undeclared system-rib:route-install call with no bus permission check",
			installed)
	}
	if err := checkOpCapRefusal("route-install", err); err != nil {
		return err
	}

	removeEntry := rpc.RouteRemoveEntry{
		Protocol: protocolStatic,
		AFI:      uint16(family.AFIIPv4),
		SAFI:     uint8(family.SAFIUnicast),
		Prefix:   busSendOpCapPrefix,
	}
	removed, err := p.RouteRemove(ctx, []rpc.RouteRemoveEntry{removeEntry})
	if err == nil {
		return fmt.Errorf("undeclared route-remove succeeded (removed=%d): opRouteRemove accepted an undeclared system-rib:route-install call with no bus permission check",
			removed)
	}
	if err := checkOpCapRefusal("route-remove", err); err != nil {
		return err
	}

	_, err = fmt.Fprintln(os.Stderr, "OK: undeclared route-install and route-remove were refused")
	return err
}

// checkOpCapRefusal rejects RPC errors caused by wiring, registration, or input
// bugs so the probe cannot pass falsely for a reason other than bus refusal.
func checkOpCapRefusal(operation string, err error) error {
	refusal := err.Error()
	if strings.Contains(refusal, "unknown protocol") {
		return fmt.Errorf("undeclared %s failed as unknown protocol rather than bus refusal: %s",
			operation, refusal)
	}
	if strings.Contains(refusal, "not registered") {
		return fmt.Errorf("undeclared %s failed because protocol or handler was not registered rather than bus refusal: %s",
			operation, refusal)
	}
	if strings.Contains(refusal, "unknown method") {
		return fmt.Errorf("undeclared %s failed as unknown method rather than bus refusal: %s",
			operation, refusal)
	}
	if strings.Contains(refusal, "engine has no Loc-RIB") {
		return fmt.Errorf("undeclared %s failed because engine has no Loc-RIB rather than bus refusal: %s",
			operation, refusal)
	}
	if strings.Contains(refusal, "bad prefix") {
		return fmt.Errorf("undeclared %s failed on prefix parse rather than bus refusal: %s",
			operation, refusal)
	}
	if strings.Contains(refusal, "bad next-hop") {
		return fmt.Errorf("undeclared %s failed on next-hop parse rather than bus refusal: %s",
			operation, refusal)
	}
	return nil
}
