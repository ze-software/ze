// Design: docs/architecture/testing/interop.md -- typed scenario registry.
// Related: check_special.go -- specialCheckers names this checker.
// Related: check_vpn_withdraw.go -- the FRR VPN table shape this file decodes.
// Related: check_labeled_withdraw.go -- frrRemoteLabel reads the label FRR holds.
// RFC: rfc/short/rfc7911.md
package bgp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// The route bgp-vpn-addpath-frr/ze.conf announces, with no path-information.
const (
	vpnAddPathScenario = "bgp-vpn-addpath-frr"
	vpnAddPathRD       = "65001:100"
	vpnAddPathPrefix   = "10.99.0.0/24"
	vpnAddPathLabel    = 1000
	frrIPv4VPNAddPath  = "ipv4Vpn"
)

// checkVPNAddPathFRR proves that FRR reads the ipv4/mpls-vpn NLRI ze writes for
// a route with no path-information, on a session where ADD-PATH is negotiated
// for that family. With ADD-PATH negotiated the NLRI starts with a 4-octet Path
// Identifier, and a Path Identifier of 0 is written like any other. A writer
// that omits the field for 0 hands FRR a Length octet, labels and an RD four
// octets out of place, so FRR never holds the exact RD, prefix and label below.
func checkVPNAddPathFRR(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return fmt.Errorf("%s has no selected IPv4 network", vpnAddPathScenario)
	}
	neighbor := networkHostAddress(check.Network, 2)
	if err := waitContains(ctx, check.Lab, peerFRR, frrNeighborCommand(neighbor), 90*time.Second, "BGP state = Established"); err != nil {
		return err
	}
	var command textbuf.Buffer
	neighborJSON, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", command.Str("show bgp neighbor ").Str(neighbor).Str(" json").String()}, nil)
	if err != nil {
		return err
	}
	// RFC 7911 Section 4: ze sends Path Identifiers only when FRR advertised
	// that it can receive them and ze advertised that it can send them.
	if err := requireVPNAddPathReceive(neighborJSON); err != nil {
		return fmt.Errorf("%w\n%s", err, strings.TrimSpace(neighborJSON))
	}
	// RFC 7911 Section 3: the Path Identifier precedes the Length octet.
	table, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 60 * time.Second, Interval: time.Second,
		Description: "FRR VPN route " + vpnAddPathRD + " " + vpnAddPathPrefix,
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, peerFRR, []string{cmdVtysh, "-c", "show bgp ipv4 vpn rd all json"}, nil)
	}, func(output string) bool {
		return requireVPNAddPathRoute(output, neighbor) == nil
	})
	if err != nil {
		return fmt.Errorf("%w: %w\n%s", err, requireVPNAddPathRoute(table, neighbor), strings.TrimSpace(table))
	}
	detailCommand := command.Reset().Str("show bgp ipv4 vpn rd ").Str(vpnAddPathRD).Byte(' ').Str(vpnAddPathPrefix).Str(" json").String()
	detail, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", detailCommand}, nil)
	if err != nil {
		return err
	}
	if label := frrRemoteLabel(detail); label != vpnAddPathLabel {
		return fmt.Errorf("FRR holds RD %s %s with remote label %d, want %d\n%s", vpnAddPathRD, vpnAddPathPrefix, label, vpnAddPathLabel, strings.TrimSpace(detail))
	}
	return nil
}

// requireVPNAddPathReceive answers nil when FRR's neighbor JSON says FRR both
// advertised and received the ADD-PATH capability that lets it receive Path
// Identifiers for ipv4 vpn, under neighborCapabilities.addPath.ipv4Vpn.
func requireVPNAddPathReceive(neighborJSON string) error {
	var document any
	if err := json.Unmarshal([]byte(neighborJSON), &document); err != nil {
		return fmt.Errorf("decode FRR neighbor JSON: %w", err)
	}
	addPath, ok := findJSONField(document, "addPath")
	if !ok {
		return fmt.Errorf("FRR neighbor JSON has no addPath capability")
	}
	families, ok := addPath.(map[string]any)
	if !ok {
		return fmt.Errorf("FRR addPath capability is not an object")
	}
	family, ok := families[frrIPv4VPNAddPath].(map[string]any)
	if !ok {
		return fmt.Errorf("FRR negotiated no ADD-PATH for %s", frrIPv4VPNAddPath)
	}
	if family["rxAdvertisedAndReceived"] != true {
		return fmt.Errorf("FRR ADD-PATH receive for %s is not advertised and received", frrIPv4VPNAddPath)
	}
	return nil
}

// requireVPNAddPathRoute answers nil when FRR's "rd all" VPN table holds exactly
// one route, vpnAddPathPrefix under vpnAddPathRD, with one path from neighbor.
// The table nests paths under routes.routeDistinguishers[RD][prefix], so the RD
// and the prefix are read as FRR keyed them rather than searched for as text.
func requireVPNAddPathRoute(output, neighbor string) error {
	var table struct {
		TotalRoutes *int `json:"totalRoutes"`
		Routes      struct {
			RDs map[string]map[string][]struct {
				Peer string `json:"peerId"`
			} `json:"routeDistinguishers"`
		} `json:"routes"`
	}
	if err := json.Unmarshal([]byte(output), &table); err != nil {
		return fmt.Errorf("decode FRR VPN table: %w", err)
	}
	if table.TotalRoutes == nil {
		return fmt.Errorf("FRR VPN table has no totalRoutes")
	}
	if *table.TotalRoutes != 1 {
		return fmt.Errorf("FRR VPN table holds %d routes, want 1", *table.TotalRoutes)
	}
	paths := table.Routes.RDs[vpnAddPathRD][vpnAddPathPrefix]
	if len(paths) != 1 {
		return fmt.Errorf("FRR VPN RD %s prefix %s holds %d paths, want 1", vpnAddPathRD, vpnAddPathPrefix, len(paths))
	}
	if paths[0].Peer != neighbor {
		return fmt.Errorf("FRR VPN RD %s prefix %s came from %s, want %s", vpnAddPathRD, vpnAddPathPrefix, paths[0].Peer, neighbor)
	}
	return nil
}
