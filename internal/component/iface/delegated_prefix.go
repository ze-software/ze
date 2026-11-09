// Design: docs/guide/pppoe.md -- numbering a LAN unit from a delegated prefix
// Related: config.go -- parseIPv6Settings reads the delegated-prefix list
// Related: register.go -- parseAndVerifyIfaceSections calls verifyDelegatedPrefixes
//
// RFC: rfc/short/rfc7084.md, rfc/short/rfc9915.md

package iface

import (
	"cmp"
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// delegatedInterfaceIDDefault is the YANG default of interface-identifier. The
// config tree delivers only what the operator wrote, so the parser repeats it.
const delegatedInterfaceIDDefault = "::1"

// delegatedSubnetBits is the length of the subnet a LAN unit takes.
const delegatedSubnetBits = 64

// delegatedPrefixConfig is one entry of a unit's ipv6 delegated-prefix list.
type delegatedPrefixConfig struct {
	// Source is the pppoe-client whose DHCPv6 lease carries the delegation.
	Source string
	// SubnetID selects the /64 within the delegated prefix.
	SubnetID uint16
	// InterfaceID is the unit's host part. Only its low 64 bits are set.
	InterfaceID netip.Addr
}

// parseDelegatedPrefixes reads the delegated-prefix list of a unit's ipv6
// container, ordered by source so the result does not depend on Go's map
// iteration.
func parseDelegatedPrefixes(v6 map[string]any) ([]delegatedPrefixConfig, error) {
	lm, ok := v6["delegated-prefix"].(map[string]any)
	if !ok {
		return nil, nil
	}
	sources := make([]string, 0, len(lm))
	for source := range lm {
		sources = append(sources, source)
	}
	slices.Sort(sources)

	out := make([]delegatedPrefixConfig, 0, len(sources))
	for _, source := range sources {
		entry, _ := lm[source].(map[string]any)
		dp, err := parseDelegatedPrefixEntry(source, entry)
		if err != nil {
			return nil, err
		}
		out = append(out, dp)
	}
	return out, nil
}

// parseDelegatedPrefixEntry parses one list entry; the list key is the source.
func parseDelegatedPrefixEntry(source string, entry map[string]any) (delegatedPrefixConfig, error) {
	dp := delegatedPrefixConfig{Source: source}
	raw, ok := entry["subnet-id"].(string)
	if !ok {
		return dp, fmt.Errorf("ipv6 delegated-prefix %s: subnet-id is missing; write ipv6 delegated-prefix %s subnet-id <0-65535>", source, source)
	}
	id, err := strconv.ParseUint(raw, 10, 16)
	if err != nil {
		return dp, fmt.Errorf("ipv6 delegated-prefix %s subnet-id %q: not a number from 0 to 65535", source, raw)
	}
	dp.SubnetID = uint16(id)

	ifid := delegatedInterfaceIDDefault
	if v, ok := entry["interface-identifier"].(string); ok {
		ifid = v
	}
	addr, err := netip.ParseAddr(ifid)
	if err != nil {
		return dp, fmt.Errorf("ipv6 delegated-prefix %s interface-identifier %q: not an IPv6 address; write one such as ::2", source, ifid)
	}
	if !addr.Is6() || addr.Is4In6() {
		return dp, fmt.Errorf("ipv6 delegated-prefix %s interface-identifier %q: not an IPv6 address; write one such as ::2", source, ifid)
	}
	octets := addr.As16()
	if binary.BigEndian.Uint64(octets[:8]) != 0 {
		return dp, fmt.Errorf("ipv6 delegated-prefix %s interface-identifier %s: the high 64 bits are set, but only the host part of the /64 is taken from it; write the low 64 bits alone, such as ::2", source, ifid)
	}
	// RFC 4291 Section 2.6.1: the all-zero interface identifier is the
	// Subnet-Router anycast address, which a unit must not claim as its own.
	if binary.BigEndian.Uint64(octets[8:]) == 0 {
		return dp, fmt.Errorf("ipv6 delegated-prefix %s interface-identifier %s: the all-zero host part is the Subnet-Router anycast address (RFC 4291 Section 2.6.1); write a non-zero one such as ::1", source, ifid)
	}
	dp.InterfaceID = addr
	return dp, nil
}

// delegatedSubnetIDMax returns the largest subnet id a delegation of the given
// length can hold, and false when the length leaves no room for a /64.
func delegatedSubnetIDMax(delegatedBits int) (uint16, bool) {
	if delegatedBits > delegatedSubnetBits {
		return 0, false
	}
	room := delegatedSubnetBits - delegatedBits
	if room >= 16 {
		return 0xffff, true
	}
	return uint16(1)<<room - 1, true
}

// errDelegationTooSmall is the run-time outcome of a subnet id the delegation
// the server chose cannot hold (RFC 9915 lets the server ignore the hint).
var errDelegationTooSmall = errors.New("the delegated prefix is too small for this subnet-id")

// deriveDelegatedAddress returns the unit's address in its /64, as a /64
// prefix whose address carries the interface identifier.
//
// RFC 7084 Section 4.3 L-2: "The IPv6 CE router MUST assign a separate /64
// from its delegated prefix(es) (and ULA prefix if configured to provide ULA
// addressing) for each of its LAN interfaces." The subnet id sits in the bits
// between the delegated length and 64, right-aligned at bit 64, so subnet-id 1
// of 2001:db8:aa00::/56 is 2001:db8:aa00:1::/64.
func deriveDelegatedAddress(delegated netip.Prefix, subnetID uint16, interfaceID netip.Addr) (netip.Prefix, error) {
	if !delegated.Addr().Is6() {
		return netip.Prefix{}, fmt.Errorf("delegated prefix %s: not IPv6", delegated)
	}
	idMax, ok := delegatedSubnetIDMax(delegated.Bits())
	if !ok {
		return netip.Prefix{}, fmt.Errorf("delegated prefix %s: longer than /64: %w", delegated, errDelegationTooSmall)
	}
	if subnetID > idMax {
		return netip.Prefix{}, fmt.Errorf("delegated prefix %s holds subnet ids 0 to %d, not %d: %w",
			delegated, idMax, subnetID, errDelegationTooSmall)
	}
	network := delegated.Masked().Addr().As16()
	host := interfaceID.As16()
	var out [16]byte
	binary.BigEndian.PutUint64(out[:8], binary.BigEndian.Uint64(network[:8])|uint64(subnetID))
	copy(out[8:], host[8:])
	return netip.PrefixFrom(netip.AddrFrom16(out), delegatedSubnetBits), nil
}

// delegatedUse names one unit's claim on a source's subnet id.
type delegatedUse struct {
	source   string
	subnetID uint16
}

// verifyDelegatedPrefixes refuses a delegated-prefix entry whose source is
// not a pppoe-client requesting a delegated prefix, whose subnet id the
// source's pd length cannot hold, or whose source and subnet id another unit
// already takes.
func verifyDelegatedPrefixes(cfg *ifaceConfig) error {
	if cfg == nil {
		return nil
	}
	sources := make(map[string]dhcpv6UnitConfig, len(cfg.PPPoEClient))
	for i := range cfg.PPPoEClient {
		c := &cfg.PPPoEClient[i]
		if !requestsDelegation(c.IPv6.DHCPv6) {
			continue
		}
		sources[c.Name] = c.IPv6.DHCPv6
	}

	// The parser builds the interface and unit lists from Go maps, so their
	// order changes from run to run. Verify walks the units sorted by interface
	// then unit name, so the same config always reports the same error: the
	// first unit refused, and the unit a duplicate names as the earlier claim.
	var units []delegatedUnit
	forEachEnabledUnit(cfg, func(ifaceName string, u *unitEntry) {
		if u.IPv6 == nil {
			return
		}
		if len(u.IPv6.DelegatedPrefixes) == 0 {
			return
		}
		units = append(units, delegatedUnit{iface: ifaceName, label: u.Label, prefixes: u.IPv6.DelegatedPrefixes})
	})
	slices.SortFunc(units, compareDelegatedUnits)

	claimed := make(map[delegatedUse]string)
	for _, du := range units {
		unit := du.iface + " unit " + du.label
		// parseDelegatedPrefixes ordered the entries by source.
		for _, dp := range du.prefixes {
			if err := verifyDelegatedEntry(unit, dp, sources, claimed); err != nil {
				return err
			}
		}
	}
	return nil
}

// delegatedUnit is one enabled unit carrying delegated-prefix entries, as
// verifyDelegatedPrefixes walks it.
type delegatedUnit struct {
	iface    string
	label    string
	prefixes []delegatedPrefixConfig
}

// compareDelegatedUnits orders units by interface name, then unit name.
func compareDelegatedUnits(a, b delegatedUnit) int {
	if c := cmp.Compare(a.iface, b.iface); c != 0 {
		return c
	}
	return cmp.Compare(a.label, b.label)
}

// requestsDelegation reports whether a pppoe-client's DHCPv6 settings make it
// a delegated-prefix source: its DHCPv6 client runs and asks for a prefix.
// Verify and the editor's completion both ask it, so they agree on the set.
func requestsDelegation(d dhcpv6UnitConfig) bool {
	if !d.Enabled {
		return false
	}
	return d.PDRequested
}

// DelegationSourceReference names the ze:reference source the delegated-prefix
// source leaf declares (`ze:reference "pppoe-client-delegation"`).
const DelegationSourceReference = "pppoe-client-delegation"

// delegationSourceReference answers the pppoe-clients a delegated-prefix entry
// may name: those of the candidate whose ipv6 dhcpv6 requests a delegated
// prefix, the set verifyDelegatedPrefixes accepts. root holds one entry per
// top-level section, as Tree.ToPluginMap lowers it, so a list is a map keyed
// by entry name. An entry whose ipv6 container does not parse is not offered;
// verify reports its error. Safe for concurrent use: it only reads root.
func delegationSourceReference(root map[string]any, _ []string) yang.ReferenceAnswer {
	section, _ := root["interface"].(map[string]any) // no interface section: no source
	clients, _ := section["pppoe-client"].(map[string]any)
	names := make([]string, 0, len(clients))
	for name, value := range clients {
		entry, _ := value.(map[string]any) // an empty entry configures no ipv6
		v6, err := parsePPPoEClientIPv6(entry)
		if err != nil {
			continue
		}
		if !requestsDelegation(v6.DHCPv6) {
			continue
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return yang.ReferenceAnswer{Names: names, Check: acceptDelegationSource}
}

// acceptDelegationSource is the commit judge of the reference: it accepts
// every value, because verifyDelegatedEntry refuses an unknown source with the
// unit that names it and the pppoe-client config that fixes it, which the
// reference walk cannot say.
func acceptDelegationSource(string) error {
	// refuse-by-default: guards verifyDelegatedEntry -- the interface verify refuses an unknown source with the unit and the fix
	return nil
}

// verifyDelegatedEntry judges one entry of one unit and records its claim.
func verifyDelegatedEntry(unit string, dp delegatedPrefixConfig, sources map[string]dhcpv6UnitConfig, claimed map[delegatedUse]string) error {
	source, ok := sources[dp.Source]
	if !ok {
		return fmt.Errorf("interface %s: ipv6 delegated-prefix %s: no pppoe-client of that name requests a delegated prefix, so the unit would never get a /64; set interface pppoe-client %s ipv6 dhcpv6 enabled true and ipv6 dhcpv6 pd length <48-64>, or name a pppoe-client that does",
			unit, dp.Source, dp.Source)
	}
	if source.PDLength > 0 {
		idMax, _ := delegatedSubnetIDMax(source.PDLength)
		if dp.SubnetID > idMax {
			return fmt.Errorf("interface %s: ipv6 delegated-prefix %s subnet-id %d: pd length %d holds subnet ids 0 to %d; write a subnet-id from 0 to %d, or shorten pppoe-client %s ipv6 dhcpv6 pd length",
				unit, dp.Source, dp.SubnetID, source.PDLength, idMax, idMax, dp.Source)
		}
	}
	// RFC 7084 Section 4.3 L-2: "a separate /64 ... for each of its LAN
	// interfaces": two units on one /64 would share it.
	use := delegatedUse{source: dp.Source, subnetID: dp.SubnetID}
	if other, taken := claimed[use]; taken {
		return fmt.Errorf("interface %s and interface %s both take subnet-id %d from delegated-prefix %s, so they would share one /64; give each unit its own subnet-id",
			other, unit, dp.SubnetID, dp.Source)
	}
	claimed[use] = unit
	return nil
}
