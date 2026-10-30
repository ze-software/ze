// Design: docs/architecture/update-building.md -- the third-party Link-Local an RFC 2545 next hop carries
// RFC: rfc/short/rfc2545.md
// Related: register_rfc2545_third_party_linux.go -- registers the two fixtures below
//
// The fixtures behind test/plugin/rfc2545-third-party-linklocal.ci. They give
// the host one veth link whose near end holds Ze's address and the test peer's
// address on 2001:db8:2545::/64, and seed the kernel neighbor table with two
// other routers on that subnet: N, whose global and Link-Local share one MAC,
// and M, whose global has no Link-Local entry. Permanent entries replace
// Neighbor Discovery, so the far end holds no address and answers nothing.

//go:build linux

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const (
	rfc2545ThirdPartyNear = "ze2545a"
	rfc2545ThirdPartyFar  = "ze2545b"
	// rfc2545ThirdPartyAddrGenModeNone is IN6_ADDR_GEN_MODE_NONE from
	// linux/if_link.h: no kernel Link-Local on either end, so the only
	// Link-Local entry on the link is the seeded one.
	rfc2545ThirdPartyAddrGenModeNone = 1
)

// rfc2545ThirdPartyNeighbor is one seeded neighbor entry on the near end.
type rfc2545ThirdPartyNeighbor struct {
	address string
	mac     string
}

// rfc2545ThirdPartyNeighbors are N's global and Link-Local (one MAC) and M's
// global alone (another MAC).
func rfc2545ThirdPartyNeighbors() []rfc2545ThirdPartyNeighbor {
	return []rfc2545ThirdPartyNeighbor{
		{address: "2001:db8:2545::9", mac: "02:00:00:25:45:09"},
		{address: "fe80::9", mac: "02:00:00:25:45:09"},
		{address: "2001:db8:2545::8", mac: "02:00:00:25:45:08"},
	}
}

// rfc2545ThirdPartySetup builds the link before the daemon starts, removing
// any left by an earlier run first. It is idempotent.
func rfc2545ThirdPartySetup(ctx context.Context, _ []string) error {
	if err := rfc2545ThirdPartyTeardown(ctx, nil); err != nil {
		return err
	}
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: rfc2545ThirdPartyNear},
		PeerName:  rfc2545ThirdPartyFar,
	}
	if err := netlink.LinkAdd(veth); err != nil {
		return fmt.Errorf("add veth %s/%s: %w", rfc2545ThirdPartyNear, rfc2545ThirdPartyFar, err)
	}
	near, err := netlink.LinkByName(rfc2545ThirdPartyNear)
	if err != nil {
		return fmt.Errorf("find %s: %w", rfc2545ThirdPartyNear, err)
	}
	far, err := netlink.LinkByName(rfc2545ThirdPartyFar)
	if err != nil {
		return fmt.Errorf("find %s: %w", rfc2545ThirdPartyFar, err)
	}
	for _, end := range []netlink.Link{near, far} {
		if err := netlink.LinkSetIP6AddrGenMode(end, rfc2545ThirdPartyAddrGenModeNone); err != nil {
			return fmt.Errorf("addrgenmode none on %s: %w", end.Attrs().Name, err)
		}
	}
	// Ze (::1) and the test peer (::2) both live on the near end, so the peer
	// shares the connected prefix with Ze and the session runs locally.
	for _, address := range []string{"2001:db8:2545::1", "2001:db8:2545::2"} {
		addr := &netlink.Addr{
			IPNet: &net.IPNet{IP: net.ParseIP(address), Mask: net.CIDRMask(64, 128)},
			Flags: unix.IFA_F_NODAD,
		}
		if err := netlink.AddrAdd(near, addr); err != nil && !errors.Is(err, syscall.EEXIST) {
			return fmt.Errorf("address %s/64 on %s: %w", address, rfc2545ThirdPartyNear, err)
		}
	}
	for _, end := range []netlink.Link{near, far} {
		if err := netlink.LinkSetUp(end); err != nil {
			return fmt.Errorf("bring %s up: %w", end.Attrs().Name, err)
		}
	}
	for _, seed := range rfc2545ThirdPartyNeighbors() {
		hardware, err := net.ParseMAC(seed.mac)
		if err != nil {
			return fmt.Errorf("fixture MAC %s: %w", seed.mac, err)
		}
		neighbor := &netlink.Neigh{
			LinkIndex:    near.Attrs().Index,
			Family:       netlink.FAMILY_V6,
			State:        netlink.NUD_PERMANENT,
			IP:           net.ParseIP(seed.address),
			HardwareAddr: hardware,
		}
		if err := netlink.NeighSet(neighbor); err != nil {
			return fmt.Errorf("neighbor %s on %s: %w", seed.address, rfc2545ThirdPartyNear, err)
		}
	}
	return nil
}

// rfc2545ThirdPartyTeardown removes the link. Deleting one end of a veth
// removes its peer, and the addresses and neighbor entries go with the link.
// A link that is already gone is not an error.
func rfc2545ThirdPartyTeardown(context.Context, []string) error {
	existing, err := netlink.LinkByName(rfc2545ThirdPartyNear)
	if err != nil {
		if _, notFound := errors.AsType[netlink.LinkNotFoundError](err); notFound {
			return nil
		}
		return fmt.Errorf("find %s: %w", rfc2545ThirdPartyNear, err)
	}
	if err := netlink.LinkDel(existing); err != nil {
		return fmt.Errorf("remove %s: %w", rfc2545ThirdPartyNear, err)
	}
	return nil
}
