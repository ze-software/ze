// Design: docs/features/interfaces.md -- Unnumbered IPv4 units
// Related: config.go -- parseIPv4Settings reads the two leaves
// Related: config_apply.go -- desiredState installs the borrowed /32
//
// An unnumbered unit borrows the IPv4 address of another interface, the
// donor (Cisco `ip unnumbered`, Junos `unnumbered-address`). Ze installs the
// donor address on the unit as a /32, which is the shape FRR and the Linux
// kernel use for an unnumbered point-to-point link: the link owns an address
// for the IP source of its packets, and no subnet. RFC 2328 Section 8.1 is the
// reason OSPF needs this: "On these interfaces, the IP source should be set to
// any of the other IP addresses belonging to the router."

package iface

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

var (
	errUnnumberedWithAddress = errors.New("ipv4 unnumbered and ipv4 address are both set: " +
		"an unnumbered unit borrows its address from the donor, so it has none of its own; " +
		"remove one of them, e.g. `delete ... ipv4 address`")
	errUnnumberedWithDHCP = errors.New("ipv4 unnumbered and ipv4 dhcp are both set: " +
		"an unnumbered unit borrows its address from the donor, so it cannot lease one; " +
		"remove one of them, e.g. `delete ... ipv4 dhcp`")
	errUnnumberedSourceAlone = errors.New("ipv4 unnumbered-source is set without ipv4 unnumbered: " +
		"the source chooses one of the donor's addresses, and there is no donor; " +
		"set `ipv4 unnumbered <donor-interface>` or delete unnumbered-source")
)

// ConfigRoot is the config section the interface component owns. A component
// that reads the candidate interface section (ConfigReads) names it here.
const ConfigRoot = configRootInterface

// loopbackConfigName is the name the loopback block answers to as a donor.
const loopbackConfigName = "loopback"

// parseIPv4Unnumbered reads the unnumbered and unnumbered-source leaves of one
// ipv4 container into s. It refuses unnumbered beside an own address here,
// because only the container shows both; the donor is resolved later, once
// every interface is parsed (resolveUnnumbered).
func parseIPv4Unnumbered(v4 map[string]any, s *ipv4Settings) error {
	if v, ok := v4["unnumbered"].(string); ok && v != "" {
		s.Unnumbered = v
	}
	if v, ok := v4["unnumbered-source"].(string); ok && v != "" {
		a, err := netip.ParseAddr(v)
		if err != nil {
			return fmt.Errorf("ipv4 unnumbered-source %q: %w", v, err)
		}
		if !a.Is4() {
			return fmt.Errorf("ipv4 unnumbered-source %q: not an IPv4 address", v)
		}
		s.UnnumberedSource = a
	}
	if s.Unnumbered == "" {
		if s.UnnumberedSource.IsValid() {
			return errUnnumberedSourceAlone
		}
		return nil
	}
	if len(s.Addresses) > 0 {
		return errUnnumberedWithAddress
	}
	return nil
}

// unnumberedDonor is what one config interface offers as a donor.
type unnumberedDonor struct {
	disabled   bool
	unnumbered bool         // a unit of the donor is itself unnumbered
	addresses  []netip.Addr // enabled units in name order, then leaf-list order
}

// resolveUnnumbered picks the donor address of every unnumbered unit and adds
// it, as a /32, to the unit's Addresses so the apply path installs it like any
// other address. Every refusal is a commit refusal, because parseIfaceConfig
// runs at verify.
func (c *ifaceConfig) resolveUnnumbered() error {
	donors := c.unnumberedDonors()
	resolve := func(owner string, units []unitEntry) error {
		for i := range units {
			u := &units[i]
			if u.IPv4 == nil || u.IPv4.Unnumbered == "" {
				continue
			}
			addr, err := pickDonorAddress(donors, u.IPv4.Unnumbered, u.IPv4.UnnumberedSource)
			if err != nil {
				return fmt.Errorf("%s unit %q: %w", owner, u.Label, err)
			}
			u.IPv4.UnnumberedAddress = addr
			u.Addresses = append(u.Addresses, netip.PrefixFrom(addr, 32).String())
		}
		return nil
	}
	for _, e := range allIfaceEntries(c) {
		if err := resolve(e.Name, e.Units); err != nil {
			return err
		}
	}
	if c.Loopback != nil {
		return resolve(loopbackConfigName, c.Loopback.Units)
	}
	return nil
}

// unnumberedDonors maps every config interface name to what it can donate.
func (c *ifaceConfig) unnumberedDonors() map[string]unnumberedDonor {
	donors := make(map[string]unnumberedDonor)
	for _, e := range allIfaceEntries(c) {
		donors[e.Name] = donorFromUnits(e.Disable, e.Units)
	}
	if c.Loopback != nil {
		donors[loopbackConfigName] = donorFromUnits(false, c.Loopback.Units)
	}
	return donors
}

func donorFromUnits(disabled bool, units []unitEntry) unnumberedDonor {
	d := unnumberedDonor{disabled: disabled}
	// parseUnits reads a JSON map, so the slice order is random. Sorting by
	// unit name makes "the donor's first address" the same on every commit.
	ordered := make([]*unitEntry, 0, len(units))
	for i := range units {
		ordered = append(ordered, &units[i])
	}
	slices.SortFunc(ordered, func(a, b *unitEntry) int {
		if a.Label < b.Label {
			return -1
		}
		if a.Label > b.Label {
			return 1
		}
		return 0
	})
	for _, u := range ordered {
		if u.Disable || u.IPv4 == nil {
			continue
		}
		if u.IPv4.Unnumbered != "" {
			d.unnumbered = true
			continue
		}
		for _, a := range u.IPv4.Addresses {
			p, err := netip.ParsePrefix(a)
			if err != nil {
				continue // parseIPv4Settings already refused it
			}
			d.addresses = append(d.addresses, p.Addr())
		}
	}
	return d
}

// pickDonorAddress returns the address an unnumbered unit borrows from donor,
// or the commit refusal that names what is wrong with the donor.
func pickDonorAddress(donors map[string]unnumberedDonor, donor string, source netip.Addr) (netip.Addr, error) {
	d, ok := donors[donor]
	if !ok {
		return netip.Addr{}, fmt.Errorf("ipv4 unnumbered %q: no interface of that name in the config; "+
			"an unnumbered unit borrows the IPv4 address of another configured interface, "+
			"so name one that exists, e.g. `ipv4 unnumbered loopback`", donor)
	}
	if d.disabled {
		return netip.Addr{}, fmt.Errorf("ipv4 unnumbered %q: the donor is disabled, so it holds no address to borrow; "+
			"enable it or name another donor", donor)
	}
	if d.unnumbered {
		return netip.Addr{}, fmt.Errorf("ipv4 unnumbered %q: the donor is itself unnumbered, so it has no address of its own; "+
			"name a donor with an IPv4 address, usually loopback", donor)
	}
	if len(d.addresses) == 0 {
		return netip.Addr{}, fmt.Errorf("ipv4 unnumbered %q: the donor holds no IPv4 address; "+
			"add one with `set interface ... unit <u> ipv4 address <a.b.c.d/32>` on the donor", donor)
	}
	if !source.IsValid() {
		return d.addresses[0], nil
	}
	if slices.Contains(d.addresses, source) {
		return source, nil
	}
	return netip.Addr{}, fmt.Errorf("ipv4 unnumbered-source %s: the donor %q does not hold this address; "+
		"pick one of the donor's IPv4 addresses, or delete unnumbered-source to take its first", source, donor)
}

// UnnumberedDevices parses the interface section among sections and returns
// the OS device names of every enabled unnumbered unit, mapped to the donor
// address it borrows. A protocol that treats an unnumbered link differently
// (OSPF accepts it only as point-to-point) calls it at its own verify, so the
// answer describes the candidate config and not what the kernel holds yet.
func UnnumberedDevices(sections []sdk.ConfigSection) (map[string]netip.Addr, error) {
	cfg, err := parseIfaceSections(sections)
	if err != nil {
		return nil, err
	}
	devices := make(map[string]netip.Addr)
	collect := func(device string, units []unitEntry) {
		for i := range units {
			u := &units[i]
			if u.Disable || u.IPv4 == nil || u.IPv4.Unnumbered == "" {
				continue
			}
			devices[unitOSName(device, u)] = u.IPv4.UnnumberedAddress
		}
	}
	for _, e := range allIfaceEntries(cfg) {
		if e.Disable {
			continue
		}
		device := e.Name
		if e.OSName != "" {
			device = e.OSName
		}
		collect(device, e.Units)
	}
	if cfg.Loopback != nil {
		collect("lo", cfg.Loopback.Units)
	}
	return devices, nil
}
