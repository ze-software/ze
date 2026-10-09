//go:build linux

// Design: docs/functional-tests.md -- real namespace BGP topology fixtures.
// Related: plugin_fixture_clamped_path_linux.go -- namespace and process lifecycle.
// Related: test/plugin/rfc2545-joint-subnet.ci -- exact recipient wire assertions.
package fixture

import (
	"context"
	"fmt"
	"net/netip"
	"runtime"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// jointSubnetLink declares both ends of one real veth. The owning namespaces
// remain separate even when the speaker bridges its two ports in the positive.
type jointSubnetLink struct {
	left, right         *testNetns
	leftName, rightName string
}

// jointSubnetAddress is an address whose namespace and interface ownership the
// fixture reads back before starting any daemon or peer.
type jointSubnetAddress struct {
	owner   *testNetns
	link    string
	address netip.Prefix
}

// jointSubnetTopology owns three namespaces until remove. Its role names match
// runWithPeers: sender is speaker S, router is recipient P, far is announcer N.
// Safe only on the fixture's locked thread.
type jointSubnetTopology struct {
	path      clampedPath
	links     []jointSubnetLink
	addresses []jointSubnetAddress
	bridge    bool
}

// jointSubnetDriver builds one RFC 2545 Section 3 scenario before using the
// established peer/process lifecycle. The caller MUST provide case <name> then
// the existing netns/peer/peer-after/run arguments. No kernel fact is injected
// into the daemon: its ordinary session setup reads the namespace interfaces.
func jointSubnetDriver(ctx context.Context, args []string) error {
	// Leave ten seconds below each carrier's hard command deadline for
	// child joins and namespace cleanup, including stalled completion fences.
	ctx, cancel := context.WithTimeout(ctx, 80*time.Second)
	defer cancel()
	if len(args) < 2 {
		return fmt.Errorf("joint-subnet: expected case <same-link|split-link|entity-off-link|recipient-off-link|replay>")
	}
	if args[0] != "case" {
		return fmt.Errorf("joint-subnet: expected case, got %q", args[0])
	}
	scenario := args[1]
	switch scenario {
	case "same-link", "split-link", "entity-off-link", "recipient-off-link", "replay":
	default:
		return fmt.Errorf("joint-subnet: unknown case %q", scenario)
	}
	plan, err := parseNetnsRunArgs(args[2:])
	if err != nil {
		return err
	}
	// Namespace switches and child inheritance are per thread. Keep this thread
	// locked for the fixture process lifetime, as clampedPathDriver does.
	runtime.LockOSThread()
	orig, err := netns.Get()
	if err != nil {
		return fmt.Errorf("joint-subnet: original namespace: %w", err)
	}
	defer orig.Close() //nolint:errcheck // best-effort descriptor close at exit
	var topology jointSubnetTopology
	defer topology.path.remove()
	if err := topology.create(plan.prefix, orig); err != nil {
		return err
	}
	if err := topology.configure(scenario, orig); err != nil {
		return fmt.Errorf("joint-subnet setup %s: %w", scenario, err)
	}
	if err := topology.assertOwnership(); err != nil {
		return fmt.Errorf("joint-subnet topology %s: %w", scenario, err)
	}
	netnsSay("TOPOLOGY-PASSED: ", scenario, " distinct S/P/N address owners and veth attachments")
	return runJointSubnetPeers(ctx, plan, &topology.path, orig)
}

// create allocates the fixed role namespaces. The caller MUST call path.remove
// even when creation fails, so an earlier successful namespace is not leaked.
func (p *jointSubnetTopology) create(prefix string, orig netns.NsHandle) error {
	for _, entry := range []struct {
		target **testNetns
		suffix string
	}{
		{&p.path.sender, "-s"},
		{&p.path.router, "-p"},
		{&p.path.far, "-n"},
	} {
		namespace, err := createTestNetns(prefix+entry.suffix, orig)
		if err != nil {
			return err
		}
		*entry.target = namespace
	}
	return nil
}

// configure creates only the two links each scenario requires. Routed negatives
// use an existing peer namespace as the intermediate router, not a dummy prefix.
func (p *jointSubnetTopology) configure(scenario string, orig netns.NsHandle) error {
	speaker, recipient, announcer := p.path.sender, p.path.router, p.path.far
	p.links = []jointSubnetLink{
		{speaker, recipient, "sp", "p0"},
		{speaker, announcer, "sn", "n0"},
	}
	if scenario == "entity-off-link" {
		p.links[1] = jointSubnetLink{recipient, announcer, "pn", "n0"}
	}
	if scenario == "recipient-off-link" {
		p.links[0] = jointSubnetLink{announcer, recipient, "np", "p0"}
	}
	for _, pair := range p.links {
		veth := &netlink.Veth{Name: pair.leftName, PeerName: pair.rightName, PeerNamespace: netlink.NsFd(int(pair.right.ns))}
		if err := pair.left.handle.LinkAdd(veth); err != nil {
			return fmt.Errorf("create %s/%s: %w", pair.leftName, pair.rightName, err)
		}
		if err := pair.left.configureLink(pair.leftName, 1500); err != nil {
			return err
		}
		if err := pair.right.configureLink(pair.rightName, 1500); err != nil {
			return err
		}
	}
	recipientGlobal, speakerGlobal, announcerGlobal := "2001:db8:a::2/64", "2001:db8:a::1/64", "2001:db8:a::9/64"
	announcerLinkLocal := "fe80::9/64"
	if scenario == "replay" {
		// Preserve the replay carrier's original exact next-hop bytes while
		// making each actor a real host on the same IPv6 link.
		recipientGlobal, speakerGlobal, announcerGlobal = "2001:db8::2/64", "2001:db8::254/64", "2001:db8::1/64"
		announcerLinkLocal = "fe80::1/64"
	}
	p.addAddress(recipient, "p0", recipientGlobal)
	p.addAddress(recipient, "p0", "fe80::2/64")
	p.addAddress(announcer, "n0", announcerLinkLocal)
	switch scenario {
	case "same-link", "replay":
		p.bridge = true
		bridge := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: "br0"}}
		if err := speaker.handle.LinkAdd(bridge); err != nil {
			return fmt.Errorf("create common bridge: %w", err)
		}
		for _, name := range []string{"sp", "sn"} {
			link, err := speaker.handle.LinkByName(name)
			if err != nil {
				return err
			}
			if err := speaker.handle.LinkSetMaster(link, bridge); err != nil {
				return err
			}
		}
		if err := speaker.configureLink("br0", 1500); err != nil {
			return err
		}
		p.addAddress(speaker, "br0", speakerGlobal)
		p.addAddress(announcer, "n0", announcerGlobal)
	case "split-link":
		p.addAddress(speaker, "sp", "2001:db8:a::1/64")
		p.addAddress(speaker, "sn", "2001:db8:b::1/64")
		p.addAddress(announcer, "n0", "2001:db8:b::9/64")
	case "entity-off-link":
		p.addAddress(speaker, "sp", "2001:db8:a::1/64")
		p.addAddress(recipient, "pn", "2001:db8:b::2/64")
		p.addAddress(announcer, "n0", "2001:db8:b::9/64")
	case "recipient-off-link":
		p.addAddress(speaker, "sn", "2001:db8:b::1/64")
		p.addAddress(announcer, "n0", "2001:db8:b::9/64")
		p.addAddress(announcer, "np", "2001:db8:a::9/64")
	}
	for _, entry := range p.addresses {
		if err := entry.owner.addAddrsNoDAD(entry.link, entry.address.String()); err != nil {
			return err
		}
	}
	switch scenario {
	case "same-link", "replay":
		return nil
	case "split-link":
		if err := recipient.addRoute("2001:db8:b::/64", "2001:db8:a::1", "p0", 0); err != nil {
			return err
		}
		if err := announcer.addRoute("2001:db8:a::/64", "2001:db8:b::1", "n0", 0); err != nil {
			return err
		}
		return speaker.writeSysctl(orig, procSysIPv6Forward, "1")
	case "entity-off-link":
		if err := speaker.addRoute("2001:db8:b::/64", "2001:db8:a::2", "sp", 0); err != nil {
			return err
		}
		if err := announcer.addRoute("2001:db8:a::/64", "2001:db8:b::2", "n0", 0); err != nil {
			return err
		}
		return recipient.writeSysctl(orig, procSysIPv6Forward, "1")
	case "recipient-off-link":
		if err := speaker.addRoute("2001:db8:a::/64", "2001:db8:b::9", "sn", 0); err != nil {
			return err
		}
		if err := recipient.addRoute("2001:db8:b::/64", "2001:db8:a::9", "p0", 0); err != nil {
			return err
		}
		return announcer.writeSysctl(orig, procSysIPv6Forward, "1")
	default:
		return fmt.Errorf("joint-subnet: unvalidated scenario %q", scenario)
	}
}

// addAddress records the expected ownership before it programs the kernel.
func (p *jointSubnetTopology) addAddress(owner *testNetns, link, address string) {
	p.addresses = append(p.addresses, jointSubnetAddress{owner, link, netip.MustParsePrefix(address)})
}

// assertOwnership reads all IPv6 addresses in all three namespaces, refusing
// unexpected global addresses and any duplicate ownership of a declared address.
// Auto-generated link-local addresses are harmless; the advertised fe80::9 must
// still exist exactly once, on N's n0, never on S or P.
func (p *jointSubnetTopology) assertOwnership() error {
	counts := make([]int, len(p.addresses))
	for _, owner := range []*testNetns{p.path.sender, p.path.router, p.path.far} {
		links, err := owner.handle.LinkList()
		if err != nil {
			return err
		}
		for _, link := range links {
			addresses, err := owner.handle.AddrList(link, netlink.FAMILY_V6)
			if err != nil {
				return err
			}
			for _, address := range addresses {
				actual, ok := netip.AddrFromSlice(address.IP)
				if !ok {
					return fmt.Errorf("invalid address on %s/%s", owner.name, link.Attrs().Name)
				}
				matched := false
				for i, expected := range p.addresses {
					if actual != expected.address.Addr() {
						continue
					}
					bits, _ := address.Mask.Size()
					if (jointSubnetAddress{owner, link.Attrs().Name, netip.PrefixFrom(actual, bits)}) != expected {
						return fmt.Errorf("wrong owner or prefix for %s on %s/%s", actual, owner.name, link.Attrs().Name)
					}
					counts[i]++
					matched = true
				}
				if actual.IsGlobalUnicast() && !matched {
					return fmt.Errorf("unexpected global address %s on %s/%s", actual, owner.name, link.Attrs().Name)
				}
			}
		}
	}
	for i, count := range counts {
		if count != 1 {
			return fmt.Errorf("address %s owner count %d, want one", p.addresses[i].address, count)
		}
	}
	for _, pair := range p.links {
		left, err := pair.left.handle.LinkByName(pair.leftName)
		if err != nil {
			return err
		}
		right, err := pair.right.handle.LinkByName(pair.rightName)
		if err != nil {
			return err
		}
		if left.Type() != "veth" {
			return fmt.Errorf("%s is not a veth", pair.leftName)
		}
		if right.Type() != "veth" {
			return fmt.Errorf("%s is not a veth", pair.rightName)
		}
		if left.Attrs().ParentIndex != right.Attrs().Index {
			return fmt.Errorf("%s has wrong peer index", pair.leftName)
		}
		if right.Attrs().ParentIndex != left.Attrs().Index {
			return fmt.Errorf("%s has wrong peer index", pair.rightName)
		}
		if p.bridge {
			bridge, err := p.path.sender.handle.LinkByName("br0")
			if err != nil {
				return err
			}
			if left.Attrs().MasterIndex != bridge.Attrs().Index {
				return fmt.Errorf("%s has wrong common-link bridge attachment", pair.leftName)
			}
			if right.Attrs().MasterIndex != 0 {
				return fmt.Errorf("%s unexpectedly bridged", pair.rightName)
			}
		} else {
			if left.Attrs().MasterIndex != 0 {
				return fmt.Errorf("%s unexpectedly bridged", pair.leftName)
			}
			if right.Attrs().MasterIndex != 0 {
				return fmt.Errorf("%s unexpectedly bridged", pair.rightName)
			}
		}
	}
	return nil
}
