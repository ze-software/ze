// Design: docs/architecture/api/architecture.md -- explicit transit grants.
// Related: test/plugin/bgp-best-route-refresh-selected.ci -- received refresh wiring.
package fixture

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/bgp-best-route-refresh-selected", bgpBestRouteDriver(bgpBestRouteRefreshSelected), StopsDaemon)
}

// bgpBestRouteRefreshSelected retains the elected route before the recipient
// exists. The recipient's original socket observes initial selected replay and
// then requests another replay with a real RFC 2918 type-5 message. No source
// UPDATE or command-dispatched refresh can supply the second announcement.
func bgpBestRouteRefreshSelected(ctx context.Context, plugin *sdk.Plugin, address string) error {
	source, err := atomicAggregatePolicyConnect(ctx, address, netip.MustParseAddr(addrLoopbackSecond), 65000)
	if err != nil {
		return err
	}
	defer source.Close() //nolint:errcheck // The scenario retains both sockets until its assertions finish.
	if err := p12WaitPeersEOR(ctx, plugin, peerNameSource); err != nil {
		return err
	}
	if err := bgpBestRouteExpect(source, nil, nil, nil); err != nil {
		return fmt.Errorf("source initial End-of-RIB: %w", err)
	}
	if err := rfc8212ReloadTopology(ctx, plugin); err != nil {
		return err
	}
	prefix := []byte{24, 10, 254, 25}
	if err := bgpBestRouteSend(source, prefix, true, false); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, plugin, "10.254.25.0/24", addrLoopbackSecond); err != nil {
		return fmt.Errorf("route was not selected before recipient establishment: %w", err)
	}
	target, err := bgpBestRouteRefreshConnect(ctx, address)
	if err != nil {
		return err
	}
	defer target.Close() //nolint:errcheck // The scenario retains the original recipient through final output.
	path := []byte{2, 1, 0xfd, 0xe8}
	if err := bgpBestRouteRefreshInitial(target, prefix, path); err != nil {
		return fmt.Errorf("late recipient initial selected snapshot: %w", err)
	}
	if err := p12WaitPeersEOR(ctx, plugin, "destination"); err != nil {
		return err
	}
	before, err := peerCounter03(ctx, plugin, addrLoopbackSecond, "updates-received")
	if err != nil {
		return err
	}
	refreshBefore, err := bgpBestRouteRefreshReceived(ctx, plugin)
	if err != nil {
		return err
	}
	// RFC 2918 Section 3: a normal refresh has AFI 1, reserved byte 0 and
	// SAFI 1. This OPEN advertises standard refresh only, so the response has
	// no RFC 7313 BoRR/EoRR markers; the selected route itself is observed.
	packet := message.PackTo(&message.RouteRefresh{AFI: 1, SAFI: 1}, nil)
	if err := target.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	if n, err := target.Write(packet); err != nil {
		return err
	} else if n != len(packet) {
		return io.ErrShortWrite
	}
	if err := target.SetWriteDeadline(time.Time{}); err != nil {
		return err
	}
	if err := bgpBestRouteExpect(target, prefix, nil, path); err != nil {
		return fmt.Errorf("recipient-originated refresh did not replay selected route: %w", err)
	}
	if err := quiesce(ctx, plugin); err != nil {
		return err
	}
	after, err := peerCounter03(ctx, plugin, addrLoopbackSecond, "updates-received")
	if err != nil {
		return err
	}
	if before != after {
		return fmt.Errorf("refresh used new source UPDATEs: %d -> %d", before, after)
	}
	refreshAfter, err := bgpBestRouteRefreshReceived(ctx, plugin)
	if err != nil {
		return err
	}
	if refreshAfter != refreshBefore+1 {
		return fmt.Errorf("recipient refresh receive count %g -> %g, want exactly one request", refreshBefore, refreshAfter)
	}
	if err := policyFinalSessions(ctx, plugin, 2); err != nil {
		return err
	}
	if err := bgpBestRouteSend(source, prefix, true, true); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, plugin, "10.254.25.0/24", ""); err != nil {
		return err
	}
	if err := bgpBestRouteExpect(target, nil, prefix, nil); err != nil {
		return fmt.Errorf("post-refresh final withdrawal on original socket: %w", err)
	}
	if err := quiesce(ctx, plugin); err != nil {
		return err
	}
	if err := bgpBestRouteAPI(ctx, plugin, target, "10.253.25.0/24", []byte{24, 10, 253, 25}, path); err != nil {
		return fmt.Errorf("post-refresh API survivor fence: %w", err)
	}
	return policyFinalSessions(ctx, plugin, 2)
}

// bgpBestRouteRefreshInitial requires the complete snapshot before refresh.
// The engine's EOR does not wait for plugin output, so either frame may arrive
// first. Exactly two UPDATEs must contain one empty EOR and one exact selected
// route; duplicates, withdrawals, other NLRIs and changed ORIGIN/AS_PATH fail.
func bgpBestRouteRefreshInitial(conn net.Conn, prefix, path []byte) error {
	seenRoute, seenEOR := false, false
	for range 2 {
		update, err := atomicAggregatePolicyRead(conn)
		if err != nil {
			return err
		}
		if len(update.NLRI) == 0 && len(update.WithdrawnRoutes) == 0 && len(update.PathAttributes) == 0 {
			if seenEOR {
				return fmt.Errorf("initial snapshot repeated EOR without selected route %x", prefix)
			}
			seenEOR = true
			continue
		}
		if seenRoute {
			return fmt.Errorf("initial snapshot repeated route before its EOR: NLRI=%x withdrawn=%x", update.NLRI, update.WithdrawnRoutes)
		}
		if !bytes.Equal(update.NLRI, prefix) || len(update.WithdrawnRoutes) != 0 {
			return fmt.Errorf("initial snapshot NLRI=%x withdrawn=%x, want only selected %x", update.NLRI, update.WithdrawnRoutes, prefix)
		}
		_, _, origin, found := attribute.AttrFind(update.PathAttributes, attribute.AttrOrigin)
		if !found || !bytes.Equal(origin, []byte{0}) {
			return fmt.Errorf("initial selected route ORIGIN=%x present=%t, want IGP", origin, found)
		}
		_, _, actualPath, found := attribute.AttrFind(update.PathAttributes, attribute.AttrASPath)
		if !found || !bytes.Equal(actualPath, path) {
			return fmt.Errorf("initial selected route AS_PATH=%x present=%t, want exact %x", actualPath, found, path)
		}
		seenRoute = true
	}
	if !seenRoute || !seenEOR {
		return fmt.Errorf("initial snapshot incomplete: selected=%t EOR=%t", seenRoute, seenEOR)
	}
	return nil
}

func bgpBestRouteRefreshReceived(ctx context.Context, plugin *sdk.Plugin) (float64, error) {
	rows, err := peerRows03(ctx, plugin, addrLoopbackThird)
	if err != nil {
		return 0, err
	}
	row := rows[addrLoopbackThird]
	capabilities, _ := row["capabilities"].(map[string]any)
	if capabilities["route-refresh"] != true || capabilities["enhanced-route-refresh"] != false {
		return 0, fmt.Errorf("recipient did not negotiate standard-only route refresh: %v", capabilities)
	}
	messages, _ := row["messages"].(map[string]any)
	received, _ := messages["received"].(map[string]any)
	count, ok := received["route-refresh"].(float64)
	if !ok {
		return 0, fmt.Errorf("recipient omitted its received route-refresh counter: %v", row)
	}
	return count, nil
}

// bgpBestRouteRefreshConnect follows the existing zero-hold-time fixture
// handshake while adding the standard route-refresh capability to the OPEN.
func bgpBestRouteRefreshConnect(ctx context.Context, address string) (net.Conn, error) {
	dialer := net.Dialer{Timeout: time.Second, LocalAddr: net.TCPAddrFromAddrPort(netip.AddrPortFrom(netip.MustParseAddr(addrLoopbackThird), 0))}
	var conn net.Conn
	var dialErr error
	if !Poll(ctx, 60, 100*time.Millisecond, func() bool {
		conn, dialErr = dialer.DialContext(ctx, "tcp4", address)
		return dialErr == nil
	}) {
		return nil, fmt.Errorf("refresh recipient listener %s: %w", address, dialErr)
	}
	failed := true
	defer func() {
		if failed {
			_ = conn.Close()
		}
	}() //nolint:errcheck // Failed handshake owns this socket.
	if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return nil, err
	}
	open := &message.Open{Version: 4, MyAS: 65002, HoldTime: 0,
		BGPIdentifier: 0x7f000003, OptionalParams: []byte{2, 2, 2, 0}}
	if _, err := conn.Write(message.PackTo(open, nil)); err != nil {
		return nil, err
	}
	if err := delayOpenFixtureRead(conn, 1); err != nil {
		return nil, err
	}
	if err := delayOpenFixtureRead(conn, 4); err != nil {
		return nil, err
	}
	if _, err := conn.Write(message.PackTo(&message.Keepalive{}, nil)); err != nil {
		return nil, err
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	failed = false
	return conn, nil
}
