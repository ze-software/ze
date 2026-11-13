// Design: docs/architecture/testing/interop.md -- OSPF redistribution to eBGP.
// Related: check_ospf_redistribute_default.go -- the live peer query.
// FRR: https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_route.c --
// bgp_show_route and route_vty_out_detail produce the prefix, paths, valid,
// bestpath.overall, peer.peerId, peer.type, aspath and nexthops fields.
package bgp

import (
	"encoding/json"
	"fmt"
	"net/netip"
)

func requireRedistributedExternalPath(output string, prefix netip.Prefix, zeHop netip.Addr) error {
	var document struct {
		Prefix netip.Prefix `json:"prefix"`
		Paths  []struct {
			Valid    bool `json:"valid"`
			BestPath struct {
				Overall bool `json:"overall"`
			} `json:"bestpath"`
			Peer struct {
				ID   netip.Addr `json:"peerId"`
				Type string     `json:"type"`
			} `json:"peer"`
		} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return fmt.Errorf("decode FRR redistributed path: %w", err)
	}
	if document.Prefix != prefix {
		return fmt.Errorf("FRR prefix = %s, want %s", document.Prefix, prefix)
	}
	if len(document.Paths) != 1 {
		return fmt.Errorf("FRR redistributed prefix has %d paths, want 1", len(document.Paths))
	}
	path := document.Paths[0]
	if !path.Valid {
		return fmt.Errorf("FRR redistributed path valid = %t, want true", path.Valid)
	}
	if !path.BestPath.Overall {
		return fmt.Errorf("FRR redistributed path best = %t, want true", path.BestPath.Overall)
	}
	if path.Peer.ID != zeHop {
		return fmt.Errorf("FRR redistributed path peer = %s, want %s", path.Peer.ID, zeHop)
	}
	if path.Peer.Type != "external" {
		return fmt.Errorf("FRR redistributed path peer type = %q, want external", path.Peer.Type)
	}
	if err := requireFRRASPath(output, "65001"); err != nil {
		return err
	}
	entries, err := parseFRRNextHops(output)
	if err != nil {
		return err
	}
	return requireSoleNextHop(entries, zeHop)
}
