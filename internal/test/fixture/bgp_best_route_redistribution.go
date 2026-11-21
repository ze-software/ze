// Design: docs/guide/redistribution.md -- configured admission to BGP.
// Related: test/plugin/bgp-best-route-redistribution-selected.ci -- AC-17 daemon proof.
package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/bgp-best-route-redistribution-selected", bgpBestRouteRedistributionDriver, StopsDaemon)
}

func bgpBestRouteRedistributionDriver(ctx context.Context, args []string) error {
	if len(args) != 3 || args[2] == "" {
		return errors.New("selected redistribution requires <internal|forked> <BGP listen address> <active config file>")
	}
	return bgpBestRouteDriver(func(ctx context.Context, plugin *sdk.Plugin, address string) error {
		return bgpBestRouteRedistributionSelected(ctx, plugin, address, args[2])
	})(ctx, args[:2])
}

// bgpBestRouteRedistributionSelected starts with useful learned P and K. Real
// static configuration then offers P with distance10, K at configured200 and
// an unadmitted system route U. P's override beats eBGP20; K's configured
// distance loses. Removing the actual import rule exposes learned P again
// while all three static contributions remain in the system Routing Table.
func bgpBestRouteRedistributionSelected(ctx context.Context, plugin *sdk.Plugin, address, activeConfig string) error {
	source, err := atomicAggregatePolicyConnect(ctx, address, netip.MustParseAddr(addrLoopbackSecond), 65001)
	if err != nil {
		return err
	}
	defer source.Close() //nolint:errcheck // Original sockets stay open through both reloads.
	target, err := atomicAggregatePolicyConnect(ctx, address, netip.MustParseAddr(addrLoopbackThird), 65002)
	if err != nil {
		return err
	}
	defer target.Close() //nolint:errcheck // Original sockets stay open through both reloads.
	if err := p12WaitPeersEOR(ctx, plugin, peerNameSource, "destination"); err != nil {
		return err
	}
	for _, socket := range []net.Conn{source, target} {
		if err := bgpBestRouteExpect(socket, nil, nil, nil); err != nil {
			return fmt.Errorf("initial End-of-RIB: %w", err)
		}
	}
	if err := bgpBestRouteRedistributionTopology(ctx, plugin); err != nil {
		return err
	}
	prefix := []byte{24, 10, 254, 17}
	control := []byte{24, 10, 254, 18}
	learnedPath := []byte{2, 2, 0xfd, 0xe8, 0xfd, 0xe9}
	for _, route := range []struct {
		text string
		nlri []byte
	}{{"10.254.17.0/24", prefix}, {"10.254.18.0/24", control}} {
		if err := bgpBestRouteSend(source, route.nlri, false, false); err != nil {
			return err
		}
		if err := rfc8212ReloadBest(ctx, plugin, route.text, addrLoopbackSecond); err != nil {
			return err
		}
		if err := bgpBestRouteExpect(target, route.nlri, nil, learnedPath); err != nil {
			return fmt.Errorf("initial learned %s: %w", route.text, err)
		}
	}
	data, err := os.ReadFile(activeConfig) //nolint:gosec // The .ci names this daemon's own active file.
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
	addition, err := config.NewParser(schema).Parse(`static {
 table default {
  route 10.254.17.0/24 { distance 10; metric 7; tag 42; blackhole { } }
  route 10.254.18.0/24 { metric 9; tag 42; blackhole { } }
  route 10.254.19.0/24 { metric 11; tag 7; blackhole { } }
 }
}`)
	if err != nil {
		return err
	}
	tree.SetContainer("static", addition.GetContainer("static"))
	if err := bgpBestRouteReload(ctx, plugin, activeConfig, config.Serialize(tree, schema)); err != nil {
		return fmt.Errorf("add real static contributions: %w", err)
	}
	tagAccepted, tagRefused := uint32(42), uint32(7)
	if err := importCandidateState(ctx, plugin, "10.254.17.0/24", protocolStatic, 10, &tagAccepted, true, "", ""); err != nil {
		return err
	}
	if err := importCandidateState(ctx, plugin, "10.254.18.0/24", protocolStatic, 200, &tagAccepted, false, "", ""); err != nil {
		return err
	}
	if err := importCandidateState(ctx, plugin, "10.254.19.0/24", protocolStatic, 200, &tagRefused, true, "", ""); err != nil {
		return fmt.Errorf("unadmitted control never became a real system route: %w", err)
	}
	if err := bgpBestRouteRedistributedWire(target, prefix); err != nil {
		return fmt.Errorf("accepted distance override did not replace learned P: %w", err)
	}
	if err := quiesce(ctx, plugin); err != nil {
		return err
	}
	originatedPath := []byte{2, 1, 0xfd, 0xe8}
	if err := bgpBestRouteAPI(ctx, plugin, target, "10.253.17.0/24", []byte{24, 10, 253, 17}, originatedPath); err != nil {
		return fmt.Errorf("unadmitted U leaked or useful learned K changed: %w", err)
	}
	before, err := peerCounter03(ctx, plugin, addrLoopbackSecond, "updates-received")
	if err != nil {
		return err
	}
	// Keep the real static routes and their distance configuration. Remove
	// only the admission rule through the normal parsed-config reload.
	tree.RemoveContainer("redistribute")
	if err := bgpBestRouteReload(ctx, plugin, activeConfig, config.Serialize(tree, schema)); err != nil {
		return fmt.Errorf("remove configured redistribution rule: %w", err)
	}
	if err := bgpBestRouteExpect(target, prefix, nil, learnedPath); err != nil {
		return fmt.Errorf("same-key learned fallback after rule removal: %w", err)
	}
	if err := policyFinalSessions(ctx, plugin, 2); err != nil {
		return fmt.Errorf("after learned fallback on rule removal: %w", err)
	}
	for _, route := range []struct {
		prefix   string
		distance uint8
		tag      *uint32
		selected bool
	}{{"10.254.17.0/24", 10, &tagAccepted, true}, {"10.254.18.0/24", 200, &tagAccepted, false}, {"10.254.19.0/24", 200, &tagRefused, true}} {
		if err := importCandidateState(ctx, plugin, route.prefix, protocolStatic, route.distance, route.tag, route.selected, "", ""); err != nil {
			return fmt.Errorf("rule removal changed system contribution: %w", err)
		}
	}
	if err := quiesce(ctx, plugin); err != nil {
		return err
	}
	after, err := peerCounter03(ctx, plugin, addrLoopbackSecond, "updates-received")
	if err != nil {
		return err
	}
	if after != before {
		return fmt.Errorf("learned fallback required new source UPDATEs: %d -> %d", before, after)
	}
	if err := bgpBestRouteAPI(ctx, plugin, target, "10.253.18.0/24", []byte{24, 10, 253, 18}, originatedPath); err != nil {
		return fmt.Errorf("post-removal API fence, learned K survivor and unadmitted U absence: %w", err)
	}
	return policyFinalSessions(ctx, plugin, 2)
}

func bgpBestRouteRedistributionTopology(ctx context.Context, plugin *sdk.Plugin) error {
	link, err := net.InterfaceByName("zeac17")
	if err != nil {
		return fmt.Errorf("AC17 runner-owned topology link: %w", err)
	}
	if link.Index == 0 || link.Flags&net.FlagUp == 0 {
		return fmt.Errorf("AC17 topology link is not a real up interface: %+v", link)
	}
	if _, err := waitCandidates(ctx, plugin, "192.0.2.0/24", func(row candidateRow) bool {
		path, found := candidatePathOf(row, "connected")
		return found && path.Selected && path.Interface == link.Name
	}); err != nil {
		return fmt.Errorf("AC17 connected next-hop subnet: %w", err)
	}
	metrics, err := plugin.RouteMetrics(ctx, []string{"192.0.2.1", "192.0.2.3"})
	if err != nil {
		return err
	}
	if len(metrics.Distances) != 2 {
		return fmt.Errorf("AC17 topology returned %d next-hop metrics, want two", len(metrics.Distances))
	}
	for _, metric := range metrics.Distances {
		if metric != (rpc.RouteMetric{Resolved: true}) {
			return fmt.Errorf("AC17 terminal topology route is not resolved: %+v", metric)
		}
	}
	return nil
}

func bgpBestRouteRedistributedWire(conn net.Conn, prefix []byte) error {
	update, err := atomicAggregatePolicyRead(conn)
	if err != nil {
		return err
	}
	if !bytes.Equal(update.NLRI, prefix) || len(update.WithdrawnRoutes) != 0 {
		return fmt.Errorf("redistributed NLRI=%x withdrawn=%x, want only %x", update.NLRI, update.WithdrawnRoutes, prefix)
	}
	for _, expected := range []struct {
		code  attribute.AttributeCode
		value []byte
	}{{attribute.AttrOrigin, []byte{2}}, {attribute.AttrASPath, []byte{2, 1, 0xfd, 0xe8}}, {attribute.AttrNextHop, []byte{127, 0, 0, 1}}} {
		_, _, value, present := attribute.AttrFind(update.PathAttributes, expected.code)
		if !present || !bytes.Equal(value, expected.value) {
			return fmt.Errorf("redistributed attribute %d=%x present=%t, want %x", expected.code, value, present, expected.value)
		}
	}
	return nil
}
