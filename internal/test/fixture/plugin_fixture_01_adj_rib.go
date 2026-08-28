package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func plugin01AdjRIBInQuery(ctx context.Context, plugin *sdk.Plugin) error {
	if _, ok := plugin01TotalRoutes(ctx, plugin, 40); !ok {
		data, status, err := plugin01DispatchMap(ctx, plugin, "show bgp adj-rib-in status")
		if err != nil {
			return fmt.Errorf("adj-rib-in stored no inbound route: status=%q data=%v error=%w", status, data, err)
		}
		return fmt.Errorf("adj-rib-in stored no inbound route: status=%q data=%v error=<nil>", status, data)
	}
	if err := plugin01Quiesce(ctx, plugin); err != nil {
		return err
	}
	if _, err := plugin01RequireDone(ctx, plugin, "show bgp rib received"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: adj-rib-in query returned done")
	return nil
}

func plugin01ReplaySetup(ctx context.Context, plugin *sdk.Plugin, peers int, source, route string, attempts int) error {
	if !plugin01WaitPeers(ctx, plugin, peers, attempts) {
		return fmt.Errorf("%d peers did not establish", peers)
	}
	if !plugin01WaitCounter(ctx, plugin, source, "eor-sent", 1, attempts/2) {
		return errors.New("ze never sent its initial-sync EOR to the source peer")
	}
	if _, _, err := plugin.UpdateRoute(ctx, source, route); err != nil {
		return err
	}
	if err := plugin01Quiesce(ctx, plugin); err != nil {
		return err
	}
	if _, ok := plugin01TotalRoutes(ctx, plugin, attempts/2); !ok {
		return errors.New("adj-rib-in stored no route from the source peer")
	}
	return nil
}

func plugin01ReplayOne(ctx context.Context, plugin *sdk.Plugin, peer string) error {
	data, err := plugin01RequireDone(ctx, plugin, "request bgp adj-rib-in replay "+peer+" 0")
	if err != nil {
		return err
	}
	replayed := plugin01Number(data["replayed"])
	if replayed < 1 {
		return fmt.Errorf("replay to %s selected no route: %v", peer, data)
	}
	fmt.Fprintf(os.Stderr, "OK: replay to %s selected %.0f route(s)\n", peer, replayed)
	return nil
}

func plugin01ReplayAddPath(ctx context.Context, plugin *sdk.Plugin) error {
	const source = "127.0.0.1"
	destinations := []string{"127.0.0.2", "127.0.0.3"}
	if err := plugin01ReplaySetup(ctx, plugin, 3, source,
		"update text origin igp nhop 1.1.1.1 nlri ipv4/unicast add 192.0.2.0/24", 40); err != nil {
		return err
	}
	for _, peer := range destinations {
		if !Poll(ctx, 20, plugin01PollDelay, func() bool { return plugin01RouteUpdates(ctx, plugin, peer) >= 1 }) {
			return fmt.Errorf("%s was never sent the live forward", peer)
		}
	}
	fmt.Fprintln(os.Stderr, "OK: both destinations were sent the live forward")
	for _, peer := range destinations {
		if err := plugin01ReplayOne(ctx, plugin, peer); err != nil {
			return err
		}
	}
	for _, peer := range destinations {
		if !Poll(ctx, 24, plugin01PollDelay, func() bool { return plugin01RouteUpdates(ctx, plugin, peer) >= 2 }) {
			return fmt.Errorf("%s was never sent the relayed copy", peer)
		}
	}
	fmt.Fprintln(os.Stderr, "OK: both destinations were sent the relayed copy")
	return nil
}

func plugin01ReplayOnPeerUp(ctx context.Context, plugin *sdk.Plugin) error {
	const source, destination = "127.0.0.1", "127.0.0.2"
	if err := plugin01ReplaySetup(ctx, plugin, 2, source,
		"update text origin igp nhop 1.1.1.1 nlri ipv4/unicast add 192.0.2.0/24", 32); err != nil {
		return err
	}
	if !Poll(ctx, 16, plugin01PollDelay, func() bool { return plugin01RouteUpdates(ctx, plugin, destination) >= 1 }) {
		return errors.New("dest peer was never sent the live forward")
	}
	fmt.Fprintln(os.Stderr, "OK: dest was sent the live forward")
	if err := plugin01ReplayOne(ctx, plugin, destination); err != nil {
		return err
	}
	if !Poll(ctx, 20, plugin01PollDelay, func() bool { return plugin01RouteUpdates(ctx, plugin, destination) >= 2 }) {
		return errors.New("dest peer was never sent the relayed copy")
	}
	fmt.Fprintln(os.Stderr, "OK: dest was sent the relayed copy")
	return nil
}

func plugin01ReplayRFC2545(ctx context.Context, plugin *sdk.Plugin) error {
	const source, destination = "127.0.0.1", "127.0.0.2"
	if err := plugin01ReplaySetup(ctx, plugin, 2, source,
		"update text origin igp nhop 2001:db8:ffff::1 nlri ipv6/unicast add 2001:db8:ffff::/48", 40); err != nil {
		return err
	}
	if !Poll(ctx, 20, plugin01PollDelay, func() bool { return plugin01RouteUpdates(ctx, plugin, destination) >= 1 }) {
		return errors.New("destination was never sent the live forward")
	}
	fmt.Fprintln(os.Stderr, "OK: the destination was sent the live forward")
	if err := plugin01ReplayOne(ctx, plugin, destination); err != nil {
		return err
	}
	if !Poll(ctx, 24, plugin01PollDelay, func() bool { return plugin01RouteUpdates(ctx, plugin, destination) >= 2 }) {
		return errors.New("destination was never sent the relayed copy")
	}
	fmt.Fprintln(os.Stderr, "OK: the destination was sent the relayed copy")
	return nil
}

func plugin01ReplayUnownedPeer(ctx context.Context, plugin *sdk.Plugin) error {
	const source, destination = "127.0.0.1", "127.0.0.2"
	if !plugin01WaitPeers(ctx, plugin, 2, 32) {
		return errors.New("both peers did not establish")
	}
	if !plugin01WaitCounter(ctx, plugin, "*", "eor-sent", 2, 20) {
		return errors.New("ze never sent its initial-sync EOR to both peers")
	}
	if _, _, err := plugin.UpdateRoute(ctx, source, "update text origin igp nhop 1.1.1.1 nlri ipv4/unicast add 192.0.2.0/24"); err != nil {
		return err
	}
	if err := plugin01Quiesce(ctx, plugin); err != nil {
		return err
	}
	if _, ok := plugin01TotalRoutes(ctx, plugin, 20); !ok {
		return errors.New("adj-rib-in stored no route from the source peer")
	}
	fmt.Fprintln(os.Stderr, "OK: the source route is stored")
	sessions := plugin01PeerCounter(ctx, plugin, destination, "connections-established")
	if _, _, err := plugin.UpdateRoute(ctx, destination, "update text origin igp nhop 2.2.2.2 nlri ipv4/unicast add 203.0.113.0/24"); err != nil {
		return err
	}
	if err := plugin01Quiesce(ctx, plugin); err != nil {
		return err
	}
	if !Poll(ctx, 20, plugin01PollDelay, func() bool { return plugin01RouteUpdates(ctx, plugin, destination) >= 1 }) {
		return errors.New("observer marker never reached the dest peer")
	}
	markerUpdates := plugin01RouteUpdates(ctx, plugin, destination)
	fmt.Fprintln(os.Stderr, "OK: the marker closed the dest first session")
	if !Poll(ctx, 60, plugin01PollDelay, func() bool {
		return plugin01PeerCounter(ctx, plugin, destination, "connections-established") > sessions
	}) {
		return errors.New("dest peer never established a second session")
	}
	fmt.Fprintln(os.Stderr, "OK: dest established its second session")
	if !Poll(ctx, 40, plugin01PollDelay, func() bool { return plugin01RouteUpdates(ctx, plugin, destination) > markerUpdates }) {
		return errors.New("dest peer was replayed nothing when it came back")
	}
	fmt.Fprintln(os.Stderr, "OK: dest was replayed the stored route")
	return nil
}
