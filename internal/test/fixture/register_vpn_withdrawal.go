// Design: docs/guide/route-reflection.md -- VPN withdrawal inventory lifecycle.
// RFC 8277 Section 2.4 -- see rfc/short/rfc8277.md.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/vpn-withdrawal-inventory", vpnWithdrawalInventory)
}

// vpnWithdrawalInventory releases each source action only after the destination
// was sent the preceding UPDATE. Exact route assertions live in the peer script.
// An optional second argument is the UPDATE count the destination is sent once
// the source has left (default 4: two announcements, one withdrawal and one
// peer-down withdrawal per surviving VPN route). Pairs may follow it:
// `gates <n>` sets how many source actions the fixture releases (default 3, at
// most len(vpnWithdrawalGatePrefixes)), each gate after the first waiting until
// the destination was sent one more UPDATE; `release <peer address>` holds back a peer whose script announces only once it
// receives vpnWithdrawalReleasePrefix, which the fixture sends after that peer
// and both inventory peers are up, so a destination is never still connecting
// when the route arrives; `held <peer address>` then makes the source wait until
// bgp-rib holds a route from that peer.
func vpnWithdrawalInventory(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return errors.New("vpn-withdrawal-inventory requires rs or rr, then an optional final UPDATE count")
	}
	peers, err := vpnWithdrawalPeerArgs(args)
	if err != nil {
		return err
	}
	finalCount := 4
	if len(args) >= 2 {
		count, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("vpn-withdrawal-inventory final UPDATE count %q is not a number: %w", args[1], err)
		}
		if count < peers.gates {
			return fmt.Errorf("vpn-withdrawal-inventory final UPDATE count %d is below %d, the count before the source leaves", count, peers.gates)
		}
		finalCount = count
	}
	role := args[0]
	if role != "rs" {
		if role != "rr" {
			return fmt.Errorf("unsupported inventory role %q", role)
		}
	}
	peerCommand := commandShowRSPeers
	if role == "rr" {
		peerCommand = "show rr peers"
	}
	return runPlugin13(ctx, "vpn-withdrawal", nil, func(ctx context.Context, plugin *sdk.Plugin) error {
		if !Poll(ctx, 100, 250*time.Millisecond, func() bool {
			result := command13(ctx, plugin, peerCommand)
			if !done13(result) {
				return false
			}
			rows, ok := result.object()["peers"].([]any)
			if !ok {
				return false
			}
			up := make(map[string]bool)
			for _, value := range rows {
				row, ok := value.(map[string]any)
				if !ok {
					return false
				}
				address, _ := row["address"].(string)
				up[address], _ = row["up"].(bool)
			}
			for _, address := range peers.release {
				if !up[address] {
					return false
				}
			}
			return up["127.0.0.1"] && up["127.0.0.2"]
		}) {
			return errors.New("inventory owner never reported both VPN peers, and every released peer, up")
		}
		for _, address := range peers.release {
			command := "update text origin igp nhop 1.1.1.1 nlri add " + vpnWithdrawalReleasePrefix
			if _, _, err := plugin.UpdateRoute(ctx, address, command); err != nil {
				return fmt.Errorf("release of %s failed: %w", address, err)
			}
		}
		for _, address := range peers.held {
			if !vpnWithdrawalWaitHeld(ctx, plugin, address) {
				return fmt.Errorf("bgp-rib never held a route received from %s; the source acts only once it does", address)
			}
		}
		for index, prefix := range vpnWithdrawalGatePrefixes[:peers.gates] {
			if index > 0 {
				if !vpnWithdrawalWaitSent(ctx, plugin, index) {
					return fmt.Errorf("receiver was not sent VPN UPDATE %d", index)
				}
			}
			command := "update text origin igp nhop 1.1.1.1 nlri add " + prefix
			if _, _, err := plugin.UpdateRoute(ctx, "127.0.0.1", command); err != nil {
				return err
			}
		}
		if !vpnWithdrawalWaitSent(ctx, plugin, finalCount) {
			return fmt.Errorf("receiver was not sent %d UPDATEs, the retained withdrawals included, after source departure", finalCount)
		}
		return nil
	}, true)
}

// vpnWithdrawalReleasePrefix is the route the fixture sends a released peer.
// The peer's script expects it (NLRI bytes 18C61200) before it announces.
const vpnWithdrawalReleasePrefix = "198.18.0.0/24"

// vpnWithdrawalGatePrefixes are the routes the fixture sends the source
// (127.0.0.1), one per gate, in order. A source script expects each one
// (NLRI bytes 18C00002, 18C63364, 18CB0071, 18644000, 18644001) before its
// next action.
var vpnWithdrawalGatePrefixes = [...]string{"192.0.2.0/24", prefixTestNet2, "203.0.113.0/24", "100.64.0.0/24", "100.64.1.0/24"}

// vpnWithdrawalGatesDefault is the gate count when no `gates` pair is given.
const vpnWithdrawalGatesDefault = 3

// vpnWithdrawalPeers holds the addresses named in the `held` and `release`
// pairs, each list in argument order, and the gate count of the `gates` pair.
type vpnWithdrawalPeers struct {
	held    []string
	release []string
	gates   int
}

// vpnWithdrawalPeerArgs reads the `held <peer address>` and `release <peer
// address>` pairs in the arguments that follow the role and the final UPDATE
// count. A held peer is one whose received route bgp-rib must hold before the
// source acts, which is how a test orders a route the destination never sees
// ahead of the source. A released peer announces only once the fixture sends
// it vpnWithdrawalReleasePrefix, after the destination is up.
func vpnWithdrawalPeerArgs(args []string) (vpnWithdrawalPeers, error) {
	peers := vpnWithdrawalPeers{gates: vpnWithdrawalGatesDefault}
	if len(args) <= 2 {
		return peers, nil
	}
	rest := args[2:]
	for index := 0; index < len(rest); index += 2 {
		if index+1 >= len(rest) {
			return vpnWithdrawalPeers{}, vpnWithdrawalPairError(args)
		}
		keyword, address := rest[index], rest[index+1]
		if keyword == "gates" {
			gates, err := vpnWithdrawalGateCount(address)
			if err != nil {
				return vpnWithdrawalPeers{}, err
			}
			peers.gates = gates
			continue
		}
		if _, err := netip.ParseAddr(address); err != nil {
			return vpnWithdrawalPeers{}, fmt.Errorf("vpn-withdrawal-inventory %s peer %q is not an IP address: %w", keyword, address, err)
		}
		if keyword == "held" {
			peers.held = append(peers.held, address)
			continue
		}
		if keyword == "release" {
			peers.release = append(peers.release, address)
			continue
		}
		return vpnWithdrawalPeers{}, vpnWithdrawalPairError(args)
	}
	return peers, nil
}

// vpnWithdrawalGateCount reads the value of a `gates` pair: a number from the
// default up to the number of gate prefixes the fixture can send.
func vpnWithdrawalGateCount(value string) (int, error) {
	gates, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("vpn-withdrawal-inventory gates %q is not a number: %w", value, err)
	}
	if gates < vpnWithdrawalGatesDefault {
		return 0, fmt.Errorf("vpn-withdrawal-inventory gates %d is below %d", gates, vpnWithdrawalGatesDefault)
	}
	if gates > len(vpnWithdrawalGatePrefixes) {
		return 0, fmt.Errorf("vpn-withdrawal-inventory gates %d is above %d, the number of gate prefixes it can send", gates, len(vpnWithdrawalGatePrefixes))
	}
	return gates, nil
}

// vpnWithdrawalPairError names the argument grammar a malformed pair broke.
func vpnWithdrawalPairError(args []string) error {
	return fmt.Errorf("vpn-withdrawal-inventory takes rs or rr, an optional final UPDATE count, then `gates <n>`, `held <peer address>` or `release <peer address>` pairs; got %q", args)
}

// vpnWithdrawalWaitHeld polls bgp-rib's Adj-RIB-In until it holds a route row
// received from address. Every row names its peer, so the quoted address
// appears in the answer only once such a row exists.
func vpnWithdrawalWaitHeld(ctx context.Context, plugin *sdk.Plugin, address string) bool {
	quoted := strconv.Quote(address)
	return Poll(ctx, 100, 250*time.Millisecond, func() bool {
		result := command13(ctx, plugin, "show bgp rib received")
		return done13(result) && strings.Contains(result.text(), quoted)
	})
}

func vpnWithdrawalWaitSent(ctx context.Context, plugin *sdk.Plugin, count int) bool {
	return Poll(ctx, 100, 250*time.Millisecond, func() bool {
		result := command13(ctx, plugin, "show bgp peer 127.0.0.2 detail")
		return peerCounter13(result, "updates-sent")-peerCounter13(result, "eor-sent") >= count
	})
}
