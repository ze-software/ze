// Design: docs/architecture/testing/interop.md -- exact foreign reload observations.
package bgp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
)

type gobgpReloadMessages struct {
	Open         uint64 `json:"open"`
	Update       uint64 `json:"update"`
	Refresh      uint64 `json:"refresh"`
	Notification uint64 `json:"notification"`
}

type gobgpReloadPeer struct {
	State *struct {
		Address  string `json:"neighbor_address"`
		ASN      uint32 `json:"peer_asn"`
		RouterID string `json:"router_id"`
		Session  uint32 `json:"session_state"`
		Flops    uint32 `json:"flops"`
		Messages *struct {
			Received *gobgpReloadMessages `json:"received"`
			Sent     *gobgpReloadMessages `json:"sent"`
		} `json:"messages"`
		Remote json.RawMessage `json:"remote_cap"`
		Local  json.RawMessage `json:"local_cap"`
	} `json:"state"`
	Timers *struct {
		State *struct {
			Uptime json.RawMessage `json:"uptime"`
		} `json:"state"`
	} `json:"timers"`
	Families []struct {
		State struct {
			Advertised uint64 `json:"advertised"`
		} `json:"state"`
	} `json:"afi_safis"`
}

func parseGoBGPReloadPeer(output, address string, asn uint32) (gobgpReloadPeer, error) {
	var peer gobgpReloadPeer
	if err := json.Unmarshal([]byte(output), &peer); err != nil {
		return peer, err
	}
	if peer.State == nil {
		return peer, errors.New("GoBGP neighbor has no state object")
	}
	if peer.State.Address != address {
		return peer, fmt.Errorf("GoBGP neighbor is %q, want %s", peer.State.Address, address)
	}
	if peer.State.ASN != asn {
		return peer, fmt.Errorf("GoBGP peer AS is %d, want %d", peer.State.ASN, asn)
	}
	if peer.State.RouterID != address {
		return peer, fmt.Errorf("GoBGP peer router ID is %q, want %s", peer.State.RouterID, address)
	}
	if peer.State.Session != 6 {
		return peer, fmt.Errorf("GoBGP peer state is %d, want pinned API ESTABLISHED=6", peer.State.Session)
	}
	if peer.State.Messages == nil {
		return peer, errors.New("GoBGP neighbor has no message counters")
	}
	if peer.State.Messages.Received == nil {
		return peer, errors.New("GoBGP neighbor has no received counters")
	}
	if peer.State.Messages.Sent == nil {
		return peer, errors.New("GoBGP neighbor has no sent counters")
	}
	if peer.State.Messages.Received.Open != 1 {
		return peer, errors.New("GoBGP did not receive exactly one original OPEN")
	}
	if peer.State.Messages.Sent.Open != 1 {
		return peer, errors.New("GoBGP did not send exactly one original OPEN")
	}
	if peer.Timers == nil {
		return peer, errors.New("GoBGP neighbor has no timers")
	}
	if peer.Timers.State == nil {
		return peer, errors.New("GoBGP neighbor has no timer state")
	}
	if len(peer.Timers.State.Uptime) == 0 {
		return peer, errors.New("GoBGP neighbor omitted raw uptime")
	}
	if len(peer.Families) != 1 {
		return peer, errors.New("GoBGP neighbor does not have exactly one AFI/SAFI")
	}
	if len(peer.State.Remote) == 0 {
		return peer, errors.New("GoBGP neighbor omitted remote capabilities")
	}
	if len(peer.State.Local) == 0 {
		return peer, errors.New("GoBGP neighbor omitted local capabilities")
	}
	var err error
	peer.State.Remote, err = canonicalGoBGPReloadCapabilities(peer.State.Remote)
	if err != nil {
		return peer, fmt.Errorf("canonicalize native remote capabilities: %w", err)
	}
	peer.State.Local, err = canonicalGoBGPReloadCapabilities(peer.State.Local)
	if err != nil {
		return peer, fmt.Errorf("canonicalize native local capabilities: %w", err)
	}
	return peer, nil
}

// canonicalGoBGPReloadCapabilities preserves a multiset, not an ordered list or
// a deduplicated set. The native producer builds remote capabilities from a map.
// Callers MUST normalize at the parser boundary, never on each fence comparison.
func canonicalGoBGPReloadCapabilities(raw json.RawMessage) (json.RawMessage, error) {
	var capabilities []json.RawMessage
	if err := json.Unmarshal(raw, &capabilities); err != nil {
		return nil, err
	}
	if capabilities == nil {
		return nil, errors.New("native capability collection is null, not an array")
	}
	for index := range capabilities {
		var value map[string]any
		decoder := json.NewDecoder(bytes.NewReader(capabilities[index]))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		if value == nil {
			return nil, errors.New("native capability is null, not an object")
		}
		canonical, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		capabilities[index] = canonical
	}
	slices.SortFunc(capabilities, func(left, right json.RawMessage) int { return bytes.Compare(left, right) })
	return json.Marshal(capabilities)
}

func sameGoBGPReloadSession(before, after gobgpReloadPeer, source bool) error {
	if !bytes.Equal(before.Timers.State.Uptime, after.Timers.State.Uptime) {
		return errors.New("GoBGP original session uptime changed")
	}
	if before.State.Flops != after.State.Flops {
		return errors.New("GoBGP session flap counter changed")
	}
	if !bytes.Equal(before.State.Remote, after.State.Remote) {
		return errors.New("GoBGP remote OPEN capabilities changed")
	}
	if !bytes.Equal(before.State.Local, after.State.Local) {
		return errors.New("GoBGP local OPEN capabilities changed")
	}
	left, right := before.State.Messages, after.State.Messages
	if left.Received.Refresh != right.Received.Refresh {
		return errors.New("GoBGP received ROUTE-REFRESH during reload")
	}
	if left.Sent.Refresh != right.Sent.Refresh {
		return errors.New("GoBGP sent ROUTE-REFRESH during reload")
	}
	if left.Received.Notification != right.Received.Notification {
		return errors.New("GoBGP received NOTIFICATION during reload")
	}
	if left.Sent.Notification != right.Sent.Notification {
		return errors.New("GoBGP sent NOTIFICATION during reload")
	}
	if source {
		if left.Received.Update != right.Received.Update {
			return errors.New("source UPDATE count changed across reload")
		}
	}
	return nil
}

// sourceGoBGPReloadPaths freezes all retained attributes, not a computed export.
// Only the age is omitted: its passage is not a route or source change.
func sourceGoBGPReloadPaths(output, source string, asn uint32, prefixes []string) (map[string][]map[string]json.RawMessage, error) {
	var routes map[string][]map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &routes); err != nil {
		return nil, err
	}
	if len(routes) != len(prefixes) {
		return nil, fmt.Errorf("source %s has %d destinations, want %d", source, len(routes), len(prefixes))
	}
	for _, prefix := range prefixes {
		paths := routes[prefix]
		if len(paths) != 1 {
			return nil, fmt.Errorf("source %s prefix %s has %d paths", source, prefix, len(paths))
		}
		path := paths[0]
		var nlri struct {
			Prefix string `json:"prefix"`
		}
		if err := json.Unmarshal(path["nlri"], &nlri); err != nil {
			return nil, err
		}
		if nlri.Prefix != prefix {
			return nil, errors.New("source NLRI differs from its exact destination key")
		}
		var address string
		if err := json.Unmarshal(path["peer-address"], &address); err != nil {
			return nil, err
		}
		if address != source {
			return nil, fmt.Errorf("source path is attributed to %q, want %s", address, source)
		}
		var attrs []json.RawMessage
		if err := json.Unmarshal(path["attrs"], &attrs); err != nil {
			return nil, err
		}
		if err := requireGoBGPReloadSourceAttributes(attrs, source, asn); err != nil {
			return nil, err
		}
		delete(path, "age")
	}
	return routes, nil
}

func requireGoBGPReloadSourceAttributes(attributes []json.RawMessage, source string, asn uint32) error {
	seen := make(map[uint8]bool)
	for _, raw := range attributes {
		var attribute struct {
			Type    uint8  `json:"type"`
			Origin  *uint8 `json:"value"`
			NextHop string `json:"nexthop"`
			ASPaths []struct {
				Type  uint8    `json:"segment_type"`
				Count uint8    `json:"num"`
				ASNs  []uint32 `json:"asns"`
			} `json:"as_paths"`
		}
		if err := json.Unmarshal(raw, &attribute); err != nil {
			return err
		}
		if attribute.Type == 0 {
			return errors.New("retained source attribute has no valid type")
		}
		if seen[attribute.Type] {
			return errors.New("retained source has duplicate path attributes")
		}
		seen[attribute.Type] = true
		// Only these mandatory attributes establish native source identity.
		// Additional attributes are frozen exactly by the retained-source oracle.
		switch attribute.Type {
		case 1:
			if attribute.Origin == nil {
				return errors.New("retained source omitted ORIGIN")
			}
			if *attribute.Origin > 2 {
				return errors.New("retained source has invalid ORIGIN")
			}
		case 2:
			if len(attribute.ASPaths) != 1 {
				return errors.New("retained source AS_PATH is not one native sequence")
			}
			sequence := attribute.ASPaths[0]
			if sequence.Type != 2 {
				return errors.New("retained source AS_PATH is not AS_SEQUENCE")
			}
			if sequence.Count != 1 {
				return errors.New("retained source AS_PATH count is not one")
			}
			if len(sequence.ASNs) != 1 {
				return errors.New("retained source AS_PATH does not have exactly one AS")
			}
			if sequence.ASNs[0] != asn {
				return errors.New("retained source AS_PATH does not name the native source")
			}
		case 3:
			if attribute.NextHop != source {
				return errors.New("retained source NEXT_HOP does not name the native source")
			}
		default:
			// Other attribute types are open wire input, retained byte-for-byte.
		}
	}
	for _, required := range []uint8{1, 2, 3} {
		if !seen[required] {
			return fmt.Errorf("retained source omitted mandatory attribute %d", required)
		}
	}
	return nil
}

type gobgpReloadRoute struct {
	Prefix string `json:"prefix"`
	Paths  []struct {
		Valid  *bool `json:"valid"`
		ASPath struct {
			String string `json:"string"`
		} `json:"aspath"`
		Metric   *uint32   `json:"metric"`
		NextHops []nextHop `json:"nexthops"`
	} `json:"paths"`
}

// requireGoBGPReloadRoute accepts absence ONLY as the pinned empty object.
func requireGoBGPReloadRoute(output, prefix, aspath, nextHop string, med uint32) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &object); err != nil {
		return err
	}
	if object == nil {
		return errors.New("FRR route output is null, not an object")
	}
	if len(object) == 0 {
		if aspath == "" {
			return nil
		}
		return fmt.Errorf("FRR has no route for %s", prefix)
	}
	if aspath == "" {
		return fmt.Errorf("FRR still has a route or unknown document for absent %s: %s", prefix, output)
	}
	var route gobgpReloadRoute
	if err := json.Unmarshal([]byte(output), &route); err != nil {
		return err
	}
	if route.Prefix != prefix {
		return fmt.Errorf("FRR prefix is %q, want %s", route.Prefix, prefix)
	}
	if len(route.Paths) != 1 {
		return fmt.Errorf("FRR has %d paths for %s, want one", len(route.Paths), prefix)
	}
	path := route.Paths[0]
	if path.Valid == nil {
		return errors.New("FRR omitted path validity")
	}
	if !*path.Valid {
		return errors.New("FRR path is invalid")
	}
	if strings.Join(strings.Fields(path.ASPath.String), " ") != aspath {
		return fmt.Errorf("FRR AS_PATH is %q, want %s", path.ASPath.String, aspath)
	}
	if path.Metric == nil {
		return errors.New("FRR omitted MED")
	}
	if *path.Metric != med {
		return fmt.Errorf("FRR MED is %d, want %d", *path.Metric, med)
	}
	if len(path.NextHops) != 1 {
		return errors.New("FRR did not decode exactly one next hop")
	}
	if path.NextHops[0].IP != nextHop {
		return fmt.Errorf("FRR next hop is %s, want %s", path.NextHops[0].IP, nextHop)
	}
	return nil
}

func sameGoBGPReloadSources(before, after map[string][]map[string]json.RawMessage) error {
	if !reflect.DeepEqual(before, after) {
		return errors.New("retained source routes or exact attributes changed during reload")
	}
	return nil
}
