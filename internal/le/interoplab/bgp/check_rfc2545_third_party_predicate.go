// Design: docs/architecture/testing/interop.md -- RFC 2545 Section 3 for a third-party next hop.
// Related: check_rfc2545_third_party.go -- checkRFC2545ThirdPartyLinkLocal, the body these predicates decide for.
package bgp

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strings"
)

// ifInet6ScopeLink is the scope column /proc/net/if_inet6 writes for a
// link-local address (IPV6_ADDR_LINKLOCAL, include/net/ipv6.h).
const ifInet6ScopeLink = "20"

// onlyLinkLocal returns the one link-local address /proc/net/if_inet6 lists on
// device. The file has one line per address: 32 hex digits, the interface
// index, the prefix length, the scope, the flags and the device name. None, or
// two, is an error rather than a pick: the scenario's third-party router must
// own exactly one, or the speaker's neighbor table cannot pair its global with a
// single link-local and the receiver's answer proves nothing about Ze.
func onlyLinkLocal(ifInet6, device string) (netip.Addr, error) {
	var found []netip.Addr
	for line := range strings.SplitSeq(ifInet6, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 6 {
			continue
		}
		if fields[5] != device {
			continue
		}
		if fields[3] != ifInet6ScopeLink {
			continue
		}
		octets, err := hex.DecodeString(fields[0])
		if err != nil {
			return netip.Addr{}, fmt.Errorf("decode /proc/net/if_inet6 address %q: %w", fields[0], err)
		}
		if len(octets) != 16 {
			return netip.Addr{}, fmt.Errorf("/proc/net/if_inet6 address %q is %d octets, want 16", fields[0], len(octets))
		}
		found = append(found, netip.AddrFrom16([16]byte(octets)))
	}
	if len(found) != 1 {
		return netip.Addr{}, fmt.Errorf("%s holds %d link-local addresses %v, want exactly 1", device, len(found), found)
	}
	return found[0], nil
}

// ipNeighbor is one entry of `ip -j -6 neigh show`, as iproute2 writes it.
type ipNeighbor struct {
	Destination string   `json:"dst"`
	LinkLayer   string   `json:"lladdr"`
	State       []string `json:"state"`
}

// neighborStateUnbound names the states in which an entry holds no working
// link-layer address (NUD_INCOMPLETE, NUD_FAILED). Ze's reader refuses to bind
// through them, so a lab whose entry sits there cannot show Ze's pairing.
var neighborStateUnbound = []string{"INCOMPLETE", "FAILED"}

// requireNeighborPair reports whether the speaker's neighbor table, the JSON
// of `ip -j -6 neigh show dev eth0`, names global and linkLocal with mac, both
// bound, and no OTHER link-local with that MAC. That is the table state under
// which Ze's reader answers linkLocal for global; any other state is a lab
// that cannot discriminate, so it is reported as such rather than left for the
// receiver's answer to blame on Ze.
func requireNeighborPair(neighJSON string, global, linkLocal netip.Addr, mac net.HardwareAddr) error {
	var entries []ipNeighbor
	if err := json.Unmarshal([]byte(neighJSON), &entries); err != nil {
		return fmt.Errorf("decode the speaker's neighbor table: %w", err)
	}
	seenGlobal := false
	seenLinkLocal := false
	for _, entry := range entries {
		address, err := netip.ParseAddr(entry.Destination)
		if err != nil {
			return fmt.Errorf("invalid neighbor address %q: %w", entry.Destination, err)
		}
		if entry.LinkLayer == "" {
			continue
		}
		hardware, err := net.ParseMAC(entry.LinkLayer)
		if err != nil {
			return fmt.Errorf("invalid neighbor link-layer address %q: %w", entry.LinkLayer, err)
		}
		if !slices.Equal(hardware, mac) {
			continue
		}
		unbound := false
		for _, state := range entry.State {
			if slices.Contains(neighborStateUnbound, state) {
				unbound = true
			}
		}
		if unbound {
			continue
		}
		if address == global {
			seenGlobal = true
			continue
		}
		if !address.IsLinkLocalUnicast() {
			continue
		}
		if address != linkLocal {
			return fmt.Errorf("the speaker's neighbor table also binds link-local %s to %s, so the router's link-local is ambiguous", address, mac)
		}
		seenLinkLocal = true
	}
	if !seenGlobal {
		return fmt.Errorf("the speaker's neighbor table holds no bound entry for %s with %s", global, mac)
	}
	if !seenLinkLocal {
		return fmt.Errorf("the speaker's neighbor table holds no bound entry for %s with %s", linkLocal, mac)
	}
	return nil
}

// parseContainerMAC reads /sys/class/net/<device>/address. An empty or
// unparsable answer is an error, because the neighbor assertion compares
// against it and a zero MAC would match nothing for a reason that is not Ze's.
func parseContainerMAC(output string) (net.HardwareAddr, error) {
	text := strings.TrimSpace(output)
	if text == "" {
		return nil, errors.New("the third-party router reported no MAC address")
	}
	mac, err := net.ParseMAC(text)
	if err != nil {
		return nil, fmt.Errorf("invalid MAC address %q: %w", text, err)
	}
	return mac, nil
}
