// Design: docs/architecture/traffic/cp-survival-2-copp-port179.md -- the neighbor set CoPP derives from the bgp section
// Related: validate_mandatory_bgp.go -- IsDynamicBGPGroup and listEntries, the other readers of the bgp section's shape
//
// The addresses a BGP neighbor may connect from are declared once, in the
// `connection > remote` container of a peer or a dynamic group. This file is
// the one reader of that container. The BGP config resolve
// (bgpconfig.ResolveBGPTree), the reactor's peer parse and the CoPP plugin all
// read it here, so they cannot disagree about which sources are neighbors.
// It lives in the config component because CoPP (gate ze_copp) does not
// depend on BGP (gate ze_bgp): a build with ze_copp and without ze_bgp must
// still compile, so CoPP may import this always-on package and never a BGP one.

package config

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
)

// bgpRemoteIPDynamic is the `connection > remote > ip` value that makes a group
// a dynamic group: its sessions come from any address in its ranges.
const bgpRemoteIPDynamic = "dynamic"

// BGPRemote answers the `connection > remote` container of one BGP session
// source (a peer, a group, or a resolved peer tree), in the shape Tree.ToMap
// and Tree.ToPluginMap write. It answers false when either container is absent.
func BGPRemote(source map[string]any) (map[string]any, bool) {
	connection, ok := source["connection"].(map[string]any)
	if !ok {
		return nil, false
	}
	remote, ok := connection["remote"].(map[string]any)
	if !ok {
		return nil, false
	}
	return remote, true
}

// BGPRemoteIP answers the `connection > remote > ip` value of one BGP session
// source. It answers false when the leaf is absent or empty. The value is the
// text the operator wrote: an address, or "dynamic" on a dynamic group.
func BGPRemoteIP(source map[string]any) (string, bool) {
	remote, ok := BGPRemote(source)
	if !ok {
		return "", false
	}
	ip, ok := remote["ip"].(string)
	if !ok {
		return "", false
	}
	if ip == "" {
		return "", false
	}
	return ip, true
}

// BGPRemoteRanges parses the `range` leaf-list of a dynamic group's
// `connection > remote` container. A leaf-list arrives as one string, a
// []string or a []any of strings, depending on the lowering; a non-string
// entry in a []any is skipped, as the YANG type admits none.
//
// It answers present false when the container holds no `range` leaf, so the
// caller can name that case on its own. An entry that does not parse is an
// error naming the entry.
func BGPRemoteRanges(remote map[string]any) (ranges []netip.Prefix, present bool, err error) {
	value, present := remote["range"]
	if !present {
		return nil, false, nil
	}
	switch v := value.(type) {
	case string:
		return appendBGPRange(nil, v)
	case []string:
		for _, text := range v {
			if ranges, _, err = appendBGPRange(ranges, text); err != nil {
				return nil, true, err
			}
		}
	case []any:
		for _, item := range v {
			text, ok := item.(string)
			if !ok {
				continue
			}
			if ranges, _, err = appendBGPRange(ranges, text); err != nil {
				return nil, true, err
			}
		}
	}
	return ranges, true, nil
}

// appendBGPRange parses one range entry onto ranges.
func appendBGPRange(ranges []netip.Prefix, text string) ([]netip.Prefix, bool, error) {
	prefix, err := netip.ParsePrefix(text)
	if err != nil {
		return nil, true, fmt.Errorf("invalid range %q: %w", text, err)
	}
	return append(ranges, prefix), true, nil
}

// BGPNeighborSources answers every source address a configured BGP neighbor
// may connect from, read from the bgp section in the shape Tree.ToMap and
// Tree.ToPluginMap write (the body under the `bgp` key, not the wrapper):
//
//   - each static peer, standalone or in a group, as a host prefix of its
//     `connection > remote > ip`, resolved with the precedence ResolveBGPTree
//     applies: the peer's own value, then its group's, then the bgp level's;
//   - each dynamic group's `range` entries as written.
//
// The answer is sorted and holds no duplicate. A section with no peer and no
// group answers an empty list and no error.
//
// It refuses what the BGP config parse refuses (a static peer with no remote
// address, with `ip dynamic`, or with an address that does not parse; a
// dynamic group with no valid range), so it never omits a neighbor the parse
// would accept, and never answers a set for a section the parse rejects.
//
// RFC 7454 Section 4: "This protection SHOULD be achieved by an Access Control
// List (ACL) that would discard all packets directed to TCP port 179 on the
// local device and sourced from an address not known or permitted to become a
// BGP neighbor."
func BGPNeighborSources(bgp map[string]any) ([]netip.Prefix, error) {
	var sources []netip.Prefix

	for name, peer := range listEntriesByKey(bgp, "peer") {
		prefix, err := bgpStaticPeerSource(peer, nil, bgp)
		if err != nil {
			return nil, fmt.Errorf("bgp/peer %s: %w", name, err)
		}
		sources = append(sources, prefix)
	}

	for groupName, group := range listEntriesByKey(bgp, "group") {
		if IsDynamicBGPGroup(group) {
			remote, _ := BGPRemote(group) // IsDynamicBGPGroup answered true, so the container exists.
			ranges, present, err := BGPRemoteRanges(remote)
			if err != nil {
				return nil, fmt.Errorf("bgp/group %s: %w", groupName, err)
			}
			if !present {
				return nil, fmt.Errorf("bgp/group %s: ip dynamic requires at least one range", groupName)
			}
			if len(ranges) == 0 {
				return nil, fmt.Errorf("bgp/group %s: ip dynamic requires at least one valid range", groupName)
			}
			sources = append(sources, ranges...)
		}
		for peerName, peer := range listEntriesByKey(group, "peer") {
			prefix, err := bgpStaticPeerSource(peer, group, bgp)
			if err != nil {
				return nil, fmt.Errorf("bgp/group %s peer %s: %w", groupName, peerName, err)
			}
			sources = append(sources, prefix)
		}
	}

	slices.SortFunc(sources, comparePrefix)
	return slices.Compact(sources), nil
}

// bgpStaticPeerSource answers one static peer's source as a host prefix. The
// layers are read in ResolveBGPTree's precedence: peer, then group (nil for a
// standalone peer), then the bgp level.
func bgpStaticPeerSource(peer, group, bgp map[string]any) (netip.Prefix, error) {
	text, ok := BGPRemoteIP(peer)
	if !ok && group != nil {
		text, ok = BGPRemoteIP(group)
	}
	if !ok {
		text, ok = BGPRemoteIP(bgp)
	}
	if !ok {
		return netip.Prefix{}, errors.New("missing required connection > remote > ip")
	}
	if text == bgpRemoteIPDynamic {
		return netip.Prefix{}, errors.New("ip dynamic is only valid at group level, not on individual peers")
	}
	addr, err := netip.ParseAddr(text)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("invalid remote ip %q: %w", text, err)
	}
	// A link-local peer carries its zone; a prefix holds no zone, and the
	// filter that consumes this set matches the address alone.
	addr = addr.WithZone("")
	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

// comparePrefix orders prefixes by address, then by length, so a sorted list
// is stable across reloads.
func comparePrefix(a, b netip.Prefix) int {
	if c := a.Addr().Compare(b.Addr()); c != 0 {
		return c
	}
	return a.Bits() - b.Bits()
}

// listEntriesByKey returns the entries of the list named name in data, keyed
// by the list key, in the shape Tree.ToMap writes. A value that is not an
// entry map (a lowered list order beside the entries) is skipped.
func listEntriesByKey(data map[string]any, name string) map[string]map[string]any {
	list, ok := data[name].(map[string]any)
	if !ok {
		return nil
	}
	entries := make(map[string]map[string]any, len(list))
	for key, v := range list {
		if entry, ok := v.(map[string]any); ok {
			entries[key] = entry
		}
	}
	return entries
}
