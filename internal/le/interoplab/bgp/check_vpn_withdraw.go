// Design: docs/architecture/testing/interop.md -- independent VPN inventory proof.
// Related: check_labeled_withdraw.go -- retains the SAFI 4 proof and drives these phases.
// Upstream: https://github.com/FRRouting/frr/blob/frr-10.3.1/bgpd/bgp_route.c
// Upstream: https://github.com/FRRouting/frr/blob/frr-10.3.1/bgpd/bgp_mplsvpn.c
// Upstream: https://github.com/FRRouting/frr/blob/frr-10.3.1/bgpd/bgp_debug.c
// Upstream: https://github.com/FRRouting/frr/blob/frr-10.3.1/bgpd/bgp_vty.c
// RFC 8277 Section 2.4 -- see rfc/short/rfc8277.md.
//
// MUTATION: keyVPN in internal/core/bgp/nlri/nlrisplit/prefix_key.go returning
// raw instead of key keeps label/Compatibility bytes in identity. Immediate
// relay still removes the targets at FRR, but DOWN emits them again, so the
// exact target withdrawal count becomes two. Returning key[9:] instead drops
// RD identity: target removal erases the equal-prefix survivor from inventory,
// and FRR still holds RD65004:30 after DOWN.
package bgp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

type vpnWithdrawPhase uint8

const (
	vpnWithdrawUnspecified vpnWithdrawPhase = iota
	vpnWithdrawAnnounced
	vpnWithdrawTargetsGone
	vpnWithdrawAllGone
)

// These are the two fixture families, not a protocol support inventory.
// The explicit "rd all" selects show_bgp_ip_vpn_rd_cmd, avoiding the overlapping
// generic "all" spelling whose serializer can wrap tables by address family.
func vpnWithdrawFamilies() [2]struct {
	command string
	prefix  string
	label   int
} {
	return [2]struct {
		command string
		prefix  string
		label   int
	}{
		{"show bgp ipv4 vpn rd all json", "10.12.0.0/24", 200},
		{"show bgp ipv6 vpn rd all json", "2001:db8:12::/48", 300},
	}
}

func waitVPNWithdrawState(ctx context.Context, check *interoplab.CheckContext, phase vpnWithdrawPhase) error {
	neighbor := networkHostAddress(check.Network, 2)
	for _, family := range vpnWithdrawFamilies() {
		answer, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
			Timeout: 150 * time.Second, Interval: time.Second,
			Description: "FRR VPN RD inventory " + family.prefix,
		}, func(probe context.Context) (string, error) {
			return check.Lab.Query(probe, peerFRR, []string{cmdVtysh, "-c", family.command}, nil)
		}, func(output string) bool {
			return requireVPNWithdrawTable(output, neighbor, family.prefix, phase) == nil
		})
		if err != nil {
			return fmt.Errorf("VPN phase %d: %w; last table: %s", phase, err, answer)
		}
	}
	return nil
}

// bgp_show_table_rd nests its ordinary route_vty_out paths under
// routes.routeDistinguishers[RD][prefix]. Recursive field searches would confuse
// equal prefixes under distinct RDs. Its completely empty table is exactly {};
// the caller MUST additionally prove the live session and withdrawal log entries
// after prior positive reads before treating that form as disappearance.
func requireVPNWithdrawTable(output, neighbor, prefix string, phase vpnWithdrawPhase) error {
	want := []string{"65004:10", "65004:20", "65004:30"}
	switch phase {
	case vpnWithdrawUnspecified:
		return fmt.Errorf("unspecified VPN withdrawal phase")
	case vpnWithdrawAnnounced:
	case vpnWithdrawTargetsGone:
		want = want[2:]
	case vpnWithdrawAllGone:
		want = nil
	default:
		return fmt.Errorf("unknown VPN withdrawal phase %d", phase)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		return fmt.Errorf("decode FRR VPN table: %w", err)
	}
	if envelope == nil {
		return fmt.Errorf("FRR VPN table is null")
	}
	if len(envelope) == 0 {
		if phase == vpnWithdrawAllGone {
			return nil
		}
		return fmt.Errorf("FRR VPN table is empty before injector DOWN")
	}
	var table struct {
		RouterID    string  `json:"routerId"`
		LocalAS     *uint32 `json:"localAS"`
		TotalRoutes *int    `json:"totalRoutes"`
		TotalPaths  *int    `json:"totalPaths"`
		Routes      struct {
			RDs map[string]map[string][]struct {
				Peer string `json:"peerId"`
				Path string `json:"path"`
			} `json:"routeDistinguishers"`
		} `json:"routes"`
	}
	if err := json.Unmarshal([]byte(output), &table); err != nil {
		return fmt.Errorf("decode FRR VPN path inventory: %w", err)
	}
	if table.LocalAS == nil {
		return fmt.Errorf("FRR VPN table has no localAS")
	}
	if *table.LocalAS != 65002 {
		return fmt.Errorf("FRR VPN table belongs to AS %d", *table.LocalAS)
	}
	if table.RouterID == "" {
		return fmt.Errorf("FRR VPN table has no routerId")
	}
	if table.TotalRoutes == nil {
		return fmt.Errorf("FRR VPN table has no totalRoutes")
	}
	if table.TotalPaths == nil {
		return fmt.Errorf("FRR VPN table has no totalPaths")
	}
	if table.Routes.RDs == nil {
		return fmt.Errorf("FRR VPN table has no routeDistinguishers object")
	}
	if *table.TotalRoutes != len(want) {
		return fmt.Errorf("FRR VPN table holds %d routes, want %d", *table.TotalRoutes, len(want))
	}
	if *table.TotalPaths != len(want) {
		return fmt.Errorf("FRR VPN table holds %d paths, want %d", *table.TotalPaths, len(want))
	}
	count := 0
	for rd, prefixes := range table.Routes.RDs {
		for received, paths := range prefixes {
			count++
			if received != prefix {
				return fmt.Errorf("unexpected VPN prefix %s at RD %s", received, rd)
			}
			if len(paths) != 1 {
				return fmt.Errorf("VPN RD %s prefix %s holds %d paths", rd, prefix, len(paths))
			}
			if paths[0].Peer != neighbor {
				return fmt.Errorf("VPN RD %s prefix %s came from %s, want %s", rd, prefix, paths[0].Peer, neighbor)
			}
			if paths[0].Path != "65001 65004" {
				return fmt.Errorf("VPN RD %s prefix %s carries AS_PATH %q", rd, prefix, paths[0].Path)
			}
		}
	}
	if count != len(want) {
		return fmt.Errorf("FRR VPN table enumerates %d routes, want %d", count, len(want))
	}
	for _, rd := range want {
		if len(table.Routes.RDs[rd][prefix]) != 1 {
			return fmt.Errorf("FRR VPN RD %s has no exact path for %s", rd, prefix)
		}
	}
	return nil
}

// FRR bgp_debug_rdpfxpath2str spells a VPN identity as RD <rd> <prefix>.
// Match that adjacent tuple after the receive verb, not three unrelated tokens.
func frrVPNDecodes(log, neighbor, rd, prefix string, withdrawn bool, label int) int {
	count := 0
	for line := range strings.SplitSeq(log, "\n") {
		fields := frrDecodeFields(line, neighbor, prefix)
		for index, field := range fields {
			if field != frrReceiveVerb {
				continue
			}
			identity := index + 1
			if withdrawn {
				if len(fields) < identity+2 {
					continue
				}
				if fields[identity] != "UPDATE" {
					continue
				}
				if fields[identity+1] != "about" {
					continue
				}
				identity += 2
			}
			if len(fields) < identity+5 {
				continue
			}
			if fields[identity] != "RD" {
				continue
			}
			if fields[identity+1] != rd {
				continue
			}
			if fields[identity+2] != prefix {
				continue
			}
			if fields[identity+3] != "label" {
				continue
			}
			if withdrawn {
				if fields[len(fields)-2] != "--" {
					continue
				}
				if fields[len(fields)-1] != frrWithdrawnMarker {
					continue
				}
			} else if fields[identity+4] != strconv.Itoa(label) {
				continue
			}
			count++
			break
		}
	}
	return count
}

func requireVPNAnnouncementLabels(ctx context.Context, lab interoplab.CheckerLab, neighbor string) error {
	log, err := lab.Query(ctx, peerFRR, []string{cmdCat, frrLogPath}, nil)
	if err != nil {
		return fmt.Errorf("read FRR VPN announcement log: %w", err)
	}
	for _, family := range vpnWithdrawFamilies() {
		for index, rd := range [...]string{"65004:10", "65004:20", "65004:30"} {
			if got := frrVPNDecodes(log, neighbor, rd, family.prefix, false, family.label+index); got != 1 {
				return fmt.Errorf("FRR logged %d announcements for RD %s prefix %s label %d, want 1", got, rd, family.prefix, family.label+index)
			}
		}
	}
	return nil
}

func requireVPNWithdrawalCounts(ctx context.Context, lab interoplab.CheckerLab, neighbor string, down bool) error {
	log, err := lab.Query(ctx, peerFRR, []string{cmdCat, frrLogPath}, nil)
	if err != nil {
		return fmt.Errorf("read FRR VPN withdrawal log: %w", err)
	}
	for _, family := range vpnWithdrawFamilies() {
		for _, rd := range [...]string{"65004:10", "65004:20", "65004:30"} {
			want := 1
			if rd == "65004:30" && !down {
				want = 0
			}
			if got := frrVPNDecodes(log, neighbor, rd, family.prefix, true, 0); got != want {
				return fmt.Errorf("FRR logged %d withdrawals for RD %s prefix %s, want %d (injector DOWN=%t)", got, rd, family.prefix, want, down)
			}
		}
	}
	return nil
}

// bgp_show_peer exports the receive counter as messageStats.keepalivesRecv.
// The session generation MUST remain the one observed before the announcements.
func vpnWithdrawKeepalives(ctx context.Context, lab interoplab.CheckerLab, neighbor string, generation uint64) (uint64, error) {
	output, err := lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp neighbor " + neighbor + " json"}, nil)
	if err != nil {
		return 0, err
	}
	var peers map[string]struct {
		State      string `json:"bgpState"`
		Generation uint64 `json:"connectionsEstablished"`
		Stats      struct {
			Received *uint64 `json:"keepalivesRecv"`
		} `json:"messageStats"`
	}
	if err := json.Unmarshal([]byte(output), &peers); err != nil {
		return 0, fmt.Errorf("decode FRR KEEPALIVE fence: %w", err)
	}
	peer := peers[neighbor]
	if peer.State != stateEstablished {
		return 0, fmt.Errorf("FRR session is not Established at withdrawal fence")
	}
	if peer.Generation != generation {
		return 0, fmt.Errorf("FRR session generation changed from %d to %d", generation, peer.Generation)
	}
	if peer.Stats.Received == nil {
		return 0, fmt.Errorf("FRR reports no received KEEPALIVE counter")
	}
	return *peer.Stats.Received, nil
}

func waitVPNWithdrawFence(ctx context.Context, lab interoplab.CheckerLab, neighbor string, generation uint64) error {
	before, err := vpnWithdrawKeepalives(ctx, lab, neighbor, generation)
	if err != nil {
		return err
	}
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 90 * time.Second, Interval: time.Second,
		Description: "FRR KEEPALIVE after injector DOWN withdrawals",
	}, func(probe context.Context) (uint64, error) {
		return vpnWithdrawKeepalives(probe, lab, neighbor, generation)
	}, func(received uint64) bool { return received > before })
	return err
}
