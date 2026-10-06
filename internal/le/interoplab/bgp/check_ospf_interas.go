// Design: docs/architecture/testing/interop.md -- inter-AS source and flooded-LSA evidence.
// Related: check_ospf_bgpls.go -- the collector checks the resulting BGP-LS inventory.
// FRR: https://github.com/FRRouting/frr/blob/frr-10.3.1/ospfd/ospf_vty.c
// show_lsa_detail and show_ip_ospf_database_header define the peer JSON below.
package bgp

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/le/interoplab"
)

type ospfInterASHeader struct {
	Type     string     `json:"type"`
	ID       netip.Addr `json:"link-state-id"`
	Router   netip.Addr `json:"advertising-router"`
	Age      *uint16    `json:"age"`
	Checksum *uint16    `json:"checksum"`
	Length   uint16     `json:"length"`
}

type ospfInterASLink struct {
	Router   netip.Addr      `json:"advertising-router"`
	Scope    string          `json:"scope"`
	Instance uint32          `json:"instance"`
	LinkID   json.RawMessage `json:"link-id"`
	RemoteAS uint32          `json:"remote-as"`
	RemoteV4 netip.Addr      `json:"remote-asbr-ipv4"`
	RemoteV6 netip.Addr      `json:"remote-asbr-ipv6"`
}

func checkOSPFInterASDatabase(ctx context.Context, check *interoplab.CheckContext) error {
	command := zeCommand("show ospf database opaque-as")
	output, err := check.Lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	if err != nil {
		return err
	}
	header, err := ospfInterASSourceVerdict(output, check.Network.IPv4.Addr().Next().Next())
	if err != nil {
		return fmt.Errorf("Ze inter-AS database: %w; reply: %s", err, output)
	}
	command = []string{cmdVtysh, "-c", "show ip ospf database opaque-as json"}
	output, err = check.Lab.Query(ctx, peerFRR, command, queryEnvironment(peerFRR, command))
	if err != nil {
		return err
	}
	if err := ospfInterASFloodedVerdict(output, header); err != nil {
		return fmt.Errorf("FRR inter-AS database: %w; reply: %s", err, output)
	}
	return nil
}

// RFC 5392 Section 3.2.1: "The Remote-AS-Number sub-TLV MUST be included in the
// Link TLV of both the Inter-AS-TE-v2 LSA and Inter-AS-TE-v3 LSA."
// Match one decoded link to its live Type-11, opaque-type-6 header. Fields from
// unrelated rows must not collectively certify the configured advertisement.
func ospfInterASSourceVerdict(output string, local netip.Addr) (ospfInterASHeader, error) {
	var views []struct {
		Headers []ospfInterASHeader `json:"as-opaque"`
		Links   []ospfInterASLink   `json:"te"`
	}
	if err := json.Unmarshal([]byte(output), &views); err != nil {
		return ospfInterASHeader{}, err
	}
	remoteV4 := netip.MustParseAddr("203.0.113.9")
	remoteV6 := netip.MustParseAddr("2001:db8::9")
	for _, view := range views {
		for _, header := range view.Headers {
			if header.Type != "opaque-as" {
				continue
			}
			if header.Router != local {
				continue
			}
			if !header.ID.Is4() {
				continue
			}
			id := header.ID.As4()
			if id[0] != 6 {
				continue
			}
			if header.Age == nil {
				continue
			}
			if *header.Age >= 3600 {
				continue
			}
			if header.Checksum == nil {
				continue
			}
			if header.Length <= 20 {
				continue
			}
			instance := binary.BigEndian.Uint32(id[:]) & 0x00ffffff
			for _, decoded := range views {
				for _, link := range decoded.Links {
					if link.Router != local {
						continue
					}
					if link.Scope != "as" {
						continue
					}
					if link.Instance != instance {
						continue
					}
					if link.RemoteAS != 65001 {
						continue
					}
					if link.RemoteV4 != remoteV4 {
						continue
					}
					if link.RemoteV6 != remoteV6 {
						continue
					}
					// RFC 5392 Section 3.2.1: "The Link ID sub-TLV [OSPF-TE]
					// MUST NOT be used in the Link TLV of an Inter-AS-TE-v2
					// LSA, and the Neighbor ID sub-TLV [OSPF-V3-TE] MUST NOT
					// be used in the Link TLV of an Inter-AS-TE-v3 LSA."
					if len(link.LinkID) != 0 {
						return ospfInterASHeader{}, errors.New("inter-AS Link TLV contains a Link ID")
					}
					return header, nil
				}
			}
		}
	}
	return ospfInterASHeader{}, errors.New("no live Type-11 opaque-type-6 link with the configured remote AS and both ASBR IDs")
}

// FRR's AS-wide detail array, not a router ID elsewhere in its output, must
// contain the same live LSA. Its checksum and length bind this to Ze's body.
func ospfInterASFloodedVerdict(output string, want ospfInterASHeader) error {
	var view struct {
		Headers []struct {
			ID       netip.Addr `json:"linkStateId"`
			Router   netip.Addr `json:"advertisingRouter"`
			Age      *uint16    `json:"lsaAge"`
			Checksum string     `json:"checksum"`
			Length   uint16     `json:"length"`
		} `json:"asExternalOpaqueLsa"`
	}
	if err := json.Unmarshal([]byte(output), &view); err != nil {
		return err
	}
	for _, header := range view.Headers {
		if header.ID != want.ID {
			continue
		}
		if header.Router != want.Router {
			continue
		}
		if header.Age == nil {
			continue
		}
		if *header.Age >= 3600 {
			continue
		}
		checksum, err := strconv.ParseUint(header.Checksum, 16, 16)
		if err != nil {
			return fmt.Errorf("invalid flooded LSA checksum: %w", err)
		}
		if want.Checksum == nil {
			return errors.New("source inter-AS LSA has no checksum")
		}
		if uint16(checksum) != *want.Checksum {
			return errors.New("flooded inter-AS LSA does not match the source checksum")
		}
		if header.Length != want.Length {
			return errors.New("flooded inter-AS LSA does not match the source length")
		}
		return nil
	}
	return errors.New("FRR lacks the live inter-AS LSA in its AS-wide database")
}
