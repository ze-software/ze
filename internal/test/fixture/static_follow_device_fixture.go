// Design: docs/architecture/static-routes.md -- device presence as an activity input
// Related: static_kernel_fixture.go -- kernelLink, kernelStaticGateway and the kernel reads these reuse
// Related: register_static_follow_device.go -- registers the driver below

package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// The follow-device scenario's names. followDevice is the pppoe-client key the
// config declares; no access concentrator answers, so the dialer never creates
// it, and the scenario stands in for the session with a dummy device of that
// name.
const (
	followDevice      = "pppoe0"
	followAloneRoute  = "198.18.76.0/24"
	followMixedRoute  = "198.18.75.0/24"
	followGatewayHop  = "nexthop via " + kernelStaticGateway + " dev " + kernelLink + " weight 1"
	followDeviceHop   = "nexthop dev " + followDevice + " weight 3"
	followGatewayOnly = "via " + kernelStaticGateway + " dev " + kernelLink
)

// followStep is one change the scenario makes to the device, and the state
// the kernel owes after it.
type followStep struct {
	name string
	ip   [][]string
	up   bool
}

// staticFollowDevice proves AC-3 and AC-14 through the real pipeline (static,
// the Loc-RIB, sysrib, fib-kernel): a main-table route through the declared
// pppoe-client alone, and a mixed group with the gateway 192.0.2.1. The device
// is absent at boot, then created and set up, deleted, and recreated; after
// each step the kernel main table is read for both routes at proto 250.
func staticFollowDevice(ctx context.Context, _ *sdk.Plugin) error {
	if err := followWait(ctx, "boot, device absent", false); err != nil {
		return err
	}
	create := [][]string{{ipObjectLink, argAdd, followDevice, argType, ipTypeDummy}, {ipObjectLink, ipWordSet, followDevice, "up"}}
	steps := []followStep{
		{name: "device up", ip: create, up: true},
		{name: "device deleted", ip: [][]string{{ipObjectLink, argDel, followDevice}}, up: false},
		{name: "device recreated", ip: create, up: true},
	}
	for _, step := range steps {
		for _, args := range step.ip {
			if out, err := runIP(ctx, args...); err != nil {
				return fmt.Errorf("%s: ip %s: %w: %s", step.name, strings.Join(args, " "), err, out)
			}
		}
		if err := followWait(ctx, step.name, step.up); err != nil {
			return err
		}
	}
	return nil
}

// followWait waits for the kernel state one step owes. With the device up, the
// interface route goes out of the device and the mixed route is multipath
// through both members with their weights; with it absent, the interface route
// is gone and the mixed route goes through the gateway alone, never withdrawn.
func followWait(ctx context.Context, step string, up bool) error {
	var alone, mixed string
	ok := Poll(ctx, kernelPolls, 100*time.Millisecond, func() bool {
		alone = strings.Join(strings.Fields(kernelRoute(ctx, "main", followAloneRoute)), " ")
		mixed = strings.Join(strings.Fields(kernelRoute(ctx, "main", followMixedRoute)), " ")
		if !strings.Contains(mixed, "proto 250") {
			return false
		}
		if up {
			return strings.Contains(alone, "dev "+followDevice+" ") && strings.Contains(alone, "proto 250") &&
				strings.Contains(mixed, followGatewayHop) && strings.Contains(mixed, followDeviceHop)
		}
		return alone == "" && strings.Contains(mixed, followGatewayOnly) && !strings.Contains(mixed, "nexthop")
	})
	if !ok {
		return fmt.Errorf("%s: %s is %q and %s is %q", step, followAloneRoute, alone, followMixedRoute, mixed)
	}
	fmt.Fprintln(os.Stderr, "OK: "+step+": "+followAloneRoute+" ["+alone+"] "+followMixedRoute+" ["+mixed+"]")
	return nil
}

// followDefault is the route AC-3 and AC-10 commit through the declared
// pppoe-client while no session is up.
const followDefault = "0.0.0.0/0"

// followShowRow is one route of `show static`, with the fields that tell a
// waiting route (held, its member inactive) from a skipped one.
type followShowRow struct {
	Prefix     string          `json:"prefix"`
	Table      uint32          `json:"table"`
	Skipped    bool            `json:"skipped"`
	SkipReason string          `json:"skip-reason"`
	NextHops   []followShowHop `json:"next-hops"`
}

type followShowHop struct {
	Interface string `json:"interface"`
	Active    bool   `json:"active"`
}

// followShowDefault answers the main-table followDefault row of `show static`,
// and whether it is there at all.
func followShowDefault(ctx context.Context, plugin *sdk.Plugin) (followShowRow, bool, error) {
	var rows []followShowRow
	status, err := Dispatch(ctx, plugin, "show static", &rows)
	if err != nil {
		return followShowRow{}, false, fmt.Errorf("show static: %w", err)
	}
	if status != statusDone {
		return followShowRow{}, false, fmt.Errorf("show static: status %s", status)
	}
	for _, row := range rows {
		if row.Prefix == followDefault && row.Table == 0 {
			return row, true, nil
		}
	}
	return followShowRow{}, false, nil
}

// followWaitingRoute waits for `show static` to hold the followDefault route,
// then requires it waiting: not in the skipped set, and its followDevice
// member present and inactive. A skip is decided at apply, so once the route
// is listed its classification is final for this config.
func followWaitingRoute(ctx context.Context, plugin *sdk.Plugin, step string) error {
	var row followShowRow
	var lookErr error
	if !Poll(ctx, kernelPolls, 100*time.Millisecond, func() bool {
		var listed bool
		row, listed, lookErr = followShowDefault(ctx, plugin)
		return lookErr == nil && listed
	}) {
		if lookErr != nil {
			return fmt.Errorf("%s: %w", step, lookErr)
		}
		return fmt.Errorf("%s: show static never listed %s in the main table", step, followDefault)
	}
	if row.Skipped {
		return fmt.Errorf("%s: %s is skipped (%s), want it waiting for %s", step, followDefault, row.SkipReason, followDevice)
	}
	if len(row.NextHops) != 1 {
		return fmt.Errorf("%s: %s lists next-hops %+v, want the one member through %s", step, followDefault, row.NextHops, followDevice)
	}
	member := row.NextHops[0]
	if member.Interface != followDevice {
		return fmt.Errorf("%s: %s member is %+v, want interface %s", step, followDefault, member, followDevice)
	}
	if member.Active {
		return fmt.Errorf("%s: %s member through %s is active, with no session up", step, followDefault, followDevice)
	}
	fmt.Fprintln(os.Stderr, "OK: "+step+": "+followDefault+" waits for "+followDevice+": member inactive, route not skipped")
	return nil
}

// staticPPPoEAbsent proves AC-3 at commit: the boot config declares the
// pppoe-client and routes the default through it, and no access concentrator
// answers, so the device never exists.
func staticPPPoEAbsent(ctx context.Context, plugin *sdk.Plugin) error {
	return followWaitingRoute(ctx, plugin, "boot, no session")
}

// The two rewrites staticPPPoESameCommit makes to the boot config, in one
// reload: the client under the interface section, and the default route
// through it at the head of the main table.
const (
	followBootInterface = "\tbackend netlink\n"
	followAddClient     = followBootInterface +
		"\tpppoe-client " + followDevice + " {\n" +
		"\t\tsource-interface " + kernelLink + "\n" +
		"\t\tauthentication {\n\t\t\tusername u\n\t\t\tpassword p\n\t\t}\n\t}\n"
	followBootTable = "\ttable default {\n"
	followAddRoute  = followBootTable +
		"\t\troute " + followDefault + " {\n\t\t\tnext { interface " + followDevice + " { } }\n\t\t}\n"
)

// staticPPPoESameCommit proves AC-10: the daemon boots without the client and
// without the route, then one reload adds both. The reload must be accepted
// and the route must wait, whichever of iface and static applies first. A
// refused reload keeps the boot config, which holds no such route, so the wait
// cannot pass on a failed reload.
func staticPPPoESameCommit(ctx context.Context, plugin *sdk.Plugin) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	_, listed, err := followShowDefault(ctx, plugin)
	if err != nil {
		return fmt.Errorf("before the reload: %w", err)
	}
	if listed {
		return fmt.Errorf("before the reload: show static already lists %s, which only the reload adds", followDefault)
	}
	config, err := os.ReadFile("ze-bgp.conf")
	if err != nil {
		return fmt.Errorf("reading the daemon config to rewrite it: %w", err)
	}
	rewritten := strings.Replace(string(config), followBootInterface, followAddClient, 1)
	if rewritten == string(config) {
		return fmt.Errorf("the daemon config holds no %q to add the client after: %q", followBootInterface, config)
	}
	withRoute := strings.Replace(rewritten, followBootTable, followAddRoute, 1)
	if withRoute == rewritten {
		return fmt.Errorf("the daemon config holds no %q to add the route to: %q", followBootTable, config)
	}
	if err := os.WriteFile("ze-bgp.conf", []byte(withRoute), 0o600); err != nil {
		return err
	}
	if err := signalProcess(pid, syscall.SIGHUP); err != nil {
		return err
	}
	return followWaitingRoute(ctx, plugin, "after one reload adding client and route")
}
