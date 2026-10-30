// Design: docs/architecture/update-building.md -- the third-party Link-Local an RFC 2545 next hop carries
// RFC: rfc/short/rfc2545.md
// Related: register_rfc2545_third_party_linux.go -- registers the driver below
// Related: plugin_fixture_clamped_path_linux.go -- the namespace and peer lifecycle it reuses
//
// The driver behind test/plugin/rfc2545-third-party-linklocal.ci. It builds two
// network namespaces joined by one veth on 2001:db8:2545::/64: the sender,
// where Ze runs at ::1, and the router, where the test peer runs at ::2. It
// seeds the sender's neighbor table with two other routers on that subnet: N,
// whose global and Link-Local share one MAC, and M, whose global has no
// Link-Local entry. Permanent entries replace Neighbor Discovery, so neither
// router exists as a host. The namespaces, and every link and entry in them,
// go when the driver returns, so the host keeps nothing.

//go:build linux

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

const (
	// rfc2545ThirdPartySenderLink and rfc2545ThirdPartyRouterLink are the two
	// ends of the one veth.
	rfc2545ThirdPartySenderLink = "sp"
	rfc2545ThirdPartyRouterLink = "p0"
	rfc2545ThirdPartySenderAddr = "2001:db8:2545::1/64"
	rfc2545ThirdPartyRouterAddr = "2001:db8:2545::2/64"
	// rfc2545ThirdPartyDeadline leaves ten seconds below the carrier's own
	// command deadline for child joins and namespace removal.
	rfc2545ThirdPartyDeadline = 80 * time.Second
)

// rfc2545ThirdPartyNeighbor is one seeded neighbor entry on the sender's link.
type rfc2545ThirdPartyNeighbor struct {
	address string
	mac     string
}

// rfc2545ThirdPartyNeighbors is the table the .ci asserts against: N's global
// and Link-Local on one MAC, and M's global with no Link-Local on its MAC.
func rfc2545ThirdPartyNeighbors() []rfc2545ThirdPartyNeighbor {
	return []rfc2545ThirdPartyNeighbor{
		{address: "2001:db8:2545::9", mac: "02:00:00:25:45:09"},
		{address: "fe80::9", mac: "02:00:00:25:45:09"},
		{address: "2001:db8:2545::8", mac: "02:00:00:25:45:08"},
	}
}

// rfc2545ThirdPartyDriver is `plugin/rfc2545-third-party-linklocal`. It takes
// the clamped path's netns/peer/run arguments: `netns <prefix> peer router
// <script> run <argv...>`. The peer decides the verdict.
func rfc2545ThirdPartyDriver(ctx context.Context, args []string) error {
	ctx, cancel := context.WithTimeout(ctx, rfc2545ThirdPartyDeadline)
	defer cancel()
	plan, err := parseNetnsRunArgs(args)
	if err != nil {
		return err
	}
	if len(plan.peers) != 1 {
		return errors.New("rfc2545-third-party-linklocal: expected exactly one peer, in the router namespace")
	}
	if plan.peers[0].namespace != clampedRoleRouter {
		return errors.New("rfc2545-third-party-linklocal: expected exactly one peer, in the router namespace")
	}
	// Namespace switches and child inheritance are per thread, so the thread
	// stays locked for the life of the process, as clampedPathDriver does.
	runtime.LockOSThread()
	orig, err := netns.Get()
	if err != nil {
		return fmt.Errorf("rfc2545-third-party-linklocal: original namespace: %w", err)
	}
	defer orig.Close() //nolint:errcheck // best-effort descriptor close at exit

	var path clampedPath
	defer path.remove()
	if err := rfc2545ThirdPartyTopology(&path, plan.prefix, orig); err != nil {
		return fmt.Errorf("rfc2545-third-party-linklocal setup: %w", err)
	}
	netnsSay("TOPOLOGY-PASSED: sender 2001:db8:2545::1, router 2001:db8:2545::2, N and M seeded in the sender's neighbor table")
	return runWithPeers(ctx, plan, &path, orig)
}

// rfc2545ThirdPartyTopology creates the two namespaces, the veth, both
// addresses and the seeded neighbor entries. The caller MUST call path.remove
// even when this fails, so a namespace created before the failure is removed.
func rfc2545ThirdPartyTopology(path *clampedPath, prefix string, orig netns.NsHandle) error {
	sender, err := createTestNetns(prefix+"-s", orig)
	if err != nil {
		return err
	}
	path.sender = sender
	router, err := createTestNetns(prefix+"-p", orig)
	if err != nil {
		return err
	}
	path.router = router

	veth := &netlink.Veth{
		Name:          rfc2545ThirdPartySenderLink,
		PeerName:      rfc2545ThirdPartyRouterLink,
		PeerNamespace: netlink.NsFd(int(router.ns)),
	}
	if err := sender.handle.LinkAdd(veth); err != nil {
		return fmt.Errorf("create %s/%s: %w", rfc2545ThirdPartySenderLink, rfc2545ThirdPartyRouterLink, err)
	}
	if err := sender.configureLink(rfc2545ThirdPartySenderLink, 1500); err != nil {
		return err
	}
	if err := router.configureLink(rfc2545ThirdPartyRouterLink, 1500); err != nil {
		return err
	}
	if err := sender.addAddrsNoDAD(rfc2545ThirdPartySenderLink, rfc2545ThirdPartySenderAddr); err != nil {
		return err
	}
	if err := router.addAddrsNoDAD(rfc2545ThirdPartyRouterLink, rfc2545ThirdPartyRouterAddr); err != nil {
		return err
	}

	link, err := sender.handle.LinkByName(rfc2545ThirdPartySenderLink)
	if err != nil {
		return fmt.Errorf("find %s in %s: %w", rfc2545ThirdPartySenderLink, sender.name, err)
	}
	for _, seed := range rfc2545ThirdPartyNeighbors() {
		hardware, err := net.ParseMAC(seed.mac)
		if err != nil {
			return fmt.Errorf("fixture MAC %s: %w", seed.mac, err)
		}
		neighbor := &netlink.Neigh{
			LinkIndex:    link.Attrs().Index,
			Family:       netlink.FAMILY_V6,
			State:        netlink.NUD_PERMANENT,
			IP:           net.ParseIP(seed.address),
			HardwareAddr: hardware,
		}
		if err := sender.handle.NeighSet(neighbor); err != nil {
			return fmt.Errorf("neighbor %s on %s in %s: %w", seed.address, rfc2545ThirdPartySenderLink, sender.name, err)
		}
	}
	return nil
}
