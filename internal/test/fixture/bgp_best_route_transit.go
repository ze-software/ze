// Design: docs/functional-tests.md -- selected-route wiring through real daemon sockets.
// Related: test/plugin/bgp-best-route-transit-default.ci -- explicit transit attachments.
// Related: test/plugin/bgp-best-route-needs-export-policy.ci -- learned-only export admission.
package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const bgpBestRouteReloadApplied = "applied"

func init() {
	Register("plugin/bgp-best-route-transit-default", bgpBestRouteDriver(bgpBestRouteTransit), StopsDaemon)
	Register("plugin/bgp-best-route-needs-export-policy", bgpBestRoutePolicyDriver, StopsDaemon)
}

// bgpBestRoutePolicyDriver takes the active filename from the .ci declaration:
// each daemon placement has its own cwd and the runner names its config file.
func bgpBestRoutePolicyDriver(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("selected-route policy wiring requires <internal|forked> <BGP listen address> <active config file>")
	}
	if args[2] == "" {
		return errors.New("selected-route policy wiring requires a nonempty active config file")
	}
	return bgpBestRouteDriver(func(ctx context.Context, plugin *sdk.Plugin, address string) error {
		return bgpBestRouteExportPolicy(ctx, plugin, address, args[2])
	})(ctx, args[:2])
}

// bgpBestRouteDriver completes real plugin startup before opening the two
// zero-hold-time sockets. The placement is retained in the success verdict so
// the .ci cannot pass by observing the other transport's result.
func bgpBestRouteDriver(scenario func(context.Context, *sdk.Plugin, string) error) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) != 2 {
			return errors.New("selected-route wiring requires <internal|forked> <BGP listen address>")
		}
		if args[0] != sysribPlacementInternal {
			if args[0] != sysribPlacementForked {
				return fmt.Errorf("selected-route wiring: unknown placement %q", args[0])
			}
		}
		return Observe(ctx, "bgp-selected-test", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
			if err := scenario(ctx, plugin, args[1]); err != nil {
				return fmt.Errorf("selected-route %s: %w", args[0], err)
			}
			_, err := fmt.Fprintf(os.Stderr, "OK: selected-route %s wiring and API survivor\n", args[0])
			return err
		})
	}
}

// bgpBestRouteTransit asks whether an explicitly attached RIB advertises the
// elected iBGP route to an external peer without a route-server plugin. The
// local API baseline proves the destination works before the first election.
func bgpBestRouteTransit(ctx context.Context, plugin *sdk.Plugin, address string) error {
	source, target, err := bgpBestRouteConnect(ctx, plugin, address, 65000, 65002)
	if err != nil {
		return err
	}
	// The scenario MUST retain both original sockets until every assertion
	// finishes. These closes MUST release them on success and every failure.
	defer source.Close() //nolint:errcheck // The scenario already owns its useful result.
	defer target.Close() //nolint:errcheck // The scenario already owns its useful result.
	path := []byte{2, 1, 0xfd, 0xe8}
	if err := bgpBestRouteAPI(ctx, plugin, target, "10.253.0.0/24", []byte{24, 10, 253, 0}, path); err != nil {
		return fmt.Errorf("API baseline: %w", err)
	}
	prefix := []byte{24, 10, 254, 0}
	if err := bgpBestRouteSend(source, prefix, true, false); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, plugin, "10.254.0.0/24", addrLoopbackSecond); err != nil {
		return fmt.Errorf("received candidate did not become elected: %w", err)
	}
	if err := bgpBestRouteExpect(target, prefix, nil, path); err != nil {
		return fmt.Errorf("elected iBGP route never reached external destination: %w", err)
	}
	if err := bgpBestRouteSend(source, prefix, true, true); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, plugin, "10.254.0.0/24", ""); err != nil {
		return err
	}
	if err := bgpBestRouteExpect(target, nil, prefix, nil); err != nil {
		return fmt.Errorf("final elected-source withdrawal: %w", err)
	}
	if err := quiesce(ctx, plugin); err != nil {
		return err
	}
	if err := bgpBestRouteAPI(ctx, plugin, target, "10.253.1.0/24", []byte{24, 10, 253, 1}, path); err != nil {
		return fmt.Errorf("API survivor fence: %w", err)
	}
	return policyFinalSessions(ctx, plugin, 2)
}

// bgpBestRouteExportPolicy asks whether adding export policy releases a
// retained elected route on the original internal destination session. An API
// route after the election is the same-socket fence for the absent-policy case:
// a leaked learned announcement cannot hide behind an elapsed-time assertion.
func bgpBestRouteExportPolicy(ctx context.Context, plugin *sdk.Plugin, address, activeConfig string) error {
	source, target, err := bgpBestRouteConnect(ctx, plugin, address, 65001, 65000)
	if err != nil {
		return err
	}
	// The scenario MUST retain both original sockets through policy reload;
	// these closes MUST release them on success and every failure.
	defer source.Close() //nolint:errcheck // The scenario already owns its useful result.
	defer target.Close() //nolint:errcheck // The scenario already owns its useful result.
	if err := bgpBestRouteAPI(ctx, plugin, target, "10.253.0.0/24", []byte{24, 10, 253, 0}, nil); err != nil {
		return fmt.Errorf("unpolicied internal API baseline: %w", err)
	}
	prefix := []byte{24, 10, 254, 0}
	if err := bgpBestRouteSend(source, prefix, false, false); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, plugin, "10.254.0.0/24", addrLoopbackSecond); err != nil {
		return err
	}
	if err := quiesce(ctx, plugin); err != nil {
		return err
	}
	if err := bgpBestRouteAPI(ctx, plugin, target, "10.253.1.0/24", []byte{24, 10, 253, 1}, nil); err != nil {
		return fmt.Errorf("learned output without policy, or missing local survivor: %w", err)
	}
	if err := bgpBestRouteAddPolicy(ctx, plugin, activeConfig); err != nil {
		return err
	}
	if err := bgpBestRouteExpect(target, prefix, nil, []byte{2, 1, 0xfd, 0xe9}); err != nil {
		return fmt.Errorf("policy reload did not release retained elected route: %w", err)
	}
	if err := bgpBestRouteSend(source, prefix, false, true); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, plugin, "10.254.0.0/24", ""); err != nil {
		return err
	}
	if err := bgpBestRouteExpect(target, nil, prefix, nil); err != nil {
		return err
	}
	if err := quiesce(ctx, plugin); err != nil {
		return err
	}
	if err := bgpBestRouteAPI(ctx, plugin, target, "10.253.2.0/24", []byte{24, 10, 253, 2}, nil); err != nil {
		return fmt.Errorf("API survivor after elected withdrawal: %w", err)
	}
	return policyFinalSessions(ctx, plugin, 2)
}

// bgpBestRouteConnect waits for both passive peers to start before opening
// either socket. The shared listener's attribution changes while startup
// replaces its configured peers. It returns both sockets after EOR and
// next-hop resolution. The caller MUST close both returned sockets; this
// helper MUST close every socket it opens if setup fails.
func bgpBestRouteConnect(ctx context.Context, plugin *sdk.Plugin, address string, sourceASN, targetASN uint16) (net.Conn, net.Conn, error) {
	peers := []p12Peer{{name: peerNameSource, addr: addrLoopbackSecond}, {name: "destination", addr: addrLoopbackThird}}
	if err := p12WaitAllPeerFields(ctx, plugin, peers, "state", func(_ p12Peer, state any) bool {
		return state == stateActive
	}, "both passive peers to start before opening TCP"); err != nil {
		return nil, nil, err
	}
	source, err := atomicAggregatePolicyConnect(ctx, address, netip.MustParseAddr(addrLoopbackSecond), sourceASN)
	if err != nil {
		return nil, nil, err
	}
	target, err := atomicAggregatePolicyConnect(ctx, address, netip.MustParseAddr(addrLoopbackThird), targetASN)
	if err != nil {
		_ = source.Close() //nolint:errcheck // Setup already failed.
		return nil, nil, err
	}
	setup := func() error {
		if err := p12WaitPeersEOR(ctx, plugin, peerNameSource, "destination"); err != nil {
			return err
		}
		for _, socket := range []net.Conn{source, target} {
			if err := bgpBestRouteExpect(socket, nil, nil, nil); err != nil {
				return fmt.Errorf("initial End-of-RIB: %w", err)
			}
		}
		return rfc8212ReloadTopology(ctx, plugin)
	}
	if err := setup(); err != nil {
		_ = source.Close() //nolint:errcheck // Setup already failed.
		_ = target.Close() //nolint:errcheck // Setup already failed.
		return nil, nil, err
	}
	return source, target, nil
}

func bgpBestRouteAPI(ctx context.Context, plugin *sdk.Plugin, target net.Conn, prefix string, nlri, path []byte) error {
	if err := liveStagesRoute(ctx, plugin, addrLoopbackThird+" update text nhop 192.0.2.1 nlri add "+prefix); err != nil {
		return err
	}
	return bgpBestRouteExpect(target, nlri, nil, path)
}

// bgpBestRouteSend writes one standards-shaped source update; source selection
// never depends on test-only route injection or granting a sender permission.
func bgpBestRouteSend(conn net.Conn, prefix []byte, internal, withdraw bool) error {
	update := &message.Update{WithdrawnRoutes: prefix}
	if !withdraw {
		attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0}
		if internal {
			attrs = append(attrs, 0x40, 5, 4, 0, 0, 0, 100)
		} else {
			attrs = []byte{0x40, 1, 1, 0, 0x40, 2, 4, 2, 1, 0xfd, 0xe9}
		}
		attrs = append(attrs, 0x40, 3, 4, 192, 0, 2, 1)
		update = &message.Update{PathAttributes: attrs, NLRI: prefix}
	}
	packet := message.PackTo(update, nil)
	if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	n, err := conn.Write(packet)
	if err != nil {
		return err
	}
	if n != len(packet) {
		return io.ErrShortWrite
	}
	return conn.SetWriteDeadline(time.Time{})
}

func bgpBestRouteExpect(conn net.Conn, announced, withdrawn, expectedPath []byte) error {
	update, err := atomicAggregatePolicyRead(conn)
	if err != nil {
		return err
	}
	if !bytes.Equal(update.NLRI, announced) {
		return fmt.Errorf("wire NLRI=%x, want %x (withdrawn=%x)", update.NLRI, announced, update.WithdrawnRoutes)
	}
	if !bytes.Equal(update.WithdrawnRoutes, withdrawn) {
		return fmt.Errorf("wire withdrawn=%x, want %x (NLRI=%x)", update.WithdrawnRoutes, withdrawn, update.NLRI)
	}
	if len(announced) == 0 {
		return nil
	}
	_, _, origin, found := attribute.AttrFind(update.PathAttributes, attribute.AttrOrigin)
	if !found {
		return errors.New("useful selected output omitted ORIGIN")
	}
	if !bytes.Equal(origin, []byte{0}) {
		return fmt.Errorf("useful selected output changed ORIGIN: %x", origin)
	}
	_, _, path, found := attribute.AttrFind(update.PathAttributes, attribute.AttrASPath)
	if !found {
		return errors.New("useful selected output omitted AS_PATH")
	}
	// The socket OPEN negotiates two-octet ASNs. Comparing the complete wire
	// value checks segment framing and the exact source/local prepend, and
	// explicitly expects an empty path for API output to an internal peer.
	if !bytes.Equal(path, expectedPath) {
		return fmt.Errorf("useful output AS_PATH=%x, want exact %x", path, expectedPath)
	}
	return nil
}

func bgpBestRouteAddPolicy(ctx context.Context, plugin *sdk.Plugin, activeConfig string) error {
	data, err := os.ReadFile(activeConfig) //nolint:gosec // The .ci invocation names this functional run's own active configuration file.
	if err != nil {
		return err
	}
	schema, err := config.YANGSchema()
	if err != nil {
		return err
	}
	tree, err := config.NewParser(schema).Parse(string(data))
	if err != nil {
		return err
	}
	bgp := tree.GetContainer(rfc8212RootBGP)
	if bgp == nil {
		return errors.New("selected fixture omitted bgp")
	}
	peer := bgp.GetList("peer")["destination"]
	if peer == nil {
		return errors.New("selected fixture omitted destination")
	}
	filter := config.NewTree()
	export := config.NewTree()
	export.Set("final", actionAccept)
	filter.SetContainer(directionExport, export)
	peer.SetContainer("filter", filter)
	return bgpBestRouteReload(ctx, plugin, activeConfig, config.Serialize(tree, schema))
}

// bgpBestRouteReload changes the actual file this placement's daemon reads
// and uses the daemon's canonical request-reload command. A runner PID file is
// owned by the test root, while this plugin runs in its daemon's config cwd;
// the command targets the authenticated daemon without inferring a PID path.
func bgpBestRouteReload(ctx context.Context, plugin *sdk.Plugin, activeConfig, text string) error {
	baseline := int64(-1)
	reason := "no read"
	if !Poll(ctx, liveStagesPollAttempts, liveStagesPollDelay, func() bool {
		baseline, reason = reloadGenerationAnswer(ctx, plugin)
		return baseline >= 0
	}) {
		return fmt.Errorf("selected-route reload: no initial generation: %s", reason)
	}
	if err := os.WriteFile(activeConfig, []byte(text), 0o600); err != nil {
		return err
	}
	if _, err := done03(ctx, plugin, "request reload"); err != nil {
		return fmt.Errorf("selected-route reload: %w", err)
	}
	if !Poll(ctx, liveStagesPollAttempts, liveStagesPollDelay, func() bool {
		return reloadGeneration(ctx, plugin) > baseline
	}) {
		return fmt.Errorf("selected-route reload: generation did not advance from %d", baseline)
	}
	value, err := done03(ctx, plugin, "show system reload status")
	if err != nil {
		return err
	}
	status, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("selected-route reload: status is not an object: %v", value)
	}
	if status["last-outcome"] != bgpBestRouteReloadApplied {
		return fmt.Errorf("selected-route reload: expected applied, got %v", value)
	}
	return nil
}
