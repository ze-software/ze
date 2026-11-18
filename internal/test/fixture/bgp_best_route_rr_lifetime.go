// Design: docs/functional-tests.md -- actual reflector lifecycle on daemon sockets.
// Related: bgp_best_route_transit.go -- the shared real BGP socket helpers.
package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// bgpBestRouteRRLifetimeDriver runs only after the real plugin startup tiers.
// Each socket is owned until its last assertion and closed on every exit.
func bgpBestRouteRRLifetimeDriver(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("reflector lifetime requires <internal|forked> <BGP listen address>")
	}
	if args[0] != sysribPlacementInternal {
		if args[0] != sysribPlacementForked {
			return fmt.Errorf("unknown reflector placement %q", args[0])
		}
	}
	return Observe(ctx, "bgp-rr-lifetime-test", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := bgpBestRouteRRLifetime(ctx, plugin, args[1]); err != nil {
			return err
		}
		_, err := fmt.Fprintf(os.Stderr, "OK: actual bgp-rr %s replay, original departure and useful successors reached TCP\n", args[0])
		return err
	})
}

// bgpBestRouteRRLifetime asks whether an actual RR keeps only current A when
// its destination reconnects after B's withdrawal, then withdraws A on the
// original source's DOWN and carries useful D from its physical successor.
// Every absence is decided by the next useful same-socket message, not time.
func bgpBestRouteRRLifetime(ctx context.Context, plugin *sdk.Plugin, address string) error {
	source, target, err := bgpBestRouteConnect(ctx, plugin, address, 65000, 65000)
	if err != nil {
		return err
	}
	defer source.Close() //nolint:errcheck // Every scenario outcome owns its useful result.
	defer target.Close() //nolint:errcheck // The original target is also closed before reconnect.
	if err := bgpBestRouteAPI(ctx, plugin, target, "10.253.0.0/24", []byte{24, 10, 253, 0}, nil); err != nil {
		return err
	}
	a, b := []byte{24, 10, 254, 0}, []byte{24, 10, 254, 1}
	for _, prefix := range [][]byte{a, b} {
		if err := bgpBestRouteSend(source, prefix, true, false); err != nil {
			return err
		}
		if err := bgpBestRouteRRExpect(target, prefix, nil); err != nil {
			return fmt.Errorf("original reflected announcement: %w", err)
		}
	}
	if err := bgpBestRouteSend(source, b, true, true); err != nil {
		return err
	}
	if err := bgpBestRouteRRExpect(target, nil, b); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, plugin, "10.254.1.0/24", ""); err != nil {
		return err
	}
	if err := target.Close(); err != nil {
		return err
	}
	if err := policyFinalSessions(ctx, plugin, 1); err != nil {
		return err
	}
	successor, err := atomicAggregatePolicyConnect(ctx, address, netip.MustParseAddr(addrLoopbackThird), 65000)
	if err != nil {
		return err
	}
	defer successor.Close() //nolint:errcheck // The new original target is owned through the final fence.
	if err := bgpBestRouteRRExpect(successor, a, nil); err != nil {
		return fmt.Errorf("current A replay before EOR: %w", err)
	}
	if err := bgpBestRouteExpect(successor, nil, nil, nil); err != nil {
		return fmt.Errorf("replay EOR: %w", err)
	}
	if err := p12WaitPeersEOR(ctx, plugin, "destination"); err != nil {
		return err
	}
	if err := source.Close(); err != nil {
		return err
	}
	if err := bgpBestRouteRRExpect(successor, nil, a); err != nil {
		return fmt.Errorf("original source DOWN: %w", err)
	}
	if err := policyFinalSessions(ctx, plugin, 1); err != nil {
		return err
	}
	fresh, err := atomicAggregatePolicyConnect(ctx, address, netip.MustParseAddr(addrLoopbackSecond), 65000)
	if err != nil {
		return err
	}
	defer fresh.Close() //nolint:errcheck // The new original source stays alive through useful output.
	if err := bgpBestRouteExpect(fresh, nil, nil, nil); err != nil {
		return fmt.Errorf("source successor initial EOR: %w", err)
	}
	d := []byte{24, 10, 254, 3}
	if err := bgpBestRouteSend(fresh, d, true, false); err != nil {
		return err
	}
	if err := bgpBestRouteRRExpect(successor, d, nil); err != nil {
		return fmt.Errorf("useful source/target successors: %w", err)
	}
	if err := bgpBestRouteSend(fresh, d, true, true); err != nil {
		return err
	}
	if err := bgpBestRouteRRExpect(successor, nil, d); err != nil {
		return err
	}
	if err := quiesce(ctx, plugin); err != nil {
		return err
	}
	if err := bgpBestRouteAPI(ctx, plugin, successor, "10.253.1.0/24", []byte{24, 10, 253, 1}, nil); err != nil {
		return fmt.Errorf("API survivor after role lifecycle: %w", err)
	}
	return policyFinalSessions(ctx, plugin, 2)
}

// bgpBestRouteRRExpect reads the actual daemon's next TCP UPDATE. Useful
// announcements must carry reflection attributes and an unchanged empty iBGP
// AS_PATH; a withdrawal has exactly no attributes. Source OPEN router-id is
// fixed by atomicAggregatePolicyConnect to 127.0.0.2; the .ci fixes cluster-id.
func bgpBestRouteRRExpect(conn net.Conn, announced, withdrawn []byte) error {
	update, err := atomicAggregatePolicyRead(conn)
	if err != nil {
		return err
	}
	if !bytes.Equal(update.NLRI, announced) {
		return fmt.Errorf("reflector NLRI=%x, want %x", update.NLRI, announced)
	}
	if !bytes.Equal(update.WithdrawnRoutes, withdrawn) {
		return fmt.Errorf("reflector withdrawn=%x, want %x", update.WithdrawnRoutes, withdrawn)
	}
	if len(announced) == 0 {
		if len(update.PathAttributes) != 0 {
			return fmt.Errorf("reflector withdrawal gained attributes %x", update.PathAttributes)
		}
		return nil
	}
	for code, want := range map[attribute.AttributeCode][]byte{
		attribute.AttrOrigin: {0}, attribute.AttrASPath: {}, attribute.AttrOriginatorID: {127, 0, 0, 2}, attribute.AttrClusterList: {10, 0, 0, 1},
	} {
		_, _, value, present := attribute.AttrFind(update.PathAttributes, code)
		if !present {
			return fmt.Errorf("actual reflector omitted attribute %d", code)
		}
		if !bytes.Equal(value, want) {
			return fmt.Errorf("actual reflector attribute %d=%x, want %x", code, value, want)
		}
	}
	return nil
}
