// Design: docs/functional-tests.md -- RS native departure and successor replay.
// Related: bgp_best_route_transit.go -- original socket setup and exact UPDATE checks.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The same complete scenario runs with the actual bgp-rs registration internal
// and forked. It never imports RS or reactor, injects a lease, or substitutes a
// mock writer. Returning an error reaches fixture.Run's failure sentinel.
func bgpBestRouteRSLifetimeDriver(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("route-server lifetime requires <internal|forked> <BGP listen address>")
	}
	if args[0] != sysribPlacementInternal {
		if args[0] != sysribPlacementForked {
			return fmt.Errorf("route-server lifetime has unknown placement %q", args[0])
		}
	}
	return Observe(ctx, "bgp-rs-lifetime-test", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := bgpBestRouteRSLifetime(ctx, plugin, args[1]); err != nil {
			return fmt.Errorf("route-server %s lifetime: %w", args[0], err)
		}
		_, err := fmt.Fprintf(os.Stderr, "OK: route-server %s native departure and successor replay\n", args[0])
		return err
	})
}

// Does a withdrawn B stay absent when the destination reconnects, while A and
// a later C remain useful, and does the source's real DOWN retire A+C on that
// same successor socket without removing local API output? Units cannot prove
// the authenticated role, receive store and final TCP writer form this path.
func bgpBestRouteRSLifetime(ctx context.Context, plugin *sdk.Plugin, address string) error {
	source, originalTarget, err := bgpBestRouteConnect(ctx, plugin, address, 65001, 65002)
	if err != nil {
		return err
	}
	// The scenario MUST keep original sockets through every assertion that
	// uses them; these defers MUST close them on success and every error path.
	defer source.Close()         //nolint:errcheck // A close cannot replace this scenario's assertion error.
	defer originalTarget.Close() //nolint:errcheck // The scenario explicitly retires this original UP below.
	a, b, c := []byte{24, 10, 250, 1}, []byte{24, 10, 250, 2}, []byte{24, 10, 250, 3}
	sourcePath := []byte{2, 1, 0xfd, 0xe9}
	for _, prefix := range [][]byte{a, b} {
		if err := bgpBestRouteSend(source, prefix, false, false); err != nil {
			return err
		}
		if err := bgpBestRouteExpect(originalTarget, prefix, nil, sourcePath); err != nil {
			return fmt.Errorf("useful original RS output: %w", err)
		}
	}
	if err := bgpBestRouteSend(source, b, false, true); err != nil {
		return err
	}
	if err := bgpBestRouteExpect(originalTarget, nil, b, nil); err != nil {
		return fmt.Errorf("ordinary B withdrawal: %w", err)
	}
	if err := quiesce(ctx, plugin); err != nil {
		return err
	}
	if err := originalTarget.Close(); err != nil {
		return err
	}
	if !Poll(ctx, liveStagesPollAttempts, liveStagesPollDelay, func() bool {
		rows, err := peerRows03(ctx, plugin, "*")
		if err != nil {
			return false
		}
		return number03(rows[addrLoopbackThird]["connections-dropped"]) == 1
	}) {
		return errors.New("original destination never completed its transport DOWN")
	}
	successor, err := atomicAggregatePolicyConnect(ctx, address, netip.MustParseAddr(addrLoopbackThird), 65002)
	if err != nil {
		return err
	}
	defer successor.Close() //nolint:errcheck // The original successor socket remains owned until all final assertions finish.
	if err := bgpBestRouteExpect(successor, a, nil, sourcePath); err != nil {
		return fmt.Errorf("successor replay lost current A or resurrected withdrawn B: %w", err)
	}
	if err := bgpBestRouteExpect(successor, nil, nil, nil); err != nil {
		return fmt.Errorf("successor replay added stale B or omitted its End-of-RIB: %w", err)
	}
	if err := bgpBestRouteSend(source, c, false, false); err != nil {
		return err
	}
	if err := bgpBestRouteExpect(successor, c, nil, sourcePath); err != nil {
		return fmt.Errorf("useful C after destination replacement: %w", err)
	}
	if err := source.Close(); err != nil {
		return err
	}
	if err := bgpBestRouteRSWithdrawals(successor, [][]byte{a, c}); err != nil {
		return fmt.Errorf("native source DOWN: %w", err)
	}
	if err := quiesce(ctx, plugin); err != nil {
		return err
	}
	if err := bgpBestRouteAPI(ctx, plugin, successor, "10.253.9.0/24", []byte{24, 10, 253, 9}, nil); err != nil {
		return fmt.Errorf("same-successor API survivor or late obsolete role output: %w", err)
	}
	rows, err := peerRows03(ctx, plugin, "*")
	if err != nil {
		return err
	}
	if number03(rows[addrLoopbackSecond]["connections-established"]) != 1 {
		return errors.New("source changed physical session during the role lifecycle")
	}
	if number03(rows[addrLoopbackThird]["connections-established"]) != 2 {
		return errors.New("destination did not use exactly its original and successor sessions")
	}
	return nil
}

// Withdrawals may share one UPDATE or arrive separately; packet grouping is
// immaterial. Every expected native NLRI must appear exactly once, with no
// announcement, attribute, stale B, unrelated prefix or empty fake success.
func bgpBestRouteRSWithdrawals(socket net.Conn, expected [][]byte) error {
	remaining := make(map[string]bool, len(expected))
	for _, prefix := range expected {
		remaining[string(prefix)] = true
	}
	for range len(expected) {
		update, err := atomicAggregatePolicyRead(socket)
		if err != nil {
			return err
		}
		if len(update.NLRI) != 0 {
			return fmt.Errorf("source DOWN announced obsolete output %x", update.NLRI)
		}
		if len(update.PathAttributes) != 0 {
			return fmt.Errorf("native departure withdrawal carried attributes %x", update.PathAttributes)
		}
		if len(update.WithdrawnRoutes) == 0 {
			return errors.New("native departure produced no withdrawal")
		}
		if len(update.WithdrawnRoutes)%4 != 0 {
			return fmt.Errorf("unexpected native departure framing %x", update.WithdrawnRoutes)
		}
		for offset := 0; offset < len(update.WithdrawnRoutes); offset += 4 {
			prefix := update.WithdrawnRoutes[offset : offset+4]
			if prefix[0] != 24 {
				return fmt.Errorf("unexpected native departure length %x", prefix)
			}
			if !remaining[string(prefix)] {
				return fmt.Errorf("unexpected or duplicate native departure %x", prefix)
			}
			delete(remaining, string(prefix))
		}
		if len(remaining) == 0 {
			return nil
		}
	}
	return fmt.Errorf("native departure left %d useful rows installed", len(remaining))
}
